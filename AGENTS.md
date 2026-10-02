# AGENTS.md — gonsu-cli

`gonsu` membuat project produk GONSU One: `gonsu new <kode-produk>`. Ia hanya
**orkestrator** dan tidak membawa template. Isi project datang dari starter
kit, yaitu repository terpisah bernama `gonsu-starter-<kit>`, yang ditarik
`gonsu new` dari `main`-nya.

Repository ini PUBLIK. Kit boleh publik atau privat, dan gonsu tidak berasumsi
salah satunya.

## Peta

| Tempat | Isi |
|---|---|
| `cmd/gonsu` | `main`: menyambungkan `cli.Env` ke terminal, folder kerja, dan halaman rilis |
| `internal/cli` | perintah `new` dan `update`: flag, tanya-jawab, dan semua teks di layar |
| `internal/kit` | kontrak dengan kit: manifes (`manifest.go`), mengambil kit (`fetch.go`), mengubahnya jadi project (`apply.go`) |
| `internal/catalog` | daftar kit yang bisa dipilih |
| `internal/project` | aturan kode produk, nama tampilan, dan module path |
| `internal/selfupdate` | `gonsu update`: cek rilis, unduh, cocokkan SHA-256, ganti binary |
| `install.sh`, `install.ps1` | installer resmi |
| `scripts/release.sh` | membangun file rilis ke `dist/` |
| `.github/workflows` | `ci.yml` (test), `kit.yml` (`gonsu new` terhadap kit sungguhan), `install.yml` (installer di tiga sistem), `release.yml` (rilis saat tag `v*`) |

## Sebelum menyerahkan perubahan

```sh
gofmt -l .       # harus kosong
go vet ./...
go test ./...    # tanpa jaringan
```

Ketiganya dijalankan CI di setiap PR.

## Aturan

### Bahasa

- Identifier, nama file, dan flag dalam bahasa Inggris. Komentar, pesan CLI,
  dan dokumen dalam bahasa Indonesia.
- Teks di layar dan dokumen ditulis santai dan langsung, seperti menjelaskan
  ke rekan satu tim: sapa pembaca dengan "kamu", pakai kalimat pendek, dan
  satu gagasan per kalimat.

### Batas repository

- **Tidak ada isi project di sini.** Apa pun yang diterima tim produk — kode,
  CI, dokumen project — diubah di repository kit. gonsu-cli tidak kenal stack;
  yang ia tahu hanya manifes kit.
- **Tidak ada isi kit di sini.** Jangan menyalin kode, dokumen, atau workflow
  kit ke repository ini, termasuk sebagai contoh di test. Kit uji dibuat dari
  file rekaan. Alasannya: kit bisa dijadikan privat kapan saja, sedangkan
  repository ini dan log CI-nya tetap publik. Job yang menarik kit juga tidak
  mencetak isinya.
- **Tanpa rahasia dan tanpa nilai milik satu pemasangan.** Alamat server,
  domain, dan kredensial tidak ditulis di sini maupun di kit.

### Kontrak dengan kit: `gonsu.kit.json`

Manifes (`internal/kit`) berisi identitas contoh, file milik kit, perintah
pemasangan, dan langkah berikutnya.

- **Manifes hanya bertambah.** Isian yang tidak dikenal diabaikan, jadi kit
  boleh lebih baru daripada gonsu. Perubahan yang tidak kompatibel menaikkan
  `schema`, dan gonsu lama menolaknya dengan pesan "perbarui gonsu".
- **Identitas diganti sekali jalan** di setiap file teks (`strings.Replacer`).
  Jangan menggantinya berurutan: produk yang kodenya memuat nilai contoh akan
  terganti dua kali.
- **Nama tampilan ditulis ter-escape di file kode** (`isCode`), dan apa adanya
  di file lain. Karena itu nama tampilan harus satu baris, tanpa karakter
  kontrol.
- **Dari kit, gonsu hanya menjalankan perintah `install` di manifes**, dan
  hanya kalau pengguna tidak menolaknya (`--install=false`).
- **Kit tidak bisa keluar dari folder project.** Jalur di manifes diperiksa
  dengan `filepath.IsLocal`, dan semua yang dihapus atau ditulis atas
  permintaan kit lewat `os.Root` (`Apply`), supaya symlink bawaan kit tidak
  membawanya ke file pengguna.
- **Teks manifes yang dicetak ke terminal** (`label`, `install`,
  `next_steps`) tidak boleh memuat karakter kontrol.
- **Alamat kit dicetak dan dicatat tanpa kredensial** (`withoutCredentials`).
  Pesan galat masuk log CI, dan `.gonsu/kit.json` ikut commit pertama project.

### Katalog dan versi kit

- **Katalog** (`internal/catalog`): sebuah kit boleh `Available` hanya kalau
  repository-nya ada dan CI-nya hijau. Yang belum tetap dicantumkan beserta
  alasannya (`Pending`).
- **Tanpa `--version`, `gonsu new` mengambil `main` kit yang terbaru, tanpa
  pin.** Perubahan kit sampai ke pengguna tanpa rilis gonsu. Jangan menambahkan
  pin versi kit di sini. Yang membuat hasilnya bisa dilacak adalah commit kit
  yang dicatat di `.gonsu/kit.json` project hasil.
- **`--version 0.1.1` mengambil tag `kit-v0.1.1`** (`kitRef` di
  `internal/cli`). Tag di repository kit berawalan `kit-` karena tag `v*` di
  sana memicu pipeline rilis produk. Selain nomor versi, `--version` menerima
  nama cabang kit.

### `gonsu new`

- **Folder tujuan tidak pernah ditimpa.**
- **Tidak ada project setengah jadi.** Kalau kit gagal diambil atau
  diterapkan, yang sudah ditulis dibersihkan lagi.

### Test

- **Tanpa jaringan.** `go test ./...` tidak pernah menghubungi GitHub. Kit uji
  dibuat di folder sementara, pengambilan lewat git diuji terhadap repository
  lokal, dan `gonsu update` diuji terhadap halaman rilis tiruan di mesin itu
  sendiri.

### CI

- **Hanya runner GitHub**, tidak pernah self-hosted: PR dari fork menjalankan
  kodenya di runner yang dipakai.
- **Secret (`KIT_DEPLOY_KEY`) tidak tersedia untuk PR dari fork.** Job yang
  butuh secret itu dilewati. Secret diberikan ke langkah yang memakainya saja,
  bukan ke seluruh job.
- **Versi runner Ubuntu ditulis eksplisit** (`ubuntu-24.04`), bukan
  `ubuntu-latest`: label itu pindah ke Ubuntu baru mengikuti jadwal GitHub.
  Naik versi runner adalah PR tersendiri. macOS dan Windows memang memakai
  `-latest`, karena gonsu dan installernya berjalan di laptop pengguna.
- **Action dipasang pada commit SHA**, dengan versinya di komentar.

### Rilis dan pemasangan

- **Rilis dipicu tag `v*`** (`release.yml`): binary per sistem, `SHA256SUMS`,
  dan attestation lewat OIDC. Tidak ada kunci penandatanganan yang disimpan.
- **`install.sh` dan `install.ps1`** di akar repository adalah jalur pasang
  resmi. Keduanya WAJIB mencocokkan SHA-256 sebelum memasang, tidak butuh sudo
  atau hak administrator, dan diuji `install.yml` di Linux, macOS, dan Windows.
- **`gonsu update`** (`internal/selfupdate`) adalah jalur pasang ketiga dan
  ikut aturan yang sama: SHA-256 dicocokkan sebelum memasang, dan `gonsu` lama
  tidak disentuh sampai binary baru lolos semua pemeriksaan.
- **Versi terbaru dibaca dari redirect `releases/latest`**, bukan dari API
  GitHub, yang dibatasi 60 permintaan per jam per IP.
- **Pemeriksaan rilis di `gonsu new` tidak pernah menahan atau
  menggagalkannya.**
- **Nama file rilis dipakai di empat tempat.** Mengubahnya berarti mengubah
  `scripts/release.sh`, kedua installer, dan `internal/selfupdate` sekaligus.
- **`GONSU_RELEASES_URL` dan `GONSU_DOWNLOAD_BASE` hanya untuk test.**

### Commit, PR, dan dokumen

- **Tanpa atribusi AI** di pesan commit maupun deskripsi PR.
- **Dokumen ikut kode**, di PR yang sama: `README.md` untuk perilaku CLI dan
  cara memasangnya.
