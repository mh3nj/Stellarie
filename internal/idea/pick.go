package idea

import (
	"fmt"
	"math/rand"
	"sort"
)

// genericFallbacks are evergreen, non-personalized ideas used only when
// there aren't enough real candidates to fill out suggestions (e.g. a
// fresh install with no projects yet) - an honest empty-state filler
// (section 67: excellent empty states, never fake data), not a
// personalized recommendation. Their Reasons never claim anything about
// the person's own data, since Score only ever adds a reason when it
// found a real signal (time fit, tag match, neglect) to report.
var genericFallbacks = []Candidate{
	{Kind: KindGeneric, Title: "Start your first project", Description: "Give STELLAREI something to track - head to Projects.", EstimatedMinutes: 10, Tags: []Intent{IntentProductive}},
	{Kind: KindGeneric, Title: "Set a goal for this week", Description: "A small, concrete target is easier to start than a vague one.", EstimatedMinutes: 5, Tags: []Intent{IntentProductive, IntentLearn}},
	{Kind: KindGeneric, Title: "Explore something you've been curious about", Description: "No pressure - just poke at it for a bit.", EstimatedMinutes: 30, Tags: []Intent{IntentLearn, IntentCreative}},
}

// jitterFlat is how much random noise gets added to each eligible
// candidate's score before ranking. Keeps "I HAVE NO IDEA" from
// suggesting the literal same top candidates every single time under
// identical inputs - the person asked because they didn't know, not
// because they wanted a deterministic leaderboard - while staying small
// enough that a genuinely much-better-scoring candidate (e.g. a strong
// intent match plus heavy neglect, easily 5+ points) still wins most of
// the time.
const jitterFlat = 0.75

type scored struct {
	Candidate
	score   float64
	reasons []string
}

// PickSuggestions produces up to `count` non-chaos suggestions plus,
// if chaosPool is non-empty, exactly one chaos suggestion appended
// last - matching spec section 34's own example layout (two real ideas,
// then CHAOS as the third). recentTitles/recentChaos are exact-title
// exclusions drawn from idea_history/chaos_history (section 34's
// "previous suggestions" input, section 36's own history requirement)
// - if excluding them would leave too few candidates to satisfy
// `count`, they're allowed back in rather than returning fewer
// suggestions than asked for (repeating an idea occasionally is better
// than running out of ideas entirely).
func PickSuggestions(pool []Candidate, chaosPool []Candidate, availableMinutes TimeMinutes, intent Intent, count int, recentTitles map[string]bool, recentChaos map[string]bool, rng *rand.Rand) []Suggestion {
	rng = rngOrDefault(rng)

	if len(pool) == 0 {
		pool = genericFallbacks
	}

	picked := pickN(pool, availableMinutes, intent, count, recentTitles, rng)
	out := make([]Suggestion, 0, count+1)
	for _, p := range picked {
		out = append(out, toSuggestion(p))
	}

	if len(chaosPool) > 0 {
		if chaos := pickN(chaosPool, availableMinutes, intent, 1, recentChaos, rng); len(chaos) == 1 {
			out = append(out, toSuggestion(chaos[0]))
		}
	}

	return out
}

func pickN(pool []Candidate, availableMinutes TimeMinutes, intent Intent, n int, recent map[string]bool, rng *rand.Rand) []scored {
	scoredAll := scoreAll(pool, availableMinutes, intent)
	if len(scoredAll) == 0 {
		return nil
	}

	withoutRecent := excludeRecent(scoredAll, recent)
	if len(withoutRecent) < n {
		withoutRecent = scoredAll // see PickSuggestions' doc comment on the fallback
	}

	for i := range withoutRecent {
		withoutRecent[i].score += rng.Float64() * jitterFlat
	}
	sort.SliceStable(withoutRecent, func(i, j int) bool { return withoutRecent[i].score > withoutRecent[j].score })

	if n > len(withoutRecent) {
		n = len(withoutRecent)
	}
	return withoutRecent[:n]
}

func scoreAll(pool []Candidate, availableMinutes TimeMinutes, intent Intent) []scored {
	out := make([]scored, 0, len(pool))
	for _, c := range pool {
		score, reasons, ok := Score(c, availableMinutes, intent)
		if !ok {
			continue
		}
		out = append(out, scored{Candidate: c, score: score, reasons: reasons})
	}
	return out
}

func excludeRecent(in []scored, recent map[string]bool) []scored {
	if len(recent) == 0 {
		return in
	}
	out := make([]scored, 0, len(in))
	for _, s := range in {
		if !recent[s.Title] {
			out = append(out, s)
		}
	}
	return out
}

func toSuggestion(s scored) Suggestion {
	return Suggestion{
		Candidate:             s.Candidate,
		EstimatedMinutesLabel: minutesLabel(s.EstimatedMinutes),
		Reasons:               s.reasons,
	}
}

// minutesLabel matches spec section 34's own display format ("~40m").
func minutesLabel(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes < 60 {
		return fmt.Sprintf("~%dm", minutes)
	}
	h, m := minutes/60, minutes%60
	if m == 0 {
		return fmt.Sprintf("~%dh", h)
	}
	return fmt.Sprintf("~%dh %dm", h, m)
}
