package backup

import (
	"path/filepath"
	"testing"
)

func TestReadRestoreMarker_NoFileReturnsNilNil(t *testing.T) {
	dir := t.TempDir()
	m, err := ReadRestoreMarker(filepath.Join(dir, "does-not-exist.json"))
	if err != nil {
		t.Fatalf("expected no error for a missing marker, got: %v", err)
	}
	if m != nil {
		t.Fatalf("expected a nil marker when no file exists, got %+v", m)
	}
}

func TestWriteReadRestoreMarker_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "restore.json")

	want := RestoreMarker{
		StagingDBPath:         "/staging/stellarei.db",
		StagingScreenshotsDir: "/staging/screenshots",
		PreRestoreBackupPath:  "/backups/pre-restore.db",
		CreatedAt:             1700000000,
	}
	if err := WriteRestoreMarker(path, want); err != nil {
		t.Fatalf("WriteRestoreMarker: %v", err)
	}

	got, err := ReadRestoreMarker(path)
	if err != nil {
		t.Fatalf("ReadRestoreMarker: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil marker")
	}
	if *got != want {
		t.Fatalf("got %+v, want %+v", *got, want)
	}
}

func TestClearRestoreMarker_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "restore.json")

	if err := WriteRestoreMarker(path, RestoreMarker{StagingDBPath: "/x.db"}); err != nil {
		t.Fatalf("WriteRestoreMarker: %v", err)
	}
	if err := ClearRestoreMarker(path); err != nil {
		t.Fatalf("ClearRestoreMarker: %v", err)
	}

	m, err := ReadRestoreMarker(path)
	if err != nil {
		t.Fatalf("ReadRestoreMarker after clear: %v", err)
	}
	if m != nil {
		t.Fatal("expected no marker after ClearRestoreMarker")
	}
}

func TestClearRestoreMarker_MissingFileIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	if err := ClearRestoreMarker(filepath.Join(dir, "never-existed.json")); err != nil {
		t.Fatalf("expected clearing a nonexistent marker to be a no-op, got: %v", err)
	}
}
