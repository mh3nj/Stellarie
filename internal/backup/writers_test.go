package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSON_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.json")

	type row struct {
		Name  string  `json:"name"`
		Hours float64 `json:"hours"`
	}
	data := []row{{Name: "Serenity", Hours: 12.5}, {Name: "Parsegan", Hours: 3}}

	if err := WriteJSON(path, data); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading exported file: %v", err)
	}
	var got []row
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decoding exported JSON: %v", err)
	}
	if len(got) != 2 || got[0].Name != "Serenity" || got[1].Hours != 3 {
		t.Fatalf("round-tripped data mismatch: %+v", got)
	}
}

func TestWriteCSV_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.csv")

	headers := []string{"name", "hours"}
	rows := [][]string{{"Serenity", "12.5"}, {"Parsegan", "3"}}

	if err := WriteCSV(path, headers, rows); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading exported file: %v", err)
	}
	want := "name,hours\nSerenity,12.5\nParsegan,3\n"
	if string(raw) != want {
		t.Fatalf("got %q, want %q", raw, want)
	}
}

func TestWriteCSV_MismatchedRowLengthErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.csv")

	headers := []string{"name", "hours"}
	rows := [][]string{{"Serenity"}} // missing a column

	if err := WriteCSV(path, headers, rows); err == nil {
		t.Fatal("expected an error for a row with the wrong number of columns")
	}
}

func TestWriteCSV_EmptyRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.csv")

	if err := WriteCSV(path, []string{"name"}, nil); err != nil {
		t.Fatalf("expected an empty dataset to still produce a header-only CSV, got: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "name\n" {
		t.Fatalf("got %q, want header-only CSV", raw)
	}
}
