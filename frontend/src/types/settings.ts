/** Known setting keys - must match app/services/settings_service.go's
 * Key* constants exactly. Duplicated here rather than generated because
 * Wails' binding generator emits functions, not shared constants; if you
 * add a setting key on the Go side, add it here too. */
export const SETTINGS_KEYS = {
  closeBehavior: "close_behavior",
  startupEnabled: "startup_enabled",
  startupBehavior: "startup_behavior",
  idleThresholdSeconds: "idle_threshold_seconds",
  theme: "theme",
  language: "language",
} as const;

export type CloseBehavior = "close" | "tray" | "ask";
export type StartupBehavior = "normal" | "tray" | "silent";
