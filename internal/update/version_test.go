package update

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    int
		wantErr bool
	}{
		{"equal", "0.0.10", "0.0.10", 0, false},
		{"equal with v prefix", "v0.0.10", "v0.0.10", 0, false},
		{"equal mixed prefix", "v1.2.3", "1.2.3", 0, false},
		{"a less than b patch", "0.0.10", "0.0.12", -1, false},
		{"a greater than b patch", "0.0.12", "0.0.10", 1, false},
		{"a less than b minor", "0.1.0", "0.2.0", -1, false},
		{"a less than b major", "1.0.0", "2.0.0", -1, false},
		{"a greater than b major", "2.0.0", "1.9.99", 1, false},
		{"complex comparison", "1.2.3", "1.2.4", -1, false},
		{"major wins over minor", "2.0.0", "1.99.99", 1, false},
		{"invalid a", "abc", "1.0.0", 0, true},
		{"invalid b", "1.0.0", "xyz", 0, true},
		{"too few segments", "1.0", "1.0.0", 0, true},
		{"too many segments uses first three", "1.0.0.0", "1.0.0", 0, true},
		{"negative segment", "-1.0.0", "1.0.0", 0, true},
		{"empty string", "", "1.0.0", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareVersions(%q, %q) error = %v, wantErr %v", tt.a, tt.b, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
		wantErr bool
	}{
		{"newer available", "0.0.10", "0.0.12", true, false},
		{"same version", "0.0.10", "0.0.10", false, false},
		{"older available", "0.0.12", "0.0.10", false, false},
		{"newer with v prefix", "v0.0.10", "v0.0.12", true, false},
		{"invalid version", "invalid", "1.0.0", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsNewer(tt.current, tt.latest)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsNewer(%q, %q) error = %v, wantErr %v", tt.current, tt.latest, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.0.0", "v1.0.0"},
		{"v1.0.0", "v1.0.0"},
		{"0.0.10", "v0.0.10"},
		{"v0.0.10", "v0.0.10"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatVersion(tt.input)
			if got != tt.want {
				t.Errorf("formatVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
