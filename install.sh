#!/bin/sh
# Install the horus program on macOS (Apple silicon).
#
#   curl -fsSL https://raw.githubusercontent.com/hieplam/horus/main/install.sh | sh
#
# The script downloads horus-darwin-arm64 and checksums.txt from a GitHub
# release, checks the SHA-256, and moves the program to <home>/.local/bin/horus.
# A second run replaces the program (an update).
#
# Settings (environment variables):
#   HORUS_VERSION            a release, for example 0.1.0. Default: the latest release.
#   HORUS_INSTALL_BASE_URL   replaces the address of the release files (the CI tests use it).
#   HORUS_INSTALL_DIR        replaces the folder of the program. Default: $HOME/.local/bin.
#   HORUS_INSTALL_NO_PATH=1  leaves the shell profile unchanged.
#
# The script sends no token and reads no secret.
set -eu

ASSET="horus-darwin-arm64"
PATH_MARKER="# added by the horus installer"

fail() {
  echo "install.sh: $*" >&2
  exit 1
}

os="$(uname -s)"
cpu="$(uname -m)"
if [ "$os" != "Darwin" ] || [ "$cpu" != "arm64" ]; then
  fail "this script supports macOS on Apple silicon (Darwin arm64) only. This machine is $os $cpu. On Windows, use install.ps1."
fi

if [ -n "${HORUS_INSTALL_BASE_URL:-}" ]; then
  base="${HORUS_INSTALL_BASE_URL%/}"
elif [ -n "${HORUS_VERSION:-}" ]; then
  base="https://github.com/hieplam/horus/releases/download/v${HORUS_VERSION#v}"
else
  base="https://github.com/hieplam/horus/releases/latest/download"
fi

if [ -n "${HORUS_INSTALL_DIR:-}" ]; then
  dir="$HORUS_INSTALL_DIR"
else
  [ -n "${HOME:-}" ] || fail "HOME is not set."
  dir="$HOME/.local/bin"
fi

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    fail "no program to calculate a SHA-256 (shasum or sha256sum)."
  fi
}

tmp="$(mktemp -d "${TMPDIR:-/tmp}/horus-install.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $ASSET from $base"
curl -fsSL "$base/$ASSET" -o "$tmp/$ASSET" || fail "cannot download $base/$ASSET"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "cannot download $base/checksums.txt"

expected="$(awk -v f="$ASSET" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/checksums.txt")"
[ -n "$expected" ] || fail "checksums.txt has no line for $ASSET."
actual="$(sha256_of "$tmp/$ASSET")"
if [ "$actual" != "$expected" ]; then
  rm -f "$tmp/$ASSET"
  echo "install.sh: the SHA-256 of the download is not the SHA-256 in checksums.txt. Nothing is installed." >&2
  echo "  expected: $expected" >&2
  echo "  actual:   $actual" >&2
  exit 1
fi

mkdir -p "$dir"
chmod 755 "$tmp/$ASSET"
mv -f "$tmp/$ASSET" "$dir/horus"
echo "Installed $dir/horus"

on_path=0
case ":${PATH:-}:" in
  *":$dir:"*) on_path=1 ;;
esac

if [ "$on_path" -eq 0 ] && [ "${HORUS_INSTALL_NO_PATH:-}" != "1" ]; then
  case "$(basename "${SHELL:-/bin/zsh}")" in
    zsh) profile="$HOME/.zprofile" ;;
    bash) profile="$HOME/.bash_profile" ;;
    *) profile="$HOME/.profile" ;;
  esac
  if [ -f "$profile" ] && grep -qF "$PATH_MARKER" "$profile"; then
    echo "The profile $profile already has the PATH line of horus."
  else
    printf '\nexport PATH="%s:$PATH" %s\n' "$dir" "$PATH_MARKER" >> "$profile"
    echo "Added $dir to the PATH in $profile. Open a new terminal."
  fi
fi

echo
echo "Next commands:"
echo "  horus setup it-illustration"
echo "  horus register"
