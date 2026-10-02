package kit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Penanda bagian yang hanya milik kit. Baris di antara keduanya, beserta
// kedua baris penandanya, dibuang dari project hasil.
const (
	blockBegin = "gonsu-kit:begin"
	blockEnd   = "gonsu-kit:end"
)

// OriginPath adalah berkas di project hasil yang mencatat kit asalnya.
const OriginPath = ".gonsu/kit.json"

// Apply mengubah salinan kit di dir menjadi project produk ber-identitas id:
// berkas milik kit dibuang, bagian khusus kit dicabut, identitas contoh
// diganti, dan asalnya dicatat di OriginPath.
func Apply(dir string, m Manifest, id Identity, origin Origin) error {
	if id.ProductCode == "" || id.DisplayName == "" {
		return errors.New("kode produk dan nama tampilan wajib diisi")
	}
	if m.Identity.ModulePath != "" && id.ModulePath == "" {
		return errors.New("kit ini butuh module path Go")
	}
	for _, p := range m.Remove {
		if err := os.RemoveAll(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			return err
		}
	}
	// Manifes selalu dibuang, walau kit lupa mendaftarkannya.
	if err := os.RemoveAll(filepath.Join(dir, ManifestName)); err != nil {
		return err
	}

	raw, code := replacers(m.Identity, id)
	samples := sampleValues(m.Identity)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		for _, s := range samples {
			if strings.Contains(filepath.ToSlash(rel), s) {
				return fmt.Errorf("kit menamai %s dengan identitas contoh %q; nama berkas tidak ikut diganti", rel, s)
			}
		}
		if !d.Type().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if isBinary(content) {
			for _, s := range samples {
				if bytes.Contains(content, []byte(s)) {
					return fmt.Errorf("berkas biner %s memuat identitas contoh %q, yang tidak dapat diganti", rel, s)
				}
			}
			return nil
		}
		out, err := stripKitBlocks(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if isCode(p) {
			out = code.Replace(out)
		} else {
			out = raw.Replace(out)
		}
		if out == string(content) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(p, []byte(out), info.Mode().Perm())
	})
	if err != nil {
		return err
	}
	return writeOrigin(dir, m, origin)
}

func sampleValues(sample Identity) []string {
	out := []string{sample.ProductCode, sample.DisplayName}
	if sample.ModulePath != "" {
		out = append(out, sample.ModulePath)
	}
	return out
}

// replacers menyusun pengganti identitas contoh → identitas produk: satu
// untuk berkas teks biasa, satu untuk berkas kode.
//
// strings.Replacer mengganti dalam SATU lintasan dan tidak memeriksa ulang
// hasilnya. Itu syaratnya, bukan sekadar kecepatan: produk berkode
// "produk-contoh-2" memuat nilai contoh di dalam identitasnya sendiri, dan
// penggantian berurutan akan menggantinya dua kali.
func replacers(sample, id Identity) (raw, code *strings.Replacer) {
	var rawPairs, codePairs []string
	// Module path lebih dulu: di tiap posisi, nilai yang disebut lebih awal
	// yang menang.
	if sample.ModulePath != "" {
		rawPairs = append(rawPairs, sample.ModulePath, id.ModulePath)
		codePairs = append(codePairs, sample.ModulePath, id.ModulePath)
	}
	rawPairs = append(rawPairs, sample.ProductCode, id.ProductCode, sample.DisplayName, id.DisplayName)
	// Di berkas kode, nama tampilan berada di dalam string literal: nama yang
	// memuat tanda kutip atau garis miring terbalik harus ter-escape.
	codePairs = append(codePairs, sample.ProductCode, id.ProductCode, sample.DisplayName, escapeString(id.DisplayName))
	return strings.NewReplacer(rawPairs...), strings.NewReplacer(codePairs...)
}

// isCode: berkas yang menuliskan nama tampilan di dalam string literal.
func isCode(path string) bool {
	switch filepath.Ext(path) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json", ".go":
		return true
	}
	return false
}

// escapeString menuliskan s untuk isi string literal berkutip ganda di
// TypeScript, JSON, dan Go — tanpa kutip pembukanya.
func escapeString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // string selalu dapat di-encode
	out := strings.TrimSuffix(buf.String(), "\n")
	return out[1 : len(out)-1]
}

// isBinary: berkas yang memuat byte NUL di bagian awalnya, sama dengan cara
// git mengenalinya.
func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(len(content), 8000)], 0) >= 0
}

// stripKitBlocks membuang bagian di antara penanda gonsu-kit:begin dan
// gonsu-kit:end, beserta kedua baris penandanya.
func stripKitBlocks(content string) (string, error) {
	if !strings.Contains(content, blockBegin) && !strings.Contains(content, blockEnd) {
		return content, nil
	}
	var out []string
	inside, justClosed := false, false
	for line := range strings.SplitSeq(content, "\n") {
		switch {
		case strings.Contains(line, blockBegin):
			if inside {
				return "", errors.New("penanda " + blockBegin + " di dalam bagian yang belum ditutup")
			}
			inside = true
		case strings.Contains(line, blockEnd):
			if !inside {
				return "", errors.New("penanda " + blockEnd + " tanpa " + blockBegin)
			}
			inside, justClosed = false, true
		case inside:
		default:
			// Bagian yang diapit baris kosong meninggalkan dua baris kosong
			// berurutan; yang kedua ikut dibuang.
			blankAfterBlank := line == "" && len(out) > 0 && out[len(out)-1] == ""
			if !(justClosed && blankAfterBlank) {
				out = append(out, line)
			}
			justClosed = false
		}
	}
	if inside {
		return "", errors.New("penanda " + blockBegin + " tanpa " + blockEnd)
	}
	return strings.Join(out, "\n"), nil
}

// writeOrigin mencatat kit asal project. Dengan catatan ini, selisih antara
// tag kit asal dan tag terbaru dapat dicari saat produk ingin mengikuti
// perubahan kit.
func writeOrigin(dir string, m Manifest, origin Origin) error {
	record := struct {
		Kit string `json:"kit"`
		Origin
	}{Kit: m.Kit, Origin: origin}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filepath.FromSlash(OriginPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
