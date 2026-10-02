# AGENTS.md — gonsu-cli

`gonsu` membuat project produk GONSU One: `gonsu new <kode-produk>`. Ia
**orkestrator**: tidak membawa template. Isi project datang dari starter kit,
repository tersendiri bernama `gonsu-starter-<kit>`, yang ditarik `gonsu new`
dari `main`-nya. Repository ini PUBLIK; starter kit-nya privat.

## Aturan

- **Bahasa.** Identifier, nama berkas, dan flag dalam bahasa Inggris. Komentar,
  pesan CLI, dan dokumen dalam bahasa Indonesia — kalimat di layar santai dan
  langsung, bukan kaku.
- **Tidak ada isi project di repository ini.** Perubahan pada apa yang diterima
  tim produk — kode, CI, dokumen project — dikerjakan di repository kit.
  gonsu-cli tidak mengenal stack: yang ia tahu hanya manifes kit.
- **Kontrak dengan kit adalah `gonsu.kit.json`** (`internal/kit`): identitas
  contoh, berkas milik kit, perintah pemasangan, dan langkah berikutnya.
  - Isian baru di manifes hanya MENAMBAH. Isian yang tidak dikenal diabaikan,
    sehingga kit boleh lebih baru daripada gonsu. Perubahan yang memutus
    menaikkan `schema`, dan gonsu lama menolaknya dengan pesan "perbarui
    gonsu".
  - Identitas diganti di setiap berkas teks dalam SATU lintasan
    (`strings.Replacer`). Jangan menggantinya berurutan: produk yang kodenya
    memuat nilai contoh akan terganti dua kali.
  - Nama tampilan ditulis ter-escape di berkas kode (`isCode`), apa adanya di
    berkas lain.
  - gonsu tidak menjalankan apa pun dari kit selain perintah `install` di
    manifes, dan hanya bila pemakai tidak menolaknya (`--install=false`).
- **Katalog** (`internal/catalog`): satu kit boleh `Available` hanya bila
  repository-nya ada dan CI-nya hijau. Yang belum, tetap tercantum dengan
  alasannya (`Pending`).
- **Tanpa `--version`, `gonsu new` mengambil ujung `main` kit, tanpa pin.**
  Perubahan kit sampai ke pemakai tanpa rilis gonsu. Jangan menambahkan pin
  versi kit di sini; yang membuat hasilnya dapat dilacak adalah commit kit
  yang dicatat di `.gonsu/kit.json` project hasil.
- **`--version 0.1.1` mengambil tag `kit-v0.1.1`** (`kitRef` di
  `internal/cli`). Tag di repository kit berawalan `kit-`: tag `v*` di sana
  memicu pipeline rilis produk. Selain nomor versi, `--version` menerima nama
  cabang kit.
- **Folder tujuan tidak pernah ditimpa**, dan kit yang gagal diambil atau
  diterapkan tidak meninggalkan project setengah jadi.
- **Test tanpa jaringan.** `go test ./...` tidak pernah menghubungi GitHub: kit
  uji dibuat di folder sementara, dan pengambilan lewat git diuji terhadap
  repository lokal.
- **Repository ini publik: tidak ada isi kit di sini.** Jangan menyalin kode,
  dokumen, atau workflow kit ke repository ini — termasuk sebagai contoh di
  test (kit uji dibuat dari berkas rekaan). Log CI pun publik: job yang
  menarik kit tidak boleh mencetak isinya.
- **Tanpa rahasia dan tanpa nilai milik satu pemasangan.** Alamat server,
  domain, dan kredensial tidak ditulis di sini maupun di kit.
- **CI hanya memakai runner GitHub**, tidak pernah self-hosted — PR dari fork
  menjalankan kode di runner yang dipakainya. Secret (`KIT_DEPLOY_KEY`) tidak
  tersedia bagi PR dari fork; job yang membutuhkannya dilewati.
- **Rilis** dipicu tag `v*` (`release.yml`): binary per sistem, `SHA256SUMS`,
  dan bukti asal lewat OIDC — tidak ada kunci penandatanganan yang disimpan.
  `install.sh` dan `install.ps1` di akar repository adalah jalur pasang resmi;
  keduanya WAJIB mencocokkan SHA-256 sebelum memasang, tidak butuh sudo atau
  hak administrator, dan diuji `install.yml` di Linux, macOS, dan Windows.
  Mengubah nama berkas rilis berarti mengubah `scripts/release.sh` dan kedua
  installer bersamaan.
- **Tanpa atribusi AI** di pesan commit maupun deskripsi PR.
- **Dokumen ikut kode**, di PR yang sama: `README.md` untuk perilaku CLI dan
  cara memasangnya.
