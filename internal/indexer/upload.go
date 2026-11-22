package indexer

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	cdef "github.com/Kcchouette/cardigann-go/definition"
)

// MaxUploadBytes bounds an uploaded definition file or zip.
const MaxUploadBytes = 64 << 20

// maxUnzippedBytes bounds what the definition files in a zip unpack to.
const maxUnzippedBytes = 256 << 20

var (
	// ErrInvalidUpload means an upload is neither a definition nor a zip
	// holding any.
	ErrInvalidUpload = errors.New("invalid definition upload")
)

// fileIDPattern is the ids that are safe to use as a file name.
var fileIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// versionDirPattern matches Prowlarr's per-schema folders (definitions/v11).
var versionDirPattern = regexp.MustCompile(`^v(\d+)$`)

// ImportResult reports an upload of definitions.
type ImportResult struct {
	Saved  []string // ids written, sorted
	Errors []error  // files in the upload that were skipped
}

// Import adds the definitions in data to dir: either one definition file,
// or a zip whose *.yml / *.yaml files (at any depth) are definitions. When
// the zip has versioned folders (definitions/v11, v12 … as in Prowlarr's
// Indexers repository), only files in those are used, and a site in
// several versions comes from the newest that parses. Each definition
// is written as its own file, replacing the file that already defines its
// id. Problems with single files in a zip are returned in Errors; an
// upload with nothing usable fails with ErrInvalidUpload.
func Import(dir string, data []byte) (ImportResult, error) {
	if dir == "" {
		return ImportResult{}, errors.New("no definitions directory is configured")
	}
	var defs []uploadedDefinition
	var errs []error
	if isZip(data) {
		var err error
		if defs, errs, err = definitionsInZip(data); err != nil {
			return ImportResult{}, err
		}
	} else {
		def, err := parseDefinition(data)
		if err != nil {
			return ImportResult{}, fmt.Errorf("%w: not a definition or a zip: %v", ErrInvalidUpload, err)
		}
		defs = []uploadedDefinition{{name: "upload", def: def, data: data}}
	}
	if len(defs) == 0 {
		return ImportResult{}, fmt.Errorf("%w: the zip has no definition files (*.yml)%s", ErrInvalidUpload, firstError(errs))
	}
	return writeDefinitions(dir, defs, errs)
}

// uploadedDefinition is one parsed file of an upload.
type uploadedDefinition struct {
	name string // path within the upload
	def  cdef.IndexerDefinition
	data []byte
}

func isZip(data []byte) bool {
	return bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06"))
}

// definitionsInZip parses the definition files in a zip. When the zip has
// versioned folders, only files in them are used, newest version first: an
// id already taken by a newer version is skipped quietly, so a site whose
// newest file cannot be parsed falls back to an older one. Two files with
// one id in the same version are an error, like duplicates in the
// definitions directory.
func definitionsInZip(data []byte) ([]uploadedDefinition, []error, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: read zip: %v", ErrInvalidUpload, err)
	}
	var defs []uploadedDefinition
	var errs []error
	type origin struct {
		path    string
		version int
	}
	seen := map[string]origin{}
	var total int64
	for _, vf := range byVersion(zipDefinitionFiles(zr)) {
		body, err := readZipFile(vf.file)
		if err != nil {
			errs = append(errs, &LoadError{Path: vf.file.Name, Err: err})
			continue
		}
		if total += int64(len(body)); total > maxUnzippedBytes {
			return nil, nil, fmt.Errorf("%w: the definitions unpack to more than %d bytes", ErrInvalidUpload, maxUnzippedBytes)
		}
		def, err := parseDefinition(body)
		if err != nil {
			errs = append(errs, &LoadError{Path: vf.file.Name, Err: err})
			continue
		}
		if prev, dup := seen[def.ID]; dup {
			if prev.version == vf.version {
				errs = append(errs, &LoadError{Path: vf.file.Name, Err: fmt.Errorf("id %q is also defined in %s", def.ID, prev.path)})
			}
			continue
		}
		seen[def.ID] = origin{path: vf.file.Name, version: vf.version}
		defs = append(defs, uploadedDefinition{name: vf.file.Name, def: def, data: body})
	}
	return defs, errs, nil
}

// zipDefinitionFiles lists the zip's *.yml / *.yaml files by path, leaving
// out macOS resource forks.
func zipDefinitionFiles(zr *zip.Reader) []*zip.File {
	var files []*zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isDefinitionFile(f.Name) || strings.HasPrefix(f.Name, "__MACOSX/") || strings.HasPrefix(path.Base(f.Name), "._") {
			continue
		}
		files = append(files, f)
	}
	slices.SortFunc(files, func(a, b *zip.File) int { return strings.Compare(a.Name, b.Name) })
	return files
}

// versionedFile is a zip entry and the vN folder it is in (-1 for none).
type versionedFile struct {
	file    *zip.File
	version int
}

// byVersion orders files newest version first (then by path). When any
// file is in a vN folder, files outside one (e.g. .github/workflows) are
// left out.
func byVersion(files []*zip.File) []versionedFile {
	out := make([]versionedFile, 0, len(files))
	versioned := false
	for _, f := range files {
		v := folderVersion(f.Name)
		versioned = versioned || v >= 0
		out = append(out, versionedFile{file: f, version: v})
	}
	if versioned {
		out = slices.DeleteFunc(out, func(vf versionedFile) bool { return vf.version < 0 })
	}
	slices.SortStableFunc(out, func(a, b versionedFile) int { return b.version - a.version })
	return out
}

// folderVersion is N for a file directly in a folder named vN, else -1.
func folderVersion(name string) int {
	m := versionDirPattern.FindStringSubmatch(path.Base(path.Dir(name)))
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}

func readZipFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxDefinitionBytes {
		return nil, fmt.Errorf("larger than %d bytes", maxDefinitionBytes)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	// The header's size is not trusted: read one byte past the limit.
	body, err := io.ReadAll(io.LimitReader(rc, maxDefinitionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDefinitionBytes {
		return nil, fmt.Errorf("larger than %d bytes", maxDefinitionBytes)
	}
	return body, nil
}

// writeDefinitions writes each definition to dir, over the file that
// already defines its id or else as <id>.yml.
func writeDefinitions(dir string, defs []uploadedDefinition, errs []error) (ImportResult, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ImportResult{}, fmt.Errorf("create definitions directory: %w", err)
	}
	current, _ := LoadDir(dir) // unreadable files are reported on reload
	out := ImportResult{Saved: []string{}, Errors: errs}
	for _, u := range defs {
		if !fileIDPattern.MatchString(u.def.ID) {
			out.Errors = append(out.Errors, &LoadError{Path: u.name, Err: fmt.Errorf("id %q cannot be used as a file name", u.def.ID)})
			continue
		}
		target := filepath.Join(dir, u.def.ID+".yml")
		if existing, err := current.Get(u.def.ID); err == nil {
			target = existing.path
		}
		if err := writeFileAtomic(target, u.data); err != nil {
			return out, fmt.Errorf("save definition %q: %w", u.def.ID, err)
		}
		out.Saved = append(out.Saved, u.def.ID)
	}
	if len(out.Saved) == 0 {
		return out, fmt.Errorf("%w: no definition could be saved%s", ErrInvalidUpload, firstError(out.Errors))
	}
	slices.Sort(out.Saved)
	return out, nil
}

// writeFileAtomic replaces path with data, so a reload never sees half a file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Remove deletes the file in dir that defines id.
func Remove(dir, id string) error {
	current, _ := LoadDir(dir)
	def, err := current.Get(id)
	if err != nil {
		return err
	}
	if err := os.Remove(def.path); err != nil {
		return fmt.Errorf("remove definition %q: %w", id, err)
	}
	return nil
}

func firstError(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	return " (" + errs[0].Error() + ")"
}
