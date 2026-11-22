package projects

import "context"

// Resolver adapts Repository + Evaluate into the shape
// activity.ProjectResolver expects, so the Activity Engine can assign a
// project to each session it closes without importing this package
// directly (see activity.ProjectResolver's doc comment for why that
// matters - it avoids an import cycle through app/services).
type Resolver struct {
	repo *Repository
}

func NewResolver(repo *Repository) *Resolver {
	return &Resolver{repo: repo}
}

// ResolveProjectID loads every enabled rule (across all non-archived
// projects) and evaluates them against sample, returning 0 if nothing
// matches. Queries the database fresh on every call rather than caching
// - this runs once per closed session (not per poll tick), so the extra
// read is negligible against section 52's "avoid unnecessary WRITES"
// budget, and a fresh read means a rule you just added in the Projects
// UI takes effect on the very next session close, not after a restart.
func (r *Resolver) ResolveProjectID(ctx context.Context, processName, windowTitle string) (int64, error) {
	rules, err := r.repo.ListAllRules(ctx)
	if err != nil {
		return 0, err
	}
	return Evaluate(rules, Sample{ProcessName: processName, WindowTitle: windowTitle}), nil
}
