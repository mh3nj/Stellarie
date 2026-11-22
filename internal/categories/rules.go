package categories

import "strings"

// RuleType is which field a Rule matches against. Identical vocabulary
// to internal/projects.RuleType (spec section 65's example "IF domain =
// youtube.com THEN category = Entertainment" is a category rule, not a
// project rule) - kept as its own type rather than importing
// internal/projects' to avoid coupling two otherwise-independent
// packages over a handful of shared string constants, matching this
// codebase's established internal/ convention (internal/idea,
// internal/patterns, internal/waste, internal/calendar, internal/search,
// internal/myyear all define their own minimal input structs rather
// than importing a sibling package - see the handbook's architectural
// conventions).
type RuleType string

const (
	RuleProcess             RuleType = "process"
	RuleWindowTitleContains RuleType = "window_title_contains"
	RulePathContains        RuleType = "path_contains"
	RuleDomain              RuleType = "domain"
)

// Rule is one category association rule - the category-scoped sibling
// of internal/projects.Rule. Before this file, categories could only be
// assigned via an application's default_category_id or an explicit
// per-application override (see Repository.CategoryForApplication);
// this adds the same pattern-matching rule engine projects already had
// (spec section 65 names categories explicitly, not just projects).
type Rule struct {
	CategoryID int64
	Type       RuleType
	Pattern    string
	Priority   int // lower evaluates first
}

// Sample is what a rule gets evaluated against - identical shape and
// same rationale as internal/projects.Sample (see that file's doc
// comment for why RulePathContains/RuleDomain are approximated/
// unimplemented rather than silently faked).
type Sample struct {
	ProcessName string
	WindowTitle string
}

// Evaluate returns the CategoryID of the first rule (in the given
// slice's order - callers sort by Priority ascending) that matches
// sample, or 0 if none do. Same pure, order-preserving contract as
// internal/projects.Evaluate.
func Evaluate(rules []Rule, sample Sample) int64 {
	for _, r := range rules {
		if matches(r, sample) {
			return r.CategoryID
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
		// window title, same documented approximation as
		// internal/projects' identical case.
		return containsFold(sample.WindowTitle, r.Pattern)
	case RuleDomain:
		// Browser URL tracking is off by default and unimplemented until
		// Phase 9 (section 28) - same fail-closed reasoning as
		// internal/projects' identical case: never match on window-title
		// text that happens to contain a domain string.
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
