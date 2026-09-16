//go:build !darwin && !linux

package browser

import (
	"fmt"
	"runtime"
)

// keyset is defined per platform; on unsupported platforms it is empty.
type keyset struct {
	v10 []byte
	v11 []byte
}

// keys reports that local cookie reading is not available on this platform yet.
// macOS and Linux are supported today; Windows uses App-Bound Encryption, which
// needs a separate, carefully-scoped implementation.
func (b Browser) keys() (keyset, error) {
	return keyset{}, fmt.Errorf("reading local browser cookies is not supported on %s yet", runtime.GOOS)
}
