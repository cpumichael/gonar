package gonar

import (
	"encoding/binary"
	"errors"
	"io"
	"iter"
	"unicode/utf8"
)

// Archive reads entries out of a NAR-formatted byte stream.
type Archive struct {
	r                 io.Reader
	position          int64
	canonicalizeMtime bool
	removeXattrs      bool
}

// NewArchive wraps r as a NAR archive reader. By default, unpacked entries
// have their mtimes canonicalized to the Unix epoch and their extended
// attributes stripped.
func NewArchive(r io.Reader) *Archive {
	return &Archive{r: r, canonicalizeMtime: true, removeXattrs: true}
}

// SetCanonicalizeMtime controls whether unpacked entries have their mtime
// reset to the Unix epoch.
func (a *Archive) SetCanonicalizeMtime(canonicalize bool) {
	a.canonicalizeMtime = canonicalize
}

// SetRemoveXattrs controls whether unpacked entries have their extended
// attributes stripped.
func (a *Archive) SetRemoveXattrs(remove bool) {
	a.removeXattrs = remove
}

// Entries lazily parses the archive and yields each entry in depth-first
// order, matching the on-disk layout. Stopping iteration early (via break)
// leaves the underlying reader at an unspecified position.
func (a *Archive) Entries() iter.Seq2[*Entry, error] {
	return func(yield func(*Entry, error) bool) {
		if a.position != 0 {
			yield(nil, errors.New("cannot call Entries unless reader is at position 0"))
			return
		}

		magicBytes, err := a.readBytesPadded()
		if err == nil && string(magicBytes) != magic {
			err = errors.New("not a valid NAR archive")
		}
		if err != nil {
			yield(nil, err)
			return
		}

		stopped := false
		if err := a.parseEntry(yield, &stopped, ""); err != nil && !stopped {
			yield(nil, err)
		}
	}
}

// Unpack parses the archive and writes every entry into dst.
func (a *Archive) Unpack(dst string) error {
	for entry, err := range a.Entries() {
		if err != nil {
			return err
		}
		if err := entry.UnpackIn(dst); err != nil {
			return err
		}
	}
	return nil
}

// parseEntry recursively descends the archive, calling yield for every entry
// parsed. It returns a non-nil error only for malformed archive data; if the
// consumer stops iteration early, *stopped is set and nil is returned instead
// so the caller doesn't surface a spurious final error.
func (a *Archive) parseEntry(yield func(*Entry, error) bool, stopped *bool, path string) error {
	tag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if tag != "(" {
		return errors.New("missing open tag")
	}

	tag, err = a.readUTF8Padded()
	if err != nil {
		return err
	}
	if tag != "type" {
		return errors.New("missing type tag")
	}

	kind, err := a.readUTF8Padded()
	if err != nil {
		return err
	}

	switch kind {
	case "regular":
		return a.parseRegular(yield, stopped, path)
	case "symlink":
		return a.parseSymlink(yield, stopped, path)
	case "directory":
		return a.parseDirectory(yield, stopped, path)
	default:
		return errors.New("unrecognized file type")
	}
}

func (a *Archive) parseRegular(yield func(*Entry, error) bool, stopped *bool, path string) error {
	executable := false
	tag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}

	if tag == "executable" {
		executable = true
		empty, err := a.readUTF8Padded()
		if err != nil {
			return err
		}
		if empty != "" {
			return errors.New("incorrect executable tag")
		}
		tag, err = a.readUTF8Padded()
		if err != nil {
			return err
		}
	}

	if tag != "contents" {
		return errors.New("missing contents tag")
	}

	data, err := a.readBytesPadded()
	if err != nil {
		return err
	}

	closeTag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if closeTag != ")" {
		return errors.New("missing regular close tag")
	}

	entry := newRegularEntry(path, executable, data, a)
	if !yield(entry, nil) {
		*stopped = true
	}
	return nil
}

func (a *Archive) parseSymlink(yield func(*Entry, error) bool, stopped *bool, path string) error {
	tag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if tag != "target" {
		return errors.New("missing target tag")
	}

	target, err := a.readUTF8Padded()
	if err != nil {
		return err
	}

	closeTag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if closeTag != ")" {
		return errors.New("missing symlink close tag")
	}

	entry := newSymlinkEntry(path, target, a)
	if !yield(entry, nil) {
		*stopped = true
	}
	return nil
}

func (a *Archive) parseDirectory(yield func(*Entry, error) bool, stopped *bool, path string) error {
	entry := newDirectoryEntry(path, a)
	if !yield(entry, nil) {
		*stopped = true
		return nil
	}

	for {
		tag, err := a.readUTF8Padded()
		if err != nil {
			return err
		}

		switch tag {
		case "entry":
			if err := a.parseDirectoryEntry(yield, stopped, path); err != nil {
				return err
			}
			if *stopped {
				return nil
			}
		case ")":
			return nil
		default:
			return errors.New("incorrect directory field")
		}
	}
}

func (a *Archive) parseDirectoryEntry(yield func(*Entry, error) bool, stopped *bool, parentPath string) error {
	open, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if open != "(" {
		return errors.New("missing nested open tag")
	}

	nameTag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if nameTag != "name" {
		return errors.New("missing name field")
	}

	name, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	switch name {
	case "":
		return errors.New("entry name is empty")
	case "/":
		return errors.New("invalid name `/`")
	case "~":
		return errors.New("invalid name `~`")
	case ".":
		return errors.New("invalid name `.`")
	case "..":
		return errors.New("invalid name `..`")
	}

	nodeTag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if nodeTag != "node" {
		return errors.New("missing node field")
	}

	childPath := name
	if parentPath != "" {
		childPath = parentPath + "/" + name
	}
	if err := a.parseEntry(yield, stopped, childPath); err != nil {
		return err
	}
	if *stopped {
		return nil
	}

	closeTag, err := a.readUTF8Padded()
	if err != nil {
		return err
	}
	if closeTag != ")" {
		return errors.New("missing nested close tag")
	}
	return nil
}

func (a *Archive) readFull(buf []byte) error {
	n, err := io.ReadFull(a.r, buf)
	a.position += int64(n)
	return err
}

func (a *Archive) readBytesPadded() ([]byte, error) {
	var lenBuf [8]byte
	if err := a.readFull(lenBuf[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint64(lenBuf[:])

	data := make([]byte, length)
	if err := a.readFull(data); err != nil {
		return nil, err
	}

	remainder := length % padLen
	if remainder > 0 {
		padding := make([]byte, padLen-remainder)
		if err := a.readFull(padding); err != nil {
			return nil, err
		}
		for _, b := range padding {
			if b != 0 {
				return nil, errors.New("bad archive padding")
			}
		}
	}

	return data, nil
}

func (a *Archive) readUTF8Padded() (string, error) {
	data, err := a.readBytesPadded()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", errors.New("invalid utf-8 in archive")
	}
	return string(data), nil
}
