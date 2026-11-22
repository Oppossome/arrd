package indexer

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// uploadDefinition is fictionalDefinition with its own id and name.
func uploadDefinition(id, name string) string {
	good := strings.ReplaceAll(fictionalDefinition, "SITE", "http://"+id+".invalid")
	return strings.NewReplacer("id: examplebay", "id: "+id, "name: Example Bay", "name: "+name).Replace(good)
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImportZipPrefersNewestVersion(t *testing.T) {
	dir := t.TempDir()
	data := makeZip(t, map[string]string{
		"Indexers-main/.github/workflows/ci.yml":   "on: push",
		"Indexers-main/definitions/v10/oldbay.yml": uploadDefinition("oldbay", "Old Bay"),
		"Indexers-main/definitions/v10/sitea.yml":  uploadDefinition("sitea", "Site A v10"),
		"Indexers-main/definitions/v11/sitea.yml":  uploadDefinition("sitea", "Site A v11"),
		"Indexers-main/definitions/v11/siteb.yaml": uploadDefinition("siteb", "Site B"),
		"Indexers-main/definitions/v11/broken.yml": "id: [unclosed",
		"Indexers-main/definitions/v11/again.yml":  uploadDefinition("siteb", "Site B again"),
		"Indexers-main/definitions/v11/notes.txt":  "ignored",
		"Indexers-main/definitions/v12/sitec.yml":  uploadDefinition("sitec", "Site C"),
		"Indexers-main/definitions/v12/sitea.yml":  "id: [newer schema arrd cannot read",
		"__MACOSX/definitions/v11/._sitea.yml":     "resource fork",
	})
	res, err := Import(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	// ci.yml is outside the versioned folders; oldbay is only in v10.
	if !slices.Equal(res.Saved, []string{"oldbay", "sitea", "siteb", "sitec"}) {
		t.Errorf("saved = %v", res.Saved)
	}
	// Newest first: v12's unreadable sitea, then v11's broken.yml and its
	// second siteb ("again" sorts first, so siteb.yaml is the duplicate).
	// v10's sitea is shadowed by v11's quietly.
	want := []string{"v12/sitea.yml", "v11/broken.yml", "v11/siteb.yaml"}
	if len(res.Errors) != len(want) {
		t.Fatalf("errors = %v", res.Errors)
	}
	for i, w := range want {
		if !strings.Contains(res.Errors[i].Error(), w) {
			t.Errorf("errors[%d] = %v, want %s", i, res.Errors[i], w)
		}
	}
	catalog, errs := LoadDir(dir)
	if len(errs) != 0 || len(catalog.List()) != 4 {
		t.Fatalf("loaded = %+v, %v", catalog.List(), errs)
	}
	for id, name := range map[string]string{"sitea": "Site A v11", "siteb": "Site B again", "sitec": "Site C", "oldbay": "Old Bay"} {
		if d, err := catalog.Get(id); err != nil || d.Name != name {
			t.Errorf("%s = %q, %v; want %q", id, d.Name, err, name)
		}
	}
}

func TestImportZipWithoutVersions(t *testing.T) {
	dir := t.TempDir()
	data := makeZip(t, map[string]string{
		"sitea.yml":      uploadDefinition("sitea", "Site A"),
		"more/siteb.yml": uploadDefinition("siteb", "Site B"),
	})
	res, err := Import(dir, data)
	if err != nil || !slices.Equal(res.Saved, []string{"sitea", "siteb"}) || len(res.Errors) != 0 {
		t.Fatalf("import = %+v, %v", res, err)
	}
}

func TestImportReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	// Added by hand under another name; an upload of the same id replaces it.
	if err := os.WriteFile(filepath.Join(dir, "my-site.yml"), []byte(uploadDefinition("sitea", "Site A")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(dir, []byte(uploadDefinition("sitea", "Site A v2"))); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "my-site.yml" {
		t.Fatalf("files = %v, %v", entries, err)
	}
	catalog, _ := LoadDir(dir)
	if d, err := catalog.Get("sitea"); err != nil || d.Name != "Site A v2" {
		t.Errorf("definition = %+v, %v", d, err)
	}
}

func TestImportRejectsUnusableUploads(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"not yaml":         []byte("id: [unclosed"),
		"no id":            []byte("name: Nothing"),
		"empty zip":        makeZip(t, map[string]string{"readme.md": "hello"}),
		"only broken":      makeZip(t, map[string]string{"v11/broken.yml": "id: [unclosed"}),
		"unsafe file name": []byte(uploadDefinition("../escape", "Escape")),
	}
	for name, data := range cases {
		if _, err := Import(dir, data); !errors.Is(err, ErrInvalidUpload) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("files written: %v", entries)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	if _, err := Import(dir, []byte(uploadDefinition("sitea", "Site A"))); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, "nowhere"); !errors.Is(err, ErrUnknownDefinition) {
		t.Errorf("remove unknown = %v", err)
	}
	if err := Remove(dir, "sitea"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sitea.yml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file still there: %v", err)
	}
}
