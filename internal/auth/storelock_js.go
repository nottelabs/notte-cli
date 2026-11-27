//go:build js

package auth

import "os"

// A WebAssembly build runs one command at a time in a browser tab, so there
// is no other process to lock against.
func lockFile(*os.File) error { return nil }

func unlockFile(*os.File) error { return nil }
