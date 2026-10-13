package auth

import (
	"errors"
	"testing"
)

func TestIsSecureAPIURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://api.notte.cc", true},
		{"https://example.com:8443/v1", true},
		{"http://localhost:8000", true},
		{"http://127.0.0.1:8000", true},
		{"http://[::1]:8000", true},
		{"http://api.notte.cc", false},
		{"http://example.com:8000", false},
		{"http://10.0.0.5:8000", false},
		{"http://localhost.example.com", false},
		{"ftp://api.notte.cc", false},
		{"api.notte.cc", false},
		{"://bad", false},
	}
	for _, tt := range tests {
		if got := IsSecureAPIURL(tt.url); got != tt.want {
			t.Errorf("IsSecureAPIURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestCheckSavedKeyURL(t *testing.T) {
	if err := CheckSavedKeyURL("https://api.notte.cc"); err != nil {
		t.Fatalf("https: unexpected error %v", err)
	}
	err := CheckSavedKeyURL("http://api.notte.cc")
	if !errors.Is(err, ErrInsecureAPIURL) {
		t.Fatalf("http: got %v, want ErrInsecureAPIURL", err)
	}
}
