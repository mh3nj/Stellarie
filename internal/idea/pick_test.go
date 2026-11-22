package idea

import (
	"math/rand"
	"strings"
	"testing"
)

func fixedRNG() *rand.Rand { return rand.New(rand.NewSource(42)) }

func TestPickSuggestions_ReturnsCountPlusOneChaos(t *testing.T) {
	pool := []Candidate{
		{Title: "A", EstimatedMinutes: 20},
		{Title: "B", EstimatedMinutes: 20},
		{Title: "C", EstimatedMinutes: 20},
	}
	out := PickSuggestions(pool, chaosBank, Time30, IntentProductive, 2, nil, nil, fixedRNG())
	if len(out) != 3 {
		t.Fatalf("expected 2 + 1 chaos = 3 suggestions, got %d", len(out))
	}
	if out[2].Kind != KindChaos {
		t.Fatalf("expected the 3rd (last) suggestion to be chaos, got %v", out[2].Kind)
	}
}

func TestPickSuggestions_NoChaosPoolMeansNoChaosSlot(t *testing.T) {
	pool := []Candidate{{Title: "A", EstimatedMinutes: 20}, {Title: "B", EstimatedMinutes: 20}}
	out := PickSuggestions(pool, nil, Time30, IntentProductive, 2, nil, nil, fixedRNG())
	if len(out) != 2 {
		t.Fatalf("expected exactly 2 suggestions with no chaos pool, got %d", len(out))
	}
	for _, s := range out {
		if s.Kind == KindChaos {
			t.Fatal("must not include a chaos suggestion when chaosPool is empty")
		}
	}
}

func TestPickSuggestions_EmptyPoolFallsBackToGeneric(t *testing.T) {
	out := PickSuggestions(nil, nil, Time30, IntentProductive, 2, nil, nil, fixedRNG())
	if len(out) != 2 {
		t.Fatalf("expected 2 fallback suggestions, got %d", len(out))
	}
	for _, s := range out {
		if s.Kind != KindGeneric {
			t.Fatalf("expected KindGeneric fallbacks, got %v", s.Kind)
		}
		// Time-fit ("fits your 30 minutes") and intent-tag-match
		// ("matches \"productive\"") reasons are fine here - both are
		// objectively true of the fallback candidate itself, not a
		// claim about the person's own data. What must never appear is
		// a NEGLECT reason, since that specifically claims something
		// about the person's own history that a generic fallback (with
		// no real NeglectedDays) has no basis for.
		for _, r := range s.Reasons {
			if strings.Contains(r, "haven't touched") {
				t.Fatalf("a generic fallback must never claim a neglect-based reason, got %v", s.Reasons)
			}
		}
	}
}

func TestPickSuggestions_ExcludesCandidatesThatDontFitTime(t *testing.T) {
	pool := []Candidate{
		{Title: "Fits", EstimatedMinutes: 20},
		{Title: "TooLong", EstimatedMinutes: 300},
	}
	out := PickSuggestions(pool, nil, Time30, IntentProductive, 2, nil, nil, fixedRNG())
	for _, s := range out {
		if s.Title == "TooLong" {
			t.Fatal("a candidate far outside the time budget must never be suggested")
		}
	}
}

func TestPickSuggestions_RecentTitlesExcludedWhenEnoughAlternatives(t *testing.T) {
	pool := []Candidate{
		{Title: "A", EstimatedMinutes: 20},
		{Title: "B", EstimatedMinutes: 20},
		{Title: "C", EstimatedMinutes: 20},
	}
	recent := map[string]bool{"A": true}
	out := PickSuggestions(pool, nil, Time30, IntentProductive, 2, recent, nil, fixedRNG())
	for _, s := range out {
		if s.Title == "A" {
			t.Fatal("a recently-suggested title must be excluded when enough alternatives exist")
		}
	}
}

func TestPickSuggestions_RecentTitlesAllowedBackWhenPoolTooSmall(t *testing.T) {
	pool := []Candidate{
		{Title: "OnlyOption", EstimatedMinutes: 20},
	}
	recent := map[string]bool{"OnlyOption": true}
	// count=1 but excluding the only candidate would leave zero - must
	// fall back to allowing it rather than returning nothing.
	out := PickSuggestions(pool, nil, Time30, IntentProductive, 1, recent, nil, fixedRNG())
	if len(out) != 1 {
		t.Fatalf("expected 1 suggestion (falling back to the excluded candidate), got %d", len(out))
	}
}

func TestPickSuggestions_ChaosHistoryExcludedSameWay(t *testing.T) {
	small := []Candidate{{Kind: KindChaos, Title: "Only chaos", EstimatedMinutes: 20}, {Kind: KindChaos, Title: "Other chaos", EstimatedMinutes: 20}}
	pool := []Candidate{{Title: "A", EstimatedMinutes: 20}}
	recentChaos := map[string]bool{"Only chaos": true}
	out := PickSuggestions(pool, small, Time30, IntentProductive, 1, nil, recentChaos, fixedRNG())
	last := out[len(out)-1]
	if last.Title == "Only chaos" {
		t.Fatal("a recently-shown chaos challenge should be excluded when an alternative exists")
	}
}

func TestPickSuggestions_HigherScoringCandidatePickedMoreOften(t *testing.T) {
	// Not a single deterministic assertion (jitter is real randomness by
	// design - see pick.go's own doc comment on jitterFlat) but a
	// statistical one: across many trials with different seeds, a
	// candidate with a large, genuine scoring advantage (matches intent
	// AND heavily neglected) should be picked far more often than a
	// candidate with no advantages at all.
	strong := Candidate{Title: "Strong", EstimatedMinutes: 20, Tags: []Intent{IntentCreative}, NeglectedDays: 60}
	weak := Candidate{Title: "Weak", EstimatedMinutes: 20}
	pool := []Candidate{strong, weak}

	strongCount := 0
	trials := 200
	for i := 0; i < trials; i++ {
		out := PickSuggestions(pool, nil, Time30, IntentCreative, 1, nil, nil, rand.New(rand.NewSource(int64(i))))
		if len(out) == 1 && out[0].Title == "Strong" {
			strongCount++
		}
	}
	if strongCount < trials*8/10 {
		t.Fatalf("expected the strongly-scoring candidate to win at least 80%% of trials, got %d/%d", strongCount, trials)
	}
}

func TestPickSuggestions_ChaosBankHasNoEmptyTitles(t *testing.T) {
	// A basic sanity check on the static bank itself, since nothing else
	// exercises every entry - an empty/malformed entry would otherwise
	// only surface as a confusing blank suggestion in the UI.
	if len(chaosBank) < 10 {
		t.Fatalf("expected a reasonably large chaos bank, got %d entries", len(chaosBank))
	}
	for _, c := range chaosBank {
		if c.Title == "" {
			t.Fatal("found a chaos bank entry with an empty title")
		}
		if c.Kind != KindChaos {
			t.Fatalf("chaos bank entry %q has Kind %v, want KindChaos", c.Title, c.Kind)
		}
		if len(c.Tags) == 0 {
			t.Fatalf("chaos bank entry %q has no intent tags", c.Title)
		}
	}
}
