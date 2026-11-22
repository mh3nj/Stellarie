package categories

import "context"

// Resolver adapts Repository + Evaluate into the shape
// activity.CategoryResolver expects - same rationale as
// internal/projects.Resolver: keeps internal/activity free of a direct
// dependency on this package.
type Resolver struct {
	repo *Repository
}

func NewResolver(repo *Repository) *Resolver {
	return &Resolver{repo: repo}
}

// ResolveCategoryID tries every enabled category_rules match first
// (same fresh-read-every-call reasoning as internal/projects.Resolver
// a rule added in the UI takes effect on the very next session close),
// and only falls back to the original per-application
// default_category_id/override model (CategoryForApplication) if no
// rule matches. This ordering is deliberate: an explicit rule is a more
// specific signal than an application-wide default, mirroring how
// internal/projects' rules always take precedence over "no rule at
// all" (there's no project-side fallback to compare against, but the
// same "explicit beats default" principle applies here against the
// category default). A category with zero rules configured behaves
// exactly as before this resolver method's rule-checking was added
// this fallback path is untouched from the pre-rules implementation.
func (r *Resolver) ResolveCategoryID(ctx context.Context, applicationID int64, processName, windowTitle string) (int64, error) {
	rules, err := r.repo.ListAllRules(ctx)
	if err != nil {
		return 0, err
	}
	if id := Evaluate(rules, Sample{ProcessName: processName, WindowTitle: windowTitle}); id != 0 {
		return id, nil
	}
	return r.repo.CategoryForApplication(ctx, applicationID)
}
