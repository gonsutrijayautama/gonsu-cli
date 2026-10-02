// Package kit mengambil starter kit dan mengubahnya menjadi project produk.
//
// Starter kit adalah repository tersendiri berisi aplikasi sungguhan yang
// dapat dijalankan dengan identitas contoh. Tidak ada template: yang
// dikerjakan gonsu hanyalah
//
//  1. mengambil kit pada satu tag (Fetch);
//  2. membuang berkas dan bagian yang hanya milik kit;
//  3. mengganti identitas contoh dengan identitas produk di setiap berkas
//     teks, dalam satu lintasan (Apply).
//
// Kontraknya dengan kit adalah manifes `gonsu.kit.json` di akar kit.
package kit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestName adalah nama berkas manifes di akar kit.
const ManifestName = "gonsu.kit.json"

// schemaVersion adalah versi manifes yang dipahami gonsu ini. Isian baru
// yang hanya MENAMBAH tidak menaikkan versi; isian yang tidak dikenal
// diabaikan.
const schemaVersion = 1

// Manifest adalah isi gonsu.kit.json.
type Manifest struct {
	Schema int    `json:"schema"`
	Kit    string `json:"kit"`
	Label  string `json:"label"`
	// Identity adalah identitas CONTOH yang tertulis di kit.
	Identity Identity `json:"identity"`
	// Remove: berkas dan folder milik kit, dibuang dari project hasil.
	Remove []string `json:"remove"`
	// Install: perintah pemasangan dependency, dijalankan `gonsu new`.
	Install []Step `json:"install"`
	// NextSteps: baris "langkah berikutnya" yang dicetak sesudah project jadi.
	NextSteps []string `json:"next_steps"`
}

// Identity adalah identitas sebuah produk — contoh di manifes, sungguhan
// saat diterapkan.
type Identity struct {
	ProductCode string `json:"product_code"`
	DisplayName string `json:"display_name"`
	// ModulePath kosong untuk kit yang bukan Go.
	ModulePath string `json:"module_path,omitempty"`
}

// Step adalah satu perintah, dijalankan di Dir relatif terhadap akar project.
type Step struct {
	Dir string   `json:"dir"`
	Run []string `json:"run"`
}

// Load membaca dan memeriksa manifes kit di dir.
func Load(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{}, fmt.Errorf("%s tidak ada — sumber ini bukan starter kit GONSU", ManifestName)
	}
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("%s tidak dapat dibaca: %w", ManifestName, err)
	}
	if err := m.validate(); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestName, err)
	}
	return m, nil
}

func (m Manifest) validate() error {
	switch {
	case m.Schema > schemaVersion:
		return fmt.Errorf("skema %d lebih baru daripada yang dipahami gonsu ini (%d) — perbarui gonsu: gonsu update", m.Schema, schemaVersion)
	case m.Schema < 1:
		return errors.New("schema wajib diisi")
	case m.Kit == "":
		return errors.New("kit wajib diisi")
	}
	id := m.Identity
	if id.ProductCode == "" || id.DisplayName == "" {
		return errors.New("identity.product_code dan identity.display_name wajib diisi")
	}
	// Penggantian satu lintasan mengandaikan nilai contoh saling lepas: kode
	// produk yang menjadi bagian nilai lain akan ikut terganti di dalamnya.
	for _, other := range []string{id.DisplayName, id.ModulePath} {
		if strings.Contains(other, id.ProductCode) {
			return fmt.Errorf("identity.product_code %q adalah bagian dari %q", id.ProductCode, other)
		}
	}
	for _, p := range m.Remove {
		if !filepath.IsLocal(filepath.FromSlash(p)) {
			return fmt.Errorf("remove menyebut %q, yang keluar dari folder project", p)
		}
	}
	for _, s := range m.Install {
		if len(s.Run) == 0 {
			return errors.New("install memuat langkah tanpa perintah")
		}
		if s.Dir != "" && !filepath.IsLocal(filepath.FromSlash(s.Dir)) {
			return fmt.Errorf("install menyebut folder %q, yang keluar dari folder project", s.Dir)
		}
	}
	return nil
}
