package provider

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func testDef() Definition {
	return Definition{
		ID: "fictional", Name: "Fictional VPN",
		Fields: []Field{
			{Key: "user", Label: "User", Kind: KindText, Required: true},
			{Key: "pass", Label: "Pass", Kind: KindPassword, Required: true, Secret: true},
			{Key: "key", Label: "Key", Kind: KindTextarea, Secret: true},
			{Key: "fast", Label: "Fast", Kind: KindToggle},
		},
		Parse: func(Values) (Provider, error) { return nil, nil },
		Check: func(context.Context, Values) error { return nil },
	}
}

func TestNormalize(t *testing.T) {
	d := testDef()
	got, err := d.Normalize(Values{"user": "  ann  ", "pass": " spaced secret ", "fast": "true", "key": ""})
	if err != nil {
		t.Fatal(err)
	}
	// Non-secrets are trimmed, secrets kept byte for byte, empties dropped.
	if got["user"] != "ann" || got["pass"] != " spaced secret " || got["fast"] != "true" {
		t.Errorf("normalized = %q", got)
	}
	if _, ok := got["key"]; ok {
		t.Error("empty value kept")
	}

	for name, c := range map[string]struct {
		in    Values
		field string
	}{
		"missing required": {Values{"user": "ann"}, "pass"},
		"blank required":   {Values{"user": "  ", "pass": "x"}, "user"},
		"unknown key":      {Values{"user": "ann", "pass": "x", "region": "zz"}, "region"},
		"bad toggle":       {Values{"user": "ann", "pass": "x", "fast": "yes"}, "fast"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := d.Normalize(c.in)
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Field != c.field {
				t.Fatalf("err = %v, want field error on %s", err, c.field)
			}
		})
	}
}

func TestMergeSecretsAndRedact(t *testing.T) {
	d := testDef()
	saved := Values{"user": "ann", "pass": "old-secret", "key": "old-key"}

	merged := d.MergeSecrets(Values{"user": "bob", "key": "new-key"}, saved)
	if merged["user"] != "bob" || merged["pass"] != "old-secret" || merged["key"] != "new-key" {
		t.Errorf("merged = %q", merged)
	}
	// Non-secret fields are never filled from saved values.
	if m := d.MergeSecrets(Values{"pass": "p"}, saved); m["user"] != "" {
		t.Errorf("non-secret carried over: %q", m)
	}

	public, set := d.Redact(Values{"user": "ann", "pass": "s3cret", "fast": "true"})
	if public["pass"] != "" || public["user"] != "ann" || public["fast"] != "true" || !slices.Equal(set, []string{"pass"}) {
		t.Errorf("redact = %q, secrets set %v", public, set)
	}
}

func TestRegistry(t *testing.T) {
	d := testDef()
	reg, err := NewRegistry(d)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := reg.Get("fictional"); err != nil || got.Name != d.Name {
		t.Errorf("get = %+v, %v", got, err)
	}
	if _, err := reg.Get("other"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("unknown provider err = %v", err)
	}
	if _, err := NewRegistry(d, d); err == nil {
		t.Error("duplicate id accepted")
	}
	noOpts := testDef()
	noOpts.Fields = append(noOpts.Fields, Field{Key: "region", Kind: KindSelect, Options: true})
	if _, err := NewRegistry(noOpts); err == nil {
		t.Error("options field without an Options func accepted")
	}
	noParse := testDef()
	noParse.Parse = nil
	if _, err := NewRegistry(noParse); err == nil {
		t.Error("definition without Parse accepted")
	}
}
