package cmd

import "testing"

func TestValidateFlags(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		err := ValidateFlags(
			func() error { return nil },
			func() error { return nil },
		)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("first fails", func(t *testing.T) {
		expectedErr := &validationError{"test"}
		err := ValidateFlags(
			func() error { return expectedErr },
			func() error { return nil },
		)
		if err != expectedErr {
			t.Errorf("expected first error, got %v", err)
		}
	})

	t.Run("empty validators", func(t *testing.T) {
		err := ValidateFlags()
		if err != nil {
			t.Errorf("expected nil for empty validators, got %v", err)
		}
	})
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid session id", "sess_abc123", false},
		{"empty id", "", true},
		{"invalid prefix", "invalid_123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateSessionID(tt.id)
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSessionID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBrowser(t *testing.T) {
	tests := []struct {
		name    string
		browser string
		wantErr bool
	}{
		{"chromium", "chromium", false},
		{"chrome", "chrome", false},
		{"chrome-nightly", "chrome-nightly", false},
		{"chrome-turbo", "chrome-turbo", false},
		{"firefox", "firefox", true},
		{"webkit", "webkit", true},
		{"invalid", "safari", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateBrowser(tt.browser)
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBrowser(%q) error = %v, wantErr %v", tt.browser, err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid url", "https://example.com", false},
		{"empty url (optional)", "", false},
		{"invalid url", "not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateURL(tt.url, "test")
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRequiredURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid url", "https://example.com", false},
		{"empty url (required)", "", true},
		{"invalid url", "not-a-url", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateRequiredURL(tt.url, "test")
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequiredURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{"valid json", `{"key": "value"}`, false},
		{"empty json (optional)", "", false},
		{"invalid json", "not json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateJSON(tt.json, "test")
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSON(%q) error = %v, wantErr %v", tt.json, err, tt.wantErr)
			}
		})
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"text", "text", false},
		{"json", "json", false},
		{"invalid", "xml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateOutputFormat(tt.format)
			err := validator()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutputFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}
