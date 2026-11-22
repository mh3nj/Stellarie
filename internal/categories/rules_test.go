package categories

import "testing"

func TestEvaluate_ProcessRuleMatchesCaseInsensitively(t *testing.T) {
	rules := []Rule{{CategoryID: 1, Type: RuleProcess, Pattern: "Steam.exe"}}
	got := Evaluate(rules, Sample{ProcessName: "steam.exe", WindowTitle: "Steam"})
	if got != 1 {
		t.Errorf("Evaluate() = %d, want 1", got)
	}
}

func TestEvaluate_ProcessRuleRequiresExactMatch(t *testing.T) {
	rules := []Rule{{CategoryID: 1, Type: RuleProcess, Pattern: "Steam.exe"}}
	got := Evaluate(rules, Sample{ProcessName: "Steam.exe.backup", WindowTitle: ""})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (process rule is exact-match, not substring)", got)
	}
}

func TestEvaluate_WindowTitleContainsMatchesSubstring(t *testing.T) {
	rules := []Rule{{CategoryID: 2, Type: RuleWindowTitleContains, Pattern: "YouTube"}}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe", WindowTitle: "Cat videos - YouTube - Google Chrome"})
	if got != 2 {
		t.Errorf("Evaluate() = %d, want 2", got)
	}
}

func TestEvaluate_WindowTitleContainsIsCaseInsensitive(t *testing.T) {
	rules := []Rule{{CategoryID: 2, Type: RuleWindowTitleContains, Pattern: "youtube"}}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe", WindowTitle: "YOUTUBE - cat videos"})
	if got != 2 {
		t.Errorf("Evaluate() = %d, want 2", got)
	}
}

func TestEvaluate_EmptyPatternNeverMatches(t *testing.T) {
	rules := []Rule{{CategoryID: 2, Type: RuleWindowTitleContains, Pattern: ""}}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe", WindowTitle: "anything at all"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (an empty pattern must never match everything)", got)
	}
}

func TestEvaluate_PathContainsApproximatesAgainstWindowTitle(t *testing.T) {
	rules := []Rule{{CategoryID: 3, Type: RulePathContains, Pattern: "Invoices"}}
	got := Evaluate(rules, Sample{ProcessName: "explorer.exe", WindowTitle: "Invoices - File Explorer"})
	if got != 3 {
		t.Errorf("Evaluate() = %d, want 3", got)
	}
}

func TestEvaluate_DomainRuleNeverMatchesYet(t *testing.T) {
	rules := []Rule{{CategoryID: 4, Type: RuleDomain, Pattern: "youtube.com"}}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe", WindowTitle: "youtube.com/watch?v=xyz"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 - domain rules can't match without browser URL tracking (Phase 9)", got)
	}
}

func TestEvaluate_FirstMatchingRuleWinsInGivenOrder(t *testing.T) {
	// Evaluate does not sort - callers order by priority. Deliberately
	// fabricates an order different from insertion order to prove
	// Evaluate respects it rather than re-sorting internally.
	rules := []Rule{
		{CategoryID: 20, Type: RuleProcess, Pattern: "chrome.exe", Priority: 50},
		{CategoryID: 10, Type: RuleProcess, Pattern: "chrome.exe", Priority: 10},
	}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe"})
	if got != 20 {
		t.Errorf("Evaluate() = %d, want 20 (the first rule in the given slice, regardless of its Priority field)", got)
	}
}

func TestEvaluate_NoMatchReturnsZero(t *testing.T) {
	rules := []Rule{{CategoryID: 1, Type: RuleProcess, Pattern: "Discord.exe"}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe", WindowTitle: "main.go"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0", got)
	}
}

func TestEvaluate_EmptyRuleSetReturnsZero(t *testing.T) {
	got := Evaluate(nil, Sample{ProcessName: "Code.exe"})
	if got != 0 {
		t.Errorf("Evaluate(nil, ...) = %d, want 0", got)
	}
}

func TestEvaluate_UnknownRuleTypeNeverMatches(t *testing.T) {
	rules := []Rule{{CategoryID: 1, Type: RuleType("something_future"), Pattern: "anything"}}
	got := Evaluate(rules, Sample{ProcessName: "anything", WindowTitle: "anything"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (unrecognized rule types must fail closed, not match everything)", got)
	}
}
