package privacy

import "testing"

func TestExtractDomain_FindsADomainShapedFragment(t *testing.T) {
	domain, ok := ExtractDomain("github.com - stellarei repository")
	if !ok {
		t.Fatal("expected a match for a title containing a domain-shaped fragment")
	}
	if domain != "github.com" {
		t.Fatalf("got %q, want \"github.com\"", domain)
	}
}

func TestExtractDomain_OrdinaryPageTitleFindsNothing(t *testing.T) {
	// The realistic, common case: an ordinary browser tab title has no
	// domain in it at all - this MUST return ok=false, not a wrong
	// guess. This is the single most important test in this file.
	_, ok := ExtractDomain("Inbox (4) - John Doe - Mail")
	if ok {
		t.Fatal("an ordinary page title with no domain-shaped text must not produce a false match")
	}
}

func TestExtractDomain_EmptyTitle(t *testing.T) {
	_, ok := ExtractDomain("")
	if ok {
		t.Fatal("an empty title must never match")
	}
}

func TestExtractDomain_SingleWordDoesNotMatch(t *testing.T) {
	_, ok := ExtractDomain("Settings")
	if ok {
		t.Fatal("a single word with no dot must never match")
	}
}

func TestExtractDomain_VersionNumberDoesNotFalsePositive(t *testing.T) {
	// A decimal version number ("2.5") looks superficially similar to a
	// two-label domain but has a numeric-only "TLD", which the pattern's
	// letters-only final segment must reject.
	_, ok := ExtractDomain("MyApp 2.5 - Release Notes")
	if ok {
		t.Fatal("a version number must not be mistaken for a domain")
	}
}

func TestExtractDomain_SubdomainMatches(t *testing.T) {
	domain, ok := ExtractDomain("docs.claude.com/en/api - Overview")
	if !ok {
		t.Fatal("expected a match")
	}
	if domain != "docs.claude.com" {
		t.Fatalf("got %q, want \"docs.claude.com\"", domain)
	}
}

func TestExtractDomain_PicksFirstMatchWhenMultiple(t *testing.T) {
	domain, ok := ExtractDomain("Comparing example.com and other.org")
	if !ok {
		t.Fatal("expected a match")
	}
	if domain != "example.com" {
		t.Fatalf("got %q, want the first match \"example.com\"", domain)
	}
}
