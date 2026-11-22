package waste

import "fmt"

// MinSessionsForRepeatedReopens is the fewest sessions a category needs
// within the analysis window before RepeatedReopens will report on it.
const MinSessionsForRepeatedReopens = 20

// MaxAvgMinutesForRepeatedReopens caps how long, on average, those
// sessions can be and still count as "repeated reopening" rather than
// ordinary sustained use - 25 sessions of Development averaging 45
// minutes each is a normal workweek, not a waste pattern; the same 25
// sessions averaging 90 seconds each is the pattern spec section 33
// actually means.
const MaxAvgMinutesForRepeatedReopens = 5

// RepeatedReopens reports, for each category with at least
// MinSessionsForRepeatedReopens sessions in samples AND a short average
// duration, how many times it was opened and how short those sessions
// were - spec section 33's "repeatedly opening the same application".
// Presents the count and average as evidence only; never says this is
// bad, wasteful, or worth stopping (section 33: "do not make moral
// judgments").
func RepeatedReopens(samples []SessionSample) []Finding {
	byCategory := groupByCategory(samples)
	var out []Finding
	for _, catID := range sortedCategoryIDs(byCategory) {
		group := byCategory[catID]
		if len(group) < MinSessionsForRepeatedReopens {
			continue
		}
		var totalSeconds int64
		for _, s := range group {
			totalSeconds += s.DurationSeconds()
		}
		avgMinutes := float64(totalSeconds) / float64(len(group)) / 60
		if avgMinutes > MaxAvgMinutesForRepeatedReopens {
			continue
		}
		out = append(out, Finding{
			Kind:         "repeated_reopens",
			CategoryID:   catID,
			CategoryName: group[0].CategoryName,
			Text: fmt.Sprintf("STELLAREI observed %s was opened %d times, averaging %.1f minutes per session.",
				group[0].CategoryName, len(group), avgMinutes),
		})
	}
	return out
}

// LongIdleThresholdMinutes is how long an idle session must run before
// LongIdlePeriods counts it - spec section 9's own default idle
// threshold (2 minutes) marks the START of idle; this is a much longer
// bar for a specific STRETCH of idle time to be worth surfacing as a
// pattern, not every brief pause.
const LongIdleThresholdMinutes = 30

// MinCountForLongIdlePeriods is the fewest qualifying idle stretches
// needed before this is reported as a pattern rather than one
// unremarkable long break.
const MinCountForLongIdlePeriods = 3

// LongIdlePeriods reports how many idle stretches of at least
// LongIdleThresholdMinutes occurred, and their total duration - spec
// section 33's "long idle periods". idleCategoryID is resolved by the
// caller (the builtin "Idle" category - see migrations/0004's seed and
// waste_service.go), not hardcoded here, so a person who renames or
// reorganizes categories doesn't silently break this detector.
func LongIdlePeriods(samples []SessionSample, idleCategoryID int64) (Finding, bool) {
	if idleCategoryID == 0 {
		return Finding{}, false
	}
	var count int
	var totalSeconds int64
	for _, s := range samples {
		if s.CategoryID != idleCategoryID {
			continue
		}
		if s.DurationSeconds() < LongIdleThresholdMinutes*60 {
			continue
		}
		count++
		totalSeconds += s.DurationSeconds()
	}
	if count < MinCountForLongIdlePeriods {
		return Finding{}, false
	}
	return Finding{
		Kind: "long_idle_periods",
		Text: fmt.Sprintf("STELLAREI observed %d idle periods of %d+ minutes, totaling %s.",
			count, LongIdleThresholdMinutes, formatHoursMinutes(totalSeconds)),
	}, true
}

// ShortNoiseSessionMaxSeconds is how short a session must be to count
// as "noise" rather than real, if brief, activity.
const ShortNoiseSessionMaxSeconds = 60

// MinCountForShortNoiseSessions is the fewest such sessions needed
// before this is reported as a pattern.
const MinCountForShortNoiseSessions = 15

// ShortNoiseSessions reports how many sessions under
// ShortNoiseSessionMaxSeconds occurred - spec section 33's "sessions
// without meaningful activity". Idle sessions are excluded: idle time
// under a minute is completely unremarkable and isn't what this
// detector means; LongIdlePeriods above is the one that looks at idle
// time specifically.
func ShortNoiseSessions(samples []SessionSample, idleCategoryID int64) (Finding, bool) {
	var count int
	for _, s := range samples {
		if s.CategoryID == idleCategoryID {
			continue
		}
		if s.DurationSeconds() > 0 && s.DurationSeconds() < ShortNoiseSessionMaxSeconds {
			count++
		}
	}
	if count < MinCountForShortNoiseSessions {
		return Finding{}, false
	}
	return Finding{
		Kind: "short_noise_sessions",
		Text: fmt.Sprintf("STELLAREI observed %d sessions under a minute long - often rapid switching rather than focused activity.", count),
	}, true
}

// DetectAll runs every detector above and returns every Finding with
// enough evidence to support it, in a stable order (repeated-reopens
// findings, then long idle periods, then short noise sessions).
func DetectAll(samples []SessionSample, idleCategoryID int64) []Finding {
	var out []Finding
	out = append(out, RepeatedReopens(samples)...)
	if f, ok := LongIdlePeriods(samples, idleCategoryID); ok {
		out = append(out, f)
	}
	if f, ok := ShortNoiseSessions(samples, idleCategoryID); ok {
		out = append(out, f)
	}
	return out
}

func formatHoursMinutes(totalSeconds int64) string {
	totalMinutes := totalSeconds / 60
	h := totalMinutes / 60
	m := totalMinutes % 60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
