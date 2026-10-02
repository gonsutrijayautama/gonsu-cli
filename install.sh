#!/bin/sh
# Memasang gonsu di macOS dan Linux:
#
#   curl -fsSL https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.sh | sh
#
# Yang dikerjakan: mengunduh binary rilis untuk sistem ini, mencocokkan
# SHA-256-nya dengan SHA256SUMS rilis yang sama, lalu menaruhnya di
# ~/.local/bin. Tidak butuh sudo dan tidak mengubah berkas profil shell.
#
# Pengaturan lewat environment:
#   GONSU_VERSION        versi yang dipasang, mis. 0.1.0 (bawaan: rilis terbaru)
#   GONSU_INSTALL_DIR    folder tujuan (bawaan: ~/.local/bin)
#   GONSU_DOWNLOAD_BASE  alamat atau folder berisi berkas rilis, menggantikan
#                        GitHub Releases — untuk menguji skrip ini
set -eu

repo="gonsutrijayautama/gonsu-cli"

fail() {
  echo "gonsu: $*" >&2
  exit 1
}

# fetch <sumber> <tujuan>: sumber berupa alamat, atau berkas di folder lokal.
fetch() {
  if [ -f "$1" ]; then
    cp "$1" "$2"
  elif command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    fail "butuh curl atau wget untuk mengunduh"
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    fail "butuh sha256sum atau shasum untuk memeriksa unduhan"
  fi
}

# Seluruh pekerjaan di dalam satu fungsi yang dipanggil di baris TERAKHIR:
# skrip yang terpotong di tengah unduhan tidak menjalankan apa pun.
main() {
  case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "sistem $(uname -s) belum didukung skrip ini; di Windows pakai install.ps1" ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) fail "arsitektur $(uname -m) belum didukung" ;;
  esac

  version="${GONSU_VERSION:-}"
  dir="${GONSU_INSTALL_DIR:-$HOME/.local/bin}"
  asset="gonsu_${os}_${arch}.tar.gz"
  if [ -n "${GONSU_DOWNLOAD_BASE:-}" ]; then
    base="$GONSU_DOWNLOAD_BASE"
  elif [ -n "$version" ]; then
    base="https://github.com/$repo/releases/download/v${version#v}"
  else
    base="https://github.com/$repo/releases/latest/download"
  fi

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT

  echo "Mengunduh $asset…"
  fetch "$base/$asset" "$tmp/$asset" || fail "gagal mengunduh $base/$asset"
  fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS" || fail "gagal mengunduh $base/SHA256SUMS"

  # Unduhan yang rusak atau tertukar tidak pernah dipasang.
  expected="$(grep "  $asset\$" "$tmp/SHA256SUMS" | cut -d' ' -f1)"
  [ -n "$expected" ] || fail "SHA256SUMS tidak memuat $asset"
  actual="$(sha256 "$tmp/$asset")"
  [ "$expected" = "$actual" ] || fail "SHA-256 $asset tidak cocok dengan SHA256SUMS; unduhan tidak dipasang"

  tar -xzf "$tmp/$asset" -C "$tmp" gonsu
  mkdir -p "$dir"
  # Lewat nama sementara lalu mv: gonsu lama yang sedang berjalan tidak
  # tertimpa separuh.
  cp "$tmp/gonsu" "$dir/.gonsu.new"
  chmod 755 "$dir/.gonsu.new"
  mv "$dir/.gonsu.new" "$dir/gonsu"

  echo "Terpasang: $("$dir/gonsu" version) di $dir/gonsu"
  case ":$PATH:" in
    *":$dir:"*) ;;
    *)
      echo
      echo "$dir belum ada di PATH. Tambahkan ke profil shell Anda:"
      echo "  export PATH=\"$dir:\$PATH\""
      ;;
  esac
  echo
  echo "Berikutnya: gonsu new <kode-produk>"
  echo "Starter kit GONSU privat; lihat bagian Memasang di README untuk akses git-nya:"
  echo "  https://github.com/$repo#memasang"
}

main "$@"
