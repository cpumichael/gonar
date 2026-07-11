package gonar

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var epochTime = time.Unix(0, 0)

type entryKind int

const (
	kindDirectory entryKind = iota
	kindRegular
	kindSymlink
)

// Entry is a single file, directory, or symlink parsed out of an Archive.
type Entry struct {
	name              string // "/"-separated path relative to the archive root
	kind              entryKind
	executable        bool
	data              []byte
	target            string
	canonicalizeMtime bool
	removeXattrs      bool
}

func newDirectoryEntry(name string, a *Archive) *Entry {
	return &Entry{name: name, kind: kindDirectory, canonicalizeMtime: a.canonicalizeMtime, removeXattrs: a.removeXattrs}
}

func newRegularEntry(name string, executable bool, data []byte, a *Archive) *Entry {
	return &Entry{
		name: name, kind: kindRegular, executable: executable, data: data,
		canonicalizeMtime: a.canonicalizeMtime, removeXattrs: a.removeXattrs,
	}
}

func newSymlinkEntry(name, target string, a *Archive) *Entry {
	return &Entry{
		name: name, kind: kindSymlink, target: target,
		canonicalizeMtime: a.canonicalizeMtime, removeXattrs: a.removeXattrs,
	}
}

// Name returns the entry's path relative to the archive root, using "/" as
// the separator regardless of host OS.
func (e *Entry) Name() string { return e.name }

func (e *Entry) IsDir() bool { return e.kind == kindDirectory }

func (e *Entry) IsExecutable() bool { return e.kind == kindRegular && e.executable }

// IsFile reports whether the entry is a non-executable regular file.
func (e *Entry) IsFile() bool { return e.kind == kindRegular && !e.executable }

func (e *Entry) IsSymlink() bool { return e.kind == kindSymlink }

// Size returns the length of a regular file's contents in bytes. It is 0 for
// directories and symlinks.
func (e *Entry) Size() int64 { return int64(len(e.data)) }

// Target returns the symlink target. It is "" for non-symlink entries.
func (e *Entry) Target() string { return e.target }

func (e *Entry) SetCanonicalizeMtime(canonicalize bool) { e.canonicalizeMtime = canonicalize }

func (e *Entry) SetRemoveXattrs(remove bool) { e.removeXattrs = remove }

// String formats the entry as a single ls-l-style line: permissions, size,
// and name (with "-> target" appended for symlinks).
func (e *Entry) String() string {
	name := e.name
	if name == "" {
		name = "."
	}

	switch e.kind {
	case kindDirectory:
		return fmt.Sprintf("drwxr-xr-x %8s  %s", "-", name)
	case kindSymlink:
		return fmt.Sprintf("lrwxrwxrwx %8s  %s -> %s", "-", name, e.target)
	default:
		perm := "-r--r--r--"
		if e.executable {
			perm = "-r-xr-xr-x"
		}
		return fmt.Sprintf("%s %8d  %s", perm, len(e.data), name)
	}
}

// UnpackIn writes the entry into dst, joining it with the entry's relative
// name.
func (e *Entry) UnpackIn(dst string) error {
	rel := e.name

	// Validated per NAR-path segment, before any filepath.Join: Join calls
	// filepath.Clean, which silently collapses ".." components away, which
	// would defeat a traversal check performed on the joined path instead.
	if rel != "" {
		for _, comp := range strings.Split(rel, "/") {
			if comp == "" || comp == "." || comp == ".." {
				return fmt.Errorf("invalid path component in %q", rel)
			}
		}
	}

	path := dst
	if rel != "" {
		path = filepath.Join(dst, filepath.FromSlash(rel))
	}

	restoreParent := false
	parent := filepath.Dir(path)
	if rel != "" {
		if fi, err := os.Lstat(parent); err == nil && fi.ModTime().Equal(epochTime) {
			restoreParent = true
		}
	}

	var err error
	switch e.kind {
	case kindDirectory:
		err = unpackDir(path)
	case kindRegular:
		err = unpackFile(path, e.executable, e.data)
	case kindSymlink:
		err = unpackSymlink(path, e.target)
	}
	if err != nil {
		return err
	}

	if e.removeXattrs {
		if err := removeAllXattrs(path); err != nil {
			return err
		}
	}

	if e.canonicalizeMtime {
		if err := lchtimesZero(path); err != nil {
			return err
		}
	}

	if restoreParent {
		if err := lchtimesZero(parent); err != nil {
			return err
		}
	}

	return nil
}

func unpackDir(path string) error {
	if err := os.Mkdir(path, 0o755); err != nil {
		if os.IsExist(err) {
			if fi, statErr := os.Stat(path); statErr == nil && fi.IsDir() {
				return nil
			}
		}
		return fmt.Errorf("%w when creating dir %s", err, path)
	}
	return nil
}

func unpackFile(path string, executable bool, data []byte) error {
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	mode := os.FileMode(0o444)
	if executable {
		mode = 0o555
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func unpackSymlink(path, target string) error {
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.Symlink(target, path)
}
