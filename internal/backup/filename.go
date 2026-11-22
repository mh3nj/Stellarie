package backup

import "time"

// Filename returns a sortable, collision-resistant backup/export
// filename - second-precision timestamp in the name means two backups
// started in the same process within the same second (shouldn't happen
// in practice, since these are person-triggered actions, but "Backup
// Now" clicked twice fast is plausible) would collide; CreateSnapshot's
// own caller is responsible for retrying with a fresh time if Filename's
// caller needs a guaranteed-unique name - this function itself stays
// pure and just formats what it's given.
func Filename(now time.Time, prefix, ext string) string {
	return prefix + "-" + now.UTC().Format("20060102-150405") + "." + ext
}
