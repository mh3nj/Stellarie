// Package backup implements Backup (spec section 48) and Import/Export
// (spec section 49). The database half of both features rests on one
// real, standard SQLite mechanism - `VACUUM INTO` - which produces a
// complete, consistent snapshot of a live WAL-mode database in a single
// SQL statement, including anything committed but not yet checkpointed
// out of the WAL file. Verified against real SQLite (see the repo's own
// verification notes) before writing the Go wrapper around it.
//
// Restore is the one place this package makes a real architectural
// choice worth stating up front: STELLAREI cannot safely swap its own
// live database connection out from under every service that holds one
// mid-session (settings, timers, alarms, sessions, and more each capture
// their own *sql.DB reference at startup). Rather than pretend to do a
// live hot-swap, Restore stages the replacement file and a small marker
// (see RestoreMarker) on disk, and the actual swap happens the next time
// STELLAREI starts, before anything opens the database - the same
// restart-to-apply pattern most real desktop apps use for exactly this
// reason, not a workaround.
package backup

import "time"

// Format is one of spec section 49's own export formats.
type Format string

const (
	FormatJSON   Format = "json"
	FormatCSV    Format = "csv"
	FormatSQLite Format = "sqlite"
)

// Dataset is one of spec section 49's own exportable data types.
type Dataset string

const (
	DatasetActivities Dataset = "activities" // raw activity_events
	DatasetSessions   Dataset = "sessions"   // sessionized (Timeline's own view)
	DatasetProjects   Dataset = "projects"
	DatasetGoals      Dataset = "goals"
	DatasetBudgets    Dataset = "budgets"
	DatasetAnalytics  Dataset = "analytics" // computed category/project time totals
)

// Info describes a single backup file on disk - what BackupService lists
// and what the frontend shows in the Backups page.
type Info struct {
	Path                string
	CreatedAt           time.Time
	SizeBytes           int64
	IncludesScreenshots bool // true for a .zip bundle, false for a plain .db snapshot
}

// RestoreMarker is staged to disk by Restore and consumed once, at the
// very start of the next launch, before the live database is opened
// see this package's own doc comment for why a live in-process swap
// isn't the safer option here.
type RestoreMarker struct {
	StagingDBPath         string `json:"stagingDbPath"`
	StagingScreenshotsDir string `json:"stagingScreenshotsDir,omitempty"` // "" if the backup being restored had no screenshots bundled
	PreRestoreBackupPath  string `json:"preRestoreBackupPath"`            // the automatic safety backup taken of the CURRENT database before staging the restore (section 48: "create an automatic backup of the current database")
	CreatedAt             int64  `json:"createdAt"`
}
