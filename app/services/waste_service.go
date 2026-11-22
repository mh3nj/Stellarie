package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"stellarei/internal/ai"
	"stellarei/internal/categories"
	"stellarei/internal/sessions"
	"stellarei/internal/waste"
)

// WasteFindingView is waste.Finding shaped for the frontend - same
// field set, JSON tags already match, kept as a distinct type for the
// same DTO-naming-consistency reason PatternInsightView exists.
type WasteFindingView struct {
	Kind         string `json:"kind"`
	CategoryID   int64  `json:"categoryId,omitempty"`
	CategoryName string `json:"categoryName,omitempty"`
	Text         string `json:"text"`
}

// WasteView is the full Digital Waste Detection result (spec section
// 33). AINote is Phase 11's OPTIONAL layer too - but unlike every other
// aiNote in this codebase, it's gated on ScopeSuggestions rather than a
// feature-specific scope. This is deliberate, not an oversight: spec
// section 37 lists "suggestions" as its own AI use case distinct from
// "patterns", and Digital Waste Detection is where that distinction
// earns its keep - internal/patterns' aiNote can only ever SUMMARIZE
// what was observed (backward-looking), while this one is explicitly
// asked for a forward-looking, actionable next step (e.g. "consider a
// budget for X") - still evidence-only, still no moral judgment, just a
// different job than "patterns" does. See aiNoteForFindings' own doc
// comment for the exact instruction given to the model.
type WasteView struct {
	Findings []WasteFindingView `json:"findings"`
	AINote   string             `json:"aiNote,omitempty"`
}

// wasteLookbackDays is shorter than Pattern Discovery's 30-day window
// (see patternLookbackDays in pattern_service.go) on purpose: spec
// section 33's examples ("repeatedly opening", "long idle periods")
// read as CURRENT-behavior observations, not long-run trends the way
// Pattern Discovery's median-start-time/time-of-day comparisons are
// a week is enough evidence for "this keeps happening" without also
// surfacing something from a month ago that's no longer true.
const wasteLookbackDays = 7

// WasteService composes real session data into Digital Waste Detection
// findings. Holds no state of its own - every call recomputes from
// current data, same as PatternService.
type WasteService struct {
	sessionRepo  sessions.Repository
	categoryRepo *categories.Repository
	aiSvc        *ai.Service // see idea_service.go's identical convention: always real in practice, nil handled defensively
}

func NewWasteService(sessionRepo sessions.Repository, categoryRepo *categories.Repository, aiSvc *ai.Service) *WasteService {
	return &WasteService{sessionRepo: sessionRepo, categoryRepo: categoryRepo, aiSvc: aiSvc}
}

// GetFindings runs Digital Waste Detection over the last
// wasteLookbackDays of session data. The "Idle" category lookup is
// best-effort: if it can't be resolved (renamed, deleted, or the
// category list can't be read), LongIdlePeriods/ShortNoiseSessions
// degrade gracefully via idleCategoryID=0 (see waste.LongIdlePeriods'
// own zero-ID handling) rather than failing the whole page
// RepeatedReopens doesn't need it at all and is unaffected either way.
func (s *WasteService) GetFindings() (WasteView, error) {
	ctx := context.Background()
	now := time.Now()

	lookbackStart := now.AddDate(0, 0, -wasteLookbackDays).Unix()
	views, err := s.sessionRepo.ListForRange(ctx, lookbackStart, now.Unix())
	if err != nil {
		return WasteView{}, fmt.Errorf("waste: loading sessions: %w", err)
	}
	samples := make([]waste.SessionSample, 0, len(views))
	for _, v := range views {
		if v.Excluded {
			continue // spec section 13: excluded sessions never feed analytics of any kind
		}
		samples = append(samples, waste.SessionSample{
			CategoryID:   v.CategoryID,
			CategoryName: v.CategoryName,
			StartedAt:    v.StartedAt,
			EndedAt:      v.EndedAt,
		})
	}

	var idleCategoryID int64
	if s.categoryRepo != nil {
		if cats, err := s.categoryRepo.ListCategories(ctx); err == nil {
			for _, c := range cats {
				if c.Name == "Idle" {
					idleCategoryID = c.ID
					break
				}
			}
		}
	}

	findings := waste.DetectAll(samples, idleCategoryID)

	views2 := make([]WasteFindingView, 0, len(findings))
	for _, f := range findings {
		views2 = append(views2, WasteFindingView{Kind: f.Kind, CategoryID: f.CategoryID, CategoryName: f.CategoryName, Text: f.Text})
	}

	view := WasteView{Findings: views2}
	view.AINote = s.aiNoteForFindings(ctx, findings)
	return view, nil
}

// aiNoteForFindings is the one place in this codebase ScopeSuggestions
// is used. Same anti-fabrication contract as every other aiNote
// function (idea_service.go, receipt_service.go, pattern_service.go):
// built only from the findings already computed, explicitly forbidden
// from inventing a finding that isn't in the list. What's different
// here is the ASK - not "summarize what you observed" but "suggest one
// concrete, non-judgmental next step" - while the system prompt is
// exactly as strict as spec section 33 itself about the "no moral
// judgment" rule, so the one place this codebase asks AI to be
// actionable rather than purely observational still can't slip into
// "you should feel bad about this."
func (s *WasteService) aiNoteForFindings(ctx context.Context, findings []waste.Finding) string {
	if s.aiSvc == nil || len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	for i, f := range findings {
		fmt.Fprintf(&b, "%d. %s\n", i+1, f.Text)
	}

	result, err := s.aiSvc.Enhance(ctx, ai.ScopeSuggestions, ai.Prompt{
		SystemPrompt: "You are an optional commentary add-on inside a personal time-tracking " +
			"desktop app called STELLAREI, on its Digital Waste Detection screen. You will be " +
			"given a numbered list of findings already computed by the app's own deterministic " +
			"analysis - each one is neutral evidence, not a verdict. Your job is to suggest ONE " +
			"short, concrete, optional next step the person could consider, based only on the " +
			"findings given. You must NOT introduce any finding, statistic, or observation not " +
			"already in the list. You must NEVER moralize, shame, or imply the person did " +
			"something wrong - phrase it as a plain, optional suggestion, not a judgment. Do not " +
			"use exclamation points or emoji. Reply with only that one sentence.",
		UserPrompt: b.String(),
		MaxTokens:  120,
	})
	if err != nil || result.Unavailable {
		return ""
	}
	return strings.TrimSpace(result.Text)
}
