package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	SessionCookieName   = "lynk_session"
	LoginCSRFCookieName = "lynk_login_csrf"
	sessionIDBytes      = 32
	csrfTokenBytes      = 32
)

// Session holds server-side session data. Never store raw API token here.
type Session struct {
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
}

// SessionStore is an in-memory process-scoped session store.
type SessionStore struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	ttl         time.Duration
	now         func() time.Time
	maxSessions int
}

func NewSessionStore(ttl time.Duration, maxSessions int) *SessionStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	if maxSessions <= 0 {
		maxSessions = 10000
	}
	return &SessionStore{
		sessions:    make(map[string]*Session),
		ttl:         ttl,
		now:         time.Now,
		maxSessions: maxSessions,
	}
}

func (s *SessionStore) generateID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create creates a new session for userID, returns sessionID and CSRF token.
func (s *SessionStore) Create(userID int64) (string, string, error) {
	sid, err := s.generateID(sessionIDBytes)
	if err != nil {
		return "", "", err
	}
	csrf, err := s.generateID(csrfTokenBytes)
	if err != nil {
		return "", "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	if len(s.sessions) >= s.maxSessions {
		// Evict the session with earliest expiry
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, v := range s.sessions {
			if first || v.ExpiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.ExpiresAt
				first = false
			}
		}
		if oldestKey != "" {
			delete(s.sessions, oldestKey)
		}
	}
	s.sessions[sid] = &Session{
		UserID:    userID,
		CSRFToken: csrf,
		ExpiresAt: s.now().Add(s.ttl),
	}
	return sid, csrf, nil
}

// Get returns session if valid and not expired.
func (s *SessionStore) Get(sessionID string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if s.now().After(sess.ExpiresAt) {
		delete(s.sessions, sessionID)
		return nil, false
	}
	return sess, true
}

// Delete removes a session.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// Rotate rotates CSRF token for an existing session (keeps userID/expiry).
func (s *SessionStore) RotateCSRF(sessionID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok || s.now().After(sess.ExpiresAt) {
		if ok {
			delete(s.sessions, sessionID)
		}
		return "", false
	}
	csrf, err := s.generateID(csrfTokenBytes)
	if err != nil {
		return "", false
	}
	sess.CSRFToken = csrf
	return csrf, true
}

func (s *SessionStore) cleanupLocked() {
	// Best effort cleanup of expired sessions (called under lock)
	now := s.now()
	for k, v := range s.sessions {
		if now.After(v.ExpiresAt) {
			delete(s.sessions, k)
		}
	}
}

// SetSessionCookie sets the session cookie.
func SetSessionCookie(w http.ResponseWriter, sessionID string, secure bool, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   maxAge,
	})
}

// ClearSessionCookie clears the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}
