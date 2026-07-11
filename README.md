# gonar

A Go implementation of the NAR (Nix Archive) format — a Go port of
[libnar](https://github.com/ebkalderon/libnar). Wire-compatible with the
Rust implementation: archives produced by one can be read by the other.

## Library

```go
import "github.com/cpumichael/gonar"

// Pack a file, directory, or symlink into NAR format.
err := gonar.Pack(w, "/path/to/dir")
data, err := gonar.PackBytes("/path/to/dir")

// Read entries out of a NAR archive.
a := gonar.NewArchive(r)
for entry, err := range a.Entries() {
    if err != nil {
        // handle error
    }
    fmt.Println(entry.Name(), entry.IsDir())
}

// Or unpack the whole archive to disk.
a := gonar.NewArchive(r)
err := a.Unpack("/path/to/dest")
```

By default, unpacked entries have their mtimes canonicalized to the Unix
epoch and extended attributes stripped, matching Nix's deterministic store
semantics. Use `Archive.SetCanonicalizeMtime` / `Archive.SetRemoveXattrs` (or
the per-`Entry` equivalents) to change this.

## CLI

```sh
go install github.com/cpumichael/gonar/cmd/gonar@latest

gonar pack -o archive.nar ./some-dir
gonar list archive.nar
gonar unpack archive.nar ./dest
```

## Requirements

Go 1.23+ (uses range-over-func iterators).
