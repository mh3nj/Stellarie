// Package privacy implements the Privacy Center (spec sections 26-28):
// Privacy Mode, Screenshot Memory, per-application privacy settings, and
// the exclusion list both features share. Browser URL/domain tracking's
// own honest scope note lives in domain.go - read that before assuming
// this package does more than it does.
package privacy

import "time"

// ExclusionType is privacy_exclusions.exclusion_type.
type ExclusionType string

const (
	ExclusionApplication        ExclusionType = "application"
	ExclusionWindowTitlePattern ExclusionType = "window_title_pattern"
	ExclusionDomain             ExclusionType = "domain"
)

// AppliesTo is privacy_exclusions.applies_to - which feature(s) an
// exclusion rule blocks.
type AppliesTo string

const (
	AppliesToAll             AppliesTo = "all"
	AppliesToScreenshots     AppliesTo = "screenshots"
	AppliesToBrowserTracking AppliesTo = "browser_tracking"
)

// Exclusion mirrors a privacy_exclusions row.
type Exclusion struct {
	ID        int64
	Type      ExclusionType
	Pattern   string
	AppliesTo AppliesTo
	CreatedAt time.Time
}

// Screenshot mirrors a screenshots row.
type Screenshot struct {
	ID            int64
	TakenAt       time.Time
	FilePath      string
	SessionID     int64 // 0 = none
	ApplicationID int64 // 0 = unknown
	DeleteAfter   *time.Time
}

// ApplicationPrivacy is the privacy-relevant subset of an applications
// row (spec section 10's own field list) - deliberately separate from
// internal/categories.ApplicationCategory, which owns the
// category-assignment subset of the same table; each package touches
// only the columns its own feature area is responsible for.
type ApplicationPrivacy struct {
	ApplicationID      int64
	ProcessName        string
	TrackingEnabled    bool
	ScreenshotAllowed  bool
	BrowserURLTracking bool
	PrivacySensitive   bool // e.g. password managers, banking apps (section 27's own examples)
}
