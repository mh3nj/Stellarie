-- Seeds the built-in themes so the themes table (created in 0001) is never
-- empty and Theme Studio always has at least these two to list, duplicate,
-- and switch between.
--
-- Token values here MUST stay in sync by hand with:
--   - frontend/src/themes/tokens.css        (Midnight's :root defaults)
--   - frontend/src/themes/themes.ts         (MIDNIGHT_THEME, LIGHT_THEME)
-- There's no build step that generates one from the other - if you change
-- a token in one place, change it in all three, or the app's initial
-- unstyled-flash defaults, the seeded DB row, and the Theme Studio's
-- "built-in" list will quietly drift apart.
--
-- INSERT OR IGNORE keyed on the UNIQUE(name) constraint makes this safe to
-- run against a database that already has these rows (e.g. a future
-- migration re-running this file's logic, or a user who renamed nothing) - -- it never overwrites an existing row, so a user's own edits to a
-- same-named custom theme are never at risk (and built-ins can't be
-- renamed via ThemeService in the first place - see app/services/theme_service.go).

INSERT OR IGNORE INTO themes (name, is_builtin, tokens_json, created_at, updated_at)
VALUES (
	'Midnight',
	1,
	'{"background":"#0b0d10","backgroundElevated":"#101317","surface":"#15181d","surfaceHover":"#1b1f25","surfaceActive":"#21262d","border":"#262b33","text":"#e8eaed","textMuted":"#9aa1ac","accent":"#6ea8fe","accentHover":"#8ebbff","success":"#4caf82","warning":"#d9a441","danger":"#e0616b","info":"#6ea8fe","chartPrimary":"#6ea8fe","chartSecondary":"#9b7ede","timelineDevelopment":"#6ea8fe","timelineDesign":"#d98ee0","timelineMusic":"#f2b84b","timelineGaming":"#e0616b","timelineResearch":"#4caf82","timelineIdle":"#3a4048"}',
	unixepoch(),
	unixepoch()
);

INSERT OR IGNORE INTO themes (name, is_builtin, tokens_json, created_at, updated_at)
VALUES (
	'Light',
	1,
	'{"background":"#f5f4fb","backgroundElevated":"#ffffff","surface":"#ffffff","surfaceHover":"#ececf7","surfaceActive":"#e3e1f5","border":"#ddd9f0","text":"#1a1b2e","textMuted":"#6b6d85","accent":"#5b4fe5","accentHover":"#4a3fd1","success":"#2f9e6a","warning":"#b8791f","danger":"#c94454","info":"#5b4fe5","chartPrimary":"#5b4fe5","chartSecondary":"#8b5fd6","timelineDevelopment":"#5b4fe5","timelineDesign":"#c15fc9","timelineMusic":"#d99a2b","timelineGaming":"#c94454","timelineResearch":"#2f9e6a","timelineIdle":"#d5d3e8"}',
	unixepoch(),
	unixepoch()
);

INSERT OR IGNORE INTO theme_metadata (theme_id, author, version, is_default)
SELECT id, 'STELLAREI', 1, CASE WHEN name = 'Midnight' THEN 1 ELSE 0 END
FROM themes
WHERE name IN ('Midnight', 'Light');
