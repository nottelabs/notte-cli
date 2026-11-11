package output

import "testing"

func TestHyperlinksSupported(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      map[string]string
		terminal bool
		want     bool
	}{
		{"forced on, even when piped", map[string]string{"FORCE_HYPERLINK": "1"}, false, true},
		{"forced off", map[string]string{"FORCE_HYPERLINK": "0", "TERM_PROGRAM": "iTerm.app"}, true, false},
		{"iTerm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, true, true},
		{"VS Code", map[string]string{"TERM_PROGRAM": "vscode"}, true, true},
		{"macOS Terminal", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, true, false},
		{"piped", map[string]string{"TERM_PROGRAM": "iTerm.app"}, false, false},
		{"NO_COLOR", map[string]string{"TERM_PROGRAM": "iTerm.app", "NO_COLOR": "1"}, true, false},
		{"recent VTE", map[string]string{"VTE_VERSION": "6003"}, true, true},
		{"old VTE", map[string]string{"VTE_VERSION": "4200"}, true, false},
		{"Windows Terminal", map[string]string{"WT_SESSION": "abc"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := hyperlinksSupported(func(key string) string { return tc.env[key] }, tc.terminal)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
