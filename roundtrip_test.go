package gonar

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	src := t.TempDir()
	mustWriteFile(t, filepath.Join(src, "file.txt"), "hello world\n", 0o644)
	mustWriteFile(t, filepath.Join(src, "script.sh"), "#!/bin/sh\necho hi\n", 0o755)
	if err := os.Mkdir(filepath.Join(src, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(src, "subdir", "nested.txt"), "nested\n", 0o644)
	if err := os.Symlink("./file.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Pack(&buf, src); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	dst := t.TempDir()
	a := NewArchive(bytes.NewReader(buf.Bytes()))
	if err := a.Unpack(dst); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "file.txt"), "hello world\n")
	assertFileContent(t, filepath.Join(dst, "script.sh"), "#!/bin/sh\necho hi\n")
	assertFileContent(t, filepath.Join(dst, "subdir", "nested.txt"), "nested\n")
	assertExecutable(t, filepath.Join(dst, "script.sh"), true)
	assertExecutable(t, filepath.Join(dst, "file.txt"), false)

	target, err := os.Readlink(filepath.Join(dst, "link"))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != "./file.txt" {
		t.Errorf("symlink target = %q, want %q", target, "./file.txt")
	}

	fi, err := os.Lstat(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(epochTime) {
		t.Errorf("mtime = %v, want epoch", fi.ModTime())
	}
}

func TestRoundTripViaEntriesIterator(t *testing.T) {
	src := t.TempDir()
	mustWriteFile(t, filepath.Join(src, "a.txt"), "aaa", 0o644)
	mustWriteFile(t, filepath.Join(src, "b.txt"), "bbb", 0o644)

	data, err := PackBytes(src)
	if err != nil {
		t.Fatalf("PackBytes: %v", err)
	}

	a := NewArchive(bytes.NewReader(data))
	names := map[string]bool{}
	for entry, err := range a.Entries() {
		if err != nil {
			t.Fatalf("Entries: %v", err)
		}
		names[entry.Name()] = true
	}

	for _, want := range []string{"", "a.txt", "b.txt"} {
		if !names[want] {
			t.Errorf("missing entry %q among %v", want, names)
		}
	}
}

func mustWriteFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("content of %s = %q, want %q", path, got, want)
	}
}

func assertExecutable(t *testing.T, path string, want bool) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	got := fi.Mode()&0o111 != 0
	if got != want {
		t.Errorf("executable(%s) = %v, want %v", path, got, want)
	}
}
