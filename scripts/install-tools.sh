#!/usr/bin/env sh
set -eu
# Install only into this checkout. No remote shell execution or global replacement.
version=2.7.2
platform=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo "Unsupported architecture" >&2; exit 1 ;; esac
case "$platform" in darwin|linux) ;; *) echo "Use GoFrame's official installation instructions on this platform" >&2; exit 1 ;; esac
mkdir -p .tools
if [ -x .tools/gf ]; then
  installed=$(.tools/gf version | head -n 1)
  if [ "$installed" = "v$version" ]; then exit 0; fi
  echo "Existing .tools/gf has another version; remove it explicitly before installing" >&2
  exit 1
fi
tmp=$(mktemp .tools/gf.XXXXXX)
trap 'rm -f "$tmp"' EXIT HUP INT TERM
curl --fail --location --retry 2 "https://github.com/gogf/gf/releases/download/v${version}/gf_${platform}_${arch}" -o "$tmp"
chmod 755 "$tmp"
mv "$tmp" .tools/gf
.tools/gf version
