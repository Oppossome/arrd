package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/store/db"
)

// MediaType is the kind of media a root folder holds.
type MediaType string

const (
	MediaMovie MediaType = "movie"
	MediaTV    MediaType = "tv"
)

func (m MediaType) valid() bool { return m == MediaMovie || m == MediaTV }

// DownloadSettings control where torrents are written and how they are
// imported.
type DownloadSettings struct {
	IncompleteDir string        `json:"incomplete_dir"`
	CompleteDir   string        `json:"complete_dir"`
	ImportMode    importer.Mode `json:"import_mode"`
}

// SettingsUpdate reports the outcome of a settings change.
type SettingsUpdate struct {
	Settings DownloadSettings `json:"settings"`
	Warnings []string         `json:"warnings,omitempty"`
}

// DownloadSettings returns the current download settings.
func (s *Service) DownloadSettings(ctx context.Context) (DownloadSettings, error) {
	row, err := s.store.GetDownloadSettings(ctx)
	if err != nil {
		return DownloadSettings{}, fmt.Errorf("load download settings: %w", err)
	}
	return downloadSettingsFromRow(row), nil
}

func downloadSettingsFromRow(r db.DownloadSetting) DownloadSettings {
	return DownloadSettings{
		IncompleteDir: r.IncompleteDir,
		CompleteDir:   r.CompleteDir,
		ImportMode:    importer.Mode(r.ImportMode),
	}
}

// validateDownloadSettings checks download settings without saving them.
// With createMissing, missing directories are created first.
func validateDownloadSettings(in DownloadSettings, createMissing bool) (DownloadSettings, error) {
	if in.IncompleteDir == "" {
		return in, invalid("incomplete_dir", "is required")
	}
	if in.CompleteDir == "" {
		return in, invalid("complete_dir", "is required")
	}
	in.IncompleteDir = filepath.Clean(in.IncompleteDir)
	in.CompleteDir = filepath.Clean(in.CompleteDir)
	if err := checkDir(in.IncompleteDir, createMissing); err != nil {
		return in, invalid("incomplete_dir", "%v", err)
	}
	if err := checkDir(in.CompleteDir, createMissing); err != nil {
		return in, invalid("complete_dir", "%v", err)
	}
	// Finished downloads are renamed from incomplete_dir to complete_dir,
	// which only works within one filesystem.
	if same, err := importer.SameFilesystem(in.IncompleteDir, in.CompleteDir); err != nil {
		return in, invalid("complete_dir", "compare filesystems: %v", err)
	} else if !same {
		return in, invalid("complete_dir", "must be on the same filesystem as incomplete_dir")
	}
	if in.ImportMode == "" {
		in.ImportMode = importer.ModeHardlink
	}
	if !in.ImportMode.Valid() {
		return in, invalid("import_mode", "must be hardlink, copy or move")
	}
	return in, nil
}

// checkDir verifies path is a writable absolute directory, creating it
// first when create is set.
func checkDir(path string, create bool) error {
	if create && filepath.IsAbs(path) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return importer.CheckWritableDir(path)
}

// vpnConfigured reports whether a VPN is saved.
func (s *Service) vpnConfigured(ctx context.Context) (bool, error) {
	_, err := s.store.GetVPNSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load vpn settings: %w", err)
	}
	return true, nil
}

// UpdateDownloadSettings validates and saves new download settings. New
// paths only affect downloads grabbed afterwards.
func (s *Service) UpdateDownloadSettings(ctx context.Context, in DownloadSettings) (SettingsUpdate, error) {
	in, err := validateDownloadSettings(in, false)
	if err != nil {
		return SettingsUpdate{}, err
	}
	row, err := s.store.UpsertDownloadSettings(ctx, db.UpsertDownloadSettingsParams{
		IncompleteDir: in.IncompleteDir,
		CompleteDir:   in.CompleteDir,
		ImportMode:    string(in.ImportMode),
	})
	if err != nil {
		return SettingsUpdate{}, fmt.Errorf("save download settings: %w", err)
	}
	out := SettingsUpdate{Settings: downloadSettingsFromRow(row)}
	out.Warnings = s.hardlinkWarnings(ctx, out.Settings)
	return out, nil
}

// hardlinkWarnings explains when hardlink imports will fall back to copies.
func (s *Service) hardlinkWarnings(ctx context.Context, set DownloadSettings) []string {
	if set.ImportMode != importer.ModeHardlink {
		return nil
	}
	roots, err := s.store.ListRootFolders(ctx, string(MediaMovie))
	if err != nil {
		return []string{fmt.Sprintf("could not check root folders: %v", err)}
	}
	tvRoots, err := s.store.ListRootFolders(ctx, string(MediaTV))
	if err != nil {
		return []string{fmt.Sprintf("could not check root folders: %v", err)}
	}
	var warnings []string
	for _, r := range append(roots, tvRoots...) {
		same, err := importer.SameFilesystem(set.CompleteDir, r.Path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not compare filesystems of %s and %s: %v", set.CompleteDir, r.Path, err))
			continue
		}
		if !same {
			warnings = append(warnings, fmt.Sprintf("%s and root folder %s are on different filesystems: imports will be copied, using double the space", set.CompleteDir, r.Path))
		}
	}
	return warnings
}

// QualitySettings are the global release-selection limits.
type QualitySettings struct {
	MinResolution int   `json:"min_resolution"`
	MaxResolution int   `json:"max_resolution"`
	MaxSizeMB     int64 `json:"max_size_mb"` // 0 = unlimited
}

// QualitySettings returns the current quality limits.
func (s *Service) QualitySettings(ctx context.Context) (QualitySettings, error) {
	row, err := s.store.GetQualitySettings(ctx)
	if err != nil {
		return QualitySettings{}, fmt.Errorf("load quality settings: %w", err)
	}
	return QualitySettings{MinResolution: int(row.MinResolution), MaxResolution: int(row.MaxResolution), MaxSizeMB: row.MaxSizeMB}, nil
}

var validResolutions = map[int]bool{480: true, 720: true, 1080: true, 2160: true}

// UpdateQualitySettings validates and saves quality limits.
func (s *Service) UpdateQualitySettings(ctx context.Context, in QualitySettings) (QualitySettings, error) {
	if !validResolutions[in.MinResolution] {
		return QualitySettings{}, invalid("min_resolution", "must be one of 480, 720, 1080, 2160")
	}
	if !validResolutions[in.MaxResolution] {
		return QualitySettings{}, invalid("max_resolution", "must be one of 480, 720, 1080, 2160")
	}
	if in.MinResolution > in.MaxResolution {
		return QualitySettings{}, invalid("min_resolution", "must not exceed max_resolution")
	}
	if in.MaxSizeMB < 0 {
		return QualitySettings{}, invalid("max_size_mb", "must be 0 (unlimited) or positive")
	}
	row, err := s.store.UpdateQualitySettings(ctx, db.UpdateQualitySettingsParams{
		MinResolution: int64(in.MinResolution), MaxResolution: int64(in.MaxResolution), MaxSizeMB: in.MaxSizeMB,
	})
	if err != nil {
		return QualitySettings{}, fmt.Errorf("save quality settings: %w", err)
	}
	return QualitySettings{MinResolution: int(row.MinResolution), MaxResolution: int(row.MaxResolution), MaxSizeMB: row.MaxSizeMB}, nil
}

// RootFolder is a library location.
type RootFolder struct {
	ID        int64     `json:"id"`
	MediaType MediaType `json:"media_type"`
	Path      string    `json:"path"`
	IsDefault bool      `json:"is_default"`
}

func rootFolderFromRow(r db.RootFolder) RootFolder {
	return RootFolder{ID: r.ID, MediaType: MediaType(r.MediaType), Path: r.Path, IsDefault: r.IsDefault}
}

// RootFolders lists root folders for a media type.
func (s *Service) RootFolders(ctx context.Context, mt MediaType) ([]RootFolder, error) {
	if !mt.valid() {
		return nil, invalid("media_type", "must be movie or tv")
	}
	rows, err := s.store.ListRootFolders(ctx, string(mt))
	if err != nil {
		return nil, fmt.Errorf("list root folders: %w", err)
	}
	out := make([]RootFolder, len(rows))
	for i, r := range rows {
		out[i] = rootFolderFromRow(r)
	}
	return out, nil
}

// AddRootFolder validates and adds a root folder. The first folder of a
// media type becomes its default.
func (s *Service) AddRootFolder(ctx context.Context, mt MediaType, path string) (RootFolder, error) {
	if !mt.valid() {
		return RootFolder{}, invalid("media_type", "must be movie or tv")
	}
	path = filepath.Clean(path)
	if err := importer.CheckWritableDir(path); err != nil {
		return RootFolder{}, invalid("path", "%v", err)
	}
	var created db.RootFolder
	err := s.store.WithTx(ctx, func(q *db.Queries) error {
		existing, err := q.ListRootFolders(ctx, string(mt))
		if err != nil {
			return err
		}
		for _, r := range existing {
			if r.Path == path {
				return conflict("root folder %s already exists", path)
			}
		}
		created, err = q.CreateRootFolder(ctx, db.CreateRootFolderParams{MediaType: string(mt), Path: path, IsDefault: len(existing) == 0})
		return err
	})
	if err != nil {
		return RootFolder{}, err
	}
	return rootFolderFromRow(created), nil
}

// SetDefaultRootFolder makes id the default for its media type.
func (s *Service) SetDefaultRootFolder(ctx context.Context, id int64) error {
	return s.store.WithTx(ctx, func(q *db.Queries) error {
		r, err := q.GetRootFolder(ctx, id)
		if err != nil {
			return notFound(err, "root folder", id)
		}
		if err := q.ClearDefaultRootFolder(ctx, r.MediaType); err != nil {
			return err
		}
		_, err = q.SetDefaultRootFolder(ctx, id)
		return err
	})
}

// DeleteRootFolder removes a root folder that no movie uses. Files on disk
// are untouched.
func (s *Service) DeleteRootFolder(ctx context.Context, id int64) error {
	return s.store.WithTx(ctx, func(q *db.Queries) error {
		r, err := q.GetRootFolder(ctx, id)
		if err != nil {
			return notFound(err, "root folder", id)
		}
		movies, err := q.CountMoviesInRootFolder(ctx, id)
		if err != nil {
			return err
		}
		series, err := q.CountSeriesInRootFolder(ctx, id)
		if err != nil {
			return err
		}
		if movies+series > 0 {
			return conflict("root folder %s is used by %d movies and %d series", r.Path, movies, series)
		}
		if _, err := q.DeleteRootFolder(ctx, id); err != nil {
			return err
		}
		if !r.IsDefault {
			return nil
		}
		// Promote another folder so the media type keeps a default.
		rest, err := q.ListRootFolders(ctx, r.MediaType)
		if err != nil || len(rest) == 0 {
			return err
		}
		_, err = q.SetDefaultRootFolder(ctx, rest[0].ID)
		return err
	})
}

// defaultRootFolder returns the default root for a media type.
func (s *Service) defaultRootFolder(ctx context.Context, mt MediaType) (db.RootFolder, error) {
	rows, err := s.store.ListRootFolders(ctx, string(mt))
	if err != nil {
		return db.RootFolder{}, fmt.Errorf("list root folders: %w", err)
	}
	for _, r := range rows {
		if r.IsDefault {
			return r, nil
		}
	}
	return db.RootFolder{}, invalid("root_folder_id", "no %s root folder is configured", mt)
}
