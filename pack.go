package gonar

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Pack serializes the file, directory, or symlink at path into NAR format,
// writing the result to w.
func Pack(w io.Writer, path string) error {
	if _, err := os.Lstat(path); err != nil {
		return fmt.Errorf("path not found: %w", err)
	}
	if err := writePadded(w, []byte(magic)); err != nil {
		return err
	}
	return encodeEntry(w, path)
}

// PackBytes serializes the file, directory, or symlink at path into NAR
// format and returns the result as a byte slice.
func PackBytes(path string) ([]byte, error) {
	var buf bytes.Buffer
	if err := Pack(&buf, path); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeEntry(w io.Writer, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if err := writePadded(w, []byte("(")); err != nil {
		return err
	}
	if err := writePadded(w, []byte("type")); err != nil {
		return err
	}

	switch {
	case info.IsDir():
		if err := encodeDirectory(w, path); err != nil {
			return err
		}
	case info.Mode()&fs.ModeSymlink != 0:
		if err := encodeSymlink(w, path); err != nil {
			return err
		}
	case info.Mode().IsRegular():
		if err := encodeRegular(w, path, info); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized file type at %s", path)
	}

	return writePadded(w, []byte(")"))
}

func encodeDirectory(w io.Writer, path string) error {
	if err := writePadded(w, []byte("directory")); err != nil {
		return err
	}

	// os.ReadDir returns entries sorted by filename, matching the byte-for-byte
	// determinism the NAR format requires.
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		if err := writePadded(w, []byte("entry")); err != nil {
			return err
		}
		if err := writePadded(w, []byte("(")); err != nil {
			return err
		}
		if err := writePadded(w, []byte("name")); err != nil {
			return err
		}
		if err := writePadded(w, []byte(name)); err != nil {
			return err
		}
		if err := writePadded(w, []byte("node")); err != nil {
			return err
		}
		if err := encodeEntry(w, filepath.Join(path, name)); err != nil {
			return err
		}
		if err := writePadded(w, []byte(")")); err != nil {
			return err
		}
	}

	return nil
}

func encodeSymlink(w io.Writer, path string) error {
	if err := writePadded(w, []byte("symlink")); err != nil {
		return err
	}
	if err := writePadded(w, []byte("target")); err != nil {
		return err
	}
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	return writePadded(w, []byte(target))
}

func encodeRegular(w io.Writer, path string, info fs.FileInfo) error {
	if err := writePadded(w, []byte("regular")); err != nil {
		return err
	}

	if info.Mode()&0o111 != 0 {
		if err := writePadded(w, []byte("executable")); err != nil {
			return err
		}
		if err := writePadded(w, []byte("")); err != nil {
			return err
		}
	}

	if err := writePadded(w, []byte("contents")); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return writePaddedFromReader(w, file, info.Size())
}

func writePadded(w io.Writer, data []byte) error {
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return writePadding(w, int64(len(data)))
}

func writePaddedFromReader(w io.Writer, r io.Reader, length int64) error {
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(length))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	return writePadding(w, length)
}

func writePadding(w io.Writer, dataLen int64) error {
	remainder := dataLen % padLen
	if remainder == 0 {
		return nil
	}
	padding := make([]byte, padLen-remainder)
	_, err := w.Write(padding)
	return err
}
