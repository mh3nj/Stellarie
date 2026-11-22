package closebehavior

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want Behavior
	}{
		{"close", "close", CloseApplication},
		{"tray", "tray", MinimizeToTray},
		{"ask", "ask", AskEveryTime},
		{"empty falls back to default", "", DefaultBehavior},
		{"unknown falls back to default", "vaporize", DefaultBehavior},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Parse(tc.raw); got != tc.want {
				t.Errorf("Parse(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestValid(t *testing.T) {
	for _, raw := range []string{"close", "tray", "ask"} {
		if !Valid(raw) {
			t.Errorf("Valid(%q) = false, want true", raw)
		}
	}
	for _, raw := range []string{"", "banana", "Close"} {
		if Valid(raw) {
			t.Errorf("Valid(%q) = true, want false", raw)
		}
	}
}
