package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"stellarei/internal/backup"
	"stellarei/internal/projects"
	"stellarei/internal/sessions"
)

// BackupInfoView is a backup file on disk, shaped for the frontend.
type BackupInfoView struct {
	Path                string `json:"path"`
	Filename            string `json:"filename"`
	CreatedAt           int64  `json:"createdAt"`
	SizeBytes           int64  `json:"sizeBytes"`
	IncludesScreenshots bool   `json:"includesScreenshots"`
}

// BackupService is bound to the frontend via application.NewService in
// main.go. Implements spec sections 48 (Backup) and 49 (Import/Export).
// The actual database-snapshot mechanism (VACUUM INTO) and the
// restart-to-apply restore design are internal/backup's - this layer
// only adds what needs real repositories to answer: which dataset goes
// in an export, where files live on this particular install.
type BackupService struct {
	db            *sql.DB
	projectRepo   *projects.Repository
	sessionRepo   sessions.Repository
	goalService   *GoalService
	budgetService *BudgetService

	backupsDir     string
	exportsDir     string
	screenshotsDir string
	databasePath   string
	markerPath     string

	// restartFunc relaunches the application and quits the current
	// process - wired to a small helper in main.go, since only main.go
	// holds the *application.App needed to quit cleanly. nil is a valid
	// value (RestartToApplyRestore then just tells the person to
	// restart manually) rather than something every caller must guard.
	restartFunc func() error
}

func NewBackupService(db *sql.DB, projectRepo *projects.Repository, sessionRepo sessions.Repository, goalService *GoalService, budgetService *BudgetService, backupsDir, exportsDir, screenshotsDir, databasePath, markerPath string, restartFunc func() error) *BackupService {
	return &BackupService{
		db: db, projectRepo: projectRepo, sessionRepo: sessionRepo, goalService: goalService, budgetService: budgetService,
		backupsDir: backupsDir, exportsDir: exportsDir, screenshotsDir: screenshotsDir, databasePath: databasePath, markerPath: markerPath,
		restartFunc: restartFunc,
	}
}

// BackupNow creates a complete backup (spec section 48: "database,
// settings, themes, project data, goals, budgets" - all of which live
// in the one SQLite file VACUUM INTO snapshots, so "complete" falls out
// naturally rather than needing a table-by-table checklist) and,
// optionally, the screenshots directory alongside it.
func (s *BackupService) BackupNow(includeScreenshots bool) (BackupInfoView, error) {
	ctx := context.Background()
	now := time.Now()

	snapshotPath := filepath.Join(s.backupsDir, backup.Filename(now, "snapshot-tmp", "db"))
	if err := backup.CreateSnapshot(ctx, s.db, snapshotPath); err != nil {
		return BackupInfoView{}, err
	}

	var finalPath string
	if includeScreenshots {
		finalPath = filepath.Join(s.backupsDir, backup.Filename(now, "stellarei-backup", "zip"))
		err := backup.BundleZip(finalPath, snapshotPath, s.screenshotsDir)
		os.Remove(snapshotPath) // the temp snapshot's job is done whether bundling succeeded or not
		if err != nil {
			return BackupInfoView{}, err
		}
	} else {
		finalPath = filepath.Join(s.backupsDir, backup.Filename(now, "stellarei-backup", "db"))
		if err := os.Rename(snapshotPath, finalPath); err != nil {
			return BackupInfoView{}, fmt.Errorf("backup: finalizing backup: %w", err)
		}
	}

	stat, err := os.Stat(finalPath)
	if err != nil {
		return BackupInfoView{}, fmt.Errorf("backup: reading backup metadata: %w", err)
	}
	return BackupInfoView{Path: finalPath, Filename: filepath.Base(finalPath), CreatedAt: now.Unix(), SizeBytes: stat.Size(), IncludesScreenshots: includeScreenshots}, nil
}

// ListBackups lists everything in the backups directory that looks like
// one of STELLAREI's own backup files - newest first.
func (s *BackupService) ListBackups() ([]BackupInfoView, error) {
	entries, err := os.ReadDir(s.backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("backup: listing backups: %w", err)
	}

	out := make([]BackupInfoView, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "stellarei-backup-") {
			continue // skip subdirectories and anything that isn't one of our own finished backups (e.g. a leftover snapshot-tmp-* from an interrupted run)
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfoView{
			Path: filepath.Join(s.backupsDir, e.Name()), Filename: e.Name(),
			CreatedAt: info.ModTime().Unix(), SizeBytes: info.Size(),
			IncludesScreenshots: strings.HasSuffix(e.Name(), ".zip"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// DeleteBackup removes a backup file - refuses anything outside the
// backups directory, since this is exposed directly to the frontend and
// must never become a general-purpose "delete any file on disk" call.
func (s *BackupService) DeleteBackup(path string) error {
	if !isWithinDir(s.backupsDir, path) {
		return fmt.Errorf("backup: refusing to delete a path outside the backups directory")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("backup: deleting %s: %w", path, err)
	}
	return nil
}

// RestoreBackup stages path (a .db snapshot or a .zip bundle) to replace
// the live database on the NEXT startup - see internal/backup's own doc
// comment for why this can't safely happen while STELLAREI is running.
// Always takes an automatic safety backup of the CURRENT database first
// (spec section 48's explicit requirement), independent of whatever is
// being restored.
func (s *BackupService) RestoreBackup(path string) error {
	ctx := context.Background()
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("backup: %s not found: %w", path, err)
	}

	safetyPath := filepath.Join(s.backupsDir, backup.Filename(time.Now(), "stellarei-pre-restore", "db"))
	if err := backup.CreateSnapshot(ctx, s.db, safetyPath); err != nil {
		return fmt.Errorf("backup: creating safety backup before restore: %w", err)
	}

	stagingDir := filepath.Join(filepath.Dir(s.databasePath), "pending-restore")
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("backup: clearing previous staging directory: %w", err)
	}
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return fmt.Errorf("backup: creating staging directory: %w", err)
	}

	var stagingDBPath, stagingScreenshotsDir string
	if strings.HasSuffix(path, ".zip") {
		dbPath, hasScreenshots, err := backup.ExtractZip(path, stagingDir)
		if err != nil {
			return err
		}
		stagingDBPath = dbPath
		if hasScreenshots {
			stagingScreenshotsDir = filepath.Join(stagingDir, "screenshots")
		}
	} else {
		stagingDBPath = filepath.Join(stagingDir, "stellarei.db")
		if err := copyFile(path, stagingDBPath); err != nil {
			return fmt.Errorf("backup: staging database for restore: %w", err)
		}
	}

	marker := backup.RestoreMarker{
		StagingDBPath: stagingDBPath, StagingScreenshotsDir: stagingScreenshotsDir,
		PreRestoreBackupPath: safetyPath, CreatedAt: time.Now().Unix(),
	}
	return backup.WriteRestoreMarker(s.markerPath, marker)
}

// RestartToApplyRestore relaunches STELLAREI so a staged RestoreBackup
// takes effect. Returns an error (rather than doing nothing silently)
// when no restartFunc was wired in, so the frontend can fall back to
// telling the person to restart manually instead of a click that
// appears to do nothing.
func (s *BackupService) RestartToApplyRestore() error {
	if s.restartFunc == nil {
		return fmt.Errorf("backup: restart isn't available - please quit and reopen STELLAREI to finish the restore")
	}
	return s.restartFunc()
}

// --- export (spec section 49) ---------------------------------------------

// ExportSQLite exports the entire live database as a portable SQLite
// file - "the user owns their data" in its most literal, complete form.
func (s *BackupService) ExportSQLite() (string, error) {
	path := filepath.Join(s.exportsDir, backup.Filename(time.Now(), "stellarei-export", "db"))
	if err := backup.CreateSnapshot(context.Background(), s.db, path); err != nil {
		return "", err
	}
	return path, nil
}

// Export writes one dataset (spec section 49's own list) as JSON or CSV
// and returns the file path.
func (s *BackupService) Export(dataset, format string) (string, error) {
	ctx := context.Background()

	headers, jsonRows, csvRows, err := s.buildExportRows(ctx, backup.Dataset(dataset))
	if err != nil {
		return "", err
	}

	ext := "json"
	if backup.Format(format) == backup.FormatCSV {
		ext = "csv"
	}
	path := filepath.Join(s.exportsDir, backup.Filename(time.Now(), "stellarei-"+dataset, ext))

	if backup.Format(format) == backup.FormatCSV {
		if err := backup.WriteCSV(path, headers, csvRows); err != nil {
			return "", err
		}
	} else {
		if err := backup.WriteJSON(path, jsonRows); err != nil {
			return "", err
		}
	}
	return path, nil
}

func (s *BackupService) buildExportRows(ctx context.Context, dataset backup.Dataset) (headers []string, jsonRows any, csvRows [][]string, err error) {
	switch dataset {
	case backup.DatasetActivities:
		return s.exportActivities(ctx)
	case backup.DatasetSessions:
		return s.exportSessions(ctx)
	case backup.DatasetProjects:
		return s.exportProjects(ctx)
	case backup.DatasetGoals:
		return s.exportGoals()
	case backup.DatasetBudgets:
		return s.exportBudgets()
	case backup.DatasetAnalytics:
		return s.exportAnalytics(ctx)
	default:
		return nil, nil, nil, fmt.Errorf("backup: unknown export dataset %q", dataset)
	}
}

type activityExportRow struct {
	ID          int64  `json:"id"`
	StartedAt   int64  `json:"startedAt"`
	DurationSec int64  `json:"durationSecs"`
	Process     string `json:"process"`
	WindowTitle string `json:"windowTitle"`
	Idle        bool   `json:"idle"`
}

func (s *BackupService) exportActivities(ctx context.Context) ([]string, any, [][]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ae.id, ae.started_at, ae.duration_secs, COALESCE(a.process_name, ''), COALESCE(ae.window_title, ''), ae.idle
		FROM activity_events ae LEFT JOIN applications a ON a.id = ae.application_id
		ORDER BY ae.started_at ASC
	`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: querying activities: %w", err)
	}
	defer rows.Close()

	// Initialized empty, never nil: an empty export must serialize as
	// [] (and a header-only CSV), not a file containing `null`.
	jsonRows := make([]activityExportRow, 0)
	csvRows := make([][]string, 0)
	for rows.Next() {
		var r activityExportRow
		var idleInt int
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.DurationSec, &r.Process, &r.WindowTitle, &idleInt); err != nil {
			return nil, nil, nil, fmt.Errorf("backup: scanning activity row: %w", err)
		}
		r.Idle = idleInt != 0
		jsonRows = append(jsonRows, r)
		csvRows = append(csvRows, []string{
			strconv.FormatInt(r.ID, 10), strconv.FormatInt(r.StartedAt, 10), strconv.FormatInt(r.DurationSec, 10),
			r.Process, r.WindowTitle, strconv.FormatBool(r.Idle),
		})
	}
	headers := []string{"id", "startedAt", "durationSecs", "process", "windowTitle", "idle"}
	return headers, jsonRows, csvRows, rows.Err()
}

func (s *BackupService) exportSessions(ctx context.Context) ([]string, any, [][]string, error) {
	// All-time: unix epoch to now. ListForRange returns everything
	// overlapping the window, so 0..now covers the entire history.
	views, err := s.sessionRepo.ListForRange(ctx, 0, time.Now().Unix())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: querying sessions: %w", err)
	}
	headers := []string{"id", "startedAt", "endedAt", "application", "project", "category", "excluded"}
	csvRows := make([][]string, 0, len(views))
	for _, v := range views {
		csvRows = append(csvRows, []string{
			strconv.FormatInt(v.ID, 10), strconv.FormatInt(v.StartedAt, 10), strconv.FormatInt(v.EndedAt, 10),
			v.ApplicationName, v.ProjectName, v.CategoryName, strconv.FormatBool(v.Excluded),
		})
	}
	return headers, views, csvRows, nil
}

func (s *BackupService) exportProjects(ctx context.Context) ([]string, any, [][]string, error) {
	list, err := s.projectRepo.ListProjects(ctx, true) // include archived - an export should be complete, not just "currently active"
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: querying projects: %w", err)
	}
	headers := []string{"id", "name", "description", "status", "createdAt"}
	csvRows := make([][]string, 0, len(list))
	for _, p := range list {
		csvRows = append(csvRows, []string{strconv.FormatInt(p.ID, 10), p.Name, p.Description, p.Status, strconv.FormatInt(p.CreatedAt, 10)})
	}
	return headers, list, csvRows, nil
}

func (s *BackupService) exportGoals() ([]string, any, [][]string, error) {
	list, err := s.goalService.ListWithStatus()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: querying goals: %w", err)
	}
	headers := []string{"id", "name", "goalType", "targetValue", "unit", "actualHours", "percentComplete"}
	csvRows := make([][]string, 0, len(list))
	for _, g := range list {
		csvRows = append(csvRows, []string{
			strconv.FormatInt(g.ID, 10), g.Name, g.GoalType,
			strconv.FormatFloat(g.TargetValue, 'f', 2, 64), g.Unit,
			strconv.FormatFloat(g.ActualHours, 'f', 2, 64), strconv.FormatFloat(g.PercentComplete, 'f', 1, 64),
		})
	}
	return headers, list, csvRows, nil
}

func (s *BackupService) exportBudgets() ([]string, any, [][]string, error) {
	list, err := s.budgetService.ListWithStatus()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: querying budgets: %w", err)
	}
	headers := []string{"id", "name", "budgetType", "limitMinutes", "actualMinutes"}
	csvRows := make([][]string, 0, len(list))
	for _, b := range list {
		csvRows = append(csvRows, []string{
			strconv.FormatInt(b.ID, 10), b.Name, b.BudgetType,
			strconv.Itoa(b.LimitMinutes), strconv.Itoa(b.ActualMinutes),
		})
	}
	return headers, list, csvRows, nil
}

type analyticsExportRow struct {
	Scope        string `json:"scope"` // "category" | "project"
	Name         string `json:"name"`
	TotalMinutes int64  `json:"totalMinutes"`
}

func (s *BackupService) exportAnalytics(ctx context.Context) ([]string, any, [][]string, error) {
	// All-time category/project totals - the same real
	// SumDurationByCategory/SumDurationByProject aggregates Analytics
	// itself uses, just exported wholesale instead of for one range.
	catSums, err := s.sessionRepo.SumDurationByCategory(ctx, 0, time.Now().Unix())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: summing categories: %w", err)
	}
	projSums, err := s.sessionRepo.SumDurationByProject(ctx, 0, time.Now().Unix())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("backup: summing projects: %w", err)
	}

	// Initialized empty, never nil - see the comment on the
	// activity-export rows above for why.
	jsonRows := make([]analyticsExportRow, 0)
	csvRows := make([][]string, 0)
	if err := s.appendNamedSums(ctx, "category", catSums, &jsonRows, &csvRows); err != nil {
		return nil, nil, nil, err
	}
	if err := s.appendNamedSums(ctx, "project", projSums, &jsonRows, &csvRows); err != nil {
		return nil, nil, nil, err
	}
	headers := []string{"scope", "name", "totalMinutes"}
	return headers, jsonRows, csvRows, nil
}

// appendNamedSums resolves each id in sums to a human name (categories
// via a direct query, since no categories.Repository handle is wired
// into this service and adding one just for a name lookup here isn't
// worth the extra dependency; projects via the already-available
// projectRepo) and appends a row per entry.
//
// Names are resolved with exactly ONE query each (all categories, all
// projects), regardless of how many distinct ids are in sums - this
// used to call ListProjects (a full table fetch) or a `WHERE id = ?`
// query once PER id, which for the project case meant refetching and
// linear-scanning the entire project list again for every single
// project in the export. Same fix as BudgetService/GoalService's
// ListWithStatus (see those doc comments): batch the lookup once,
// reuse it, never touch the database inside the per-item loop.
//
// Iteration order is sorted by id - sums is a Go map, whose iteration
// order is intentionally randomized by the runtime, which would
// otherwise make every export's row order different from the last for
// no reason a person exporting their own data twice in a row should
// have to puzzle over.
func (s *BackupService) appendNamedSums(ctx context.Context, scope string, sums map[int64]int64, jsonRows *[]analyticsExportRow, csvRows *[][]string) error {
	names := map[int64]string{}
	if scope == "project" {
		if list, err := s.projectRepo.ListProjects(ctx, true); err == nil {
			for _, p := range list {
				names[p.ID] = p.Name
			}
		}
	} else {
		rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM categories`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				if rows.Scan(&id, &name) == nil {
					names[id] = name
				}
			}
		}
	}

	ids := make([]int64, 0, len(sums))
	for id := range sums {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		name, ok := names[id]
		if !ok {
			name = fmt.Sprintf("#%d", id)
		}
		minutes := sums[id] / 60
		*jsonRows = append(*jsonRows, analyticsExportRow{Scope: scope, Name: name, TotalMinutes: minutes})
		*csvRows = append(*csvRows, []string{scope, name, strconv.FormatInt(minutes, 10)})
	}
	return nil
}

// --- shared helpers ---------------------------------------------------

func copyFile(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// isWithinDir mirrors internal/backup's own zip-slip guard, reused here
// for DeleteBackup's path-confinement check - kept as its own small
// copy rather than exporting internal/backup's unexported helper, since
// this one operates on plain filesystem paths with no zip entry
// involved.
func isWithinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
