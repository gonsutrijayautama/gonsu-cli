# gonsu — CLI untuk membuat produk GONSU One

**Untuk tim yang membangun produk untuk dijual di GONSU One.**

`gonsu new` membuat project yang sejak menit pertama sudah dapat login lewat
GONSU, membaca hak pakai paketnya, memberi akses login ke karyawan pelanggan,
dan diterbitkan sebagai rilis — sehingga tim produk langsung menulis bisnis
aplikasinya.

```sh
gonsu new <kode-produk>
```

`gonsu` sendiri tidak membawa template. Ia **menarik starter kit** — repository
tersendiri berisi aplikasi sungguhan — mengganti identitas contohnya dengan
identitas produk Anda, lalu menyiapkan git dan dependency.

```sh
gonsu new toko-baju                    # kit terbaru
gonsu new toko-baju --version 0.1.1    # versi kit tertentu
```

Tanpa `--version`, yang diambil selalu keadaan terbaru kit (`main`), jadi
perubahan kit langsung dipakai tanpa memperbarui `gonsu`.

| Kit | Repository | Isi |
|---|---|---|
| `go-nextjs` | `gonsu-starter-go-nextjs` | backend Go, frontend Next.js |
| `nextjs` *(belum tersedia)* | `gonsu-starter-nextjs` | menunggu SDK GONSU untuk JavaScript |
| `laravel` *(belum tersedia)* | `gonsu-starter-laravel` | menunggu SDK GONSU untuk PHP |

Di terminal, `gonsu new` menanyakan yang belum diberikan: nama tampilan,
starter kit, module path Go, repository git, dan pemasangan dependency. Kit
yang belum tersedia tampil dengan alasannya dan tidak dapat dipilih.

Tanpa tanya-jawab — untuk skrip dan CI:

```sh
gonsu new toko --kit go-nextjs --module github.com/organisasi/toko -n
```

`gonsu new -h` untuk seluruh flag.

**Kode produk harus sama persis dengan kode produk di Console GONSU**: huruf
kecil, angka, dan tanda hubung di antaranya, 2–60 karakter.

## Memasang

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.ps1 | iex
```

Skripnya mengunduh binary rilis terbaru, mencocokkan SHA-256-nya dengan
`SHA256SUMS` rilis itu, lalu menaruhnya di `~/.local/bin` (Windows:
`%LOCALAPPDATA%\Programs\gonsu`, yang ditambahkan ke PATH pengguna). Tanpa sudo
dan tanpa hak administrator. Menjalankannya lagi memperbarui `gonsu`.

| Variabel | Arti |
|---|---|
| `GONSU_VERSION` | versi yang dipasang, misalnya `0.1.0`; bawaannya rilis terbaru |
| `GONSU_INSTALL_DIR` | folder tujuan |

Dengan Go: `go install github.com/gonsutrijayautama/gonsu-cli/cmd/gonsu@latest`.

Setiap binary rilis membawa bukti asal yang dapat diperiksa:
`gh attestation verify gonsu_linux_amd64.tar.gz --repo gonsutrijayautama/gonsu-cli`.

`gonsu new` menarik starter kit dengan git, jadi git harus terpasang. Kit yang
publik langsung bisa ditarik.

**Bila kit-nya privat**, yang dibutuhkan, sekali per laptop:

1. **Akses.** Akun GitHub Anda diundang ke repository kit itu.
2. **Git dapat masuk ke GitHub**, dengan salah satu cara:

   ```sh
   gh auth login && gh auth setup-git          # lewat GitHub CLI
   ```

   ```sh
   # atau lewat SSH key yang sudah terdaftar di akun GitHub Anda
   git config --global url."git@github.com:".insteadOf "https://github.com/"
   ```

Tidak ada token atau secret tambahan. Repository produk yang dihasilkan berdiri
sendiri: CI-nya tidak pernah menarik kit, hanya pustaka publik
(`gonsu-one-sdk-go`, `gonsu-appkit-go`).

Bila `gonsu new` gagal mengambil kit, pesannya menyebut jawaban git.

## Yang didapat dari kit `go-nextjs`

- login GONSU, Pengguna & Akses, hak pakai, banner lisensi, dan tautan ke
  Portal GONSU;
- modul standar GONSU dari pustaka
  [`gonsu-appkit-go`](https://github.com/gonsutrijayautama/gonsu-appkit-go):
  profil bisnis (nama, kontak, NPWP, alamat, logo) dan wilayah Indonesia
  sampai desa — sama di setiap produk, di-upgrade lewat `go get`;
- modul contoh Catatan, dari tabel sampai layar, dengan uji peramban;
- `Dockerfile`, CI setiap PR, dan pipeline rilis ke GONSU saat tag `v*`
  didorong;
- `AGENTS.md` — peta project, resep menambah modul, dan aturan yang tidak
  boleh dilanggar — sehingga agen AI langsung bekerja mengikuti pola produk
  GONSU, serta panduan UI yang ditegakkan `make lint`.

Project hasil mencatat kit asalnya di `.gonsu/kit.json` (kit dan commit-nya).
Saat ingin mengikuti perubahan kit, selisih antara commit itu dan `main`
repository kit adalah catatan naik versinya.

## Pengembangan

```sh
go test ./...                 # gonsu sendiri, tanpa jaringan
go run ./cmd/gonsu new contoh # mencoba di terminal; butuh akses ke kit
```

**Merilis gonsu:** dorong tag `v*` (`git tag v0.2.0 && git push origin v0.2.0`).
`release.yml` menjalankan test, membangun binary untuk Linux, macOS, dan
Windows (`scripts/release.sh`), lalu menerbitkannya sebagai GitHub Release
beserta `SHA256SUMS` dan bukti asal. Installer selalu mengambil rilis terbaru.

**Mencoba installer tanpa rilis:**

```sh
scripts/release.sh v0.0.0-uji
GONSU_DOWNLOAD_BASE="$PWD/dist" GONSU_INSTALL_DIR=/tmp/gonsu-uji sh install.sh
```

**Mengubah isi project hasil dikerjakan di repository kit**, bukan di sini:
kit adalah aplikasi yang dapat dijalankan, jadi perubahan dicoba langsung di
sana (`make run`, `make test`, `make e2e`). Aturannya ada di `KIT.md` kit itu.

**Mencoba perubahan kit sebelum digabung ke `main`-nya:**

```sh
gonsu new uji-kit --kit-source ../gonsu-starter-go-nextjs   # working tree lokal
gonsu new uji-kit --version nama-cabang                     # cabang kit yang sudah didorong
```

**Tanpa `--version`, `gonsu new` mengambil `main` kit.** gonsu tidak perlu
dirilis ulang saat kit berubah — dan karena itu `main` kit harus selalu siap
dipakai: perubahan masuk lewat PR dengan CI hijau.

**`--version 0.1.1` mengambil tag `kit-v0.1.1`** di repository kit. Tag kit
berawalan `kit-` karena tag `v*` di sana memicu pipeline rilis produk; pemakai
cukup menulis nomornya (`0.1.1`, `v0.1.1`, dan `kit-v0.1.1` sama saja).

Aturan repository ini ada di [AGENTS.md](AGENTS.md).

## Lisensi

Proprietary; lihat [LICENSE](LICENSE). Starter kit, SDK GONSU One
([gonsu-one-sdk-go](https://github.com/gonsutrijayautama/gonsu-one-sdk-go)),
dan `gonsu-appkit-go` berlisensi sendiri; baca lisensinya di repository
masing-masing.
