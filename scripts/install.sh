#!/usr/bin/env sh
set -eu

VERSION="${1:-${GOSVC_VERSION:-}}"
REPOSITORY="${GOSVC_REPOSITORY:-__GOSVC_REPOSITORY__}"
INSTALL_DIR="${GOSVC_INSTALL_DIR:-$HOME/.local/bin}"

if [ -z "$VERSION" ]; then
  echo "usage: GOSVC_REPOSITORY=owner/repo $0 <version>" >&2
  exit 2
fi
if [ -z "$REPOSITORY" ]; then
  echo "GOSVC_REPOSITORY is required (for example ailtonmacedo/gosvc)" >&2
  exit 2
fi

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 3 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 3 ;;
esac

VERSION="${VERSION#v}"
ASSET="gosvc_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="${GOSVC_RELEASE_BASE_URL:-https://github.com/${REPOSITORY}/releases/download/v${VERSION}}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP_DIR}/${ASSET}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

EXPECTED="$(awk -v asset="$ASSET" '$2 == asset { print $1 }' "${TMP_DIR}/checksums.txt")"
if [ -z "$EXPECTED" ]; then
  echo "checksum for ${ASSET} not found" >&2
  exit 4
fi
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
else
  ACTUAL="$(shasum -a 256 "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
fi
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "checksum verification failed for ${ASSET}" >&2
  exit 4
fi

tar -xzf "${TMP_DIR}/${ASSET}" -C "$TMP_DIR"
mkdir -p "$INSTALL_DIR"
install -m 0755 "${TMP_DIR}/gosvc_${VERSION}_${OS}_${ARCH}/gosvc" "${INSTALL_DIR}/gosvc"
printf 'gosvc %s installed at %s/gosvc\n' "$VERSION" "$INSTALL_DIR"
