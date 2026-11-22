# Packaging STELLAREI - runbook

This is the "what to actually run" companion to the README's Phase 13
section. Update 2026-09-15: with Go 1.27.1 + `wails3` v3.0.0-beta.22 on
a real Windows machine, Steps 0-3 below have now actually been run
against this repo (see "Verified 2026-09-15" at the bottom) - the
remaining unchecked items are installer packaging (needs NSIS) and the
end-to-end Definition-of-Done checks. Every command is sourced from
Wails v3's own current documentation (`v3.wails.io`) rather than memory
of the framework.

## Prerequisites

- Go ≥ 1.26 (verified with 1.27.1; `go.mod` requires 1.26 via
  modernc.org/sqlite, Wails v3 itself requires ≥ 1.25)
- The `wails3` CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
  (verified with v3.0.0-beta.22)
- Node.js + npm (verified with Node 26 / npm 10 - `frontend/` builds
  and typechecks cleanly)
- Platform build tooling (only needed for installers, NOT for `wails3
  build`): NSIS for the Windows installer, Xcode command line tools
  for macOS, standard build-essential-equivalent packages for Linux
  .deb/AppImage output

## What's already real in this repo (nothing to redo)

- `build/appicon.png` - STELLAREI's actual mark, at the exact path
  `wails3 generate icons` defaults to (pass `-input build/appicon.png`
  explicitly to be safe on older CLI builds). Generated (1024×1024,
  transparent, 6% padding) from the root `stellarie.png` by
  `python scripts/generate_brand_assets.py`, which also regenerates the
  frontend favicons (`frontend/public/`), the theme-aware logo variants
  (`frontend/src/assets/logo-*.png`), and the tray icons
  (`assets/tray-icon-*.png`) from that one source - run it after any logo
  change, then `wails3 task common:generate:icons`.
- `build/config.yml` - real project metadata in the generator's own
  schema: `productName: STELLAREI`, `productIdentifier:
  com.dahgan.stellarei`, `version: "0.1.0"` (matches
  `frontend/package.json`), `companyName: DAHGAN Studio`.
- `Taskfile.yml` + `build/Taskfile.yml` + `build/windows|darwin|linux|docker/` - the real generator-produced build system (scaffolded via `wails3
  init` into a scratch project on 2026-09-15 and copied in, keeping
  STELLAREI's own icon and metadata). The handwritten placeholders plus
  their honesty-note headers are kept as `Taskfile.yml.bak-handwritten`
  and `build/config.yml.bak-handwritten` for the record.
- `build/windows/icon.ico` + `build/darwin/icons.icns` - generated from
  STELLAREI's real mark via `wails3 task common:generate:icons`.
- `frontend/bindings/` - generated via `wails3 generate bindings -ts`
  (28 services, 131 methods, 68 models). One benign warning: "function
  types are not supported by encoding/json" for `app/services` - that
  refers to the unexported `ShortcutRegistrarFunc`/`restartFunc`/callback
  fields, none of which are exposed to the frontend; no method is
  dropped because of it.
- `bin/stellarei.exe` - real production build (`wails3 build`, ~17MB).

## Step 1 - (DONE 2026-09-15) generate icons and the per-platform Taskfiles

```bash
# Windows + macOS icon set, from the app icon already in this repo:
wails3 task common:generate:icons

# Regenerate platform assets whenever build/config.yml `info:` changes:
wails3 task common:update:build-assets
```

## Step 2 - verify the dev loop works before trying to package anything

```bash
wails3 dev -config ./build/config.yml
```

This should build the Go backend, start Vite for `frontend/`, and open
a live-reloading window. If this doesn't work, packaging won't either - it's the cheapest possible smoke test and worth doing before Step 3.

## Step 3 - build and package

```bash
# Debug build for the current platform:
wails3 build

# Or drive Task directly for more control (see `wails3 task --list`
# once Step 1 has generated the platform Taskfiles):
wails3 task build
wails3 task windows:build
wails3 task darwin:build
wails3 task linux:build

# Platform installers/packages - spec section 2's primary target is
# Windows 10/11 AMD64, so start there:
wails3 task windows:package     # NSIS installer
wails3 task darwin:package:universal
wails3 task linux:create:deb
wails3 task linux:create:appimage
```

## Verified 2026-10-01 (icon refresh + installer rebuild)

- The brand mark was replaced (root `stellarie.png`, pink pinwheel on
  transparency) and every derived asset regenerated via
  `scripts/generate_brand_assets.py` + `wails3 task common:generate:icons`:
  app icon/.ico/.icns, frontend favicons + logo variants, tray icons.
- `wails3 task windows:package INSTALL_SCOPE=user` - rebuilt
  `bin/stellarei.exe` (17.8MB) and `bin/stellarei-amd64-installer.exe`
  (~9.1MB) with the new icon embedded in the exe and installer UI.
  Note: `makensis` had to be added to PATH manually this shell
  (`export PATH="/c/Program Files (x86)/NSIS:$PATH"`) despite being
  installed - winget's shim didn't persist across shells.

## Verified 2026-09-15 (Go 1.27.1, wails3 v3.0.0-beta.22, Windows 10/amd64)

- `go mod tidy` resolves cleanly (Wails v3.0.0-beta.22, modernc sqlite
  v1.58.0, x/sys v0.48.0)
- `go vet ./...` clean, `go test ./...` - all packages pass
- `go build .` succeeds (this also caught and fixed a real compile bug:
  `shortcutsService` was referenced in the `Services` list before its
  definition - fixed via appRef/secondInstanceWindow indirection, see
  main.go's comment)
- `wails3 generate bindings -ts` - 28 services / 131 methods / 68 models
- `npx tsc --noEmit` clean against the freshly generated bindings
- `wails3 build` - `bin/stellarei.exe` (~17MB, `-H windowsgui`)
- Smoke test: launched the exe, process alive and steady at ~75MB after
  12s, then stopped. Full UI walkthrough not yet done.
- NSIS 3.12 installed via winget (needed one retry - transient source
  cert error; choco needs admin, SourceForge mirrors are bot-walled for
  scripts, so winget is the documented route here).
- `wails3 task windows:package INSTALL_SCOPE=user` - produced
  `bin/stellarei-amd64-installer.exe` (~9MB). Per-user scope chosen
  deliberately: it matches spec section 74 ("without requiring
  administrator privileges").
- Installer run silently (`/S`) as a normal user, no UAC: installs to
  `%LOCALAPPDATA%\Programs\STELLAREI\` with `uninstall.exe`, Start Menu
  `STELLAREI.lnk` pointing at the installed exe.
- Installed app launched: single process; a second launch exited
  immediately leaving exactly 1 process (single-instance lock holds
  end-to-end, sections 75-76).
- First launch created `%AppData%\STELLAREI\` with `stellarei.db`
  (+WAL) and all 7 migrations recorded in `schema_migrations`
  (0001-0007), plus backups/exports/logs/screenshots dirs.
- Run-key round-trip via the REAL `internal/platform` code (temporary
  `zz_smoke_startup_test.go`, run once, deleted after): Register →
  IsRegistered true → Unregister → state restored to initial, all
  against HKCU, no elevation (section 74's OS path proven; reboot
  persistence itself still unchecked).
- PE subsystem parsed directly: installed exe = 2 (Windows GUI, no
  console). Cautionary tale: `bin\stellarei.exe` briefly read as
  subsystem 3 (Console) mid-session - that was a DEV binary
  (`wails3 build DEV=true` omits `-H windowsgui`) written over `bin/`
  by an accidentally-started `wails3 dev` whose tree was then reaped,
  orphaning vite + a dev app window; orphans identified via
  Win32_Process command lines and killed, `wails3 task windows:build`
  re-run, subsystem re-verified as 2. The accident also proved the dev
  pipeline's halves: DEV backend links, `npm run build:dev` compiles,
  and the orphaned tree showed vite serving on :9245 with the dev app
  running - i.e. `wails3 dev` works; only the interactive hot-reload
  keystroke itself wasn't exercised. `frontend/dist` rebuilt to
  production afterwards.

## Things to check once a real build exists (spec section 85's
## Definition of Done, packaging-relevant items)

- [x] Runs as a native desktop application, no console window flashing
      on Windows launch (PE subsystem = Windows GUI, verified by direct
      header parse; app launches and holds steady)
- [ ] Starts directly into the tray when configured to (section 24) - needs a person to flip the setting and relaunch
- [x] Single-instance lock prevents a second launch (sections 75-76) - verified live: 2nd process exits, count stays 1
- [~] Run-at-startup registration (section 74) - HKCU Run-key
      Register/IsRegistered/Unregister round-trip passes against the
      real code with no elevation; surviving an actual reboot is
      unchecked, as are the macOS/Linux paths
- [x] The NSIS installer doesn't require administrator privileges for
      a normal per-user install (installed silently with zero UAC)
- [x] Database/settings/theme files land in the correct per-OS
      app-data directory (`%AppData%\STELLAREI\`, all 7 migrations
      applied on first launch)
- [ ] WebView2 bootstrap behaves correctly on a clean Windows machine
      that doesn't already have it (this machine already has WebView2
      151, so the bootstrapper path never triggered)

## Fixed 2026-09-15: "bindings haven't been generated yet" on every tab

Symptom in the installed app: every tab showed `STELLAREI: bindings
for <X>Service haven't been generated yet`, even though
`frontend/bindings/` existed and `wails3 generate bindings` had run.
Root cause: `frontend/src/lib/bindings.ts` loaded each service via
`import(/* @vite-ignore */ relativePath)` with a RUNTIME variable.
Vite leaves such imports as runtime URLs pointing at
`frontend/bindings/…`, which is served from disk by `wails3 dev` but
does NOT exist inside the production `dist/` bundle (verified: zero
`bindings/stellarei` references in `dist/assets/*.js`). Fix: each of
the 27 call sites now passes a closure wrapping a LITERAL
`import("…")` specifier, which Rollup code-splits into `dist` (27
per-service chunks + `calls-*.js`, verified present after rebuild).
Lesson: `@vite-ignore` dynamic imports only ever work in dev mode - never use them for files that must ship. Rebuilt, repackaged,
reinstalled, relaunched; reinstall preserves `%AppData%\STELLAREI\`
(the pre-existing `stellarei.db` survives untouched).

## Fixed 2026-09-15: Calendar day click → black screen + tab state loss

Two user-reported bugs, one audit, one new contract:

1. **Black screen (root cause: Go nil slices → JSON null).**
   Clicking a Calendar day navigates to `/receipt?date=…`; on a fresh
   database `TimeReceiptView.Goals/Budgets` were nil slices, marshaled
   as `null`, and `receipt.goals.length` threw during render. With no
   error boundary the whole tree unmounted (black window), and the
   crashing hash route persisted, so every reload re-crashed - "totally
   unusable". Fixed at the source: `receipt_service.go` (goals,
   budgets), `myyear_service.go` (abandoned, goalsComplete),
   `theme_service.go` (ListThemes), `search_service.go` (instant
   results, superlative branch), `backup_service.go` (empty exports
   wrote files containing `null`) now all initialize empty. Audited
   every other service boundary the same way - all already used
   `make()`. New contract, enforced by the first-ever test in
   `app/services` (`empty_views_test.go`, asserting JSON-array
   marshaling on empty DBs - verified non-vacuous by reverting one fix
   and watching it fail with exactly `"goals" … got <nil>`).
2. **Severity backstop: `RouteErrorBoundary`.** Any future page crash
   now shows a recovery screen (translated en/fa/de, new `crash.*`
   keys with parity verified) with Back-to-Overview + Reload, auto-
   resetting on navigation - no crash can ever trap the app again.
3. **Tab state loss (by design, now fixed).** Routes unmounted on
   switch, discarding typed text/drafts. `KeepAliveRoutes` mounts each
   page on first visit and hides (not unmounts) inactive ones; startup
   cost unchanged. Required companion fixes: TimeReceipt and Timeline
   now sync their anchor/zoom from the URL via effects (previously
   initializer-only, so date→date links would have gone stale under
   keep-alive).
4. Belt-and-braces `?? []` at the two proven crash sites
   (TimeReceipt, Search); nested/optional fields (pomodoro, snooze,
   blockedApps, reasons) audited - all already guarded.

## Sources

Every command above is drawn from, in order of how recently each was
checked: `https://v3.wails.io/guides/build/building/`,
`https://v3.wails.io/concepts/build-system/`,
`https://v3.wails.io/guides/cli/`,
`https://v3.wails.io/guides/build/macos/`, and
`https://github.com/wailsapp/wails/pull/5825` (for the icon-defaults
fix note above). Wails v3 was still in beta at the time this was
written (`v3.0.0-beta.16` was the latest tag `go install` found) - re-check these pages before following this runbook verbatim if much
time has passed, since beta APIs are exactly the kind of thing that
still moves.
