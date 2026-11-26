package importer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPickMovieFile(t *testing.T) {
	files := []File{
		{"Movie/movie.nfo", 1 << 10},
		{"Movie/Sample/movie-sample.mkv", 100 << 20},
		{"Movie/movie.mkv", 4 << 30},
		{"Movie/extras/featurette.mkv", 300 << 20},
		{"Movie/tiny.mp4", 1 << 20},
	}
	got, err := PickMovieFile(files)
	if err != nil || got.Path != "Movie/movie.mkv" {
		t.Errorf("PickMovieFile = %+v, %v", got, err)
	}
	if _, err := PickMovieFile([]File{{"a.nfo", 1 << 30}, {"sample.mkv", 1 << 30}}); err == nil {
		t.Error("expected error with no usable video")
	}
}

func TestNames(t *testing.T) {
	if got := MovieFolder("Agent Zero: Part / One?", 2018); got != "Agent Zero- Part  One (2018)" {
		t.Errorf("MovieFolder = %q", got)
	}
	if got := MovieFileName("The Glass Orchard", 1999, "Bluray-1080p", ".MKV"); got != "The Glass Orchard (1999) [Bluray-1080p].mkv" {
		t.Errorf("MovieFileName = %q", got)
	}
	if got := MovieFolder("Untitled", 0); got != "Untitled" {
		t.Errorf("MovieFolder no year = %q", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTransferModes(t *testing.T) {
	dir := t.TempDir()
	for _, mode := range []Mode{ModeHardlink, ModeCopy, ModeMove} {
		t.Run(string(mode), func(t *testing.T) {
			src := filepath.Join(dir, string(mode), "src.mkv")
			dst := filepath.Join(dir, string(mode), "lib", "Movie (2000)", "dst.mkv")
			writeFile(t, src, "data")

			res, err := Transfer(src, dst, mode)
			if err != nil {
				t.Fatal(err)
			}
			if res.Method != mode {
				t.Errorf("method = %s, want %s (same filesystem)", res.Method, mode)
			}
			if b, err := os.ReadFile(dst); err != nil || string(b) != "data" {
				t.Errorf("dst = %q, %v", b, err)
			}
			_, srcErr := os.Stat(src)
			if mode == ModeMove && !os.IsNotExist(srcErr) {
				t.Error("move left the source in place")
			}
			if mode != ModeMove && srcErr != nil {
				t.Errorf("%s removed the source: %v", mode, srcErr)
			}
			if mode == ModeHardlink {
				si, _ := os.Stat(src)
				di, _ := os.Stat(dst)
				if !os.SameFile(si, di) {
					t.Error("hardlink produced a separate file")
				}
				// Re-running is a no-op.
				if _, err := Transfer(src, dst, mode); err != nil {
					t.Errorf("repeat hardlink import: %v", err)
				}
			}
		})
	}
}

func TestTransferRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a.mkv"), filepath.Join(dir, "b.mkv")
	writeFile(t, src, "new")
	writeFile(t, dst, "existing")
	if _, err := Transfer(src, dst, ModeCopy); err == nil {
		t.Fatal("Transfer overwrote an existing file")
	}
	if b, _ := os.ReadFile(dst); string(b) != "existing" {
		t.Error("existing file was modified")
	}
	if _, err := Transfer(src, filepath.Join(dir, "c.mkv"), "symlink"); err == nil {
		t.Error("unknown mode accepted")
	}
}

func TestCheckWritableDir(t *testing.T) {
	dir := t.TempDir()
	if err := CheckWritableDir(dir); err != nil {
		t.Errorf("temp dir: %v", err)
	}
	if err := CheckWritableDir("relative/path"); err == nil {
		t.Error("relative path accepted")
	}
	if err := CheckWritableDir(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing dir accepted")
	}
	file := filepath.Join(dir, "f")
	writeFile(t, file, "")
	if err := CheckWritableDir(file); err == nil {
		t.Error("file accepted as dir")
	}
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	// Mode bits cannot make a directory unwritable on Windows (only ACLs
	// can), and root ignores them.
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		if err := CheckWritableDir(ro); err == nil {
			t.Error("read-only dir accepted")
		}
	}
	same, err := SameFilesystem(dir, ro)
	if err != nil || !same {
		t.Errorf("SameFilesystem = %v, %v", same, err)
	}
}

func TestTVNames(t *testing.T) {
	if got := EpisodeFileName("Harbor Watch", 1, []int{2}, "Low Tide?", "WEBDL-1080p", ".MKV"); got != "Harbor Watch - S01E02 - Low Tide [WEBDL-1080p].mkv" {
		t.Errorf("single = %q", got)
	}
	if got := EpisodeFileName("Harbor Watch", 1, []int{2, 3}, "", "HDTV-720p", ".mkv"); got != "Harbor Watch - S01E02-E03 [HDTV-720p].mkv" {
		t.Errorf("multi = %q", got)
	}
	if SeasonFolder(3) != "Season 03" || SeasonFolder(0) != "Specials" {
		t.Error("season folder")
	}
	files := VideoFiles([]File{{"a/ep1.mkv", 300 << 20}, {"a/Sample/ep1.mkv", 30 << 20}, {"a/ep1.nfo", 1}, {"a/tiny.mkv", 1 << 20}})
	if len(files) != 1 || files[0].Path != "a/ep1.mkv" {
		t.Errorf("VideoFiles = %+v", files)
	}
}
