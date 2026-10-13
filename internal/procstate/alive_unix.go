//go:build !windows

package procstate

import (
	"errors"
	"syscall"
)

// Alive reports whether a process with pid exists. A process owned by another
// user still counts as alive.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
