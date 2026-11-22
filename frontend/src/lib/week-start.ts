import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getWeekStartDay } from "@/lib/i18n";
import { getSettingsBindings } from "@/lib/bindings";

// Manual week-start override - the person's own choice of which weekday
// calendars and week views start on, distinct from the automatic
// locale-tying in lib/i18n.ts's getWeekStartDay (English/German → Monday,
// Persian → Saturday). Same setting key the Go backend already defines
// (app/services/settings_service.go's KeyWeekStart); this file is its
// first consumer via the generic GetString/SetString, the same way
// lib/language.ts was the first consumer of KeyLanguage with zero new Go
// code needed.
export const WEEK_START_SETTING_KEY = "week_start";

// Stored values: "auto" (or unset/empty) follows the active language's own
// convention; "0".."6" pins a specific weekday as a JS Date.getDay() index
// (0=Sunday...6=Saturday), regardless of language.
export const WEEK_START_AUTO = "auto";

function isStoredWeekStartDay(value: string): boolean {
  return /^[0-6]$/.test(value);
}

/** Resolves the effective week-start day (a JS Date.getDay() index) from
 * the active language plus the persisted override. A missing, empty,
 * "auto", or unrecognized stored value all mean "follow the language"
 * a corrupted or pre-this-feature settings row must never break a
 * calendar grid, same discipline as lib/language.ts's
 * applyPersistedLanguage. */
export function resolveWeekStartDay(lang: string, stored: string | null | undefined): number {
  if (stored !== null && stored !== undefined && stored !== "" && stored !== WEEK_START_AUTO) {
    if (isStoredWeekStartDay(stored)) return Number(stored);
  }
  return getWeekStartDay(lang);
}

/** Reads the persisted override - "auto" when never set or unreadable,
 * so callers always get something resolveWeekStartDay accepts. Never
 * rejects: a bindings failure degrades to the locale default rather
 * than blocking the page that asked. */
export async function loadWeekStartSetting(): Promise<string> {
  try {
    const settings = await getSettingsBindings();
    const stored = await settings.GetString(WEEK_START_SETTING_KEY);
    if (stored === "" || stored === WEEK_START_AUTO || isStoredWeekStartDay(stored)) {
      return stored === "" ? WEEK_START_AUTO : stored;
    }
    return WEEK_START_AUTO;
  } catch {
    return WEEK_START_AUTO;
  }
}

/** Persists the override - called from Settings' week-start picker. */
export async function saveWeekStartSetting(value: string): Promise<void> {
  const settings = await getSettingsBindings();
  await settings.SetString(WEEK_START_SETTING_KEY, value);
}

/** React hook returning the effective week-start day, loading the
 * persisted override once on mount. First paint uses the locale default
 * (identical to the pre-override behavior), then re-renders with the
 * stored choice once it arrives - the same load-then-apply shape as the
 * theme and language preferences elsewhere on Settings. */
export function useWeekStartDay(): number {
  const { i18n } = useTranslation();
  const [stored, setStored] = useState<string>(WEEK_START_AUTO);
  useEffect(() => {
    let cancelled = false;
    loadWeekStartSetting().then((value) => {
      if (!cancelled) setStored(value);
    });
    return () => {
      cancelled = true;
    };
  }, []);
  return resolveWeekStartDay(i18n.language, stored);
}
