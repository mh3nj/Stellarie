package projects

import "testing"

func TestEvaluate_ProcessRuleMatchesCaseInsensitively(t *testing.T) {
	rules := []Rule{{ProjectID: 1, Type: RuleProcess, Pattern: "Code.exe"}}
	got := Evaluate(rules, Sample{ProcessName: "code.exe", WindowTitle: "main.go"})
	if got != 1 {
		t.Errorf("Evaluate() = %d, want 1", got)
	}
}

func TestEvaluate_ProcessRuleRequiresExactMatch(t *testing.T) {
	rules := []Rule{{ProjectID: 1, Type: RuleProcess, Pattern: "Code.exe"}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe.backup", WindowTitle: ""})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (process rule is exact-match, not substring)", got)
	}
}

func TestEvaluate_WindowTitleContainsMatchesSubstring(t *testing.T) {
	rules := []Rule{{ProjectID: 2, Type: RuleWindowTitleContains, Pattern: "Serenity"}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe", WindowTitle: "main.go - Serenity - Visual Studio Code"})
	if got != 2 {
		t.Errorf("Evaluate() = %d, want 2", got)
	}
}

func TestEvaluate_WindowTitleContainsIsCaseInsensitive(t *testing.T) {
	rules := []Rule{{ProjectID: 2, Type: RuleWindowTitleContains, Pattern: "serenity"}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe", WindowTitle: "SERENITY - main.go"})
	if got != 2 {
		t.Errorf("Evaluate() = %d, want 2", got)
	}
}

func TestEvaluate_EmptyPatternNeverMatches(t *testing.T) {
	rules := []Rule{{ProjectID: 2, Type: RuleWindowTitleContains, Pattern: ""}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe", WindowTitle: "anything at all"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (an empty pattern must never match everything)", got)
	}
}

func TestEvaluate_PathContainsApproximatesAgainstWindowTitle(t *testing.T) {
	rules := []Rule{{ProjectID: 3, Type: RulePathContains, Pattern: "HeartLib"}}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe", WindowTitle: "index.ts - HeartLib"})
	if got != 3 {
		t.Errorf("Evaluate() = %d, want 3", got)
	}
}

func TestEvaluate_DomainRuleNeverMatchesYet(t *testing.T) {
	rules := []Rule{{ProjectID: 4, Type: RuleDomain, Pattern: "github.com"}}
	got := Evaluate(rules, Sample{ProcessName: "chrome.exe", WindowTitle: "github.com/user/repo"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 - domain rules can't match without browser URL tracking (Phase 9)", got)
	}
}

func TestEvaluate_FirstMatchingRuleWinsInGivenOrder(t *testing.T) {
	// Evaluate does not sort - callers order by priority. This test
	// fabricates an order deliberately different from insertion order to
	// prove Evaluate respects it rather than re-sorting internally.
	rules := []Rule{
		{ProjectID: 20, Type: RuleProcess, Pattern: "Code.exe", Priority: 50},
		{ProjectID: 10, Type: RuleProcess, Pattern: "Code.exe", Priority: 10},
	}
	got := Evaluate(rules, Sample{ProcessName: "Code.exe"})
	if got != 20 {
		t.Errorf("Evaluate() = %d, want 20 (the first rule in the given slice, regardless of its Priority field)", got)
	}
}

func TestEvaluate_NoMatchReturnsZero(t *testing.T) {
	rules := []Rule{{ProjectID: 1, Type: RuleProcess, Pattern: "FL Studio.exe"}}
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
	rules := []Rule{{ProjectID: 1, Type: RuleType("something_future"), Pattern: "anything"}}
	got := Evaluate(rules, Sample{ProcessName: "anything", WindowTitle: "anything"})
	if got != 0 {
		t.Errorf("Evaluate() = %d, want 0 (unrecognized rule types must fail closed, not match everything)", got)
	}
}
