package auth

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/99designs/keyring"

	"github.com/nottelabs/notte-cli/internal/config"
)

// brokenKeyring fails every operation, like a locked Keychain over SSH.
type brokenKeyring struct{ removed []string }

var errBroken = errors.New("user interaction is not allowed")

func (*brokenKeyring) Get(string) (keyring.Item, error) { return keyring.Item{}, errBroken }
func (*brokenKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, errBroken
}
func (*brokenKeyring) Set(keyring.Item) error { return errBroken }
func (b *brokenKeyring) Remove(key string) error {
	b.removed = append(b.removed, key)
	return errBroken
}
func (*brokenKeyring) Keys() ([]string, error) { return nil, errBroken }

// useStores replaces the system and file stores for one test. A nil system
// store means the platform has none.
func useStores(t *testing.T, system, file keyring.Keyring) {
	t.Helper()
	origSystem, origFile := openSystemKeyring, openFileKeyring
	openSystemKeyring = func() (keyring.Keyring, error) { return system, nil }
	openFileKeyring = func() (keyring.Keyring, error) { return file, nil }
	t.Cleanup(func() { openSystemKeyring, openFileKeyring = origSystem, origFile })
}

func storeHas(t *testing.T, ring keyring.Keyring, key, want string) {
	t.Helper()
	item, err := ring.Get(key)
	if err != nil {
		t.Fatalf("Get(%q): %v", key, err)
	}
	if string(item.Data) != want {
		t.Fatalf("Get(%q) = %q, want %q", key, item.Data, want)
	}
}

func storeLacks(t *testing.T, ring keyring.Keyring, key string) {
	t.Helper()
	if _, err := ring.Get(key); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("Get(%q) error = %v, want ErrKeyNotFound", key, err)
	}
}

// memoryStore is an in-memory system store that, like the real ones, reports
// removing a missing key as an error.
type memoryStore struct{ *keyring.ArrayKeyring }

func (m memoryStore) Remove(key string) error {
	if _, err := m.Get(key); err != nil {
		return err
	}
	return m.ArrayKeyring.Remove(key)
}

func systemStore(kv ...string) memoryStore {
	ring := memoryStore{keyring.NewArrayKeyring(nil)}
	for i := 0; i < len(kv); i += 2 {
		_ = ring.Set(keyring.Item{Key: kv[i], Data: []byte(kv[i+1])})
	}
	return ring
}

// fileStore is the real encrypted file store, in a temporary directory.
func fileStore(t *testing.T, kv ...string) keyring.Keyring {
	t.Helper()
	cfg := keyringConfig(t.TempDir())
	cfg.AllowedBackends = []keyring.BackendType{keyring.FileBackend}
	ring, err := keyring.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(kv); i += 2 {
		if err := ring.Set(keyring.Item{Key: kv[i], Data: []byte(kv[i+1])}); err != nil {
			t.Fatal(err)
		}
	}
	return ring
}

func TestStores_GetReadsSystemStore(t *testing.T) {
	system := systemStore("api_key:prod", "system-key")
	useStores(t, system, fileStore(t))

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "system-key" {
		t.Fatalf("got (%q, %v), want system-key", got, err)
	}
}

func TestStores_GetPrefersCurrentFileEntryAndSyncsSystemStore(t *testing.T) {
	// A key saved to the file while the system store was unusable is newer
	// than the entry the system store kept.
	system := systemStore("api_key:prod", "old-key")
	file := fileStore(t)
	useStores(t, system, file)
	if err := file.Set(keyring.Item{Key: "api_key:prod", Data: []byte("new-key"), Description: fileCurrentMarker}); err != nil {
		t.Fatal(err)
	}

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "new-key" {
		t.Fatalf("got (%q, %v), want new-key", got, err)
	}
	storeHas(t, system, "api_key:prod", "new-key")
}

func TestStores_GetPrefersSystemStoreOverOlderFileEntry(t *testing.T) {
	// An older version saved A to the file on a headless login, then B to the
	// system store on a desktop login, and never removed A.
	system := systemStore("api_key:prod", "key-b")
	file := fileStore(t, "api_key:prod", "key-a")
	useStores(t, system, file)

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "key-b" {
		t.Fatalf("got (%q, %v), want key-b", got, err)
	}
	storeHas(t, system, "api_key:prod", "key-b")
}

func TestStores_GetMigratesFileEntryAndKeepsIt(t *testing.T) {
	system := systemStore()
	file := fileStore(t, "api_key:prod", "file-key")
	useStores(t, system, file)

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "file-key" {
		t.Fatalf("got (%q, %v), want file-key", got, err)
	}
	storeHas(t, system, "api_key:prod", "file-key")
	// Older SDK versions only read the file: the entry stays until the next
	// login or logout, now marked as an identical copy.
	storeHas(t, file, "api_key:prod", "file-key")
	if item, _ := file.Get("api_key:prod"); item.Description != fileCurrentMarker {
		t.Fatalf("migrated file entry description = %q, want %q", item.Description, fileCurrentMarker)
	}
}

func TestStores_GetReadsFileWhenSystemStoreFails(t *testing.T) {
	system := &brokenKeyring{}
	file := fileStore(t, "api_key:prod", "file-key")
	useStores(t, system, file)

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "file-key" {
		t.Fatalf("got (%q, %v), want file-key", got, err)
	}
}

func TestStores_GetWithoutSystemStore(t *testing.T) {
	file := fileStore(t, "api_key:prod", "file-key")
	useStores(t, nil, file)

	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "file-key" {
		t.Fatalf("got (%q, %v), want file-key", got, err)
	}

	_, err = getFromSystemKeyring("api_key:staging")
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("missing key error = %v, want ErrKeyNotFound", err)
	}
}

func TestStores_SetUsesSystemStoreAndDropsFileCopy(t *testing.T) {
	system := systemStore()
	file := fileStore(t, "api_key:prod", "old-key")
	useStores(t, system, file)

	if err := setInSystemKeyring("api_key:prod", "new-key"); err != nil {
		t.Fatal(err)
	}
	storeHas(t, system, "api_key:prod", "new-key")
	// A file copy would now hold an outdated key.
	storeLacks(t, file, "api_key:prod")
}

// staleKeyring holds an old key it can still return, but cannot change: a
// store that fails to write or remove.
type staleKeyring struct{ memoryStore }

func (staleKeyring) Set(keyring.Item) error { return errBroken }
func (staleKeyring) Remove(string) error    { return errBroken }

func TestStores_FallbackLoginIsNotShadowedByOldSystemKey(t *testing.T) {
	system := staleKeyring{systemStore("api_key:prod", "old-account-key")}
	file := fileStore(t)
	useStores(t, system, file)

	if err := setInSystemKeyring("api_key:prod", "new-account-key"); err != nil {
		t.Fatal(err)
	}
	got, err := getFromSystemKeyring("api_key:prod")
	if err != nil || got != "new-account-key" {
		t.Fatalf("got (%q, %v), want the key just saved", got, err)
	}
}

func TestStores_SetFallsBackToFile(t *testing.T) {
	system := &brokenKeyring{}
	file := fileStore(t)
	useStores(t, system, file)

	if err := setInSystemKeyring("api_key:prod", "new-key"); err != nil {
		t.Fatal(err)
	}
	storeHas(t, file, "api_key:prod", "new-key")
	// An older entry in the system store must not shadow the new key.
	if len(system.removed) != 1 || system.removed[0] != "api_key:prod" {
		t.Fatalf("system removals = %v, want [api_key:prod]", system.removed)
	}
}

func TestStores_SetWithoutSystemStore(t *testing.T) {
	file := fileStore(t)
	useStores(t, nil, file)

	if err := setInSystemKeyring("api_key:prod", "new-key"); err != nil {
		t.Fatal(err)
	}
	storeHas(t, file, "api_key:prod", "new-key")
}

func TestStores_DeleteRemovesBothCopies(t *testing.T) {
	system := systemStore("api_key:prod", "key")
	file := fileStore(t, "api_key:prod", "key")
	useStores(t, system, file)

	if err := deleteFromSystemKeyring("api_key:prod"); err != nil {
		t.Fatal(err)
	}
	storeLacks(t, system, "api_key:prod")
	storeLacks(t, file, "api_key:prod")

	err := deleteFromSystemKeyring("api_key:prod")
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("second delete error = %v, want ErrKeyNotFound", err)
	}
}

func TestStores_DeleteReportsSystemStoreFailure(t *testing.T) {
	file := fileStore(t, "api_key:prod", "key")
	useStores(t, &brokenKeyring{}, file)

	if err := deleteFromSystemKeyring("api_key:prod"); !errors.Is(err, errBroken) {
		t.Fatalf("error = %v, want the system store failure", err)
	}
	// The file copy is still removed.
	storeLacks(t, file, "api_key:prod")
}

func TestStores_TunnelSecretMigratesLikeAPIKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows file names cannot hold this key; the Credential Manager is used there.
		t.Skip("the file store cannot hold tunnel secrets on Windows")
	}
	const key = "tunnel:home:oauth_client_secret"
	system := systemStore()
	file := fileStore(t, key, "tskey-client-secret")
	useStores(t, system, file)
	ResetKeyring()

	got, err := GetKeyringSecret(key)
	if err != nil || got != "tskey-client-secret" {
		t.Fatalf("got (%q, %v), want the saved secret", got, err)
	}
	storeHas(t, system, key, "tskey-client-secret")
	storeHas(t, file, key, "tskey-client-secret")

	if err := DeleteKeyringSecret(key); err != nil {
		t.Fatal(err)
	}
	storeLacks(t, system, key)
	storeLacks(t, file, key)
}

func TestStores_APIKeyLoginAfterUpgrade(t *testing.T) {
	system := systemStore()
	file := fileStore(t, "api_key:prod", "file-key")
	useStores(t, system, file)
	ResetKeyring()
	t.Setenv("NOTTE_API_URL", "")

	key, source, err := GetAPIKey(t.TempDir() + "/missing.json")
	if err != nil || key != "file-key" || source != SourceKeyring {
		t.Fatalf("GetAPIKey = (%q, %q, %v), want the file key", key, source, err)
	}
	if err := SetKeyringAPIKey("new-key"); err != nil {
		t.Fatal(err)
	}
	storeHas(t, system, "api_key:prod", "new-key")
	storeLacks(t, file, "api_key:prod")
}

// pausingStore runs hook the first time a key is written, before writing it.
type pausingStore struct {
	memoryStore
	once sync.Once
	hook func()
}

func (p *pausingStore) Set(item keyring.Item) error {
	p.once.Do(p.hook)
	return p.memoryStore.Set(item)
}

func TestStores_LoginWaitsForMigration(t *testing.T) {
	config.SetTestConfigDir(t.TempDir())
	t.Cleanup(func() { config.SetTestConfigDir("") })
	file := fileStore(t, "api_key:prod", "old-key")
	login := make(chan error, 1)
	system := &pausingStore{memoryStore: systemStore()}
	system.hook = func() {
		// A login starts while this read is migrating the old key.
		go func() { login <- (&realKeyring{}).Set("api_key:prod", "new-key") }()
		select {
		case err := <-login:
			t.Errorf("login finished during the migration (err=%v)", err)
		case <-time.After(200 * time.Millisecond):
		}
	}
	useStores(t, system, file)

	if got, err := (&realKeyring{}).Get("api_key:prod"); err != nil || got != "old-key" {
		t.Fatalf("Get = (%q, %v), want old-key", got, err)
	}
	if err := <-login; err != nil {
		t.Fatal(err)
	}
	if got, err := (&realKeyring{}).Get("api_key:prod"); err != nil || got != "new-key" {
		t.Fatalf("after login Get = (%q, %v), want new-key", got, err)
	}
}
