package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestConfirmAction_Skip(t *testing.T) {
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	ok, err := ConfirmAction("persona", "persona_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation to be skipped")
	}
}

func TestConfirmActionWithIO_YesNo(t *testing.T) {
	var out bytes.Buffer
	ok, err := ConfirmActionWithIO(strings.NewReader("y\n"), &out, "vault", "vault_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected yes to confirm")
	}

	out.Reset()
	ok, err = ConfirmActionWithIO(strings.NewReader("no\n"), &out, "vault", "vault_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected no to decline")
	}
}

func TestConfirmActionWithIO_Errors(t *testing.T) {
	if _, err := ConfirmActionWithIO(strings.NewReader("y\n"), errWriter{}, "vault", "vault_123"); err == nil {
		t.Fatal("expected write error")
	}

	if _, err := ConfirmActionWithIO(errReader{}, &bytes.Buffer{}, "vault", "vault_123"); err == nil {
		t.Fatal("expected read error")
	}
}

func TestConfirmReplaceSession_Skip(t *testing.T) {
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	ok, err := confirmReplaceSession("sess_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation to be skipped")
	}
}

func TestConfirmReplaceSessionWithIO_YesNo(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"yes\n", true},
		{"Y\n", true},
		{"\n", false}, // default is no
		{"", false},   // non-interactive EOF is also no
		{"n\n", false},
		{"no\n", false},
		{"N\n", false},
		{"NO\n", false},
		{"maybe\n", false},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		ok, err := confirmReplaceSessionWithIO(strings.NewReader(tt.input), &out, "sess_123")
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", tt.input, err)
		}
		if ok != tt.expected {
			t.Fatalf("input %q: expected %v, got %v", tt.input, tt.expected, ok)
		}
	}

	// Verify prompt text
	var out bytes.Buffer
	_, _ = confirmReplaceSessionWithIO(strings.NewReader("y\n"), &out, "sess_abc")
	if !strings.Contains(out.String(), "sess_abc") {
		t.Errorf("expected prompt to contain session ID, got %q", out.String())
	}
	if !strings.Contains(out.String(), "[y/N]") {
		t.Errorf("expected prompt to contain [y/N], got %q", out.String())
	}
}

func TestConfirmReplaceSessionWithIO_Errors(t *testing.T) {
	if _, err := confirmReplaceSessionWithIO(strings.NewReader("y\n"), errWriter{}, "sess_123"); err == nil {
		t.Fatal("expected write error")
	}

	if _, err := confirmReplaceSessionWithIO(errReader{}, &bytes.Buffer{}, "sess_123"); err == nil {
		t.Fatal("expected read error")
	}
}
