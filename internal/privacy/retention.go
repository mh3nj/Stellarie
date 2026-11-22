package privacy

import "time"

// RetentionDays is section 27's own set of options. 0 means "never"
// no automatic deletion.
type RetentionDays int

const (
	RetentionOneDay    RetentionDays = 1
	RetentionSevenDays RetentionDays = 7
	RetentionThirty    RetentionDays = 30
	RetentionNinety    RetentionDays = 90
	RetentionNever     RetentionDays = 0
)

// ComputeDeleteAfter returns when a screenshot taken at takenAt should
// be purged, or nil for RetentionNever - stored directly in
// screenshots.delete_after so the sweep (ShouldPurge below) never has
// to recompute the retention policy from scratch for old rows even if
// the person changes their retention SETTING later; each screenshot
// keeps the deletion date that was in effect when it was taken, exactly
// the way section 47 ("Do not delete historical summaries merely
// because raw events were purged") treats retention as a per-record
// decision, not a live recompute.
func ComputeDeleteAfter(takenAt time.Time, retention RetentionDays) *time.Time {
	if retention == RetentionNever {
		return nil
	}
	when := takenAt.AddDate(0, 0, int(retention))
	return &when
}

// ShouldPurge reports whether a screenshot's stored DeleteAfter has
// arrived. A nil DeleteAfter (RetentionNever, or an old row from before
// retention was configured) is never purged.
func ShouldPurge(s Screenshot, now time.Time) bool {
	if s.DeleteAfter == nil {
		return false
	}
	return !now.Before(*s.DeleteAfter)
}
