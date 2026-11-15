#!/usr/bin/env bash
# Build the two release programs of horus and their checksums.
#
#   scripts/build-release.sh <version> <out folder>
#
# <version> is the version without a leading v, for example 0.1.0. The folder
# gets exactly 3 files: horus-windows-amd64.exe, horus-darwin-arm64 and
# checksums.txt (the format of sha256sum, so that "sha256sum -c" reads it).
# The same script runs in the CI job release-build and in the release job.
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <version> <out folder>" >&2
  exit 2
fi
version="$1"
out="$2"
case "$version" in
  v*) echo "build-release: give the version without a leading v: $version" >&2; exit 2 ;;
  "") echo "build-release: the version is empty" >&2; exit 2 ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$out"
out="$(cd "$out" && pwd)"
rm -f "$out"/horus-windows-amd64.exe "$out"/horus-darwin-arm64 "$out"/checksums.txt

ldflags="-X github.com/hieplam/horus/internal/version.Version=${version}"
cd "$root"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$out/horus-windows-amd64.exe" ./cmd/horus
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$ldflags" -o "$out/horus-darwin-arm64" ./cmd/horus

cd "$out"
sha256sum horus-windows-amd64.exe horus-darwin-arm64 > checksums.txt
echo "build-release: $version"
cat checksums.txt
