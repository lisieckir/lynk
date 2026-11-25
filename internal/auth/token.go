package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// TokenLengthBytes is the number of random bytes used to generate tokens.
	TokenLengthBytes = 20
	// TokenHexLen is the expected length of the hex-encoded token.
	TokenHexLen = 40
)

var (
	// ErrUnauthorized indicates authentication failed (invalid/missing/revoked credentials).
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidTokenFormat indicates the token is not a strict 40-hex string.
	ErrInvalidTokenFormat = errors.New("invalid token format")
	// ErrMissingAuthHeader indicates no Authorization header was provided.
	ErrMissingAuthHeader = errors.New("missing authorization header")
	// ErrUnsupportedAuthScheme indicates the Authorization scheme is not Token or Bearer.
	ErrUnsupportedAuthScheme = errors.New("unsupported authorization scheme")
)

var hex40Re = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)

// GenerateToken creates a cryptographically random 40-hex-character token.
// The raw token must be shown to the user once and never stored.
func GenerateToken() (string, error) {
	b := make([]byte, TokenLengthBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 digest of the token string (UTF-8 of the hex token).
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	out := make([]byte, len(h))
	copy(out, h[:])
	return out
}

// TokenPrefix returns a short non-secret prefix for display/lookup aid.
func TokenPrefix(token string) string {
	if len(token) < 8 {
		return token
	}
	return token[:8]
}

// ValidateTokenFormat checks that token is exactly 40 hexadecimal characters (case-insensitive).
func ValidateTokenFormat(token string) error {
	if len(token) != TokenHexLen {
		return ErrInvalidTokenFormat
	}
	if !hex40Re.MatchString(token) {
		return ErrInvalidTokenFormat
	}
	return nil
}

// ParsedAuth holds the extracted token from an Authorization header.
type ParsedAuth struct {
	Token string
}

// ParseAuthorization extracts and validates a token from the Authorization header.
// It accepts schemes "Token" and "Bearer" case-insensitively.
// It does not validate the token against a database; it only checks format and scheme.
func ParseAuthorization(authHeader string) (*ParsedAuth, error) {
	if strings.TrimSpace(authHeader) == "" {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, ErrMissingAuthHeader)
	}
	// Split into scheme and credentials (only first space)
	idx := strings.IndexAny(authHeader, " \t")
	if idx <= 0 {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, ErrUnsupportedAuthScheme)
	}
	scheme := authHeader[:idx]
	cred := strings.TrimSpace(authHeader[idx+1:])
	if cred == "" {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, ErrInvalidTokenFormat)
	}
	// Case-insensitive scheme check
	schemeLower := strings.ToLower(scheme)
	if schemeLower != "token" && schemeLower != "bearer" {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, ErrUnsupportedAuthScheme)
	}
	token := strings.TrimSpace(cred)
	if err := ValidateTokenFormat(token); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	return &ParsedAuth{Token: strings.ToLower(token)}, nil
}
