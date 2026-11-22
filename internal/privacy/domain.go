package privacy

import "regexp"

// ExtractDomain is section 28's browser URL/domain tracking - and the
// most important scope note in this whole package.
//
// STELLAREI does NOT have real browser URL/domain data. The Activity
// Engine only ever sees a window's TITLE (spec section 6's own field
// list - no address bar, no page URL), and ordinary browsers in their
// default configuration put the page's TITLE in the window title, not
// its domain ("GitHub - stellarei: personal time..." not
// "github.com"). Reliable domain tracking needs one of: a browser
// extension talking to STELLAREI over native messaging, or an OS
// accessibility API reading the address bar directly. Neither exists in
// this codebase - building either is a real, separate integration
// project, not something this function can shortcut its way to.
//
// What this function actually is: a narrow, conservative regex applied
// to whatever window title is available, for the minority of cases
// where a domain-shaped string genuinely does appear in it - some
// browsers-in-app-mode, some PWAs, some Electron wrappers, and any
// title a person has manually included a URL in. It returns ok=false
// for the vast majority of ordinary browser tabs, which is the honest
// and correct behavior, not a bug: returning nothing is far better than
// a confident-looking wrong guess for a privacy feature. Not wired into
// live session/domain tracking anywhere in this phase (no domain column
// exists on activity_events/sessions to store a result in even if
// extracted) - see internal/projects.RuleDomain's own identical honesty
// note (spec section 65) for the matching decision on the project-rules
// side of the same limitation.
func ExtractDomain(windowTitle string) (domain string, ok bool) {
	match := domainPattern.FindString(windowTitle)
	if match == "" {
		return "", false
	}
	return match, true
}

// domainPattern matches a conservative "looks like a real domain"
// shape: at least two dot-separated labels, letters/digits/hyphens
// only, a plausible top-level domain (2-24 letters - long enough to
// cover ordinary and newer gTLDs without matching arbitrary
// dot-separated text). Deliberately does NOT try to match a full URL
// (scheme, path, query) - a window title is far more likely to contain
// a bare domain-looking fragment than a well-formed URL.
var domainPattern = regexp.MustCompile(`\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,24}\b`)
