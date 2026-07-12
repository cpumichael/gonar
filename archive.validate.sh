#!/bin/bash

set -xeuo pipefail

cleanup() {
  rm -rf gonar-gzip.nar guix-gzip-1.10.nar guix-gzip-1.10
  rm -rf gonar-glibc.nar nix-glibc.nar nix-glibc
}
trap cleanup EXIT

# Fetch Guix nar archive
curl -fsL 'https://ci.guix.gnu.org/nar/gzip/ncydgq2znms5n1d2k5yqshhf58nsixwv-gzip-1.10' \
  | gzip -dc > guix-gzip-1.10.nar


# Fetch Nix nar archive
base=https://cache.nixos.org; \
curl -fsSL "$base/7crrmih8c52r8fbnqb933dxrsp44md93.narinfo" \
| awk -v base="$base" '/^URL: / { print base "/" $2 }' \
| xargs curl -fsL \
| xz -dc > nix-glibc.nar


# Unpack them
gonar unpack guix-gzip-1.10.nar guix-gzip-1.10
gonar unpack nix-glibc.nar nix-glibc

# Repack archives with gonar
sum=$(gonar pack -checksum -o gonar-gzip.nar guix-gzip-1.10)
sumgzip="${sum#sha256:}"

sum=$(gonar pack -checksum -o gonar-glibc.nar  nix-glibc)
sumglibc="${sum#sha256:}"

# Check output
sha256sum -c - <<EOF
$sumgzip gonar-gzip.nar
$sumgzip guix-gzip-1.10.nar
$sumglibc gonar-glibc.nar
$sumglibc nix-glibc.nar
EOF

