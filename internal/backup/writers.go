package backup

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
)

// WriteJSON marshals data (any of the row-slice types
// app/services.BackupService builds per Dataset) as pretty-printed JSON
// to destPath. Kept generic and dataset-agnostic on purpose - the
// per-dataset SQL and row-shaping lives in app/services, where the rest
// of this codebase's data-shaping logic already lives (FocusService/
// IdeaService follow the same split); this function only ever knows how
// to write bytes to a file.
func WriteJSON(destPath string, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("backup: encoding JSON export: %w", err)
	}
	if err := os.WriteFile(destPath, b, 0o600); err != nil {
		return fmt.Errorf("backup: writing JSON export: %w", err)
	}
	return nil
}

// WriteCSV writes headers as the first row, then rows, to destPath.
// Every row must have the same length as headers - mismatched rows are
// a caller bug (the per-dataset code in app/services controls both), so
// this returns an error rather than silently padding/truncating, which
// would produce a CSV that quietly misattributes column values.
func WriteCSV(destPath string, headers []string, rows [][]string) error {
	f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("backup: creating CSV export: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("backup: writing CSV header: %w", err)
	}
	for i, row := range rows {
		if len(row) != len(headers) {
			return fmt.Errorf("backup: row %d has %d columns, want %d (matching the header)", i, len(row), len(headers))
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("backup: writing CSV row %d: %w", i, err)
		}
	}
	w.Flush()
	return w.Error()
}
