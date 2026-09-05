//go:build !windows

package telegram

import (
	"errors"
	"os"
	"syscall"
)

// openCodeFileNoFollow opens the code file without following symlinks,
// so a symlink swap between check and read (TOCTOU) fails at open time
// with ELOOP instead of redirecting the read.
func openCodeFileNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}

func isSymlinkError(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}
