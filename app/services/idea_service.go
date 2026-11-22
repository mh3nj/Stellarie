package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stellarei/internal/ai"
	"stellarei/internal/categories"
	"stellarei/internal/idea"
	"stellarei/internal/projects"
	"stellarei/internal/sessions"
)

// SuggestionView is one of the "I HAVE NO IDEA" suggestions (spec
// section 34), shaped for the frontend.
type SuggestionView struct {
	Kind                  string   `json:"kind"` // project | goal | chaos | generic
	ProjectID             int64    `json:"projectId,omitempty"`
	GoalID                int64    `json:"goalId,omitempty"`
	Title                 string   `json:"title"`
	Description           string   `json:"description,omitempty"`
	EstimatedMinutesLabel string   `json:"estimatedMinutesLabel,omitempty"`
	Reasons               []string `json:"reasons,omitempty"`
}

// IdeaResult is a full "I HAVE NO IDEA" generation - the suggestions
// plus the history row id, so the frontend can call RecordAcceptance
// with it.
//
// AINote is ALWAYS empty unless the person has explicitly turned AI on
// AND opted the "i_have_no_idea" scope in specifically (spec section 37:
// disabled by default, opt-in per feature) AND the configured local
// server actually answered. The three Suggestions above are never
// affected by whether AINote is present - they come entirely from
// internal/idea's deterministic engine, exactly as before Phase 11. The
// frontend must render AINote (when non-empty) as a clearly-labeled
// optional add-on, per section 37's "clearly labeled" requirement - see
// NoIdea.tsx.
type IdeaResult struct {
	HistoryID   int64            `json:"historyId"`
	Suggestions []SuggestionView `json:"suggestions"`
	AINote      string           `json:"aiNote,omitempty"`
}

// ProjectIdeaView is section 35's "I want to build something" output.
// AINote follows the exact same all-off-by-default, opt-in, clearly-
// labeled contract as IdeaResult.AINote above (gated on the
// "project_ideas" scope here instead).
type ProjectIdeaView struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	EstimatedMinutes int    `json:"estimatedMinutes"`
	EstimatedLabel   string `json:"estimatedLabel"`
	Difficulty       string `json:"difficulty"`
	Why              string `json:"why"`
	AINote           string `json:"aiNote,omitempty"`
}

// IdeaService is bound to the frontend via application.NewService in
// main.go. Its whole job is turning REAL data from
// internal/projects/internal/goals (via *GoalService, same package)/
// internal/sessions/internal/categories into internal/idea.Candidates,
// letting that package's pure scoring/selection logic do the picking,
// and persisting the result to internal/idea's history tables so future
// generations don't repeat themselves (sections 34/36). No AI, no
// network call, nothing here that couldn't run entirely offline
// section 37 in spirit as much as internal/idea's own package doc
// insists on in letter.
type IdeaService struct {
	repo         *idea.Repository
	projectRepo  *projects.Repository
	goalService  *GoalService
	sessionRepo  sessions.Repository
	categoryRepo *categories.Repository

	// aiSvc is Phase 11's OPTIONAL enhancement layer (see internal/ai's
	// package doc and this file's own doc comment above). Always a real,
	// non-nil *ai.Service - "off" is represented by its Config, not by a
	// nil check here, matching how AIService itself always holds a live
	// ai.Service. Every call site below treats a nil aiSvc as valid too
	// (skipping straight to "no note"), purely so existing tests/callers
	// that construct an IdeaService without wiring AI yet don't need to
	// change.
	aiSvc *ai.Service
}

func NewIdeaService(repo *idea.Repository, projectRepo *projects.Repository, goalService *GoalService, sessionRepo sessions.Repository, categoryRepo *categories.Repository, aiSvc *ai.Service) *IdeaService {
	return &IdeaService{repo: repo, projectRepo: projectRepo, goalService: goalService, sessionRepo: sessionRepo, categoryRepo: categoryRepo, aiSvc: aiSvc}
}

// goalCandidateMinutesCap bounds a goal candidate's EstimatedMinutes to
// "make progress on this goal", never "finish it entirely" - a goal
// with 40 hours remaining shouldn't produce a suggestion that reads like
// a 40-hour ask.
const goalCandidateMinutesCap = 240

// Generate runs the full "I HAVE NO IDEA" flow (section 34): builds
// real candidates from active projects and in-progress goals, scores
// them against the person's stated time/intent, excludes anything
// suggested in the last 10 generations when enough alternatives exist,
// and always appends one Chaos Mode challenge as the final suggestion.
func (s *IdeaService) Generate(availableMinutes int, intentStr string) (IdeaResult, error) {
	ctx := context.Background()
	timeChoice := idea.TimeMinutes(availableMinutes)
	intent := idea.Intent(intentStr)
	if intentStr != "" && !idea.ValidIntent(intentStr) {
		return IdeaResult{}, fmt.Errorf("idea: invalid intent %q", intentStr)
	}

	pool, err := s.buildCandidatePool(ctx)
	if err != nil {
		return IdeaResult{}, err
	}

	recentTitles, err := s.repo.RecentTitles(ctx, 10)
	if err != nil {
		recentTitles = nil // a history read failure shouldn't block generating suggestions - just means no de-dup this time
	}
	recentChaos, err := s.repo.RecentChaosChallenges(ctx, 10)
	if err != nil {
		recentChaos = nil
	}

	suggestions := idea.PickSuggestions(pool, idea.ChaosBank(), timeChoice, intent, 2, recentTitles, recentChaos, nil)

	historyID, err := s.repo.RecordSuggestions(ctx, timeChoice, intent, suggestions)
	if err != nil {
		historyID = 0 // logging the generation is best-effort; a failure here must not prevent the person from seeing suggestions they already got
	}

	// Chaos suggestions get their own history row too (chaos_history is
	// separate from idea_history so RecentChaosChallenges can exclude
	// purely on chaos-challenge text, independent of ordinary suggestion
	// titles).
	for _, sg := range suggestions {
		if sg.Kind == idea.KindChaos {
			_, _ = s.repo.RecordChaosShown(ctx, sg.Title) // best-effort, same reasoning as above
		}
	}

	return IdeaResult{
		HistoryID:   historyID,
		Suggestions: toSuggestionViews(suggestions),
		AINote:      s.aiNoteForSuggestions(ctx, suggestions),
	}, nil
}

// aiNoteForSuggestions asks the configured local AI (if any) for one
// short, optional sentence about the ALREADY-DECIDED suggestions
// never new suggestions, never a replacement for internal/idea's
// scoring. Every fact the prompt contains (titles, descriptions,
// estimated minutes) is something the person is about to see on screen
// anyway; nothing hidden is sent. Returns "" - not an error - for every
// reason AI might not answer (off, scope not permitted, unreachable,
// timed out), so a slow or misconfigured local server can never delay
// or break "I HAVE NO IDEA" itself. See internal/ai.Service.Enhance's
// own doc comment for why that's the contract this relies on.
func (s *IdeaService) aiNoteForSuggestions(ctx context.Context, suggestions []idea.Suggestion) string {
	if s.aiSvc == nil || len(suggestions) == 0 {
		return ""
	}
	var b strings.Builder
	for i, sg := range suggestions {
		fmt.Fprintf(&b, "%d. %s", i+1, sg.Title)
		if sg.Description != "" {
			fmt.Fprintf(&b, " - %s", sg.Description)
		}
		if sg.EstimatedMinutes > 0 {
			fmt.Fprintf(&b, " (~%dm)", sg.EstimatedMinutes)
		}
		b.WriteByte('\n')
	}

	result, err := s.aiSvc.Enhance(ctx, ai.ScopeIHaveNoIdea, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI. The person has just been shown the numbered list of " +
			"suggestions below, already chosen by the app's own logic - your only job is to add ONE " +
			"short, plain, non-judgmental sentence that might help them pick between the options " +
			"already listed. Do not invent new suggestions, statistics, or facts not in the list. " +
			"Do not use exclamation points or emoji. Reply with only that one sentence.",
		UserPrompt: b.String(),
		MaxTokens:  120,
	})
	if err != nil || result.Unavailable {
		return "" // graceful degradation - see doc comment above
	}
	return strings.TrimSpace(result.Text)
}

// RecordAcceptance marks which suggestion (by its 0-based position in
// the array Generate returned) the person went with.
func (s *IdeaService) RecordAcceptance(historyID int64, index int) error {
	return s.repo.RecordAcceptance(context.Background(), historyID, index)
}

// GenerateProjectIdea runs section 35's "I want to build something".
func (s *IdeaService) GenerateProjectIdea(availableMinutes int, platform string) (ProjectIdeaView, error) {
	ctx := context.Background()

	recentProject, recentCategory := s.mostNeglectedProjectAndCategory(ctx)

	recentTitles, err := s.repo.RecentTitles(ctx, 10)
	if err != nil {
		recentTitles = nil
	}

	result := idea.GenerateProjectIdea(idea.ProjectIdeaInput{
		AvailableMinutes: idea.TimeMinutes(availableMinutes),
		Platform:         platform,
		RecentCategory:   recentCategory,
		NeglectedProject: recentProject,
	}, recentTitles, nil)

	// Logged through the same idea_history mechanism ordinary
	// suggestions use (intent left blank - a project idea isn't tied to
	// one) so it also counts toward future de-duplication.
	_, _ = s.repo.RecordSuggestions(ctx, idea.TimeMinutes(availableMinutes), "", []idea.Suggestion{
		{Candidate: idea.Candidate{Title: result.Title}},
	})

	return ProjectIdeaView{
		Title: result.Title, Description: result.Description, EstimatedMinutes: result.EstimatedMinutes,
		EstimatedLabel: result.EstimatedLabel, Difficulty: result.Difficulty, Why: result.Why,
		AINote: s.aiNoteForProjectIdea(ctx, result),
	}, nil
}

// aiNoteForProjectIdea mirrors aiNoteForSuggestions (see its doc
// comment for the full contract) for section 35's single generated
// project idea, gated on the separate "project_ideas" scope so a person
// can opt one of these two features into AI without the other.
func (s *IdeaService) aiNoteForProjectIdea(ctx context.Context, result idea.ProjectIdea) string {
	if s.aiSvc == nil {
		return ""
	}
	userPrompt := fmt.Sprintf("Project idea: %s\nDescription: %s\nEstimated time: %s\nDifficulty: %s\nWhy this was picked: %s",
		result.Title, result.Description, result.EstimatedLabel, result.Difficulty, result.Why)

	res, err := s.aiSvc.Enhance(ctx, ai.ScopeProjectIdeas, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI. The person has just been shown the single generated " +
			"project idea below, already chosen by the app's own logic - your only job is to add " +
			"ONE short, plain, encouraging-but-not-hyperbolic sentence about getting started. Do " +
			"not invent facts not in the description. Do not use exclamation points or emoji. " +
			"Reply with only that one sentence.",
		UserPrompt: userPrompt,
		MaxTokens:  120,
	})
	if err != nil || res.Unavailable {
		return ""
	}
	return strings.TrimSpace(res.Text)
}

// StartProjectFromIdea turns a generated project idea into a real
// project (spec section 35's "Start project" action) - nothing
// speculative about it once this is called: it's the same
// projects.Repository.CreateProject any other new project goes through.
func (s *IdeaService) StartProjectFromIdea(title, description string) (int64, error) {
	return s.projectRepo.CreateProject(context.Background(), title, description, "")
}

func (s *IdeaService) buildCandidatePool(ctx context.Context) ([]idea.Candidate, error) {
	var pool []idea.Candidate

	activeProjects, err := s.projectRepo.ListProjects(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("idea: listing projects: %w", err)
	}
	lastActivity, err := s.sessionRepo.LastActivityByProject(ctx)
	if err != nil {
		lastActivity = nil // missing recency data just means no neglect signal this time, not a hard failure
	}
	now := time.Now()
	for _, p := range activeProjects {
		if p.Status != "active" {
			continue
		}
		c := idea.Candidate{
			Kind: idea.KindProject, ProjectID: p.ID, Title: "Work on " + p.Name,
			Description: p.Description, EstimatedMinutes: 45, // a reasonable default work-session length; per-project targets aren't tracked at the Go layer yet
		}
		if lastTouched, ok := lastActivity[p.ID]; ok {
			days := int(now.Sub(time.Unix(lastTouched, 0)).Hours() / 24)
			if days > 0 {
				c.NeglectedDays = days
			}
		}
		pool = append(pool, c)
	}

	if s.goalService != nil {
		goalStatuses, err := s.goalService.ListWithStatus()
		if err == nil {
			for _, g := range goalStatuses {
				if g.PercentComplete >= 100 {
					continue // already complete - nothing left to suggest
				}
				remainingHours := g.TargetValue - g.ActualHours
				if remainingHours <= 0 {
					continue
				}
				minutes := int(remainingHours * 60)
				if minutes > goalCandidateMinutesCap {
					minutes = goalCandidateMinutesCap
				}
				pool = append(pool, idea.Candidate{
					Kind: idea.KindGoal, ProjectID: g.ProjectID, GoalID: g.ID,
					Title:            "Make progress on " + g.Name,
					Description:      fmt.Sprintf("%.0f%% complete toward %.0f %s", g.PercentComplete, g.TargetValue, g.Unit),
					EstimatedMinutes: minutes,
				})
			}
		}
	}

	return pool, nil
}

// mostNeglectedProjectAndCategory finds the single most-neglected active
// project (by real LastActivityByProject data) for GenerateProjectIdea's
// grounding - falling back to a category with no clear project-level
// signal only when there's no project data at all (e.g. a fresh
// install), and to ("", "") when there's truly nothing to ground on.
func (s *IdeaService) mostNeglectedProjectAndCategory(ctx context.Context) (project, category string) {
	activeProjects, err := s.projectRepo.ListProjects(ctx, false)
	if err != nil || len(activeProjects) == 0 {
		return "", s.anyCategoryName(ctx)
	}
	lastActivity, err := s.sessionRepo.LastActivityByProject(ctx)
	if err != nil || len(lastActivity) == 0 {
		return "", s.anyCategoryName(ctx)
	}

	var mostNeglectedName string
	var oldestTouch int64 = -1
	for _, p := range activeProjects {
		if p.Status != "active" {
			continue
		}
		touched, ok := lastActivity[p.ID]
		if !ok {
			continue // never touched at all isn't "neglected" in a groundable way - no specific date to reference honestly
		}
		if oldestTouch == -1 || touched < oldestTouch {
			oldestTouch = touched
			mostNeglectedName = p.Name
		}
	}
	if mostNeglectedName != "" {
		return mostNeglectedName, ""
	}
	return "", s.anyCategoryName(ctx)
}

func (s *IdeaService) anyCategoryName(ctx context.Context) string {
	cats, err := s.categoryRepo.ListCategories(ctx)
	if err != nil || len(cats) == 0 {
		return ""
	}
	return cats[0].Name
}

func toSuggestionViews(suggestions []idea.Suggestion) []SuggestionView {
	out := make([]SuggestionView, 0, len(suggestions))
	for _, s := range suggestions {
		out = append(out, SuggestionView{
			Kind: string(s.Kind), ProjectID: s.ProjectID, GoalID: s.GoalID,
			Title: s.Title, Description: s.Description,
			EstimatedMinutesLabel: s.EstimatedMinutesLabel, Reasons: s.Reasons,
		})
	}
	return out
}
