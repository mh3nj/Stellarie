# Stellarie repository cleanup notes

This is a review guide, not an automated deletion list. The overwrite package intentionally does not delete source files.

## Safe candidates to remove after checking local tooling

- `.task/` — generated task checksum/cache state. Usually local build metadata, not application source.
- `Taskfile.yml.bak-handwritten` — explicitly retained backup of an older handwritten build file. The active `Taskfile.yml` is the generated/real build entry point.
- `build/config.yml.bak-handwritten` — explicitly retained backup of the earlier handwritten config. `build/config.yml` is the active config.
- `frontend/tsconfig.tsbuildinfo` and `frontend/tsconfig.node.tsbuildinfo` — TypeScript incremental build caches.
- `frontend/vite.config.js` and `frontend/vite.config.d.ts` — generated JavaScript/declaration outputs if the project only maintains `vite.config.ts`. Check whether any workflow or package command imports them before deleting.
- `build/windows/nsis/MicrosoftEdgeWebview2Setup.exe` — bundled executable installer. It is large and may be unnecessary to version if the NSIS script downloads or references the official WebView2 bootstrapper instead. Inspect `build/windows/nsis/project.nsi` first; do not remove if the script expects this exact local file.

## Keep

- `frontend/bindings/` — generated Wails bindings used by the frontend at runtime/build time.
- `frontend/package-lock.json` and `go.sum` — reproducible dependency locks.
- `migrations/` — required database schema history.
- `build/`, `assets/`, `app/`, `internal/`, `frontend/src/`, platform-specific source and icon assets — part of the actual application/build pipeline.
- `scripts/` — repository maintenance/brand asset scripts.
- `PACKAGING.md` — useful build and packaging runbook.

## Do not delete based on this audit alone

The README describes a long development history and some verified/unverified work. Keep the implementation and tests intact. The only clear archival cleanup candidates are backups and generated caches. If you want the repository to remain a faithful historical snapshot, retaining the `.bak-handwritten` files is defensible; they are not required by the active build.
