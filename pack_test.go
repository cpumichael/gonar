package gonar

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func padded(data []byte) []byte {
	var buf bytes.Buffer
	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], uint64(len(data)))
	buf.Write(lenBuf[:])
	buf.Write(data)
	if r := len(data) % 8; r > 0 {
		buf.Write(make([]byte, 8-r))
	}
	return buf.Bytes()
}

func field(s string) []byte { return padded([]byte(s)) }

func concat(chunks ...[]byte) []byte {
	var buf bytes.Buffer
	for _, c := range chunks {
		buf.Write(c)
	}
	return buf.Bytes()
}

func TestWritePaddedExactMultipleOfEight(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 16)
	var buf bytes.Buffer
	if err := writePadded(&buf, data); err != nil {
		t.Fatal(err)
	}
	want := append(binary.LittleEndian.AppendUint64(nil, 16), data...)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got %v, want %v", buf.Bytes(), want)
	}
}

func TestWritePaddedNonMultipleOfEight(t *testing.T) {
	data := bytes.Repeat([]byte{1}, 5)
	var buf bytes.Buffer
	if err := writePadded(&buf, data); err != nil {
		t.Fatal(err)
	}
	want := append(binary.LittleEndian.AppendUint64(nil, 5), data...)
	want = append(want, 0, 0, 0)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("got %v, want %v", buf.Bytes(), want)
	}
}

func TestPackRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("lorem ipsum dolor sic amet\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	want := concat(
		field(magic),
		field("("),
		field("type"),
		field("regular"),
		field("contents"),
		padded([]byte("lorem ipsum dolor sic amet\n")),
		field(")"),
	)

	got, err := PackBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestPackExecutableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.sh")
	content := "#!/bin/sh\nset -euo pipefail\nexit 0\n"
	if err := os.WriteFile(path, []byte(content), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o777); err != nil {
		t.Fatal(err)
	}

	want := concat(
		field(magic),
		field("("),
		field("type"),
		field("regular"),
		field("executable"),
		field(""),
		field("contents"),
		padded([]byte(content)),
		field(")"),
	)

	got, err := PackBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestPackSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo")
	if err := os.Symlink("./foo", path); err != nil {
		t.Fatal(err)
	}

	want := concat(
		field(magic),
		field("("),
		field("type"),
		field("symlink"),
		field("target"),
		field("./foo"),
		field(")"),
	)

	got, err := PackBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestPackDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "file"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	want := concat(
		field(magic),
		field("("),
		field("type"),
		field("directory"),
		field("entry"),
		field("("),
		field("name"),
		field("subdir"),
		field("node"),
		field("("),
		field("type"),
		field("directory"),
		field("entry"),
		field("("),
		field("name"),
		field("file"),
		field("node"),
		field("("),
		field("type"),
		field("regular"),
		field("contents"),
		padded([]byte("hello world")),
		field(")"),
		field(")"),
		field(")"),
		field(")"),
		field(")"),
	)

	got, err := PackBytes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}
