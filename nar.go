// Package gonar implements the NAR (Nix Archive) format: a self-describing,
// length-prefixed serialization of a file, directory, or symlink tree.
package gonar

const (
	magic  = "nix-archive-1"
	padLen = 8
)
