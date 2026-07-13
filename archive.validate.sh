#!/bin/bash

set -xeuo pipefail

cleanup() {
  rm -rf gonar-gzip.nar guix-gzip-1.10.nar guix-gzip-1.10 guix-gzip.status.json
  rm -rf gonar-glibc.nar nix-glibc.nar nix-glibc glibc.status.json
}
trap cleanup EXIT

# Fetch Guix NAR archive
curl -fsL 'https://ci.guix.gnu.org/nar/gzip/ncydgq2znms5n1d2k5yqshhf58nsixwv-gzip-1.10' \
  | gzip -dc > guix-gzip-1.10.nar

# Fetch Nix NAR archive
base=https://cache.nixos.org
curl -fsSL "$base/7crrmih8c52r8fbnqb933dxrsp44md93.narinfo" \
  | awk -v base="$base" '/^URL: / { print base "/" $2 }' \
  | xargs curl -fsL \
  | xz -dc > nix-glibc.nar

# Unpack them
gonar unpack guix-gzip-1.10.nar guix-gzip-1.10
gonar unpack nix-glibc.nar nix-glibc

# Repack archives with gonar and write machine-readable status JSON
gonar pack -o gonar-gzip.nar --checksum --status-file guix-gzip.status.json guix-gzip-1.10
gonar pack -o gonar-glibc.nar --checksum --status-file glibc.status.json nix-glibc

# Extract raw sha256 values from status JSON
sumgzip="$(jq -r '.checksum | ltrimstr("sha256:")' guix-gzip.status.json)"
sumglibc="$(jq -r '.checksum | ltrimstr("sha256:")' glibc.status.json)"

# Check gonar output and original archive are byte-identical
sha256sum -c - <<EOF
$sumgzip  gonar-gzip.nar
$sumgzip  guix-gzip-1.10.nar
$sumglibc  gonar-glibc.nar
$sumglibc  nix-glibc.nar
EOF

