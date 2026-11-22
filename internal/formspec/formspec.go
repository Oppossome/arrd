// Package formspec describes the settings an integration needs as a list
// of fields. The web UI renders a form from them and the values are stored
// as one JSON object, so an integration (a VPN provider, an indexer type)
// needs no schema, API or frontend changes of its own.
package formspec

import (
	"fmt"
	"slices"
	"strings"
)

// FieldKind is how a field is entered.
type FieldKind string

const (
	KindText     FieldKind = "text"
	KindPassword FieldKind = "password"
	KindTextarea FieldKind = "textarea"
	KindSelect   FieldKind = "select"
	KindToggle   FieldKind = "toggle" // value "true" or "false"
)

// Field is one setting an integration needs.
type Field struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Kind        FieldKind `json:"kind"`
	Required    bool      `json:"required,omitempty"`
	Help        string    `json:"help,omitempty"`
	Placeholder string    `json:"placeholder,omitempty"`
	// Default is the value a new form starts with.
	Default string `json:"default,omitempty"`
	// Secret values are write-only: the API never returns them, and an
	// empty value on save keeps the saved one.
	Secret bool `json:"secret,omitempty"`
	// Options means a select's choices are loaded from the server.
	Options bool `json:"options,omitempty"`
	// Choices are a select's fixed choices, when they are known up front.
	Choices []Option `json:"choices,omitempty"`
	// FileAccept lets a textarea be filled from a local file (e.g. ".conf").
	FileAccept string `json:"file_accept,omitempty"`
}

// Option is one choice for a select field.
type Option struct {
	Value string   `json:"value"`
	Label string   `json:"label"`
	Tags  []string `json:"tags,omitempty"` // capabilities, e.g. "port_forward"
}

// Values are settings by field key.
type Values map[string]string

// Bool reads a toggle field.
func (v Values) Bool(key string) (bool, error) {
	switch v[key] {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, &FieldError{Field: key, Message: "must be true or false"}
	}
}

// FieldError is a problem with one field's value.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Message }

// Fieldf builds a FieldError.
func Fieldf(field, format string, args ...any) error {
	return &FieldError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Fields is an integration's settings form.
type Fields []Field

// Get returns the field with key.
func (fs Fields) Get(key string) (Field, bool) {
	i := slices.IndexFunc(fs, func(f Field) bool { return f.Key == key })
	if i < 0 {
		return Field{}, false
	}
	return fs[i], true
}

// Normalize checks values against the fields: no unknown keys, required
// fields present, toggles well-formed, fixed choices respected. Non-secret
// values are trimmed. owner names the integration in errors. It returns a
// new map.
func (fs Fields) Normalize(owner string, v Values) (Values, error) {
	out := make(Values, len(v))
	for k, val := range v {
		f, ok := fs.Get(k)
		if !ok {
			return nil, Fieldf(k, "is not a %s setting", owner)
		}
		if !f.Secret {
			val = strings.TrimSpace(val)
		}
		if val != "" {
			out[k] = val
		}
	}
	for _, f := range fs {
		val := out[f.Key]
		if f.Required && val == "" {
			return nil, Fieldf(f.Key, "is required")
		}
		if f.Kind == KindToggle {
			if _, err := out.Bool(f.Key); err != nil {
				return nil, err
			}
		}
		if val != "" && len(f.Choices) > 0 && !slices.ContainsFunc(f.Choices, func(o Option) bool { return o.Value == val }) {
			return nil, Fieldf(f.Key, "must be one of the listed choices")
		}
	}
	return out, nil
}

// MergeSecrets fills secret fields left empty in v from saved. Call it only
// when v is for the same integration as saved.
func (fs Fields) MergeSecrets(v, saved Values) Values {
	out := make(Values, len(v))
	for k, val := range v {
		out[k] = val
	}
	for _, f := range fs {
		if f.Secret && out[f.Key] == "" && saved[f.Key] != "" {
			out[f.Key] = saved[f.Key]
		}
	}
	return out
}

// Redact splits values into what may be shown (secrets removed) and the
// keys of secret fields that have a saved value.
func (fs Fields) Redact(v Values) (public Values, secretsSet []string) {
	public = Values{}
	for _, f := range fs {
		val, ok := v[f.Key]
		switch {
		case !ok || val == "":
		case f.Secret:
			secretsSet = append(secretsSet, f.Key)
		default:
			public[f.Key] = val
		}
	}
	return public, secretsSet
}
