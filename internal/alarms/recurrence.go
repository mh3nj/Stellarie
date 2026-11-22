package alarms

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Recurrence is a parsed alarms.recurrence_rule. The stored string format
// is a small, deliberately RRULE-*flavored* (not full RFC 5545 - section
// 46 only asks for "a simple RRULE-like string") key=value grammar,
// semicolon-separated:
//
//	FREQ=DAILY;TIME=09:00
//	FREQ=WEEKLY;BYDAY=MO,WE,FR;TIME=18:30
//	FREQ=WEEKDAYS;TIME=09:00        (shorthand for BYDAY=MO,TU,WE,TH,FR)
//
// TIME is required and is 24-hour HH:MM in the user's local time (the
// same local-day convention internal/periods uses - section 70).
// BYDAY is only meaningful for FREQ=WEEKLY; omitting it means "every
// day", making FREQ=WEEKLY;TIME=09:00 with no BYDAY equivalent to
// FREQ=DAILY.
type Recurrence struct {
	Hour, Minute int
	Weekdays     []time.Weekday // empty = every day
}

// ParseRecurrenceRule parses the format documented on Recurrence.
// Returns an error for anything malformed rather than guessing - a
// silently-misparsed alarm that fires at the wrong time (or never) is
// far worse than a rejected save the UI can surface immediately.
func ParseRecurrenceRule(rule string) (Recurrence, error) {
	fields := map[string]string{}
	for _, part := range strings.Split(rule, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return Recurrence{}, fmt.Errorf("alarms: malformed recurrence segment %q", part)
		}
		fields[strings.ToUpper(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
	}

	timeStr, ok := fields["TIME"]
	if !ok {
		return Recurrence{}, fmt.Errorf("alarms: recurrence rule missing TIME=HH:MM")
	}
	hour, minute, err := parseClock(timeStr)
	if err != nil {
		return Recurrence{}, err
	}

	freq := strings.ToUpper(fields["FREQ"])
	var weekdays []time.Weekday
	switch freq {
	case "", "DAILY":
		// every day
	case "WEEKDAYS":
		weekdays = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
	case "WEEKLY":
		if byday, ok := fields["BYDAY"]; ok && byday != "" {
			weekdays, err = parseByDay(byday)
			if err != nil {
				return Recurrence{}, err
			}
		}
		// no BYDAY on a WEEKLY rule = every day, same as DAILY (documented above)
	default:
		return Recurrence{}, fmt.Errorf("alarms: unrecognized FREQ %q (want DAILY, WEEKLY, or WEEKDAYS)", fields["FREQ"])
	}

	return Recurrence{Hour: hour, Minute: minute, Weekdays: weekdays}, nil
}

func parseClock(s string) (hour, minute int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("alarms: invalid TIME %q, want HH:MM", s)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("alarms: invalid hour in TIME %q", s)
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("alarms: invalid minute in TIME %q", s)
	}
	return hour, minute, nil
}

var dayAbbrev = map[string]time.Weekday{
	"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday,
	"TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday,
}

func parseByDay(s string) ([]time.Weekday, error) {
	var out []time.Weekday
	for _, tok := range strings.Split(s, ",") {
		tok = strings.ToUpper(strings.TrimSpace(tok))
		day, ok := dayAbbrev[tok]
		if !ok {
			return nil, fmt.Errorf("alarms: unrecognized BYDAY value %q", tok)
		}
		out = append(out, day)
	}
	return out, nil
}

func (r Recurrence) allowsWeekday(d time.Weekday) bool {
	if len(r.Weekdays) == 0 {
		return true
	}
	for _, w := range r.Weekdays {
		if w == d {
			return true
		}
	}
	return false
}

// NextFireAfter returns the earliest instant strictly after `after` that
// matches this recurrence, in after's own location (a person's "9am
// alarm" means 9am in their local timezone, not UTC - section 70).
// Searches at most 8 days ahead, which always finds a match for any
// non-empty rule (even a single allowed weekday recurs within 7 days)
// the +1 day of headroom is defensive, not load-bearing.
func (r Recurrence) NextFireAfter(after time.Time) time.Time {
	loc := after.Location()
	for offset := 0; offset <= 8; offset++ {
		candidateDay := after.AddDate(0, 0, offset)
		candidate := time.Date(candidateDay.Year(), candidateDay.Month(), candidateDay.Day(), r.Hour, r.Minute, 0, 0, loc)
		if !candidate.After(after) {
			continue
		}
		if r.allowsWeekday(candidate.Weekday()) {
			return candidate
		}
	}
	// Unreachable for any rule produced by ParseRecurrenceRule (Weekdays
	// is always either empty or non-empty with at least one real day),
	// but return a far-future sentinel rather than a zero time if it
	// somehow is - a zero time.Time would look like "already due".
	return after.AddDate(1, 0, 0)
}

// sortedWeekdays is a small helper used only by tests, to compare
// []time.Weekday slices independent of the order ParseRecurrenceRule
// happened to produce them in.
func sortedWeekdays(days []time.Weekday) []time.Weekday {
	out := append([]time.Weekday(nil), days...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
