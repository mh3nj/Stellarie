package ai

import "testing"

func TestScopeSettingKey(t *testing.T) {
	cases := map[Scope]string{
		ScopeSummaries:    "ai_scope_summaries",
		ScopePatterns:     "ai_scope_patterns",
		ScopeSuggestions:  "ai_scope_suggestions",
		ScopeProjectIdeas: "ai_scope_project_ideas",
		ScopeIHaveNoIdea:  "ai_scope_i_have_no_idea",
		ScopeDailyReviews: "ai_scope_daily_reviews",
	}
	for scope, want := range cases {
		if got := ScopeSettingKey(scope); got != want {
			t.Errorf("ScopeSettingKey(%q) = %q, want %q", scope, got, want)
		}
	}
}

// Every scope must map to a settings key distinct from every other scope
// and from the four fixed keys - a collision here would mean two
// different toggles silently sharing one row.
func TestScopeSettingKey_AllDistinct(t *testing.T) {
	seen := map[string]Scope{
		SettingKeyEnabled:  "",
		SettingKeyProvider: "",
		SettingKeyEndpoint: "",
		SettingKeyModel:    "",
	}
	for _, scope := range AllScopes {
		key := ScopeSettingKey(scope)
		if existing, ok := seen[key]; ok {
			t.Fatalf("key collision: scope %q and %q both map to %q", scope, existing, key)
		}
		seen[key] = scope
	}
}
