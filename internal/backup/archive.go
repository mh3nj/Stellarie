package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// dbEntryName/screenshotsPrefix are the fixed layout of a bundled
// backup zip - ExtractZip below relies on these exact names, so a
// bundle produced by BundleZip is always readable by ExtractZip
// regardless of what the person happened to name the .zip file itself.
const (
	dbEntryName       = "stellarei.db"
	screenshotsPrefix = "screenshots/"
)

// BundleZip packages a database snapshot (already produced by
// CreateSnapshot) together with the screenshots directory into a single
// .zip - spec section 48's "optionally screenshots". screenshotsDir may
// be "" to produce a database-only bundle (equivalent to just renaming
// the snapshot, but kept as one code path so callers don't need two).
func BundleZip(destZipPath, dbSnapshotPath, screenshotsDir string) error {
	out, err := os.Create(destZipPath)
	if err != nil {
		return fmt.Errorf("backup: creating archive: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	if err := addFileToZip(zw, dbSnapshotPath, dbEntryName); err != nil {
		return fmt.Errorf("backup: adding database to archive: %w", err)
	}

	if screenshotsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(screenshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no screenshots directory yet (screenshot memory never enabled) - not an error
		}
		return fmt.Errorf("backup: reading screenshots directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue // screenshots are flat files, section 27 - no subdirectories to recurse into
		}
		src := filepath.Join(screenshotsDir, entry.Name())
		if err := addFileToZip(zw, src, screenshotsPrefix+entry.Name()); err != nil {
			return fmt.Errorf("backup: adding screenshot %s to archive: %w", entry.Name(), err)
		}
	}
	return nil
}

func addFileToZip(zw *zip.Writer, srcPath, entryName string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	w, err := zw.Create(entryName)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

// ExtractZip unpacks a bundle produced by BundleZip into destDir,
// returning the extracted database's path and whether any screenshots
// were included. Used by Restore to stage a zip-format backup exactly
// the way a plain .db backup is staged - see RestoreMarker.
//
// Every extracted path is verified to stay within destDir before being
// written - a zip entry name containing ".." (a "zip slip") must never
// be allowed to write outside the intended directory, regardless of
// whether the .zip came from STELLAREI's own BundleZip or was edited by
// hand.
func ExtractZip(zipPath, destDir string) (dbPath string, hasScreenshots bool, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", false, fmt.Errorf("backup: opening archive: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", false, fmt.Errorf("backup: creating extraction directory: %w", err)
	}

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)
		if !isWithinDir(destDir, target) {
			return "", false, fmt.Errorf("backup: archive entry %q escapes the extraction directory", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return "", false, fmt.Errorf("backup: creating directory %s: %w", target, err)
			}
			continue
		}

		if err := extractOne(f, target); err != nil {
			return "", false, err
		}

		switch {
		case f.Name == dbEntryName:
			dbPath = target
		case len(f.Name) > len(screenshotsPrefix) && f.Name[:len(screenshotsPrefix)] == screenshotsPrefix:
			hasScreenshots = true
		}
	}

	if dbPath == "" {
		return "", false, fmt.Errorf("backup: archive does not contain %s - not a STELLAREI backup", dbEntryName)
	}
	return dbPath, hasScreenshots, nil
}

func extractOne(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("backup: creating directory for %s: %w", target, err)
	}
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("backup: opening archive entry %s: %w", f.Name, err)
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("backup: creating %s: %w", target, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("backup: extracting %s: %w", f.Name, err)
	}
	return nil
}

func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !hasDotDotPrefix(rel)
}

func hasDotDotPrefix(rel string) bool {
	return rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator))
}
