package services

import (
	"context"

	"stellarei/internal/sessions"
)

// CategoryTotal / ProjectTotal are one breakdown row. Name/ColorToken
// are left for the frontend to join from the category/project lists it
// already has loaded (via CategoryService/ProjectService) - an id of 0
// means "uncategorized"/"unassigned" and simply won't match anything in
// those lists, which the frontend renders as its own labeled bucket.
type CategoryTotal struct {
	CategoryID int64 `json:"categoryId"`
	Seconds    int64 `json:"seconds"`
}

type ProjectTotal struct {
	ProjectID int64 `json:"projectId"`
	Seconds   int64 `json:"seconds"`
}

// ContextSwitchStats mirrors sessions.ContextSwitchStats for the
// frontend (section 32 - "digital friction").
type ContextSwitchStats struct {
	Switches       int   `json:"switches"`
	SessionCount   int   `json:"sessionCount"`
	AverageSeconds int64 `json:"averageSeconds"`
	LongestSeconds int64 `json:"longestSeconds"`
}

// Trend compares total tracked time in [startUnix, endUnix) against the
// immediately preceding period of equal length (section 15's "↑ 12%
// vs previous week" style comparison). Never claims causation - section
// 31/66 apply here too, though the number itself is exact, not an
// estimate; only the framing in the UI needs to stay descriptive.
type Trend struct {
	CurrentSeconds  int64   `json:"currentSeconds"`
	PreviousSeconds int64   `json:"previousSeconds"`
	PercentChange   float64 `json:"percentChange"` // 0 if PreviousSeconds is 0 (nothing to compare against)
}

type AnalyticsService struct {
	sessionRepo sessions.Repository
}

func NewAnalyticsService(sessionRepo sessions.Repository) *AnalyticsService {
	return &AnalyticsService{sessionRepo: sessionRepo}
}

func (s *AnalyticsService) GetCategoryBreakdown(startUnix, endUnix int64) ([]CategoryTotal, error) {
	sums, err := s.sessionRepo.SumDurationByCategory(context.Background(), startUnix, endUnix)
	if err != nil {
		return nil, err
	}
	out := make([]CategoryTotal, 0, len(sums))
	for id, seconds := range sums {
		out = append(out, CategoryTotal{CategoryID: id, Seconds: seconds})
	}
	return out, nil
}

func (s *AnalyticsService) GetProjectBreakdown(startUnix, endUnix int64) ([]ProjectTotal, error) {
	sums, err := s.sessionRepo.SumDurationByProject(context.Background(), startUnix, endUnix)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectTotal, 0, len(sums))
	for id, seconds := range sums {
		out = append(out, ProjectTotal{ProjectID: id, Seconds: seconds})
	}
	return out, nil
}

func (s *AnalyticsService) GetContextSwitchStats(startUnix, endUnix int64) (ContextSwitchStats, error) {
	stats, err := s.sessionRepo.ContextSwitchStats(context.Background(), startUnix, endUnix)
	if err != nil {
		return ContextSwitchStats{}, err
	}
	return ContextSwitchStats{
		Switches:       stats.Switches,
		SessionCount:   stats.SessionCount,
		AverageSeconds: stats.AverageSeconds,
		LongestSeconds: stats.LongestSeconds,
	}, nil
}

// GetTrend compares [startUnix, endUnix) against the equal-length period
// immediately before it - e.g. this week vs last week, if the caller
// passes this week's range.
func (s *AnalyticsService) GetTrend(startUnix, endUnix int64) (Trend, error) {
	ctx := context.Background()
	span := endUnix - startUnix

	currentByCategory, err := s.sessionRepo.SumDurationByCategory(ctx, startUnix, endUnix)
	if err != nil {
		return Trend{}, err
	}
	previousByCategory, err := s.sessionRepo.SumDurationByCategory(ctx, startUnix-span, startUnix)
	if err != nil {
		return Trend{}, err
	}

	var current, previous int64
	for _, v := range currentByCategory {
		current += v
	}
	for _, v := range previousByCategory {
		previous += v
	}

	trend := Trend{CurrentSeconds: current, PreviousSeconds: previous}
	if previous > 0 {
		trend.PercentChange = (float64(current-previous) / float64(previous)) * 100
	}
	return trend, nil
}
