package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sumLine(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

// releases adalah halaman rilis tiruan: /latest mengalihkan ke tag terbaru,
// dan /download/<tag>/<berkas> menyajikan isi files.
func releases(t *testing.T, latest string, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			if latest == "" {
				// Repository tanpa rilis: dialihkan ke daftar rilisnya.
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/tag/"+latest, http.StatusFound)
			return
		}
		name, ok := strings.CutPrefix(r.URL.Path, "/download/"+latest+"/")
		if data, found := files[name]; ok && found {
			_, _ = w.Write(data)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// installed menulis "gonsu lama" dan mengembalikan path-nya.
func installed(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "gonsu")
	if err := os.WriteFile(exe, []byte("lama"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func accept(context.Context, string, string) error { return nil }

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// leftovers: berkas sementara tidak boleh tertinggal di folder pemasangan.
func leftovers(t *testing.T, exe string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(exe))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() != filepath.Base(exe) && e.Name() != filepath.Base(exe)+".old" {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestLatest(t *testing.T) {
	ctx := context.Background()

	srv := releases(t, "v1.4.0", nil)
	got, err := Client{Base: srv.URL}.Latest(ctx)
	if err != nil || got != "v1.4.0" {
		t.Errorf("Latest = %q, %v; ingin v1.4.0", got, err)
	}

	t.Run("belum ada rilis", func(t *testing.T) {
		srv := releases(t, "", nil)
		if got, err := (Client{Base: srv.URL}).Latest(ctx); err == nil {
			t.Errorf("Latest = %q, ingin galat", got)
		}
	})
	t.Run("bukan pengalihan", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		defer srv.Close()
		if got, err := (Client{Base: srv.URL}).Latest(ctx); err == nil {
			t.Errorf("Latest = %q, ingin galat", got)
		}
	})
	t.Run("server tidak terjangkau", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		if got, err := (Client{Base: srv.URL}).Latest(ctx); err == nil {
			t.Errorf("Latest = %q, ingin galat", got)
		}
	})
}

// Nama berkas harus sama dengan yang ditulis scripts/release.sh.
func TestAsset(t *testing.T) {
	for platform, want := range map[string]string{
		"linux/amd64":   "gonsu_linux_amd64.tar.gz",
		"linux/arm64":   "gonsu_linux_arm64.tar.gz",
		"darwin/amd64":  "gonsu_darwin_amd64.tar.gz",
		"darwin/arm64":  "gonsu_darwin_arm64.tar.gz",
		"windows/amd64": "gonsu_windows_amd64.zip",
		"windows/arm64": "gonsu_windows_arm64.zip",
	} {
		goos, goarch, _ := strings.Cut(platform, "/")
		got, err := Client{GOOS: goos, GOARCH: goarch}.Asset()
		if err != nil || got != want {
			t.Errorf("Asset(%s) = %q, %v; ingin %q", platform, got, err, want)
		}
	}
	if got, err := (Client{GOOS: "plan9", GOARCH: "amd64"}).Asset(); err == nil {
		t.Errorf("Asset(plan9) = %q, ingin galat", got)
	}
	// Tanpa isian: sistem yang menjalankan test ini.
	if got, err := (Client{}).Asset(); err == nil && !strings.Contains(got, runtime.GOOS+"_"+runtime.GOARCH) {
		t.Errorf("Asset() = %q, ingin untuk %s/%s", got, runtime.GOOS, runtime.GOARCH)
	}
}

func TestInstall(t *testing.T) {
	const asset = "gonsu_linux_amd64.tar.gz"
	archive := tarGz(t, map[string]string{"gonsu": "baru", "LICENSE": "lisensi"})
	srv := releases(t, "v1.4.0", map[string][]byte{
		asset:        archive,
		"SHA256SUMS": []byte(sumLine([]byte("lain"), "gonsu_darwin_arm64.tar.gz") + sumLine(archive, asset)),
	})
	exe := installed(t)

	var verified []string
	c := Client{Base: srv.URL, GOOS: "linux", GOARCH: "amd64", Executable: exe,
		Verify: func(_ context.Context, binary, version string) error {
			// Yang diperiksa adalah binary BARU, sebelum yang lama diganti.
			verified = append(verified, read(t, binary)+" "+version+" lama="+read(t, exe))
			return nil
		}}
	if err := c.Install(context.Background(), "v1.4.0"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := read(t, exe); got != "baru" {
		t.Errorf("isi gonsu = %q, ingin binary baru", got)
	}
	if len(verified) != 1 || verified[0] != "baru v1.4.0 lama=lama" {
		t.Errorf("pemeriksaan = %v", verified)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(exe)
		if err != nil || info.Mode().Perm() != 0o755 {
			t.Errorf("izin gonsu = %v, %v; ingin 0755", info.Mode().Perm(), err)
		}
	}
	if extra := leftovers(t, exe); len(extra) > 0 {
		t.Errorf("berkas tertinggal di folder pemasangan: %v", extra)
	}
}

// Di Windows binary yang sedang berjalan tidak dapat ditimpa: yang lama
// disingkirkan dengan ganti nama, yang baru menempati namanya.
func TestInstallOnWindows(t *testing.T) {
	const asset = "gonsu_windows_amd64.zip"
	archive := zipOf(t, map[string]string{"gonsu.exe": "baru", "LICENSE": "lisensi"})
	// SHA256SUMS buatan Windows menandai nama berkas dengan bintang.
	sum := sha256.Sum256(archive)
	srv := releases(t, "v1.4.0", map[string][]byte{
		asset:        archive,
		"SHA256SUMS": []byte(strings.ToUpper(hex.EncodeToString(sum[:])) + " *" + asset + "\r\n"),
	})
	exe := installed(t)
	c := Client{Base: srv.URL, GOOS: "windows", GOARCH: "amd64", Executable: exe, Verify: accept}

	for round, want := range []string{"lama", "baru"} {
		if err := c.Install(context.Background(), "v1.4.0"); err != nil {
			t.Fatalf("Install ke-%d: %v", round+1, err)
		}
		if got := read(t, exe); got != "baru" {
			t.Errorf("isi gonsu = %q, ingin binary baru", got)
		}
		// Pembaruan berikutnya membuang .old yang lama lebih dulu.
		if got := read(t, exe+".old"); got != want {
			t.Errorf("isi gonsu.old sesudah pembaruan ke-%d = %q, ingin %q", round+1, got, want)
		}
		if extra := leftovers(t, exe); len(extra) > 0 {
			t.Errorf("berkas tertinggal di folder pemasangan: %v", extra)
		}
	}
}

// Apa pun yang gagal sebelum penukaran, gonsu lama tidak tersentuh dan tidak
// ada berkas sementara yang tertinggal.
func TestInstallLeavesOldBinaryOnFailure(t *testing.T) {
	const asset = "gonsu_linux_amd64.tar.gz"
	good := tarGz(t, map[string]string{"gonsu": "baru"})
	rejected := errors.New("bukan gonsu")

	tests := []struct {
		name   string
		files  map[string][]byte
		verify func(context.Context, string, string) error
		want   string
	}{
		{"SHA-256 tidak cocok",
			map[string][]byte{asset: good, "SHA256SUMS": []byte(sumLine([]byte("isi lain"), asset))},
			accept, "tidak cocok"},
		{"SHA256SUMS tidak memuat berkasnya",
			map[string][]byte{asset: good, "SHA256SUMS": []byte(sumLine(good, "gonsu_darwin_arm64.tar.gz"))},
			accept, "tidak memuat " + asset},
		{"SHA256SUMS tidak ada",
			map[string][]byte{asset: good},
			accept, "SHA256SUMS"},
		{"berkas rilis tidak ada",
			map[string][]byte{"SHA256SUMS": []byte(sumLine(good, asset))},
			accept, asset},
		{"arsip tanpa binary gonsu",
			func() map[string][]byte {
				empty := tarGz(t, map[string]string{"LICENSE": "lisensi"})
				return map[string][]byte{asset: empty, "SHA256SUMS": []byte(sumLine(empty, asset))}
			}(),
			accept, "tidak memuat gonsu"},
		{"arsip rusak",
			map[string][]byte{asset: []byte("bukan arsip"), "SHA256SUMS": []byte(sumLine([]byte("bukan arsip"), asset))},
			accept, "membuka"},
		{"binary baru tidak lolos pemeriksaan",
			map[string][]byte{asset: good, "SHA256SUMS": []byte(sumLine(good, asset))},
			func(context.Context, string, string) error { return rejected }, "tidak lolos pemeriksaan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := releases(t, "v1.4.0", tt.files)
			exe := installed(t)
			c := Client{Base: srv.URL, GOOS: "linux", GOARCH: "amd64", Executable: exe, Verify: tt.verify}
			err := c.Install(context.Background(), "v1.4.0")
			if err == nil {
				t.Fatal("Install lolos")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("galat = %q, ingin memuat %q", err, tt.want)
			}
			if got := read(t, exe); got != "lama" {
				t.Errorf("gonsu lama berubah menjadi %q", got)
			}
			if extra := leftovers(t, exe); len(extra) > 0 {
				t.Errorf("berkas tertinggal di folder pemasangan: %v", extra)
			}
		})
	}
}

// gonsu yang dipanggil lewat symlink: yang diganti berkas aslinya, dan
// symlink-nya tetap symlink.
func TestInstallFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink butuh hak khusus di Windows")
	}
	const asset = "gonsu_linux_amd64.tar.gz"
	archive := tarGz(t, map[string]string{"gonsu": "baru"})
	srv := releases(t, "v1.4.0", map[string][]byte{asset: archive, "SHA256SUMS": []byte(sumLine(archive, asset))})
	exe := installed(t)
	link := filepath.Join(t.TempDir(), "gonsu")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}

	c := Client{Base: srv.URL, GOOS: "linux", GOARCH: "amd64", Executable: link, Verify: accept}
	if err := c.Install(context.Background(), "v1.4.0"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := read(t, exe); got != "baru" {
		t.Errorf("isi berkas asli = %q, ingin binary baru", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("symlink berubah menjadi berkas biasa: %v, %v", info, err)
	}
}

// Folder pemasangan yang tidak dapat ditulis dijelaskan, bukan sekadar
// "permission denied".
func TestInstallExplainsUnwritableDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("butuh folder hanya-baca yang ditegakkan sistem")
	}
	const asset = "gonsu_linux_amd64.tar.gz"
	archive := tarGz(t, map[string]string{"gonsu": "baru"})
	srv := releases(t, "v1.4.0", map[string][]byte{asset: archive, "SHA256SUMS": []byte(sumLine(archive, asset))})
	exe := installed(t)
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	c := Client{Base: srv.URL, GOOS: "linux", GOARCH: "amd64", Executable: exe, Verify: accept}
	err := c.Install(context.Background(), "v1.4.0")
	if err == nil || !strings.Contains(err.Error(), "tidak dapat ditulis") || !strings.Contains(err.Error(), "installer") {
		t.Errorf("galat = %v, ingin menjelaskan folder yang tidak dapat ditulis", err)
	}
	if got := read(t, exe); got != "lama" {
		t.Errorf("gonsu lama berubah menjadi %q", got)
	}
}
