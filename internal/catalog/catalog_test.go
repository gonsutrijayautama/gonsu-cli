package catalog

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	if k, err := Resolve("go-nextjs"); err != nil || k.ID != "go-nextjs" {
		t.Fatalf("go-nextjs = %v %v", k.ID, err)
	}
	for id, want := range map[string]string{
		"rust":    "tidak dikenal",
		"laravel": "belum tersedia",
		"":        "tidak dikenal",
	} {
		if _, err := Resolve(id); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Resolve(%q) = %v, ingin menyebut %q", id, err, want)
		}
	}
}

// Bawaan harus kit yang dapat dipakai: gonsu new -n tanpa --kit tidak boleh
// berakhir di pilihan yang belum tersedia.
func TestDefaultIsAvailable(t *testing.T) {
	if k, ok := FirstAvailable(); !ok || !k.Available {
		t.Fatal("tidak ada kit bawaan yang tersedia")
	}
}

// Kit yang tersedia wajib punya alamat. Kit yang belum tersedia wajib menyebut
// alasannya — itulah yang tampil di samping namanya.
func TestKitsAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kits {
		if seen[k.ID] {
			t.Errorf("kit %s tercantum dua kali", k.ID)
		}
		seen[k.ID] = true
		if !k.Available {
			if k.Pending == "" {
				t.Errorf("kit %s belum tersedia tanpa alasan", k.ID)
			}
			continue
		}
		if !strings.HasPrefix(k.Repository, "https://") || !strings.HasSuffix(k.Repository, "/gonsu-starter-"+k.ID+".git") {
			t.Errorf("kit %s: alamat %q harus https://…/gonsu-starter-%s.git", k.ID, k.Repository, k.ID)
		}
	}
}
