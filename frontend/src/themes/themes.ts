/**
 * Theme token contract (spec section 38-40).
 *
 * Phase 1 ships one built-in theme (Midnight, defined in tokens.css) and
 * this validator, which is the foundation Theme Studio (Phase 8) builds
 * its live editor and import/export on top of. Themes are DATA ONLY
 * validateTheme never evaluates anything from the imported file, it only
 * checks shape and that every value is a plausible CSS color string.
 */

export const THEME_TOKEN_KEYS = [
  "background",
  "backgroundElevated",
  "surface",
  "surfaceHover",
  "surfaceActive",
  "border",
  "text",
  "textMuted",
  "accent",
  "accentHover",
  "success",
  "warning",
  "danger",
  "info",
  "chartPrimary",
  "chartSecondary",
  "timelineDevelopment",
  "timelineDesign",
  "timelineMusic",
  "timelineGaming",
  "timelineResearch",
  "timelineIdle",
] as const;

export type ThemeTokenKey = (typeof THEME_TOKEN_KEYS)[number];
export type ThemeTokens = Record<ThemeTokenKey, string>;

export interface ThemeFile {
  name: string;
  version: number;
  author?: string;
  tokens: ThemeTokens;
}

/** A conservative check for "looks like a CSS color", not a full CSS
 * grammar parser. Deliberately permissive about *which* color function is
 * used (hex, rgb(), hsl(), oklch(), named colors) - the goal is rejecting
 * garbage/script content, not enforcing one color format. */
const PLAUSIBLE_COLOR = /^[a-zA-Z0-9#().,%\s-]{1,64}$/;

export interface ThemeValidationResult {
  valid: boolean;
  errors: string[];
}

/** Validates a parsed JSON value against the theme file contract. Never
 * throws - callers (theme import, in particular) should always get a
 * structured result they can show to the user rather than a crash from a
 * malformed file (section 40: "Reject malformed files safely"). */
export function validateTheme(input: unknown): ThemeValidationResult {
  const errors: string[] = [];

  if (typeof input !== "object" || input === null) {
    return { valid: false, errors: ["Theme file must be a JSON object."] };
  }

  const file = input as Partial<ThemeFile>;

  if (typeof file.name !== "string" || file.name.trim() === "") {
    errors.push('Missing or empty "name".');
  }
  if (typeof file.version !== "number" || !Number.isInteger(file.version)) {
    errors.push('Missing or invalid "version" (must be an integer).');
  }
  if (file.author !== undefined && typeof file.author !== "string") {
    errors.push('"author" must be a string if present.');
  }
  if (typeof file.tokens !== "object" || file.tokens === null) {
    errors.push('Missing "tokens" object.');
    return { valid: false, errors };
  }

  const tokens = file.tokens as Record<string, unknown>;
  for (const key of THEME_TOKEN_KEYS) {
    const value = tokens[key];
    if (typeof value !== "string") {
      errors.push(`Token "${key}" is missing or not a string.`);
      continue;
    }
    if (!PLAUSIBLE_COLOR.test(value)) {
      errors.push(`Token "${key}" doesn't look like a valid color: ${JSON.stringify(value)}`);
    }
  }

  return { valid: errors.length === 0, errors };
}

/** Applies a validated theme's tokens as CSS custom properties on the
 * document root. Callers must run validateTheme first - this function
 * trusts its input is already checked, to keep it usable for the live
 * "as you type" preview in Theme Studio without re-validating every
 * keystroke. */
export function applyThemeTokens(tokens: ThemeTokens): void {
  const root = document.documentElement;
  for (const key of THEME_TOKEN_KEYS) {
    root.style.setProperty(`--${key}`, tokens[key]);
  }
}

/** The built-in default theme, kept in sync with tokens.css by hand (see
 * the comment at the top of that file). Exposed so Theme Studio's "Reset"
 * action and the theme library list (section 41) have a real object to
 * point at even before user-created themes exist. */
export const MIDNIGHT_THEME: ThemeFile = {
  name: "Midnight",
  version: 1,
  author: "STELLAREI",
  tokens: {
    background: "#0b0d10",
    backgroundElevated: "#101317",
    surface: "#15181d",
    surfaceHover: "#1b1f25",
    surfaceActive: "#21262d",
    border: "#262b33",
    text: "#e8eaed",
    textMuted: "#9aa1ac",
    accent: "#6ea8fe",
    accentHover: "#8ebbff",
    success: "#4caf82",
    warning: "#d9a441",
    danger: "#e0616b",
    info: "#6ea8fe",
    chartPrimary: "#6ea8fe",
    chartSecondary: "#9b7ede",
    timelineDevelopment: "#6ea8fe",
    timelineDesign: "#d98ee0",
    timelineMusic: "#f2b84b",
    timelineGaming: "#e0616b",
    timelineResearch: "#4caf82",
    timelineIdle: "#3a4048",
  },
};

/** The second built-in theme - light mode, sharing Midnight's indigo
 * accent hue (matching the app logo) so the two feel like siblings
 * rather than unrelated defaults. Token values here MUST stay in sync by
 * hand with migrations/0003_seed_builtin_themes.sql's seed row - see
 * that file's own comment on why there's no single source of truth. */
export const LIGHT_THEME: ThemeFile = {
  name: "Light",
  version: 1,
  author: "STELLAREI",
  tokens: {
    background: "#f5f4fb",
    backgroundElevated: "#ffffff",
    surface: "#ffffff",
    surfaceHover: "#ececf7",
    surfaceActive: "#e3e1f5",
    border: "#ddd9f0",
    text: "#1a1b2e",
    textMuted: "#6b6d85",
    accent: "#5b4fe5",
    accentHover: "#4a3fd1",
    success: "#2f9e6a",
    warning: "#b8791f",
    danger: "#c94454",
    info: "#5b4fe5",
    chartPrimary: "#5b4fe5",
    chartSecondary: "#8b5fd6",
    timelineDevelopment: "#5b4fe5",
    timelineDesign: "#c15fc9",
    timelineMusic: "#d99a2b",
    timelineGaming: "#c94454",
    timelineResearch: "#2f9e6a",
    timelineIdle: "#d5d3e8",
  },
};

export const BUILTIN_THEMES: ThemeFile[] = [MIDNIGHT_THEME, LIGHT_THEME];

/** Classifies a token set as "light" or "dark" from its background
 * color's perceived luminance - the standard broadcast-luma weighting
 * (0.299R + 0.587G + 0.114B), thresholded at 0.5. This determines which
 * logo variant (frontend/src/components/Logo.tsx) and, indirectly, which
 * tray icon (internal/theme.ComputeMode on the Go side) get shown.
 *
 * MUST stay in exact sync with internal/theme/mode.go's ComputeMode
 * same formula, same threshold - or the frontend logo and the Go-side
 * tray icon could disagree about the same theme's mode. Falls back to
 * "dark" if the background token is missing or unparseable, matching the
 * Go side's same fail-safe default.
 */
export function computeThemeMode(tokens: ThemeTokens): "light" | "dark" {
  const hex = (tokens.background ?? "").trim().replace(/^#/, "");
  const full =
    hex.length === 3
      ? hex
          .split("")
          .map((c) => c + c)
          .join("")
      : hex;
  if (!/^[0-9a-fA-F]{6}$/.test(full)) {
    return "dark";
  }
  const r = parseInt(full.slice(0, 2), 16);
  const g = parseInt(full.slice(2, 4), 16);
  const b = parseInt(full.slice(4, 6), 16);
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance < 0.5 ? "dark" : "light";
}
