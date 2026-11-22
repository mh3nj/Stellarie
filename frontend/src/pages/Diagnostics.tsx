import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getDiagnosticsBindings, type DiagnosticsView } from "@/lib/bindings";

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (h === 0) return `${m}m`;
  return `${h}h ${m}m`;
}

export function Diagnostics() {
  const { t } = useTranslation();
  const [view, setView] = useState<DiagnosticsView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const bindings = await getDiagnosticsBindings();
      setView(await bindings.GetDiagnostics());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="page">
      <h1>{t("diagnostics.heading")}</h1>
      <p className="settings__hint">{t("diagnostics.hint")}</p>

      <button onClick={load} disabled={loading}>
        {loading ? t("common.refreshing") : t("common.refresh")}
      </button>

      {error && <p className="settings__error">{error}</p>}

      {view && (
        <>
          <section className="settings__section">
            <h2>{t("diagnostics.applicationHeading")}</h2>
            <ul className="rule-list">
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.versionLabel")}</span>
                <span>{view.appVersion}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.platformLabel")}</span>
                <span>
                  {view.os} / {view.arch}
                </span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.processUptimeLabel")}</span>
                <span>{formatUptime(view.processUptimeSeconds)}</span>
              </li>
            </ul>
          </section>

          <section className="settings__section">
            <h2>{t("diagnostics.databaseHeading")}</h2>
            <ul className="rule-list">
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.reachableLabel")}</span>
                <span>{view.databaseReachable ? t("common.yes") : t("common.no")}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.sizeLabel")}</span>
                <span>{formatBytes(view.databaseSizeBytes)}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.eventsStoredLabel")}</span>
                <span>{view.eventsStored.toLocaleString()}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.sessionsStoredLabel")}</span>
                <span>{view.sessionsStored.toLocaleString()}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.pathLabel")}</span>
                <span>{view.databasePath}</span>
              </li>
            </ul>
          </section>

          <section className="settings__section">
            <h2>{t("diagnostics.activityEngineHeading")}</h2>
            <ul className="rule-list">
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.statusLabel")}</span>
                <span>{view.activityEngineRunning ? t("diagnostics.runningValue") : t("diagnostics.pausedValue")}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.idleLabel")}</span>
                <span>{view.activityEngineIdle ? t("common.yes") : t("common.no")}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.privacyModeLabel")}</span>
                <span>{view.activityEnginePrivacyMode ? t("diagnostics.activeValue") : t("diagnostics.offValue")}</span>
              </li>
            </ul>
          </section>

          <section className="settings__section">
            <h2>{t("diagnostics.startupHeading")}</h2>
            <ul className="rule-list">
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.startupRegisteredLabel")}</span>
                <span>
                  {view.startupCheckError
                    ? t("diagnostics.startupCheckErrorValue", { error: view.startupCheckError })
                    : view.startupRegistered
                      ? t("common.yes")
                      : t("common.no")}
                </span>
              </li>
            </ul>
          </section>

          <section className="settings__section">
            <h2>{t("diagnostics.performanceHeading")}</h2>
            <ul className="rule-list">
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.goroutinesLabel")}</span>
                <span>{view.goroutineCount}</span>
              </li>
              <li className="rule-list__item">
                <span className="rule-list__pattern">{t("diagnostics.memoryLabel")}</span>
                <span>{view.memoryAllocMb.toFixed(1)} MB</span>
              </li>
            </ul>
          </section>
        </>
      )}
    </div>
  );
}
