package idea

import "testing"

func TestScore_ExcludesCandidateFarOverTime(t *testing.T) {
	c := Candidate{EstimatedMinutes: 180}
	_, _, ok := Score(c, Time30, IntentProductive)
	if ok {
		t.Fatal("a 3-hour candidate must be excluded for a 30-minute ask")
	}
}

func TestScore_AllowsSoftOverage(t *testing.T) {
	// 40 minutes for a 30-minute ask is within the 1.5x soft cap
	// (45 min) - spec section 34's own example does exactly this.
	c := Candidate{EstimatedMinutes: 40}
	_, reasons, ok := Score(c, Time30, IntentProductive)
	if !ok {
		t.Fatal("40 minutes must fit a 30-minute ask (within the 1.5x soft cap)")
	}
	found := false
	for _, r := range reasons {
		if r == "close to your 30 minutes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 'close to' reason, got %v", reasons)
	}
}

func TestScore_ExactFitScoresHigherThanCloseFit(t *testing.T) {
	exact := Candidate{EstimatedMinutes: 25}
	close_ := Candidate{EstimatedMinutes: 40}
	exactScore, _, _ := Score(exact, Time30, IntentProductive)
	closeScore, _, _ := Score(close_, Time30, IntentProductive)
	if exactScore <= closeScore {
		t.Fatalf("exact fit (%v) should score higher than a close-but-over fit (%v)", exactScore, closeScore)
	}
}

func TestScore_NoEstimateAlwaysFits(t *testing.T) {
	c := Candidate{EstimatedMinutes: 0}
	_, _, ok := Score(c, Time15, IntentProductive)
	if !ok {
		t.Fatal("a candidate with no estimate should never be excluded on time grounds")
	}
}

func TestScore_WhateverAcceptsAnyDuration(t *testing.T) {
	c := Candidate{EstimatedMinutes: 600}
	_, _, ok := Score(c, TimeWhatever, IntentProductive)
	if !ok {
		t.Fatal("TimeWhatever must accept any duration")
	}
}

func TestScore_IntentMatchAddsBonus(t *testing.T) {
	matching := Candidate{EstimatedMinutes: 30, Tags: []Intent{IntentCreative}}
	nonMatching := Candidate{EstimatedMinutes: 30, Tags: []Intent{IntentLearn}}
	matchScore, reasons, _ := Score(matching, Time30, IntentCreative)
	nonMatchScore, _, _ := Score(nonMatching, Time30, IntentCreative)
	if matchScore <= nonMatchScore {
		t.Fatalf("a matching-intent candidate (%v) should outscore a non-matching one (%v)", matchScore, nonMatchScore)
	}
	found := false
	for _, r := range reasons {
		if r == `matches "creative"` {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an intent-match reason, got %v", reasons)
	}
}

func TestScore_SurpriseGivesNoIntentBonus(t *testing.T) {
	matching := Candidate{EstimatedMinutes: 30, Tags: []Intent{IntentCreative}}
	nonMatching := Candidate{EstimatedMinutes: 30, Tags: []Intent{IntentLearn}}
	matchScore, _, _ := Score(matching, Time30, IntentSurprise)
	nonMatchScore, _, _ := Score(nonMatching, Time30, IntentSurprise)
	if matchScore != nonMatchScore {
		t.Fatalf("\"surprise\" must not favor any particular tag: got %v vs %v", matchScore, nonMatchScore)
	}
}

func TestScore_NeglectAddsBonusAndReason(t *testing.T) {
	neglected := Candidate{EstimatedMinutes: 30, NeglectedDays: 14}
	fresh := Candidate{EstimatedMinutes: 30, NeglectedDays: 0}
	neglectedScore, reasons, _ := Score(neglected, Time30, IntentProductive)
	freshScore, _, _ := Score(fresh, Time30, IntentProductive)
	if neglectedScore <= freshScore {
		t.Fatalf("a neglected candidate (%v) should outscore a fresh one (%v)", neglectedScore, freshScore)
	}
	found := false
	for _, r := range reasons {
		if r == "you haven't touched this in 14 days" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a neglect reason, got %v", reasons)
	}
}

func TestScore_NeglectBonusIsCapped(t *testing.T) {
	moderatelyNeglected := Candidate{EstimatedMinutes: 30, NeglectedDays: 21} // exactly at the 3-point cap
	extremelyNeglected := Candidate{EstimatedMinutes: 30, NeglectedDays: 900}
	moderateScore, _, _ := Score(moderatelyNeglected, Time30, IntentProductive)
	extremeScore, _, _ := Score(extremelyNeglected, Time30, IntentProductive)
	if moderateScore != extremeScore {
		t.Fatalf("neglect bonus should be capped: 21 days (%v) should score the same as 900 days (%v)", moderateScore, extremeScore)
	}
}
