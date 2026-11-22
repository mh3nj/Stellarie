package backup

import (
	"strings"
	"testing"
	"time"
)

func TestFilename_Format(t *testing.T) {
	at := time.Date(2026, 8, 16, 9, 5, 3, 0, time.UTC)
	got := Filename(at, "stellarei-backup", "db")
	want := "stellarei-backup-20260816-090503.db"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFilename_UsesUTCRegardlessOfInputLocation(t *testing.T) {
	loc := time.FixedZone("UTC+3:30", 3*3600+30*60)
	at := time.Date(2026, 8, 16, 12, 35, 3, 0, loc) // 09:05:03 UTC
	got := Filename(at, "export", "json")
	if !strings.Contains(got, "090503") {
		t.Fatalf("expected UTC-normalized time in filename, got %q", got)
	}
}

func TestFilename_SortsChronologically(t *testing.T) {
	earlier := Filename(time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC), "b", "db")
	later := Filename(time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC), "b", "db")
	if earlier >= later {
		t.Fatalf("expected lexical ordering to match chronological ordering: %q should sort before %q", earlier, later)
	}
}
