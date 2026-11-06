package playground

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScenariosFileReloads(t *testing.T) {
	scenarios, err := LoadScenarios()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "scenarios.json")
	for _, title := range []string{"First revision", "Edited without rebuilding"} {
		scenarios[0].Title = title
		data, err := json.Marshal(scenarios)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadScenariosFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if loaded[0].Title != title {
			t.Fatalf("file reload returned %q, want %q", loaded[0].Title, title)
		}
	}
}

func TestLoadScenariosFileRejectsBrokenExamples(t *testing.T) {
	valid := `{"id":"demo","args":["sessions","list"],"responses":[{"path":"/sessions","status":200,"body":{}}]}`
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"malformed JSON", `[{`, "decode scenarios"},
		{"empty list", `[]`, "at least one"},
		{"missing scenario ID", `[{"args":[],"responses":[]}]`, "nonempty"},
		{"duplicate scenario IDs", `[` + valid + `,` + valid + `]`, "unique"},
		{"missing response body", `[` + strings.Replace(valid, `,"body":{}`, "", 1) + `]`, "invalid response"},
		{"query instead of path", `[` + strings.Replace(valid, `"/sessions"`, `"/sessions?page=1"`, 1) + `]`, "URL paths"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scenarios.json")
			if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadScenariosFile(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
