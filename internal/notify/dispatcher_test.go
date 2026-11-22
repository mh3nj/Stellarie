package notify

import (
	"testing"
)

// Wails' NotificationService.SendNotification calls
// validateNotificationOptions first, which rejects any options value whose
// ID is empty. The dispatcher originally left ID unset, so every alarm,
// timer completion and focus warning failed validation and was silently
// never displayed. These tests pin the contract that would have caught it.
func TestBuildOptions_SetsNonEmptyID(t *testing.T) {
	opts := buildOptions("Alarm", "Stand up and stretch", "")
	if opts.ID == "" {
		t.Fatal("buildOptions left ID empty; Wails will reject the notification")
	}
	if opts.Title != "Alarm" {
		t.Fatalf("buildOptions title = %q, want %q", opts.Title, "Alarm")
	}
	if opts.Body != "Stand up and stretch" {
		t.Fatalf("buildOptions body = %q, want %q", opts.Body, "Stand up and stretch")
	}
}

func TestBuildOptions_IDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		opts := buildOptions("Alarm", "body", "")
		if opts.ID == "" {
			t.Fatal("buildOptions produced an empty ID")
		}
		if seen[opts.ID] {
			t.Fatalf("buildOptions reused ID %q", opts.ID)
		}
		seen[opts.ID] = true
	}
}

func TestBuildOptions_SoundMapping(t *testing.T) {
	if opts := buildOptions("t", "b", ""); opts.Sound != nil {
		t.Fatalf("empty sound should use the platform default (nil), got %+v", opts.Sound)
	}
	if opts := buildOptions("t", "b", "silent"); opts.Sound == nil || !opts.Sound.Silent {
		t.Fatalf("silent sound not mapped: %+v", opts.Sound)
	}
	if opts := buildOptions("t", "b", "Ping"); opts.Sound == nil || opts.Sound.Name != "Ping" {
		t.Fatalf("named sound not mapped: %+v", opts.Sound)
	}
}
