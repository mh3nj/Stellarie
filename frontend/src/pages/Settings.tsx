import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import {
  getSettingsBindings,
  getStartupBindings,
  getAIBindings,
  getShortcutsBindings,
  AI_SCOPES,
  type SettingsBindings,
  type StartupBindings,
  type AIBindings,
  type AIConfigView,
  type AIResultView,
  type AIDiagnosticsView,
  type ShortcutsBindings,
  type ShortcutView,
} from "@/lib/bindings";
import type { CloseBehavior, StartupBehavior } from "@/types/settings";
import { useTheme } from "@/lib/theme-context";
import i18n, { SUPPORTED_LANGUAGES, type SupportedLanguage } from "@/lib/i18n";
import { setLanguage } from "@/lib/language";
import { WEEK_START_AUTO, loadWeekStartSetting, saveWeekStartSetting } from "@/lib/week-start";

const LANGUAGE_LABELS: Record<SupportedLanguage, string> = {
  en: "English",
  fa: "فارسی (Persian)",
  de: "Deutsch (German)",
};

const CLOSE_OPTIONS: { value: CloseBehavior; labelKey: string; hintKey: string }[] = [
  { value: "tray", labelKey: "settings.closeTrayLabel", hintKey: "settings.closeTrayHint" },
  { value: "close", labelKey: "settings.closeCloseLabel", hintKey: "settings.closeCloseHint" },
  { value: "ask", labelKey: "settings.closeAskLabel", hintKey: "settings.closeAskHint" },
];

const STARTUP_OPTIONS: { value: StartupBehavior; labelKey: string }[] = [
  { value: "normal", labelKey: "settings.startupNormalLabel" },
  { value: "tray", labelKey: "settings.startupTrayLabel" },
  { value: "silent", labelKey: "settings.startupSilentLabel" },
];

const AI_PROVIDER_OPTIONS: { value: string; labelKey: string; endpointHint: string }[] = [
  { value: "ollama", labelKey: "settings.providerOllama", endpointHint: "e.g. http://localhost:11434" },
  { value: "openai_compatible", labelKey: "settings.providerLmStudio", endpointHint: "e.g. http://localhost:1234" },
];

const AI_SCOPE_LABEL_KEYS: Record<string, string> = {
  summaries: "settings.aiScopeSummaries",
  patterns: "settings.aiScopePatterns",
  suggestions: "settings.aiScopeSuggestions",
  project_ideas: "settings.aiScopeProjectIdeas",
  i_have_no_idea: "settings.aiScopeIHaveNoIdea",
  daily_reviews: "settings.aiScopeDailyReviews",
};

// Matches internal/shortcuts.Actions' fixed display order exactly, so
// this list's order never needs to be kept in sync with a second
// source of truth beyond "same order the backend lists them in."
const SHORTCUT_LABEL_KEYS: Record<string, string> = {
  focus: "settings.shortcutFocusLabel",
  pause: "settings.shortcutPauseLabel",
  timer: "settings.shortcutTimerLabel",
  no_idea: "settings.shortcutNoIdeaLabel",
};

// Week-start picker values are JS Date.getDay() indexes as strings
// "auto" plus Monday-first fixed order (a picker list, not a grid, so
// it stays put regardless of locale). Day names reuse Calendar's
// already-translated weekdayMon..weekdaySun keys rather than adding
// seven near-duplicate settings.* keys - same cross-page reuse the
// handbook's i18n pass established.
const WEEK_START_OPTIONS: { value: string; labelKey: string }[] = [
  { value: WEEK_START_AUTO, labelKey: "settings.weekStartAutoOption" },
  { value: "1", labelKey: "calendar.weekdayMon" },
  { value: "2", labelKey: "calendar.weekdayTue" },
  { value: "3", labelKey: "calendar.weekdayWed" },
  { value: "4", labelKey: "calendar.weekdayThu" },
  { value: "5", labelKey: "calendar.weekdayFri" },
  { value: "6", labelKey: "calendar.weekdaySat" },
  { value: "0", labelKey: "calendar.weekdaySun" },
];

export function Settings() {
  const { t } = useTranslation();
  const [language, setLanguageState] = useState<SupportedLanguage>(i18n.language as SupportedLanguage);
  const [languageError, setLanguageError] = useState<string | null>(null);
  // Manual week-start override. Loaded and errored independently of the
  // settings/startup bindings above - same reasoning as the AI and
  // shortcuts blocks: a missing binding here must never block Close
  // Behavior / Startup / Theme / Language.
  const [weekStart, setWeekStartState] = useState<string>(WEEK_START_AUTO);
  const [weekStartError, setWeekStartError] = useState<string | null>(null);
  const themeCtx = useTheme();
  const [themeSwitchError, setThemeSwitchError] = useState<string | null>(null);

  const [settings, setSettings] = useState<SettingsBindings | null>(null);
  const [startup, setStartup] = useState<StartupBindings | null>(null);
  const [bindingsError, setBindingsError] = useState<string | null>(null);

  const [closeBehavior, setCloseBehaviorState] = useState<CloseBehavior>("tray");
  const [startupEnabled, setStartupEnabledState] = useState(false);
  const [startupBehavior, setStartupBehaviorState] = useState<StartupBehavior>("tray");

  const [loading, setLoading] = useState(true);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Phase 11: Optional Local AI. Loaded and errored independently of the
  // block above - AI is OFF by default and entirely optional (spec
  // section 37), so a problem loading its bindings (e.g. in a dev
  // environment where wails3 generate bindings hasn't run yet) must
  // never block Close Behavior / Startup / Theme, which is why this has
  // its own error state instead of feeding into bindingsError.
  const [ai, setAi] = useState<AIBindings | null>(null);
  const [aiConfig, setAiConfig] = useState<AIConfigView | null>(null);
  const [aiBindingsError, setAiBindingsError] = useState<string | null>(null);
  const [aiSaveError, setAiSaveError] = useState<string | null>(null);
  const [endpointDraft, setEndpointDraft] = useState("");
  const [modelDraft, setModelDraft] = useState("");
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<AIResultView | null>(null);
  const [diagnostics, setDiagnostics] = useState<AIDiagnosticsView | null>(null);

  // Spec section 45: global shortcuts. Loaded and errored independently
  // of Close Behavior / Startup / Theme, same reasoning as the AI block
  // above - a missing binding here must never block the rest of the
  // page. shortcutDrafts holds each row's in-progress text separately
  // from the last-known-saved ShortcutView list, so typing doesn't get
  // clobbered by a stale re-render before the person blurs the field.
  const [shortcuts, setShortcuts] = useState<ShortcutsBindings | null>(null);
  const [shortcutViews, setShortcutViews] = useState<ShortcutView[]>([]);
  const [shortcutDrafts, setShortcutDrafts] = useState<Record<string, string>>({});
  const [shortcutErrors, setShortcutErrors] = useState<Record<string, string>>({});
  const [shortcutsBindingsError, setShortcutsBindingsError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const bindings = await getShortcutsBindings();
        if (cancelled) return;
        setShortcuts(bindings);
        const views = await bindings.ListShortcuts();
        if (cancelled) return;
        setShortcutViews(views);
        setShortcutDrafts(Object.fromEntries(views.map((v) => [v.action, v.accelerator])));
      } catch (err) {
        if (!cancelled) setShortcutsBindingsError(err instanceof Error ? err.message : String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const bindings = await getAIBindings();
        if (cancelled) return;
        setAi(bindings);
        const cfg = await bindings.GetConfig();
        if (cancelled) return;
        setAiConfig(cfg);
        setEndpointDraft(cfg.endpoint);
        setModelDraft(cfg.model);
      } catch (err) {
        if (!cancelled) setAiBindingsError(err instanceof Error ? err.message : String(err));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const stored = await loadWeekStartSetting();
      if (!cancelled) setWeekStartState(stored);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const [s, st] = await Promise.all([getSettingsBindings(), getStartupBindings()]);
        if (cancelled) return;
        setSettings(s);
        setStartup(st);

        const [cb, enabled] = await Promise.all([s.GetCloseBehavior(), st.IsEnabled()]);
        if (cancelled) return;
        setCloseBehaviorState(cb);
        setStartupEnabledState(enabled);
      } catch (err) {
        if (!cancelled) {
          setBindingsError(err instanceof Error ? err.message : String(err));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  async function handleCloseBehaviorChange(value: CloseBehavior) {
    if (!settings) return;
    setCloseBehaviorState(value); // optimistic - this is a local preference, not shared state
    setSaveError(null);
    try {
      await settings.SetCloseBehavior(value);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleStartupToggle(enabled: boolean) {
    if (!startup) return;
    setSaveError(null);
    try {
      await startup.SetEnabled(enabled, startupBehavior);
      setStartupEnabledState(enabled);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleStartupBehaviorChange(value: StartupBehavior) {
    setStartupBehaviorState(value);
    if (!startup || !startupEnabled) return; // only re-persist if startup is actually on
    setSaveError(null);
    try {
      await startup.SetEnabled(true, value);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleThemeChange(name: string) {
    setThemeSwitchError(null);
    try {
      await themeCtx.activateTheme(name);
    } catch (err) {
      setThemeSwitchError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleLanguageChange(lang: SupportedLanguage) {
    setLanguageError(null);
    try {
      await setLanguage(lang);
      setLanguageState(lang);
    } catch (err) {
      setLanguageError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleWeekStartChange(value: string) {
    setWeekStartError(null);
    try {
      await saveWeekStartSetting(value);
      setWeekStartState(value);
    } catch (err) {
      setWeekStartError(err instanceof Error ? err.message : String(err));
    }
  }

  // Every AI handler below re-reads GetConfig() after a write rather than
  // optimistically patching local state - these settings gate whether
  // real network calls to the person's own local server happen at all
  // (spec section 37), so this section shows exactly what's saved, not
  // what the UI assumed would be saved.
  async function refreshAiConfig() {
    if (!ai) return;
    const cfg = await ai.GetConfig();
    setAiConfig(cfg);
    setEndpointDraft(cfg.endpoint);
    setModelDraft(cfg.model);
  }

  async function handleAiEnabledToggle(enabled: boolean) {
    if (!ai) return;
    setAiSaveError(null);
    try {
      await ai.SetEnabled(enabled);
      await refreshAiConfig();
    } catch (err) {
      setAiSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleAiProviderChange(provider: string) {
    if (!ai) return;
    setAiSaveError(null);
    try {
      await ai.SetProvider(provider);
      await refreshAiConfig();
    } catch (err) {
      setAiSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function commitEndpoint() {
    if (!ai || !aiConfig || endpointDraft === aiConfig.endpoint) return;
    setAiSaveError(null);
    try {
      await ai.SetEndpoint(endpointDraft);
      await refreshAiConfig();
    } catch (err) {
      setAiSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function commitModel() {
    if (!ai || !aiConfig || modelDraft === aiConfig.model) return;
    setAiSaveError(null);
    try {
      await ai.SetModel(modelDraft);
      await refreshAiConfig();
    } catch (err) {
      setAiSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleScopeToggle(scope: string, enabled: boolean) {
    if (!ai) return;
    setAiSaveError(null);
    try {
      await ai.SetScope(scope, enabled);
      await refreshAiConfig();
    } catch (err) {
      setAiSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleTestConnection() {
    if (!ai) return;
    setTesting(true);
    setTestResult(null);
    try {
      const result = await ai.TestConnection();
      setTestResult(result);
      setDiagnostics(await ai.Diagnostics());
    } catch (err) {
      setTestResult({ text: "", unavailable: true, reason: err instanceof Error ? err.message : String(err), latencyMs: 0 });
    } finally {
      setTesting(false);
    }
  }

  async function refreshShortcuts() {
    if (!shortcuts) return;
    const views = await shortcuts.ListShortcuts();
    setShortcutViews(views);
    setShortcutDrafts(Object.fromEntries(views.map((v) => [v.action, v.accelerator])));
  }

  // Commits a row's draft only if it actually changed from the last
  // saved value - mirrors commitEndpoint/commitModel's onBlur pattern
  // above. On failure, the draft text is left exactly as the person
  // typed it (not reverted) so they can see and fix what they entered;
  // only a successful SetShortcut or an explicit reset touches the
  // saved ShortcutView list.
  async function handleShortcutCommit(action: string) {
    if (!shortcuts) return;
    const draft = shortcutDrafts[action] ?? "";
    const saved = shortcutViews.find((v) => v.action === action)?.accelerator ?? "";
    if (draft === saved) return;
    setShortcutErrors((prev) => ({ ...prev, [action]: "" }));
    try {
      await shortcuts.SetShortcut(action, draft);
      await refreshShortcuts();
    } catch (err) {
      setShortcutErrors((prev) => ({ ...prev, [action]: err instanceof Error ? err.message : String(err) }));
    }
  }

  async function handleShortcutReset(action: string) {
    if (!shortcuts) return;
    setShortcutErrors((prev) => ({ ...prev, [action]: "" }));
    try {
      await shortcuts.ResetShortcut(action);
      await refreshShortcuts();
    } catch (err) {
      setShortcutErrors((prev) => ({ ...prev, [action]: err instanceof Error ? err.message : String(err) }));
    }
  }

  if (bindingsError) {
    return (
      <div className="page">
        <h1>{t("settings.pageHeading")}</h1>
        <p className="settings__error">
          {t("settings.settingsCantLoad", { error: bindingsError })}
        </p>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="page">
        <h1>{t("settings.pageHeading")}</h1>
        <p>{t("common.loading")}</p>
      </div>
    );
  }

  return (
    <div className="page">
      <h1>{t("settings.pageHeading")}</h1>
      {saveError && <p className="settings__error">{saveError}</p>}

      <section className="settings__section">
        <h2>{t("settings.closingWindowHeading")}</h2>
        <div className="settings__options">
          {CLOSE_OPTIONS.map((opt) => (
            <label key={opt.value} className="settings__radio">
              <input
                type="radio"
                name="close-behavior"
                checked={closeBehavior === opt.value}
                onChange={() => handleCloseBehaviorChange(opt.value)}
              />
              <span>
                <strong>{t(opt.labelKey)}</strong>
                <br />
                <span className="settings__hint">{t(opt.hintKey)}</span>
              </span>
            </label>
          ))}
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("settings.startupHeading")}</h2>
        <label className="settings__checkbox">
          <input
            type="checkbox"
            checked={startupEnabled}
            onChange={(e) => handleStartupToggle(e.target.checked)}
          />
          {t("settings.runOnStartupLabel")}
        </label>

        {startupEnabled && (
          <div className="settings__options settings__options--indented">
            {STARTUP_OPTIONS.map((opt) => (
              <label key={opt.value} className="settings__radio">
                <input
                  type="radio"
                  name="startup-behavior"
                  checked={startupBehavior === opt.value}
                  onChange={() => handleStartupBehaviorChange(opt.value)}
                />
                {t(opt.labelKey)}
              </label>
            ))}
          </div>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("settings.themeHeading")}</h2>
        {themeSwitchError && <p className="settings__error">{themeSwitchError}</p>}
        <label className="settings__select">
          <span>{t("settings.activeThemeLabel")}</span>
          <select value={themeCtx.activeThemeName} onChange={(e) => handleThemeChange(e.target.value)}>
            {themeCtx.themes.map((theme) => (
              <option key={theme.name} value={theme.name}>
                {theme.name}
                {theme.isBuiltin ? t("settings.builtinSuffix") : ""}
              </option>
            ))}
          </select>
        </label>
        <p className="settings__hint">
          {t("settings.themeStudioHintPrefix")} <Link to="/themes">{t("themes.heading")}</Link>.
        </p>
      </section>

      <section className="settings__section">
        <h2>{t("settings.languageHeading")}</h2>
        <label className="settings__select">
          <span>{t("settings.languageHeading")}</span>
          <select value={language} onChange={(e) => handleLanguageChange(e.target.value as SupportedLanguage)}>
            {SUPPORTED_LANGUAGES.map((lang) => (
              <option key={lang} value={lang}>
                {LANGUAGE_LABELS[lang]}
              </option>
            ))}
          </select>
        </label>
        {languageError && <p className="settings__error">{languageError}</p>}
        <p className="settings__hint">{t("settings.languageHint")}</p>
      </section>

      <section className="settings__section">
        <h2>{t("settings.weekStartHeading")}</h2>
        <label className="settings__select">
          <span>{t("settings.weekStartHeading")}</span>
          <select value={weekStart} onChange={(e) => handleWeekStartChange(e.target.value)}>
            {WEEK_START_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {t(opt.labelKey)}
              </option>
            ))}
          </select>
        </label>
        {weekStartError && <p className="settings__error">{weekStartError}</p>}
        <p className="settings__hint">{t("settings.weekStartHint")}</p>
      </section>

      <section className="settings__section">
        <h2>{t("settings.aiHeading")}</h2>
        <p className="settings__hint">{t("settings.aiIntro")}</p>

        {aiBindingsError && <p className="settings__error">{t("settings.aiLoadError", { error: aiBindingsError })}</p>}
        {aiSaveError && <p className="settings__error">{aiSaveError}</p>}

        {aiConfig && (
          <>
            <label className="settings__checkbox">
              <input
                type="checkbox"
                checked={aiConfig.enabled}
                onChange={(e) => handleAiEnabledToggle(e.target.checked)}
              />
              {t("settings.enableAiCheckbox")}
            </label>

            {aiConfig.enabled && (
              <div className="settings__options settings__options--indented">
                <label className="settings__select">
                  <span>{t("settings.providerLabel")}</span>
                  <select
                    value={aiConfig.provider === "none" ? "" : aiConfig.provider}
                    onChange={(e) => handleAiProviderChange(e.target.value)}
                  >
                    <option value="" disabled>
                      {t("settings.chooseProviderOption")}
                    </option>
                    {AI_PROVIDER_OPTIONS.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {t(opt.labelKey)}
                      </option>
                    ))}
                  </select>
                </label>

                {aiConfig.provider !== "none" && (
                  <>
                    <label className="settings__select">
                      <span>{t("settings.localServerAddressLabel")}</span>
                      <input
                        type="text"
                        value={endpointDraft}
                        placeholder={AI_PROVIDER_OPTIONS.find((o) => o.value === aiConfig.provider)?.endpointHint}
                        onChange={(e) => setEndpointDraft(e.target.value)}
                        onBlur={commitEndpoint}
                      />
                    </label>

                    <label className="settings__select">
                      <span>{t("settings.modelLabel")}</span>
                      <input
                        type="text"
                        value={modelDraft}
                        placeholder={t("settings.modelPlaceholder")}
                        onChange={(e) => setModelDraft(e.target.value)}
                        onBlur={commitModel}
                      />
                    </label>

                    <div>
                      <button onClick={handleTestConnection} disabled={testing}>
                        {testing ? t("settings.testingButton") : t("settings.testConnectionButton")}
                      </button>
                      {testResult && (
                        <p className={testResult.unavailable ? "settings__error" : "settings__hint"}>
                          {testResult.unavailable
                            ? t("settings.notAvailablePrefix", { reason: testResult.reason })
                            : t("settings.connectedMessage", { latency: testResult.latencyMs, text: testResult.text })}
                        </p>
                      )}
                      {diagnostics?.hasRun && (
                        <p className="settings__hint">
                          {t("settings.lastAiCallPrefix", {
                            status: diagnostics.success ? t("settings.succeededWord") : t("settings.failedWord", { reason: diagnostics.reason }),
                            scope: AI_SCOPE_LABEL_KEYS[diagnostics.scope] ? t(AI_SCOPE_LABEL_KEYS[diagnostics.scope]) : diagnostics.scope,
                            latency: diagnostics.latencyMs,
                          })}
                        </p>
                      )}
                    </div>

                    <div>
                      <p className="settings__hint" style={{ marginBottom: "var(--space-2)" }}>
                        {t("settings.aiScopesIntro")}
                      </p>
                      {AI_SCOPES.map((scope) => (
                        <label key={scope} className="settings__checkbox">
                          <input
                            type="checkbox"
                            checked={aiConfig.scopes[scope] ?? false}
                            onChange={(e) => handleScopeToggle(scope, e.target.checked)}
                          />
                          {t(AI_SCOPE_LABEL_KEYS[scope])}
                        </label>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}
          </>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("settings.shortcutsHeading")}</h2>
        <p className="settings__hint">{t("settings.shortcutsIntro")}</p>

        {shortcutsBindingsError && (
          <p className="settings__error">{t("settings.shortcutsLoadError", { error: shortcutsBindingsError })}</p>
        )}

        {shortcutViews.length > 0 && (
          <div className="settings__shortcuts">
            {shortcutViews.map((view) => (
              <div key={view.action} className="settings__shortcut-row">
                <span className="settings__shortcut-label">
                  {t(SHORTCUT_LABEL_KEYS[view.action] ?? view.action)}
                </span>
                <input
                  type="text"
                  className="settings__shortcut-input"
                  value={shortcutDrafts[view.action] ?? ""}
                  onChange={(e) =>
                    setShortcutDrafts((prev) => ({ ...prev, [view.action]: e.target.value }))
                  }
                  onBlur={() => handleShortcutCommit(view.action)}
                  aria-label={t(SHORTCUT_LABEL_KEYS[view.action] ?? view.action)}
                />
                {view.isCustom && (
                  <button onClick={() => handleShortcutReset(view.action)}>
                    {t("settings.shortcutResetButton")}
                  </button>
                )}
                {shortcutErrors[view.action] && (
                  <p className="settings__error settings__shortcut-error">{shortcutErrors[view.action]}</p>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
