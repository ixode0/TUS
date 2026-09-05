//go:build windows

package telegram

import "os"

// Windows has no O_NOFOLLOW; plain open is the best available primitive.
// Symlink protection there relies on default ACLs, not on open flags.
func openCodeFileNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY, 0)
}

func isSymlinkError(err error) bool {
	return false
}
