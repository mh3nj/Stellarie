// Package idea implements "I HAVE NO IDEA" (spec sections 34-36):
// three time-and-intent-aware suggestions drawn from real STELLAREI
// data, a combinatorial project-idea generator, and Chaos Mode.
//
// Deliberately, entirely AI-free. Section 37 is unambiguous: "AI must
// NEVER be required for the core product. STELLAREI must function
// completely without AI." Everything in this package is a plain,
// deterministic (modulo an injected *rand.Rand - never the global RNG,
// so every decision here is reproducible in a test) scoring and
// combination function over real data the caller assembles from
// internal/projects, internal/goals, and internal/sessions. There is no
// LLM call anywhere in this package, on purpose - local AI enhancement
// of "I HAVE NO IDEA" is explicitly a LATER, opt-in layer (spec section
// 37, roadmap Phase 11), not something this phase depends on.
package idea

import "math/rand"

// TimeMinutes is "How much time do you have?" (section 34). 0 means
// "Whatever" - no time constraint on what gets suggested.
type TimeMinutes int

const (
	Time15       TimeMinutes = 15
	Time30       TimeMinutes = 30
	Time60       TimeMinutes = 60
	Time120      TimeMinutes = 120
	TimeWhatever TimeMinutes = 0
)

// Intent is "What do you want?" (section 34).
type Intent string

const (
	IntentProductive Intent = "productive"
	IntentCreative   Intent = "creative"
	IntentFun        Intent = "fun"
	IntentLearn      Intent = "learn"
	IntentSurprise   Intent = "surprise"
)

func ValidIntent(s string) bool {
	switch Intent(s) {
	case IntentProductive, IntentCreative, IntentFun, IntentLearn, IntentSurprise:
		return true
	default:
		return false
	}
}

// Kind distinguishes what a Candidate/Suggestion actually points at
// the frontend uses this to decide what "Start" means (open a project,
// open a goal, or just show the chaos challenge text).
type Kind string

const (
	KindProject Kind = "project"
	KindGoal    Kind = "goal"
	KindChaos   Kind = "chaos"
	KindGeneric Kind = "generic" // a non-personalized fallback idea - see pick.go's doc comment on empty candidate pools
)

// Candidate is one thing that COULD be suggested. Built entirely by the
// CALLER (services.IdeaService) from real data - this package never
// touches a database or invents a fact about the person's projects; it
// only scores and selects among whatever Candidates it's handed. Every
// field here must be something the caller can actually back with real
// data; there is no field for "why this might interest you" that isn't
// derivable from NeglectedDays/Tags/EstimatedMinutes.
type Candidate struct {
	Kind             Kind
	ProjectID        int64 // set for KindProject and KindGoal (the goal's own linked project, if any)
	GoalID           int64 // set for KindGoal
	Title            string
	Description      string
	EstimatedMinutes int      // the caller's best real estimate; 0 means "no particular estimate" (still eligible under TimeWhatever)
	Tags             []Intent // which intents this candidate genuinely fits - a project's Tags come from its category (see IdeaService), a chaos challenge's are baked into chaosbank.go
	NeglectedDays    int      // days since last touched, from real session data (internal/sessions.LastActivityByProject) - 0 if unknown or touched very recently
}

// Suggestion is a Candidate plus why it was picked - the "why" is
// assembled entirely from Score's own reasoning, never free-form prose
// invented after the fact (section 66: never invent statistics, and the
// same discipline applies to reasoning about a person's own data).
type Suggestion struct {
	Candidate
	EstimatedMinutesLabel string   // e.g. "~40m" - spec section 34's own display format
	Reasons               []string // e.g. ["fits your 30 minutes", "matches \"creative\"", "you haven't touched this in 12 days"]
}

// rngOrDefault returns rng if non-nil, otherwise a fresh, time-seeded
// source - callers doing real generation pass nil (or their own
// process-lifetime *rand.Rand) and get real variety; tests pass a
// fixed-seed *rand.Rand for determinism.
func rngOrDefault(rng *rand.Rand) *rand.Rand {
	if rng != nil {
		return rng
	}
	return rand.New(rand.NewSource(rand.Int63()))
}
