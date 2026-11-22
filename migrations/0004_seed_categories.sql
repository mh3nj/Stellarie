-- Default categories, matching the examples used throughout the spec
-- (sections 6, 14: Development, Design, Music, Gaming, Research,
-- Communication, Entertainment, Idle, Unknown). Additive and idempotent
-- via INSERT OR IGNORE keyed on categories.name's UNIQUE constraint - -- safe to run against a database that already has these rows, and never
-- touches a category the user has renamed or recolored.
--
-- color_token values reference frontend/src/themes/themes.ts's token
-- keys. The six timeline* tokens exist specifically for this; the
-- remaining three categories reuse general-purpose tokens (info,
-- chartSecondary, textMuted) since the spec's token list doesn't define
-- dedicated timelineCommunication/timelineEntertainment/timelineUnknown
-- tokens - kept consistent with tokens.css/themes.ts rather than
-- inventing new token names here that those files don't define.

INSERT OR IGNORE INTO categories (name, color_token, is_builtin, created_at) VALUES
	('Development',   'timelineDevelopment', 1, unixepoch()),
	('Design',        'timelineDesign',      1, unixepoch()),
	('Music',         'timelineMusic',       1, unixepoch()),
	('Gaming',        'timelineGaming',      1, unixepoch()),
	('Research',      'timelineResearch',    1, unixepoch()),
	('Communication', 'info',                1, unixepoch()),
	('Entertainment', 'chartSecondary',      1, unixepoch()),
	('Idle',          'timelineIdle',        1, unixepoch()),
	('Unknown',       'textMuted',           1, unixepoch());
