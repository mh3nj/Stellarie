-- Phase 2 (Activity Engine) needs to record which application a session
-- belongs to. Every event the sessionizer merges into one session is
-- guaranteed to share the same application_id (see
-- internal/sessions/sessionizer.go's merge rule), so storing it directly
-- on the row - rather than requiring a join through session_events on
-- every read - is the correct normalized shape, not a denormalization
-- shortcut.
--
-- Additive only, per the up-only migration rule (section 46/85): existing
-- rows simply get a NULL application_id, which is exactly correct since
-- Phase 1 never had sessions in the first place.

ALTER TABLE sessions ADD COLUMN application_id INTEGER REFERENCES applications(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_application ON sessions(application_id);
