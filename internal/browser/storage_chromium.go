//go:build !js

package browser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// // Chromium stores keys as _<serialized StorageKey>\x00<encoded key>. A plain
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
	type fileVersion struct {
		size     int64
		modified int64
	}
	versions := map[string]fileVersion{}
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
		versions[name] = fileVersion{info.Size(), info.ModTime().UnixNano()}
		if copied > 512*1024*1024 {
			return nil, fmt.Errorf("local storage database is too large; use --cdp-url with --domain")
		}
		if err := copyFile(filepath.Join(source, name), filepath.Join(tmp, name)); err != nil {
			return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
		}
	}
	// A multi-file copy is valid only while the source stays unchanged. A WAL
	// append, compaction or manifest rotation must fail rather than import a
	// potentially incomplete mix of database versions.
	after, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
	}
	for _, entry := range after {
		name := entry.Name()
		if entry.IsDir() || name == "LOCK" || (name != "CURRENT" && !strings.HasPrefix(name, "MANIFEST-") && !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".ldb") && !strings.HasSuffix(name, ".sst")) {
			continue
		}
		info, err := entry.Info()
		version, existed := versions[name]
		if err != nil || !existed || !info.Mode().IsRegular() || version != (fileVersion{info.Size(), info.ModTime().UnixNano()}) {
			return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
		}
		delete(versions, name)
	}
	if len(versions) != 0 {
		return nil, fmt.Errorf("local storage changed while being copied; close the browser and retry")
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
