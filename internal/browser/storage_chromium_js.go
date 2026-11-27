//go:build js

package browser

import "fmt"

// The WebAssembly build has no local browser profiles, and LevelDB does not
// compile for it.
func (p Profile) readChromiumStorage(domains []string) (map[string]map[string]string, error) {
	return nil, fmt.Errorf("reading a local browser profile is not available in the browser playground")
}
