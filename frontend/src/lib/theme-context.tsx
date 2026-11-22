import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { Events } from "@wailsio/runtime";
import { getThemeBindings, type ThemeRecord } from "@/lib/bindings";
import {
  applyThemeTokens,
  computeThemeMode,
  MIDNIGHT_THEME,
  validateTheme,
  type ThemeTokens,
} from "@/themes/themes";

export interface ParsedTheme {
  name: string;
  isBuiltin: boolean;
  tokens: ThemeTokens;
}

interface ThemeContextValue {
  mode: "light" | "dark";
  activeThemeName: string;
  activeTokens: ThemeTokens;
  themes: ParsedTheme[];
  loading: boolean;
  /** Set when the backend couldn't be reached - Midnight is applied
   * locally as a safe fallback so the app stays usable, but nothing is
   * persisted until the backend is available again. */
  error: string | null;
  refresh: () => Promise<void>;
  activateTheme: (name: string) => Promise<void>;
  saveTheme: (name: string, tokens: ThemeTokens) => Promise<void>;
  deleteTheme: (name: string) => Promise<void>;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme() must be used within <ThemeProvider>");
  return ctx;
}

function parseThemes(records: ThemeRecord[]): ParsedTheme[] {
  const out: ParsedTheme[] = [];
  for (const rec of records) {
    try {
      const parsed = JSON.parse(rec.tokensJson) as ThemeTokens;
      out.push({ name: rec.name, isBuiltin: rec.isBuiltin, tokens: parsed });
    } catch {
      // A theme row with unparseable JSON shouldn't take down the whole
      // list - skip it silently rather than crash Theme Studio over one
      // bad row (section 56).
    }
  }
  return out;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [themes, setThemes] = useState<ParsedTheme[]>([{ name: MIDNIGHT_THEME.name, isBuiltin: true, tokens: MIDNIGHT_THEME.tokens }]);
  const [activeThemeName, setActiveThemeName] = useState(MIDNIGHT_THEME.name);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const applyLocally = useCallback((name: string, tokens: ThemeTokens) => {
    applyThemeTokens(tokens);
    setActiveThemeName(name);
  }, []);

  const refresh = useCallback(async () => {
    try {
      const backend = await getThemeBindings();
      const [records, activeName] = await Promise.all([backend.ListThemes(), backend.GetActiveThemeName()]);
      const parsed = parseThemes(records);
      setThemes(parsed.length > 0 ? parsed : [{ name: MIDNIGHT_THEME.name, isBuiltin: true, tokens: MIDNIGHT_THEME.tokens }]);

      const active = parsed.find((t) => t.name === activeName);
      if (active) {
        applyLocally(active.name, active.tokens);
      } else {
        // Active theme name doesn't match anything we got back - fall
        // back to Midnight rather than apply nothing.
        applyLocally(MIDNIGHT_THEME.name, MIDNIGHT_THEME.tokens);
      }
      setError(null);
    } catch (err) {
      applyLocally(MIDNIGHT_THEME.name, MIDNIGHT_THEME.tokens);
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [applyLocally]);

  useEffect(() => {
    refresh();
    // "theme:changed" fires from Go whenever ThemeService's active theme
    // changes - including from an in-place edit to the currently active
    // theme (see app/services/theme_service.go's SaveTheme). Also covers
    // the tray or any future surface switching themes on our behalf.
    return Events.On("theme:changed", refresh);
  }, [refresh]);

  const activateTheme = useCallback(
    async (name: string) => {
      const target = themes.find((t) => t.name === name);
      if (!target) throw new Error(`No theme named "${name}" is loaded.`);
      const backend = await getThemeBindings();
      await backend.SetActiveThemeName(name);
      applyLocally(target.name, target.tokens);
    },
    [themes, applyLocally],
  );

  const saveTheme = useCallback(
    async (name: string, tokens: ThemeTokens) => {
      const result = validateTheme({ name, version: 1, tokens });
      if (!result.valid) {
        throw new Error(result.errors.join(" "));
      }
      const backend = await getThemeBindings();
      await backend.SaveTheme(name, JSON.stringify(tokens));
      await refresh();
    },
    [refresh],
  );

  const deleteTheme = useCallback(
    async (name: string) => {
      const backend = await getThemeBindings();
      await backend.DeleteTheme(name);
      await refresh();
    },
    [refresh],
  );

  const activeTokens = useMemo(
    () => themes.find((t) => t.name === activeThemeName)?.tokens ?? MIDNIGHT_THEME.tokens,
    [themes, activeThemeName],
  );

  const mode = useMemo(() => computeThemeMode(activeTokens), [activeTokens]);

  const value: ThemeContextValue = {
    mode,
    activeThemeName,
    activeTokens,
    themes,
    loading,
    error,
    refresh,
    activateTheme,
    saveTheme,
    deleteTheme,
  };

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
