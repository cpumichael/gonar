# gonar

A Go implementation of the NAR (Nix Archive) format — a Go port inspired by
the Rust [`libnar`](https://github.com/ebkalderon/libnar) implementation.

`gonar` reads and writes the stable Nix Archive (`nix-archive-1`) format
used by Nix and Guix. It is cross-tested against real Nix and Guix archive
data, producing byte-identical, reproducible NAR output for conforming
archives.

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
```

```
usage: gonar <command> [arguments]

commands:
  pack [-o out.nar] [--checksum] [--force-stdout] [--status-file f] <path>
                               serialize path into NAR format
  unpack [--status-file f] <archive.nar> <dst>
                               extract a NAR archive into dst
  list [-l|-j|--jsonl] [--status-file f] <archive.nar>
                               print the entries in a NAR archive
                               (default: one name per line; -l: long form;
                               -j: pretty-printed JSON array document;
                               --jsonl: streaming JSON, one object per line)

  --checksum      (pack only) include archive's SHA-256 checksum in the json
                  status file
  --force-stdout  (pack only) write the archive to stdout even if stdout is
                  a terminal
  --status-file f (all commands) write a JSON {success, errors, ...} object
                  to f, decoupled from stdout/stderr so it composes with
                  pipelines like: gonar pack dir | zstd > out.nar.zst

flags must come before positional arguments.
```

Examples:

```sh
# Pack a directory to a file, or to stdout for piping.
gonar pack -o archive.nar ./some-dir
gonar pack ./some-dir | zstd > archive.nar.zst

# List entries: short names (default), long ls -l-style, or JSON.
gonar list archive.nar
gonar list -l archive.nar
gonar list -j archive.nar        # pretty-printed JSON array document
gonar list --jsonl archive.nar   # streaming: one compact JSON object per line

# Unpack an archive.
gonar unpack archive.nar ./dest

# Write a machine-readable {command, success, errors, checksum} result to a
# file, independent of stdout/stderr -- useful when stdout is piped elsewhere.
gonar pack --checksum --status-file=status.json ./some-dir | zstd > archive.nar.zst
```

## Requirements

Go 1.23+ (uses range-over-func iterators).
