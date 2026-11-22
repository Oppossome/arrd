package library

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"generic-arr-project/internal/importer"
	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/netx"
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
		SetupComplete: s.SetupComplete(),
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
	AdminPassword string           `json:"admin_password"`
	TMDBAPIKey    string           `json:"tmdb_api_key"`
	Downloads     DownloadSettings `json:"downloads"`
	VPN           *VPNInput        `json:"vpn,omitempty"` // nil = no VPN
	MovieRoot     string           `json:"movie_root"`
	TVRoot        string           `json:"tv_root"`
	Indexers      []IndexerInput   `json:"indexers"`
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
func errSetupDone() error { return conflict("setup is already complete") }

// CompleteSetup validates the whole wizard input, saves it in one
// transaction, generates the API key and starts downloads.
func (s *Service) CompleteSetup(ctx context.Context, in SetupInput) (SetupResult, error) {
	if s.SetupComplete() {
		return SetupResult{}, errSetupDone()
	}
	if err := validatePassword("admin_password", in.AdminPassword); err != nil {
		return SetupResult{}, err
	}
	in.TMDBAPIKey = strings.TrimSpace(in.TMDBAPIKey)
	if in.TMDBAPIKey == "" {
		return SetupResult{}, invalid("tmdb_api_key", "is required")
	}
	downloads, err := validateDownloadSettings(in.Downloads, in.CreateMissing)
	if err != nil {
		return SetupResult{}, prefixField("downloads.", err)
	}
	type root struct {
		mt   MediaType
		path string
	}
	var roots []root
	for _, r := range []root{{MediaMovie, in.MovieRoot}, {MediaTV, in.TVRoot}} {
		if strings.TrimSpace(r.path) == "" {
			continue
		}
		r.path = filepath.Clean(r.path)
		if err := checkDir(r.path, in.CreateMissing); err != nil {
			return SetupResult{}, invalid(string(r.mt)+"_root", "%v", err)
		}
		roots = append(roots, r)
	}
	if len(roots) == 0 {
		return SetupResult{}, invalid("movie_root", "a movie or TV library folder is required")
	}
	indexers := make([]indexerParams, 0, len(in.Indexers))
	names, sites := map[string]bool{}, map[string]bool{}
	for i, ixIn := range in.Indexers {
		p, err := validateIndexer(ixIn, nil, s.definitions.Load())
		if err != nil {
			return SetupResult{}, prefixField(fmt.Sprintf("indexers[%d].", i), err)
		}
		if sites[p.definition] {
			return SetupResult{}, invalid(fmt.Sprintf("indexers[%d].definition", i), "site %q is added twice", p.definition)
		}
		if names[p.name] {
			return SetupResult{}, invalid(fmt.Sprintf("indexers[%d].name", i), "duplicate name %q", p.name)
		}
		names[p.name], sites[p.definition] = true, true
		indexers = append(indexers, p)
	}
	// Checked last: the provider may be asked over the network.
	var vpnParams *db.UpsertVPNSettingsParams
	if in.VPN != nil {
		def, values, err := s.prepareVPN(ctx, *in.VPN, nil)
		if err != nil {
			return SetupResult{}, prefixField("vpn.", err)
		}
		p, err := upsertVPNParams(def, values)
		if err != nil {
			return SetupResult{}, err
		}
		vpnParams = &p
	}
	apiKey, err := newAPIKey()
	if err != nil {
		return SetupResult{}, err
	}
	passwordHash, err := hashPassword(in.AdminPassword)
	if err != nil {
		return SetupResult{}, err
	}
	var session string

	err = s.store.WithTx(ctx, func(q *db.Queries) error {
		if _, err := q.CreateAppSettings(ctx, db.CreateAppSettingsParams{APIKey: apiKey, TMDBAPIKey: in.TMDBAPIKey, PasswordHash: passwordHash}); err != nil {
			if isUniqueViolation(err) {
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
		for _, p := range indexers {
			if _, err := q.CreateIndexer(ctx, p.createParams()); err != nil {
				return err
			}
		}
		session, err = s.createSession(ctx, q)
		return err
	})
	if err != nil {
		return SetupResult{}, fmt.Errorf("save setup: %w", err)
	}
	s.apiKey.Store(&apiKey)
	s.setupDone.Store(true)
	vpnProvider := ""
	if vpnParams != nil {
		vpnProvider = vpnParams.Provider
	}
	s.log.Info("setup complete", "vpn_provider", vpnProvider)

	out := SetupResult{APIKey: apiKey, SessionToken: session, Warnings: s.hardlinkWarnings(ctx, downloads)}
	if err := s.ApplyRuntime(ctx); err != nil {
		out.RuntimeError = err.Error()
	}
	return out, nil
}

// prefixField namespaces a validation error's field ("downloads.complete_dir").
func prefixField(prefix string, err error) error {
	if ve, ok := err.(*ValidationError); ok {
		return &ValidationError{Field: prefix + ve.Field, Message: ve.Message}
	}
	return err
}

func newAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// PathsInput is checked by ValidatePaths.
type PathsInput struct {
	Downloads     DownloadSettings `json:"downloads"`
	MovieRoot     string           `json:"movie_root"`
	TVRoot        string           `json:"tv_root"`
	CreateMissing bool             `json:"create_missing"`
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
	if _, err := validateDownloadSettings(dl, in.CreateMissing); err != nil {
		if ve, ok := err.(*ValidationError); ok {
			out.Errors["downloads."+ve.Field] = ve.Message
		} else {
			out.Errors["downloads"] = err.Error()
		}
	}
	for field, p := range map[string]string{"movie_root": in.MovieRoot, "tv_root": in.TVRoot} {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := checkDir(filepath.Clean(p), in.CreateMissing); err != nil {
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

// ValidateTMDBKey checks a TMDB API key with one request.
func (s *Service) ValidateTMDBKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return invalid("tmdb_api_key", "is required")
	}
	c := &metadata.TMDB{APIKey: key, BaseURL: s.tmdbBaseURL, HTTP: netx.HTTPClient(s.direct, tmdbTimeout)}
	if err := c.Validate(ctx); err != nil {
		return invalid("tmdb_api_key", "TMDB rejected the key: %v", err)
	}
	return nil
}
