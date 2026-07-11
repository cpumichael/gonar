// Command gonar packs and unpacks NAR (Nix Archive) files.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
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
  pack [-o out.nar] <path>    serialize path into NAR format
  unpack <archive.nar> <dst>  extract a NAR archive into dst
  list [-l|-j] <archive.nar>  print the entries in a NAR archive
                               (default: one name per line; -l: long form;
                               -j: JSON, one object per line)

flags must come before positional arguments.
`)
}

func runPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	out := fs.String("o", "", "output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gonar pack [-o out.nar] <path>")
	}
	path := fs.Arg(0)

	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}

	bw := bufio.NewWriter(w)
	if err := gonar.Pack(bw, path); err != nil {
		return err
	}
	return bw.Flush()
}

func runUnpack(args []string) error {
	fs := flag.NewFlagSet("unpack", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 2 {
		return fmt.Errorf("usage: gonar unpack <archive.nar> <dst>")
	}
	archivePath, dst := fs.Arg(0), fs.Arg(1)

	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	a := gonar.NewArchive(bufio.NewReader(f))
	return a.Unpack(dst)
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	long := fs.Bool("l", false, "long form: permissions, size, and name")
	jsonOut := fs.Bool("j", false, "JSON output: one object per line")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gonar list [-l|-j] <archive.nar>")
	}
	if *long && *jsonOut {
		return fmt.Errorf("gonar list: -l and -j are mutually exclusive")
	}

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	a := gonar.NewArchive(bufio.NewReader(f))
	enc := json.NewEncoder(os.Stdout)

	for entry, err := range a.Entries() {
		if err != nil {
			return err
		}
		switch {
		case *jsonOut:
			if err := enc.Encode(entryJSON(entry)); err != nil {
				return err
			}
		case *long:
			fmt.Println(entry)
		default:
			fmt.Println(shortName(entry))
		}
	}
	return nil
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
