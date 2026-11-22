package idea

import (
	"fmt"
	"math"
	"math/rand"
)

// ProjectIdeaInput is what the person told "I want to build something"
// (spec section 35's own input list, scoped to what STELLAREI can
// actually back with real data): AvailableMinutes and Platform come
// straight from the person; RecentCategory/NeglectedProject are real
// signals the caller (services.IdeaService) pulls from
// internal/categories/internal/sessions - either may be "" when there's
// no strong real signal to ground the idea in, in which case each
// template's own DefaultSubject is used instead (never a fabricated
// specific).
type ProjectIdeaInput struct {
	AvailableMinutes TimeMinutes
	Platform         string // "" or "any" | "desktop" | "web" | "cli"
	RecentCategory   string
	NeglectedProject string
}

// ProjectIdea is spec section 35's own example shape: Title,
// Description, an estimated range, a Difficulty, and one honest Why
// line - grounded in real data when RecentCategory/NeglectedProject
// backed it, or a neutral, non-fabricated line when neither did.
type ProjectIdea struct {
	Title            string
	Description      string
	EstimatedMinutes int
	EstimatedLabel   string // e.g. "3-5 hours", matching spec's own example range format
	Difficulty       string // Easy | Medium | Hard
	Why              string
	Platform         string // "" if the template fits any platform
}

type projectTemplate struct {
	Title          string
	DescriptionFmt string // exactly one %s, filled with the subject
	DefaultSubject string
	BaseMinutes    int
	Difficulty     string   // Easy | Medium | Hard
	Platforms      []string // empty = fits any platform
	Tags           []Intent
}

// projectTemplates are deliberately COMPLETE, pre-written (title,
// description) pairs rather than word-by-word combinatorics - gluing
// random nouns/verbs together reliably produces either boring or
// grammatically broken results, and section 35's own example
// ("Local Activity Constellation" / "Build a tiny visualization of
// today's computer activity as a constellation") reads like a specific,
// considered idea, not a mad-lib. Variety instead comes from: which
// template gets picked (weighted by real signals via Score, same
// mechanism pick.go uses for ordinary suggestions), which real subject
// fills the %s, and chaosBank-style history-based exclusion so the same
// idea doesn't come back next time.
var projectTemplates = []projectTemplate{
	{Title: "Local Activity Constellation", DescriptionFmt: "A tiny visualization of %s as a constellation - each point a session, brightness for how focused it was.", DefaultSubject: "today's computer activity", BaseMinutes: 240, Difficulty: "Medium", Tags: []Intent{IntentCreative, IntentLearn}},
	{Title: "Five-Minute Atlas", DescriptionFmt: "A map-style view of %s, zoomable down to individual minutes.", DefaultSubject: "your week", BaseMinutes: 180, Difficulty: "Medium", Tags: []Intent{IntentCreative, IntentLearn}},
	{Title: "Silent Hour Sketchbook", DescriptionFmt: "A tiny drawing tool built around %s - one constraint, one canvas.", DefaultSubject: "a single unusual input method", BaseMinutes: 60, Difficulty: "Easy", Platforms: []string{"desktop", "web"}, Tags: []Intent{IntentCreative, IntentFun}},
	{Title: "Second-Screen Companion", DescriptionFmt: "A minimal always-on-top widget showing %s at a glance.", DefaultSubject: "one number you care about right now", BaseMinutes: 90, Difficulty: "Easy", Platforms: []string{"desktop"}, Tags: []Intent{IntentProductive}},
	{Title: "One-Command Ritual", DescriptionFmt: "A single CLI command that automates %s, start to finish.", DefaultSubject: "a small annoying manual step in your workflow", BaseMinutes: 60, Difficulty: "Easy", Platforms: []string{"cli"}, Tags: []Intent{IntentProductive, IntentLearn}},
	{Title: "Terminal Weather Vane", DescriptionFmt: "A CLI toy that reacts to %s the way a weather vane reacts to wind.", DefaultSubject: "whatever's currently in your clipboard history", BaseMinutes: 90, Difficulty: "Medium", Platforms: []string{"cli"}, Tags: []Intent{IntentFun, IntentCreative}},
	{Title: "Paper Trail", DescriptionFmt: "A tiny static site that turns %s into a scrollable timeline.", DefaultSubject: "your recent project history", BaseMinutes: 150, Difficulty: "Medium", Platforms: []string{"web"}, Tags: []Intent{IntentCreative, IntentProductive}},
	{Title: "One-Button Instrument", DescriptionFmt: "A tiny generative-music toy built around %s, controlled with a single button.", DefaultSubject: "one sound source", BaseMinutes: 60, Difficulty: "Easy", Tags: []Intent{IntentFun, IntentCreative}},
	{Title: "Neglect Radar", DescriptionFmt: "A small dashboard that surfaces %s before you forget about it.", DefaultSubject: "whatever you haven't touched in a while", BaseMinutes: 180, Difficulty: "Medium", Tags: []Intent{IntentProductive, IntentLearn}},
	{Title: "Constraint Engine", DescriptionFmt: "A generator that hands you one weird, specific constraint for %s and nothing else.", DefaultSubject: "your next creative session", BaseMinutes: 120, Difficulty: "Medium", Tags: []Intent{IntentCreative, IntentFun}},
	{Title: "Tiny Time Capsule", DescriptionFmt: "A one-file app that snapshots %s and lets future-you compare against it.", DefaultSubject: "your current desktop and window layout", BaseMinutes: 90, Difficulty: "Easy", Platforms: []string{"desktop"}, Tags: []Intent{IntentCreative, IntentLearn}},
	{Title: "Reverse Changelog", DescriptionFmt: "A tool that writes %s's changelog backwards - starting from what it could become.", DefaultSubject: "one of your existing projects", BaseMinutes: 45, Difficulty: "Easy", Tags: []Intent{IntentCreative, IntentProductive}},
	{Title: "Unlikely Bridge", DescriptionFmt: "A small integration connecting %s to something it was never meant to talk to.", DefaultSubject: "two tools you already use", BaseMinutes: 240, Difficulty: "Hard", Tags: []Intent{IntentLearn, IntentCreative}},
	{Title: "Pocket Archivist", DescriptionFmt: "A background tool that quietly indexes %s so it's searchable later.", DefaultSubject: "a folder you keep meaning to organize", BaseMinutes: 150, Difficulty: "Medium", Platforms: []string{"desktop", "cli"}, Tags: []Intent{IntentProductive, IntentLearn}},
	{Title: "Deliberate Glitch", DescriptionFmt: "An interface for %s that treats visual glitches as a feature, not a bug.", DefaultSubject: "a screen you look at every day", BaseMinutes: 90, Difficulty: "Medium", Platforms: []string{"web", "desktop"}, Tags: []Intent{IntentCreative, IntentFun}},
	{Title: "Micro Metronome", DescriptionFmt: "A tiny tool that keeps a steady rhythm for %s - no UI beyond a pulse.", DefaultSubject: "short focused work intervals", BaseMinutes: 45, Difficulty: "Easy", Tags: []Intent{IntentProductive, IntentFun}},
	{Title: "Second Opinion", DescriptionFmt: "A tool that generates the OPPOSITE approach to %s, just to see it.", DefaultSubject: "a decision you already made", BaseMinutes: 30, Difficulty: "Easy", Tags: []Intent{IntentCreative, IntentLearn}},
	{Title: "Quiet Companion", DescriptionFmt: "A single-purpose app that does one small kind thing related to %s and nothing else.", DefaultSubject: "how your day is actually going", BaseMinutes: 120, Difficulty: "Medium", Tags: []Intent{IntentCreative, IntentFun}},
}

func templateFitsPlatform(t projectTemplate, platform string) bool {
	if len(t.Platforms) == 0 || platform == "" || platform == "any" {
		return true
	}
	for _, p := range t.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// smallestTemplate returns the template with the lowest BaseMinutes
// among candidates - the honest fallback when nothing fit the
// requested time budget (see GenerateProjectIdea). Deterministic (no
// rng needed): the smallest is the smallest.
func smallestTemplate(candidates []Candidate, templateByTitle map[string]projectTemplate) projectTemplate {
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.EstimatedMinutes < best.EstimatedMinutes {
			best = c
		}
	}
	return templateByTitle[best.Title]
}

// GenerateProjectIdea picks one template (weighted the same way ordinary
// suggestions are - see pick.go: real intent/time-fit scoring plus
// jitter for variety, history-aware exclusion so the same idea doesn't
// keep coming back), fills in a subject grounded in real data when
// available, and returns a complete idea. Never returns an error: an
// empty/unusable input always still produces a generic-but-honest idea,
// the same "excellent empty state, no fake data, no dead end" principle
// pick.go's genericFallbacks follows.
func GenerateProjectIdea(input ProjectIdeaInput, recentTitles map[string]bool, rng *rand.Rand) ProjectIdea {
	rng = rngOrDefault(rng)

	subject, why := resolveSubject(input)

	var candidates []Candidate
	templateByTitle := map[string]projectTemplate{}
	for _, t := range projectTemplates {
		if !templateFitsPlatform(t, input.Platform) {
			continue
		}
		candidates = append(candidates, Candidate{Title: t.Title, EstimatedMinutes: t.BaseMinutes, Tags: t.Tags})
		templateByTitle[t.Title] = t
	}
	if len(candidates) == 0 {
		// Every template filtered out by platform (shouldn't happen with
		// the current bank - several have no platform restriction at
		// all - but if a future platform value matches nothing, fall
		// back to the whole bank rather than the caller getting nothing
		// back for a section that promises "always generates an idea").
		for _, t := range projectTemplates {
			candidates = append(candidates, Candidate{Title: t.Title, EstimatedMinutes: t.BaseMinutes, Tags: t.Tags})
			templateByTitle[t.Title] = t
		}
	}

	picked := pickN(candidates, input.AvailableMinutes, IntentSurprise, 1, recentTitles, rng)
	var chosen projectTemplate
	if len(picked) == 1 {
		chosen = templateByTitle[picked[0].Title]
	} else {
		// Nothing fit within the time budget's soft cap - a project
		// idea's minimum realistic scope (30+ minutes for anything in
		// this bank) can genuinely exceed a short ask like 15 minutes.
		// Falling back to a uniformly random template here would risk
		// handing back the single LARGEST idea in the bank for the
		// SHORTEST time budget - the opposite of honest. Falling back to
		// the smallest-scoped template instead keeps the estimate as
		// close to feasible as this bank gets.
		chosen = smallestTemplate(candidates, templateByTitle)
	}

	return ProjectIdea{
		Title:            chosen.Title,
		Description:      fmt.Sprintf(chosen.DescriptionFmt, subject),
		EstimatedMinutes: chosen.BaseMinutes,
		EstimatedLabel:   estimateRangeLabel(chosen.BaseMinutes),
		Difficulty:       chosen.Difficulty,
		Why:              why,
		Platform:         input.Platform,
	}
}

// resolveSubject decides what fills a template's %s and what the "Why"
// line honestly says about it. Real data wins when available - a
// neglected project is the strongest, most specific real signal
// (matches spec section 35's example: "You haven't experimented with
// data visualization recently"); a recently-used category is a weaker
// but still real fallback; with neither, the subject and why stay
// generic rather than inventing a fact.
func resolveSubject(input ProjectIdeaInput) (subject, why string) {
	switch {
	case input.NeglectedProject != "":
		return fmt.Sprintf("your %q project", input.NeglectedProject),
			fmt.Sprintf("you haven't touched %q in a while - this could be a way back in without committing to a full session on it.", input.NeglectedProject)
	case input.RecentCategory != "":
		return fmt.Sprintf("your recent %s work", input.RecentCategory),
			fmt.Sprintf("you've been active in %s lately, so the muscle memory is fresh.", input.RecentCategory)
	default:
		return "", "a reasonable fit for the time you have, with room for a bit of creative variety."
	}
}

// estimateRangeLabel matches spec section 35's own display format
// ("Estimated: 3-5 hours") - a ±range around the template's base
// estimate rather than false single-number precision, since these are
// necessarily rough.
func estimateRangeLabel(baseMinutes int) string {
	if baseMinutes <= 0 {
		return ""
	}
	lo := int(float64(baseMinutes) * 0.8)
	hi := int(math.Ceil(float64(baseMinutes) * 1.3))
	if lo < 1 {
		lo = 1
	}
	return fmt.Sprintf("%s - %s", shortDuration(lo), shortDuration(hi))
}

func shortDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	hours := float64(minutes) / 60
	if hours == math.Trunc(hours) {
		return fmt.Sprintf("%d hours", int(hours))
	}
	return fmt.Sprintf("%.1f hours", hours)
}
