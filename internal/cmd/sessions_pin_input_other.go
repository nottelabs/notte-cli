//go:build !unix

package cmd

import "time"

// inputReady cannot look ahead here, so a lone Esc is taken as the Esc key.
func inputReady(int, time.Duration) bool { return false }
