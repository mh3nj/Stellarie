// Package search implements global Search (spec sections 29 and 62):
// "When did I work on HeartLib?", "gaming last month", "longest coding
// session", "Serenity August 10", "What was I doing at 14:30 on
// August 3?".
//
// HONEST SCOPE STATEMENT - read this before assuming a query "should"
// work: this package recognizes a bounded, documented vocabulary of
// date/time phrases and matches PROJECT/CATEGORY/APPLICATION NAMES AS
// THE PERSON ACTUALLY CONFIGURED THEM - it does not understand English,
// and it does not know synonyms. Spec's own example "longest coding
// session" only resolves correctly if a category or project is
// literally named something containing "coding" - the builtin category
// is named "Development", so that exact spec example will NOT match
// unless the person has a project/category with "coding" in its name.
// This is a deliberate boundary, not an oversight: building a synonym
// dictionary ("coding" → "Development", "gaming" → "Entertainment" or
// whatever the person actually calls it) would mean guessing at a
// mapping with no way to get it right for every person's own naming
// exactly the kind of invented behavior spec section 83 warns against
// ("do not implement fake functionality"). What this package DOES
// reliably handle: exact-vocabulary date phrases (see Parse's doc
// comment for the full list), whole-word entity-name matching, and the
// "longest"/"shortest" superlatives - which together cover most of
// spec's own examples correctly.
package search

// EntityKind identifies which table a matched entity came from.
type EntityKind string

const (
	EntityProject     EntityKind = "project"
	EntityCategory    EntityKind = "category"
	EntityApplication EntityKind = "application"
)

// KnownEntity is one project/category/application the parser can match
// a query against - supplied by the caller (app/services/search_service.go),
// which already has these lists loaded for other purposes. This package
// never queries a database itself, matching every other package in
// this family (internal/idea, internal/patterns, internal/waste,
// internal/calendar).
type KnownEntity struct {
	Kind EntityKind
	ID   int64
	Name string
}

// ParsedQuery is the structured result of Parse. Superlative is
// "longest", "shortest", or "" (none requested). Entity is nil if no
// known project/category/application name was found in the query.
// HasDateRange and HasInstant are mutually exclusive - a query with
// both an explicit day and a specific clock time resolves to a single
// Instant (a moment to look up, spec's "What was I doing at 14:30 on
// August 3?"), while a query with only a day/period resolves to a
// Range. Unmatched is true only when NEITHER an entity NOR any
// date/time was recognized at all - the caller's signal to fall back
// to plain substring matching against session notes rather than a
// structured query.
type ParsedQuery struct {
	RawText     string
	Superlative string // "longest" | "shortest" | ""
	Entity      *KnownEntity

	HasDateRange bool
	RangeStart   int64 // unix seconds, inclusive
	RangeEnd     int64 // unix seconds, exclusive

	HasInstant  bool
	InstantUnix int64

	Unmatched bool
}
