-- Phase 11/12's Pattern Discovery (spec section 31) uses the
-- pattern_insights table 0001 already created, but 0001 gave it no
-- natural key - insight_type alone was never made unique, so there was
-- no way to "upsert the latest wording of this same observation" or
-- "the person dismissed this kind of observation, don't show it again"
-- without either duplicating rows on every recompute or hand-rolling a
-- check-then-insert race.
--
-- insight_type is keyed as "<kind>:<categoryId>" by internal/patterns'
-- caller (app/services/pattern_service.go) - e.g. "typical_start:3" - -- so the SAME kind of observation about the SAME category always
-- refreshes one row instead of accumulating a new one every time
-- someone opens the Patterns page, and dismissing it can never be
-- silently undone by the next recompute inserting a fresh row instead
-- of hitting the dismissed one.
--
-- Additive only, per the up-only migration rule (section 46/85 #42):
-- existing pattern_insights rows (there are none yet - this table has
-- had no writer until this session) are unaffected.

CREATE UNIQUE INDEX IF NOT EXISTS idx_pattern_insights_type ON pattern_insights(insight_type);
