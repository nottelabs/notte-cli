package browser

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/golang/snappy"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const MaxStateBytes = 8 * 1024 * 1024

// ValidateDomains rejects malformed explicit scopes before any browser read.
// A typo must never turn a scoped sync into an unfiltered credential upload.
func ValidateDomains(domains []string) error {
	for _, value := range domains {
		host := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "."))
		if _, err := netip.ParseAddr(host); err == nil {
			continue
		}
		if host == "" || len(host) > 253 {
			return fmt.Errorf("--domain requires a hostname or IP address without a URL, path or port")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return fmt.Errorf("--domain requires a valid hostname or IP address")
			}
			for _, r := range label {
				if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
					return fmt.Errorf("--domain requires a hostname or IP address without a URL, path or port")
				}
			}
		}
	}
	return nil
}

// State is the same origin-scoped snapshot saved by a cloud browser. Cookies
// are mapped to the API shape by the command layer, never printed here.
type State struct {
	Cookies         []Cookie
	LocalStorage    map[string]map[string]string
	SessionStorage  map[string]map[string]string
	DecryptFailures int
}

// ReadState reads a private copy of on-disk state. In-memory sessionStorage is
// available only through ReadCDPState; it is never guessed from stale tab files.
func (p Profile) ReadState(domains []string) (State, error) {
	state := State{LocalStorage: map[string]map[string]string{}, SessionStorage: map[string]map[string]string{}}
	var err error
	if p.Browser.kind == firefoxKind {
		state.LocalStorage, err = p.readFirefoxStorage(domains)
	} else {
		state.LocalStorage, err = p.readChromiumStorage(domains)
	}
	if err != nil {
		return State{}, err
	}
	cookiePath := p.cookieDBPath()
	if p.Browser.kind == firefoxKind {
		cookiePath = filepath.Join(p.path, "cookies.sqlite")
	} else {
		for _, candidate := range []string{filepath.Join(p.path, "Network", "Cookies"), filepath.Join(p.path, "Cookies")} {
			if _, err := os.Stat(candidate); err != nil && !os.IsNotExist(err) {
				return State{}, fmt.Errorf("could not inspect cookie database; nothing was uploaded")
			}
		}
	}
	if _, err := os.Stat(cookiePath); err == nil && cookiePath != "" {
		result, err := p.ReadCookies(domains)
		if err != nil {
			return State{}, err
		}
		state.Cookies, state.DecryptFailures = result.Cookies, result.DecryptFailures
	} else if err != nil && !os.IsNotExist(err) {
		return State{}, fmt.Errorf("could not read cookie database")
	}
	if state.DecryptFailures > 0 {
		return State{}, fmt.Errorf("%d selected cookies could not be decrypted; unlock the browser keychain or use --cdp-url; nothing was uploaded", state.DecryptFailures)
	}
	return state, checkStateSize(state)
}

func checkStateSize(state State) error {
	raw, err := json.Marshal(state)
	if err != nil || len(raw) > MaxStateBytes {
		return fmt.Errorf("browser state exceeds the 8 MiB sync limit")
	}
	return nil
}

func storageOrigin(raw string, domains []string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	if !domainMatches(u.Hostname(), normalizeDomains(domains)) {
		return "", false
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = ""
	return u.String(), true
}

// Chromium stores keys as _<serialized StorageKey>\x00<encoded key>. A plain
// first-party StorageKey is its HTTP(S) origin. Partitioned StorageKeys carry
// additional separators and are excluded rather than flattened into logins.
func (p Profile) readChromiumStorage(domains []string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	source := filepath.Join(p.path, "Local Storage", "leveldb")
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return out, nil
	} else if err != nil {
		return nil, fmt.Errorf("could not inspect local storage")
	}
	tmp, err := os.MkdirTemp("", "notte-storage-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("could not read local storage directory")
	}
	var copied int64
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "LOCK" || (name != "CURRENT" && !strings.HasPrefix(name, "MANIFEST-") && !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".ldb") && !strings.HasSuffix(name, ".sst")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("local storage contains an unexpected file")
		}
		copied += info.Size()
		if copied > 512*1024*1024 {
			return nil, fmt.Errorf("local storage database is too large; use --cdp-url with --domain")
		}
		if err := copyFile(filepath.Join(source, name), filepath.Join(tmp, name)); err != nil {
			return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
		}
	}
	db, err := leveldb.OpenFile(tmp, &opt.Options{ReadOnly: true, ErrorIfMissing: true, Strict: opt.StrictAll})
	if err != nil {
		return nil, fmt.Errorf("could not open local storage snapshot; close the browser and retry")
	}
	defer func() { _ = db.Close() }()
	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	bytesRead := 0
	for iter.Next() {
		key := iter.Key()
		if len(key) == 0 || key[0] != '_' {
			continue
		}
		originBytes, itemKey, ok := bytes.Cut(key[1:], []byte{0})
		if !ok {
			continue
		}
		origin, allowed := storageOrigin(string(originBytes), domains)
		if !allowed {
			continue
		}
		if len(itemKey) > 2*MaxStateBytes+1 || len(iter.Value()) > 2*MaxStateBytes+1 {
			return nil, fmt.Errorf("selected local storage exceeds the 8 MiB sync limit")
		}
		name, err := decodeChromeString(itemKey)
		if err != nil {
			return nil, err
		}
		value, err := decodeChromeString(iter.Value())
		if err != nil {
			return nil, err
		}
		bytesRead += len(name) + len(value)
		if bytesRead > MaxStateBytes {
			return nil, fmt.Errorf("selected local storage exceeds the 8 MiB sync limit")
		}
		if out[origin] == nil {
			out[origin] = map[string]string{}
		}
		out[origin][name] = value
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("local storage snapshot is corrupt; nothing was uploaded")
	}
	return out, nil
}

func decodeChromeString(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("local storage contains an invalid encoded string")
	}
	switch raw[0] {
	case 1:
		runes := make([]rune, len(raw)-1)
		for i, b := range raw[1:] {
			runes[i] = rune(b)
		}
		return string(runes), nil
	case 0:
		return decodeUTF16(raw[1:])
	default:
		return "", fmt.Errorf("local storage uses an unsupported string encoding")
	}
}

func decodeUTF16(raw []byte) (string, error) {
	if len(raw)%2 != 0 {
		return "", fmt.Errorf("local storage contains truncated UTF-16")
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	// JSON/UTF-8 cannot preserve unpaired DOMString surrogates. Refuse instead
	// of changing a login token while reporting success.
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u <= 0xdbff {
			if i+1 == len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", fmt.Errorf("local storage contains an unsupported unpaired surrogate")
			}
			i++
		} else if u >= 0xdc00 && u <= 0xdfff {
			return "", fmt.Errorf("local storage contains an unsupported unpaired surrogate")
		}
	}
	return string(utf16.Decode(units)), nil
}

func (p Profile) readFirefoxStorage(domains []string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	root := filepath.Join(p.path, "storage", "default")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not inspect Firefox local storage")
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.Contains(entry.Name(), "^") {
			continue
		} // containers and partitioned origins
		// Firefox encodes scheme://host[:port] as scheme+++host[+port].
		// Filter the directory before touching unrelated databases, then still
		// validate the stored origin inside each selected database.
		parts := strings.SplitN(entry.Name(), "+++", 2)
		if len(parts) != 2 || (parts[0] != "http" && parts[0] != "https") {
			continue
		}
		if _, allowed := storageOrigin(parts[0]+"://"+strings.ReplaceAll(parts[1], "+", ":"), domains); !allowed {
			continue
		}
		dbPath := filepath.Join(root, entry.Name(), "ls", "data.sqlite")
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("could not inspect Firefox local storage")
		}
		if err := readFirefoxOrigin(dbPath, domains, out); err != nil {
			return nil, err
		}
		if err := checkStateSize(State{LocalStorage: out}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func readFirefoxOrigin(dbPath string, domains []string, out map[string]map[string]string) error {
	tmp, err := os.MkdirTemp("", "notte-storage-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	snapshot, err := snapshotDB(dbPath, tmp)
	if err != nil {
		return fmt.Errorf("could not snapshot Firefox local storage")
	}
	db, err := sql.Open("sqlite", "file:"+snapshot+"?mode=ro")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	var rawOrigin string
	if err := db.QueryRow(`SELECT origin FROM database`).Scan(&rawOrigin); err != nil {
		return fmt.Errorf("unsupported Firefox local storage metadata")
	}
	origin, allowed := storageOrigin(rawOrigin, domains)
	if !allowed {
		return nil
	}
	rows, err := db.Query(`SELECT key, conversion_type, compression_type, value FROM data`)
	if err != nil {
		// Firefox's earlier LSNG schema stores UTF-8 with a compressed flag.
		// Read it without migrating or modifying the source database.
		rows, err = db.Query(`SELECT key, 1, compressed, value FROM data`)
		if err != nil {
			return fmt.Errorf("unsupported Firefox local storage schema; open Firefox to migrate it and retry")
		}
	}
	defer func() { _ = rows.Close() }()
	out[origin] = map[string]string{}
	var total int
	for rows.Next() {
		var key string
		var conversion, compression int
		var raw []byte
		if err := rows.Scan(&key, &conversion, &compression, &raw); err != nil {
			return fmt.Errorf("could not read Firefox local storage")
		}
		if compression == 1 {
			size, err := snappy.DecodedLen(raw)
			if err != nil || size > MaxStateBytes {
				return fmt.Errorf("invalid or oversized compressed Firefox storage")
			}
			raw, err = snappy.Decode(nil, raw)
			if err != nil {
				return fmt.Errorf("corrupt Firefox local storage")
			}
		} else if compression != 0 {
			return fmt.Errorf("unsupported Firefox storage compression")
		}
		var value string
		switch conversion {
		case 0:
			value, err = decodeUTF16(raw)
		case 1:
			if !utf8.Valid(raw) {
				return fmt.Errorf("invalid UTF-8 in Firefox storage")
			}
			value = string(raw)
		default:
			return fmt.Errorf("unsupported Firefox storage encoding")
		}
		if err != nil {
			return err
		}
		total += len(key) + len(value)
		if total > MaxStateBytes {
			return fmt.Errorf("firefox local storage exceeds the sync limit")
		}
		out[origin][key] = value
	}
	return rows.Err()
}
