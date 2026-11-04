#!/usr/bin/env bash
# ==============================================================================
# AO Sphere Multi-Architecture Cross-Compilation Script (Bash / CI)
# Builds statically linked binaries for Linux, macOS, and Windows.
# ==============================================================================
set -euo pipefail

VERSION="${1:-1.0.0}"
OUTPUT_DIR="${2:-dist}"

echo "==> Starting multi-platform cross-compilation for AO Sphere v${VERSION}..."

rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

TARGETS=(
  "linux/amd64/"
  "linux/arm64/"
  "darwin/amd64/"
  "darwin/arm64/"
  "windows/amd64/.exe"
)

LDFLAGS="-s -w -X 'main.Version=v${VERSION}'"

for target in "${TARGETS[@]}"; do
  IFS="/" read -r GOOS GOARCH EXT <<< "${target}"
  BIN_NAME="aosphere-v${VERSION}-${GOOS}-${GOARCH}${EXT}"
  OUT_PATH="${OUTPUT_DIR}/${BIN_NAME}"

  echo "==> Compiling for ${GOOS}/${GOARCH} -> ${BIN_NAME}..."
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build \
    -trimpath \
    -ldflags "${LDFLAGS}" \
    -o "${OUT_PATH}" .

  # Generate sha256 checksum
  if command -v sha256sum &> /dev/null; then
    (cd "${OUTPUT_DIR}" && sha256sum "${BIN_NAME}" > "${BIN_NAME}.sha256")
  elif command -v shasum &> /dev/null; then
    (cd "${OUTPUT_DIR}" && shasum -a 256 "${BIN_NAME}" > "${BIN_NAME}.sha256")
  fi

  echo "    [SUCCESS] ${BIN_NAME} created."
done

echo "==> All target binaries compiled and checksummed into ${OUTPUT_DIR}/:"
ls -lh "${OUTPUT_DIR}"

