package update

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareVersions compares two semver strings (with or without "v" prefix).
// Returns: -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) (int, error) {
	aParts, err := parseSemver(a)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", a, err)
	}
	bParts, err := parseSemver(b)
	if err != nil {
		return 0, fmt.Errorf("invalid version %q: %w", b, err)
	}

	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1, nil
		}
		if aParts[i] > bParts[i] {
			return 1, nil
		}
	}
	return 0, nil
}

// IsNewer returns true if latest is a newer version than current.
func IsNewer(current, latest string) (bool, error) {
	cmp, err := CompareVersions(current, latest)
	if err != nil {
		return false, err
	}
	return cmp < 0, nil
}

// parseSemver parses a version string into [major, minor, patch].
func parseSemver(v string) ([3]int, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("expected MAJOR.MINOR.PATCH, got %q", v)
	}

	var result [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("non-numeric segment %q: %w", p, err)
		}
		if n < 0 {
			return [3]int{}, fmt.Errorf("negative segment %d", n)
		}
		result[i] = n
	}
	return result, nil
}

// formatVersion ensures a version string has the "v" prefix.
func formatVersion(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}
