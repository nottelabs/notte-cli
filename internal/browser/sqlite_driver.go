//go:build !js

package browser

// The driver is left out of the WebAssembly build, which cannot compile it and
// has no local browser profiles to read.
import _ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
