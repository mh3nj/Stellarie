package search

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	timeOfDayRe   = regexp.MustCompile(`\b([01]?\d|2[0-3]):([0-5]\d)\b`)
	superlativeRe = regexp.MustCompile(`(?i)\b(longest|shortest)\b`)
	monthDayRe    = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2})(?:(?:st|nd|rd|th))?(?:,?\s+(\d{4}))?\b`)
)

var monthByName = func() map[string]time.Month {
	m := map[string]time.Month{}
	for i := time.January; i <= time.December; i++ {
		m[strings.ToLower(i.String())] = i
	}
	return m
}()

// relativePhrase is one recognized fixed date phrase and the window it
// resolves to, given "now". Checked in the order defined in
// relativePhrases below - longer, more specific phrases first, so
// "last month" is matched before a hypothetical looser "month" pattern
// (this package has no such looser pattern, but the ordering discipline
// is deliberate and worth keeping if one is ever added).
type relativePhrase struct {
	phrase string
	window func(now time.Time, loc *time.Location) (start, end time.Time)
}

var relativePhrases = []relativePhrase{
	{"yesterday", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		day := dayStart(now.AddDate(0, 0, -1), loc)
		return day, day.AddDate(0, 0, 1)
	}},
	{"today", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		day := dayStart(now, loc)
		return day, day.AddDate(0, 0, 1)
	}},
	{"last week", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisWeek := weekStart(now, loc)
		return thisWeek.AddDate(0, 0, -7), thisWeek
	}},
	{"this week", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisWeek := weekStart(now, loc)
		return thisWeek, thisWeek.AddDate(0, 0, 7)
	}},
	{"last month", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisMonth := monthStart(now, loc)
		return thisMonth.AddDate(0, -1, 0), thisMonth
	}},
	{"this month", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisMonth := monthStart(now, loc)
		return thisMonth, thisMonth.AddDate(0, 1, 0)
	}},
	{"last year", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisYear := yearStart(now, loc)
		return thisYear.AddDate(-1, 0, 0), thisYear
	}},
	{"this year", func(now time.Time, loc *time.Location) (time.Time, time.Time) {
		thisYear := yearStart(now, loc)
		return thisYear, thisYear.AddDate(1, 0, 0)
	}},
}

func dayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func weekStart(t time.Time, loc *time.Location) time.Time {
	day := dayStart(t, loc)
	// Monday-first week, matching Timeline's own convention
	// (frontend/src/pages/Timeline.tsx's "(firstOfMonth.getDay()+6)%7").
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func monthStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func yearStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, loc)
}

// Parse turns rawQuery into a ParsedQuery. now and loc anchor every
// relative phrase ("today", "last month", ...) to a real point in time
// and the person's own timezone (spec section 70).
//
// Recognized date/time vocabulary - exactly this list, nothing fuzzier:
// "today", "yesterday", "this week", "last week", "this month", "last
// month", "this year", "last year"; an explicit "<Month> <Day>[,
// <Year>]" (e.g. "August 10" or "August 10, 2026" - a year-less date
// resolves to the most recent past occurrence, so "August 10" found in
// February means last August, not next August); and a 24-hour "HH:MM"
// clock time, which combines with an explicit or relative single-day
// date (not a multi-day range) into a specific Instant rather than a
// Range.
func Parse(rawQuery string, entities []KnownEntity, now time.Time, loc *time.Location) ParsedQuery {
	q := ParsedQuery{RawText: rawQuery}

	if m := superlativeRe.FindString(rawQuery); m != "" {
		q.Superlative = strings.ToLower(m)
	}

	var dayResolved bool
	var dayStartT, dayEndT time.Time
	var rangeResolved bool

	if m := monthDayRe.FindStringSubmatch(rawQuery); m != nil {
		month := monthByName[strings.ToLower(m[1])]
		dayNum, _ := strconv.Atoi(m[2])
		year := now.In(loc).Year()
		if m[3] != "" {
			year, _ = strconv.Atoi(m[3])
		}
		candidate := time.Date(year, month, dayNum, 0, 0, 0, 0, loc)
		if m[3] == "" && candidate.After(now.In(loc)) {
			// No year given and the date hasn't happened yet this year
			// - assume the most recent past occurrence (see doc
			// comment above).
			candidate = time.Date(year-1, month, dayNum, 0, 0, 0, 0, loc)
		}
		dayStartT = candidate
		dayEndT = candidate.AddDate(0, 0, 1)
		dayResolved = true
		rangeResolved = true
	} else {
		lower := strings.ToLower(rawQuery)
		for _, rp := range relativePhrases {
			if strings.Contains(lower, rp.phrase) {
				dayStartT, dayEndT = rp.window(now, loc)
				dayResolved = rp.phrase == "today" || rp.phrase == "yesterday"
				rangeResolved = true
				break
			}
		}
	}

	if timeMatch := timeOfDayRe.FindStringSubmatch(rawQuery); timeMatch != nil && dayResolved {
		hour, _ := strconv.Atoi(timeMatch[1])
		minute, _ := strconv.Atoi(timeMatch[2])
		q.HasInstant = true
		q.InstantUnix = time.Date(dayStartT.Year(), dayStartT.Month(), dayStartT.Day(), hour, minute, 0, 0, loc).Unix()
	} else if rangeResolved {
		q.HasDateRange = true
		q.RangeStart = dayStartT.Unix()
		q.RangeEnd = dayEndT.Unix()
	}

	q.Entity = matchEntity(rawQuery, entities)

	q.Unmatched = q.Entity == nil && !q.HasDateRange && !q.HasInstant
	return q
}

// matchEntity finds the KnownEntity whose Name appears as a whole word
// in text, preferring the longest matching name if several do (so a
// short, coincidentally-substring-matching name doesn't win over a more
// specific longer one). Matching is case-insensitive; entity names are
// regexp-escaped before building the boundary pattern so a real project
// named e.g. "C++" or "Node.js" can't corrupt the regex.
func matchEntity(text string, entities []KnownEntity) *KnownEntity {
	type candidate struct {
		entity KnownEntity
	}
	var matches []candidate
	for _, e := range entities {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			continue
		}
		pattern := `(?i)\b` + regexp.QuoteMeta(name) + `\b`
		if regexp.MustCompile(pattern).MatchString(text) {
			matches = append(matches, candidate{entity: e})
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return len(matches[i].entity.Name) > len(matches[j].entity.Name)
	})
	return &matches[0].entity
}
