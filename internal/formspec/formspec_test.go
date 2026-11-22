package formspec

import (
	"errors"
	"testing"
)

func TestNormalizeChoices(t *testing.T) {
	fs := Fields{{Key: "site", Kind: KindSelect, Choices: []Option{{Value: "https://a.invalid/"}, {Value: "https://b.invalid/"}}}}
	if v, err := fs.Normalize("Example", Values{"site": " https://b.invalid/ "}); err != nil || v["site"] != "https://b.invalid/" {
		t.Errorf("listed choice = %v, %v", v, err)
	}
	var fe *FieldError
	if _, err := fs.Normalize("Example", Values{"site": "https://elsewhere.invalid/"}); !errors.As(err, &fe) || fe.Field != "site" {
		t.Errorf("unlisted choice = %v", err)
	}
	// An empty optional select is fine.
	if _, err := fs.Normalize("Example", Values{}); err != nil {
		t.Errorf("empty = %v", err)
	}
}
