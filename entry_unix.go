//go:build unix

package gonar

import (
	"github.com/pkg/xattr"
	"golang.org/x/sys/unix"
)

func removeAllXattrs(path string) error {
	names, err := xattr.LList(path)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := xattr.LRemove(path, name); err != nil {
			return err
		}
	}
	return nil
}

// lchtimesZero sets both atime and mtime to the Unix epoch without following
// symlinks. The stdlib os.Chtimes always follows symlinks on unix, so this
// needs the raw lutimes syscall via x/sys.
func lchtimesZero(path string) error {
	zero := unix.NsecToTimeval(0)
	return unix.Lutimes(path, []unix.Timeval{zero, zero})
}
