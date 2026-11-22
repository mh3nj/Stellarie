package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"stellarei/internal/categories"
	"stellarei/internal/projects"
	"stellarei/internal/search"
	"stellarei/internal/sessions"
)

// searchLookbackYears bounds how far back an entity-only or unmatched
// (free-text) search looks when no date was recognized in the query
// spec's own examples ("when did I work on HeartLib?") imply searching
// a long history, and the overlap query this uses was benchmarked as
// fast even at extreme scale (see README's Phase 12 section), so this
// is generous rather than tightly optimized.
const searchLookbackYears = 5

// maxSearchResults caps how many session results a non-superlative
// query returns - spec doesn't give a number, but an unbounded list for
// a broad query like "gaming" over years of history would be more noise
// than a "search" result, not less.
const maxSearchResults = 30

// SearchResultView is one matched session, shaped for the frontend to
// display and link into Timeline - spec section 29: "Results should
// link directly into the relevant timeline." DateKey is the
// "YYYY-MM-DD" the frontend passes straight through as
// `/timeline?date=<DateKey>&zoom=day`, the same URL-param convention
// Calendar's day-click already established.
type SearchResultView struct {
	SessionID int64  `json:"sessionId"`
	Title     string `json:"title"`    // category or project name, whichever is more specific
	Subtitle  string `json:"subtitle"` // e.g. "2h 15m"
	DateKey   string `json:"dateKey"`
	StartedAt int64  `json:"startedAt"`
	EndedAt   int64  `json:"endedAt"`
}

// SearchView is the full result. Interpretation is ALWAYS present and
// always honest about what was actually understood - see
// internal/search's package doc for why this matters: the parser has a
// deliberately bounded vocabulary, and a person typing something it
// doesn't recognize deserves to know that's what happened, not silently
// get an empty or wrong-seeming result.
type SearchView struct {
	Interpretation string             `json:"interpretation"`
	Results        []SearchResultView `json:"results"`
}

// SearchService composes internal/search's query parser with real
// session, project, category, and application data. Holds no state of
// its own - every call recomputes from current data.
type SearchService struct {
	sessionRepo  sessions.Repository
	projectRepo  *projects.Repository
	categoryRepo *categories.Repository
}

func NewSearchService(sessionRepo sessions.Repository, projectRepo *projects.Repository, categoryRepo *categories.Repository) *SearchService {
	return &SearchService{sessionRepo: sessionRepo, projectRepo: projectRepo, categoryRepo: categoryRepo}
}

// Search parses rawQuery and returns matching sessions plus an honest
// explanation of how it was understood.
func (s *SearchService) Search(rawQuery string) (SearchView, error) {
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery == "" {
		return SearchView{Interpretation: "Type something to search for."}, nil
	}

	ctx := context.Background()
	now := time.Now()
	loc := time.Local

	entities := s.loadKnownEntities(ctx)
	parsed := search.Parse(rawQuery, entities, now, loc)

	switch {
	case parsed.HasInstant:
		return s.searchInstant(ctx, parsed, loc)
	case parsed.Entity != nil && parsed.HasDateRange:
		return s.searchEntityInRange(ctx, parsed, parsed.RangeStart, parsed.RangeEnd, loc)
	case parsed.Entity != nil:
		start := now.AddDate(-searchLookbackYears, 0, 0).Unix()
		return s.searchEntityInRange(ctx, parsed, start, now.Unix(), loc)
	case parsed.HasDateRange:
		return s.searchRangeOnly(ctx, parsed, loc)
	default:
		return s.searchFreeText(ctx, parsed, now, loc)
	}
}

func (s *SearchService) loadKnownEntities(ctx context.Context) []search.KnownEntity {
	var entities []search.KnownEntity
	if s.projectRepo != nil {
		if list, err := s.projectRepo.ListProjects(ctx, true); err == nil {
			for _, p := range list {
				entities = append(entities, search.KnownEntity{Kind: search.EntityProject, ID: p.ID, Name: p.Name})
			}
		}
	}
	if s.categoryRepo != nil {
		if list, err := s.categoryRepo.ListCategories(ctx); err == nil {
			for _, c := range list {
				entities = append(entities, search.KnownEntity{Kind: search.EntityCategory, ID: c.ID, Name: c.Name})
			}
		}
		if list, err := s.categoryRepo.ListApplications(ctx); err == nil {
			for _, a := range list {
				entities = append(entities, search.KnownEntity{Kind: search.EntityApplication, ID: a.ApplicationID, Name: a.ProcessName})
			}
		}
	}
	return entities
}

func (s *SearchService) searchInstant(ctx context.Context, parsed search.ParsedQuery, loc *time.Location) (SearchView, error) {
	dayStart := time.Unix(parsed.InstantUnix, 0).In(loc)
	dayStart = time.Date(dayStart.Year(), dayStart.Month(), dayStart.Day(), 0, 0, 0, 0, loc)
	views, err := s.sessionRepo.ListForRange(ctx, dayStart.Unix(), dayStart.AddDate(0, 0, 1).Unix())
	if err != nil {
		return SearchView{}, fmt.Errorf("search: loading sessions: %w", err)
	}
	// Initialized empty, never nil - see receipt_service.go's comment
	// on why a view-carrying slice must never marshal as null.
	results := make([]SearchResultView, 0)
	for _, v := range views {
		if v.Excluded || v.StartedAt > parsed.InstantUnix || v.EndedAt <= parsed.InstantUnix {
			continue
		}
		results = append(results, toSearchResultView(v))
	}
	instant := time.Unix(parsed.InstantUnix, 0).In(loc)
	interp := fmt.Sprintf("Looking for what you were doing at %s on %s", instant.Format("15:04"), instant.Format("January 2, 2006"))
	if len(results) == 0 {
		interp += " - nothing tracked at that exact moment."
	}
	return SearchView{Interpretation: interp, Results: results}, nil
}

func (s *SearchService) searchEntityInRange(ctx context.Context, parsed search.ParsedQuery, start, end int64, loc *time.Location) (SearchView, error) {
	views, err := s.sessionRepo.ListForRange(ctx, start, end)
	if err != nil {
		return SearchView{}, fmt.Errorf("search: loading sessions: %w", err)
	}
	var matched []sessions.View
	for _, v := range views {
		if v.Excluded {
			continue
		}
		if entityMatches(v, parsed.Entity) {
			matched = append(matched, v)
		}
	}
	results := rankAndLimit(matched, parsed.Superlative)

	interp := fmt.Sprintf("Looking for: %s", parsed.Entity.Name)
	if parsed.Superlative != "" {
		interp = fmt.Sprintf("Looking for your %s %s session", parsed.Superlative, parsed.Entity.Name)
	}
	if parsed.HasDateRange {
		interp += fmt.Sprintf(", %s", describeRange(start, end, loc))
	}
	if len(matched) == 0 {
		interp += " - nothing found."
	}
	return SearchView{Interpretation: interp, Results: results}, nil
}

func (s *SearchService) searchRangeOnly(ctx context.Context, parsed search.ParsedQuery, loc *time.Location) (SearchView, error) {
	views, err := s.sessionRepo.ListForRange(ctx, parsed.RangeStart, parsed.RangeEnd)
	if err != nil {
		return SearchView{}, fmt.Errorf("search: loading sessions: %w", err)
	}
	var matched []sessions.View
	for _, v := range views {
		if !v.Excluded {
			matched = append(matched, v)
		}
	}
	results := rankAndLimit(matched, parsed.Superlative)

	interp := "Looking for: " + describeRange(parsed.RangeStart, parsed.RangeEnd, loc)
	if parsed.Superlative != "" {
		interp = fmt.Sprintf("Looking for your %s session, %s", parsed.Superlative, describeRange(parsed.RangeStart, parsed.RangeEnd, loc))
	}
	if len(matched) == 0 {
		interp += " - nothing found."
	}
	return SearchView{Interpretation: interp, Results: results}, nil
}

func (s *SearchService) searchFreeText(ctx context.Context, parsed search.ParsedQuery, now time.Time, loc *time.Location) (SearchView, error) {
	start := now.AddDate(-searchLookbackYears, 0, 0).Unix()
	views, err := s.sessionRepo.ListForRange(ctx, start, now.Unix())
	if err != nil {
		return SearchView{}, fmt.Errorf("search: loading sessions: %w", err)
	}
	needle := strings.ToLower(parsed.RawText)
	var matched []sessions.View
	for _, v := range views {
		if v.Excluded {
			continue
		}
		haystack := strings.ToLower(v.Note + " " + v.ProjectName + " " + v.CategoryName + " " + v.ApplicationName)
		if strings.Contains(haystack, needle) {
			matched = append(matched, v)
		}
	}
	results := rankAndLimit(matched, parsed.Superlative)

	interp := fmt.Sprintf("Didn't recognize a specific date or name - searching notes and titles for %q", parsed.RawText)
	if len(matched) == 0 {
		interp += ". Nothing found. Try a project/category name, or a date like \"August 10\" or \"last month\"."
	}
	return SearchView{Interpretation: interp, Results: results}, nil
}

func entityMatches(v sessions.View, e *search.KnownEntity) bool {
	switch e.Kind {
	case search.EntityProject:
		return v.ProjectID == e.ID
	case search.EntityCategory:
		return v.CategoryID == e.ID
	case search.EntityApplication:
		return v.ApplicationID == e.ID
	default:
		return false
	}
}

// rankAndLimit sorts matched sessions and returns the right slice for
// the query shape: exactly the top 1 for a superlative ("longest"/
// "shortest"), or the maxSearchResults most recent otherwise.
func rankAndLimit(matched []sessions.View, superlative string) []SearchResultView {
	if superlative != "" {
		if len(matched) == 0 {
			return make([]SearchResultView, 0)
		}
		sort.Slice(matched, func(i, j int) bool {
			di := matched[i].EndedAt - matched[i].StartedAt
			dj := matched[j].EndedAt - matched[j].StartedAt
			if superlative == "shortest" {
				return di < dj
			}
			return di > dj
		})
		return []SearchResultView{toSearchResultView(matched[0])}
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].StartedAt > matched[j].StartedAt })
	if len(matched) > maxSearchResults {
		matched = matched[:maxSearchResults]
	}
	out := make([]SearchResultView, 0, len(matched))
	for _, v := range matched {
		out = append(out, toSearchResultView(v))
	}
	return out
}

func toSearchResultView(v sessions.View) SearchResultView {
	title := v.ProjectName
	if title == "" {
		title = v.CategoryName
	}
	if title == "" {
		title = v.ApplicationName
	}
	if title == "" {
		title = "Uncategorized"
	}
	day := time.Unix(v.StartedAt, 0)
	duration := time.Duration(v.EndedAt-v.StartedAt) * time.Second
	return SearchResultView{
		SessionID: v.ID,
		Title:     title,
		Subtitle:  fmt.Sprintf("%s · %s", day.Format("Jan 2, 2006"), duration.String()),
		DateKey:   day.Format("2006-01-02"),
		StartedAt: v.StartedAt,
		EndedAt:   v.EndedAt,
	}
}

func describeRange(start, end int64, loc *time.Location) string {
	s := time.Unix(start, 0).In(loc)
	e := time.Unix(end, 0).In(loc).Add(-time.Second)
	if s.Format("2006-01-02") == e.Format("2006-01-02") {
		return s.Format("January 2, 2006")
	}
	return fmt.Sprintf("%s to %s", s.Format("Jan 2, 2006"), e.Format("Jan 2, 2006"))
}
