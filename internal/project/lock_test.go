package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project that has never deployed has no lock, which is not an error.
func TestMissingLockIsEmptyNotAnError(t *testing.T) {
	lock, err := LoadLock(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Functions) != 0 || lock.Version != LockVersion {
		t.Fatalf("got %+v", lock)
	}
}

func TestRecordAndReadBackPerEnvironment(t *testing.T) {
	lock := &Lock{Version: LockVersion}
	lock.Record("fn/main.py", "prod", EnvState{FunctionID: "prod-id", SourceSHA256: "aaa", ArtifactSHA256: "111"})
	lock.Record("fn/main.py", "dev", EnvState{FunctionID: "dev-id", SourceSHA256: "bbb", ArtifactSHA256: "222"})

	prod, ok := lock.State("fn/main.py", "prod")
	if !ok || prod.FunctionID != "prod-id" || prod.SourceSHA256 != "aaa" {
		t.Fatalf("prod = %+v ok=%v", prod, ok)
	}
	dev, _ := lock.State("fn/main.py", "dev")
	if dev.FunctionID != "dev-id" {
		t.Fatalf("dev = %+v", dev)
	}
	if len(lock.Functions) != 1 {
		t.Fatalf("two environments should share one entry, got %d", len(lock.Functions))
	}
}

// The property that lets one tree serve every environment: deploying to prod
// must not mark dev as up to date.
func TestPushingToOneEnvironmentDoesNotTouchAnother(t *testing.T) {
	lock := &Lock{Version: LockVersion}
	lock.Record("fn/main.py", "dev", EnvState{FunctionID: "dev-id", SourceSHA256: "old"})
	lock.Record("fn/main.py", "prod", EnvState{FunctionID: "prod-id", SourceSHA256: "new"})

	dev, _ := lock.State("fn/main.py", "dev")
	if dev.SourceSHA256 != "old" {
		t.Fatalf("dev hash moved to %q when prod was written", dev.SourceSHA256)
	}
}

// Version metadata is allowed to go stale; the content hash is not. Tying the
// hash to a successful read-back is what minted duplicate upstream versions in
// marketplace when the confirmation request failed.
func TestRecordKeepsPreviousVersionWhenReadBackFailed(t *testing.T) {
	lock := &Lock{Version: LockVersion}
	lock.Record("fn/main.py", "prod", EnvState{
		FunctionID: "id", Version: "v1", Versions: []string{"v1"}, SourceSHA256: "aaa",
	})
	// A write that landed, but whose follow-up read failed: no version known.
	lock.Record("fn/main.py", "prod", EnvState{FunctionID: "id", SourceSHA256: "bbb"})

	st, _ := lock.State("fn/main.py", "prod")
	if st.SourceSHA256 != "bbb" {
		t.Fatalf("hash must advance to what was pushed, got %q", st.SourceSHA256)
	}
	if st.Version != "v1" || len(st.Versions) != 1 {
		t.Fatalf("version metadata should survive as stale, got %+v", st)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	lock := &Lock{Version: LockVersion}
	lock.Record("b/main.py", "prod", EnvState{FunctionID: "b", SourceSHA256: "2", ArtifactSHA256: "22"})
	lock.Record("a/main.py", "prod", EnvState{FunctionID: "a", SourceSHA256: "1", ArtifactSHA256: "11"})
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}

	back, err := LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Functions) != 2 || back.Functions[0].Path != "a/main.py" {
		t.Fatalf("not sorted on save: %+v", back.Functions)
	}
	st, _ := back.State("b/main.py", "prod")
	if st.ArtifactSHA256 != "22" {
		t.Fatalf("round trip lost data: %+v", st)
	}
}

// One function per line is what keeps a 2,000-entry lock reviewable: a change
// shows as one changed line rather than a reflowed block.
func TestSaveWritesOneFunctionPerLine(t *testing.T) {
	dir := t.TempDir()
	lock := &Lock{Version: LockVersion}
	for _, p := range []string{"a.py", "b.py", "c.py"} {
		lock.Record(p, "prod", EnvState{FunctionID: p, SourceSHA256: "x"})
	}
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, LockName))
	if err != nil {
		t.Fatal(err)
	}
	entries := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), `{"path"`) {
			entries++
		}
	}
	if entries != 3 {
		t.Fatalf("expected 3 single-line entries, got %d:\n%s", entries, raw)
	}
}

func TestSaveIsDeterministic(t *testing.T) {
	build := func() string {
		dir := t.TempDir()
		lock := &Lock{Version: LockVersion}
		lock.Record("z.py", "prod", EnvState{FunctionID: "z", SourceSHA256: "1"})
		lock.Record("a.py", "dev", EnvState{FunctionID: "a", SourceSHA256: "2"})
		lock.Record("a.py", "prod", EnvState{FunctionID: "a2", SourceSHA256: "3"})
		if err := lock.Save(dir); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, LockName))
		return string(raw)
	}
	first := build()
	for i := 0; i < 5; i++ {
		if build() != first {
			t.Fatal("lock output is not stable across runs")
		}
	}
}

func TestLockFromNewerSchemaIsRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LockName), []byte(`{"version":99,"functions":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLock(dir)
	if err == nil || !strings.Contains(err.Error(), "newer notte") {
		t.Fatalf("expected a schema-version error, got %v", err)
	}
}

func TestPruneDropsOnlyUnknownPaths(t *testing.T) {
	lock := &Lock{Version: LockVersion}
	for _, p := range []string{"keep.py", "gone.py"} {
		lock.Record(p, "prod", EnvState{FunctionID: p})
	}
	dropped := lock.Prune(map[string]bool{"keep.py": true})
	if len(dropped) != 1 || dropped[0] != "gone.py" {
		t.Fatalf("dropped = %v", dropped)
	}
	if len(lock.Functions) != 1 || lock.Functions[0].Path != "keep.py" {
		t.Fatalf("remaining = %+v", lock.Functions)
	}
}

func TestCorruptLockIsReportedNotIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LockName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLock(dir); err == nil {
		t.Fatal("a corrupt lock must be an error, not silently an empty one")
	}
}
