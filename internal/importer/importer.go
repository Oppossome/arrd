// Package importer moves finished downloads into the media library.
package importer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"generic-arr-project/internal/closer"
)

// Mode is how files get from the download dir into the library.
type Mode string

const (
	ModeHardlink Mode = "hardlink" // falls back to copy across filesystems
	ModeCopy     Mode = "copy"
	ModeMove     Mode = "move" // stops seeding
)

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool { return m == ModeHardlink || m == ModeCopy || m == ModeMove }

// File is a file inside a download, relative to the download's directory.
type File struct {
	Path string
	Size int64
}

var videoExts = map[string]bool{".mkv": true, ".mp4": true, ".avi": true, ".m4v": true, ".ts": true, ".wmv": true, ".mov": true}

// minMovieSize skips samples and extras.
const minMovieSize = 50 << 20

// PickMovieFile returns the main feature: the largest video file that is
// not a sample.
func PickMovieFile(files []File) (File, error) {
	var best File
	for _, f := range files {
		if !videoExts[strings.ToLower(filepath.Ext(f.Path))] {
			continue
		}
		if strings.Contains(strings.ToLower(filepath.Base(f.Path)), "sample") || f.Size < minMovieSize {
			continue
		}
		if f.Size > best.Size {
			best = f
		}
	}
	if best.Path == "" {
		return File{}, errors.New("no video file of at least 50 MiB in download")
	}
	return best, nil
}

// MovieFolder is "Title (Year)" with characters illegal on common
// filesystems removed.
func MovieFolder(title string, year int) string {
	name := SanitizeName(title)
	if year > 0 {
		name = fmt.Sprintf("%s (%d)", name, year)
	}
	return name
}

// MovieFileName is "Title (Year) [Quality].ext".
func MovieFileName(title string, year int, quality, ext string) string {
	return fmt.Sprintf("%s [%s]%s", MovieFolder(title, year), SanitizeName(quality), strings.ToLower(ext))
}

// SanitizeName removes path separators and characters that Windows/SMB
// shares reject, and trims trailing dots and spaces.
func SanitizeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '?', '*', '"', '<', '>', '|':
			return -1
		case ':':
			return '-'
		}
		if r < 32 {
			return -1
		}
		return r
	}, s)
	return strings.TrimRight(strings.TrimSpace(s), ". ")
}

// Result describes what Transfer did.
type Result struct {
	// Method is the method actually used; a hardlink request may end up as
	// a copy when source and destination are on different filesystems.
	Method Mode
}

// Transfer places src at dst using mode. It is idempotent: if dst already is
// src (same inode) it succeeds without doing anything. Any other existing
// dst is an error; it never overwrites.
func Transfer(src, dst string, mode Mode) (Result, error) {
	if !mode.Valid() {
		return Result{}, fmt.Errorf("unknown import mode %q", mode)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return Result{}, fmt.Errorf("import source: %w", err)
	}
	if dstInfo, err := os.Stat(dst); err == nil {
		if os.SameFile(srcInfo, dstInfo) {
			return Result{Method: mode}, nil
		}
		return Result{}, fmt.Errorf("import destination %q already exists", dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return Result{}, fmt.Errorf("create library folder: %w", err)
	}

	switch mode {
	case ModeHardlink:
		err := os.Link(src, dst)
		if err == nil {
			return Result{Method: ModeHardlink}, nil
		}
		if !isCrossDevice(err) {
			return Result{}, fmt.Errorf("hardlink: %w", err)
		}
		if err := copyFile(src, dst); err != nil {
			return Result{}, err
		}
		return Result{Method: ModeCopy}, nil
	case ModeMove:
		err := os.Rename(src, dst)
		if err == nil {
			return Result{Method: ModeMove}, nil
		}
		if !isCrossDevice(err) {
			return Result{}, fmt.Errorf("move: %w", err)
		}
		if err := copyFile(src, dst); err != nil {
			return Result{}, err
		}
		if err := os.Remove(src); err != nil {
			return Result{}, fmt.Errorf("move: remove source after copy: %w", err)
		}
		return Result{Method: ModeMove}, nil
	default: // ModeCopy
		if err := copyFile(src, dst); err != nil {
			return Result{}, err
		}
		return Result{Method: ModeCopy}, nil
	}
}

// copyFile copies via a temporary name so a crash never leaves a partial
// file at dst.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	defer closer.Join(&err, in)
	tmp := dst + ".arrd-partial"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(fmt.Errorf("copy: %w", err), out.Close(), os.Remove(tmp))
	}
	if err := out.Close(); err != nil {
		return errors.Join(fmt.Errorf("copy: %w", err), os.Remove(tmp))
	}
	if err := os.Rename(tmp, dst); err != nil {
		return errors.Join(fmt.Errorf("copy: %w", err), os.Remove(tmp))
	}
	return nil
}

// minEpisodeSize skips samples and extras in TV downloads.
const minEpisodeSize = 20 << 20

// VideoFiles returns the non-sample video files of at least 20 MiB, for
// TV downloads where every episode file is wanted.
func VideoFiles(files []File) []File {
	var out []File
	for _, f := range files {
		if !videoExts[strings.ToLower(filepath.Ext(f.Path))] || f.Size < minEpisodeSize {
			continue
		}
		if strings.Contains(strings.ToLower(f.Path), "sample") {
			continue
		}
		out = append(out, f)
	}
	return out
}

// SeriesFolder is "Title (Year)", like MovieFolder.
func SeriesFolder(title string, year int) string { return MovieFolder(title, year) }

// SeasonFolder is "Season 01", or "Specials" for season 0.
func SeasonFolder(season int) string {
	if season == 0 {
		return "Specials"
	}
	return fmt.Sprintf("Season %02d", season)
}

// EpisodeFileName is "Series - S01E02 - Title [Quality].ext"; several
// episodes become "S01E02-E03". An empty episode title is left out.
func EpisodeFileName(series string, season int, episodes []int, episodeTitle, quality, ext string) string {
	tag := fmt.Sprintf("S%02dE%02d", season, episodes[0])
	if len(episodes) > 1 {
		tag += fmt.Sprintf("-E%02d", episodes[len(episodes)-1])
	}
	name := SanitizeName(series) + " - " + tag
	if t := SanitizeName(episodeTitle); t != "" {
		name += " - " + t
	}
	return fmt.Sprintf("%s [%s]%s", name, SanitizeName(quality), strings.ToLower(ext))
}
