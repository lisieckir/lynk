package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetLoginCSRFCookie sets a short-lived CSRF cookie for the login form (double-submit).
func SetLoginCSRFCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     LoginCSRFCookieName,
		Value:    token,
		Path:     "/login",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600, // 10 minutes
	})
}

// ClearLoginCSRFCookie clears the login CSRF cookie.
func ClearLoginCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     LoginCSRFCookieName,
		Value:    "",
		Path:     "/login",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}

// SetRegisterCSRFCookie sets a short-lived CSRF cookie for the register form.
func SetRegisterCSRFCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RegisterCSRFCookieName,
		Value:    token,
		Path:     "/register",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   600,
	})
}

// ClearRegisterCSRFCookie clears the register CSRF cookie.
func ClearRegisterCSRFCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RegisterCSRFCookieName,
		Value:    "",
		Path:     "/register",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	})
}

// ValidateLoginCSRF validates the double-submit CSRF for login: form token must equal cookie token.
func ValidateLoginCSRF(r *http.Request, formToken string) bool {
	c, err := r.Cookie(LoginCSRFCookieName)
	if err != nil || c.Value == "" || formToken == "" {
		return false
	}
	if len(c.Value) != len(formToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(formToken)) == 1
}

// ValidateRegisterCSRF validates the double-submit CSRF for register.
func ValidateRegisterCSRF(r *http.Request, formToken string) bool {
	c, err := r.Cookie(RegisterCSRFCookieName)
	if err != nil || c.Value == "" || formToken == "" {
		return false
	}
	if len(c.Value) != len(formToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(formToken)) == 1
}

// ValidateSessionCSRF validates CSRF token from form against session CSRF token.
func ValidateSessionCSRF(session *Session, formToken string) bool {
	if session == nil || formToken == "" || session.CSRFToken == "" {
		return false
	}
	if len(session.CSRFToken) != len(formToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(formToken)) == 1
}
