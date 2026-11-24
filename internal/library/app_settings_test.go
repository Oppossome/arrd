package library

import "testing"

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"short is fully masked", "fict-key", "****************"},
		{"keeps both ends", "ab00112233445566778899zf", "ab****************zf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskSecret(tt.in); got != tt.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
