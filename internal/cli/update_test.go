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

// restarts mencatat argumen gonsu yang dijalankan ulang, dan menjawab err.
type restarts struct {
	args [][]string
	err  error
}

func (r *restarts) run(_ context.Context, args []string) error {
	r.args = append(r.args, args)
	return r.err
}

var newArgs = []string{"new", "toko", "-n", "--git=false", "--install=false"}

// Di terminal, gonsu new memasang rilis yang lebih baru dulu, lalu
// menyerahkan perintahnya ke gonsu yang baru itu.
func TestNewUpdatesFirst(t *testing.T) {
	w := newWorld(t, true)
	releases, restarted := &fakeReleases{latest: "v0.2.1"}, &restarts{}
	w.env.Version, w.env.Releases, w.env.Restart = "v0.2.0", releases, restarted.run
	if err := Run(context.Background(), newArgs, w.env); err != nil {
		t.Fatalf("new: %v", err)
	}
	if strings.Join(releases.installed, ",") != "v0.2.1" {
		t.Errorf("yang dipasang = %v, ingin v0.2.1", releases.installed)
	}
	// --update=false: gonsu yang baru tidak memeriksa rilis sekali lagi.
	want := "new --update=false toko -n --git=false --install=false"
	if len(restarted.args) != 1 || strings.Join(restarted.args[0], " ") != want {
		t.Errorf("dijalankan ulang dengan %q, ingin %q", restarted.args, want)
	}
	// Project dibuat gonsu yang baru, bukan yang ini.
	if len(w.fetched) != 0 {
		t.Errorf("gonsu lama tetap mengambil kit: %q", w.fetched)
	}
	for _, want := range []string{"memperbarui gonsu v0.2.0 → v0.2.1", "gonsu v0.2.1 terpasang"} {
		if !strings.Contains(w.out.String(), want) {
			t.Errorf("keluaran tidak memuat %q:\n%s", want, w.out)
		}
	}

	// Galat gonsu yang baru menjadi galat perintah ini, dengan kode yang sama.
	w = newWorld(t, true)
	w.env.Version, w.env.Releases = "v0.2.0", &fakeReleases{latest: "v0.2.1"}
	w.env.Restart = (&restarts{err: ExitError{Code: 2}}).run
	err := Run(context.Background(), newArgs, w.env)
	if exit, ok := errors.AsType[ExitError](err); !ok || exit.Code != 2 {
		t.Errorf("galat = %v, ingin ExitError berkode 2", err)
	}
}

// Yang tidak diperbarui: bukan terminal (skrip dan CI), gonsu yang sudah
// terbaru, build pengembangan, rilis yang tidak terjangkau, dan
// --update=false. Project tetap dibuat gonsu yang terpasang.
func TestNewDoesNotUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		interactive bool
		version     string
		releases    *fakeReleases
		args        []string
	}{
		"bukan terminal":       {false, "v0.2.0", &fakeReleases{latest: "v0.2.1"}, nil},
		"sudah terbaru":        {true, "v0.2.1", &fakeReleases{latest: "v0.2.1"}, nil},
		"terpasang lebih baru": {true, "v0.3.0", &fakeReleases{latest: "v0.2.1"}, nil},
		"build pengembangan":   {true, "(devel)", &fakeReleases{latest: "v0.2.1"}, nil},
		"rilis tak terjangkau": {true, "v0.2.0", &fakeReleases{latestErr: errors.New("mati")}, nil},
		"nomor rilis aneh":     {true, "v0.2.0", &fakeReleases{latest: "terbaru"}, nil},
		"--update=false":       {true, "v0.2.0", &fakeReleases{latest: "v0.2.1"}, []string{"--update=false"}},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, tc.interactive)
			restarted := &restarts{}
			w.env.Version, w.env.Releases, w.env.Restart = tc.version, tc.releases, restarted.run
			if err := Run(context.Background(), append(newArgs, tc.args...), w.env); err != nil {
				t.Fatalf("new: %v", err)
			}
			if len(tc.releases.installed) != 0 || len(restarted.args) != 0 {
				t.Errorf("tetap memperbarui: dipasang %v, dijalankan ulang %q", tc.releases.installed, restarted.args)
			}
			if len(w.fetched) != 1 || !strings.Contains(w.out.String(), "Langkah berikutnya") {
				t.Errorf("project tidak dibuat:\n%s", w.out)
			}
		})
	}

	// Yang menolak diperbarui tetap diberi tahu ada rilis baru.
	w := newWorld(t, true)
	w.env.Version, w.env.Releases, w.env.Restart = "v0.2.0", &fakeReleases{latest: "v0.2.1"}, (&restarts{}).run
	if err := Run(context.Background(), append(newArgs, "--update=false"), w.env); err != nil {
		t.Fatalf("new: %v", err)
	}
	if want := "gonsu v0.2.1 tersedia (terpasang v0.2.0). Jalankan: gonsu update"; !strings.Contains(w.out.String(), want) {
		t.Errorf("keluaran tidak memuat %q:\n%s", want, w.out)
	}
}

// Pembaruan yang gagal tidak pernah menggagalkan gonsu new: project dibuat
// gonsu yang terpasang, dan pemakai diberi tahu sebabnya.
func TestNewContinuesWhenUpdateFails(t *testing.T) {
	down := errors.New("folder /usr/local/bin tidak dapat ditulis\nPasang ulang lewat installer.")
	for name, tc := range map[string]struct {
		releases *fakeReleases
		restart  error
		want     string
	}{
		"pemasangan gagal": {&fakeReleases{latest: "v0.2.1", installErr: down}, nil,
			"gonsu v0.2.1 belum bisa dipasang (folder /usr/local/bin tidak dapat ditulis); lanjut dengan v0.2.0. Coba nanti: gonsu update"},
		"gonsu baru tidak mau jalan": {&fakeReleases{latest: "v0.2.1"}, errors.New("exec format error"),
			"gonsu v0.2.1 tidak bisa dijalankan dari sini (exec format error); lanjut dengan v0.2.0"},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, true)
			w.env.Version, w.env.Releases = "v0.2.0", tc.releases
			w.env.Restart = (&restarts{err: tc.restart}).run
			if err := Run(context.Background(), newArgs, w.env); err != nil {
				t.Fatalf("new: %v", err)
			}
			for _, want := range []string{tc.want, "Langkah berikutnya"} {
				if !strings.Contains(w.out.String(), want) {
					t.Errorf("keluaran tidak memuat %q:\n%s", want, w.out)
				}
			}
		})
	}

	// Rilis yang tidak menjawab tidak menahan gonsu new lebih dari batasnya.
	w := newWorld(t, true)
	w.env.Version, w.env.Releases, w.env.Restart = "v0.2.0", &fakeReleases{slow: true}, (&restarts{}).run
	start := time.Now()
	if err := Run(context.Background(), newArgs, w.env); err != nil {
		t.Fatalf("new: %v", err)
	}
	if !strings.Contains(w.out.String(), "Langkah berikutnya") {
		t.Errorf("project tidak dibuat:\n%s", w.out)
	}
	if waited := time.Since(start); waited > 10*time.Second {
		t.Errorf("gonsu new tertahan %s oleh pemeriksaan rilis", waited)
	}

	// Ctrl+C di tengah pembaruan: gonsu berhenti, tidak lanjut membuat project.
	ctx, cancel := context.WithCancel(context.Background())
	w = newWorld(t, true)
	w.env.Version, w.env.Restart = "v0.2.0", (&restarts{}).run
	w.env.Releases = interrupted{fakeReleases: &fakeReleases{latest: "v0.2.1"}, cancel: cancel}
	err := Run(ctx, newArgs, w.env)
	if err == nil || !strings.Contains(err.Error(), "dibatalkan") || len(w.fetched) != 0 {
		t.Errorf("galat = %v, kit yang diambil = %q", err, w.fetched)
	}
}

// interrupted adalah halaman rilis yang pemasangannya dihentikan pemakai.
type interrupted struct {
	*fakeReleases
	cancel context.CancelFunc
}

func (i interrupted) Install(ctx context.Context, _ string) error {
	i.cancel()
	return ctx.Err()
}

// Tanpa pembaruan otomatis, gonsu memberi tahu sesudah project jadi bila ada
// rilis yang lebih baru — hanya di terminal, dan tidak pernah menahan maupun
// menggagalkan gonsu new.
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
