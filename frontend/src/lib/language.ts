import i18n, { isRTL, SUPPORTED_LANGUAGES, type SupportedLanguage } from "@/lib/i18n";
import { getSettingsBindings } from "@/lib/bindings";

// Same setting key the Go backend already defined
// (app/services/settings_service.go's KeyLanguage = "language") before
// any i18n code existed on the frontend - this file is the first
// consumer of a setting the schema had already anticipated, the same
// situation Pattern Discovery's dismissed column and MY YEAR's
// ScopeSummaries were each in.
const LANGUAGE_SETTING_KEY = "language";

function isSupportedLanguage(value: string): value is SupportedLanguage {
  return (SUPPORTED_LANGUAGES as readonly string[]).includes(value);
}

/** Sets <html dir> and <html lang> to match the active language - spec
 * section 69's RTL requirement applied at the document root, so it
 * affects native form controls and scrollbars too, not just React-
 * rendered text. */
export function applyDocumentDirection(lang: string) {
  document.documentElement.dir = isRTL(lang) ? "rtl" : "ltr";
  document.documentElement.lang = lang;
}

/** Reads the persisted language setting (if any) and applies it
* called once at app startup. A missing, empty, or unrecognized stored
 * value is treated the same as "never set": stays on i18next's own
 * "en" default rather than erroring, since a corrupted or
 * pre-this-feature settings row shouldn't break the app on launch. */
export async function applyPersistedLanguage(): Promise<void> {
  try {
    const settings = await getSettingsBindings();
    const stored = await settings.GetString(LANGUAGE_SETTING_KEY);
    if (stored && isSupportedLanguage(stored)) {
      await i18n.changeLanguage(stored);
    }
  } catch {
    // Bindings not available yet, or the call failed - stay on the
    // default rather than block startup on a settings read.
  } finally {
    applyDocumentDirection(i18n.language);
  }
}

/** Changes the active language and persists the choice - called from
 * Settings' language picker. */
export async function setLanguage(lang: SupportedLanguage): Promise<void> {
  await i18n.changeLanguage(lang);
  applyDocumentDirection(lang);
  try {
    const settings = await getSettingsBindings();
    await settings.SetString(LANGUAGE_SETTING_KEY, lang);
  } catch {
    // Persistence failure shouldn't undo the in-session language
    // change - worst case, the choice doesn't survive a restart, which
    // the person can just make again rather than see it silently
    // revert right now.
  }
}
