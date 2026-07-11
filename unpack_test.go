package gonar

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEntriesRejectsBadMagic(t *testing.T) {
	a := NewArchive(bytes.NewReader(field("not-nix-archive-1")))

	count := 0
	var gotErr error
	for _, err := range a.Entries() {
		count++
		gotErr = err
	}

	if count != 1 {
		t.Fatalf("expected exactly one yielded item, got %d", count)
	}
	if gotErr == nil {
		t.Fatal("expected an error for bad magic")
	}
}

func TestEntriesRejectsBadPadding(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(field(magic))

	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], 1)
	buf.Write(lenBuf[:])
	buf.WriteByte('(')
	buf.Write([]byte{0, 0, 0, 0, 0, 0, 1}) // last padding byte corrupted (non-zero)

	a := NewArchive(&buf)
	var gotErr error
	for _, err := range a.Entries() {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatal("expected bad padding error")
	}
}

func TestEntriesRejectsDotDotName(t *testing.T) {
	data := concat(
		field(magic),
		field("("),
		field("type"),
		field("directory"),
		field("entry"),
		field("("),
		field("name"),
		field(".."),
		field("node"),
		field("("),
		field("type"),
		field("directory"),
		field(")"),
		field(")"),
		field(")"),
	)

	a := NewArchive(bytes.NewReader(data))
	var gotErr error
	sawRootDir := false
	for entry, err := range a.Entries() {
		if err != nil {
			gotErr = err
			break
		}
		if entry.IsDir() {
			sawRootDir = true
		}
	}

	if !sawRootDir {
		t.Fatal("expected root directory entry to be yielded before the error")
	}
	if gotErr == nil {
		t.Fatal("expected error for invalid entry name \"..\"")
	}
}

func TestEntriesStopsEarlyOnBreak(t *testing.T) {
	data := concat(
		field(magic),
		field("("),
		field("type"),
		field("directory"),
		field("entry"),
		field("("),
		field("name"),
		field("a"),
		field("node"),
		field("("),
		field("type"),
		field("directory"),
		field(")"),
		field(")"),
		field("entry"),
		field("("),
		field("name"),
		field("b"),
		field("node"),
		field("("),
		field("type"),
		field("directory"),
		field(")"),
		field(")"),
		field(")"),
	)

	a := NewArchive(bytes.NewReader(data))
	count := 0
	for range a.Entries() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 entry before break, got %d", count)
	}
}

func TestUnpackInRejectsParentDirTraversal(t *testing.T) {
	dst := t.TempDir()
	e := &Entry{name: "../evil", kind: kindRegular, data: []byte("x")}
	if err := e.UnpackIn(dst); err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestUnpackInRejectsEmbeddedParentDirTraversal(t *testing.T) {
	dst := t.TempDir()
	e := &Entry{name: "foo/../../evil", kind: kindRegular, data: []byte("x")}
	if err := e.UnpackIn(dst); err == nil {
		t.Fatal("expected embedded .. traversal to be rejected")
	}
}
