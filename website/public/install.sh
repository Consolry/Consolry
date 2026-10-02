#!/bin/sh
# Installs Consolry on Linux.
#
#   curl -fsSL https://www.consolry.com/install.sh | sh
#
# It downloads the newest release from GitHub, checks it against the published
# fingerprint, and puts one program called "consolry" on your PATH. Nothing else is changed.
set -eu

repo="DinoNaedYT/Consolry"
base="https://github.com/$repo/releases/latest/download"

if [ "$(uname -s)" != "Linux" ]; then
  echo "This installer is for Linux. On Windows, download Consolry.exe from https://www.consolry.com/download" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *)
    echo "Consolry has no build for this processor ($(uname -m))." >&2
    exit 1
    ;;
esac

# Root installs for everyone; anyone else installs just for themselves, with no sudo needed.
if [ "$(id -u)" = "0" ]; then
  target="/usr/local/bin"
else
  target="$HOME/.local/bin"
fi
mkdir -p "$target"

file="consolry-linux-$arch"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Downloading Consolry for Linux ($arch)..."
curl -fsSL "$base/$file" -o "$work/$file"
curl -fsSL "$base/checksums.txt" -o "$work/checksums.txt"

echo "Checking the download..."
(cd "$work" && grep " $file\$" checksums.txt | sha256sum -c - >/dev/null) || {
  echo "The download did not match its published fingerprint, so it was not installed." >&2
  exit 1
}

chmod +x "$work/$file"
mv "$work/$file" "$target/consolry"

echo
echo "Consolry is installed at $target/consolry"
case ":$PATH:" in
  *":$target:"*) echo "Start it with:  consolry" ;;
  *) echo "Start it with:  $target/consolry" ;;
esac
echo "Then open http://127.0.0.1:8700 in a browser on this machine."
echo "On a server with no screen, see https://www.consolry.com/download for how to reach it from another computer."
