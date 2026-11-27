package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinUsernameLen and MaxUsernameLen bound accepted usernames.
	MinUsernameLen = 3
	MaxUsernameLen = 64
	// MinPasswordLen and MaxPasswordLen bound accepted passwords.
	MinPasswordLen = 8
	MaxPasswordLen = 72 // bcrypt limit is 72 bytes
	// bcryptCost is the work factor for password hashing. 10 is a reasonable default.
	bcryptCost = 10
)

var (
	ErrUsernameTooShort   = errors.New("username too short")
	ErrUsernameTooLong    = errors.New("username too long")
	ErrPasswordTooShort   = errors.New("password too short")
	ErrPasswordTooLong    = errors.New("password too long")
	ErrInvalidCredentials = errors.New("invalid username or password")
)

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// ValidateUsername checks username length bounds.
func ValidateUsername(username string) error {
	n := utf8.RuneCountInString(username)
	if n < MinUsernameLen {
		return ErrUsernameTooShort
	}
	if n > MaxUsernameLen {
		return ErrUsernameTooLong
	}
	return nil
}

// ValidatePassword checks password length bounds (by bytes for bcrypt compatibility).
func ValidatePassword(password string) error {
	bn := len(password)
	if bn < MinPasswordLen {
		return ErrPasswordTooShort
	}
	if bn > MaxPasswordLen {
		return ErrPasswordTooLong
	}
	_ = utf8.RuneCountInString(password) // still valid utf8 is fine; bcrypt uses bytes
	return nil
}
