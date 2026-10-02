// Package catalog adalah daftar starter kit yang dapat dipilih di `gonsu new`.
//
// Setiap kit adalah repository tersendiri — aplikasi sungguhan yang dapat
// dijalankan — dan gonsu hanya menariknya. Yang diambil selalu ujung cabang
// bawaan kit (`main`): perubahan kit langsung sampai ke `gonsu new` tanpa
// rilis gonsu. Karena itu `main` kit harus selalu siap dipakai.
//
// Kit yang belum tersedia tetap tercantum supaya tim tahu arahnya, tetapi
// tidak dapat dipilih: kit tanpa SDK GONSU untuk bahasanya berarti menulis
// ulang login OIDC dan pembacaan lease di dalam kit — persis yang ingin
// dihindari.
package catalog

import "fmt"

// Kit adalah satu starter kit.
type Kit struct {
	// ID dipakai di flag --kit dan sama dengan akhiran nama repository-nya:
	// gonsu-starter-<ID>.
	ID    string
	Label string
	// Repository adalah alamat git kit. HTTPS, supaya satu aturan
	// `url.<...>.insteadOf` atau credential helper milik pemakai berlaku
	// untuk git dan untuk `go get`.
	Repository string
	// Available: kit-nya ada dan teruji. Yang belum, tampil dengan Pending
	// sebagai alasannya.
	Available bool
	Pending   string
}

// Kits adalah seluruh pilihan, urut seperti ditampilkan.
var Kits = []Kit{
	{
		ID:         "go-nextjs",
		Label:      "Go + Next.js",
		Repository: "https://github.com/gonsutrijayautama/gonsu-starter-go-nextjs.git",
		Available:  true,
	},
	{ID: "nextjs", Label: "Next.js", Pending: "menunggu SDK GONSU untuk JavaScript"},
	{ID: "laravel", Label: "Laravel", Pending: "menunggu SDK GONSU untuk PHP"},
}

// Lookup mengembalikan kit dengan id tertentu.
func Lookup(id string) (Kit, bool) {
	for _, k := range Kits {
		if k.ID == id {
			return k, true
		}
	}
	return Kit{}, false
}

// FirstAvailable adalah kit bawaan.
func FirstAvailable() (Kit, bool) {
	for _, k := range Kits {
		if k.Available {
			return k, true
		}
	}
	return Kit{}, false
}

// Resolve memeriksa pilihan kit, dengan pesan yang menyebut sebabnya.
func Resolve(id string) (Kit, error) {
	k, ok := Lookup(id)
	if !ok {
		return Kit{}, fmt.Errorf("kit %q tidak dikenal", id)
	}
	if !k.Available {
		return Kit{}, fmt.Errorf("kit %s belum tersedia (%s)", k.Label, k.Pending)
	}
	return k, nil
}
