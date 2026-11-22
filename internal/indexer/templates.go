package indexer

import (
	"maps"
	"slices"
	"strings"

	cdef "github.com/Kcchouette/cardigann-go/definition"
)

// Prowlarr reads a double-quoted argument in a template action verbatim
// ({{ re_replace .Keywords "\b0(\d{1})\b" "$1" }} is a regex with word
// boundaries), but the library parses templates with Go's text/template,
// which reads it as a Go string literal: "\d" fails to parse, and "\b"
// would become a backspace. adaptTemplates rewrites such arguments as Go
// raw strings (`\b0(\d{1})\b`), which keep every character as written.
//
// Only search paths and inputs are rewritten: across Prowlarr's
// definitions, that is the only place such arguments appear.
// TODO: remove once cardigann-go's template preprocessing does this.
func adaptTemplates(def cdef.IndexerDefinition) cdef.IndexerDefinition {
	def.Search.Inputs = rawLiteralsInValues(def.Search.Inputs)
	paths := slices.Clone(def.Search.Paths)
	for i := range paths {
		paths[i].Path = rawStringLiterals(paths[i].Path)
		paths[i].Inputs = rawLiteralsInValues(paths[i].Inputs)
	}
	def.Search.Paths = paths
	return def
}

func rawLiteralsInValues(inputs map[string]string) map[string]string {
	out := maps.Clone(inputs)
	for k, v := range out {
		out[k] = rawStringLiterals(v)
	}
	return out
}

// rawStringLiterals rewrites the double-quoted string literals inside the
// template actions of src that contain a backslash as raw strings, with
// the same characters between the delimiters. Text outside actions, and a
// literal that itself contains a backtick (which a raw string cannot
// hold), are left as they are.
func rawStringLiterals(src string) string {
	if !strings.Contains(src, "{{") || !strings.Contains(src, `\`) {
		return src
	}
	var b strings.Builder
	rest := src
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:open+2])
		rest = rest[open+2+rewriteAction(&b, rest[open+2:]):]
	}
}

// rewriteAction copies the template action at the start of src (just
// after its "{{") to b, through its closing "}}", rewriting its string
// literals. It returns how many bytes of src it consumed.
func rewriteAction(b *strings.Builder, src string) int {
	i := 0
	for i < len(src) {
		switch {
		case strings.HasPrefix(src[i:], "}}"):
			b.WriteString("}}")
			return i + 2
		case src[i] == '"':
			end, closed := quotedEnd(src, i, '"')
			if content := src[i+1 : max(end-1, i+1)]; closed && strings.Contains(content, `\`) && !strings.Contains(content, "`") {
				b.WriteByte('`')
				b.WriteString(content)
				b.WriteByte('`')
			} else {
				b.WriteString(src[i:end])
			}
			i = end
		case src[i] == '\'':
			end, _ := quotedEnd(src, i, '\'')
			b.WriteString(src[i:end])
			i = end
		case src[i] == '`':
			end := len(src)
			if j := strings.IndexByte(src[i+1:], '`'); j >= 0 {
				end = i + 1 + j + 1
			}
			b.WriteString(src[i:end])
			i = end
		default:
			b.WriteByte(src[i])
			i++
		}
	}
	return i
}

// quotedEnd returns the index just past the literal that opens with quote
// at src[start], skipping backslash-escaped characters, and whether the
// literal is closed (when it is not, the index is len(src)).
func quotedEnd(src string, start int, quote byte) (int, bool) {
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i + 1, true
		}
	}
	return len(src), false
}
