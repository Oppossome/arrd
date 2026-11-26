package indexer

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	cdef "github.com/Kcchouette/cardigann-go/definition"

	"generic-arr-project/internal/formspec"
)

// baseURLKey is the setting that picks which of the site's addresses to use.
const baseURLKey = "base_url"

// Definition is one site arrd can search, and the settings it needs. A
// definition arrd cannot run is still listed, with the reason in
// Unsupported.
type Definition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Fields      formspec.Fields `json:"fields"`
	// Unsupported explains why the site cannot be used (e.g. it needs a
	// captcha). Empty for usable definitions.
	Unsupported string `json:"unsupported,omitempty"`
	// RSS means the site can list recent releases (an empty search),
	// which RSS sync relies on.
	RSS bool `json:"rss"`

	def  cdef.IndexerDefinition // adapted for the engine
	path string                 // the file it was loaded from
}

// newDefinition describes def.
func newDefinition(def cdef.IndexerDefinition) Definition {
	// Classified before normalizing: the library's usenet heuristic
	// (IsUsenet) takes any definition whose categories are all numeric
	// Newznab IDs for a usenet one, which normalized torrent sites are.
	reason := unsupported(def)
	def = adaptTemplates(adaptFields(normalizeCategories(def)))
	d := Definition{
		ID:          def.ID,
		Name:        def.Name,
		Description: description(def),
		RSS:         def.Caps.AllowRawSearch,
		def:         def,
	}
	fields, err := settingFields(def)
	if err != nil {
		d.Unsupported = err.Error()
		return d
	}
	d.Fields = fields
	d.Unsupported = reason
	return d
}

// ErrUnsupported is returned for settings of a definition that cannot be used.
var ErrUnsupported = errors.New("site cannot be used")

// Normalize checks settings against the definition's fields and that a
// searcher can be built from them. Problems with a field are
// *formspec.FieldError.
func (d Definition) Normalize(v formspec.Values) (formspec.Values, error) {
	if d.Unsupported != "" {
		return nil, fmt.Errorf("%w: %s: %s", ErrUnsupported, d.Name, d.Unsupported)
	}
	v, err := d.Fields.Normalize(d.Name, v)
	if err != nil {
		return nil, err
	}
	if _, err := d.NewSearcher(v, http.DefaultClient); err != nil {
		return nil, err
	}
	return v, nil
}

// NewSearcher builds a searcher from normalized settings, without network
// access. Every request goes through client's transport (the indexer's
// network).
func (d Definition) NewSearcher(v formspec.Values, client *http.Client) (*Searcher, error) {
	if d.Unsupported != "" {
		return nil, fmt.Errorf("%w: %s: %s", ErrUnsupported, d.Name, d.Unsupported)
	}
	return newSearcher(d.def, v, client)
}

func description(def cdef.IndexerDefinition) string {
	parts := []string{}
	if def.Description != "" {
		parts = append(parts, strings.TrimSpace(def.Description))
	}
	if def.Type != "" {
		parts = append(parts, fmt.Sprintf("(%s site)", def.Type))
	}
	return strings.Join(parts, " ")
}

// unsupported is why arrd cannot run def, or "".
func unsupported(def cdef.IndexerDefinition) string {
	switch {
	case def.ID == "" || def.Name == "":
		return "the definition has no id or name"
	case len(def.Links) == 0:
		return "the definition lists no site address"
	case def.Type == "usenet" || def.Type == "ddl" || def.Type == "streaming" || def.IsUsenet():
		return "only torrent sites are supported"
	case def.Login.Captcha != nil:
		return "signing in needs a captcha"
	}
	for _, s := range def.Settings {
		if s.Type == "info_flaresolverr" {
			return "the site needs FlareSolverr to pass a browser challenge"
		}
	}
	return ""
}

// settingFields turns the definition's settings into form fields, after a
// field choosing the site address.
func settingFields(def cdef.IndexerDefinition) (formspec.Fields, error) {
	links := slices.Concat(def.Links, def.LegacyLinks)
	site := formspec.Field{
		Key: baseURLKey, Label: "Site address", Kind: formspec.KindSelect, Required: true,
		Help: "The address arrd reaches the site at.",
	}
	for _, l := range links {
		site.Choices = append(site.Choices, formspec.Option{Value: l, Label: l})
	}
	if len(def.Links) > 0 {
		site.Default = def.Links[0]
	}
	fields := formspec.Fields{site}
	var info []string
	for _, s := range def.Settings {
		if s.Name == baseURLKey {
			return nil, fmt.Errorf("the definition's setting %q clashes with arrd's own", baseURLKey)
		}
		f := formspec.Field{Key: s.Name, Label: label(s), Default: s.Default}
		switch {
		case s.Type == "text":
			f.Kind = formspec.KindText
		case s.Type == "password":
			f.Kind, f.Secret = formspec.KindPassword, true
		case s.Type == "checkbox":
			f.Kind = formspec.KindToggle
			f.Default = toggleDefault(s.Default)
		case s.Type == "select":
			f.Kind = formspec.KindSelect
			for _, value := range s.Options.Keys() {
				f.Choices = append(f.Choices, formspec.Option{Value: value, Label: s.Options.Value(value)})
			}
		case s.Type == "info" || strings.HasPrefix(s.Type, "info_"):
			// Explanatory text, not a value.
			if s.Label != "" {
				info = append(info, s.Label)
			}
			continue
		default:
			return nil, fmt.Errorf("setting %q has unsupported type %q", s.Name, s.Type)
		}
		// Credentials are what a private site needs to be searched at all.
		f.Required = isCredential(s.Name) && def.Type != "public"
		fields = append(fields, f)
	}
	if len(info) > 0 && len(fields) > 0 {
		fields[0].Help = strings.Join(append([]string{fields[0].Help}, info...), " ")
	}
	return fields, nil
}

func label(s cdef.Setting) string {
	if s.Label != "" {
		return s.Label
	}
	return s.Name
}

func isCredential(name string) bool {
	switch strings.ToLower(name) {
	case "username", "password", "cookie", "apikey", "api_key", "passkey", "pid", "rsskey":
		return true
	}
	return false
}

func toggleDefault(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return "true"
	}
	return "false"
}

// normalizeCategories rewrites category mappings written as Newznab names
// ("Movies/HD", as many definitions do) to numeric IDs, which is
// what the engine matches requested categories against.
func normalizeCategories(def cdef.IndexerDefinition) cdef.IndexerDefinition {
	mappings := slices.Clone(def.Caps.CategoryMappings)
	for i, m := range mappings {
		if id, ok := newznabID(m.Cat); ok {
			mappings[i].Cat = fmt.Sprint(id)
		}
	}
	def.Caps.CategoryMappings = mappings
	if def.Caps.Categories.Len() > 0 {
		var pairs []string
		for _, native := range def.Caps.Categories.Keys() {
			cat := def.Caps.Categories.Value(native)
			if id, ok := newznabID(cat); ok {
				cat = fmt.Sprint(id)
			}
			pairs = append(pairs, native, cat)
		}
		def.Caps.Categories = cdef.StringMapFrom(pairs...)
	}
	def.Caps.Modes = maps.Clone(def.Caps.Modes)
	return def
}
