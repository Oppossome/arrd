package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/auth"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/vpnconfig"
	"generic-arr-project/internal/store/db"
)

// SetupStatus tells a client whether onboarding is needed, with suggested
// defaults for the wizard.
type SetupStatus struct {
	SetupComplete bool           `json:"setup_complete"`
	Suggested     SuggestedPaths `json:"suggested"`
}

// SuggestedPaths are defaults under the data directory.
type SuggestedPaths struct {
	IncompleteDir string `json:"incomplete_dir"`
	CompleteDir   string `json:"complete_dir"`
	MovieRoot     string `json:"movie_root"`
	TVRoot        string `json:"tv_root"`
}

// SetupStatus reports onboarding state.
func (s *Service) SetupStatus() SetupStatus {
	return SetupStatus{
		SetupComplete: s.auth.SetupComplete(),
		Suggested: SuggestedPaths{
			IncompleteDir: filepath.Join(s.dataDir, "downloads", "incomplete"),
			CompleteDir:   filepath.Join(s.dataDir, "downloads", "complete"),
			MovieRoot:     filepath.Join(s.dataDir, "media", "Movies"),
			TVRoot:        filepath.Join(s.dataDir, "media", "TV"),
		},
	}
}

// SetupInput is everything the onboarding wizard collects.
type SetupInput struct {
	AdminPassword string                    `json:"admin_password"`
	TMDBAPIKey    string                    `json:"tmdb_api_key"`
	Downloads     settings.DownloadSettings `json:"downloads"`
	VPN           *vpnconfig.VPNInput       `json:"vpn,omitempty"` // nil = no VPN
	MovieRoot     string                    `json:"movie_root"`
	TVRoot        string                    `json:"tv_root"`
	Indexers      []indexers.IndexerInput   `json:"indexers"`
	// CreateMissing creates download and library directories that do not
	// exist yet.
	CreateMissing bool `json:"create_missing"`
}

// SetupResult is returned once, when onboarding completes.
type SetupResult struct {
	APIKey string `json:"api_key"`
	// SessionToken signs the browser in; the HTTP layer moves it into a
	// cookie and never puts it in a response body.
	SessionToken string   `json:"-"`
	Warnings     []string `json:"warnings,omitempty"`
	RuntimeError string   `json:"runtime_error,omitempty"`
}

// errSetupDone is returned by onboarding calls after setup.
func errSetupDone() error { return apperr.Conflict("setup is already complete") }

// CompleteSetup validates the whole wizard input, saves it in one
// transaction, generates the API key and starts downloads.
func (s *Service) CompleteSetup(ctx context.Context, in SetupInput) (SetupResult, error) {
	if s.auth.SetupComplete() {
		return SetupResult{}, errSetupDone()
	}
	if err := auth.ValidatePassword("admin_password", in.AdminPassword); err != nil {
		return SetupResult{}, err
	}
	in.TMDBAPIKey = strings.TrimSpace(in.TMDBAPIKey)
	if in.TMDBAPIKey == "" {
		return SetupResult{}, apperr.Invalid("tmdb_api_key", "is required")
	}
	downloads, err := settings.ValidateDownloadSettings(in.Downloads, in.CreateMissing)
	if err != nil {
		return SetupResult{}, apperr.PrefixField("downloads.", err)
	}
	type root struct {
		mt   settings.MediaType
		path string
	}
	var roots []root
	for _, r := range []root{{settings.MediaMovie, in.MovieRoot}, {settings.MediaTV, in.TVRoot}} {
		if strings.TrimSpace(r.path) == "" {
			continue
		}
		r.path = filepath.Clean(r.path)
		if err := settings.CheckDir(r.path, in.CreateMissing); err != nil {
			return SetupResult{}, apperr.Invalid(string(r.mt)+"_root", "%v", err)
		}
		roots = append(roots, r)
	}
	if len(roots) == 0 {
		return SetupResult{}, apperr.Invalid("movie_root", "a movie or TV library folder is required")
	}
	indexerRows := make([]indexers.Params, 0, len(in.Indexers))
	names, sites := map[string]bool{}, map[string]bool{}
	for i, ixIn := range in.Indexers {
		p, err := s.indexers.ValidateNew(ixIn)
		if err != nil {
			return SetupResult{}, apperr.PrefixField(fmt.Sprintf("indexers[%d].", i), err)
		}
		if sites[p.Definition()] {
			return SetupResult{}, apperr.Invalid(fmt.Sprintf("indexers[%d].definition", i), "site %q is added twice", p.Definition())
		}
		if names[p.Name()] {
			return SetupResult{}, apperr.Invalid(fmt.Sprintf("indexers[%d].name", i), "duplicate name %q", p.Name())
		}
		names[p.Name()], sites[p.Definition()] = true, true
		indexerRows = append(indexerRows, p)
	}
	// Checked last: the provider may be asked over the network.
	var vpnParams *db.UpsertVPNSettingsParams
	if in.VPN != nil {
		p, err := s.vpn.SetupParams(ctx, *in.VPN)
		if err != nil {
			return SetupResult{}, err
		}
		vpnParams = &p
	}
	apiKey, err := auth.NewAPIKey()
	if err != nil {
		return SetupResult{}, err
	}
	passwordHash, err := auth.HashPassword(in.AdminPassword)
	if err != nil {
		return SetupResult{}, err
	}
	var session string

	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		if _, err := q.CreateAppSettings(ctx, db.CreateAppSettingsParams{APIKey: apiKey, TMDBAPIKey: in.TMDBAPIKey, PasswordHash: passwordHash}); err != nil {
			if apperr.IsUniqueViolation(err) {
				return errSetupDone()
			}
			return err
		}
		if vpnParams != nil {
			if err := q.UpsertVPNSettings(ctx, *vpnParams); err != nil {
				return err
			}
		}
		if _, err := q.UpsertDownloadSettings(ctx, db.UpsertDownloadSettingsParams{
			IncompleteDir: downloads.IncompleteDir, CompleteDir: downloads.CompleteDir,
			ImportMode: string(downloads.ImportMode),
		}); err != nil {
			return err
		}
		for _, r := range roots {
			if _, err := q.CreateRootFolder(ctx, db.CreateRootFolderParams{MediaType: string(r.mt), Path: r.path, IsDefault: true}); err != nil {
				return err
			}
		}
		for _, p := range indexerRows {
			if _, err := q.CreateIndexer(ctx, p.CreateParams()); err != nil {
				return err
			}
		}
		session, err = s.auth.CreateSession(ctx, q)
		return err
	})
	if err != nil {
		return SetupResult{}, fmt.Errorf("save setup: %w", err)
	}
	s.auth.SetupCompleted(apiKey)
	vpnProvider := ""
	if vpnParams != nil {
		vpnProvider = vpnParams.Provider
	}
	s.log.Info("setup complete", "vpn_provider", vpnProvider)

	out := SetupResult{APIKey: apiKey, SessionToken: session, Warnings: s.settings.HardlinkWarnings(ctx, downloads)}
	if err := s.engine.Apply(ctx); err != nil {
		out.RuntimeError = err.Error()
	}
	return out, nil
}

// PathsInput is checked by ValidatePaths.
type PathsInput struct {
	Downloads     settings.DownloadSettings `json:"downloads"`
	MovieRoot     string                    `json:"movie_root"`
	TVRoot        string                    `json:"tv_root"`
	CreateMissing bool                      `json:"create_missing"`
}

// PathsCheck lists problems per field, plus warnings that do not block.
type PathsCheck struct {
	Valid    bool              `json:"valid"`
	Errors   map[string]string `json:"errors"`
	Warnings []string          `json:"warnings"`
}

// ValidatePaths checks wizard paths without saving anything.
func ValidatePaths(in PathsInput) PathsCheck {
	out := PathsCheck{Errors: map[string]string{}, Warnings: []string{}}
	dl := in.Downloads
	if _, err := settings.ValidateDownloadSettings(dl, in.CreateMissing); err != nil {
		if ve, ok := err.(*apperr.ValidationError); ok {
			out.Errors["downloads."+ve.Field] = ve.Message
		} else {
			out.Errors["downloads"] = err.Error()
		}
	}
	for field, p := range map[string]string{"movie_root": in.MovieRoot, "tv_root": in.TVRoot} {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := settings.CheckDir(filepath.Clean(p), in.CreateMissing); err != nil {
			out.Errors[field] = err.Error()
			continue
		}
		if dl.CompleteDir == "" {
			continue
		}
		if same, err := importer.SameFilesystem(dl.CompleteDir, p); err == nil && !same {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s and %s are on different filesystems: imports will be copied, using double the space", dl.CompleteDir, p))
		}
	}
	if strings.TrimSpace(in.MovieRoot) == "" && strings.TrimSpace(in.TVRoot) == "" {
		out.Errors["movie_root"] = "a movie or TV library folder is required"
	}
	out.Valid = len(out.Errors) == 0
	return out
}
