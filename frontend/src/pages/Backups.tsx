import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getBackupBindings, type BackupInfoView } from "@/lib/bindings";

const DATASET_KEYS: { value: string; labelKey: string }[] = [
  { value: "activities", labelKey: "backups.datasetActivities" },
  { value: "sessions", labelKey: "backups.datasetSessions" },
  { value: "projects", labelKey: "backups.datasetProjects" },
  { value: "goals", labelKey: "backups.datasetGoals" },
  { value: "budgets", labelKey: "backups.datasetBudgets" },
  { value: "analytics", labelKey: "backups.datasetAnalytics" },
];

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDateTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function Backups() {
  const { t } = useTranslation();
  const [backups, setBackups] = useState<BackupInfoView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [includeScreenshots, setIncludeScreenshots] = useState(false);
  const [backingUp, setBackingUp] = useState(false);

  const [confirmingRestore, setConfirmingRestore] = useState<string | null>(null);
  const [restoreStaged, setRestoreStaged] = useState(false);
  const [restarting, setRestarting] = useState(false);

  const [dataset, setDataset] = useState(DATASET_KEYS[0].value);
  const [format, setFormat] = useState<"json" | "csv">("json");
  const [exporting, setExporting] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      const bindings = await getBackupBindings();
      setBackups(await bindings.ListBackups());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleBackupNow() {
    setBackingUp(true);
    setError(null);
    setNotice(null);
    try {
      const bindings = await getBackupBindings();
      const info = await bindings.BackupNow(includeScreenshots);
      setNotice(t("backups.backupSavedNotice", { filename: info.filename, size: formatBytes(info.sizeBytes) }));
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBackingUp(false);
    }
  }

  async function handleDelete(path: string) {
    setError(null);
    try {
      const bindings = await getBackupBindings();
      await bindings.DeleteBackup(path);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleConfirmRestore(path: string) {
    setError(null);
    try {
      const bindings = await getBackupBindings();
      await bindings.RestoreBackup(path);
      setConfirmingRestore(null);
      setRestoreStaged(true);
      setNotice(t("backups.restoreStagedNotice"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleRestartNow() {
    setRestarting(true);
    setError(null);
    try {
      const bindings = await getBackupBindings();
      await bindings.RestartToApplyRestore();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setRestarting(false);
    }
  }

  async function handleExportSQLite() {
    setExporting(true);
    setError(null);
    setNotice(null);
    try {
      const bindings = await getBackupBindings();
      const path = await bindings.ExportSQLite();
      setNotice(t("backups.exportedNotice", { path }));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setExporting(false);
    }
  }

  async function handleExportDataset() {
    setExporting(true);
    setError(null);
    setNotice(null);
    try {
      const bindings = await getBackupBindings();
      const path = await bindings.Export(dataset, format);
      setNotice(t("backups.exportedNotice", { path }));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setExporting(false);
    }
  }

  if (restoreStaged) {
    return (
      <div className="page page--wide">
        <h1>{t("backups.heading")}</h1>
        <section className="settings__section focus-hero" style={{ padding: "var(--space-5)" }}>
          <span className="focus-hero__label">{t("backups.restoreStagedLabel")}</span>
          <p className="settings__hint" style={{ textAlign: "center", maxWidth: 480 }}>
            {t("backups.restoreStagedHint")}
          </p>
          <div className="focus-hero__actions">
            <button onClick={handleRestartNow} disabled={restarting}>
              {restarting ? t("backups.restartingButton") : t("backups.restartNowButton")}
            </button>
            <button onClick={() => setRestoreStaged(false)}>{t("backups.notNowButton")}</button>
          </div>
        </section>
        {error && <p className="settings__error">{error}</p>}
      </div>
    );
  }

  return (
    <div className="page page--wide">
      <h1>{t("backups.heading")}</h1>
      {error && <p className="settings__error">{error}</p>}
      {notice && <p className="settings__hint">{notice}</p>}

      <section className="settings__section">
        <h2>{t("backups.backupNowHeading")}</h2>
        <div className="project-form">
          <label className="focus-form__checkbox">
            <input
              type="checkbox"
              checked={includeScreenshots}
              onChange={(e) => setIncludeScreenshots(e.target.checked)}
            />
            {t("backups.includeScreenshots")}
          </label>
          <button onClick={handleBackupNow} disabled={backingUp}>
            {backingUp ? t("backups.backingUpButton") : t("backups.backupNowButton")}
          </button>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("backups.yourBackupsHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : backups.length === 0 ? (
          <p className="settings__hint">{t("backups.noBackupsYet")}</p>
        ) : (
          <div className="alarm-list">
            {backups.map((b) => (
              <div key={b.path} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{formatDateTime(b.createdAt)}</div>
                  <div className="alarm-row__meta">
                    {formatBytes(b.sizeBytes)}
                    {b.includesScreenshots ? ` · ${t("backups.screenshotsIncludedSuffix")}` : ""}
                  </div>
                </div>
                {confirmingRestore === b.path ? (
                  <div className="alarm-row__actions">
                    <span className="settings__hint">{t("backups.restoreConfirmPrompt")}</span>
                    <button onClick={() => handleConfirmRestore(b.path)}>{t("backups.confirmButton")}</button>
                    <button onClick={() => setConfirmingRestore(null)}>{t("backups.cancelButton")}</button>
                  </div>
                ) : (
                  <div className="alarm-row__actions">
                    <button onClick={() => setConfirmingRestore(b.path)}>{t("backups.restoreButton")}</button>
                    <button onClick={() => handleDelete(b.path)}>{t("backups.deleteButton")}</button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("backups.exportHeading")}</h2>
        <p className="settings__hint">{t("backups.exportHint")}</p>
        <div className="project-form">
          <button onClick={handleExportSQLite} disabled={exporting}>
            {t("backups.exportDatabaseButton")}
          </button>
        </div>
        <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
          <select value={dataset} onChange={(e) => setDataset(e.target.value)}>
            {DATASET_KEYS.map((d) => (
              <option key={d.value} value={d.value}>
                {t(d.labelKey)}
              </option>
            ))}
          </select>
          <select value={format} onChange={(e) => setFormat(e.target.value as "json" | "csv")}>
            <option value="json">JSON</option>
            <option value="csv">CSV</option>
          </select>
          <button onClick={handleExportDataset} disabled={exporting}>
            {exporting ? t("backups.exportingButton") : t("backups.exportButton")}
          </button>
        </div>
      </section>
    </div>
  );
}
