// Package migrations embeds the SQL migration files into the compiled
// binary so STELLAREI never depends on a migrations/ folder existing next
// to the executable at runtime.
package migrations

import "embed"

// FS contains every *.sql file in this directory. Files are applied in
// lexical filename order by internal/database.Migrate, which is why they're
// numbered "0001_", "0002_", etc. - always zero-pad to keep sort order
// correct past file 9.
//
//go:embed *.sql
var FS embed.FS
