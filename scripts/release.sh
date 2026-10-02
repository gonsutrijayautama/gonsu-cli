#!/usr/bin/env bash
# Membangun berkas rilis gonsu ke dist/: satu arsip per sistem dan
# arsitektur, plus SHA256SUMS. Dipanggil release.yml saat tag v* didorong, dan
# dapat dijalankan di laptop untuk mencoba installer:
#
#   scripts/release.sh v0.0.0-uji
#   GONSU_DOWNLOAD_BASE="$PWD/dist" GONSU_INSTALL_DIR=/tmp/gonsu-uji sh install.sh
set -euo pipefail

version=${1:?pemakaian: scripts/release.sh <versi>, misalnya v0.1.0}
dist=dist
rm -rf "$dist"
mkdir -p "$dist"

targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64)
for target in "${targets[@]}"; do
  os=${target%/*}
  arch=${target#*/}
  work=$(mktemp -d)
  binary=gonsu
  [ "$os" = windows ] && binary=gonsu.exe

  # CGO mati: binary statis, tanpa ketergantungan pada libc mesin pemakai.
  # -trimpath: path laptop atau runner tidak ikut tertanam.
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$work/$binary" ./cmd/gonsu
  cp LICENSE "$work/LICENSE"

  if [ "$os" = windows ]; then
    archive="$PWD/$dist/gonsu_${os}_${arch}.zip"
    if command -v zip >/dev/null; then
      (cd "$work" && zip -q -X "$archive" "$binary" LICENSE)
    else
      # Mesin tanpa zip: modul zipfile bawaan Python menghasilkan arsip yang sama.
      python=$(command -v python3 || command -v python)
      (cd "$work" && "$python" -m zipfile -c "$archive" "$binary" LICENSE)
    fi
  else
    tar -czf "$dist/gonsu_${os}_${arch}.tar.gz" -C "$work" "$binary" LICENSE
  fi
  rm -rf "$work"
done

# Nama berkas tanpa folder: installer mencocokkan baris "<hash>  <nama>".
if command -v sha256sum >/dev/null; then
  (cd "$dist" && sha256sum gonsu_* > SHA256SUMS)
else
  (cd "$dist" && shasum -a 256 gonsu_* > SHA256SUMS)
fi
ls -l "$dist"
