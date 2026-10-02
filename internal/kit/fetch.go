package kit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Origin mencatat dari mana sebuah project berasal.
type Origin struct {
	// Source adalah alamat git kit, atau "local" untuk kit dari folder.
	Source string `json:"source"`
	// Version adalah tag atau cabang yang diminta; kosong berarti ujung cabang
	// bawaan kit.
	Version string `json:"version,omitempty"`
	// Commit adalah commit kit yang disalin — inilah yang menentukan isi
	// project, apa pun Version-nya. Kosong bila tidak diketahui.
	Commit string `json:"commit,omitempty"`
}

// LocalSource adalah Origin.Source untuk kit yang disalin dari folder. Path
// folder itu sendiri tidak dicatat: ia path laptop seseorang.
const LocalSource = "local"

// Fetch menyalin kit ke dir, yang belum ada atau kosong.
//
// source berupa folder disalin apa adanya dari working tree — untuk perawat
// kit yang mencoba perubahannya sebelum menggabungnya. Selain itu source
// adalah alamat git, dan yang diambil adalah tag atau cabang version; version
// kosong berarti ujung cabang bawaan kit.
func Fetch(ctx context.Context, source, version, dir string) (Origin, error) {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		commit, err := copyTree(ctx, source, dir)
		if err != nil {
			return Origin{}, fmt.Errorf("menyalin kit dari %s: %w", source, err)
		}
		return Origin{Source: LocalSource, Commit: commit}, nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Origin{}, errors.New("git tidak ditemukan — gonsu mengambil starter kit dengan git")
	}

	args := []string{"clone", "--quiet", "--depth", "1", "-c", "advice.detachedHead=false"}
	if version != "" {
		args = append(args, "--branch", version)
	}
	args = append(args, "--", source, dir)
	if out, err := git(ctx, "", args...); err != nil {
		// Kit-nya terjangkau, hanya tag atau cabangnya yang tidak ada: petunjuk
		// soal akses di bawah akan menyesatkan.
		if version != "" && bytes.Contains(out, []byte("not found in upstream")) {
			return Origin{}, fmt.Errorf("versi %s tidak ada di kit %s", version, source)
		}
		return Origin{}, fmt.Errorf("mengambil kit %s gagal: %s\n"+
			"Starter kit GONSU privat. Pastikan akun GitHub Anda diberi akses ke repository-nya dan git dapat masuk:\n"+
			"  gh auth login && gh auth setup-git\n"+
			"atau, bila memakai SSH key:\n"+
			"  git config --global url.\"git@github.com:\".insteadOf \"https://github.com/\"",
			describe(source, version), firstLines(out, 3))
	}
	commit, err := git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return Origin{}, fmt.Errorf("membaca commit kit: %w", err)
	}
	// Riwayat kit bukan riwayat produk.
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		return Origin{}, err
	}
	return Origin{Source: source, Version: version, Commit: strings.TrimSpace(string(commit))}, nil
}

func describe(source, version string) string {
	if version == "" {
		return source
	}
	return source + " pada " + version
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Tanpa ini git yang tidak punya kredensial berhenti menunggu nama
	// pengguna di terminal, dan `gonsu new` tampak macet.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func firstLines(out []byte, n int) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.Join(lines[:min(n, len(lines))], "\n")
}

// copyTree menyalin kit dari folder src ke dst dan mengembalikan commit-nya
// bila src repository git.
//
// Dari repository git, yang disalin adalah berkas yang dilacak ditambah yang
// baru dan tidak diabaikan — working tree apa adanya, tanpa node_modules dan
// hasil build. Folder biasa disalin seluruhnya.
func copyTree(ctx context.Context, src, dst string) (string, error) {
	var files []string
	commit := ""
	if _, err := os.Stat(filepath.Join(src, ".git")); err == nil {
		out, err := git(ctx, src, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
		if err != nil {
			return "", fmt.Errorf("git ls-files: %s", firstLines(out, 3))
		}
		for name := range bytes.SplitSeq(out, []byte{0}) {
			if len(name) > 0 {
				files = append(files, string(name))
			}
		}
		if head, err := git(ctx, src, "rev-parse", "HEAD"); err == nil {
			commit = strings.TrimSpace(string(head))
		}
	} else {
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	for _, rel := range files {
		from := filepath.Join(src, filepath.FromSlash(rel))
		info, err := os.Lstat(from)
		if errors.Is(err, os.ErrNotExist) {
			continue // dilacak git, tetapi sudah dihapus di working tree
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			continue // symlink dan berkas khusus tidak ikut
		}
		if err := copyFile(from, filepath.Join(dst, filepath.FromSlash(rel)), info.Mode().Perm()); err != nil {
			return "", err
		}
	}
	return commit, nil
}

func copyFile(from, to string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
