import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  getPrivacyBindings,
  getSettingsBindings,
  type ExclusionView,
  type ApplicationPrivacyView,
  type ScreenshotView,
} from "@/lib/bindings";

const RETENTION_OPTIONS = [
  { value: 1, labelKey: "privacy.retention1Day" },
  { value: 7, labelKey: "privacy.retention7Days" },
  { value: 30, labelKey: "privacy.retention30Days" },
  { value: 90, labelKey: "privacy.retention90Days" },
  { value: 0, labelKey: "privacy.retentionNever" },
];

const INTERVAL_OPTIONS = [
  { value: 60, labelKey: "privacy.interval1Min" },
  { value: 300, labelKey: "privacy.interval5Min" },
  { value: 600, labelKey: "privacy.interval10Min" },
  { value: 900, labelKey: "privacy.interval15Min" },
];

const EXCLUSION_TYPES = [
  { value: "application", labelKey: "privacy.exclusionTypeApplication" },
  { value: "window_title_pattern", labelKey: "privacy.exclusionTypeWindowTitle" },
];

const APPLIES_TO_OPTIONS = [
  { value: "all", labelKey: "privacy.appliesToAll" },
  { value: "screenshots", labelKey: "privacy.appliesToScreenshots" },
  { value: "browser_tracking", labelKey: "privacy.appliesToBrowser" },
];

function formatDateTime(unixSeconds?: number): string {
  if (!unixSeconds) return "-";
  return new Date(unixSeconds * 1000).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function Privacy() {
  const { t } = useTranslation();
  const [privacyMode, setPrivacyMode] = useState(false);
  const [screenshotEnabled, setScreenshotEnabled] = useState(false);
  const [screenshotInterval, setScreenshotInterval] = useState(300);
  const [screenshotRetention, setScreenshotRetention] = useState(7);
  const [browserTrackingEnabled, setBrowserTrackingEnabled] = useState(false);
  const [exclusions, setExclusions] = useState<ExclusionView[]>([]);
  const [applications, setApplications] = useState<ApplicationPrivacyView[]>([]);
  const [screenshots, setScreenshots] = useState<ScreenshotView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [exclusionType, setExclusionType] = useState(EXCLUSION_TYPES[0].value);
  const [exclusionPattern, setExclusionPattern] = useState("");
  const [exclusionAppliesTo, setExclusionAppliesTo] = useState("all");

  const load = useCallback(async () => {
    setError(null);
    try {
      const [privacyBindings, settingsBindings] = await Promise.all([getPrivacyBindings(), getSettingsBindings()]);
      const [mode, shotEnabled, interval, retention, browserEnabled, exclusionRows, appRows, screenshotRows] =
        await Promise.all([
          privacyBindings.GetPrivacyMode(),
          settingsBindings.GetBool("screenshot_enabled"),
          settingsBindings.GetInt("screenshot_interval_secs", 300),
          settingsBindings.GetInt("screenshot_retention_days", 7),
          settingsBindings.GetBool("browser_tracking_enabled"),
          privacyBindings.ListExclusions(),
          privacyBindings.ListApplicationPrivacy(),
          privacyBindings.ListScreenshots(50),
        ]);
      setPrivacyMode(mode);
      setScreenshotEnabled(shotEnabled);
      setScreenshotInterval(interval);
      setScreenshotRetention(retention);
      setBrowserTrackingEnabled(browserEnabled);
      setExclusions(exclusionRows);
      setApplications(appRows);
      setScreenshots(screenshotRows);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function withReload(action: () => Promise<unknown>) {
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function togglePrivacyMode() {
    await withReload(async () => (await getPrivacyBindings()).SetPrivacyMode(!privacyMode));
  }

  async function toggleScreenshotEnabled() {
    await withReload(async () => (await getSettingsBindings()).SetBool("screenshot_enabled", !screenshotEnabled));
  }

  async function changeInterval(value: number) {
    await withReload(async () => (await getSettingsBindings()).SetInt("screenshot_interval_secs", value));
  }

  async function changeRetention(value: number) {
    await withReload(async () => (await getSettingsBindings()).SetInt("screenshot_retention_days", value));
  }

  async function toggleBrowserTracking() {
    await withReload(async () =>
      (await getSettingsBindings()).SetBool("browser_tracking_enabled", !browserTrackingEnabled),
    );
  }

  async function handleAddExclusion() {
    if (!exclusionPattern.trim()) {
      setError(t("privacy.enterPatternError"));
      return;
    }
    await withReload(async () => {
      const bindings = await getPrivacyBindings();
      await bindings.CreateExclusion(exclusionType, exclusionPattern.trim(), exclusionAppliesTo);
      setExclusionPattern("");
    });
  }

  return (
    <div className="page page--wide">
      <h1>{t("privacy.heading")}</h1>
      {error && <p className="settings__error">{error}</p>}

      <section className="settings__section focus-hero" style={{ padding: "var(--space-5)" }}>
        <span className="focus-hero__label">{privacyMode ? t("privacy.privacyModeActive") : t("privacy.privacyModeLabel")}</span>
        <p className="settings__hint" style={{ textAlign: "center", maxWidth: 480 }}>
          {t("privacy.privacyModeHint")}
        </p>
        <div className="focus-hero__actions">
          <button onClick={togglePrivacyMode}>{privacyMode ? t("privacy.turnOffButton") : t("privacy.turnOnButton")}</button>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("privacy.screenshotMemoryHeading")}</h2>
        <p className="settings__hint">{t("privacy.screenshotMemoryHint")}</p>
        <div className="project-form">
          <label className="focus-form__checkbox">
            <input type="checkbox" checked={screenshotEnabled} onChange={toggleScreenshotEnabled} />
            {t("privacy.enabledLabel")}
          </label>
          <select
            value={screenshotInterval}
            onChange={(e) => changeInterval(Number(e.target.value))}
            disabled={!screenshotEnabled}
          >
            {INTERVAL_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {t("privacy.everyPrefix", { interval: t(o.labelKey) })}
              </option>
            ))}
          </select>
          <select
            value={screenshotRetention}
            onChange={(e) => changeRetention(Number(e.target.value))}
            disabled={!screenshotEnabled}
          >
            {RETENTION_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {t("privacy.keepForPrefix", { duration: t(o.labelKey) })}
              </option>
            ))}
          </select>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("privacy.browserTrackingHeading")}</h2>
        <p className="settings__hint">{t("privacy.browserTrackingHint")}</p>
        <label className="focus-form__checkbox">
          <input type="checkbox" checked={browserTrackingEnabled} onChange={toggleBrowserTracking} />
          {t("privacy.enabledLabel")}
        </label>
      </section>

      <section className="settings__section">
        <h2>{t("privacy.applicationsHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : applications.length === 0 ? (
          <p className="settings__hint">{t("analytics.noApplicationsTracked")}</p>
        ) : (
          <div className="alarm-list">
            {applications.map((a) => (
              <div key={a.applicationId} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{a.processName}</div>
                  <div className="alarm-row__meta">{a.privacySensitive ? t("privacy.markedSensitive") : t("privacy.notSensitive")}</div>
                </div>
                <div className="alarm-row__actions" style={{ flexWrap: "wrap", maxWidth: 360 }}>
                  <label className="focus-form__checkbox">
                    <input
                      type="checkbox"
                      checked={a.trackingEnabled}
                      onChange={() =>
                        withReload(async () =>
                          (await getPrivacyBindings()).SetApplicationPrivacy(
                            a.applicationId,
                            !a.trackingEnabled,
                            a.screenshotAllowed,
                            a.browserUrlTracking,
                            a.privacySensitive,
                          ),
                        )
                      }
                    />
                    {t("privacy.trackCheckbox")}
                  </label>
                  <label className="focus-form__checkbox">
                    <input
                      type="checkbox"
                      checked={a.screenshotAllowed}
                      onChange={() =>
                        withReload(async () =>
                          (await getPrivacyBindings()).SetApplicationPrivacy(
                            a.applicationId,
                            a.trackingEnabled,
                            !a.screenshotAllowed,
                            a.browserUrlTracking,
                            a.privacySensitive,
                          ),
                        )
                      }
                    />
                    {t("privacy.screenshotsCheckbox")}
                  </label>
                  <label className="focus-form__checkbox">
                    <input
                      type="checkbox"
                      checked={a.privacySensitive}
                      onChange={() =>
                        withReload(async () =>
                          (await getPrivacyBindings()).SetApplicationPrivacy(
                            a.applicationId,
                            a.trackingEnabled,
                            a.screenshotAllowed,
                            a.browserUrlTracking,
                            !a.privacySensitive,
                          ),
                        )
                      }
                    />
                    {t("privacy.sensitiveCheckbox")}
                  </label>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("privacy.exclusionsHeading")}</h2>
        {exclusions.length === 0 ? (
          <p className="settings__hint">{t("privacy.noExclusionRules")}</p>
        ) : (
          <div className="alarm-list">
            {exclusions.map((ex) => (
              <div key={ex.id} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{ex.pattern}</div>
                  <div className="alarm-row__meta">
                    {t("privacy.appliesToLine", { type: ex.type, appliesTo: ex.appliesTo })}
                  </div>
                </div>
                <div className="alarm-row__actions">
                  <button onClick={() => withReload(async () => (await getPrivacyBindings()).DeleteExclusion(ex.id))}>
                    {t("privacy.deleteButton")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
        <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
          <select value={exclusionType} onChange={(e) => setExclusionType(e.target.value)}>
            {EXCLUSION_TYPES.map((et) => (
              <option key={et.value} value={et.value}>
                {t(et.labelKey)}
              </option>
            ))}
          </select>
          <input
            placeholder={exclusionType === "application" ? t("privacy.exclusionPatternAppPlaceholder") : t("privacy.exclusionPatternTitlePlaceholder")}
            value={exclusionPattern}
            onChange={(e) => setExclusionPattern(e.target.value)}
          />
          <select value={exclusionAppliesTo} onChange={(e) => setExclusionAppliesTo(e.target.value)}>
            {APPLIES_TO_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {t(o.labelKey)}
              </option>
            ))}
          </select>
          <button onClick={handleAddExclusion}>{t("privacy.addButton")}</button>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("privacy.screenshotsHeading")}</h2>
        {screenshots.length === 0 ? (
          <p className="settings__hint">{t("privacy.noScreenshotsYet")}</p>
        ) : (
          <div className="alarm-list">
            {screenshots.map((sc) => (
              <div key={sc.id} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{formatDateTime(sc.takenAt)}</div>
                  <div className="alarm-row__meta">
                    {sc.filePath}
                    {sc.deleteAfter ? t("privacy.deletesAtSuffix", { when: formatDateTime(sc.deleteAfter) }) : t("privacy.keptForeverSuffix")}
                  </div>
                </div>
                <div className="alarm-row__actions">
                  <button onClick={() => withReload(async () => (await getPrivacyBindings()).DeleteScreenshot(sc.id))}>
                    {t("privacy.deleteButton")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
