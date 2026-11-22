package indexer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	cdef "github.com/Kcchouette/cardigann-go/definition"
)

// maxDefinitionBytes bounds one definition file.
const maxDefinitionBytes = 1 << 20

// LoadError is a definition file that could not be read.
type LoadError struct {
	Path string
	Err  error
}

func (e *LoadError) Error() string { return e.Path + ": " + e.Err.Error() }
func (e *LoadError) Unwrap() error { return e.Err }

// Catalog is the set of site definitions arrd knows, sorted by name.
type Catalog struct {
	defs []Definition
}

// ErrUnknownDefinition is returned for an id no loaded definition has.
var ErrUnknownDefinition = errors.New("unknown site definition")

// Get returns the definition with id.
func (c *Catalog) Get(id string) (Definition, error) {
	for _, d := range c.defs {
		if d.ID == id {
			return d, nil
		}
	}
	return Definition{}, fmt.Errorf("%w %q", ErrUnknownDefinition, id)
}

// List returns all definitions, sorted by name.
func (c *Catalog) List() []Definition { return slices.Clone(c.defs) }

// LoadDir reads every *.yml / *.yaml definition in dir (not recursing),
// e.g. a checkout of Prowlarr's definitions/v11. Files that cannot be
// parsed are returned as LoadErrors alongside the catalog; a missing dir
// gives an empty catalog.
func LoadDir(dir string) (*Catalog, []error) {
	c := &Catalog{}
	if dir == "" {
		return c, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, []error{fmt.Errorf("read definitions: %w", err)}
	}
	var errs []error
	seen := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !isDefinitionFile(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		def, err := loadFile(path)
		if err != nil {
			errs = append(errs, &LoadError{Path: path, Err: err})
			continue
		}
		if prev, dup := seen[def.ID]; dup {
			errs = append(errs, &LoadError{Path: path, Err: fmt.Errorf("id %q is also defined in %s", def.ID, prev)})
			continue
		}
		seen[def.ID] = path
		d := newDefinition(def)
		d.path = path
		c.defs = append(c.defs, d)
	}
	slices.SortFunc(c.defs, func(a, b Definition) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return c, errs
}

func isDefinitionFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yml" || ext == ".yaml"
}

func loadFile(path string) (cdef.IndexerDefinition, error) {
	f, err := os.Open(path)
	if err != nil {
		return cdef.IndexerDefinition{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return cdef.IndexerDefinition{}, err
	}
	if st.Size() > maxDefinitionBytes {
		return cdef.IndexerDefinition{}, fmt.Errorf("larger than %d bytes", maxDefinitionBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cdef.IndexerDefinition{}, err
	}
	return parseDefinition(data)
}

// parseDefinition parses the contents of one definition file.
func parseDefinition(data []byte) (cdef.IndexerDefinition, error) {
	if len(data) > maxDefinitionBytes {
		return cdef.IndexerDefinition{}, fmt.Errorf("larger than %d bytes", maxDefinitionBytes)
	}
	def, err := cdef.Parse(data)
	if err != nil {
		return cdef.IndexerDefinition{}, err
	}
	if def.ID == "" {
		return cdef.IndexerDefinition{}, errors.New("no id")
	}
	return def, nil
}
