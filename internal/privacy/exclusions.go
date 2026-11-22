package privacy

import "strings"

// Context is what a single moment's exclusion decision is made from
// deliberately just the fields the Activity Engine's own State already
// has (process name, window title) plus the two per-application privacy
// flags, so this package never needs its own copy of "what's currently
// happening" logic.
type Context struct {
	ProcessName       string
	WindowTitle       string
	PrivacySensitive  bool // from the application's own registry row (section 27's "password managers, banking apps")
	ScreenshotAllowed bool // application-level screenshot_allowed column; false always blocks a screenshot regardless of exclusion rules
}

// IsScreenshotBlocked reports whether a screenshot must NOT be taken
// right now - section 27's own list, applied in order:
//  1. The application's own screenshot_allowed flag (an explicit
//     per-app override always wins, on or off).
//  2. privacy_sensitive (password managers, banking apps - section 27's
//     own examples) is an automatic block regardless of exclusion list
//     contents, since these are inherently sensitive by nature, not by
//     someone remembering to add a rule for them.
//  3. Any matching Exclusion whose AppliesTo is "all" or "screenshots".
//
// Privacy Mode itself and "tracking paused" are NOT checked here - the
// sweeper checks those separately (via activity.State) before ever
// calling this function, since they're session-wide states, not
// per-application ones.
func IsScreenshotBlocked(ctx Context, exclusions []Exclusion) bool {
	if !ctx.ScreenshotAllowed {
		return true
	}
	if ctx.PrivacySensitive {
		return true
	}
	return matchesAny(ctx, exclusions, AppliesToScreenshots)
}

// IsBrowserTrackingBlocked mirrors IsScreenshotBlocked for section 28's
// domain tracking - kept as its own function (not a shared "IsBlocked"
// with a mode parameter) because the two features don't share the same
// per-application flag: screenshots gate on ScreenshotAllowed, browser
// tracking gates on the application's own BrowserURLTracking flag,
// which the caller (services.PrivacyService) resolves before building
// Context.
func IsBrowserTrackingBlocked(ctx Context, browserURLTrackingEnabled bool, exclusions []Exclusion) bool {
	if !browserURLTrackingEnabled {
		return true
	}
	if ctx.PrivacySensitive {
		return true
	}
	return matchesAny(ctx, exclusions, AppliesToBrowserTracking)
}

func matchesAny(ctx Context, exclusions []Exclusion, feature AppliesTo) bool {
	for _, ex := range exclusions {
		if ex.AppliesTo != AppliesToAll && ex.AppliesTo != feature {
			continue
		}
		if matchesOne(ctx, ex) {
			return true
		}
	}
	return false
}

func matchesOne(ctx Context, ex Exclusion) bool {
	switch ex.Type {
	case ExclusionApplication:
		return strings.EqualFold(ctx.ProcessName, ex.Pattern)
	case ExclusionWindowTitlePattern:
		return ex.Pattern != "" && strings.Contains(strings.ToLower(ctx.WindowTitle), strings.ToLower(ex.Pattern))
	case ExclusionDomain:
		// Same honest limitation as internal/projects' RuleDomain
		// (spec section 65) and domain.go's own doc comment: there's no
		// reliable real domain signal to match a domain exclusion
		// against yet, so this never matches rather than guessing off
		// the window title and risking a FALSE negative that lets a
		// screenshot through when the person explicitly tried to
		// exclude a sensitive site.
		return false
	default:
		return false
	}
}
