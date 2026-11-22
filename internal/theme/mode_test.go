package theme

import "testing"

func TestComputeMode(t *testing.T) {
	cases := []struct {
		name string
		hex  string
		want Mode
	}{
		{"Midnight theme background (near-black)", "#0b0d10", Dark},
		{"Light theme background (near-white)", "#f5f4fb", Light},
		{"pure black", "#000000", Dark},
		{"pure white", "#ffffff", Light},
		{"3-digit shorthand black", "#000", Dark},
		{"3-digit shorthand white", "#fff", Light},
		{"no leading #", "0b0d10", Dark},
		{"uppercase hex", "#FFFFFF", Light},
		{"mid-gray leans on the 0.5 threshold", "#7f7f7f", Dark}, // 0.498 luminance, just under 0.5
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ComputeMode(tc.hex)
			if err != nil {
				t.Fatalf("ComputeMode(%q) error = %v", tc.hex, err)
			}
			if got != tc.want {
				t.Errorf("ComputeMode(%q) = %q, want %q", tc.hex, got, tc.want)
			}
		})
	}
}

func TestComputeMode_InvalidInput(t *testing.T) {
	for _, bad := range []string{"", "notacolor", "#12", "#1234567", "rgb(0,0,0)"} {
		if _, err := ComputeMode(bad); err == nil {
			t.Errorf("ComputeMode(%q) error = nil, want an error", bad)
		}
	}
}
