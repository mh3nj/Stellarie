// Package waste implements Digital Waste Detection (spec section 33):
// a plain, deterministic, entirely AI-free analysis of real session
// data that presents evidence, never verdicts - spec's own words:
// "Do not make moral judgments. Present evidence." Every Finding this
// package produces is a factual count or duration, phrased the same
// observational way internal/patterns' insights are ("STELLAREI
// observed..."), never "you wasted time on..." or any framing that
// implies the person did something wrong.
//
// Honest scope note: spec section 33 lists six example patterns to
// detect. This package covers three of them:
//
//   - "repeatedly opening the same application" → RepeatedReopens
//   - "long idle periods" → LongIdlePeriods
//   - "sessions without meaningful activity" → ShortNoiseSessions
//
// The other three are deliberately NOT reimplemented here:
//
//   - "rapid switching" already has a real implementation
//     AnalyticsService.GetContextSwitchStats - and this package has no
//     business computing a second, possibly-inconsistent copy of that
//     same math (the same reasoning internal/patterns.ContextSwitchTrend
//     already applies to the identical question).
//   - "excessive entertainment time" is exactly what Budgets (spec
//     section 17, already built) exist to let the PERSON define for
//     themselves. Inventing a fixed "excessive" threshold here - with
//     no budget configured, no context on what's normal for that
//     person - is precisely the kind of moral judgment section 33
//     explicitly forbids: "3 hours of gaming" is not evidence of
//     anything without the person's own sense of what's too much.
//   - "repeated navigation" needs browser URL/domain tracking, which
//     spec section 28 makes OFF by default - this package has nothing
//     to analyze for it unless that's separately enabled, and doesn't
//     pretend otherwise.
package waste

import "sort"

// SessionSample is the minimal per-session data this package needs
// decoupled from internal/sessions the same way internal/idea and
// internal/patterns each independently are (see their own package
// docs). The caller (app/services/waste_service.go) builds these from
// sessions.View.
type SessionSample struct {
	CategoryID   int64
	CategoryName string
	StartedAt    int64 // unix seconds
	EndedAt      int64 // unix seconds
}

func (s SessionSample) DurationSeconds() int64 {
	if s.EndedAt <= s.StartedAt {
		return 0
	}
	return s.EndedAt - s.StartedAt
}

// Finding is one piece of evidence. Kind identifies which detector
// produced it. Text is the complete, display-ready, non-judgmental
// sentence - this package never returns raw numbers for the frontend to
// phrase itself, for the same reason internal/patterns.Insight doesn't:
// the exact wording (observational, evidence-only, no verdict) is the
// entire point.
type Finding struct {
	Kind         string `json:"kind"`
	CategoryID   int64  `json:"categoryId,omitempty"`
	CategoryName string `json:"categoryName,omitempty"`
	Text         string `json:"text"`
}

func groupByCategory(samples []SessionSample) map[int64][]SessionSample {
	out := map[int64][]SessionSample{}
	for _, s := range samples {
		if s.CategoryID == 0 {
			continue
		}
		out[s.CategoryID] = append(out[s.CategoryID], s)
	}
	return out
}

func sortedCategoryIDs(m map[int64][]SessionSample) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
