package idea

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateProjectIdea_NeverEmpty(t *testing.T) {
	got := GenerateProjectIdea(ProjectIdeaInput{}, nil, fixedRNG())
	if got.Title == "" || got.Description == "" || got.Difficulty == "" {
		t.Fatalf("expected a complete idea even with a zero-value input, got %+v", got)
	}
}

func TestGenerateProjectIdea_RespectsTimeFit(t *testing.T) {
	// 30 minutes is comfortably above every template's minimum, so a
	// mid-size time budget should never return the largest (240-minute)
	// template.
	for i := 0; i < 50; i++ {
		got := GenerateProjectIdea(ProjectIdeaInput{AvailableMinutes: Time30}, nil, rand.New(rand.NewSource(int64(i))))
		softCap := float64(Time30) * 1.5
		if float64(got.EstimatedMinutes) > softCap {
			t.Fatalf("trial %d: got a %d-minute idea for a 30-minute ask", i, got.EstimatedMinutes)
		}
	}
}

func TestGenerateProjectIdea_TooShortForAnyTemplateFallsBackToSmallest(t *testing.T) {
	// Every template in the bank needs at least 30 real minutes - none
	// fit Time15's soft cap of 22.5 minutes. The honest fallback (see
	// GenerateProjectIdea's doc comment) is always the single
	// smallest-scoped template in the bank, deterministically - never a
	// uniformly random pick that could hand back the largest one.
	var smallest projectTemplate
	for _, tpl := range projectTemplates {
		if smallest.Title == "" || tpl.BaseMinutes < smallest.BaseMinutes {
			smallest = tpl
		}
	}

	for i := 0; i < 20; i++ {
		got := GenerateProjectIdea(ProjectIdeaInput{AvailableMinutes: Time15}, nil, rand.New(rand.NewSource(int64(i))))
		if got.Title != smallest.Title {
			t.Fatalf("trial %d: expected the smallest template %q, got %q (%d min)", i, smallest.Title, got.Title, got.EstimatedMinutes)
		}
	}
}

func TestGenerateProjectIdea_PlatformFilter(t *testing.T) {
	for i := 0; i < 30; i++ {
		got := GenerateProjectIdea(ProjectIdeaInput{Platform: "cli"}, nil, rand.New(rand.NewSource(int64(i))))
		var tpl projectTemplate
		found := false
		for _, c := range projectTemplates {
			if c.Title == got.Title {
				tpl, found = c, true
			}
		}
		if !found {
			t.Fatalf("trial %d: returned title %q doesn't match any known template", i, got.Title)
		}
		if !templateFitsPlatform(tpl, "cli") {
			t.Fatalf("trial %d: %q doesn't fit the \"cli\" platform filter", i, got.Title)
		}
	}
}

func TestGenerateProjectIdea_NeglectedProjectGroundsWhy(t *testing.T) {
	got := GenerateProjectIdea(ProjectIdeaInput{NeglectedProject: "Serenity"}, nil, fixedRNG())
	if !strings.Contains(got.Why, "Serenity") {
		t.Fatalf("expected the Why line to reference the real neglected project, got %q", got.Why)
	}
	if !strings.Contains(got.Description, "Serenity") {
		t.Fatalf("expected the description's subject to reference the real neglected project, got %q", got.Description)
	}
}

func TestGenerateProjectIdea_RecentCategoryGroundsWhyWhenNoNeglectedProject(t *testing.T) {
	got := GenerateProjectIdea(ProjectIdeaInput{RecentCategory: "Music"}, nil, fixedRNG())
	if !strings.Contains(got.Why, "Music") {
		t.Fatalf("expected the Why line to reference the real recent category, got %q", got.Why)
	}
}

func TestGenerateProjectIdea_NeglectedProjectTakesPriorityOverCategory(t *testing.T) {
	got := GenerateProjectIdea(ProjectIdeaInput{NeglectedProject: "Serenity", RecentCategory: "Music"}, nil, fixedRNG())
	if !strings.Contains(got.Why, "Serenity") {
		t.Fatalf("expected the more specific neglected-project signal to win, got %q", got.Why)
	}
}

func TestGenerateProjectIdea_NoRealSignalStaysHonest(t *testing.T) {
	got := GenerateProjectIdea(ProjectIdeaInput{}, nil, fixedRNG())
	// Must not claim anything that looks like it came from real data
	// when none was given.
	if strings.Contains(got.Why, "haven't touched") || strings.Contains(got.Why, "been active in") {
		t.Fatalf("expected a neutral Why with no real signal, got %q", got.Why)
	}
}

func TestGenerateProjectIdea_RecentTitlesExcludedWhenAlternativesExist(t *testing.T) {
	recent := map[string]bool{}
	for _, tpl := range projectTemplates[:len(projectTemplates)-1] { // exclude all but the last template
		recent[tpl.Title] = true
	}
	last := projectTemplates[len(projectTemplates)-1]
	for i := 0; i < 20; i++ {
		got := GenerateProjectIdea(ProjectIdeaInput{}, recent, rand.New(rand.NewSource(int64(i))))
		if got.Title != last.Title {
			t.Fatalf("trial %d: expected the only non-excluded template %q, got %q", i, last.Title, got.Title)
		}
	}
}

func TestEstimateRangeLabel(t *testing.T) {
	label := estimateRangeLabel(240)
	if label == "" {
		t.Fatal("expected a non-empty estimate label for a positive duration")
	}
	if estimateRangeLabel(0) != "" {
		t.Fatal("expected an empty label for a zero/unknown duration")
	}
}

func TestShortDuration(t *testing.T) {
	if got := shortDuration(45); got != "45 min" {
		t.Fatalf("got %q, want \"45 min\"", got)
	}
	if got := shortDuration(120); got != "2 hours" {
		t.Fatalf("got %q, want \"2 hours\"", got)
	}
	if got := shortDuration(90); got != "1.5 hours" {
		t.Fatalf("got %q, want \"1.5 hours\"", got)
	}
}

func TestProjectTemplates_AllHaveOnePlaceholder(t *testing.T) {
	for _, tpl := range projectTemplates {
		if strings.Count(tpl.DescriptionFmt, "%s") != 1 {
			t.Fatalf("template %q must have exactly one %%s placeholder, has description %q", tpl.Title, tpl.DescriptionFmt)
		}
		if tpl.Title == "" || tpl.DefaultSubject == "" || tpl.BaseMinutes <= 0 || tpl.Difficulty == "" {
			t.Fatalf("template %q is missing a required field: %+v", tpl.Title, tpl)
		}
	}
}
