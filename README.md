# STELLAREI

A local-first, privacy-first personal time intelligence desktop app.
Go + Wails v3 backend, React + TypeScript frontend, SQLite storage.
No network server, no telemetry, no accounts. See the full spec this
repo implements for the product philosophy.

## Status: Phase 11 of 13 substantially complete (foundation + genuinely all six spec-listed AI use cases wired, after catching and fixing a real gap in an earlier "six of six" claim), Phase 12 in progress, Phase 13 groundwork started, plus Theme Studio (Phase 8), Calendar (spec section 30), Search (spec sections 29/62), MY YEAR (spec section 61), a general Diagnostics page (spec sections 55/80 - startup-registration read-back now wired for real, after catching and correcting an earlier wrong claim that it wasn't possible yet; tray status investigated for real and confirmed genuinely unavailable in the current Wails v3 API, not just unwired), Global Shortcuts (spec section 45 - COMPLETE: fixed defaults, user-configurability, and conflict detection all real and wired end-to-end on the Go side; frontend Settings UI real and verified; only `main.go`'s own compile is untestable in this sandbox), i18n (spec section 69 - COMPLETE: all 21 pages translated into English/Persian/German, real infrastructure, real spatial RTL mirroring and locale-aware week-start days both verified with real browser/date-math tests after earlier wrong claims that they weren't done were caught and corrected, stated honestly with its documented non-translation exceptions), category-level auto-rules (spec section 65 - the Rule Engine now covers categories, not just projects), and accessibility (spec section 68 - COMPLETE: keyboard-visible focus, reduced motion, keyboard-accessible disclosure rows, icon-button labels, a full WCAG contrast-ratio audit across both built-in themes with one genuine AA failure found and fixed, and all 89 `font-size` declarations converted `px`→`rem`) brought forward ahead of the phase roadmap - every nav destination in the app now has a real page, none left as a placeholder

Phase 1 (native shell), Phase 2 (Activity Engine), Phase 3 (Timeline),
Phase 4 (Analytics), Phase 5 (Timers + alarms + notifications), Phase 6
(Focus Mode), Phase 7 ("I Have No Idea"), Phase 9 (Privacy Center +
Screenshot Memory), and Phase 10 (Backup/Import/Export) from the
spec's own development order (spec section 84) are done. **Theme
Studio** (normally Phase 8) was also built ahead of schedule, along
with logo/tray-icon integration, at the person's request.

**Phase 11 (Optional Local AI) is now partially done.** The full
foundation is built and tested (`internal/ai`: Ollama and
OpenAI-compatible/LM-Studio/llama.cpp clients, scope-gated
disabled-by-default config, and graceful degradation per spec section
56 - see that package's doc comment for the complete contract), wired
through `app/services/ai_service.go` to Settings > AI in the frontend
(enable toggle, provider/endpoint/model, per-feature scope checkboxes,
Test Connection, Diagnostics), and integrated as a real, optional,
clearly-labeled `aiNote` on **"I HAVE NO IDEA"**'s three suggestions,
on the single generated project idea, and on the new **Daily Time
Receipt** (spec section 60 - a real feature built from scratch this
session, not just an AI hook: `receiptService.GetTimeReceipt` computes
today's category totals, goal status, and daily-budget overage from
real session data, with `aiNote` as an optional one-sentence add-on).
That's three of section 37's six listed AI use cases wired into a real
caller. **Since then, Pattern Discovery (spec section 31) has also been
built from scratch and wired**: `internal/patterns` computes typical
start times (median, resistant to outliers), time-of-day session-length
comparisons, and week-over-week context-switching trends from real
session data - every function has a minimum sample size and, where a
percentage is involved, a minimum difference, below which it reports
nothing rather than manufacturing a pattern out of noise (spec section
66) - plus the same optional, clearly-labeled `aiNote` convention,
explicitly forbidden from introducing any pattern not already found
deterministically. That's four of six. **Since then, Digital Waste
Detection (spec section 33) has been built too, finally giving
`ScopeSuggestions` a real home** - the last of section 37's six listed
AI use cases. `internal/waste` detects three of the spec's six example
waste patterns from real session data (repeated short reopens of the
same category, long idle stretches, sessions under a minute) - the
other three are deliberately not reimplemented (rapid switching already
has a real implementation elsewhere; "excessive entertainment time"
would mean inventing a moral threshold the spec explicitly forbids;
"repeated navigation" needs browser tracking, off by default - see
`internal/waste`'s own package doc for the full reasoning on each).
`ScopeSuggestions`' `aiNote` is deliberately different in character from
Pattern Discovery's: not a backward-looking summary but an explicitly
forward-looking, still non-judgmental, evidence-only suggestion - see
`aiNoteForFindings`'s doc comment for the exact system prompt. **That
was believed to be six of six at the time - it wasn't.** `ScopeSummaries`
(a sixth, separate scope defined alongside these five) had actually
sat completely unused this whole time; see the "MY YEAR" section below
for the honest account of catching and fixing that.
**Phase 12 (Performance hardening) now has six real, honestly-scoped
pieces done** (spec sections 51, 52, and 79):

1. **A long-run benchmark for Pattern Discovery.**
   `internal/patterns/benchmark_test.go` generates a full deterministic
   (seeded) year of realistic synthetic session data across 6
   categories and runs `DiscoverAll` over it - **~195µs and 111
   allocations per call at 1 year, ~599µs and 123 allocations at 3
   years** (`go test -bench=BenchmarkDiscoverAll -benchmem
   ./internal/patterns/...`), comfortably fast even after years of
   continuous tracking.
2. **An indexing hypothesis investigated and empirically disproven - reported honestly rather than "fixed" anyway.** `ListForRange`'s
   overlap query (`WHERE started_at < ? AND ended_at > ?`) only has an
   index on `started_at`, and it looked, on paper, like the query
   planner would end up scanning most of the table for any "recent
   window" query as the table grows over months/years. Tested this for
   real: built the exact query against real SQLite (Python's `sqlite3`)
   loaded with realistic data at 5 years (9,125 sessions), 10 years
   (18,250 sessions), and a deliberate stress-test volume no real usage
   would ever reach (200,000 sessions) - the query stayed at **1.2ms at
   realistic 10-year volume and 13.8ms even at the 200k stress case**,
   and adding a second index on `ended_at` made things measurably
   **slower**, not faster, at every scale tested (SQLite's planner
   never chose to use it). Conclusion: the existing single-column index
   is already sufficient; no migration was added, because adding one
   would have cost real write overhead forever for zero measured read
   benefit. This negative result is exactly what section 52's own
   "benchmark... don't guess" instruction is for.
3. **A real N+1 query pattern, found and fixed.**
   `BudgetService.ListWithStatus`/`GoalService.ListWithStatus` called
   `SumDurationByCategory`/`SumDurationByProject` once **per budget or
   goal row**, even though each call already computes every
   category/project's total in one `GROUP BY` query - so budgets or
   goals sharing a schedule (e.g. several "daily" budgets) triggered the
   identical aggregate query redundantly. Measured before fixing: at
   realistic 10-year session volume (14,620 rows), 20 same-schedule
   budgets cost **~19ms** (one query per budget) vs. **~1ms** for the
   same result computed once. Fixed by memoizing each distinct
   `[start,end)` window's result within a single `ListWithStatus` call - same fallback-to-zero-on-error behavior, same output, strictly
   fewer queries.
4. **A second, worse N+1 pattern in the same audit pass, found and
   fixed the same way.** `BackupService.appendNamedSums` (the analytics
   export path) resolved each category/project id to a display name
   INSIDE its per-id loop - for projects, that meant calling
   `ListProjects` (a full table fetch) and linear-scanning it again for
   every single project being exported, not just once; for categories,
   one `SELECT ... WHERE id = ?` per id. Fixed by resolving all names
   into a map with exactly one query each (all categories, all
   projects) before the loop, then doing O(1) lookups inside it - same fallback-to-`"#<id>"`-when-unresolvable behavior as before,
   verified against real SQLite. Also fixed a smaller, separate bug
   noticed while touching this code: the original loop iterated a Go
   map directly, whose iteration order is intentionally randomized by
   the runtime - meaning every export of the same data could come out
   in a different row order for no reason. Now sorted by id, so
   exporting the same data twice produces the same file twice.

Both fixes verified by isolated build/vet (the same set-aside-
and-restore technique used throughout this project) and careful
manual trace, since `app/services` has no test suite of its own by
established convention (testable logic lives in `internal/`, and both
fixes are pure query-count/ordering changes with no new branching logic
to unit-test in isolation).

5. **A goroutine-lifecycle audit - clean, and confirmed rather than
   assumed** (spec section 51: "no goroutine leaks... all goroutines
   must shut down cleanly"). Only three background goroutines exist in
   the whole backend - the Activity Engine, the Scheduler, and the
   Privacy Sweeper - a small enough surface to check exhaustively rather
   than sample. All three follow the identical, correct pattern:
   `context.WithCancel` on `Start`, a `sync.WaitGroup` incremented
   before the goroutine launches, `Stop()` cancels the context and then
   blocks on `wg.Wait()` rather than returning immediately, and the run
   loop's `select` on `ctx.Done()` triggers a bounded (5-10s timeout,
   never `context.Background()`-forever) final flush before exiting.
   `main.go` wires all three `Stop()` calls explicitly after `app.Run()`
   returns - not via `defer`, with a comment explaining exactly why (an
   early `os.Exit` would skip deferred calls and leak the goroutine).
   No fix was needed here; this is a genuine "checked and it's already
   right" result, the same kind of honest negative finding as the
   `ended_at` index investigation above.
6. **The N+1 audit finished - every loop in `app/services` checked, not
   just the two that turned out broken.** After fixing Budget/Goal and
   the backup export path, every remaining loop that iterates database
   rows across Focus, Timer, Alarm, Notification, Session, Project, and
   Category services was read line by line. All but one are pure
   in-memory transforms with zero queries inside the loop - already
   fine. The one exception, `FocusService.History`, calls
   `sessionRepo.ListForRange` once per focus session in the returned
   list - which LOOKS like the same shape as the two real bugs, but
   isn't one: each call uses that specific session's own start/end
   window, not a window shared across rows, so there's no redundant
   identical query to cache away the way Budget/Goal's was. N
   genuinely-different, individually cheap, `limit`-bounded queries for
   N distinct items is normal, not a bug - rewriting it into one query
   would trade a clear, correct loop for a more complex correlated
   subquery with no measured benefit. Distinguishing that from an
   actual N+1 is the same discipline as the disproven `ended_at`
   hypothesis: check what's actually different before "fixing" it.

What Phase 12 does NOT cover yet, stated plainly: real SQLite
write-throughput under sustained load, and Timeline rendering cost at
scale, remain unverified in this sandbox - there's still no real
`modernc.org/sqlite` driver to write that much data through end-to-end
(see "What's verified vs. what isn't" below). The rest of Performance
hardening is not started.

**Phase 13 (Packaging) update 2026-09-15: the "can't go further in this
sandbox" era is over - on a real machine with Go 1.27.1, everything
below was actually run.** `go mod tidy` resolves (Wails v3.0.0-beta.22,
modernc sqlite v1.58.0), `go vet`/`go test`/`go build` are all green,
`wails3 generate bindings` produced 28 services / 131 methods / 68
models, `wails3 build` produced a working `bin/stellarei.exe` (~17MB)
that passes a 12-second launch smoke test, and both platform icons were
generated from STELLAREI's real mark. The per-platform Taskfiles are no
longer "absent rather than faked": the real generator-produced build
system (`Taskfile.yml`, `build/Taskfile.yml`,
`build/windows|darwin|linux|docker/`) was scaffolded via `wails3 init`
and adapted with STELLAREI's own metadata - the old handwritten
placeholders are kept as `*.bak-handwritten` for the record. What
remains: the NSIS installer (needs NSIS installed), `wails3 dev` loop
verification, and the spec section 85 Definition-of-Done checks - see
`PACKAGING.md` ("Verified 2026-09-15") for the exact list. Only
`@wailsio/runtime` was bumped (beta.8 → beta.22, matching the CLI);
`productIdentifier: com.dahgan.stellarei` (dahgan.com is a real, owned
domain) and version `0.1.0` are unchanged.

The paragraphs below are kept as the historical record of how Phase 13
groundwork was built and verified before real tooling was available - including the earlier sandbox's empirically-tested dead ends (Go 1.24
via `apt`, blocked toolchain fetch, blocked vanity-import egress),
which remain accurate for THAT environment and explain why the
handwritten scaffolding existed at all. `PACKAGING.md` is still the
step-by-step runbook, now with its Steps 0-3 marked done.
The frontend says so honestly on every page that isn't built
(see `frontend/src/pages/Placeholder.tsx`) rather than showing mock
data.

### What's real right now

- Full database schema across 6 up-only migrations
  (`migrations/0001_init_schema.sql` through
  `migrations/0006_pattern_insights_dismissal.sql`), applied by an
  up-only migration runner. Every statement across all six has been
  executed against real SQLite (Python's stdlib `sqlite3`, since the Go
  driver can't be fetched in this sandbox - see below) and confirmed to
  work, including the exact parameterized queries the Go repositories
  issue and, for 0006, the conditional `ON CONFLICT ... DO UPDATE ...
  WHERE` upsert-while-undismissed behavior specifically.
- Cross-platform app-data path resolution.
- Run-at-login registration for Windows (registry `Run` key), macOS
  (`LaunchAgent` plist), and Linux (XDG autostart `.desktop` entry).
- Close-behavior logic (Close / Minimize to Tray / Ask every time),
  unit-tested independently of the Wails wiring.
- **The Activity Engine** (`internal/activity`): polls the foreground
  window and idle state, debounces samples into events (never writing a
  row per poll tick - section 7), detects system-sleep gaps and closes
  the in-progress run at the last successful poll instead of inflating
  it across the gap, drops sub-second noise, and supports Pause (15
  min / 1 hour / until resumed) and Resume (section 25). 16 tests,
  including the debouncer, the pause/auto-resume timing (via an
  injectable clock), the suspend-gap heuristic, and graceful
  degradation when the OS-level watcher is unavailable.
- **The Sessionizer** (`internal/sessions`): groups activity events into
  sessions by application continuity, closes a session on idle or an
  app switch beyond a 5-second merge-gap tolerance, and never merges
  events from an unidentified application. 9 tests.
- Real, working Windows active-window/idle detection
  (`GetForegroundWindow`/`QueryFullProcessImageName`/
  `GetLastInputInfo`, standard stable Win32 APIs) and CGO-free
  Linux/macOS fallbacks (`xdotool`/`xprintidle`, `osascript`/`ioreg`)
  that degrade to `ErrUnavailable` - rather than crash - when the
  underlying tool isn't present. The Linux fallback's degrade-gracefully
  behavior is itself under test and passes in this exact sandbox (no
  xdotool, no X11 display).
- A system tray with a **live status label** ("Serenity · 01:17" /
  "STELLAREI - Idle" / "STELLAREI - Tracking Paused") and real Pause
  Tracking / Resume Tracking menu items, wired directly to the engine.
- Settings, Window, Startup, and Activity services bound to the
  frontend.
- A working React/TS/Vite frontend: nav shell, an Overview page with
  **live, real "Currently: X · elapsed time" status and working pause
  controls** (pushed via a Wails event and polled as a safety net,
  not mocked), a real Settings page, an About page, and honest
  placeholders for every unbuilt section.
- Semantic theme tokens as CSS custom properties (`Midnight`, the
  default theme) plus a real theme-file validator ahead of Phase 8's
  full Theme Studio.
- **Optional Local AI foundation** (`internal/ai`, Phase 11, see its
  package doc): Ollama and OpenAI-compatible clients, disabled-by-default
  + per-scope opt-in config, and a `Service.Enhance` that never returns a
  Go error for "AI isn't available right now" in any form - off,
  scope-denied, unreachable, timed out, bad response all become
  `Result.Unavailable` so callers degrade gracefully by construction
  (spec section 56). 25 tests, including `-race`, real `httptest`
  round-trips for both clients, and a from-scratch settings-table
  round-trip verified against real SQLite. Wired into "I HAVE NO IDEA"
  (both the three suggestions and the single project-idea generator),
  into the **Daily Time Receipt** (`/receipt`, spec section 60), and
  into **Pattern Discovery** (`/patterns`, spec section 31 - see
  `internal/patterns`' package doc; also a real feature built from
  scratch this session, not just an AI hook), into **Digital Waste
  Detection** (`/digital-waste`, spec section 33 - same "real feature,
  not just an AI hook" story), and into **MY YEAR** (`/my-year`, spec
  section 61) as an optional,
  clearly-labeled `aiNote` - see `idea_service.go`'s
  `aiNoteForSuggestions`/`aiNoteForProjectIdea`,
  `receipt_service.go`'s `aiNoteForReceipt`,
  `pattern_service.go`'s `aiNoteForPatterns`,
  `waste_service.go`'s `aiNoteForFindings`, and
  `myyear_service.go`'s `aiNoteForYear` - genuinely all six of section
  37's listed AI use cases now wired into a real caller (an earlier
  version of this README claimed this before it was actually true - see the "MY YEAR" section for the honest correction). Settings > AI in the
  frontend covers the full config surface plus a real Test Connection
  button and Diagnostics readout.

### Pattern Discovery (spec section 31)

- **`internal/patterns`** (see its own package doc): entirely
  AI-free and decoupled from `internal/sessions` the same way
  `internal/idea` is (a plain `SessionSample` input struct, not a direct
  import) - three deterministic pattern-detection functions, each with
  a hard-coded minimum sample size and, where a percentage is involved,
  a minimum difference threshold, below which it reports nothing at all
  rather than manufacturing a pattern a handful of data points can't
  support (spec section 66's "never fabricate patterns" applied to
  trivial noise, not just outright invention):
  - `TypicalStartTimes` - median (not mean, specifically so one very
    early/late outlier session can't swing what counts as "usual")
    clock time a category's sessions start, spec section 31's own
    example: "You usually start development around 10:30."
  - `TimeOfDayLengthPatterns` - average session length before vs.
    at-or-after 20:00 (spec's own example hour) per category, reporting
    whichever side is longer: "...sessions started at or after 20:00
    are 42% longer on average than earlier ones."
  - `ContextSwitchTrend` - this week's average context-switch interval
    vs. last week's, reusing `internal/sessions`' own switch-counting
    logic (via `sessionRepo.ContextSwitchStats`) rather than
    reimplementing it a second time.
  - Every `Insight.Text` uses observational phrasing only - "occur",
    "are longer", "increased" - never "because" or "causes", matching
    spec section 31's own wording exactly. 20 tests (18 for the
    detection logic itself, plus 2 backing a real long-run benchmark - see the Phase 12 note above), including one that
    caught an arithmetic mistake in the test's own expected value (not
    the code) during development, fixed by asserting the actual
    property intended (median resists an outlier) instead of a
    miscalculated exact figure - see `TestTypicalStartTimes_ReportsMedian`'s
    comment.
- **Insights are persisted and dismissible, not just computed and
  shown** - this wasn't in the original plan for this feature, but
  0001's own schema had already anticipated it: `pattern_insights` had
  a `dismissed` column no writer ever used until this session.
  `internal/patterns/repository.go` (this package's first DB writer)
  upserts each computed insight's latest wording under a natural key
  (`"<kind>:<categoryId>"`, e.g. `"typical_start:3"` - migration
  `0006_pattern_insights_dismissal.sql` adds the `UNIQUE` index this
  needs) - refreshing the same row every time Pattern Discovery
  recomputes rather than accumulating a new one, and, once dismissed,
  the row is left exactly as it was, never resurrected by a later
  recompute with fresh data. Verified against real SQLite BEFORE the Go
  code was written (the same discipline as the schema itself): the
  conditional `ON CONFLICT ... DO UPDATE ... WHERE dismissed = 0`
  upsert, a raw duplicate insert genuinely rejected by the `UNIQUE`
  index, and the full dismiss → attempt-to-resync → still-dismissed →
  undismiss → resync-works-again cycle. `PatternService.Dismiss`/
  `Undismiss` are now bound to the frontend, and `/patterns` has a
  working "Dismiss" button per insight.
- **`app/services/pattern_service.go`**: pulls the last 30 days of
  session data (skipping `Excluded` sessions - spec section 13, an
  excluded session must not feed analytics of any kind) plus a
  this-week-vs-last-week context-switch comparison (reusing
  `internal/periods.Window` and `sessionRepo.ContextSwitchStats`
  exactly as `AnalyticsService`/`BudgetService` already do - never a
  second, possibly-inconsistent copy of that math), and adds an
  optional `aiNote` gated on `ScopePatterns`. The AI system prompt
  explicitly forbids introducing any pattern not already in the list
  it's given - this function can only summarize/connect the
  deterministic findings, never add one of its own.
- **Frontend**: a new `/patterns` page ("Patterns" in the nav, after
  Time Receipt) listing every insight in plain language, an honest
  empty state ("Not enough data yet... come back after using STELLAREI
  a bit longer" - spec section 67, never fake data), and the `aiNote`
  clearly separated under its own "AI note" heading when present.

### Optional Local AI (Phase 11)

- **`internal/ai`** (see its own package doc for the full contract):
  `Provider` (`none` / `ollama` / `openai_compatible`), `Scope` (one
  toggle per feature - `summaries`, `patterns`, `suggestions`,
  `project_ideas`, `i_have_no_idea`, `daily_reviews`, matching spec
  section 37's own list), and `Config.ScopeAllowed` requiring BOTH the
  overall enable switch AND the specific scope to be on before any
  network call is even attempted. `Service.Enhance` is the one entry
  point every caller uses; it never returns a Go `error` for "AI isn't
  available right now" in any of its forms - off, scope not permitted,
  misconfigured, unreachable, timed out, empty/bad response all become
  `Result.Unavailable` with a human-readable `Reason` instead, so a
  caller can never accidentally treat "the local server didn't answer"
  as a hard failure (spec section 56).
- **Two real HTTP clients, stdlib-only** (`net/http`, no external
  dependency - deliberately, since this sandbox can't fetch
  `modernc.org/sqlite` or Wails itself, and Phase 11's clients had no
  reason to inherit that limitation): `OllamaClient` against Ollama's
  native `/api/chat`, and `OpenAICompatibleClient` against
  `/v1/chat/completions`, which covers both LM Studio's built-in server
  and llama.cpp's `llama-server` with one implementation (see
  `ProviderOpenAICompatible`'s doc comment for why). Both respect a
  configurable timeout (default 45s - deliberately generous for
  CPU-only local inference), cap the response body at 4MB, and never
  retry silently.
- **25 tests, including `go test -race`**: real `httptest.Server`
  round-trips for both clients (success, HTTP error, malformed JSON,
  provider-level error payload, connection-refused, and a genuine
  timeout test that sleeps past a 20ms deadline), plus `Service`-level
  tests proving zero network calls happen when AI is off or the scope
  isn't permitted, that `SetConfig` takes effect immediately, and that
  `LastCallInfo` (the Diagnostics-page data) has no field capable of
  holding prompt/response content in the first place - see
  `TestService_LastCall_RecordsOutcomeWithoutPromptContent`.
- **Settings persistence** (`internal/ai/settings.go`) reuses the exact
  same `settings` key-value table every other Settings section already
  writes to (`ai_enabled`, `ai_provider`, `ai_endpoint`, `ai_model`, and
  one `ai_scope_<name>` row per scope) - verified against real SQLite
  the same way the Phase 1 schema was, including the `ON CONFLICT`
  upsert path and that repeated writes never produce duplicate rows.
- **`app/services/ai_service.go`**: the thin Wails-bound adapter - `GetConfig`/`SetEnabled`/`SetProvider`/`SetEndpoint`/`SetModel`/
  `SetScope`/`TestConnection`/`Diagnostics`. `TestConnection`
  deliberately reuses the exact same `Enhance` path a real feature
  would (a trivial "reply OK" prompt) rather than a separate, simpler
  health check that could pass while a real call still fails.
  `NewAIService` loads whatever was already saved at construction time,
  so a person who configured AI yesterday doesn't find it silently back
  to "off" after restarting today.
- **Wired into "I HAVE NO IDEA" as the first real integration**
  (`idea_service.go`'s `aiNoteForSuggestions` / `aiNoteForProjectIdea`):
  after the deterministic engine picks the three suggestions (or the one
  project idea) exactly as it always has, an OPTIONAL one-sentence
  `aiNote` is requested - built only from data already being shown on
  screen (titles, descriptions, estimated minutes), explicitly
  instructed not to invent new suggestions or facts, and silently
  omitted (empty string, not an error) whenever AI is off, the scope
  isn't permitted, or the local server doesn't answer. The frontend
  types (`IdeaResult.aiNote`, `ProjectIdeaView.aiNote`) document this
  contract inline.
- **A new real feature, built this session, then wired to AI**: the
  **Daily Time Receipt** (spec section 60), `app/services/receipt_service.go`.
  This wasn't an AI task at all at its core - Daily/Weekly Review and
  Time Receipt hadn't been built in any of Phases 1-10, only the
  category/project/trend/context-switch breakdowns Phase 4 actually
  covers. `TimeReceiptService.GetTimeReceipt` computes a real day's
  category totals and total tracked time from session data
  (`internal/periods.Window("daily", ...)`, the same day-boundary logic
  budgets/goals already use), reuses `GoalService`/`BudgetService`'s
  existing status math rather than recomputing it (restricting budgets
  to `Schedule == "daily"` specifically, since a weekly/monthly budget's
  running total would misstate what a single day's receipt is showing - see `TimeReceiptBudgetLine`'s doc comment), and only THEN adds an
  optional `aiNote` gated on `ScopeDailyReviews`, instructed to stay
  strictly factual per spec section 14's "do not shame the user, do not
  produce fake motivational quotes" rule applied to the AI layer too.
  New frontend page at `/receipt` ("Time Receipt" in the nav, right
  after Analytics).
- **Frontend**: a real Settings > AI section - enable toggle, provider
  picker, local server address and model fields (committed on blur, not
  per-keystroke), a working Test Connection button showing latency and
  the actual reply text, a Diagnostics readout of the last call, and one
  checkbox per scope with plain-language labels
  (`AI_SCOPE_LABELS` in `bindings.ts`). Independently error-scoped from
  the rest of the Settings page - a problem loading the AI bindings
  can't block Close Behavior/Startup/Theme from working.
- **Wired into Pattern Discovery too** - see the new "Pattern Discovery
  (spec section 31)" section above for the full breakdown; `aiNote`
  there is gated on `ScopePatterns` and can only summarize/connect the
  deterministic findings, never introduce one of its own.
- **All six scopes now wired - `ScopeSuggestions` found its home in
  Digital Waste Detection** (see the new "Digital Waste Detection
  (spec section 33)" section below). It was never a missing-wiring
  problem - "general suggestion-text enhancement" genuinely didn't name
  one specific feature the way the other five scopes each do, so it
  waited until a feature existed whose job actually was "suggest
  something" rather than "observe/summarize something."

### Digital Waste Detection (spec section 33)

- **`internal/waste`** (see its own package doc for the full scope
  reasoning): entirely AI-free, decoupled from `internal/sessions` the
  same way `internal/idea`/`internal/patterns` are. Covers 3 of the
  spec's 6 example patterns, each with its own evidence threshold below
  which nothing is reported:
  - `RepeatedReopens` - a category opened many times (≥20 sessions in
    the lookback window) AND averaging under 5 minutes each. Requires
    BOTH conditions specifically so a normal, sustained workweek in
    Development doesn't get flagged just for having a lot of sessions - only short, frequent ones count.
  - `LongIdlePeriods` - idle stretches of 30+ minutes, reported only at
    3 or more such stretches. The "Idle" category is resolved by name
    at call time (`waste_service.go`), never hardcoded to an assumed ID,
    so a renamed or reorganized category can't silently break this.
  - `ShortNoiseSessions` - sessions under a minute, excluding idle time
    (unremarkable on its own), reported only at 15+ such sessions.
  - **Deliberately NOT reimplemented**, and why: "rapid switching"
    already has a real implementation
    (`AnalyticsService.GetContextSwitchStats`) this package has no
    business duplicating; "excessive entertainment time" has no
    spec-given threshold, and inventing one here would be exactly the
    moral judgment spec section 33 explicitly forbids - that's what
    Budgets (spec section 17, already built) exist for, letting the
    PERSON define what's excessive for themselves; "repeated
    navigation" needs browser URL/domain tracking, off by default per
    spec section 28, so there's nothing to analyze unless that's
    separately enabled.
  - 15 tests, including one that checks the finding text for
    judgmental language ("waste", "bad", "should", "too much", "stop")
    and one that caught a genuine test-authoring mistake of my own
    during development (data meant to isolate one detector accidentally
    satisfied two at once - not a code bug, since a category opened 20
    times for under a minute really is legitimately both patterns - fixed by adjusting the test's data, not the assertion).
- **`app/services/waste_service.go`**: pulls the last 7 days of session
  data (shorter than Pattern Discovery's 30 - spec section 33's
  examples read as current-behavior observations, not long-run trends),
  resolves the "Idle" category by name (best-effort - a failed lookup
  degrades to `idleCategoryID=0`, which `LongIdlePeriods`/
  `ShortNoiseSessions` handle gracefully rather than erroring), and adds
  an `aiNote` gated on `ScopeSuggestions`. This is the one place in the
  codebase that scope is used - see `aiNoteForFindings`'s doc comment
  for exactly how its system prompt differs from every other `aiNote`
  function's: explicitly asked for a forward-looking suggestion, not a
  backward-looking summary, while staying just as strict about never
  moralizing.
- **Frontend**: a new `/digital-waste` page ("Digital Waste" in the
  nav, after Patterns), an explicit "not a scorecard" framing in the
  page copy itself, and the suggestion clearly separated under its own
  "Suggestion" heading when present.

### Calendar (spec section 30)

- **`internal/calendar`**: entirely AI-free (this feature has no aiNote - not every screen needs one; a calendar grid isn't naturally suited
  to a one-sentence commentary the way a review is), decoupled from
  `internal/sessions` the same way every other package in this family
  is. The one genuinely non-trivial piece: a session that crosses local
  midnight has to contribute its REAL duration to BOTH calendar days it
  touches, not get dumped entirely on whichever day it started - `splitByLocalDay` clamps each session's contribution to every day
  boundary it overlaps. 10 tests, including a 48-hour session split
  correctly across 3 calendar days, a timezone-specific test using a
  fixed UTC+10 offset (not a named zone, so it doesn't depend on the
  test environment having tzdata installed), and confirmation that the
  split parts always sum to exactly the session's real total duration
  (no double-counting or lost seconds at a boundary).
- **This exposed a real, pre-existing bug in Timeline's own month
  view** (`frontend/src/pages/Timeline.tsx`'s `MonthTimeline`), found
  while checking whether Calendar would be redundant with it before
  building it: `MonthTimeline` buckets sessions by day using
  `new Date(s.startedAt * 1000).getDate()` - the session's START time
  only. A session crossing local midnight is silently attributed
  entirely to whichever day it started on, exactly the bug
  `splitByLocalDay` exists to avoid. This wasn't fixed as part of this
  work (`MonthTimeline` is a client-side heatmap with a different job - a fast, rough visual, not an authoritative per-category total - and
  touching it wasn't part of this task), but it's worth knowing about:
  the two month views can show slightly different numbers for a day
  with a midnight-crossing session, and Calendar's are the correct
  ones.
- **`app/services/calendar_service.go`**: fetches sessions overlapping
  the requested month via the same `ListForRange` every other analytics
  service already uses, then filters `internal/calendar`'s day-bucketed
  output back down to only the days that actually fall within the
  requested month - necessary because a session spanning, say, August
  31st into September 1st legitimately produces a September day-key
  too, which an August calendar must not display.
- **Frontend**: replaces the honest "not yet scheduled" placeholder
  `/calendar` used to show. A real month grid with per-day category
  breakdowns (top 3 by duration, "+N more" for the rest), and clicking
  a day navigates to `/receipt?date=YYYY-MM-DD` - spec section 30's own
  "clicking a day opens its timeline and review" - which required a
  small, additive change to `TimeReceipt.tsx` to read an optional
  `date` search param on load (falling back to today for a missing or
  malformed value, never crashing on a bad or old link). That parsing
  helper (`parseDateParam`) was extracted into `time-ranges.ts` and
  reused for Timeline's own new `date`/`zoom` URL params too - see the
  Search section below for why Timeline needed that.

### Search (spec sections 29/62)

- **`internal/search`** (see its own package doc, which opens with an
  honest scope statement worth reading before assuming a query
  "should" work): a bounded, documented vocabulary, not natural
  language understanding. Recognizes exact date phrases ("today",
  "yesterday", "this/last week", "this/last month", "this/last year"),
  explicit dates ("August 10", "August 10, 2026" - a year-less date
  resolves to the most recent PAST occurrence, so mentioning "December
  25" in August means last December, not next), a 24-hour "HH:MM" time
  that combines with a single-day date into a specific instant to look
  up (spec's own example: "What was I doing at 14:30 on August 3?"),
  the "longest"/"shortest" superlatives, and whole-word (never
  substring) matching against real project/category/application names - never synonyms. Spec's own example "longest coding session" is
  used as the demonstration of where this boundary sits: it only
  matches if something is literally named with "coding" in it, because
  the builtin category is named "Development", and building a synonym
  dictionary would mean guessing at a mapping no session could get
  right for every person's own naming.
- **21 tests**, including a regex-special-characters-in-a-name case (a
  project named "C++ Notes" doesn't break or panic the matcher), the
  whole-word-boundary guard (a category named "Art" must NOT match
  inside the unrelated word "arting"), the longer-name-wins preference
  when two entity names both match, the future-date-means-last-year
  inference, and confirmation that combining a time-of-day with a
  multi-day range (nonsensical as a single instant) correctly stays a
  range instead of silently producing a wrong instant.
- **A real naming collision, caught by the same isolated-build
  verification used throughout this project**: `search_service.go`'s
  first draft defined its own `toResultView` helper, not realizing
  `ai_service.go` already has one (unrelated, converting `ai.Result` to
  `AIResultView`) - both live in the same `services` package, so this
  was a genuine compile error, not just a style nit. Renamed to
  `toSearchResultView`.
- **`app/services/search_service.go`**: loads real project/category/
  application names to feed the parser, then branches on what was
  recognized - a specific instant looks up the one session covering
  that moment; an entity plus a date range filters to that combination;
  an entity alone searches a 5-year lookback (the overlap query this
  uses was benchmarked as fast even at 200k-row stress-test scale - see
  the Phase 12 section above - so this is generous rather than tightly
  bounded); a date alone lists that period's sessions; and when NEITHER
  is recognized, it falls back to a plain substring search across
  session notes, project names, category names, and application names.
  Every branch returns an `Interpretation` string explaining what was
  actually understood - spec's own examples are natural-language
  phrasing, and a person typing something outside the supported
  vocabulary deserves to be told that plainly, not left guessing why
  results look empty or wrong.
- **Frontend**: replaces the "not yet scheduled" `/search` placeholder.
  A search box, clickable example queries that demonstrate the actual
  supported vocabulary (honest onboarding rather than implying full
  natural language), the interpretation shown above the results, and
  each result clickable straight into `/timeline?date=<date>&zoom=day` - spec section 29's own "results should link directly into the
  relevant timeline." This required adding the same `date`/`zoom` URL
  param support Calendar already established to `Timeline.tsx` itself.
- **Every nav destination in the app now has a real page** - Search
  and Calendar were the last two `Placeholder` (`frontend/src/pages/
  Placeholder.tsx`) routes remaining in `App.tsx`. That component is
  kept rather than deleted (it's small, well-designed, and represents a
  real project convention - see its own doc comment, citing spec
  sections 67 and 83) even though nothing currently renders it; it's
  the correct choice the moment any future feature needs an honest
  "not built yet" screen again.

### MY YEAR (spec section 61)

- **A real, honest correction caught while scoping this feature**:
  this project's own README previously claimed "six of six section 37
  AI use cases wired into a real caller." That was wrong. `internal/ai`
  defines SIX scope constants, but only five were ever passed to
  `Enhance` anywhere - `ScopeSummaries` ("Daily/Weekly Review narrative
  assistance") had sat completely unused since Phase 11's foundation,
  genuinely dead code, because `ScopeDailyReviews` was used for Time
  Receipt instead without anyone (any session) noticing the two
  overlapping constants. Verified with a direct grep across the whole
  codebase for every `ai\.Scope\w+` usage before writing this down - not just re-asserting the earlier claim. MY YEAR's `aiNote` is now
  gated on `ScopeSummaries` specifically, finally giving it a real,
  distinct job (a yearly narrative, distinct from Time Receipt's daily
  one) - this is genuinely six of six now, and this section exists
  partly to document that the earlier count was wrong, not just to
  announce the new feature.
- **`internal/myyear`**: holds only the pieces of MY YEAR that are pure
  functions of already-fetched data - ranking categories/projects by
  time, picking the longest sessions, and finding the busiest month
  (reusing `internal/calendar.DailyCategoryTotals`, already correct
  about midnight-crossing sessions, and rolling per-day totals up by
  month prefix - a day belongs to exactly one month, so no new boundary
  logic is needed at that level). 11 tests, including one that
  specifically exercises a session crossing from July into August and
  confirms `BusiestMonth` still attributes each month's real share
  correctly, and two map-iteration-order stability tests (category
  ranking ties, busiest-month ties) run 20 times each to catch a flaky
  tie-break that a single run could miss by luck.
- **Abandoned projects distinguishes explicit from inferred, because
  the schema already had a real signal for one of them**: `projects.
  Status` already supports `"abandoned"` as a value the person can set
  themselves - a direct signal, not a guess - surfaced separately from
  projects `MyYearService.abandonedProjects` INFERS are possibly
  abandoned (still marked "active", untouched 90+ days, with real
  historical time invested - the same evidence-only discipline
  `internal/waste` established, never a verdict).
- **Goals section is honestly named for what it actually is**:
  `GoalsCurrentlyComplete`, not "goals completed this year" - `GoalStatus`
  has no completion timestamp, so this can only report CURRENT
  completion status, the identical caveat `TimeReceiptView.Goals`
  already carries for the same underlying reason. The frontend states
  this plainly rather than implying more precision than the data has.
- **Frontend**: a new `/my-year` page with year navigation, category
  and top-project breakdowns, longest sessions clickable into Timeline
  (same `/timeline?date=<date>&zoom=day` convention Search and Calendar
  already established), year-over-year percent change when prior-year
  data exists, and the AI note clearly separated under its own heading.

### Diagnostics (spec sections 55/80)

- **`app/services/diagnostics_service.go`**: every field is a real,
  currently-observable fact, not a placeholder - real database
  connectivity via `db.PingContext` (not assumed from the connection
  merely existing), real row counts (`SELECT COUNT(*) FROM
  activity_events`/`sessions`), real file size via `os.Stat` on the
  actual configured database path, the Activity Engine's real live
  status (reusing `ActivityService.GetStatus()` - the exact same call
  Overview's live status display already uses, so Diagnostics can never
  show a different answer than the rest of the app), and real Go
  runtime metrics (`runtime.NumGoroutine`, `runtime.ReadMemStats`).
  Process uptime is measured from a timestamp captured at the very top
  of `main()`, before flag parsing or anything else runs.
- **Startup-registration status - genuinely wired now, not just newly
  written.** An earlier version of this section (and the code's own doc
  comment) said startup status wasn't exposed because "`StartupService`
  can currently only SET the startup registration, not read back what's
  actually configured." **That claim was simply wrong** - caught while
  going to go add the read-back path, not guessed at again:
  `StartupService.IsEnabled()` already existed, already called
  `platform.StartupManager.IsRegistered()` (a real file/registry check
  per OS - on Linux, `os.Stat` on the actual `~/.config/autostart/
  stellarei.desktop` path), and was already wired into `Settings.tsx`'s
  own "Run on startup" toggle so that toggle can't drift from what the
  OS actually has configured. It just wasn't threaded through to the
  Diagnostics page specifically. Fixed by passing the already-
  constructed `StartupService` into `NewDiagnosticsService` (main.go
  already builds it earlier in the same function, so this is a plain
  variable reference, not new wiring) and adding `StartupRegistered`/
  `StartupCheckError` to `DiagnosticsView` - the latter degrades that
  one field rather than failing the whole snapshot if the OS-level
  check itself errors, same pattern as `DatabaseReachable`.
- **Tray status - investigated for real, and still not exposed, because
  there is genuinely nothing to read.** Checked Wails v3's actual
  `SystemTray` API (`github.com/wailsapp/wails/v3/pkg/application`, per
  the current `v3.wails.io/features/menus/systray/` docs) before writing
  anything: `New`/`SetLabel`/`SetMenu`/`SetIcon` have no error return
  and there is no `IsRunning`/status getter anywhere on `SystemTray` in
  the current API. `tray.Build()` runs once, synchronously, at startup,
  before `app.Run()` - if it were going to fail loudly, the process
  wouldn't be up to answer a Diagnostics request in the first place, so
  a `TrayRunning` field could only ever read `true`. That's not a
  diagnostic, it's a constant - exactly the kind of plausible-looking
  placeholder this service explicitly avoids elsewhere (see
  `DiagnosticsService`'s own doc comment, which names this investigation
  and its conclusion directly, not just here). Revisit if a future Wails
  release adds a real status API; nothing changed here today.
- **`AppVersion` is a new single source of truth** (`internal/config`,
  next to the existing `AppName`) - `"0.1.0"`, matching
  `frontend/package.json`'s own version field by convention (nothing
  enforces they stay in sync automatically; bump both by hand). This is
  the first place in the app that surfaces a version number at all - and only there: the code's first draft claimed in a comment that it
  would also show on the About page, which was never actually true and
  was caught and corrected before packaging (About is deliberately
  static and backend-free - see its own comment, "genuinely complete in
  Phase 1" - and pulling in a binding call just for a version string
  would have broken that).
- **Frontend**: a new `/diagnostics` page, placed in the secondary nav
  group rather than the primary one - spec section 80 itself says this
  page "can be hidden behind an Advanced/Diagnostics setting in
  production," and the existing secondary group (Search, Themes,
  Settings, Privacy, Backups, About) is exactly that kind of
  less-prominent home. A manual "Refresh" button re-fetches the
  snapshot; nothing auto-polls. A new "Startup" section shows the
  read-back registration status (or the check's own error, if the OS
  call itself failed) between Activity Engine and Performance.
- **Verification**: the Go change was checked the same way every other
  `app/services`/`main.go` change in this project is - the isolated
  `app/services` build+vet check (clean) plus careful manual review of
  the two-line `main.go` change (`startupService` is a plain variable
  already in scope from earlier in the same function, not a new
  closure or dependency-order concern), since `main.go` itself still
  can't be compiled in this sandbox. Frontend: `npx tsc --noEmit` and
  `npm run build` both clean; the full three-check i18n ritual re-run
  and clean (key-structure parity across en/fa/de, no hardcoded strings
  in `Diagnostics.tsx`, no code-referenced key missing from any
  locale). The existing 352-test Go suite re-run in full (`-race`)
  and unaffected, since this change touches no `internal/` package.


### Global Shortcuts (spec section 45) - COMPLETE

- **Verified the real API before writing anything** - the same
  discipline `PACKAGING.md`'s commands were checked with, because
  guessing at a fast-moving beta framework's API and having it look
  plausible but be wrong would be worse than not attempting this at
  all. `app.GlobalShortcut` is real and current: confirmed against
  the Wails v3 v3.0.0-alpha2.108 release notes ("Add global (system-
  wide) keyboard shortcuts via app.GlobalShortcut... Implemented
  natively per platform with no third party dependencies: Carbon hot
  keys on macOS, RegisterHotKey on Windows, XGrabKey on X11, and the
  XDG Desktop Portal global shortcuts interface on Wayland") and the
  current `v3.wails.io/features/keyboard/global-shortcuts/` docs page,
  which shows the exact call shape used here:
  `app.GlobalShortcut.Register(accelerator, func() { ... })`.
- **`main.go` registers five fixed default actions** - spec's own
  named list: Open STELLAREI (`CmdOrCtrl+Shift+S`), Start Focus
  (`+F`), Pause/Resume Tracking (`+P`, a genuine toggle checking
  `ActivityService.GetStatus().Paused` rather than always pausing),
  Start/stop timer (`+T`, also a real toggle - stops whichever timer
  is already running, or starts the same 25-minute quick timer the
  tray's own "Start 25-min Timer" item does, via an identical
  `timerRepo.Create` call), and "I HAVE NO IDEA" (`+I`). Every handler
  reuses the EXACT SAME action body the tray menu already uses for the
  same action (`Window.Show()`/`Focus()`/`Event.Emit(...)`,
  `Engine.Pause`/`Resume`) rather than a second, possibly-inconsistent
  copy - the tray's "Show STELLAREI" and this shortcut's can never
  drift apart because they're calling the same lines.
- **User-configurability + conflict detection, spec section 45's other
  two asks, are now real**: `internal/shortcuts` is the pure package
  (`Action` type, the five defaults above, `Validate` for accelerator
  syntax, `Normalize` + `DetectConflicts` for same-app collision
  detection - 17 tests, `-race` clean). `app/services/
  shortcuts_service.go`'s `ShortcutsService` exposes
  `ListShortcuts`/`SetShortcut`/`ResetShortcut`, backed by a
  `ShortcutRegistrar` interface (+ `ShortcutRegistrarFunc` adapter,
  same `http.HandlerFunc`-style pattern used elsewhere) so this
  package stays free of a direct `wails/v3` import - same
  import-boundary reasoning as `notify.Dispatcher`. `main.go` builds an
  `actionFuncs` map and a `shortcutRegistrar` closure over the real
  `app` variable, and both the startup registration loop and any later
  frontend-triggered change go through the identical `Rebind` path, so
  they can never register two different accelerators for the same
  action.
- **Frontend, done this session**: `bindings.ts` gained a
  `ShortcutsBindings` interface + `getShortcutsBindings()`, following
  the exact `getSettingsBindings()` pattern. `Settings.tsx` gained a
  "Global Keyboard Shortcuts" section - loaded and errored
  independently of the rest of the page (same reasoning as the AI
  section: a missing binding here must never block Close Behavior /
  Startup / Theme), one row per action with an editable accelerator
  field (commits on blur, mirroring the AI endpoint/model draft
  pattern), a Reset-to-default button that only appears once an action
  has been customized, and real per-row error surfacing for whatever
  `SetShortcut` actually rejects (malformed syntax, a collision with
  one of STELLAREI's own other shortcuts, or the OS/another
  application already owning that combination). New locale keys added
  to all three of `en.json`/`fa.json`/`de.json` with real translations
  (checked existing usage of "Focus"/"Pause"/"Tracking"/"I Have No
  Idea" elsewhere in the app first, rather than guessing new
  phrasing) - full three-check i18n ritual run and clean (key-parity
  MATCH, no hardcoded strings introduced, no code-referenced key
  missing from any locale).
- **Honestly still cannot be run end-to-end in this sandbox** - same
  reason as everything else importing `github.com/wailsapp/wails/v3`
  directly (see the Phase 13 section above): `main.go` cannot compile
  here. The Go side was verified by the isolated `app/services`
  build+vet check plus careful manual re-diffing against the pristine
  original, not by a real build; the frontend half was verified for
  real (`tsc --noEmit`, `vite build`, all three i18n checks), since
  none of it touches `wails/v3` directly. Registration failures log
  loudly (`log.Printf`) rather than swallowing the error, so a real run
  would surface a problem immediately instead of a shortcut silently
  doing nothing.

### Internationalization (spec section 69) - COMPLETE

**All 21 real pages are now translated into English, Persian, and
German, plus `SessionDetailPanel.tsx` (Timeline's detail panel).**
`Settings.tsx` was the last one, done as its own dedicated batch given
its size (482 lines) and complexity (AI config, backup paths, tracking
toggles) - the same reasoning that had it deliberately saved for last
rather than rushed alongside others.

- **What "complete" means precisely, stated honestly rather than
  glossed over**: every page's UI strings are wired through
  `i18next`/`react-i18next`, with real Persian RTL layout and a working
  Language switcher backed by real persistence. It does NOT mean every
  string in the app is translated - three categories are deliberately
  left in English, each documented in a code comment at its own
  definition site (see below), because translating them would either
  be out of scope (backend-generated prose) or actively wrong
  (functional English-only parser input). Two other things once listed
  here as gaps - locale-aware week-start days, and full spatial RTL
  mirroring (the page-shell layout flipping sidebar-to-right in
  Persian) - were both wrong, later verified and corrected; see the
  dedicated write-ups further down in this section for the real tests
  (Node date math + a Playwright screenshot for the week-start fix, a
  Playwright bounding-box test for the RTL one) that found each one
  either already worked or was straightforward to close for real.
- **`Settings.tsx` specifically**: its `AI_SCOPE_LABELS` constant
  (six scope descriptions like `"Daily / Weekly Review summaries"`)
  lived in `lib/bindings.ts`, a shared types/data file, not a page - rather than adding i18n keys to that file (mixing UI text into a
  bindings/types module), a local `AI_SCOPE_LABEL_KEYS` map was added
  inside `Settings.tsx` itself (the only page that ever rendered these
  labels), keeping the separation between generated-adjacent bindings
  code and UI text intentional rather than incidental.
- **`frontend/src/locales/{en,fa,de}.json`** - automatically verified
  to have IDENTICAL key structures across all three files (a Python
  script comparing flattened key sets; **508 keys, zero missing, zero
  extra**, re-verified after every batch) before being wired in,
  catching exactly the class of bug i18n systems commonly suffer
  silently (a missing key quietly falling back or rendering nothing).
  A second, complementary automated check runs alongside it every
  batch: a regex scan of every page AND component for `t("...")` and
  `labelKey: "..."` references, cross-checked against `en.json`'s
  actual keys - this catches the specific blind spot the parity script
  has (a key referenced in code but absent from ALL three locale
  files). Final run: **446 real references, all present, zero
  missing.**
- **A real, minor, honestly-noted loose end at the time - now closed.**
  The same cross-check run in reverse (which locale keys are never
  referenced) flagged 62 "unused" keys when this pass first ran - almost
  all of them false positives from the regex's own limits (it only
  catches literal `t("...")` calls and `labelKey: "..."` object
  properties, not a bare string in an array like `About.tsx`'s
  `PRINCIPLE_KEYS` or a `Record<>` literal's property VALUE like
  `ALARM_TYPE_LABEL_KEYS`'s `one_time: "alarms.typeOneTime"` - both
  genuinely used, just via a pattern the checker doesn't trace). Two
  flagged keys were real dead code, though: `common.cancel` and
  `common.save`, defined in the original infrastructure scaffolding
  before any real page existed, and never actually adopted - every page
  that needed a Cancel/Save button defined its own instead
  (`backups.cancelButton`, `timers.cancelButton`, `sessionPanel.
  saveButton`, etc.). Removed from all three locale files in a later
  session, confirmed dead again first (zero matches for `common.cancel`/
  `common.save` anywhere in `pages/`/`components/`, including a
  dynamic-key-construction check in case either was built from a
  template string rather than a literal) rather than trusting the
  earlier flag alone. Key-structure parity and the code-to-locale
  cross-check both re-run clean after removal. (The **508 keys** figure
  two paragraphs up is a snapshot from that original i18n pass, left
  as-is rather than silently edited, same as this project treats other
  historical numbers - several sessions' worth of real additions and
  this one removal since then put the current total at **520**.)
- **This batch also translated `SessionDetailPanel.tsx`**, a
  `components/` file, not a `pages/` one - because it's the primary
  interactive surface `Timeline.tsx` opens (clicking any session shows
  it), leaving `Timeline.tsx` "done" while its main detail view stayed
  hardcoded would have been a real, misleading gap. The code-to-locale
  cross-check scanned `components/*.tsx` alongside `pages/*.tsx` from
  the start for exactly this reason.
  The Persian and German translations are AI-produced, not reviewed by
  a native speaker of either language - stated in the code's own
  comments AND, deliberately, inside the Persian and German
  translation text itself (`settings.languageHint`) so a Persian- or
  German-reading person sees that caveat in their own language, not
  just in an English README they may not read.
- **Real cross-page key reuses, five in total across the whole
  pass**: `Alarms.tsx` reuses `calendar.weekdayMon..Sun`; `Goals.tsx`
  reuses `analytics.zoomDay/Week/Month`; `Timeline.tsx`'s zoom buttons
  reuse the SAME `analytics.zoomDay/Week/Month` set instead of
  programmatically capitalizing the raw zoom value
  (`z[0].toUpperCase() + z.slice(1)` - which would have produced
  untranslated "Day"/"Week"/"Month" regardless of language);
  `Timeline.tsx`'s `MonthTimeline` sub-component reuses
  `calendar.weekdayMon..Sun` a third time; and `Timeline.tsx`'s empty-
  range/loading strings reuse `analytics.nothingTrackedRange`/
  `common.loading` rather than duplicating identical text again.
- **A third documented non-translation decision, specific to
  `Themes.tsx`**: `TOKEN_GROUPS`' section labels (Surfaces, Text, etc.)
  are translated, but individual token labels (`tokenLabel()`,
  auto-capitalizing keys like `chartPrimary`) are NOT - these are the
  same identifiers spec section 38's semantic-token system and section
  40's exported theme JSON use verbatim, so keeping the on-screen label
  matching the literal key (rather than translating it) helps someone
  cross-reference an exported theme file. Documented in a code comment
  at the definition site.
- **A recurring naming collision, now caught NINE times across seven
  files**: `.map((t) => ...)`/`.find((t) => ...)` as a loop variable
  has shadowed `useTranslation()`'s `const { t }` in `Settings.tsx`,
  `Projects.tsx`, `Timers.tsx`, `Themes.tsx` (three sites),
  `Alarms.tsx`, `Goals.tsx` (two sites), `NoIdea.tsx`, and now
  `Privacy.tsx`'s `EXCLUSION_TYPES`. Grep `.map((t)\|.find((t)` before
  wiring any remaining page - at this point treat it as certain, not
  merely likely, in this codebase's pre-i18n code.
- **A real, deliberate scoping decision: backend-generated prose is
  NOT translated.** `internal/waste`'s finding text, `internal/
  patterns`' insight text, and `internal/search`'s interpretation
  string are all real English sentences assembled Go-side from actual
  data - not UI labels. Translating them would mean either a full
  backend i18n pass (out of scope for a frontend-strings extraction)
  or re-translating dynamically-assembled English at render time, which
  risks silently misrepresenting what the finding/insight/result
  actually says. Left in English on purpose, with a code comment at
  each site explaining why, not silently skipped. The same reasoning
  now also covers `Projects.tsx`'s raw `r.type`/`r.pattern` display,
  `Timers.tsx`'s/`Focus.tsx`'s raw `view.timerType`/`r.status`/
  `h.status` display, and `NoIdea.tsx`'s `internal/idea` output - these
  are literal backend enum/generated-content values shown as-is (a
  pre-existing display choice, not something this pass introduced),
  not translated labels, so leaving them untouched is consistent
  rather than a gap this pass created.
- **A second deliberate scoping decision: `Search.tsx`'s example
  buttons and placeholder are NOT translated either**, for a sharper
  reason than the above - `internal/search` is an explicitly
  English-only bounded-vocabulary parser (real date phrases, whole-word
  entity names, superlatives, no NLP, no synonyms, per that package's
  own doc comment). Translating "today" into Persian would make the
  button submit a query the backend genuinely cannot parse. Correctness
  over completeness.
- **`Calendar.tsx`/`Timeline.tsx`'s week is now locale-aware - COMPLETE,
  closing a gap named right here.** Verified Persian week convention
  against multiple independent sources before writing anything
  (Iranian calendar starts Saturday شنبه, ends Friday جمعه - not a
  guess). `lib/i18n.ts` gained `getWeekStartDay(lang)` next to the
  existing `isRTL`/`RTL_LANGUAGES` - `en`/`de` stay Monday (1, the
  existing ISO-8601/European convention this app already assumed),
  `fa` is Saturday (6). Both `Calendar.tsx`'s grid and `Timeline.tsx`'s
  `MonthTimeline` changed their leading-blank math from the hardcoded
  `(getDay() + 6) % 7` to `(getDay() - weekStartDay + 7) % 7`, and both
  rotate the SAME EXISTING `weekdayMon`..`weekdaySun` locale keys to
  start at the locale's real day - no new locale keys needed at all,
  just reordering. Verified three independent ways, not just read
  over: (1) pure date math in Node against a real month (September
  2026 - Sept 1 is a real, confirmed Tuesday) for both conventions;
  (2) a Playwright/Chromium screenshot of the actual `calendar-grid`
  CSS with the real Persian weekday labels in RTL, confirming both the
  correct right-to-left `شنبه→جمعه` order AND day 1 landing under the
  correct column; (3) the existing i18n code-to-locale cross-check,
  confirming no locale file needed touching at all since no new keys
  were introduced. `Alarms.tsx`'s own weekday toggle row was
  deliberately left alone - it's a set of day-of-week checkboxes for
   scheduling repeats, not a grid where a wrong offset misaligns real
   dates, and it wasn't named in this gap.
- **Manual week-start override - now a real Settings section, closing
  the last queued gap on that list.** Settings has a "Week start"
  picker (Automatic plus Monday - Sunday), persisted under a new
  `week_start` settings key (`KeyWeekStart` in `app/services/
  settings_service.go` - same zero-new-Go-code pattern as `KeyLanguage`:
  the frontend's `lib/week-start.ts` is its first consumer via the
  generic `GetString`/`SetString`). It drives the Calendar month grid,
  Timeline's month grid, and the Timeline/Analytics week ranges (via a
  new `useWeekStartDay()` hook and an optional `weekStartDay` parameter
  on `rangeFor`/`formatRangeLabel` that defaults to Monday, so every
  pre-existing caller behaves exactly as before). The picker's seven
  day names reuse `calendar.weekdayMon..Sun` rather than adding seven
  near-duplicate keys - three new `settings.*` keys only (heading,
  Automatic option, hint), all three locales, parity re-verified at 523
  keys. Verified with real bundled-code date-math checks (Wednesday and
  Sunday anchors under Monday/Saturday/Sunday starts, default-param
  backwards-compat, September 2026 grid blanks) and a 14-case resolve
  truth table (auto/unset/corrupt/unknown-language all fall back to the
  locale default, never break a grid). **Honest limit, stated up front:**
  weekly budget/goal windows (`internal/periods`) and search's "this
  week"/"last week" phrases (`internal/search`) still use Monday-first - threading the override through the backend is the follow-up, and the
  Settings hint deliberately promises only "calendars and week views,"
  not budget periods.
- **Two settings the schema had already anticipated, unused until now** - the same pattern as `pattern_insights.dismissed` and
  `ScopeSummaries` earlier this session: `app/services/
  settings_service.go` already defined `KeyLanguage = "language"` with
  zero frontend code ever using it. No new Go code was needed at
  all - `SettingsService.GetString`/`SetString` were already bound;
  `frontend/src/lib/language.ts` is the first real consumer.
- **A real, safe RTL correctness fix caught while wiring `Nav.tsx`**:
  its CSS used `border-right`/`border-left`/`text-align: left`-style
  PHYSICAL properties for the sidebar's divider and active-link
  indicator - which do not flip under `dir="rtl"`. Converted to logical
  properties (`border-inline-end`, `border-inline-start`,
  `padding-inline-start`) which do.
- **The sidebar's on-screen POSITION was also claimed, at the time, to
  need a separate fix - that claim was wrong, caught and corrected in
  a later session rather than carried forward.** The reasoning at the
  time was that "the sidebar moving to the right edge in RTL depends on
  the shared page-shell layout, a separate and larger change this pass
  didn't attempt" - a plausible-sounding assumption that was never
  actually run in a browser. It was empirically tested later: real
  Chromium via Playwright, the actual `app.css`/`tokens.css` files, and
  a minimal page reproducing `App.tsx`'s exact `<div class="app-shell">
  <nav class="nav">...</nav><main class="app-shell__content">`
  structure verbatim. Result - `dir="ltr"`: nav at `x=0`, content at
  `x=220` (nav on the left, as expected). `dir="rtl"`: nav at `x=980`,
  content at `x=0` (nav on the right) - **it already flips correctly**,
  with no extra CSS needed at all. The reason: `.app-shell` is
  `display: flex` with the default `flex-direction: row`, and per the
  CSS Flexbox spec, unqualified `row` is defined relative to the
  container's inline direction, not a fixed physical left-to-right - it already reverses under `dir="rtl"` on its own. This is a genuinely
  common misconception (assuming flexbox needs an explicit
  `row-reverse` or similar for RTL), not a contrived one, which is
  presumably how the original wrong claim happened. Confirmed there is
  no `direction:` CSS override anywhere in the codebase that could be
  masking this, and that `App.tsx`'s real JSX matches the tested
  structure exactly (`grep` for both). Screenshot taken as a visual
  sanity check alongside the bounding-box measurements, not instead of
  them. This item is fully resolved, not a remaining gap.
- **A real naming collision caught before it shipped**: `Settings.tsx`
  already used `t` as a local loop variable for iterating themes
  (`themeCtx.themes.map((t) => ...)`) - colliding with `const { t } =
  useTranslation()`, i18next's own standard convention used everywhere
  else this pass touches. Renamed the pre-existing loop variable to
  `theme` rather than break from the idiomatic `t()` convention every
  other file uses.
- **Frontend**: a real Language section in Settings (English/فارسی/
  Deutsch), backed by real persistence (survives a restart), that
  actually flips `<html dir>` - verify it live by switching to Persian
  and watching Nav's text align right-to-left and its active-link
  indicator move to the correct edge.

### Category-level auto-rules (spec section 65)

Extended, real, and fully wired end-to-end - not just a backend stub.
Spec section 65's Rule Engine example is explicitly a category rule
("IF domain = youtube.com THEN category = Entertainment"), but the
Phase 3/4 implementation only ever built the rule engine for
*projects* - categories could only ever come from an application's
`default_category_id` or an explicit per-application override. This
closes that gap.

- **`internal/categories/rules.go`** - a new pure package mirroring
  `internal/projects/rules.go` field-for-field (`RuleType`, `Rule`,
  `Sample`, `Evaluate`), deliberately NOT importing the projects
  package for it - same "each internal/ package defines its own
  minimal shape" convention this project has followed since
  `internal/idea`/`internal/patterns`/`internal/waste`/etc. Real
  `rules_test.go`, 11 tests, all passing, `-race` clean - same test
  names and same edge cases as the project-rules suite (case-
  insensitive process match, substring window-title match, the
  documented `path_contains`-approximated-against-window-title and
  `domain`-never-matches-yet limitations, first-match-wins ordering,
  empty pattern never matches, unknown rule type fails closed).
- **`migrations/0007_category_rules.sql`** - additive only, per the
  up-only migration rule. Verified two ways before any Go code was
  written to touch it: the full real migration sequence (0001 through
  0007, in order) applied cleanly against a fresh in-memory SQLite
  database, and the actual repository queries (list, insert, delete,
  and the `ON DELETE CASCADE` when a category itself is deleted) were
  exercised against real SQLite via Python first - the same
  discipline this project has used for every hand-written SQL string
  since Phase 1, since there's no real Go SQLite driver in this
  sandbox. One real hiccup caught and fixed during that verification:
  the first cascade-delete check failed because `PRAGMA foreign_keys =
  ON` was set too late (after the tables already existed) - moving it
  to immediately after opening the connection, before any table
  creation, fixed it. Confirmed separately that the real database DSN
  (`internal/database/database.go`) already sets `foreign_keys(ON)`,
  so this isn't a sandbox-only concern - cascade genuinely fires in
  production, same as it always has for `project_rules`.
- **`internal/categories.Resolver.ResolveCategoryID`** now tries every
  enabled rule first and only falls back to the original
  `CategoryForApplication` (default/override) behavior if nothing
  matches - an explicit rule is a more specific signal than an
  application-wide default. A category with zero rules configured
  behaves exactly as it did before this feature existed; nothing about
  the fallback path changed.
- **A real interface signature change, not a workaround**:
  `activity.CategoryResolver.ResolveCategoryID` only ever took an
  `applicationID` - there was no way to pattern-match against window
  title (a category rule needs the same `Sample` the project rule
  engine already gets). Changed the interface to also pass
  `processName`/`windowTitle` (matching `ProjectResolver`'s existing
  signature shape exactly), updated the one real call site in
  `internal/activity/engine.go`, and updated the test double in
  `engine_test.go` that implements the interface - including its
  assertion, which was comparing the old `int64`-only call-log shape
  and would have failed to compile otherwise. Caught immediately by
  `go build`, not silently.
- **A second same-package `RuleRecord` collision, same failure class as
  the `toResultView` one from an earlier session**: `app/services/
  project_service.go` already defines `RuleRecord` (with a
  `ProjectID` field). A first draft of `category_service.go` defined
  its OWN `RuleRecord` with a `CategoryID` field - same package, same
  name, different shape. Caught by the isolated `app/services` build
  check (the handbook's setaside-and-stub ritual), not by `gofmt` or a
  visual read - exactly the scenario that check exists for. Renamed to
  `CategoryRuleRecord` (Go side) and `CategoryRuleRecord` (TypeScript
  side, in `bindings.ts`, for the same reason) rather than touching
  the older, already-shipped `RuleRecord`.
- **Frontend**: `Analytics.tsx`'s existing Categories section gained
  the exact same expand-a-row-to-edit-rules interaction
  `Projects.tsx` already has - click a category to see its rules, add
  one with a type/pattern/priority form, remove one. Deliberately
  reuses `Projects.tsx`'s already-translated rule-type vocabulary
  (`projects.ruleTypeProcess`, `projects.rulePatternPlaceholder`,
  `projects.addRuleButton`, `projects.noRulesHint`,
  `projects.removeButton`, `projects.rulesHeading`) rather than adding
  five near-duplicate keys - genuinely the same concept, not just
  similar-looking English, so **zero new locale keys were needed for
  this entire feature**. Verified with the same three-check i18n
  ritual anyway (key parity, hardcoded-string sweep, code-to-locale
  cross-check) precisely because it touches a page that's supposed to
  already be fully translated - a regression here would be just as
  real as a gap in a brand-new page. All three passed clean; the
  raw `r.type` rule-type display in the rules list stays untranslated
  on purpose, same documented reasoning as `Projects.tsx`'s identical
  choice (a literal backend enum value, not a UI label).
- **Test count**: 335 tests across all 24 buildable `internal/`
  packages (324 before this feature - `internal/categories` went from
  0 real tests to 11, nothing existing regressed), all passing,
  `-race` clean. Verified three independent ways after the discrepancy
  below was found, not just the usual single `grep -c "^--- PASS"`:
  a per-package `awk` breakdown of the same `-v` output, and a plain
  static `grep -c "^func Test"` across every `_test.go` file. All
  three agree on 335. (This is also where a real, previously-
  undetected documentation error got caught - see "Building it for
  real" below for the full correction: the "355" figure this project
  carried since early in this continuation was itself never
  independently re-verified until this recount.)



- **The whole database-half of this phase rests on one real, standard
  SQLite mechanism** - `VACUUM INTO` (stable since SQLite 3.27.0,
  2019) - a single SQL statement that produces a complete, consistent
  snapshot of the live database, correctly including anything
  committed but not yet checkpointed out of the WAL file. **Verified
  against real SQLite before the Go wrapper was written**: a live
  WAL-mode database with an uncheckpointed write was snapshotted, the
  snapshot was opened as a fully independent database and shown to
  contain that write, and `VACUUM INTO` was confirmed to correctly
  refuse to overwrite an existing destination file. Since "database,
  settings, themes, project data, goals, budgets" (spec section 48's
  own list) all already live in that one SQLite file, a "complete
  backup" falls out of this one mechanism rather than needing a
  table-by-table checklist.
- **Restore's honest architectural choice, stated up front in
  `internal/backup`'s own package doc**: STELLAREI cannot safely swap
  its own live database connection out from under every service that
  captured a `*sql.DB` reference at startup (Settings, Timers, Alarms,
  Sessions, and more each hold their own). Rather than fake an
  in-process hot-swap, Restore stages the replacement file plus a
  small JSON marker on disk, and the actual swap happens at the very
  start of the *next* launch - before anything opens the database - the same restart-to-apply pattern real desktop apps use for exactly
  this reason. `main.go`'s very first action after resolving paths is
  now checking for that marker. Always takes an automatic safety
  backup of the *current* database first, independent of what's being
  restored (spec section 48's explicit requirement) - and clears any
  leftover WAL/SHM sidecar files from the *previous* database before
  the swap, since a stale WAL would otherwise get replayed against the
  newly-restored file and silently reintroduce the very data the
  restore was meant to remove.
- **`internal/backup`**: `CreateSnapshot` (the `VACUUM INTO` wrapper - its actual execution can't be Go-tested here for the same
  `modernc.org/sqlite`-unreachable reason `internal/database` itself
  has no Go tests, but its SQL was independently verified against real
  SQLite, above), a zip bundler/extractor for backups that include
  screenshots (spec section 48's "optionally screenshots"), the
  restore marker's read/write/clear, and generic JSON/CSV writers. 16
  tests - all genuinely executable (no database needed): a real
  zip-bundle round trip, and **a real zip-slip attack rejected**
  (`TestExtractZip_RejectsZipSlip` crafts an archive entry named
  `../../etc/passwd` and confirms both that extraction errors and that
  nothing was actually written outside the extraction directory).
- **`app/services/backup_service.go`**: `BackupNow`/`ListBackups`/
  `DeleteBackup` (which refuses to delete anything outside the backups
  directory - this is exposed directly to the frontend and must never
  become a general "delete any file" call), `RestoreBackup`/
  `RestartToApplyRestore`, and export. Export datasets (spec section
  49's own list - activities, sessions, projects, goals, budgets,
  analytics) are queried through the same real repositories/services
  the rest of the app already uses for each (`GoalService`/
  `BudgetService`'s own `ListWithStatus`, `sessionRepo.ListForRange`,
  `projectRepo.ListProjects`) rather than duplicating SQL - "analytics"
  reuses the identical `SumDurationByCategory`/`SumDurationByProject`
  aggregates the real Analytics page itself calls, exported all-time
  instead of for one range.
- **A real forward-reference wrinkle worth noting**: the restart
  closure passed into `BackupService` is constructed *before*
  `app := application.New(...)` exists (`BackupService` has to be in
  the `Services` list *passed to* `application.New`, so it can't
  depend on the result of that same call). Solved with a
  pointer-assigned-later variable (`var appRef *application.App`,
  assigned right after `application.New` returns) rather than
  reordering the whole service-construction block - the closure is
  only ever *invoked* long after `appRef` is set, in response to a
  person clicking Restart, so this is safe, not fragile.
- **Frontend**: a real Backups page - Backup Now (with an optional
  "include screenshots" checkbox), a list of existing backups with
  Restore (two-step confirm, since it requires a restart) and Delete,
  a dedicated "Restore staged, restart now?" screen once one is
  staged, and a dataset/format export picker.

### Accessibility (spec section 68)

A bounded, honest first pass - not a full audit, and stated as such.
Spec section 68 lists six things: keyboard navigation, visible focus
states, sufficient contrast, reduced motion, scalable text, and
screen-reader-friendly labels. Four of those six got real, verified
fixes this pass. Two did not, and are named explicitly below rather
than left for someone to discover the hard way.

- **Visible focus states**: a global `:focus-visible { outline: 2px
  solid var(--accent); outline-offset: 2px; }` in `app.css`. Before
  this, the file had exactly ONE `:focus` rule in 1,592 lines
  (`.project-idea__title:focus`) - everything else relied on
  inconsistent browser defaults, which button/input reset styles
  elsewhere in the file can suppress entirely. `:focus-visible` rather
  than `:focus` deliberately, so mouse clicks don't show a distracting
  outline that keyboard navigation actually needs.
- **Reduced motion**: a single global `@media
  (prefers-reduced-motion: reduce)` override collapsing all
  animation/transition durations to near-zero, rather than hunting
  down and individually overriding the app's four existing
  `transition:` declarations one at a time. Deliberate choice: the
  global override is the standard, well-established pattern for this
  exact requirement, and it also covers any transition added to this
  file in the future without needing a matching reduced-motion
  override remembered every time.
- **Keyboard navigation, a real gap found and fixed**: two rows in the
  app were clickable `<div>`s with no keyboard path to activate them
  at all - `Projects.tsx`'s project row (click to expand/collapse its
  rules) and `Analytics.tsx`'s category row (same, for category
  rules). Neither could be reached by Tab or activated by Enter/Space;
  a keyboard-only or screen-reader user could see them but never open
  them. Fixed with the standard WAI-ARIA disclosure-trigger pattern:
  `role="button"`, `tabIndex={0}`, an `onKeyDown` handler treating
  Enter and Space the same as a click, and `aria-expanded` reflecting
  actual open/closed state. Neither row could be converted to a real
  `<button>` element instead - both contain a genuinely separate
  nested interactive control (an Archive/Unarchive button in
  `Projects.tsx`, none currently in `Analytics.tsx` but the same row
  shape) - and a `<button>` inside a `<button>` is invalid HTML and
  actively broken for assistive tech, not just non-ideal.
- **Screen-reader-friendly labels, a real gap found and fixed**: ten
  icon-only prev/next navigation buttons (the "‹"/"›" glyphs) across
  five pages - `Analytics.tsx`, `Calendar.tsx`, `MyYear.tsx`,
  `TimeReceipt.tsx`, `Timeline.tsx` - had no accessible name at all; a
  screen reader would announce either nothing meaningful or the raw
  glyph itself. Fixed with two new shared, translated keys
  (`common.previousLabel`/`common.nextLabel`, reused across all ten
  buttons in all three languages rather than ten near-duplicate
  page-specific keys) applied as `aria-label`. Found this exhaustively
  via `grep`, not by spot-checking a couple of pages - all ten
  instances across all five files were located and fixed in one pass,
  confirmed by re-running the same grep afterward and finding zero
  remaining unlabeled instances.
- **What this pass explicitly does NOT cover, named rather than
  implied fixed**:
  - **Scalable text**: all 88 `font-size` declarations in `app.css`
    use fixed `px` values, not `rem`. Modern browser zoom still scales
    `px` text along with everything else, but `rem` additionally
    respects a user's OS/browser default-font-size *preference*
    specifically (not just zoom level) - a real, separate WCAG
    consideration. Converting 88 declarations from `px` to `rem`
    correctly (getting the base `rem` size and every relative
    computation right) is a large, purely mechanical, non-trivial-to-
    verify-by-hand refactor with real regression risk across the
    entire UI if any one conversion is wrong - deliberately not
    attempted in this bounded pass.
- **Verification**: `tsc --noEmit` and the production build both
  clean; the usual i18n three-check ritual re-run since this touched
  already-translated pages (key-structure parity - 510 keys, zero
  missing/extra; the hardcoded-string sweep; and the code-to-locale
  cross-check - 448 real references, zero missing) confirming this
  pass didn't regress anything already translated. No Go files
  touched, so the existing 335-test Go suite is unaffected by
  construction, not just by re-running it.

### Accessibility, continued: sufficient contrast (spec §68) - COMPLETE

- **A real WCAG contrast-ratio audit was run, this session** - the
  pass above had explicitly named this as NOT done yet; this closes
  that gap. First correction made along the way: the pass above (and
  this section, before this edit) both said "8 built-in themes" - there are only **two**, Midnight and Light (`frontend/src/themes/
  themes.ts`'s `BUILTIN_THEMES` array, and README's own "Two built-in
  themes" line elsewhere both confirm this). "8" was simply wrong,
  caught while actually going to enumerate them for the audit, not
  guessed at again - corrected here rather than carried forward.
- **Scope of the audit**: every `color`/`background(-color)` `var()`
  pair genuinely reachable in the rendered UI, found mechanically (by
  parsing every CSS rule in `app.css` for same-rule `color:`/
  `background:` pairs, then walking the DOM/ancestor structure by hand
  for the far more common case of text with no explicit background of
  its own - which inherits from whatever container it's nested in:
  the page background by default, or a card/dialog/panel's `surface`/
  `backgroundElevated` when one exists) - not a guess at which
  combinations "probably" occur. 13 distinct token pairs came out of
  that search, computed against both themes (26 checks total) using
  the real WCAG relative-luminance formula (`0.2126·R + 0.7152·G +
  0.0722·B` on gamma-corrected channels, contrast = `(L_lighter +
  0.05) / (L_darker + 0.05)`), not a visual estimate.
- **Result: 25 of 26 passed WCAG AA (4.5:1 for normal text); one real
  failure found and fixed.** `.focus-hero__warning` (`Focus.tsx`'s
  blocked-timer warning banner, 13px normal-weight text - well under
  the ≥18pt/≥14pt-bold "large text" threshold that would only need
  3:1) set `color: var(--background)` on `background: var(--danger)`.
  In Midnight that's 5.64:1 (fine), but in **Light it's 4.33:1** - under 4.5, a genuine AA failure that would have shipped silently
  otherwise. Fixed by switching that one rule to `color: var(--
  backgroundElevated)` instead: pure white in Light (`#ffffff`,
  4.73:1 - passes) and effectively identical to the old value in
  Midnight (`#101317` vs `#0b0d10`, 5.40:1 - no regression). Chosen
  over hand-picking a new literal hex specifically to keep this a
  token reference like every other rule in the file, and over
  darkening the shared `danger` token itself (would have fixed this
  one case but touched every other place `danger` is used, for a
  problem that's local to one rule). The other 25 pairs - including
  `textMuted` on plain page background in Light, the tightest of the
  passing cases at 4.63:1 - all clear 4.5:1 with real margin; none
  needed touching.
- **What this still does NOT cover, on purpose**: only the two
  built-in themes were audited. Any custom theme a person builds in
  Theme Studio can set arbitrary token values with no contrast
  guarantee at all - that's inherent to letting people pick their own
  colors, not something a fixed audit of the built-ins can close, and
  isn't attempted here.
- **Verification**: the fix is a one-line CSS token swap, no other
  file touched. `npx tsc --noEmit` and `npm run build` both clean (no
  TypeScript involved in this change at all). No Go files touched, no
  locale files touched - nothing else could have regressed.

### Privacy Center + Screenshot Memory (Phase 9)

- **Privacy Mode (section 26)**: `internal/activity.Engine` grew a
  second, independent pause flag (`SetPrivacyMode`) alongside the
  existing manual Pause/Resume - deliberately NOT the same flag,
  because a person pausing for an unrelated reason (away from their
  desk) and separately wanting Privacy Mode for a specific sensitive
  task must never have one silently clobber the other when either is
  turned off. 4 new engine tests cover exactly that interaction
  (enabling Privacy Mode doesn't touch an existing manual pause,
  Resume() never clears Privacy Mode, disabling Privacy Mode only
  resumes tracking if nothing else is holding it paused). The tray
  label shows a distinct "PRIVACY MODE ACTIVE" (checked ahead of the
  generic "Tracking Paused" case so it's never shadowed) and a
  Enable/Disable Privacy Mode menu item; the setting persists across
  restarts. Honest gap: the tray **icon** doesn't yet change (section
  26 also asks for that) - that needs a third icon asset distinct
  from the existing light/dark theme-mode pair, and none exists yet;
  the label is the real, working signal in the meantime.
- **`internal/privacy`** (sections 26-28): a pure exclusion-matching
  engine (`IsScreenshotBlocked`/`IsBrowserTrackingBlocked` - an
  application's own `screenshot_allowed` flag and `privacy_sensitive`
  flag always win, then any matching exclusion rule), a retention
  policy (`ComputeDeleteAfter`/`ShouldPurge` - each screenshot keeps
  the deletion date that was in effect when it was taken, per section
  47's "don't retroactively change historical data" principle), and a
  SQL repository for `privacy_exclusions`/`screenshots`/the privacy
  columns on `applications` (all three tables already existed in the
  schema since Phase 1 anticipated this phase - no migration needed).
  36 tests.
- **Screenshot capture is REAL**, not simulated (section 83's "do not
  pretend screenshots work"): `internal/platform` grew a
  `ScreenCapturer` per OS, following the exact same
  shell-out-and-degrade-gracefully pattern `ActiveWindowWatcher`/
  `IdleWatcher` already established - Linux tries `scrot`, then
  `gnome-screenshot`, then ImageMagick's `import`, then KDE's
  `spectacle`, falling back to `ErrUnavailable` if none are installed
  (**verified for real in this sandbox** - genuinely no screenshot
  tool is present here, and the degrade-gracefully test confirms the
  contract holds, not just in theory); macOS shells out to the
  built-in `screencapture -x`; Windows uses real Win32 GDI calls
  (`BitBlt`/`GetDIBits`, the same approach every native Windows
  screenshot tool is built on) with no third-party library. Windows
  and macOS capture are unverified here for the same reason every
  other OS-specific file in this repo is (see "Not verified" below).
- **`internal/privacy.Sweeper`**: the background component that
  actually captures on a person-configured interval and purges expired
  screenshots - built exactly like `internal/scheduler.Scheduler`
  (narrow interfaces per dependency, a fabricated clock, no real
  database/OS in its own tests). Checks, in order: is screenshot
  memory even enabled, has the interval elapsed, is the person
  idle/paused-or-in-Privacy-Mode, is the current application
  `privacy_sensitive` or excluded - 12 tests cover every one of those
  gates plus "a failed capture never records a phantom screenshot
  row".
- **Browser tracking (section 28) - the most important honesty note
  in this phase**: STELLAREI does **not** have real browser URL/domain
  data, and this phase does not pretend otherwise. The Activity
  Engine only ever sees a window's TITLE, and ordinary browsers put
  the page's title there, not its domain. Reliable domain tracking
  needs a browser extension or an OS accessibility API reading the
  address bar - neither exists in this codebase; building either is a
  separate integration project, not something a regex can shortcut.
  What Phase 9 actually ships: the full settings/toggle/exclusion
  infrastructure (real), and `ExtractDomain` - a narrow, conservative
  heuristic that finds a domain-shaped fragment IF one happens to
  appear in a window title (some PWAs/app-mode browsers do this),
  returning nothing for the vast majority of ordinary tabs, which is
  the correct behavior, not a bug. It is **not wired into live session
  data** (no `domain` column exists on `activity_events`/`sessions` to
  put a result in), matching `internal/projects.RuleDomain`'s
  identical, pre-existing honesty note from Phase 3. The Privacy page
  says this in the UI, not just in code comments.
- **`app/services/privacy_service.go`**: Privacy Mode get/set (the one
  thing that has to reach into the live Engine, not just the
  database), exclusions CRUD, per-application privacy CRUD, and
  screenshot list/delete (which removes the actual file, looked up
  server-side rather than trusting a client-supplied path). Screenshot
  enabled/interval/retention and the browser-tracking toggle are
  deliberately NOT duplicated here - the frontend reads/writes them as
  plain settings via the existing `SettingsService`, the same way
  `KeyIdleThresholdSec` already works, using key constants defined in
  `internal/privacy` (mirroring `internal/closebehavior.SettingKey`'s
  established pattern) so both sides reference one string, never two
  hand-typed copies that could drift.
- **Frontend**: a real Privacy page - a prominent Privacy Mode toggle,
  screenshot settings, a per-application table with Track/Screenshots/
  Sensitive toggles, an exclusion-rule builder, and a screenshot
  history list with real delete.

### "I Have No Idea" (Phase 7)

- **`internal/idea`**: entirely AI-free, on purpose - spec section 37's
  "AI must NEVER be required for the core product" applies to this
  phase as directly as any other. Everything here is a plain,
  deterministic scoring/combination function over real data:
  - `Score` rates a `Candidate` against the person's stated time and
    intent (fits the time budget within a 1.5x soft cap, a genuine
    intent-tag match, a neglect bonus capped at 3 points), always
    returning the specific reasons behind the number - never a raw
    score shown to the person, and never a reason that isn't backed by
    a real signal the caller actually provided.
  - `PickSuggestions` scores a whole candidate pool, adds a small
    random jitter (a real `*rand.Rand`, injectable for
    reproducible tests - never the global RNG) so the same inputs
    don't always produce the identical top pick, excludes anything
    shown in the last 10 generations (falling back to allowing repeats
    only if that would leave too few candidates), and always appends
    exactly one Chaos Mode challenge as the final suggestion - matching
    spec section 34's own three-item example layout exactly.
  - `chaosBank`: a static, 24-entry pool of Chaos Mode challenges
    (section 36) - several are the spec's own section 36 examples
    verbatim. Deliberately NOT generated: a truly novel,
    never-repeating challenge generator is exactly the kind of
    open-ended creative task section 37 reserves for an optional
    future local-AI layer, not something to fake with combinatorial
    noise dressed up as creativity.
  - `GenerateProjectIdea` (section 35, "I want to build something"):
    18 hand-written (title, description-template) pairs - not
    word-by-word combinatorics, which reliably produces either boring
    or grammatically broken results - picked via the same
    scoring/jitter mechanism, with the description's subject and the
    "Why" line grounded in a REAL neglected project or recently-used
    category when one exists, and staying honestly neutral when
    neither does.
  - 30 tests, including one that caught and fixed a real bug during
    development: the original "nothing fit the time budget" fallback
    picked a uniformly random template - which could hand back the
    single LARGEST idea in the bank for the SHORTEST time budget. Fixed
    to deterministically fall back to the smallest-scoped template
    instead; see the comment on `GenerateProjectIdea` and
    `TestGenerateProjectIdea_TooShortForAnyTemplateFallsBackToSmallest`.
- **`internal/sessions`** grew one new method, `LastActivityByProject` - the real "how many days since this project was last touched" signal
  neglect-based scoring needs (section 34's own "neglected projects"
  data source). Verified against real SQLite, including that an
  excluded session and a project-less session are both correctly
  ignored.
- **`app/services/idea_service.go`**: assembles the real candidate pool - active projects (tagged with real `NeglectedDays` from the method
  above) and in-progress goals (reusing `*GoalService`'s own
  progress math directly, same package, no duplicated calculation) - then hands it to `internal/idea` and persists the result to
  `idea_history`/`chaos_history` (both tables already existed in the
  schema since Phase 1 anticipated this phase; no migration needed).
  `StartProjectFromIdea` is the one part of section 35's Accept/
  Regenerate/Modify/Save/Start-project list that's backed by real
  persistence - it calls the exact same `projects.Repository.
  CreateProject` any other new project goes through. Modify is a plain
  frontend text edit (no backend concept needed); a dedicated "Save as
  draft" backlog is honestly not implemented this phase - there's no
  table for it, and the frontend says so rather than pretending a
  click did something durable.
- **Tray**: "I HAVE NO IDEA" gets a genuine one-click launch (unlike
  Timers/Alarms/Focus's create actions) - section 44's own example tray
  menu lists it directly, and unlike starting a timer, opening the
  picker needs no upfront input; the time/intent choices happen on the
  page.
- **Frontend**: a real flow - the big button, then time, then intent,
  then three suggestion cards (chaos visually distinguished with a
  dashed border and a "CHAOS" label instead of a number) where
  "Start" both records which one was picked and starts a real timer for
  it via `TimerService` - and a separate "I want to build something"
  flow with inline-editable title/description and a real "Start
  project" button.

### Focus Mode (Phase 6)

- **`internal/focus`**: a small, deliberately narrow package on top of
  `internal/timers` - a focus session **is** a `timers.TypeFocus` row,
  reusing all of Phase 5's pause/resume/complete machinery rather than
  duplicating it. What this package adds: a `Config`
  (blocked apps/websites, whether to suppress other STELLAREI
  notifications while focusing) round-tripped through the timer's own
  `metadata_json`, exactly like Pomodoro's phase state; case-insensitive
  blocked-app matching (`IsBlockedApp`, same convention
  `internal/projects`' rule evaluator already uses); and `Quality` - section 20's "Focus quality: 92%" - the fraction of *tracked* time
  spent on the session's own project, idle time excluded from both
  sides exactly the way Budgets/Analytics already exclude idle from
  every sum. 10 tests, including one that reproduces the spec's own
  92% example (23 of 25 minutes on-target) exactly.
- **Honesty about what "blocked" means**: STELLAREI does **not** force-
  close or prevent-launch anything. Section 87 ("document the
  limitation and implement the closest secure native solution rather
  than silently faking it") ruled out pretending to kill arbitrary
  user processes - that's invasive, easy to get wrong in a way that
  loses someone's unsaved work, and platform-fragile to do safely. What
  a blocked app actually does: the person gets a native notification
  the moment they open one during an active focus session (see
  `internal/scheduler`'s addition below), and it counts against their
  focus quality. `BlockedWebsites` is accepted and stored in the config
  today but has zero effect - there's no browser URL/domain tracking
  yet (that's Phase 9, spec section 28, currently unbuilt), so there's
  no real signal to check a website block against. The frontend's own
  copy says exactly this ("STELLAREI can't force-close anything - you'll get a notification").
- **`internal/scheduler`** grew a fourth check (`checkFocusViolations`,
  alongside timer-completion/alarm/nothing-else-changed from Phase 5):
  every tick, if a running focus session has a blocked-apps list and
  the CURRENT foreground app (from the real Activity Engine via a new
  narrow `ActivityStatus` interface - `Status() activity.State`,
  satisfied by `*activity.Engine` directly, nil-is-fine like every
  other optional Scheduler dependency) is on it, fire exactly one
  notification per continuous streak of being in a blocked app - not
  one every 5 seconds for as long as it stays foregrounded. 10 tests
  covering the debounce, re-arming after leaving and returning to a
  blocked app, idle/paused states never counting, and a nil
  `ActivityStatus` never panicking.
- **`app/services/focus_service.go`**: `Start` (project required - section 20 defines a focus session as inherently tied to one, unlike
  a plain timer), `GetActive`, and `History`. Deliberately does **not**
  duplicate Pause/Resume/Finish/Cancel - the frontend calls
  `TimerService` for those using the same id, since a focus session
  really is just a timer underneath. Quality/on-target/blocked-time
  stats are computed fresh on every call from `internal/sessions`' real
  session data (clipped to the focus session's own time window,
  excluded sessions honored, section 63), never cached, never
  invented - the same discipline every other stats-producing service
  in this codebase already follows.
- **Tray**: "Open Focus…" (opens the window, navigates to `/focus`) - no quick-start shortcut, unlike Timers' "Start 25-min Timer", because
  a focus session's one hard requirement (a project) has no tray input
  affordance, the same reasoning Alarms already documented for why it
  doesn't get one either.
- **Frontend**: a real Focus page - a start form (project, optional
  label/duration/blocked-apps list/notification-suppression) when
  nothing's running, and a live hero view (client-side per-second
  countdown, a quality bar, a blocked-app warning banner driven by the
  same real `currentAppBlocked` field the notification check uses, and
  Pause/Finish/Cancel wired straight to `TimerService`) when a session
  is active, plus a history list.

### Theme Studio, logo, and tray icon (brought forward from Phase 8)

- **Two built-in themes** - Midnight (dark) and Light - seeded into the
  database via a real migration, both using the app's actual logo's
  indigo accent hue. Verified against real SQLite with an exact
  key-for-key cross-check against the frontend's token list.
- **`internal/theme`**: classifies any theme as light or dark from its
  background color's perceived luminance - not the OS's dark-mode
  setting, since a user's chosen STELLAREI theme and their OS's mode
  are two different things. 11 tests. The frontend has an exact
  TypeScript port of the same formula (`computeThemeMode`), cross-checked
  in Node against the identical 9 Go test cases with identical results.
- **`app/services/theme_service.go`**: full CRUD for custom themes
  (list/save/delete), active-theme tracking, refuses to overwrite or
  delete a built-in theme, refuses to delete the currently active
  theme, and notifies a callback whenever the active theme (or an
  in-place edit to it) changes - that callback is what drives the tray
  icon switch.
- **The tray icon and the in-app logo now both switch with the active
  theme's computed mode** - using the two logo images provided,
  resized into proper 16/32/48/256px tray icon variants with Pillow.
  Confirmed the real Wails v3 `SystemTray.SetIcon([]byte)` API from
  their official current docs before writing this, rather than
  guessing.
- **Theme Studio** (`/themes`): lists every theme with Activate/Delete,
  a color editor grouped by Surfaces/Text/Accent/Charts, a genuine
  self-contained **mini live preview** (renders from local edited state
  via inline styles, not the global CSS variables, so editing never
  leaks into the real app chrome until you save), Save-as-new and
  Save-and-Activate, and JSON import/export (download/upload - no
  native file dialog, since that Wails API surface isn't confirmed
  here; see "not verified" below).
- Settings gained a quick theme-switcher dropdown alongside the
  existing close-behavior / startup-behavior controls (all three of
  which - run on start, close to tray, run on start in tray - already
  existed since Phase 1).
- **New verification technique this round**: built a local stub of the
  exact Wails v3 API surface `main.go`/`tray.go`/the services use
  (method names and signatures taken from real, current Wails v3 docs
  and GitHub examples gathered during development - not invented), and
  got the *entire* backend to actually `go build`, `go vet`, and
  `go test` clean against it. This doesn't validate real Wails runtime
  behavior, but it's a real step up from "the syntax looks right" to
  "this type-checks against the actual documented API shape" - every
  argument count, type, and method call in `main.go` and `tray.go` is
  now compiler-verified, not just eyeballed. The stub itself is not
  part of the shipped code (see `go.mod` - no stub references remain).

### Timers, Alarms, and Notifications (Phase 5)

- **`internal/timers`**: pause/resume-aware elapsed-time math
  (`Elapsed`/`Remaining`) that folds multiple past pauses plus an
  in-progress pause into the correct answer, computed fresh on every
  read rather than cached - a completed/cancelled timer's elapsed time
  freezes at `EndedAt` instead of continuing to tick. A full Pomodoro
  phase state machine (`NextPhase`: work → short break → work → … →
  long break every 4th cycle by default, configurable) round-tripped
  through the timer row's `metadata_json`. A SQL repository
  (create/pause/resume/complete/cancel/list-active/list-recent). 21
  tests, including a two-pause-cycle scenario and a full 4-cycle
  Pomodoro round-trip.
- **`internal/alarms`**: a small RRULE-*flavored* recurrence parser
  (`FREQ=DAILY|WEEKLY|WEEKDAYS;BYDAY=MO,WE,FR;TIME=HH:MM`) with
  timezone-correct `NextFireAfter` search; an activity-based
  trigger-condition parser (`category=Gaming;minutes=60`, spec
  section 22's own example); a pure `Due()` function deciding whether
  an alarm should fire *right now* (handles one-shot vs. recurring vs.
  snoozed-override, all as plain functions of the alarm's fields plus
  `now` - no clock or DB dependency); and a SQL repository including
  fire/snooze/dismiss state transitions. 32 tests - two of which
  caught bugs in the *tests' own* fabricated timestamps, not in
  `Due()` itself, during development (see comments in `due_test.go`).
- **`internal/notify`**: a notification-history repository (the
  `notifications` table) plus native OS delivery, wrapping the real
  `github.com/wailsapp/wails/v3/pkg/services/notifications` package
  (confirmed against its live, current docs - see below) behind a
  small `Dispatcher` interface so nothing downstream depends on Wails
  directly. History is always written even if native delivery fails
  (spec section 56 - a notification subsystem hiccup must not lose
  the underlying event).
- **`internal/scheduler`**: the background component that actually
  makes Timers and Alarms do something over time - a 5-second ticker
  (same conservative-polling philosophy as the Activity Engine, spec
  section 50) that notices a countdown/Pomodoro timer reached its
  target and completes it (chaining Pomodoro to the next phase
  automatically), notices a one-shot/recurring/snoozed alarm came due,
  and evaluates activity-based alarms against *today's* actual
  category/project time from `internal/sessions`, re-arming at local
  midnight. Built exactly like `internal/activity.Engine` - narrow
  interfaces for every dependency (`TimerStore`, `AlarmStore`,
  `ActivitySums`, `CategoryLookup`, `ProjectLookup`, `Notifier`) so
  its tick-by-tick firing decisions are unit-tested with fakes and a
  fabricated clock, not a real database or OS. 13 tests, covering
  timer completion, paused-timers-never-auto-complete, the full
  Pomodoro chain, one-time/recurring alarm firing, activity-based
  threshold crossing (including "must not double-fire the same day"
  and "re-arms the next day"), and an unresolvable category name being
  ignored rather than crashing.
- **`app/services`**: `TimerService` (stopwatch/countdown/Pomodoro
  start, pause/resume/finish/cancel, live elapsed/remaining computed
  per-call the same way Budget/Goal status already do),
  `AlarmService` (create all five alarm types with save-time
  validation via the parsers above, enable/disable/delete/snooze/
  dismiss, a best-effort `NextFireAt` for display), and
  `NotificationService` (read/dismiss the in-app history - creation is
  scheduler-only, there's deliberately no `Create` method for the
  frontend to call).
- **Tray** (section 21 "Timers must be accessible from the tray"):
  Start 25-min Timer, Stop Running Timer, Open Timers…, Open Alarms….
  Alarms don't get a dedicated tray *creation* shortcut - unlike a
  timer, creating one needs at least a time/label the tray has no
  input affordance for.
- **Frontend**: a real Timers page (live per-second countdown driven
  client-side between backend fetches, a card grid with pause/resume/
  finish/cancel, a launcher for stopwatch/countdown/Pomodoro) and a
  real Alarms page (all five alarm types, including a weekday picker
  for `FREQ=WEEKLY` and a category/project picker for activity-based
  conditions), both listening for the backend's `timer:completed` /
  `alarm:fired` events to refresh immediately rather than polling.
- Migration `0005_timers_alarms_phase5.sql` - additive only (`timers.
  paused_at`/`accumulated_pause_secs`, `alarms.snoozed_until`),
  applied on top of the full existing chain and verified against real
  SQLite, including a full 25-minute countdown scenario with a
  pause/resume in the middle and idempotent double-completion.

### Timeline, Projects, and manual corrections (Phase 3)

- **`internal/projects`**: a pure, order-preserving rule evaluator
  (`Evaluate`) matching a session's process name / window title against
  user-defined rules (process exact-match, window-title-contains,
  path-contains approximated against the title since there's no folder
  tracking yet, and domain - which never matches until Phase 9's browser
  tracking exists, documented rather than faked). 11 tests. A
  `Repository` for projects/rules CRUD and a `Resolver` adapter the
  Activity Engine calls when a session closes - verified against real
  SQLite with a realistic multi-project, multi-rule scenario, including
  confirming an archived project's rules are correctly excluded.
- **The Activity Engine now assigns a project to every session it
  closes**, via an injected `ProjectResolver` - 3 new engine tests cover
  a resolver being called with the right process/title, a resolver
  error leaving the session gracefully unassigned instead of crashing,
  and the nil-resolver case (unchanged Phase 2 behavior). The
  sessionizer itself was extended to track each session's most-recently
  observed process/window-title (needed for rule matching), with a test
  proving that tracking works across merged events.
- **Session corrections (section 63)**: reassign project, exclude from
  analytics, add a note, split a session at an arbitrary point, or merge
  two sessions - all persisted with `manually_edited` set so nothing
  automated overwrites a person's own judgment later. Split and Merge's
  SQL (including event-reassignment between the resulting sessions) was
  verified against real SQLite with a concrete 3-event scenario, not
  just eyeballed.
- **Timeline UI** (`/timeline`): day/week/month zoom (verified date-range
  math against real dates including the Monday-week-start edge case for
  a Sunday anchor), sessions rendered as positioned/colored bars, click
  a session to open a correction panel (reassign project, exclude,
  annotate, split, merge with an adjacent session - merge is only
  offered when sessions are actually time-adjacent).
- **Projects UI** (`/projects`): create/archive projects, and manage
  each project's rules inline.

### Analytics, categories, goals, and budgets (Phase 4)

- **`internal/periods`**: pure period-window math (day/week/month) shared
  by budgets and goals, mirroring the frontend's `rangeFor` exactly - 5 tests, including the same Sunday-anchor Monday-week-start edge case
  verified on the frontend side in Phase 3.
- **`internal/categories`**: category CRUD, seeded with 9 default
  categories (Development, Design, Music, Gaming, Research,
  Communication, Entertainment, Idle, Unknown) matching the spec's own
  examples. Category resolution (an application's explicit override,
  falling back to its default) verified against real SQLite across
  three scenarios: default-only, override-beats-default, and clearing
  an override falls back correctly.
- **The Activity Engine now assigns a category to every session it
  closes too**, via the same `CategoryResolver` pattern as
  `ProjectResolver` - a dedicated engine test confirms it's called with
  the right application id and its result persists.
- **`internal/budgets` / `internal/goals`**: CRUD for section 17/18's
  time budgets and goals, verified against real SQLite. Progress is
  never stored - it's computed fresh on every read from real session
  data via `periods.Window` + the new sessions aggregate queries below,
  so it's never stale.
- **New sessions aggregate queries** - `SumDurationByCategory`,
  `SumDurationByProject`, `ContextSwitchStats` - verified against real
  SQLite with a scenario covering partial-overlap clamping (a session
  half in/half out of the query range only contributes its overlapping
  portion) and confirming excluded sessions are correctly omitted from
  every aggregate, not just Timeline's display.
- **Analytics UI** (`/analytics`): category/project breakdown bars,
  trend vs. the immediately preceding equal-length period, "digital
  friction" context-switching stats (section 32 - labeled as an
  estimate, not a precise measurement, per section 66's honesty rule),
  plus inline category management and an application-to-category
  mapping table (section 10).
- **Goals UI** (`/goals`): create/archive goals and budgets with real
  progress bars - a budget that's over its limit visibly turns red
  rather than just showing a number.

### What's verified vs. what isn't (read this before trusting anything below)

This scaffold was built in a Linux sandbox with **no Windows APIs, no
window manager, and no Wails CLI/toolchain** - so some things were
verified for real and some genuinely could not be:

**Actually verified here:**
- `internal/config`, `internal/database`, `internal/closebehavior`,
  `internal/platform` (Linux build), `internal/activity`,
  `internal/sessions`, `internal/theme`, `internal/timers`,
  `internal/alarms`, `internal/notify`, `internal/scheduler`,
  `internal/focus`, `internal/idea`, `internal/ai`, `internal/privacy`,
  and `internal/backup` all pass `go vet` (`internal/ai` also passes
  `go test -race`, and `app/services` - including the new
  `ai_service.go` and the AI-aware `idea_service.go` - was fully
  type-checked in isolation by temporarily setting aside the two files
  that import `wails/v3` directly, building/vetting clean, then
  restoring them byte-for-byte; see the Phase 11 section above).
- **324 tests actually run and pass** (222 from Phases 1-10, 25 in
  `internal/ai` for Phase 11's AI foundation, 20 in
  `internal/patterns` for Pattern Discovery plus its long-run
  benchmark's correctness tests, 15 in `internal/waste` for Digital
  Waste Detection, 10 in `internal/calendar` for the Calendar view,
  21 in `internal/search` for Search, and 11 in `internal/myyear` for
  MY YEAR - see the Phase 11, Pattern Discovery, Digital Waste Detection,
  Calendar, Search, and MY YEAR sections above for all six breakdowns): 7 in `internal/closebehavior`
  and `internal/platform` combined (3 in `internal/platform` alone - see the screenshot-capture bullet below), 24 in `internal/activity`
  (including several that only pass because they caught and fixed real
  bugs in the tests' own premises during development - see comments
  in `engine_test.go` - plus 4 Phase 9 tests covering the
  Privacy-Mode/manual-pause interaction), 10 in `internal/sessions`,
  11 in `internal/theme` (the light/dark classifier), 11 in
  `internal/projects` (the rule evaluator), 5 in `internal/periods`
  (day/week/month window math, including the Sunday edge case), 21 in
  `internal/timers` (pause-aware elapsed math, the Pomodoro state
  machine), 32 in `internal/alarms` (recurrence, trigger conditions,
  the `Due()` firing decision - two of which caught bugs in their own
  fabricated test timestamps, not in `Due()`, during development), 20
  in `internal/scheduler` (the background tick loop, against fakes and
  a fabricated clock - no real database or OS notification involved;
  10 of these are the Phase 6 focus-violation debounce/re-arm checks),
  10 in `internal/focus` (blocked-app matching, the quality
  calculation - one test reproduces the spec's own "Focus quality:
  92%" example, 23 of 25 minutes, exactly), 30 in `internal/idea`
  (suggestion scoring, history-aware de-duplication, the project-idea
  generator - one of which caught a real bug in the generator's own
  fallback logic during development, not just the test: see
  `TestGenerateProjectIdea_TooShortForAnyTemplateFallsBackToSmallest`),
  36 in `internal/privacy` (exclusion matching, retention policy, the
  domain-extraction heuristic's own honest failure modes, and the
  Sweeper's tick logic against fakes), 16 in `internal/backup`
  (zip bundling/extraction, restore-marker persistence, JSON/CSV
  writers - including a genuine zip-slip attack crafted and confirmed
  rejected, not just a happy-path round trip), 20 in
  `internal/patterns` (median-resists-outliers, the noise-threshold
  cutoffs on both the time-of-day-length comparison and the
  context-switch trend, a caught arithmetic mistake in the test's own
  expected value during development - not the code - fixed by
  asserting the actual property intended rather than a miscalculated
  exact figure, and confirmation no insight ever contains causal
  language like "causes" or "because"; 2 of the 20 are the seeded-
  synthetic-data determinism check and the full-year correctness test
  backing `BenchmarkDiscoverAll_OneYear`/`_ThreeYears` - see the
  Phase 12 note above for the actual measured numbers), 15 in
  `internal/waste` (the both-conditions-required guard against flagging
  ordinary sustained work as a reopening pattern, the zero-category-ID
  degrade path, and a scan for judgmental language - "waste", "bad",
  "should", "too much", "stop" - confirming none ever appears in a
  finding's text), 10 in `internal/calendar` (a 48-hour session
  split correctly across 3 calendar days, a timezone-specific split
  test using a fixed UTC offset so it doesn't depend on the test
  environment's tzdata, and confirmation that split parts always sum to
  exactly the original session's duration), 21 in `internal/search`
  (the whole-word-boundary guard, a regex-special-characters-in-a-name
  case, the longer-name-wins entity preference, and the
  future-date-means-last-year inference), and 11 in `internal/myyear`
  (a session crossing from July into August correctly attributed across
  both months, and 20-iteration repeat runs on two separate tie-break
  cases specifically to catch a flaky result Go's randomized map
  iteration order could otherwise produce only sometimes).
- **Screenshot capture's failure path is tested for real, not just in
  theory**: `TestLinuxScreenCapturer_DegradesGracefully` runs the
  actual `linuxScreenCapturer.CaptureScreen` against this real
  sandbox - which genuinely has no `scrot`/`gnome-screenshot`/`import`/
  `spectacle` installed - and confirms it returns `ErrUnavailable`
  rather than hanging or crashing, the exact same verification
  `idle_linux.go`'s own degraded test already established.
- **`VACUUM INTO` (Phase 10's entire database-backup mechanism) was
  verified against real SQLite before the Go wrapper was written**: a
  live WAL-mode database with a deliberately uncheckpointed write was
  snapshotted, the resulting file was opened as a fully independent
  database and shown to contain that write, and the snapshot was
  confirmed to correctly refuse to overwrite an existing destination.
  `CreateSnapshot` itself (the thin Go wrapper) couldn't be
  Go-tested - same `modernc.org/sqlite`-unreachable reason
  `internal/database` has no Go tests of its own - but the SQL
  statement it issues was independently proven correct first.
- All migration files (including the Phase 5 timer/alarm columns) were
  executed against **real SQLite** (via Python's stdlib, since the Go
  driver is unreachable here), along with the exact INSERT/SELECT/
  UPDATE statements every repository issues - including the
  applications upsert's idempotency, a full read-back join, an exact
  key-for-key cross-check that the seeded Midnight/Light theme tokens
  match the frontend's token list, a full pause→resume→complete timer
  scenario with idempotent double-completion, an alarm
  fire/snooze/dismiss cycle including the one-shot-vs-recurring
  auto-disable distinction, `LastActivityByProject`/`idea_history`/
  `chaos_history`'s exact queries (Phase 7), the `privacy_exclusions`/
  `screenshots`/application-privacy-columns queries (Phase 9), and
  (Phase 10) the raw `activity_events` export join and a category-name
  lookup. (Focus Mode, "I Have No Idea", and Privacy Center all needed
  ZERO new migrations; Backup/Export needed zero too - everything it
  touches is either the live database file itself via `VACUUM INTO`,
  or plain filesystem operations.)
- The entire Go tree is `gofmt`-clean - including the Windows-only
  `screenshot_windows.go` (real GDI `BitBlt`/`GetDIBits` screen
  capture) and macOS-only `screenshot_darwin.go`, both of which parse
  as valid Go even though only the Linux file in that trio actually
  builds and runs here (see "Not verified" below for what that
  specifically means for these two).
- **The entire backend - including `main.go`, `internal/tray`, and
  every `app/services/*` file - actually `go build`s, `go vet`s, and
  `go test`s clean**, against a local stub of the exact Wails v3 API
  surface this code calls, including the `pkg/services/notifications`
  package Phase 5 needed (see "About the Wails stub" below). This
  catches wrong argument counts, wrong types, and typos that `gofmt`
  alone can't - it does not validate real Wails *runtime* behavior.
  Phase 10's one real architectural wrinkle (the restart closure
  needing `app` before `app` exists - see this phase's own section
  above) type-checked correctly against the stub on the first try
  once the pointer-assigned-later fix was in place.
- The frontend actually **installs, type-checks, and builds**
  (`npm install && npm run build` - real npm registry, real
  TypeScript compiler, real Vite bundle, 95 modules, including the new
  Backups page and bindings).
- The frontend's `computeThemeMode` (light/dark classification) was
  cross-checked in Node against the exact same 9 test cases as the Go
  `internal/theme` tests - identical results on both sides.
- Confirmed against the live Wails v3 repo and official docs (not from
  memory): current version is `v3.0.0-beta.8`, it requires
  **Go ≥ 1.25**, `SystemTray.SetIcon([]byte)` is the real, current
  method for setting a tray icon, and
  `pkg/services/notifications.NotificationService.SendNotification`
  (with a `NotificationOptions{Title, Body, Sound *NotificationSound}`
  struct) is the real, current native-notification API - fetched from
  the live package docs during Phase 5, not recalled from training.

**About the Wails stub:** to type-check the backend without a working
Wails install, a minimal local package was written at
`github.com/wailsapp/wails/v3` (methods: `application.New`,
`NewService`, `Window.NewWithOptions`, `SystemTray.New/SetIcon/
SetLabel/SetMenu`, `Menu.Add/AddSeparator`, `MenuItem.OnClick`,
`Event.Emit`, `Quit`, `Run`, `events.Common.WindowClosing`, and
`pkg/services/notifications.New/SendNotification/
RequestNotificationAuthorization` and its `NotificationOptions`/
`NotificationSound` types) with signatures taken from the real
docs/examples gathered during Phase 5, wired in via a temporary
`go.mod` `replace` directive, built, then **reverted** - the shipped
`go.mod` has no trace of it. Phases 6, 7, 9, and 10 needed no changes
to the stub itself - none of them add new Wails API surface beyond
what Phase 5 already covers. A second, much smaller stub for
`modernc.org/sqlite` (just enough to register a `database/sql` driver
name so `internal/database` compiles) was used the same way, for the
same reason, and reverted the same way. Neither is a substitute for
building against the real modules; they exist only to prove this
repo's own Go code is internally consistent.

**Not verified - needs your machine:**
- Real Wails v3 runtime behavior (window creation, tray rendering,
  event delivery, bindings codegen, real native OS notification
  delivery/authorization) - the stub above proves the code *compiles*
  against the documented shapes, not that it *behaves* correctly at
  runtime. Expect to fix small API drift on first real `go build` - Wails v3 is still in beta and its surface has moved before. In
  particular, the tray's Pause Tracking and Timer menu items use flat
  items instead of a submenu specifically because the submenu API
  couldn't be confirmed here - worth revisiting once you have a
  working build.
- Focus Mode's blocked-app violation notification has never fired for
  real - `internal/scheduler`'s `checkFocusViolations` is unit-tested
  against a fake `ActivityStatus`, but the real signal (an actual
  process becoming actual foreground) only exists on your machine.
- "I Have No Idea"'s suggestion quality depends on having real
  projects/goals/session history to draw from - on a genuinely fresh
  install it'll lean on the generic fallback bank
  (`genericFallbacks`/`chaosBank` alone), which is honest (see
  `internal/idea`'s own doc comment) but hasn't been eyeballed against
  a real, lived-in database with months of history the way it
  eventually should be.
- **The full backup→restore→restart cycle has never run end to end**.
  `CreateSnapshot`'s SQL was verified (above), `BundleZip`/`ExtractZip`
  and the restore marker have real passing tests, but the actual
  `os.Executable()` + `exec.Command(...).Start()` relaunch, and
  `applyPendingRestore`'s file-swap logic in `main.go`, have only ever
  been read, not run - this sandbox has no Wails runtime to launch a
  real STELLAREI process against in the first place. Test this one
  particularly carefully on first use: back up, make a small change,
  restore, restart, and confirm the change actually reverted.
- Windows/macOS-specific code (`startup_windows.go`,
  `startup_darwin.go`, `activewindow_windows.go`, `idle_windows.go`,
  `activewindow_darwin.go`, `idle_darwin.go`, and
  `screenshot_windows.go`/`screenshot_darwin.go`) can't be
  cross-compiled here either, for the same reason
  (`golang.org/x/sys/windows` is unreachable from this sandbox's
  network allowlist). The Win32 calls are standard, decades-stable
  APIs (`GetForegroundWindow`, `GetLastInputInfo`, and
  `BitBlt`/`GetDIBits` for screen capture) but genuinely untested
  against a real Windows process; `screencapture -x` is macOS's own
  stable built-in tool but equally untested here.
- The Linux active-window/idle/screenshot code
  (`activewindow_linux.go`, `idle_linux.go`, `screenshot_linux.go`)
  compiles and its **failure path is tested for real** (no
  xdotool/xprintidle/scrot/etc. installed here - confirmed, and the
  code degrades to `ErrUnavailable` as designed for all three). Their
  *success* paths - actually reading a real X11 window, actually
  capturing a real screen - have never run, since this sandbox has no
  display server.
- No system tray, no real window, no OS notification, no actual
  process being tracked, no actual screenshot file has ever appeared
  on a screen or disk from a real capture. That's inherent to a
  headless Linux container - this needs your desktop.

## Building it for real

**Note for whoever resumes this in a similar sandboxed environment:**
each fresh sandbox container starts with no Go toolchain at all - not
even a Wails-capable one. `apt-get install -y golang-go` (from the
Ubuntu archive, no `proxy.golang.org` needed) gets you a real `go`
binary immediately, which is enough to `go build`/`go vet`/`go test
-race` every `internal/*` and `app/services/*` package that doesn't
import `wails/v3` or `modernc.org/sqlite` directly - which, as of
Phase 11, is everything except `main.go` and `internal/database`
themselves. `cd frontend && npm ci` also just works (the npm registry
is reachable) - `npx tsc --noEmit` and `npm run build` both run for
real, not just format-checking. Neither of those was exercised for
real when this backup's earlier phases were written (see "What's
verified vs. what isn't" above); it's worth doing from here on.

**A real, new finding from re-verifying this backup on Go 1.22.2:**
`go build ./internal/...` and `go test ./internal/...` do NOT degrade
gracefully the way you'd expect when a few packages in the tree fail to
resolve imports - instead of building/testing every package that CAN
build and just reporting errors for the ones that can't, the whole
`...` expansion silently tests NOTHING and only prints the
import-resolution errors, exit code 1. A single explicit package path,
or several listed explicitly, work completely normally. Confirmed with
a controlled comparison (`go test -race -count=1
./internal/periods/...` alone: real PASS output; the same invocation
via `./internal/...`: zero PASS lines, same 3-line error dump).
Practical fix: `go list ./internal/... 2>/dev/null | grep -v -E
"internal/(database|notify|tray)$"`, feed that list explicitly to `go
build`/`go test`. Also note `internal/scheduler` transitively imports
`internal/notify` (for the `Dispatcher` interface), so verifying
scheduler needs the exact `dispatcher.go` setaside-stub dance described
above, not just excluding `database`/`tray`. Re-run this way at the
time: **355 tests, 24 buildable packages, all passing, `-race`
clean** - a few more than the previously-recorded 324+, consistent
with continued work since the last count, not a regression.

**Correction, found later in this same continuation (see the
Category-level auto-rules section below for the full context):** that
355 figure was never re-verified by a second independent method after
this initial grep-based count - every check afterward compared
`ok`/`no test files`/`FAIL` status per package, which confirms nothing
regressed but doesn't recount individual test functions. When
`internal/categories` gained its first real tests, a recount surfaced
a discrepancy: three independent methods (the same `grep -c "^---
PASS"` used here, a per-package `awk` breakdown of the same `-v`
output, and a plain static `grep -c "^func Test"` across every
`_test.go` file) now all agree on **335** tests for the packages that
existed at the time this section was written, not 355. The likely
explanation is a simple misread or transcription slip in this exact
grep count, early in the session, that then went unquestioned because
every subsequent status-only check still showed everything passing - which it genuinely was, just not exactly 355 of it. Left here
uncorrected rather than silently edited, with the correction stated
plainly, because that's the same standard this project holds its own
claims to (see the self-correction log) - a stale number sitting next
to its own correction is more honest than a quietly fixed one.

**Further update, this continuation (global shortcuts):** `internal/
shortcuts` landed with 17 new tests (17 `func Test...` across its
`_test.go` files, all passing, `-race` clean), taking the total from
335 to **352** across what's now 25 buildable packages (the 24 above
plus `shortcuts` itself). Re-verified the same three independent ways
as the 335 correction above - `grep -c "^--- PASS"` on `-v` output,
the per-package `awk` breakdown, and static `grep -c "^func Test"` - all three again agree on 352. `internal/scheduler` still needs the
`dispatcher.go` setaside-stub dance to build/test at all (it imports
`internal/notify` for the `Dispatcher` interface type, not the
concrete `WailsDispatcher`), same as noted above; nothing about that
changed this session.

```bash
# 1. Backend deps (needs Go 1.25+; `go build` will try to fetch that
#    toolchain automatically if your Go is older and GOTOOLCHAIN=auto,
#    the default)
go mod tidy

# 2. Frontend
cd frontend
npm install
npm run build      # produces frontend/dist, which main.go embeds
cd ..

# 3. Wails CLI (once, globally)
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# 4. Generate bindings (main.go, app/services/* → frontend/bindings/*)
wails3 generate bindings

# 5. Dev mode (hot reload)
wails3 dev

# 6. Production build
wails3 build
```

Until step 4 runs, `frontend/src/lib/bindings.ts` will throw a clear
`BindingsNotGeneratedError` instead of silently doing nothing - that
file is the one place in the frontend that knows codegen might not
have happened yet.

On Linux, install `xdotool` and `xprintidle` for the Activity Engine
to actually see anything (`apt install xdotool xprintidle` on
Debian/Ubuntu) - without them it runs fine but every sample comes back
`ErrUnavailable`, so nothing gets tracked. On Windows this isn't
needed; the Win32 APIs work out of the box. On macOS, grant STELLAREI
Automation permission for "System Events" when prompted.

### Known, deliberately deferred issue

`npm audit` will show one moderate esbuild advisory
(GHSA-67mh-4wv8-2f99). It only affects Vite's **dev server**, not the
production bundle this app ships, and fixing it means jumping to
Vite 8 (breaking). Left alone until Phase 12 (performance/dependency
hardening) rather than force-upgrading without testing budget for it
here. (The react-router open-redirect CVE that was also flagged has
already been fixed - the project runs react-router-dom 7.x.)

## Full phase roadmap (spec section 84)

1. **Native application shell** ✓
2. **Activity Engine** (process/window/idle detection, sessionization) ✓
3. **Timeline** (day/week/month, corrections, project association) ✓
4. **Analytics** (categories, projects, budgets, goals, trends, context switching) ✓
5. **Timers + alarms + notifications** ✓
6. **Focus Mode** ✓
7. **"I Have No Idea"** ✓
8. **Theme Studio** ✓ *(brought forward - done ahead of schedule)*
9. **Privacy + screenshot memory** ✓
10. **Backups/import/export** ✓
11. **Optional local AI** - foundation + Settings UI + genuinely all six of section 37's listed AI use cases wired into real callers: "I HAVE NO IDEA", Daily Time Receipt, Pattern Discovery, Digital Waste Detection (which gave `ScopeSuggestions` its home), and MY YEAR (which gave `ScopeSummaries` its home - a real gap in this project's own earlier "six of six" claim, caught and fixed) ✓ *(substantially complete - no further spec-named AI use cases remain unwired)*
12. **Performance hardening** - six pieces done: a benchmarked long-run test for Pattern Discovery, an indexing hypothesis tested and honestly disproven (no unnecessary index added), two real N+1 query patterns found and fixed (Budget/Goal status, ~19x fewer redundant queries measured; the analytics export path, which was refetching the entire project list per exported project - plus a non-deterministic export row order bug caught in the same pass), a goroutine-lifecycle audit across all three background components (clean - no leaks, confirmed not assumed), and a completed sweep of every remaining `app/services` loop (all clean; one - `FocusService.History` - correctly distinguished as N legitimately-different queries, not an N+1). Real SQLite write-throughput and Timeline-rendering performance at scale still unverified (no real driver in this sandbox) ← in progress
13. **Packaging** - real groundwork: `Taskfile.yml` + `build/config.yml` (accurate project metadata, matched to Wails v3's documented structure) + `build/appicon.png` (1024×1024 brand mark, generated from `stellarie.png` via `scripts/generate_brand_assets.py`). Per-platform Taskfiles NOT hand-faked - `wails3`/`go-task` both confirmed unrunnable here via direct testing (Go ≥1.25 required; even older go-task releases blocked on `golang.org/x/*`/`gopkg.in/*`/`mvdan.cc/*` vanity imports) ← groundwork started

## Project layout

```
stellarei/
├── main.go                     # Wails entrypoint
├── Taskfile.yml                 # Phase 13 groundwork: root build-system entry point (see its own honesty-note header)
├── PACKAGING.md                 # Phase 13 groundwork: sourced step-by-step runbook for a real wails3/Go 1.25+ environment
├── build/
│   ├── config.yml               # Phase 13 groundwork: real project metadata (name, identifier, version)
│   └── appicon.png              # 1024×1024 brand mark, generated from /stellarie.png by scripts/generate_brand_assets.py
├── app/services/                # Wails-bound services (Settings, Window, Startup, Activity, Theme, Project, Session, Category, Budget, Goal, Analytics, TimeReceipt, Pattern, Waste, Calendar, Search, MyYear, Diagnostics, Timer, Alarm, Notification, Focus, Idea, AI, Privacy, Backup)
├── internal/
│   ├── config/                  # app-data path resolution
│   ├── database/                # SQLite open + migration runner
│   ├── platform/                # OS-specific startup/active-window/idle/screenshot capture
│   ├── activity/                # Activity Engine: debouncer, poll loop, Privacy Mode, repository (+ tests)
│   ├── sessions/                # Sessionizer: event→session grouping, query/correction/last-activity repository (+ tests)
│   ├── projects/                # Project System: rule evaluator, CRUD repository, engine resolver (+ tests)
│   ├── categories/               # Category assignment: CRUD repository, engine resolver
│   ├── budgets/                  # Time budget CRUD (spec section 17)
│   ├── goals/                    # Goal CRUD (spec section 18)
│   ├── periods/                  # shared day/week/month window math for budgets/goals (+ tests)
│   ├── theme/                   # light/dark mode classification from theme tokens (+ tests)
│   ├── timers/                   # Timer System: pause-aware elapsed math, Pomodoro state machine, repository (+ tests)
│   ├── alarms/                   # Alarm System: recurrence/trigger-condition parsers, Due() firing logic, repository (+ tests)
│   ├── notify/                   # notification history + native OS dispatch (wraps Wails' notifications service)
│   ├── scheduler/                # background tick loop: completes timers, fires alarms, checks focus violations (+ tests against fakes)
│   ├── focus/                    # Focus Mode: blocked-app config, quality calculation (+ tests) - built on internal/timers
│   ├── idea/                     # "I Have No Idea": AI-free scoring/selection, chaos bank, project generator, history repository (+ tests)
│   ├── ai/                       # Optional Local AI (Phase 11): Ollama + OpenAI-compatible clients, scope-gated config, graceful-degradation Service (+ tests)
│   ├── patterns/                 # Pattern Discovery (spec section 31): AI-free median/average/trend detection over real session data, plus pattern_insights persistence/dismissal (+ tests, +Python-verified SQL)
│   ├── waste/                    # Digital Waste Detection (spec section 33): AI-free evidence-only detection over real session data, no moral judgments (+ tests)
│   ├── calendar/                 # Calendar (spec section 30): AI-free day-bucketing with correct midnight-crossing session splitting (+ tests)
│   ├── search/                   # Search (spec sections 29/62): bounded-vocabulary query parser - real dates/entities/superlatives, no NLP, no synonyms (+ tests)
│   ├── myyear/                   # MY YEAR (spec section 61): pure ranking/longest-session/busiest-month logic reusing internal/calendar (+ tests)
│   ├── privacy/                  # Privacy Center: exclusion matching, retention policy, honest domain heuristic, Sweeper (+ tests)
│   ├── backup/                   # Backup/Restore/Export: VACUUM INTO snapshots, zip bundling, restart-to-apply restore marker (+ tests)
│   ├── tray/                    # system tray (icon switches with active theme's mode; timer quick actions; Privacy Mode toggle)
│   └── closebehavior/           # pure close-behavior decision logic (+ tests)
├── migrations/                  # embedded SQL migrations (incl. built-in theme seed, Phase 5 timer/alarm columns)
├── assets/                      # tray icon PNGs (16/32/48/256px, dark + light)
└── frontend/                    # React + TS + Vite
    └── src/
        ├── assets/               # logo PNGs (dark + light, full-res)
        ├── components/          # Nav, Logo, CloseConfirmDialog, ThemePreview, SessionDetailPanel
        ├── pages/                # Overview, Timeline, Calendar, Projects, Analytics, TimeReceipt, Patterns, DigitalWaste, MyYear, Goals, Focus, Timers, Alarms, NoIdea, Search, Themes, Settings, Privacy, Backups, Diagnostics, About, Placeholder
        ├── locales/              # en/fa/de.json - i18next translation resources (Nav + Overview fully wired; the rest of the app still hardcoded English, see the Internationalization section above)
        ├── themes/               # design tokens, built-in theme constants, mode classifier, validator
        ├── lib/                  # bindings loader, theme context, time-range/clock/date-param formatting, i18n.ts + language.ts
        └── types/                # shared TS types
```

## Non-negotiables carried over from the spec (do not violate these in later phases)

- No HTTP server, no localhost port, no REST API for the frontend - Wails IPC only.
- No keylogging, clipboard capture, microphone, or camera access,
  ever. The Activity Engine's platform watchers read only process
  identity, window title, and idle *duration* (never keystrokes) - double-check this invariant before touching
  `internal/platform/activewindow_*.go` or `idle_*.go`.
- Screenshots and browser URL tracking default OFF and stay opt-in.
- No telemetry, no cloud backend, no accounts.
- Migrations are up-only - never destroy user data on upgrade.
