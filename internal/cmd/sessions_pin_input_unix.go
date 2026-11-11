//go:build unix

package cmd

import (
	"time"

	"golang.org/x/sys/unix"
)

// inputReady reports whether fd has input waiting within timeout.
func inputReady(fd int, timeout time.Duration) bool {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, int(timeout.Milliseconds()))
	return err == nil && n > 0
}
