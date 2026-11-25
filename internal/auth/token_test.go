package auth

import (
	"strings"
	"testing"
)

func TestGenerateTokenFormat(t *testing.T) {
	tok, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(tok) != TokenHexLen {
		t.Errorf("token length = %d, want %d", len(tok), TokenHexLen)
	}
	if err := ValidateTokenFormat(tok); err != nil {
		t.Errorf("ValidateTokenFormat: %v", err)
	}
}

func TestValidateTokenFormat(t *testing.T) {
	// valid 40 hex
	valid := strings.Repeat("a", 40)
	if err := ValidateTokenFormat(valid); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	// too short
	if err := ValidateTokenFormat(strings.Repeat("a", 39)); err != ErrInvalidTokenFormat {
		t.Errorf("expected ErrInvalidTokenFormat, got %v", err)
	}
	// too long
	if err := ValidateTokenFormat(strings.Repeat("a", 41)); err != ErrInvalidTokenFormat {
		t.Errorf("expected ErrInvalidTokenFormat, got %v", err)
	}
	// non-hex
	bad := strings.Repeat("g", 40)
	if err := ValidateTokenFormat(bad); err != ErrInvalidTokenFormat {
		t.Errorf("expected ErrInvalidTokenFormat, got %v", err)
	}
}

func TestParseAuthorization(t *testing.T) {
	validToken := strings.Repeat("1", 40)
	tests := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{"empty", "", true},
		{"no scheme", validToken, true},
		{"wrong scheme", "Basic abc", true},
		{"token scheme lowercase", "token " + validToken, false},
		{"token scheme mixed case", "Token " + validToken, false},
		{"bearer scheme", "Bearer " + validToken, false},
		{"bearer scheme mixed", "bEaReR " + validToken, false},
		{"bad token", "Token " + strings.Repeat("z", 40), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseAuthorization(tt.header)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if parsed.Token != strings.ToLower(validToken) {
				t.Errorf("token = %q, want %q", parsed.Token, strings.ToLower(validToken))
			}
		})
	}
}
