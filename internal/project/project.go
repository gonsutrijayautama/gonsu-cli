// Package project memegang isian yang membentuk satu produk baru, beserta
// aturannya.
package project

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/mod/module"
)

// productCodePattern sama dengan aturan kolom products.code di GONSU One.
var productCodePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateProductCode memeriksa kode produk dengan aturan yang sama dengan
// Console.
func ValidateProductCode(code string) error {
	switch {
	case code == "":
		return errors.New("kode produk wajib diisi")
	case len(code) < 2 || len(code) > 60:
		return fmt.Errorf("kode produk harus 2–60 karakter, diterima %d", len(code))
	case !productCodePattern.MatchString(code):
		return fmt.Errorf("kode produk %q hanya boleh huruf kecil, angka, dan tanda hubung di antaranya — misalnya toko-baju", code)
	}
	return nil
}

// ValidateDisplayName memeriksa nama tampilan: 1–200 karakter, sama dengan
// nama produk di Console.
func ValidateDisplayName(name string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	if n == 0 {
		return errors.New("nama tampilan wajib diisi")
	}
	if n > 200 {
		return fmt.Errorf("nama tampilan paling panjang 200 karakter, diterima %d", n)
	}
	return nil
}

// ValidateModulePath memeriksa module path Go.
func ValidateModulePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("module path Go wajib diisi, misalnya github.com/organisasi/toko")
	}
	if err := module.CheckPath(path); err != nil {
		return fmt.Errorf("module path Go %q tidak sah: %w", path, err)
	}
	return nil
}

// DefaultDisplayName menyusun nama tampilan dari kode: "toko-baju" → "Toko Baju".
func DefaultDisplayName(code string) string {
	words := strings.Split(code, "-")
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
		}
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// DefaultModulePath adalah module path bawaan untuk tim GONSU.
func DefaultModulePath(code string) string {
	return "github.com/gonsutrijayautama/" + code
}
