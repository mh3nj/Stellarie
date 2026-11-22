package patterns

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// MinSamplesForTypicalStart is the fewest sessions a category needs
// before TypicalStartTimes will report anything for it. Below this, a
// "usual" start time is just wherever the few samples happened to fall,
// not a real pattern.
const MinSamplesForTypicalStart = 5

// TypicalStartTimes reports, for each category with at least
// MinSamplesForTypicalStart sessions in samples, the median clock time
// its sessions started - spec section 31's own example: "You usually
// start development around 10:30." Median rather than mean specifically
// because start times are typically skewed (one very-early or
// very-late outlier session shouldn't swing what counts as "usual"),
// and "usually... around" never implies more precision than a median
// across scattered samples actually has.
func TypicalStartTimes(samples []SessionSample, loc *time.Location) []Insight {
	byCategory := groupByCategory(samples)
	var out []Insight
	for _, catID := range sortedKeys(byCategory) {
		group := byCategory[catID]
		if len(group) < MinSamplesForTypicalStart {
			continue
		}
		minutes := make([]int, 0, len(group))
		for _, s := range group {
			minutes = append(minutes, minuteOfDay(s.StartedAt, loc))
		}
		out = append(out, Insight{
			Kind:         "typical_start",
			CategoryID:   catID,
			CategoryName: group[0].CategoryName,
			Text:         fmt.Sprintf("STELLAREI observed you usually start %s around %s.", group[0].CategoryName, clockLabel(medianInt(minutes))),
		})
	}
	return out
}

// EveningThresholdHour is the hour (local, 24h) TimeOfDayLengthPatterns
// splits each category's sessions on - spec section 31's own example
// hour ("...occur after 20:00").
const EveningThresholdHour = 20

// MinSamplesForTimeOfDayPattern is the fewest sessions a category needs
// on EACH side of EveningThresholdHour before a comparison is reported.
const MinSamplesForTimeOfDayPattern = 5

// MinPercentDiffForTimeOfDayPattern is the smallest before/after average
// difference worth reporting - below this, the difference is more
// likely sampling noise than a real pattern (spec section 66: "never
// fabricate patterns" applied to trivial differences, not just outright
// invention).
const MinPercentDiffForTimeOfDayPattern = 20.0

// TimeOfDayLengthPatterns reports, for each category with enough
// samples on both sides of EveningThresholdHour, how average session
// length compares before vs. at-or-after that hour - spec section 31's
// two related examples: "Your longest design sessions occur after
// 20:00" and "...are 42% longer on average." Only ever reports the
// side that's actually longer; never claims why.
func TimeOfDayLengthPatterns(samples []SessionSample, loc *time.Location) []Insight {
	type bucket struct {
		beforeTotal, afterTotal int64
		beforeCount, afterCount int
	}
	byCategory := map[int64]*bucket{}
	names := map[int64]string{}
	for _, s := range samples {
		if s.CategoryID == 0 {
			continue
		}
		b, ok := byCategory[s.CategoryID]
		if !ok {
			b = &bucket{}
			byCategory[s.CategoryID] = b
		}
		names[s.CategoryID] = s.CategoryName
		if minuteOfDay(s.StartedAt, loc) >= EveningThresholdHour*60 {
			b.afterTotal += s.DurationSeconds()
			b.afterCount++
		} else {
			b.beforeTotal += s.DurationSeconds()
			b.beforeCount++
		}
	}

	var out []Insight
	for _, catID := range sortedKeys(byCategory) {
		b := byCategory[catID]
		if b.beforeCount < MinSamplesForTimeOfDayPattern || b.afterCount < MinSamplesForTimeOfDayPattern {
			continue
		}
		beforeAvg := float64(b.beforeTotal) / float64(b.beforeCount)
		afterAvg := float64(b.afterTotal) / float64(b.afterCount)
		if beforeAvg <= 0 || afterAvg <= 0 {
			continue
		}
		percentDiff := (afterAvg - beforeAvg) / beforeAvg * 100
		if math.Abs(percentDiff) < MinPercentDiffForTimeOfDayPattern {
			continue
		}

		name := names[catID]
		var text string
		if percentDiff > 0 {
			text = fmt.Sprintf("STELLAREI observed %s sessions started at or after %02d:00 are %.0f%% longer on average than earlier ones.",
				name, EveningThresholdHour, percentDiff)
		} else {
			text = fmt.Sprintf("STELLAREI observed %s sessions started before %02d:00 are %.0f%% longer on average than later ones.",
				name, EveningThresholdHour, math.Abs(percentDiff))
		}
		out = append(out, Insight{Kind: "time_of_day_length", CategoryID: catID, CategoryName: name, Text: text})
	}
	return out
}

// MinSwitchesForTrend is the fewest context switches this week needs
// before ContextSwitchTrend reports anything - a handful of switches
// can swing an average wildly, and section 66 applies here too.
const MinSwitchesForTrend = 3

// MinPercentDiffForTrend is the smallest week-over-week change worth
// reporting.
const MinPercentDiffForTrend = 15.0

// ContextSwitchTrend compares this week's average context-switch
// interval against last week's - spec section 31's example: "Your
// average context-switching interval increased this week." Takes
// pre-computed averages rather than raw samples, deliberately: unlike
// the two functions above, context-switch detection already has a real
// implementation (internal/sessions' own switch-counting logic, exposed
// via AnalyticsService.GetContextSwitchStats) that this package must
// not reimplement a second, possibly-inconsistent copy of.
func ContextSwitchTrend(thisWeekAvgSeconds, lastWeekAvgSeconds int64, thisWeekSwitchCount int) (Insight, bool) {
	if thisWeekSwitchCount < MinSwitchesForTrend || lastWeekAvgSeconds <= 0 {
		return Insight{}, false
	}
	percentChange := float64(thisWeekAvgSeconds-lastWeekAvgSeconds) / float64(lastWeekAvgSeconds) * 100
	if math.Abs(percentChange) < MinPercentDiffForTrend {
		return Insight{}, false
	}
	direction := "increased"
	if percentChange < 0 {
		direction = "decreased"
	}
	text := fmt.Sprintf("STELLAREI observed your average context-switching interval %s %.0f%% this week compared to last week.",
		direction, math.Abs(percentChange))
	return Insight{Kind: "context_switch_trend", Text: text}, true
}

// DiscoverAll runs every pattern-detection function above and returns
// every Insight with enough data to support it, in a stable order
// (typical-start insights, then time-of-day-length insights, then the
// context-switch trend if present) - never randomized, so the frontend
// list doesn't reshuffle between otherwise-identical calls.
func DiscoverAll(samples []SessionSample, loc *time.Location, thisWeekAvgSwitchSeconds, lastWeekAvgSwitchSeconds int64, thisWeekSwitchCount int) []Insight {
	var out []Insight
	out = append(out, TypicalStartTimes(samples, loc)...)
	out = append(out, TimeOfDayLengthPatterns(samples, loc)...)
	if trend, ok := ContextSwitchTrend(thisWeekAvgSwitchSeconds, lastWeekAvgSwitchSeconds, thisWeekSwitchCount); ok {
		out = append(out, trend)
	}
	return out
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

func sortedKeys[V any](m map[int64]V) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// medianInt returns the median of a non-empty slice, without mutating
// the caller's slice.
func medianInt(values []int) int {
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
