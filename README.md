# gonsu — CLI untuk membuat produk GONSU One

**Untuk tim yang membangun produk yang dijual di GONSU One.**

Satu perintah, dan kamu dapat project yang sejak menit pertama sudah bisa login
lewat GONSU, membaca hak pakai paketnya, memberi akses login ke karyawan
pelanggan, dan dirilis ke GONSU. Kamu tinggal menulis bisnis aplikasinya.

```sh
gonsu new toko-baju
```

`gonsu` tidak membawa template. Ia **menarik starter kit** — repository terpisah
yang isinya aplikasi sungguhan dan bisa langsung dijalankan — lalu mengganti
identitas contohnya dengan identitas produkmu dan menyiapkan git serta
dependency.

## Memasang

```sh
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.ps1 | iex
```

Skrip ini mengunduh binary rilis terbaru, mencocokkan SHA-256-nya dengan
`SHA256SUMS` rilis itu, lalu menaruhnya di `~/.local/bin`. Di Windows tempatnya
`%LOCALAPPDATA%\Programs\gonsu`, dan folder itu ditambahkan ke PATH pengguna.
Tidak perlu sudo atau hak administrator.

| Variabel | Arti |
|---|---|
| `GONSU_VERSION` | versi yang dipasang, misalnya `0.1.0`. Kosong berarti rilis terbaru |
| `GONSU_INSTALL_DIR` | folder tujuan |

Kalau sudah punya Go, bisa juga:
`go install github.com/gonsutrijayautama/gonsu-cli/cmd/gonsu@latest`.

`gonsu new` menarik starter kit dengan git, jadi git harus terpasang. Kit yang
publik langsung bisa ditarik.

### Kalau kit-nya privat

Cukup sekali per laptop:

1. **Minta akses.** Akun GitHub kamu diundang ke repository kit itu.
2. **Pastikan git bisa masuk ke GitHub.** Pilih salah satu:

   ```sh
   gh auth login && gh auth setup-git          # lewat GitHub CLI
   ```

   ```sh
   # atau lewat SSH key yang sudah terdaftar di akun GitHub kamu
   git config --global url."git@github.com:".insteadOf "https://github.com/"
   ```

Tidak ada token atau secret tambahan. Kalau `gonsu new` gagal mengambil kit,
pesannya menyertakan jawaban dari git.

Repository produk yang dihasilkan berdiri sendiri: CI-nya tidak pernah menarik
kit, hanya pustaka publik (`gonsu-one-sdk-go`, `gonsu-appkit-go`).

### Memperbarui gonsu

Biasanya kamu tidak perlu melakukan apa-apa: di terminal, `gonsu new` memasang
rilis terbaru dulu kalau ada, lalu melanjutkan dengan versi baru itu.

```sh
gonsu -v               # versi yang terpasang
gonsu update --check   # cuma cek, ada rilis baru atau tidak
gonsu update           # pasang rilis terbaru
```

Pembaruan lewat `gonsu new` dan lewat `gonsu update` memakai jalur yang sama:
rilis terbaru untuk sistem kamu diunduh, SHA-256-nya dicocokkan dengan
`SHA256SUMS` seperti installer, lalu binary barunya dijalankan sekali untuk
memastikan ia utuh. Baru sesudah itu `gonsu` yang terpasang diganti. Kalau
salah satu langkah gagal, `gonsu` yang lama tidak disentuh.

Pembaruan lewat `gonsu new` tidak pernah menggagalkan `gonsu new`:

- Kalau rilisnya tidak terjangkau atau foldernya tidak bisa ditulis, project
  tetap dibuat dengan versi yang terpasang, dan kamu diberi tahu sebabnya.
- Di skrip dan CI (bukan terminal), gonsu tidak mengganti dirinya.
- Untuk melewatinya di terminal, pakai `--update=false`. gonsu tetap memberi
  tahu kalau ada rilis baru.

Pembaruan otomatis ada sejak v0.3.0, dan `gonsu update` sejak v0.2.0. Dari
versi yang lebih lama, jalankan `gonsu update` sekali, atau perintah pemasangan
di atas kalau `gonsu update` belum ada.

## Membuat project

```sh
gonsu new toko-baju                    # kit terbaru
gonsu new toko-baju --version 0.1.1    # versi kit tertentu
```

Di terminal, `gonsu new` menanyakan yang belum kamu isi lewat flag: nama
tampilan, starter kit, module path Go, perlu repository git atau tidak, dan
dependency langsung dipasang atau tidak.

Untuk skrip dan CI, tambahkan `-n` supaya tidak ada pertanyaan:

```sh
gonsu new toko --kit go-nextjs --module github.com/organisasi/toko -n
```

Daftar flag lengkapnya: `gonsu new -h`.

### Kode produk

**Kode produk harus sama persis dengan kode produk di Console GONSU**: huruf
kecil, angka, dan tanda hubung di antaranya, 2–60 karakter. Console hanya bisa
dibuka staf platform, jadi minta tim platform GONSU mendaftarkan produknya.

### Starter kit

| Kit | Repository | Isi |
|---|---|---|
| `go-nextjs` | `gonsu-starter-go-nextjs` | backend Go, frontend Next.js |
| `nextjs` *(belum tersedia)* | `gonsu-starter-nextjs` | menunggu SDK GONSU untuk JavaScript |
| `laravel` *(belum tersedia)* | `gonsu-starter-laravel` | menunggu SDK GONSU untuk PHP |

Kit yang belum tersedia tetap muncul di pilihan beserta alasannya, tapi belum
bisa dipilih.

### Versi kit

Tanpa `--version`, yang diambil selalu `main` kit yang terbaru. Jadi perubahan
kit langsung terpakai, tanpa perlu memperbarui `gonsu`.

`--version 0.1.1` mengambil tag `kit-v0.1.1` di repository kit. Cukup tulis
nomornya: `0.1.1`, `v0.1.1`, dan `kit-v0.1.1` sama saja. Tag kit berawalan
`kit-` karena tag `v*` di sana memicu pipeline rilis produk.

Project hasil mencatat kit asalnya di `.gonsu/kit.json`: nama kit dan
commit-nya. Kalau nanti mau ikut perubahan kit, bandingkan commit itu dengan
`main` kit. Selisihnya adalah catatan naik versinya.

## Yang didapat dari kit `go-nextjs`

- login GONSU, Pengguna & Akses, hak pakai, banner lisensi, dan tautan ke
  Portal GONSU;
- modul standar GONSU dari pustaka
  [`gonsu-appkit-go`](https://github.com/gonsutrijayautama/gonsu-appkit-go):
  profil bisnis (nama, kontak, NPWP, alamat, logo), halaman depan publik,
  media, dan wilayah Indonesia sampai desa. Modulnya sama di setiap produk dan
  di-upgrade lewat `go get`;
- modul contoh Catatan, dari tabel sampai layar, lengkap dengan uji peramban;
- `Dockerfile`, CI di setiap PR, dan pipeline rilis ke GONSU saat tag `v*`
  didorong;
- `AGENTS.md` berisi peta project, resep menambah modul, dan aturan yang tidak
  boleh dilanggar, jadi agen AI langsung bekerja mengikuti pola produk GONSU.
  Panduan UI-nya ditegakkan `make lint`.

## Keamanan

**Yang dijalankan dari kit.** `gonsu new` hanya menjalankan perintah `install`
yang tertulis di manifes kit (`gonsu.kit.json`), misalnya `go mod download` dan
`bun install`. Tiap perintah dicetak sebelum dijalankan, dan semuanya bisa
dilewati dengan `--install=false`. Di luar itu gonsu tidak menjalankan apa pun
dari kit, dan tidak menghapus atau menulis file di luar folder project — juga
kalau kit-nya membawa symlink.

Kit di katalog adalah repository milik GONSU. Lain cerita kalau kamu memakai
`--kit-source` ke kit yang tidak kamu kenal: itu sama saja dengan menjalankan
perintah orang lain di laptopmu. Baca dulu `gonsu.kit.json`-nya, atau pakai
`--install=false`.

**Alamat kit yang membawa token** (`https://TOKEN@github.com/...`) tidak ikut
tercetak di pesan galat dan tidak dicatat di `.gonsu/kit.json`.

**Keaslian binary.** Installer dan `gonsu update` mencocokkan SHA-256 unduhan
dengan `SHA256SUMS` dari rilis yang sama. Itu menangkap unduhan yang rusak atau
tertukar. Untuk memastikan sebuah binary memang dibangun dari repository dan
tag ini, periksa attestation-nya:

```sh
gh attestation verify gonsu_linux_amd64.tar.gz --repo gonsutrijayautama/gonsu-cli
```

## Pengembangan

```sh
go test ./...                 # gonsu sendiri, tanpa jaringan
go run ./cmd/gonsu new contoh # coba di terminal; butuh akses ke kit
```

Aturan repository ini ada di [AGENTS.md](AGENTS.md).

**Merilis gonsu.** Dorong tag `v*`:

```sh
git tag v0.2.0 && git push origin v0.2.0
```

`release.yml` menjalankan test, membangun binary untuk Linux, macOS, dan
Windows (`scripts/release.sh`), lalu menerbitkannya sebagai GitHub Release
beserta `SHA256SUMS` dan attestation-nya. Installer selalu mengambil rilis
terbaru.

**Mencoba installer tanpa rilis.**

```sh
scripts/release.sh v0.0.0-uji
GONSU_DOWNLOAD_BASE="$PWD/dist" GONSU_INSTALL_DIR=/tmp/gonsu-uji sh install.sh
```

**Mengubah isi project hasil.** Itu dikerjakan di repository kit, bukan di
sini. Kit adalah aplikasi yang bisa dijalankan, jadi perubahannya dicoba
langsung di sana (`make run`, `make test`, `make e2e`). Aturannya ada di
`KIT.md` kit itu.

**Mencoba perubahan kit sebelum masuk ke `main`-nya.**

```sh
gonsu new uji-kit --kit-source ../gonsu-starter-go-nextjs   # working tree lokal
gonsu new uji-kit --version nama-cabang                     # cabang kit yang sudah didorong
```

Karena `gonsu new` selalu mengambil `main` kit, `main` kit harus selalu siap
dipakai: perubahan masuk lewat PR dengan CI hijau.

## Lisensi

Proprietary; lihat [LICENSE](LICENSE). Starter kit, SDK GONSU One
([gonsu-one-sdk-go](https://github.com/gonsutrijayautama/gonsu-one-sdk-go)),
dan `gonsu-appkit-go` punya lisensinya sendiri. Baca di repository
masing-masing.
