package idea

import "fmt"

// fitsTime reports whether a candidate's estimate is reasonable for the
// available time. TimeWhatever (0) accepts anything. A candidate with
// no estimate of its own (EstimatedMinutes == 0) always fits - there's
// nothing to reject it on. Otherwise, a candidate may run up to 50%
// over the available time (a 40-minute idea when you asked for 30 is
// still a reasonable suggestion - spec section 34's own example shows
// exactly this: "YOU HAVE 45 MINUTES" / "1. Finish Serenity Settings
// ~40m" fits comfortably); anything beyond that is excluded rather than
// scored low, because suggesting a 3-hour project idea for a 15-minute
// break isn't a worse fit, it's not a fit.
func fitsTime(estimatedMinutes int, availableMinutes TimeMinutes) bool {
	if availableMinutes == TimeWhatever || estimatedMinutes <= 0 {
		return true
	}
	return float64(estimatedMinutes) <= float64(availableMinutes)*1.5
}

// Score rates a candidate against the person's stated time and intent,
// returning both the numeric score (used only to rank candidates
// against each other - the raw number is never shown to the person)
// and the human-readable reasons that produced it, in the order they
// were found - pick.go uses these reasons verbatim as a Suggestion's
// Reasons.
//
// ok is false when the candidate should not be offered at all (fails
// the time-fit check) - callers must filter these out entirely, not
// just rank them last.
func Score(c Candidate, availableMinutes TimeMinutes, intent Intent) (score float64, reasons []string, ok bool) {
	if !fitsTime(c.EstimatedMinutes, availableMinutes) {
		return 0, nil, false
	}

	if availableMinutes != TimeWhatever && c.EstimatedMinutes > 0 {
		if c.EstimatedMinutes <= int(availableMinutes) {
			score += 2
			reasons = append(reasons, fmt.Sprintf("fits your %d minutes", int(availableMinutes)))
		} else {
			// Within the 1.5x soft cap but over the exact ask - still a
			// real fit, just a smaller bonus and a more honest reason.
			score += 1
			reasons = append(reasons, fmt.Sprintf("close to your %d minutes", int(availableMinutes)))
		}
	}

	// "Surprise me" deliberately gets no intent-tag bonus - every
	// eligible candidate is equally likely on its merits (time fit +
	// neglect), and pick.go's own random jitter does the rest. Every
	// other intent rewards a genuine tag match.
	if intent != IntentSurprise {
		for _, tag := range c.Tags {
			if tag == intent {
				score += 3
				reasons = append(reasons, fmt.Sprintf("matches %q", string(intent)))
				break
			}
		}
	}

	if c.NeglectedDays > 0 {
		// Capped at 3 points (roughly 3 weeks' worth at this rate) so a
		// project neglected for a year doesn't completely dominate every
		// other signal - it's a nudge toward "you haven't touched this
		// in a while", not the only thing that matters.
		neglectScore := float64(c.NeglectedDays) / 7
		if neglectScore > 3 {
			neglectScore = 3
		}
		score += neglectScore
		reasons = append(reasons, fmt.Sprintf("you haven't touched this in %d days", c.NeglectedDays))
	}

	return score, reasons, true
}
