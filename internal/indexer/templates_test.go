package indexer

import (
	"context"
	"strings"
	"testing"

	cdef "github.com/Kcchouette/cardigann-go/definition"
)

func TestRawStringLiterals(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"regex argument", `{{ re_replace .Keywords "\b0(\d{1})\b" "$1" }}`, "{{ re_replace .Keywords `\\b0(\\d{1})\\b` \"$1\" }}"},
		{"inside if", `{{ if .Keywords }}{{ re_replace .Keywords "[\s]+" "+" }}{{ else }}{{ end }}`, "{{ if .Keywords }}{{ re_replace .Keywords `[\\s]+` \"+\" }}{{ else }}{{ end }}"},
		{"no backslash", `{{ join .Categories "," }}`, `{{ join .Categories "," }}`},
		{"text outside actions", `a\d "\d" {{ .Keywords }}`, `a\d "\d" {{ .Keywords }}`},
		{"literal holding a backtick", "{{ re_replace .Keywords \"`\\d\" \"\" }}", "{{ re_replace .Keywords \"`\\d\" \"\" }}"},
		{"raw string kept", "{{ re_replace .Keywords `\\d` \"\" }}", "{{ re_replace .Keywords `\\d` \"\" }}"},
		{"closing braces in a literal", `{{ re_replace .Keywords "\d}}" "" }}\d`, "{{ re_replace .Keywords `\\d}}` \"\" }}\\d"},
		{"unterminated", `{{ re_replace .Keywords "\d`, `{{ re_replace .Keywords "\d`},
	}
	for _, tt := range tests {
		if got := rawStringLiterals(tt.in); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
}

// A search input whose regex argument Go cannot read as a string literal
// (Prowlarr's Nyaa definition strips a leading zero this way) renders,
// with the regex applied as written.
func TestRegexArgumentInSearchInput(t *testing.T) {
	s := newSite(t)
	yml := strings.ReplaceAll(fictionalDefinition, "SITE", s.URL)
	yml = strings.Replace(yml, `q: "{{ .Keywords }}"`, `q: "{{ re_replace .Keywords \"\\b0(\\d{1})\\b\" \"$1\" }}"`, 1)
	def, err := cdef.Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	d := newDefinition(def)
	v, err := d.Normalize(goodSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	srch, err := d.NewSearcher(v, s.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srch.Search(context.Background(), Query{Type: SearchGeneric, Q: "Example Show 05"}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	q := s.queries[0]
	s.mu.Unlock()
	if !strings.Contains(q, "q=Example+Show+5&") && !strings.HasSuffix(q, "q=Example+Show+5") {
		t.Errorf("query = %s", q)
	}
}
