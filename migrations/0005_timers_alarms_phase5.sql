-- Phase 5 (Timers + alarms + notifications). Additive only, per the
-- up-only migration rule (section 46/85 #42) - 0001 already created
-- timers/alarms/notifications, this just adds the columns Phase 5's
-- actual pause/resume and snooze behavior needs that weren't anticipated
-- up front.
--
-- timers.status already distinguishes running/paused/completed/cancelled,
-- but there was nowhere to record WHEN a pause started or how much pause
-- time has accumulated across possibly-multiple pauses - both required to
-- compute "elapsed" correctly for a timer that's been paused and resumed
-- more than once (see internal/timers/elapsed.go). Existing rows get
-- accumulated_pause_secs = 0 (correct: no phase-5 code ever paused them)
-- and paused_at = NULL (correct: nothing pre-existing is mid-pause).
ALTER TABLE timers ADD COLUMN paused_at INTEGER;
ALTER TABLE timers ADD COLUMN accumulated_pause_secs INTEGER NOT NULL DEFAULT 0;

-- alarms.snoozed_until (section 22 "Allow snooze"). NULL = not snoozed.
-- Kept separate from fire_at (which stays the alarm's original/rule-driven
-- time) so snoozing never mutates what the alarm was actually configured
-- to do - it's purely a temporary "fire again at X instead" override the
-- scheduler checks first.
ALTER TABLE alarms ADD COLUMN snoozed_until INTEGER;
