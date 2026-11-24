package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := checkPassword("s3cret-pass", hash); err != nil || !ok {
		t.Errorf("correct password rejected: %v", err)
	}
	if ok, _ := checkPassword("s3cret-pasS", hash); ok {
		t.Error("wrong password accepted")
	}
	if other, _ := HashPassword("s3cret-pass"); other == hash {
		t.Error("salt is not random")
	}
	if _, err := checkPassword("x", "$bcrypt$nope"); err == nil {
		t.Error("foreign hash format accepted")
	}
}
