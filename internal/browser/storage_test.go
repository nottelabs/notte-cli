package browser

import (
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/golang/snappy"
	"github.com/syndtr/goleveldb/leveldb"
)

func encodedUTF16(value string) []byte {
	units := utf16.Encode([]rune(value))
	out := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[i*2:], u)
	}
	return out
}
func chromeText(value string) []byte { return append([]byte{0}, encodedUTF16(value)...) }
func localStorageProfile(t *testing.T) (Profile, *leveldb.DB) {
	t.Helper()
	profile := Profile{Dir: "Default", Name: "Synthetic", path: t.TempDir()}
	dir := filepath.Join(profile.path, "Local Storage", "leveldb")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return profile, db
}

func putStorage(t *testing.T, db *leveldb.DB, origin, key, value string) {
	t.Helper()
	rawKey := append([]byte("_"+origin+"\x00"), chromeText(key)...)
	if err := db.Put(rawKey, chromeText(value), nil); err != nil {
		t.Fatal(err)
	}
}

func TestStorageOnlyProfileIncludesBlueskyLogin(t *testing.T) {
	p, db := localStorageProfile(t)
	value := `{"session":{"accounts":[{"accessJwt":"synthetic-access","refreshJwt":"synthetic-refresh"}]}}`
	putStorage(t, db, "https://bsky.app", "BSKY_STORAGE", value)
	putStorage(t, db, "https://unrelated.test", "secret", "excluded")
	putStorage(t, db, "https://evilbsky.app", "secret", "excluded")
	putStorage(t, db, "https://bsky.app^0https://other.test", "partitioned", "excluded")
	state, err := p.ReadState([]string{"bsky.app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Cookies) != 0 || len(state.LocalStorage) != 1 || state.LocalStorage["https://bsky.app"]["BSKY_STORAGE"] != value {
		t.Fatal("storage-only login was lost or domain filtering failed")
	}
}

func TestChromiumSnapshotReadsLiveWALAndLatestValues(t *testing.T) {
	p, db := localStorageProfile(t)
	for i := 0; i < 100; i++ {
		putStorage(t, db, "https://bsky.app", "login", strings.Repeat("x", i))
	}
	putStorage(t, db, "https://bsky.app", "gone", "old")
	if err := db.Delete(append([]byte("_https://bsky.app\x00"), chromeText("gone")...), nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		state, err := p.ReadState(nil)
		if err != nil {
			t.Fatal(err)
		}
		if state.LocalStorage["https://bsky.app"]["login"] != strings.Repeat("x", 99) {
			t.Fatal("stale WAL state")
		}
		if _, ok := state.LocalStorage["https://bsky.app"]["gone"]; ok {
			t.Fatal("deleted value was resurrected")
		}
	}
	if err := db.Put([]byte("still-open"), []byte("yes"), nil); err != nil {
		t.Fatal("reader modified or locked the original profile")
	}
}

func TestChromeStringEncodings(t *testing.T) {
	for _, value := range []string{"", "ascii", "éàø", "日本語", "🔐\x00\n</script>"} {
		decoded, err := decodeChromeString(chromeText(value))
		if err != nil || decoded != value {
			t.Fatalf("UTF-16 roundtrip: %v", err)
		}
	}
	decoded, err := decodeChromeString([]byte{1, 0xe9, 0xff})
	if err != nil || decoded != "éÿ" {
		t.Fatal("Chromium Latin-1 was decoded as UTF-8")
	}
	for _, bad := range [][]byte{nil, {2, 1}, {0, 1}, {0, 0, 0xd8}, {0, 0, 0xdc}} {
		if _, err := decodeChromeString(bad); err == nil {
			t.Fatal("invalid string accepted")
		}
	}
}

func TestStorageCorruptionAndSizeFailBeforeUpload(t *testing.T) {
	p, db := localStorageProfile(t)
	if err := db.Put(append([]byte("_https://bsky.app\x00"), chromeText("key")...), []byte{9}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadState([]string{"bsky.app"}); err == nil {
		t.Fatal("corrupt selected value accepted")
	}
	if _, err := p.ReadState([]string{"unrelated.test"}); err != nil {
		t.Fatal("unselected corrupt value must not block scoped sync")
	}
	putStorage(t, db, "https://bsky.app", "key", strings.Repeat("x", MaxStateBytes+1))
	if _, err := p.ReadState(nil); err == nil {
		t.Fatal("oversized storage accepted")
	}
}

func TestFirefoxStorageFormats(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		for _, conversion := range []int{0, 1} {
			t.Run(strings.Join([]string{string(rune('0' + conversion)), map[bool]string{false: "plain", true: "snappy"}[compressed]}, "-"), func(t *testing.T) {
				p := Profile{Browser: Browser{kind: firefoxKind}, path: t.TempDir()}
				dir := filepath.Join(p.path, "storage", "default", "https+++bsky.app", "ls")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				db, err := sql.Open("sqlite", filepath.Join(dir, "data.sqlite"))
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = db.Close() }()
				for _, stmt := range []string{`CREATE TABLE database(origin TEXT)`, `INSERT INTO database VALUES('https://bsky.app')`, `CREATE TABLE data(key TEXT, conversion_type INTEGER, compression_type INTEGER, value BLOB)`} {
					if _, err := db.Exec(stmt); err != nil {
						t.Fatal(err)
					}
				}
				value := `{"token":"synthetic-🔐"}`
				raw := []byte(value)
				if conversion == 0 {
					raw = encodedUTF16(value)
				}
				compression := 0
				if compressed {
					raw = snappy.Encode(nil, raw)
					compression = 1
				}
				if _, err := db.Exec(`INSERT INTO data VALUES(?,?,?,?)`, "BSKY_STORAGE", conversion, compression, raw); err != nil {
					t.Fatal(err)
				}
				state, err := p.ReadState([]string{"bsky.app"})
				if err != nil {
					t.Fatal(err)
				}
				if state.LocalStorage["https://bsky.app"]["BSKY_STORAGE"] != value {
					t.Fatal("Firefox login changed during decoding")
				}
			})
		}
	}
}

func FuzzChromeStringRoundTrip(f *testing.F) {
	for _, s := range []string{"", "token", "日本語🔐", "\x00\n\""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		raw := chromeText(value)
		decoded, err := decodeChromeString(raw)
		if err != nil {
			t.Fatal(err)
		}
		if decoded != string([]rune(value)) {
			t.Fatal("UTF-16 value changed")
		}
	})
}

func TestFirefoxLegacyLSNGStorage(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		p := Profile{Browser: Browser{kind: firefoxKind}, path: t.TempDir()}
		dir := filepath.Join(p.path, "storage", "default", "https+++bsky.app", "ls")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", filepath.Join(dir, "data.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		value := []byte("synthetic-legacy-🔐")
		compression := 0
		if compressed {
			value = snappy.Encode(nil, value)
			compression = 1
		}
		for _, statement := range []string{`CREATE TABLE database(origin TEXT)`, `INSERT INTO database VALUES('https://bsky.app')`, `CREATE TABLE data(key TEXT, compressed INTEGER, value BLOB)`} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`INSERT INTO data VALUES(?,?,?)`, "login", compression, value); err != nil {
			t.Fatal(err)
		}
		_ = db.Close()
		state, err := p.ReadState([]string{"bsky.app"})
		if err != nil || state.LocalStorage["https://bsky.app"]["login"] != "synthetic-legacy-🔐" {
			t.Fatal("legacy Firefox state changed", err)
		}
	}
}

func TestFirefoxScopedSyncSkipsUnrelatedCorruptDatabase(t *testing.T) {
	p := Profile{Browser: Browser{kind: firefoxKind}, path: t.TempDir()}
	dir := filepath.Join(p.path, "storage", "default", "https+++unrelated.test", "ls")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.sqlite"), []byte("broken database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReadState([]string{"bsky.app"}); err != nil {
		t.Fatal("unrelated corrupt storage must not block scoped sync", err)
	}
	if _, err := p.ReadState([]string{"unrelated.test"}); err == nil {
		t.Fatal("selected corrupt storage must fail")
	}
}
