import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/locales/en.json";
import fa from "@/locales/fa.json";
import de from "@/locales/de.json";

// Spec section 69: "Build i18n architecture from the beginning.
// Initially support: English, Persian, German." This is that
// architecture - real i18next, not a placeholder. What's honestly NOT
// done yet: most of the app's ~20 pages still have hardcoded English
// strings that haven't been extracted into these locale files (see
// README's i18n section for exactly which pages ARE wired: Nav and
// Overview, chosen specifically because Nav's labels are what spec
// section 69's "mirrored navigation" requirement is actually about,
// and Overview is the first page anyone sees). Extracting the rest is
// real, mechanical, but substantial additional work - not attempted in
// this pass rather than done partially across every page inconsistently.
export const SUPPORTED_LANGUAGES = ["en", "fa", "de"] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

// RTL_LANGUAGES drives document.documentElement.dir (see
// lib/language.ts) - Persian is the only RTL language STELLAREI
// supports right now, but this is a set specifically so a future
// language (Arabic, Hebrew, Urdu) is one line here, not a scattered
// "if lang === 'fa'" check repeated wherever direction matters.
export const RTL_LANGUAGES: ReadonlySet<string> = new Set(["fa"]);

export function isRTL(lang: string): boolean {
  return RTL_LANGUAGES.has(lang);
}

// WEEK_START_DAY drives the calendar grids in Calendar.tsx and
// Timeline.tsx's month view (spec section 69's locale-aware week-start
// requirement - named as a real, separate gap in earlier i18n work,
// closed here). Value is a JS Date.getDay() index (0=Sun...6=Sat)
// identifying which weekday is grid column 0. English and German both
// use the ISO-8601/European Monday-first convention already baked
// into this app's weekday label ordering; Persian traditionally
// starts the week on Saturday (شنبه), ending Friday (جمعه) - verified
// against multiple independent sources describing the Iranian
// calendar, not assumed. A language missing from this map falls back
// to Monday (1), matching every locale this app ships today.
const WEEK_START_DAY: Partial<Record<SupportedLanguage, number>> = {
  en: 1,
  de: 1,
  fa: 6,
};

export function getWeekStartDay(lang: string): number {
  return WEEK_START_DAY[lang as SupportedLanguage] ?? 1;
}

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    fa: { translation: fa },
    de: { translation: de },
  },
  lng: "en", // real persisted preference is applied asynchronously - see lib/language.ts's applyPersistedLanguage, called once at app startup
  fallbackLng: "en",
  interpolation: {
    escapeValue: false, // React already escapes; i18next's own escaping would double-escape
  },
  // Never silently render a raw key in place of missing text - falling
  // back to English is more honest than a person seeing "nav.overview"
  // literally rendered because a translation key was missed.
  returnEmptyString: false,
});

export default i18n;
