package aac

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fingerprint = "f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"

// fakeAAC writes an aac that prints `connections` as given and records its environment.
func fakeAAC(t *testing.T, connections string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "aac")
	script := "#!/bin/sh\n" +
		"echo \"$NO_COLOR $*\" >> \"" + filepath.Join(dir, "calls") + "\"\n" +
		"cat <<'EOF'\n" + connections + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readStore(t *testing.T, home string) pskStore {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".access-protocol", pskStoreFile))
	if err != nil {
		t.Fatal(err)
	}
	var store pskStore
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestReusablePSKIsCreatedOnceInAACsStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	psk, err := ReusablePSK()
	if err != nil {
		t.Fatal(err)
	}
	store := readStore(t, home)
	if len(store.Psks) != 1 || store.Psks[0].PskHex != psk {
		t.Fatalf("store %+v does not hold %s", store, psk)
	}
	raw, err := hex.DecodeString(psk)
	if err != nil || len(raw) != pskLength {
		t.Fatalf("invalid PSK %q", psk)
	}
	// aac looks a PSK up by this id.
	digest := sha256.Sum256(raw)
	if store.Psks[0].PskID != hex.EncodeToString(digest[:8]) {
		t.Fatalf("psk_id %q is not aac's", store.Psks[0].PskID)
	}
	info, err := os.Stat(filepath.Join(home, ".access-protocol", pskStoreFile))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("store must be private: %v %v", info.Mode(), err)
	}

	again, err := ReusablePSK()
	if err != nil || again != psk {
		t.Fatalf("second call returned %q, %v; want the stored PSK", again, err)
	}
}

func TestReusablePSKUsesTheFirstPSKAACStored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	first, second := strings.Repeat("ab", 32), strings.Repeat("cd", 32)
	data := `{"psks":[{"psk_id":"x","psk_hex":"` + first + `","name":null,"created_at":1},{"psk_id":"y","psk_hex":"` + second + `","created_at":2}]}`
	if err := os.MkdirAll(filepath.Join(home, ".access-protocol"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".access-protocol", pskStoreFile), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	psk, err := ReusablePSK()
	if err != nil || psk != first {
		t.Fatalf("got %q, %v; want the first stored PSK", psk, err)
	}
	if len(readStore(t, home).Psks) != 2 {
		t.Fatal("the store must be left as aac wrote it")
	}
}

func TestReusablePSKRefusesAnInvalidStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".access-protocol"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`not json`, `{"psks":[{"psk_id":"x","psk_hex":"abcd","created_at":1}]}`} {
		if err := os.WriteFile(filepath.Join(home, ".access-protocol", pskStoreFile), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReusablePSK(); err == nil {
			t.Fatalf("store %q must be refused, never overwritten", data)
		}
	}
}

func TestFingerprintReadsTheListenerIdentity(t *testing.T) {
	binary := fakeAAC(t, "User client (listen side)\n  Your identity: "+fingerprint+"\n  Connections: (none)")
	got, err := Fingerprint(context.Background(), binary)
	if err != nil || got != fingerprint {
		t.Fatalf("got %q, %v", got, err)
	}
	calls, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "calls"))
	if err != nil || strings.TrimSpace(string(calls)) != "1 connections list --client-type user" {
		t.Fatalf("aac called as %q", calls)
	}
}

func TestFingerprintBeforeTheListenerRan(t *testing.T) {
	binary := fakeAAC(t, "User client (listen side)\n  Your identity: (no key yet)")
	if _, err := Fingerprint(context.Background(), binary); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("got %v, want ErrNoIdentity", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := WaitForFingerprint(ctx, binary, 10*time.Millisecond); err == nil {
		t.Fatal("waiting must end with the context")
	}
}

func TestToken(t *testing.T) {
	psk := strings.Repeat("ab", 32)
	token := Token(psk, fingerprint)
	if token != psk+"_"+fingerprint || !ValidToken(token) {
		t.Fatalf("invalid token %q", token)
	}
	for _, invalid := range []string{"ABC-DEF-GHI", strings.ToUpper(token), token + "\n", psk + "-" + fingerprint, token[1:]} {
		if ValidToken(invalid) {
			t.Errorf("%q must be refused", invalid)
		}
	}
}
