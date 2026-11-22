-- STELLAREI initial schema
--
-- Design notes:
--   * WAL mode + synchronous=NORMAL are set at connection time (see
--     internal/database/database.go), not here - PRAGMAs are per-connection.
--   * Every high-cardinality table (activity_events, sessions) is indexed on
--     its timestamp range plus the foreign keys it's most often filtered by,
--     since those are the columns Timeline/Analytics/Search will hit.
--   * Nothing here is destructive. Future migrations must only ADD - see
--     internal/database/migrate.go for the up-only migration runner.
--   * Money-free, telemetry-free by design: no columns exist for anything
--     this app must never collect (keystrokes, clipboard, credentials).

PRAGMA foreign_keys = ON;

-- ─────────────────────────────────────────────────────────────────────────
-- Settings: a single flexible key/value store. Typed accessors live in
-- app/services/settings_service.go; this table just stores strings.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    value_type  TEXT NOT NULL DEFAULT 'string', -- string | bool | int | json
    updated_at  INTEGER NOT NULL
);

-- ─────────────────────────────────────────────────────────────────────────
-- Categories: Development, Design, Music, Gaming, Research, Communication,
-- Entertainment, Idle, Unknown, etc. User-editable, not a hardcoded enum.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    color_token TEXT,                 -- references a theme token, e.g. "timelineDevelopment"
    icon        TEXT,
    is_builtin  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

-- ─────────────────────────────────────────────────────────────────────────
-- Projects
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS projects (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    description  TEXT,
    icon         TEXT,
    color_token  TEXT,
    category_id  INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    status       TEXT NOT NULL DEFAULT 'active', -- active | paused | completed | abandoned
    goal_hours   REAL,
    target_hours REAL,
    deadline     INTEGER,             -- unix seconds, nullable
    created_at   INTEGER NOT NULL,
    archived_at  INTEGER
);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);

-- Rules that auto-associate activity with a project (section 11/65).
-- Evaluated in ascending `priority` order; first match wins.
CREATE TABLE IF NOT EXISTS project_rules (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    rule_type    TEXT NOT NULL,       -- process | window_title_contains | path_contains | domain
    pattern      TEXT NOT NULL,
    priority     INTEGER NOT NULL DEFAULT 100,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_project_rules_project ON project_rules(project_id);

-- ─────────────────────────────────────────────────────────────────────────
-- Application registry (section 10)
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS applications (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    process_name          TEXT NOT NULL UNIQUE, -- e.g. "Code.exe"
    display_name          TEXT,
    icon_path             TEXT,
    default_category_id   INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    category_id           INTEGER REFERENCES categories(id) ON DELETE SET NULL, -- user override
    tracking_enabled       INTEGER NOT NULL DEFAULT 1,
    screenshot_allowed     INTEGER NOT NULL DEFAULT 1,
    browser_url_tracking   INTEGER NOT NULL DEFAULT 0,
    privacy_sensitive      INTEGER NOT NULL DEFAULT 0, -- e.g. password managers, banking apps
    notes                 TEXT,
    first_seen_at         INTEGER NOT NULL,
    last_seen_at          INTEGER NOT NULL
);

-- ─────────────────────────────────────────────────────────────────────────
-- Activity events (raw, high-frequency - see section 7). Debounced/
-- aggregated at write time by the Activity Engine (Phase 2); this table is
-- NOT written to once per poll tick.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS activity_events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at     INTEGER NOT NULL,   -- unix seconds
    duration_secs  INTEGER NOT NULL,
    application_id INTEGER REFERENCES applications(id) ON DELETE SET NULL,
    window_title   TEXT,
    idle           INTEGER NOT NULL DEFAULT 0,
    category_id    INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    project_id     INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    confidence     REAL NOT NULL DEFAULT 1.0,
    source         TEXT NOT NULL DEFAULT 'activity_engine',
    metadata_json  TEXT
);
CREATE INDEX IF NOT EXISTS idx_activity_events_started_at ON activity_events(started_at);
CREATE INDEX IF NOT EXISTS idx_activity_events_project ON activity_events(project_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_application ON activity_events(application_id);

-- ─────────────────────────────────────────────────────────────────────────
-- Sessions (sessionized, user-facing - section 8)
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS sessions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at       INTEGER NOT NULL,
    ended_at         INTEGER NOT NULL,
    project_id       INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    category_id      INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    confidence       REAL NOT NULL DEFAULT 1.0,
    manually_edited  INTEGER NOT NULL DEFAULT 0, -- corrections take precedence (section 63)
    excluded         INTEGER NOT NULL DEFAULT 0,
    note             TEXT,
    created_at       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id);

-- Join table: which raw events rolled up into which session.
CREATE TABLE IF NOT EXISTS session_events (
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    event_id   INTEGER NOT NULL REFERENCES activity_events(id) ON DELETE CASCADE,
    PRIMARY KEY (session_id, event_id)
);

-- ─────────────────────────────────────────────────────────────────────────
-- Goals & budgets (sections 17/18)
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS goals (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    goal_type    TEXT NOT NULL,       -- time | project | activity | custom
    project_id   INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    target_value REAL NOT NULL,       -- hours, or count for activity-based goals
    unit         TEXT NOT NULL DEFAULT 'hours',
    period       TEXT NOT NULL DEFAULT 'month', -- day | week | month | custom
    period_start INTEGER,
    period_end   INTEGER,
    created_at   INTEGER NOT NULL,
    archived_at  INTEGER
);

CREATE TABLE IF NOT EXISTS budgets (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    category_id  INTEGER REFERENCES categories(id) ON DELETE CASCADE,
    project_id   INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    budget_type  TEXT NOT NULL,       -- maximum | target
    limit_minutes INTEGER NOT NULL,
    schedule     TEXT NOT NULL DEFAULT 'daily', -- daily | weekly | monthly | custom
    warn_at_minutes INTEGER,
    hard_limit   INTEGER NOT NULL DEFAULT 0, -- soft block / app blocking, opt-in (section 19)
    created_at   INTEGER NOT NULL,
    archived_at  INTEGER
);

-- ─────────────────────────────────────────────────────────────────────────
-- Timers & alarms (sections 21/22)
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS timers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    timer_type    TEXT NOT NULL,      -- stopwatch | countdown | pomodoro | interval | focus | project | custom
    project_id    INTEGER REFERENCES projects(id) ON DELETE SET NULL,
    label         TEXT,
    duration_secs INTEGER,            -- null for stopwatch
    started_at    INTEGER NOT NULL,
    ended_at      INTEGER,
    status        TEXT NOT NULL DEFAULT 'running', -- running | paused | completed | cancelled
    metadata_json TEXT
);

CREATE TABLE IF NOT EXISTS alarms (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    alarm_type     TEXT NOT NULL,     -- one_time | recurring | relative | activity_based | schedule
    label          TEXT,
    fire_at        INTEGER,           -- unix seconds, for one_time/relative
    recurrence_rule TEXT,             -- e.g. simple RRULE-like string for recurring
    trigger_condition TEXT,           -- for activity_based, e.g. "category=Gaming;minutes=60"
    enabled        INTEGER NOT NULL DEFAULT 1,
    snooze_minutes INTEGER NOT NULL DEFAULT 5,
    sound          TEXT,
    created_at     INTEGER NOT NULL,
    last_fired_at  INTEGER
);

CREATE TABLE IF NOT EXISTS notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source      TEXT NOT NULL,        -- alarm | budget | goal | system
    source_id   INTEGER,
    title       TEXT NOT NULL,
    body        TEXT,
    created_at  INTEGER NOT NULL,
    dismissed_at INTEGER
);

-- ─────────────────────────────────────────────────────────────────────────
-- Themes (sections 38-41)
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS themes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    is_builtin  INTEGER NOT NULL DEFAULT 0,
    tokens_json TEXT NOT NULL,        -- validated against the theme token schema before insert
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS theme_metadata (
    theme_id    INTEGER PRIMARY KEY REFERENCES themes(id) ON DELETE CASCADE,
    author      TEXT,
    version     INTEGER NOT NULL DEFAULT 1,
    is_default  INTEGER NOT NULL DEFAULT 0
);

-- ─────────────────────────────────────────────────────────────────────────
-- Screenshot memory (section 27) - table exists even though the feature
-- ships OFF by default; rows only appear if the user opts in.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS screenshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    taken_at    INTEGER NOT NULL,
    file_path   TEXT NOT NULL,
    session_id  INTEGER REFERENCES sessions(id) ON DELETE SET NULL,
    application_id INTEGER REFERENCES applications(id) ON DELETE SET NULL,
    delete_after INTEGER              -- unix seconds; a background sweep purges past this
);
CREATE INDEX IF NOT EXISTS idx_screenshots_taken_at ON screenshots(taken_at);

CREATE TABLE IF NOT EXISTS privacy_exclusions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    exclusion_type TEXT NOT NULL,     -- application | window_title_pattern | domain
    pattern        TEXT NOT NULL,
    applies_to     TEXT NOT NULL DEFAULT 'all', -- all | screenshots | browser_tracking
    created_at     INTEGER NOT NULL
);

-- ─────────────────────────────────────────────────────────────────────────
-- Rollups: computed once, kept "forever" even if raw events are purged
-- under retention settings (section 47).
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS daily_summaries (
    day_key      TEXT PRIMARY KEY,    -- "2026-08-16" in the user's local timezone
    totals_json  TEXT NOT NULL,       -- {"Development": 16260, "Gaming": 6720, ...} in seconds
    tracked_secs INTEGER NOT NULL,
    idle_secs    INTEGER NOT NULL,
    generated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS weekly_summaries (
    week_key     TEXT PRIMARY KEY,    -- ISO week, e.g. "2026-W33"
    totals_json  TEXT NOT NULL,
    tracked_secs INTEGER NOT NULL,
    idle_secs    INTEGER NOT NULL,
    generated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pattern_insights (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    insight_type TEXT NOT NULL,
    body         TEXT NOT NULL,       -- pre-rendered observation text, correlation-only phrasing
    confidence   REAL,
    evidence_json TEXT,
    generated_at INTEGER NOT NULL,
    dismissed    INTEGER NOT NULL DEFAULT 0
);

-- ─────────────────────────────────────────────────────────────────────────
-- "I HAVE NO IDEA" / Chaos Mode history (sections 34-36) - prevents
-- repeated suggestions.
-- ─────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS idea_history (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    generated_at   INTEGER NOT NULL,
    available_minutes INTEGER,
    intent         TEXT,              -- productive | creative | fun | learn | surprise
    suggestions_json TEXT NOT NULL,
    accepted_index INTEGER            -- which of the 3 suggestions (if any) was accepted
);

CREATE TABLE IF NOT EXISTS chaos_history (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    generated_at INTEGER NOT NULL,
    challenge    TEXT NOT NULL,
    accepted     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS notes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_type TEXT NOT NULL,       -- session | project | day
    subject_id  TEXT NOT NULL,        -- session id, project id, or day_key
    body        TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notes_subject ON notes(subject_type, subject_id);
