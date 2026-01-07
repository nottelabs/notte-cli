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
