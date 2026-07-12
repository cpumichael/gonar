// Command gonar packs and unpacks NAR (Nix Archive) files.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/cpumichael/gonar"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "pack":
		err = runPack(os.Args[2:])
	case "unpack":
		err = runUnpack(os.Args[2:])
	case "list":
		err = runList(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "gonar: unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "gonar: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: gonar <command> [arguments]

commands:
  pack [-o out.nar] [--checksum] [--status-file f] <path>
                               serialize path into NAR format
  unpack [--status-file f] <archive.nar> <dst>
                               extract a NAR archive into dst
  list [-l|-j|--jsonl] [--status-file f] <archive.nar>
                               print the entries in a NAR archive
                               (default: one name per line; -l: long form;
                               -j: pretty-printed JSON array document;
                               --jsonl: streaming JSON, one object per line)

  --checksum      (pack only) print the archive's SHA-256 checksum to stderr
  --status-file f (all commands) write a JSON {success, errors, ...} object
                  to f, decoupled from stdout/stderr so it composes with
                  pipelines like: gonar pack dir | zstd > out.nar.zst

flags must come before positional arguments.
`)
}

type statusResult struct {
	Command  string   `json:"command"`
	Success  bool     `json:"success"`
	Errors   []string `json:"errors"`
	Checksum string   `json:"checksum,omitempty"`
}

func addStatusFileFlag(fs *flag.FlagSet) *string {
	return fs.String("status-file", "", "write a JSON status object (success, errors, ...) to this path")
}

func writeStatusFile(path string, result statusResult) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func runPack(args []string) (err error) {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	out := fs.String("o", "", "output file (default: stdout)")
	checksum := fs.Bool("checksum", false, "print the SHA-256 checksum of the archive to stderr")
	statusFile := addStatusFileFlag(fs)
	if perr := fs.Parse(args); perr != nil {
		return perr
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gonar pack [-o out.nar] [--checksum] [--status-file f] <path>")
	}
	path := fs.Arg(0)

	result := statusResult{Command: "pack", Errors: []string{}}
	defer func() {
		result.Success = err == nil
		if err != nil {
			result.Errors = []string{err.Error()}
		}
		if werr := writeStatusFile(*statusFile, result); werr != nil {
			fmt.Fprintf(os.Stderr, "gonar: failed to write status file: %v\n", werr)
		}
	}()

	w := os.Stdout
	if *out != "" {
		f, ferr := os.Create(*out)
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		w = f
	}

	bw := bufio.NewWriter(w)

	var hasher hash.Hash
	dst := io.Writer(bw)
	if *checksum {
		hasher = sha256.New()
		dst = io.MultiWriter(bw, hasher)
	}

	if err = gonar.Pack(dst, path); err != nil {
		return err
	}
	if err = bw.Flush(); err != nil {
		return err
	}

	if *checksum {
		result.Checksum = fmt.Sprintf("sha256:%x", hasher.Sum(nil))
		fmt.Fprintln(os.Stdout, result.Checksum)
	}
	return nil
}

func runUnpack(args []string) (err error) {
	fs := flag.NewFlagSet("unpack", flag.ExitOnError)
	statusFile := addStatusFileFlag(fs)
	if perr := fs.Parse(args); perr != nil {
		return perr
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("usage: gonar unpack [--status-file f] <archive.nar> <dst>")
	}
	archivePath, dst := fs.Arg(0), fs.Arg(1)

	result := statusResult{Command: "unpack", Errors: []string{}}
	defer func() {
		result.Success = err == nil
		if err != nil {
			result.Errors = []string{err.Error()}
		}
		if werr := writeStatusFile(*statusFile, result); werr != nil {
			fmt.Fprintf(os.Stderr, "gonar: failed to write status file: %v\n", werr)
		}
	}()

	f, ferr := os.Open(archivePath)
	if ferr != nil {
		return ferr
	}
	defer f.Close()

	a := gonar.NewArchive(bufio.NewReader(f))
	err = a.Unpack(dst)
	return err
}

func runList(args []string) (err error) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	long := fs.Bool("l", false, "long form: permissions, size, and name")
	jsonOut := fs.Bool("j", false, "JSON output: a single pretty-printed array document")
	jsonl := fs.Bool("jsonl", false, "JSON output: one compact object per line (streaming)")
	statusFile := addStatusFileFlag(fs)
	if perr := fs.Parse(args); perr != nil {
		return perr
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gonar list [-l|-j|--jsonl] [--status-file f] <archive.nar>")
	}
	if countTrue(*long, *jsonOut, *jsonl) > 1 {
		return fmt.Errorf("gonar list: -l, -j, and --jsonl are mutually exclusive")
	}

	result := statusResult{Command: "list", Errors: []string{}}
	defer func() {
		result.Success = err == nil
		if err != nil {
			result.Errors = []string{err.Error()}
		}
		if werr := writeStatusFile(*statusFile, result); werr != nil {
			fmt.Fprintf(os.Stderr, "gonar: failed to write status file: %v\n", werr)
		}
	}()

	f, ferr := os.Open(fs.Arg(0))
	if ferr != nil {
		return ferr
	}
	defer f.Close()

	a := gonar.NewArchive(bufio.NewReader(f))
	entries := []jsonEntry{}
	enc := json.NewEncoder(os.Stdout)

	for entry, entryErr := range a.Entries() {
		if entryErr != nil {
			return entryErr
		}
		switch {
		case *jsonOut:
			entries = append(entries, entryJSON(entry))
		case *jsonl:
			if err = enc.Encode(entryJSON(entry)); err != nil {
				return err
			}
		case *long:
			fmt.Println(entry)
		default:
			fmt.Println(shortName(entry))
		}
	}

	if *jsonOut {
		data, merr := json.MarshalIndent(entries, "", "  ")
		if merr != nil {
			return merr
		}
		fmt.Println(string(data))
	}
	return nil
}

func countTrue(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

func shortName(entry *gonar.Entry) string {
	name := entry.Name()
	if name == "" {
		name = "."
	}
	if entry.IsSymlink() {
		return name + " -> " + entry.Target()
	}
	return name
}

type jsonEntry struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Executable bool   `json:"executable,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Target     string `json:"target,omitempty"`
}

func entryJSON(entry *gonar.Entry) jsonEntry {
	je := jsonEntry{Name: entry.Name()}
	switch {
	case entry.IsDir():
		je.Kind = "directory"
	case entry.IsSymlink():
		je.Kind = "symlink"
		je.Target = entry.Target()
	default:
		je.Kind = "regular"
		je.Executable = entry.IsExecutable()
		je.Size = entry.Size()
	}
	return je
}
