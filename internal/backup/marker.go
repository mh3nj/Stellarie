package backup

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteRestoreMarker persists m as JSON at markerPath - main.go checks
// for this file at the very start of every launch, before opening the
// live database (see this package's own doc comment on why the swap
// happens on next startup rather than in-process).
func WriteRestoreMarker(markerPath string, m RestoreMarker) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("backup: encoding restore marker: %w", err)
	}
	if err := os.WriteFile(markerPath, b, 0o600); err != nil {
		return fmt.Errorf("backup: writing restore marker: %w", err)
	}
	return nil
}

// ReadRestoreMarker returns (nil, nil) - not an error - when no marker
// file exists, since "nothing pending" is the overwhelmingly common
// case on every ordinary startup.
func ReadRestoreMarker(markerPath string) (*RestoreMarker, error) {
	b, err := os.ReadFile(markerPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("backup: reading restore marker: %w", err)
	}
	var m RestoreMarker
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("backup: decoding restore marker: %w", err)
	}
	return &m, nil
}

// ClearRestoreMarker removes the marker once its restore has been
// applied - main.go calls this immediately after swapping the database
// file, before opening it, so a crash between "swap applied" and
// "marker cleared" is the only window where a restore could be
// (harmlessly) re-attempted, never silently skipped.
func ClearRestoreMarker(markerPath string) error {
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("backup: clearing restore marker: %w", err)
	}
	return nil
}
