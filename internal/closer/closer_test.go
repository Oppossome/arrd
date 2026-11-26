package closer

import (
	"errors"
	"testing"
)

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

func TestJoin(t *testing.T) {
	errWork := errors.New("work failed")
	errClose := errors.New("close failed")
	cases := []struct {
		name     string
		work     error
		close    error
		wantNil  bool
		wantErrs []error
	}{
		{name: "both succeed", wantNil: true},
		{name: "work fails", work: errWork, wantErrs: []error{errWork}},
		{name: "close fails", close: errClose, wantErrs: []error{errClose}},
		{name: "both fail", work: errWork, close: errClose, wantErrs: []error{errWork, errClose}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.work
			Join(&err, closeFunc(func() error { return tc.close }))
			if tc.wantNil {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			for _, want := range tc.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("err = %v, want it to wrap %v", err, want)
				}
			}
		})
	}
}

func TestJoinKeepsErrorIdentityWhenCloseSucceeds(t *testing.T) {
	errWork := errors.New("work failed")
	err := errWork
	Join(&err, closeFunc(func() error { return nil }))
	if err != errWork {
		t.Fatalf("err = %#v, want the original error value", err)
	}
}
