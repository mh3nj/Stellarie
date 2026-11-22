package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

func TestBundleZip_DatabaseOnly_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	dbSnapshot := filepath.Join(dir, "snapshot.db")
	writeTestFile(t, dbSnapshot, "fake sqlite content")

	zipPath := filepath.Join(dir, "backup.zip")
	if err := BundleZip(zipPath, dbSnapshot, ""); err != nil {
		t.Fatalf("BundleZip: %v", err)
	}

	extractDir := filepath.Join(dir, "extracted")
	dbPath, hasScreenshots, err := ExtractZip(zipPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	if hasScreenshots {
		t.Fatal("expected hasScreenshots=false for a database-only bundle")
	}
	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("reading extracted database: %v", err)
	}
	if string(got) != "fake sqlite content" {
		t.Fatalf("extracted database content mismatch: got %q", got)
	}
}

func TestBundleZip_WithScreenshots_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	dbSnapshot := filepath.Join(dir, "snapshot.db")
	writeTestFile(t, dbSnapshot, "db content")

	shotsDir := filepath.Join(dir, "screenshots")
	if err := os.MkdirAll(shotsDir, 0o700); err != nil {
		t.Fatalf("creating screenshots dir: %v", err)
	}
	writeTestFile(t, filepath.Join(shotsDir, "20260816-090000.png"), "fake png 1")
	writeTestFile(t, filepath.Join(shotsDir, "20260816-091000.png"), "fake png 2")

	zipPath := filepath.Join(dir, "backup.zip")
	if err := BundleZip(zipPath, dbSnapshot, shotsDir); err != nil {
		t.Fatalf("BundleZip: %v", err)
	}

	extractDir := filepath.Join(dir, "extracted")
	dbPath, hasScreenshots, err := ExtractZip(zipPath, extractDir)
	if err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	if !hasScreenshots {
		t.Fatal("expected hasScreenshots=true when screenshots were bundled")
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("extracted database missing: %v", err)
	}
	shot1, err := os.ReadFile(filepath.Join(extractDir, "screenshots", "20260816-090000.png"))
	if err != nil {
		t.Fatalf("reading extracted screenshot: %v", err)
	}
	if string(shot1) != "fake png 1" {
		t.Fatalf("extracted screenshot content mismatch: got %q", shot1)
	}
}

func TestBundleZip_MissingScreenshotsDirIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	dbSnapshot := filepath.Join(dir, "snapshot.db")
	writeTestFile(t, dbSnapshot, "db content")

	zipPath := filepath.Join(dir, "backup.zip")
	// Screenshot memory was never enabled, so the directory genuinely
	// doesn't exist - must not fail the whole backup.
	err := BundleZip(zipPath, dbSnapshot, filepath.Join(dir, "does-not-exist"))
	if err != nil {
		t.Fatalf("expected a missing screenshots directory to be handled gracefully, got: %v", err)
	}
}

func TestExtractZip_RejectsArchiveWithoutDatabase(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "not-a-backup.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating test zip: %v", err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("random-file.txt")
	_, _ = w.Write([]byte("not a backup"))
	zw.Close()
	f.Close()

	_, _, err = ExtractZip(zipPath, filepath.Join(dir, "extracted"))
	if err == nil {
		t.Fatal("expected an error extracting an archive with no stellarei.db entry")
	}
}

func TestExtractZip_RejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "malicious.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating test zip: %v", err)
	}
	zw := zip.NewWriter(f)
	// A crafted entry name trying to escape the extraction directory.
	w, _ := zw.Create("../../etc/passwd")
	_, _ = w.Write([]byte("malicious"))
	zw.Close()
	f.Close()

	extractDir := filepath.Join(dir, "extracted")
	_, _, err = ExtractZip(zipPath, extractDir)
	if err == nil {
		t.Fatal("expected ExtractZip to reject a zip-slip entry, not silently write outside the extraction directory")
	}

	// Confirm nothing was actually written outside extractDir.
	if _, statErr := os.Stat(filepath.Join(dir, "etc", "passwd")); statErr == nil {
		t.Fatal("zip-slip entry was actually written to disk outside the extraction directory")
	}
}
