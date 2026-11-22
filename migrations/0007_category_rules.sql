-- Category-level auto-rules (spec section 65). 0001's project_rules
-- table covered project assignment only; categories could only ever
-- come from an application's default_category_id or an explicit
-- per-application override (see internal/categories.Repository.
-- CategoryForApplication) - no pattern-matching rule engine, even
-- though section 65's own example ("IF domain = youtube.com THEN
-- category = Entertainment") is a category rule, not a project rule.
--
-- Structurally identical to project_rules - same columns, same
-- ON DELETE CASCADE semantics, same enabled/priority/created_at shape - -- because internal/categories.Rule mirrors internal/projects.Rule
-- exactly (see that package's rules.go doc comment for why it's a
-- parallel type rather than a shared import).
--
-- Additive only, per the up-only migration rule (section 46/85 #42):
-- no existing table or column is touched. A category with zero rules
-- behaves exactly as before this migration - falls back to
-- default_category_id / the explicit per-application override, same
-- as always.

CREATE TABLE IF NOT EXISTS category_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    category_id  INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    rule_type    TEXT NOT NULL,       -- process | window_title_contains | path_contains | domain
    pattern      TEXT NOT NULL,
    priority     INTEGER NOT NULL DEFAULT 100,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_category_rules_category ON category_rules(category_id);
