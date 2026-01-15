package service

import (
	"testing"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{"valid email", "test@example.com", true},
		{"valid email with subdomain", "user@mail.example.com", true},
		{"missing @", "testexample.com", false},
		{"missing domain", "test@", false},
		{"missing local part", "@example.com", false},
		{"missing dot in domain", "test@example", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidEmail(tt.email)
			if got != tt.want {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  string
	}{
		{"normal email", "testuser@example.com", "te***@example.com"},
		{"short local part", "ab@example.com", "a***@example.com"}, // <=2 chars shows only first char
		{"single char", "a@example.com", "a***@example.com"},
		{"invalid email", "invalid", "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskEmail(tt.email)
			if got != tt.want {
				t.Errorf("maskEmail(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}
