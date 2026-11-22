package privacy

import "testing"

func TestIsScreenshotBlocked_ScreenshotAllowedFalseAlwaysBlocks(t *testing.T) {
	ctx := Context{ProcessName: "Code.exe", ScreenshotAllowed: false}
	if !IsScreenshotBlocked(ctx, nil) {
		t.Fatal("ScreenshotAllowed=false must always block, regardless of exclusions")
	}
}

func TestIsScreenshotBlocked_PrivacySensitiveAlwaysBlocks(t *testing.T) {
	ctx := Context{ProcessName: "1Password.exe", ScreenshotAllowed: true, PrivacySensitive: true}
	if !IsScreenshotBlocked(ctx, nil) {
		t.Fatal("a privacy_sensitive application must always block, even with no exclusion rules configured")
	}
}

func TestIsScreenshotBlocked_NotBlockedByDefault(t *testing.T) {
	ctx := Context{ProcessName: "Code.exe", ScreenshotAllowed: true}
	if IsScreenshotBlocked(ctx, nil) {
		t.Fatal("an ordinary, allowed, non-sensitive application must not be blocked with no exclusions")
	}
}

func TestIsScreenshotBlocked_ApplicationExclusionMatches(t *testing.T) {
	ctx := Context{ProcessName: "Steam.exe", ScreenshotAllowed: true}
	exclusions := []Exclusion{{Type: ExclusionApplication, Pattern: "steam.exe", AppliesTo: AppliesToScreenshots}}
	if !IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("expected a case-insensitive application exclusion match to block")
	}
}

func TestIsScreenshotBlocked_ExclusionAppliesToOtherFeatureDoesNotBlock(t *testing.T) {
	ctx := Context{ProcessName: "Chrome.exe", ScreenshotAllowed: true}
	exclusions := []Exclusion{{Type: ExclusionApplication, Pattern: "chrome.exe", AppliesTo: AppliesToBrowserTracking}}
	if IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("an exclusion scoped to browser_tracking must not block screenshots")
	}
}

func TestIsScreenshotBlocked_AppliesToAllBlocksBothFeatures(t *testing.T) {
	exclusions := []Exclusion{{Type: ExclusionApplication, Pattern: "signal.exe", AppliesTo: AppliesToAll}}
	ctx := Context{ProcessName: "Signal.exe", ScreenshotAllowed: true}
	if !IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("an AppliesToAll exclusion must block screenshots")
	}
	if !IsBrowserTrackingBlocked(ctx, true, exclusions) {
		t.Fatal("an AppliesToAll exclusion must also block browser tracking")
	}
}

func TestIsScreenshotBlocked_WindowTitlePatternMatches(t *testing.T) {
	ctx := Context{ProcessName: "chrome.exe", WindowTitle: "My Bank - Account Overview", ScreenshotAllowed: true}
	exclusions := []Exclusion{{Type: ExclusionWindowTitlePattern, Pattern: "my bank", AppliesTo: AppliesToScreenshots}}
	if !IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("expected a case-insensitive substring window-title match to block")
	}
}

func TestIsScreenshotBlocked_EmptyWindowTitlePatternNeverMatches(t *testing.T) {
	ctx := Context{ProcessName: "chrome.exe", WindowTitle: "anything", ScreenshotAllowed: true}
	exclusions := []Exclusion{{Type: ExclusionWindowTitlePattern, Pattern: "", AppliesTo: AppliesToScreenshots}}
	if IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("an empty pattern must never match everything")
	}
}

func TestIsScreenshotBlocked_DomainExclusionNeverMatches(t *testing.T) {
	// Honest limitation (see exclusions.go's own comment) - must never
	// match, since there's no reliable domain signal to check yet.
	ctx := Context{ProcessName: "chrome.exe", WindowTitle: "example.com - some page", ScreenshotAllowed: true}
	exclusions := []Exclusion{{Type: ExclusionDomain, Pattern: "example.com", AppliesTo: AppliesToScreenshots}}
	if IsScreenshotBlocked(ctx, exclusions) {
		t.Fatal("a domain exclusion must never match yet - no real domain signal exists to check it against")
	}
}

func TestIsBrowserTrackingBlocked_DisabledGloballyAlwaysBlocks(t *testing.T) {
	ctx := Context{ProcessName: "chrome.exe"}
	if !IsBrowserTrackingBlocked(ctx, false, nil) {
		t.Fatal("browser tracking disabled for this application must always block")
	}
}

func TestIsBrowserTrackingBlocked_EnabledAndNotExcludedAllows(t *testing.T) {
	ctx := Context{ProcessName: "chrome.exe"}
	if IsBrowserTrackingBlocked(ctx, true, nil) {
		t.Fatal("browser tracking enabled with no exclusions must not be blocked")
	}
}

func TestIsBrowserTrackingBlocked_PrivacySensitiveAlwaysBlocks(t *testing.T) {
	ctx := Context{ProcessName: "chrome.exe", PrivacySensitive: true}
	if !IsBrowserTrackingBlocked(ctx, true, nil) {
		t.Fatal("a privacy_sensitive application must block browser tracking even when enabled")
	}
}
