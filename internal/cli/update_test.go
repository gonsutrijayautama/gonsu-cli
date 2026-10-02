package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeReleases adalah halaman rilis tiruan: tanpa jaringan.
type fakeReleases struct {
	latest     string
	latestErr  error
	installErr error
	// slow menahan Latest sampai konteksnya selesai, seperti jaringan mati.
	slow      bool
	installed []string
}

func (f *fakeReleases) Latest(ctx context.Context) (string, error) {
	if f.slow {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return f.latest, f.latestErr
}

func (f *fakeReleases) Install(_ context.Context, version string) error {
	f.installed = append(f.installed, version)
	return f.installErr
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		latest    string
		args      []string
		installed []string
		out       string
	}{
		{"ada rilis baru", "v0.1.1", "v0.2.0", nil, []string{"v0.2.0"}, "gonsu v0.2.0 terpasang"},
		{"ada rilis baru, hanya memeriksa", "v0.1.1", "v0.2.0", []string{"--check"}, nil,
			"gonsu v0.2.0 tersedia (terpasang v0.1.1). Jalankan: gonsu update"},
		{"sudah terbaru", "v0.2.0", "v0.2.0", nil, nil, "gonsu v0.2.0 sudah yang terbaru"},
		{"sudah terbaru, hanya memeriksa", "v0.2.0", "v0.2.0", []string{"--check"}, nil, "sudah yang terbaru"},
		// Urutan versi, bukan urutan teks: v0.10.0 lebih baru daripada v0.9.0.
		{"urutan versi", "v0.9.0", "v0.10.0", nil, []string{"v0.10.0"}, "gonsu v0.10.0 terpasang"},
		// gonsu yang lebih baru daripada rilis (pra-rilis yang dibangun
		// sendiri) tidak diturunkan.
		{"terpasang lebih baru", "v0.3.0", "v0.2.0", nil, nil, "sudah yang terbaru"},
		{"pra-rilis lebih lama daripada rilisnya", "v0.2.0-rc.1", "v0.2.0", nil, []string{"v0.2.0"}, "terpasang"},
		{"build pengembangan, hanya memeriksa", "(devel)", "v0.2.0", []string{"--check"}, nil,
			"build pengembangan (devel); rilis terbaru v0.2.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t, false)
			releases := &fakeReleases{latest: tt.latest}
			w.env.Version, w.env.Releases = tt.current, releases
			if err := Run(context.Background(), append([]string{"update"}, tt.args...), w.env); err != nil {
				t.Fatalf("update: %v", err)
			}
			if strings.Join(releases.installed, ",") != strings.Join(tt.installed, ",") {
				t.Errorf("yang dipasang = %v, ingin %v", releases.installed, tt.installed)
			}
			if !strings.Contains(w.out.String(), tt.out) {
				t.Errorf("keluaran tidak memuat %q:\n%s", tt.out, w.out)
			}
		})
	}
}

func TestUpdateFailures(t *testing.T) {
	down := errors.New("jaringan mati")
	tests := []struct {
		name     string
		current  string
		releases Releases
		args     []string
		want     string
		usage    bool
	}{
		{"rilis tidak terjangkau", "v0.1.1", &fakeReleases{latestErr: down}, nil, "memeriksa rilis terbaru: jaringan mati", false},
		{"nomor rilis bukan versi", "v0.1.1", &fakeReleases{latest: "terbaru"}, nil, "bukan nomor versi", false},
		{"pemasangan gagal", "v0.1.1", &fakeReleases{latest: "v0.2.0", installErr: down}, nil, "memperbarui gonsu: jaringan mati", false},
		// Build pengembangan tidak ditimpa diam-diam.
		{"build pengembangan", "(devel)", &fakeReleases{latest: "v0.2.0"}, nil, "tidak diperbarui otomatis", false},
		{"tanpa halaman rilis", "v0.1.1", nil, nil, "tidak dapat memeriksa rilis", false},
		{"argumen berlebih", "v0.1.1", &fakeReleases{latest: "v0.2.0"}, []string{"v0.2.0"}, "tidak menerima argumen", true},
		{"flag tidak dikenal", "v0.1.1", &fakeReleases{latest: "v0.2.0"}, []string{"--paksa"}, "paksa", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t, false)
			w.env.Version = tt.current
			if tt.releases != nil {
				w.env.Releases = tt.releases
			}
			err := Run(context.Background(), append([]string{"update"}, tt.args...), w.env)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("galat = %v, ingin memuat %q", err, tt.want)
			}
			if errors.Is(err, ErrUsage) != tt.usage {
				t.Errorf("galat pemakaian = %v, ingin %v", errors.Is(err, ErrUsage), tt.usage)
			}
			if f, ok := tt.releases.(*fakeReleases); ok && tt.name != "pemasangan gagal" && len(f.installed) > 0 {
				t.Errorf("tetap memasang %v", f.installed)
			}
		})
	}
}

// Sesudah project jadi, gonsu memberi tahu bila ada rilis yang lebih baru —
// hanya di terminal, dan tidak pernah menahan maupun menggagalkan gonsu new.
func TestNewMentionsNewerRelease(t *testing.T) {
	run := func(t *testing.T, interactive bool, version string, releases *fakeReleases) string {
		t.Helper()
		w := newWorld(t, interactive)
		w.env.Version, w.env.Releases = version, releases
		if err := Run(context.Background(), []string{"new", "toko", "-n", "--git=false", "--install=false"}, w.env); err != nil {
			t.Fatalf("new: %v", err)
		}
		return w.out.String()
	}
	const notice = "gonsu v0.2.0 tersedia (terpasang v0.1.1). Jalankan: gonsu update"

	if out := run(t, true, "v0.1.1", &fakeReleases{latest: "v0.2.0"}); !strings.Contains(out, notice) {
		t.Errorf("tidak ada pemberitahuan rilis baru:\n%s", out)
	}
	for name, out := range map[string]string{
		"sudah terbaru":        run(t, true, "v0.2.0", &fakeReleases{latest: "v0.2.0"}),
		"bukan terminal":       run(t, false, "v0.1.1", &fakeReleases{latest: "v0.2.0"}),
		"build pengembangan":   run(t, true, "(devel)", &fakeReleases{latest: "v0.2.0"}),
		"rilis tak terjangkau": run(t, true, "v0.1.1", &fakeReleases{latestErr: errors.New("mati")}),
	} {
		if strings.Contains(out, "tersedia") {
			t.Errorf("%s: ada pemberitahuan rilis baru:\n%s", name, out)
		}
	}

	// Jaringan yang tidak menjawab tidak menahan gonsu new lebih dari batasnya.
	start := time.Now()
	out := run(t, true, "v0.1.1", &fakeReleases{slow: true})
	if strings.Contains(out, "tersedia") || !strings.Contains(out, "Langkah berikutnya") {
		t.Errorf("keluaran saat rilis tidak menjawab:\n%s", out)
	}
	if waited := time.Since(start); waited > 10*time.Second {
		t.Errorf("gonsu new tertahan %s oleh pemeriksaan rilis", waited)
	}
}
