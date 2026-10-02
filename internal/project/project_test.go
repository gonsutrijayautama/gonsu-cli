package project

import (
	"strings"
	"testing"
)

// Aturan kode produk sama dengan kolom products.code GONSU One: kode yang
// lolos di sini tetapi ditolak Console berarti project yang tidak pernah
// dapat dijual.
func TestValidateProductCode(t *testing.T) {
	valid := []string{"toko", "toko-baju", "a1", "garment", strings.Repeat("a", 60)}
	for _, c := range valid {
		if err := ValidateProductCode(c); err != nil {
			t.Errorf("%q ditolak: %v", c, err)
		}
	}
	invalid := []string{"", "a", "Toko", "toko_baju", "-toko", "toko-", "toko--baju", "toko baju", "tokó", strings.Repeat("a", 61)}
	for _, c := range invalid {
		if err := ValidateProductCode(c); err == nil {
			t.Errorf("%q diterima", c)
		}
	}
}

func TestValidateDisplayName(t *testing.T) {
	if err := ValidateDisplayName("  Toko Baju "); err != nil {
		t.Error(err)
	}
	// Nama ditulis apa adanya ke berkas yang bukan kode: harus satu baris.
	for _, n := range []string{"", "   ", strings.Repeat("x", 201), "Toko\nBaju", "Toko\tBaju", "Toko\x1b[2K"} {
		if err := ValidateDisplayName(n); err == nil {
			t.Errorf("%q diterima", n)
		}
	}
}

func TestValidateModulePath(t *testing.T) {
	if err := ValidateModulePath("github.com/organisasi/toko-baju"); err != nil {
		t.Error(err)
	}
	for _, p := range []string{"", "Toko Baju", "github.com/org/toko//x"} {
		if err := ValidateModulePath(p); err == nil {
			t.Errorf("%q diterima", p)
		}
	}
}

func TestDefaults(t *testing.T) {
	if got := DefaultDisplayName("toko-baju-muslim"); got != "Toko Baju Muslim" {
		t.Errorf("nama = %q", got)
	}
	if got := DefaultModulePath("toko"); got != "github.com/gonsutrijayautama/toko" {
		t.Errorf("module = %q", got)
	}
}
