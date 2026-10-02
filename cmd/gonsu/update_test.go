package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gonsutrijayautama/gonsu-cli/internal/selfupdate"
)

// build membangun gonsu bernomor version ke dir, seperti scripts/release.sh.
func build(t *testing.T, dir, version string) string {
	t.Helper()
	name := "gonsu"
	if runtime.GOOS == "windows" {
		name = "gonsu.exe"
	}
	out := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+version, "-o", out, ".")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, msg)
	}
	return out
}

// pack membungkus binary menjadi berkas rilis untuk sistem ini.
func pack(t *testing.T, binary string) (asset string, archive []byte) {
	t.Helper()
	asset, err := selfupdate.Client{}.Asset()
	if err != nil {
		t.Skipf("sistem ini tidak punya berkas rilis: %v", err)
	}
	content, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if strings.HasSuffix(asset, ".zip") {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(filepath.Base(binary))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return asset, buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "gonsu", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return asset, buf.Bytes()
}

// gonsu yang SUNGGUHAN memperbarui dirinya sendiri selagi berjalan, terhadap
// halaman rilis tiruan di mesin ini. Inilah yang berbeda antar sistem: di
// Windows berkas yang sedang berjalan tidak dapat ditimpa.
func TestUpdateReplacesRunningBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("membangun dua binary gonsu")
	}
	const oldVersion, newVersion = "v0.0.1", "v0.0.2"
	gonsu := build(t, t.TempDir(), oldVersion)
	asset, archive := pack(t, build(t, t.TempDir(), newVersion))
	sum := sha256.Sum256(archive)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			http.Redirect(w, r, "/tag/"+newVersion, http.StatusFound)
		case "/download/" + newVersion + "/" + asset:
			_, _ = w.Write(archive)
		case "/download/" + newVersion + "/SHA256SUMS":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + asset + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(gonsu, args...)
		cmd.Env = append(os.Environ(), "GONSU_RELEASES_URL="+srv.URL)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("gonsu %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	if out := run("version"); strings.TrimSpace(out) != "gonsu "+oldVersion {
		t.Fatalf("version sebelum pembaruan = %q", out)
	}
	// --check hanya memberi tahu.
	if out := run("update", "--check"); !strings.Contains(out, "gonsu "+newVersion+" tersedia") {
		t.Errorf("update --check = %q", out)
	}
	if out := run("version"); strings.TrimSpace(out) != "gonsu "+oldVersion {
		t.Fatalf("update --check mengganti binary: %q", out)
	}

	if out := run("update"); !strings.Contains(out, "gonsu "+newVersion+" terpasang") {
		t.Errorf("update = %q", out)
	}
	if out := run("version"); strings.TrimSpace(out) != "gonsu "+newVersion {
		t.Fatalf("version sesudah pembaruan = %q, ingin gonsu %s", out, newVersion)
	}
	// Yang baru saja dipasang tahu dirinya sudah terbaru.
	if out := run("update"); !strings.Contains(out, "sudah yang terbaru") {
		t.Errorf("update kedua = %q", out)
	}
}
