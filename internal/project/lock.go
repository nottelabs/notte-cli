package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LockVersion is the schema version of notte.lock.json.
const LockVersion = 1

// Lock maps functions in the tree to the function they became, per environment.
//
// Path is the identity and ids live per environment, which is what lets one
// tree serve dev, staging and prod. marketplace established both halves: an id
// in a filename ties the tree to one environment, and a tree-wide content hash
// means pushing to prod silently marks dev up to date.
type Lock struct {
	Version   int         `json:"version"`
	Functions []LockEntry `json:"functions"`
}

// LockEntry is one function's per-environment state.
type LockEntry struct {
	// Path is the entrypoint relative to the functions directory.
	Path string `json:"path"`
	// Envs is keyed by environment name.
	Envs map[string]EnvState `json:"envs"`
}

// EnvState is what one environment knows about one function.
type EnvState struct {
	FunctionID string `json:"function_id"`
	// Version is the server-assigned version last seen, e.g. v20260821_162138.
	Version  string   `json:"version,omitempty"`
	Versions []string `json:"versions,omitempty"`

	// SourceSHA256 covers the set of contributing source files and answers
	// "does this need deploying?".
	SourceSHA256 string `json:"source_sha256"`
	// ArtifactSHA256 covers the bundled bytes and answers "what changed
	// upstream?". Two hashes rather than one because bundling is lossy: the
	// artifact cannot be turned back into the sources that produced it.
	ArtifactSHA256 string `json:"artifact_sha256"`
}

// LoadLock reads notte.lock.json. A missing lock is an empty one, not an
// error: that is a project that has never deployed.
func LoadLock(dir string) (*Lock, error) {
	raw, err := os.ReadFile(filepath.Join(dir, LockName))
	if os.IsNotExist(err) {
		return &Lock{Version: LockVersion}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", LockName, err)
	}

	var lock Lock
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, fmt.Errorf("%s: %w", LockName, err)
	}
	if lock.Version > LockVersion {
		return nil, fmt.Errorf("%s was written by a newer notte (schema v%d, this build understands v%d)",
			LockName, lock.Version, LockVersion)
	}
	lock.Version = LockVersion
	return &lock, nil
}

// Save writes the lock, sorted, one function per line.
//
// The formatting is deliberate. marketplace carries 2,049 entries and stays
// reviewable in a diff only because each is a single line, so a change shows
// as one changed line rather than a reflowed block.
func (l *Lock) Save(dir string) error {
	sort.Slice(l.Functions, func(i, j int) bool { return l.Functions[i].Path < l.Functions[j].Path })

	var buf bytes.Buffer
	buf.WriteString("{\n")
	fmt.Fprintf(&buf, "  \"version\": %d,\n", l.Version)
	buf.WriteString("  \"functions\": [\n")
	for i, entry := range l.Functions {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("encode %s: %w", entry.Path, err)
		}
		buf.WriteString("    ")
		buf.Write(line)
		if i < len(l.Functions)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("  ]\n}\n")

	return os.WriteFile(filepath.Join(dir, LockName), buf.Bytes(), 0o644)
}

// Entry returns the entry for a path, or nil.
func (l *Lock) Entry(path string) *LockEntry {
	for i := range l.Functions {
		if l.Functions[i].Path == path {
			return &l.Functions[i]
		}
	}
	return nil
}

// State returns what env knows about path.
func (l *Lock) State(path, env string) (EnvState, bool) {
	entry := l.Entry(path)
	if entry == nil {
		return EnvState{}, false
	}
	st, ok := entry.Envs[env]
	return st, ok
}

// Record stores the outcome of a write to env.
//
// The content hashes always advance to what was pushed, even when the caller
// could not read the version back. marketplace learned this the hard way:
// tying the hash to a successful read-back meant a transient error on the
// confirmation request minted a duplicate upstream version on the next run.
// Stale version strings are recoverable; a re-push is not.
func (l *Lock) Record(path, env string, state EnvState) {
	entry := l.Entry(path)
	if entry == nil {
		l.Functions = append(l.Functions, LockEntry{Path: path, Envs: map[string]EnvState{}})
		entry = &l.Functions[len(l.Functions)-1]
	}
	if entry.Envs == nil {
		entry.Envs = map[string]EnvState{}
	}
	prev := entry.Envs[env]
	if state.Version == "" {
		state.Version = prev.Version
	}
	if state.Versions == nil {
		state.Versions = prev.Versions
	}
	entry.Envs[env] = state
}

// Prune drops entries whose path is no longer in the tree.
//
// Only safe to call after a complete walk. A partial run — a --limit, or a
// failed download — must never reach this, or absence gets read as deletion.
func (l *Lock) Prune(known map[string]bool) []string {
	var dropped []string
	kept := l.Functions[:0]
	for _, entry := range l.Functions {
		if known[entry.Path] {
			kept = append(kept, entry)
			continue
		}
		dropped = append(dropped, entry.Path)
	}
	l.Functions = kept
	sort.Strings(dropped)
	return dropped
}
