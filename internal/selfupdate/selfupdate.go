// Package selfupdate memeriksa rilis gonsu terbaru dan mengganti binary yang
// sedang berjalan dengannya.
//
// Jalurnya sama dengan install.sh dan install.ps1: berkas rilis untuk sistem
// ini diunduh dari GitHub Releases, SHA-256-nya dicocokkan dengan SHA256SUMS
// rilis yang sama, dan baru sesudah itu dipasang. Nama berkas rilis ditentukan
// scripts/release.sh; mengubahnya berarti mengubah package ini dan kedua
// installer bersamaan.
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
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultBase adalah alamat rilis gonsu.
const DefaultBase = "https://github.com/gonsutrijayautama/gonsu-cli/releases"

const (
	// maxArchive membatasi unduhan: berkas rilis gonsu beberapa megabyte.
	maxArchive = 64 << 20
	maxSums    = 1 << 20
	maxBinary  = 128 << 20
)

// Client berbicara dengan halaman rilis gonsu.
type Client struct {
	// Base adalah alamat rilis. Kosong: DefaultBase.
	Base string
	// HTTP kosong: klien bawaan dengan batas waktu.
	HTTP *http.Client
	// GOOS dan GOARCH kosong: sistem yang menjalankan gonsu ini.
	GOOS, GOARCH string
	// Executable adalah binary yang diganti. Kosong: binary yang sedang
	// berjalan.
	Executable string
	// Verify memeriksa binary baru SEBELUM menggantikan yang lama. Nil:
	// menjalankannya dengan `version` dan mencocokkan jawabannya.
	Verify func(ctx context.Context, binary, version string) error
}

func (c Client) base() string {
	if c.Base != "" {
		return strings.TrimRight(c.Base, "/")
	}
	return DefaultBase
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func (c Client) platform() (goos, goarch string) {
	goos, goarch = c.GOOS, c.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

// Latest mengembalikan versi rilis terbaru, misalnya "v0.2.0".
//
// Dibaca dari pengalihan `<Base>/latest` ke halaman tag-nya, bukan dari API
// GitHub: API tanpa token dibatasi 60 permintaan per jam per alamat IP, dan
// kantor yang berbagi satu IP akan kehabisan.
func (c Client) Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.base()+"/latest", nil)
	if err != nil {
		return "", err
	}
	noRedirect := *c.http()
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", fmt.Errorf("menghubungi %s: %w", c.base(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return "", fmt.Errorf("%s/latest menjawab %s", c.base(), resp.Status)
	}
	// Location: <Base>/tag/v0.2.0. Repository tanpa rilis dialihkan ke
	// daftar rilisnya, tanpa /tag/.
	_, version, found := strings.Cut(resp.Header.Get("Location"), "/tag/")
	if !found || version == "" || strings.ContainsAny(version, "/?#") {
		return "", errors.New("belum ada rilis gonsu yang terbit")
	}
	return version, nil
}

// Asset adalah nama berkas rilis untuk sistem ini, sama dengan yang ditulis
// scripts/release.sh.
func (c Client) Asset() (string, error) {
	goos, goarch := c.platform()
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
		return "gonsu_" + goos + "_" + goarch + ".tar.gz", nil
	case "windows/amd64", "windows/arm64":
		return "gonsu_" + goos + "_" + goarch + ".zip", nil
	}
	return "", fmt.Errorf("gonsu tidak dirilis untuk %s/%s", goos, goarch)
}

// Install mengunduh rilis version, mencocokkan SHA-256-nya, lalu mengganti
// binary gonsu dengannya. Binary lama tidak disentuh sampai yang baru lolos
// seluruh pemeriksaan.
func (c Client) Install(ctx context.Context, version string) error {
	asset, err := c.Asset()
	if err != nil {
		return err
	}
	exe, err := c.executable()
	if err != nil {
		return err
	}

	from := c.base() + "/download/" + version
	archive, err := c.get(ctx, from+"/"+asset, maxArchive)
	if err != nil {
		return err
	}
	sums, err := c.get(ctx, from+"/SHA256SUMS", maxSums)
	if err != nil {
		return err
	}
	// Unduhan yang rusak atau tertukar tidak pernah dipasang.
	want, ok := checksumFor(sums, asset)
	if !ok {
		return fmt.Errorf("SHA256SUMS rilis %s tidak memuat %s", version, asset)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("SHA-256 %s tidak cocok dengan SHA256SUMS; unduhan tidak dipasang", asset)
	}

	goos, _ := c.platform()
	name := "gonsu"
	if goos == "windows" {
		name = "gonsu.exe"
	}
	binary, err := extract(archive, asset, name)
	if err != nil {
		return err
	}
	return c.replace(ctx, exe, binary, version, goos)
}

func (c Client) executable() (string, error) {
	exe := c.Executable
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return "", fmt.Errorf("mencari binary gonsu: %w", err)
		}
	}
	// gonsu yang dipanggil lewat symlink: yang diganti berkas aslinya.
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("mencari binary gonsu: %w", err)
	}
	return resolved, nil
}

func (c Client) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("mengunduh %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mengunduh %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("mengunduh %s: %w", url, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s lebih besar daripada yang wajar; unduhan dihentikan", url)
	}
	return data, nil
}

// checksumFor mencari SHA-256 name di isi SHA256SUMS. Barisnya
// "<hash>  <nama>", atau "<hash> *<nama>" bila dibuat di Windows.
func checksumFor(sums []byte, name string) (string, bool) {
	for line := range strings.Lines(string(sums)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// extract mengambil berkas name dari arsip rilis.
func extract(archive []byte, asset, name string) ([]byte, error) {
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("membuka %s: %w", asset, err)
		}
		for _, f := range zr.File {
			if path.Base(f.Name) != name || f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("membuka %s: %w", asset, err)
			}
			defer rc.Close()
			return readBinary(rc, asset)
		}
		return nil, fmt.Errorf("%s tidak memuat %s", asset, name)
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("membuka %s: %w", asset, err)
	}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s tidak memuat %s", asset, name)
		}
		if err != nil {
			return nil, fmt.Errorf("membuka %s: %w", asset, err)
		}
		if header.Typeflag == tar.TypeReg && path.Base(header.Name) == name {
			return readBinary(tr, asset)
		}
	}
}

func readBinary(r io.Reader, asset string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBinary+1))
	if err != nil {
		return nil, fmt.Errorf("membuka %s: %w", asset, err)
	}
	if len(data) > maxBinary {
		return nil, fmt.Errorf("isi %s lebih besar daripada yang wajar", asset)
	}
	return data, nil
}

// replace menulis binary baru di samping yang lama, memeriksanya, lalu
// menukarnya.
func (c Client) replace(ctx context.Context, exe string, binary []byte, version, goos string) error {
	dir := filepath.Dir(exe)
	// Di folder yang sama: penukaran lewat rename hanya atomik di dalam satu
	// filesystem. Akhiran .exe supaya Windows mau menjalankannya saat diperiksa.
	pattern := ".gonsu-baru-*"
	if goos == "windows" {
		pattern += ".exe"
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return fmt.Errorf("folder %s tidak dapat ditulis: %w\nPasang ulang lewat installer, atau jalankan gonsu update dengan hak tulis ke folder itu.", dir, err)
	}
	staged := tmp.Name()
	defer os.Remove(staged) // sudah tidak ada bila penukaran berhasil
	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return err
	}

	verify := c.Verify
	if verify == nil {
		verify = runsAs
	}
	if err := verify(ctx, staged, version); err != nil {
		return fmt.Errorf("binary %s tidak lolos pemeriksaan dan tidak dipasang: %w", version, err)
	}

	if goos != "windows" {
		// gonsu yang sedang berjalan tetap memegang berkas lamanya.
		return os.Rename(staged, exe)
	}
	// Windows tidak mengizinkan berkas yang sedang berjalan ditimpa, tetapi
	// mengizinkannya diganti nama. Yang lama disingkirkan dulu, dan dihapus
	// pada pembaruan berikutnya.
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("menyingkirkan gonsu lama: %w", err)
	}
	if err := os.Rename(staged, exe); err != nil {
		_ = os.Rename(old, exe) // kembalikan: jangan tinggalkan pemakai tanpa gonsu
		return fmt.Errorf("memasang gonsu baru: %w", err)
	}
	return nil
}

// runsAs menjalankan binary dan memastikan ia memperkenalkan diri sebagai
// gonsu versi version.
func runsAs(ctx context.Context, binary, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "version").Output()
	if err != nil {
		return fmt.Errorf("tidak dapat dijalankan: %w", err)
	}
	if got := strings.TrimSpace(string(out)); got != "gonsu "+version {
		return fmt.Errorf("memperkenalkan diri sebagai %q, bukan gonsu %s", got, version)
	}
	return nil
}
