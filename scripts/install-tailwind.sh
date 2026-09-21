#!/usr/bin/env sh
# Downloads the Tailwind CSS v4 standalone CLI (no Node.js required) to ./bin.
set -eu

VERSION="${TAILWIND_VERSION:-v4.3.3}"

case "$(uname -s)" in
  Darwin) os="macos" ;;
  Linux) os="linux" ;;
  *) echo "unsupported OS: $(uname -s) (download manually from github.com/tailwindlabs/tailwindcss/releases)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64 | aarch64) arch="arm64" ;;
  x86_64 | amd64) arch="x64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

mkdir -p bin
url="https://github.com/tailwindlabs/tailwindcss/releases/download/${VERSION}/tailwindcss-${os}-${arch}"
echo "Downloading ${url}"
curl -fsSL -o bin/tailwindcss "$url"
chmod +x bin/tailwindcss
./bin/tailwindcss --help | head -1
