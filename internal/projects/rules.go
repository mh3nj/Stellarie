// Package projects implements the Project System (spec section 11) and
// its rule engine (section 65): matching a session's process name and
// window title against user-defined rules to assign a project
// automatically, plus the CRUD backing Theme-Studio-style management of
// projects and their rules.
package projects

import "strings"

// RuleType is which field a Rule matches against. Kept as a Go type
// (not a raw string) so Evaluate's switch is exhaustive-checkable, even
// though the underlying storage (project_rules.rule_type) is TEXT.
type RuleType string

const (
	RuleProcess             RuleType = "process"
	RuleWindowTitleContains RuleType = "window_title_contains"
	RulePathContains        RuleType = "path_contains"
	RuleDomain              RuleType = "domain"
)

// Rule is one project association rule (section 65's example: "IF
// process = FL Studio THEN category = Music" - here specialized to
// projects specifically, since that's Phase 3's scope; category rules
// are Phase 4's Analytics work).
type Rule struct {
	ProjectID int64
	Type      RuleType
	Pattern   string
	Priority  int // lower evaluates first
}

// Sample is what a rule gets evaluated against - deliberately just the
// two fields the Activity Engine actually has (process name, window
// title). No folder path or browser domain tracking exists yet (browser
// URL tracking is Phase 9, off by default per section 28), so
// RulePathContains falls back to matching against the window title (many
// apps show a file path there) and RuleDomain never matches - see
// Evaluate's doc comment.
type Sample struct {
	ProcessName string
	WindowTitle string
}

// Evaluate returns the ProjectID of the first rule (in the given slice's
// order - callers are responsible for sorting by Priority ascending,
// typically via the repository's own ORDER BY) that matches sample, or 0
// if none do. Rule priority/ordering (section 65) is the caller's
// responsibility precisely so this function stays a pure, order-preserving
// scan with no hidden sort.
func Evaluate(rules []Rule, sample Sample) int64 {
	for _, r := range rules {
		if matches(r, sample) {
			return r.ProjectID
		}
	}
	return 0
}

func matches(r Rule, sample Sample) bool {
	switch r.Type {
	case RuleProcess:
		return strings.EqualFold(sample.ProcessName, r.Pattern)
	case RuleWindowTitleContains:
		return containsFold(sample.WindowTitle, r.Pattern)
	case RulePathContains:
		// No folder/path tracking exists yet - approximated against the
		// window title, which many applications populate with a file
		// path (editors, IDEs, file managers). Documented approximation,
		// not silently pretending to have real path data.
		return containsFold(sample.WindowTitle, r.Pattern)
	case RuleDomain:
		// Browser URL tracking is off by default and unimplemented until
		// Phase 9 (section 28) - a domain rule can exist (the UI lets you
		// create one) but can never match anything yet. Returning false
		// here, rather than matching on window title, avoids a
		// misleading false-positive project assignment from a browser
		// tab title that happens to contain the domain string.
		return false
	default:
		return false
	}
}

func containsFold(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
