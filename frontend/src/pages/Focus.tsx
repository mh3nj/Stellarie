import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import {
  getFocusBindings,
  getTimerBindings,
  getProjectBindings,
  type FocusView,
  type ProjectRecord,
} from "@/lib/bindings";
import { formatClock, formatDuration } from "@/lib/time-ranges";

function parseAppList(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

export function Focus() {
  const { t } = useTranslation();
  const [active, setActive] = useState<FocusView | null>(null);
  const [history, setHistory] = useState<FocusView[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [, forceTick] = useState(0);
  const fetchedAtMs = useRef(Date.now());

  const [projectId, setProjectId] = useState(0);
  const [label, setLabel] = useState("");
  const [durationMinutes, setDurationMinutes] = useState(45);
  const [openEnded, setOpenEnded] = useState(false);
  const [blockedAppsRaw, setBlockedAppsRaw] = useState("");
  const [suppressNotifications, setSuppressNotifications] = useState(true);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [focusBindings, projectBindings] = await Promise.all([getFocusBindings(), getProjectBindings()]);
      const [activeView, historyRows, projectRows] = await Promise.all([
        focusBindings.GetActive(),
        focusBindings.History(10),
        projectBindings.ListProjects(false),
      ]);
      setActive(activeView);
      setHistory(historyRows);
      setProjects(projectRows);
      setProjectId((prev) => prev || projectRows[0]?.id || 0);
      fetchedAtMs.current = Date.now();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Live per-second tick between fetches, same reasoning as the Timers
  // page - the backend computes elapsed/remaining/quality fresh at
  // request time, not pushed every second.
  useEffect(() => {
    const id = setInterval(() => forceTick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, []);

  // A focus timer completing (its target duration reached) fires the
  // same "timer:completed" event every other timer type does - reload
  // right away so the session moves from "active" to "history"
  // immediately rather than on the next visit.
  useEffect(() => {
    return Events.On("timer:completed", () => load());
  }, [load]);

  function liveView(view: FocusView): FocusView {
    if (view.status !== "running") return view;
    const driftSecs = (Date.now() - fetchedAtMs.current) / 1000;
    return {
      ...view,
      elapsedSecs: view.elapsedSecs + driftSecs,
      remainingSecs: view.durationSecs > 0 ? Math.max(0, view.remainingSecs - driftSecs) : 0,
    };
  }

  async function withReload(action: () => Promise<unknown>) {
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleStart() {
    if (!projectId) {
      setError(t("focus.pickProjectError"));
      return;
    }
    await withReload(async () => {
      const bindings = await getFocusBindings();
      await bindings.Start(
        projectId,
        label.trim(),
        openEnded ? 0 : durationMinutes,
        parseAppList(blockedAppsRaw),
        [],
        suppressNotifications,
      );
      setLabel("");
    });
  }

  function projectName(id: number): string {
    return projects.find((p) => p.id === id)?.name ?? "";
  }

  const view = active ? liveView(active) : null;

  return (
    <div className="page page--wide">
      <h1>{t("focus.heading")}</h1>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : view ? (
        <div className="focus-hero">
          <span className="focus-hero__label">
            {view.status === "paused" ? t("focus.focusPausedLabel") : t("focus.focusLabel")}
          </span>
          <span className="focus-hero__project">
            {projectName(view.projectId)}
            {view.label && ` · ${view.label}`}
          </span>
          <span className="focus-hero__clock">
            {formatClock(view.durationSecs > 0 ? view.remainingSecs : view.elapsedSecs)}
          </span>

          {view.qualityKnown ? (
            <>
              <span className="focus-hero__quality">{t("focus.qualityLabel", { percent: Math.round(view.qualityPercent) })}</span>
              <div className="focus-hero__quality-bar">
                <div className="focus-hero__quality-fill" style={{ width: `${view.qualityPercent}%` }} />
              </div>
            </>
          ) : (
            <span className="focus-hero__quality settings__hint">{t("focus.notEnoughData")}</span>
          )}

          {view.currentAppBlocked && (
            <div className="focus-hero__warning">{t("focus.blockedWarning")}</div>
          )}

          <div className="focus-stats">
            <span>{t("focus.onTarget", { duration: formatDuration(view.onTargetSecs) })}</span>
            <span>{t("focus.tracked", { duration: formatDuration(view.trackedSecs) })}</span>
            {view.blockedAppSecs > 0 && <span>{t("focus.blockedApps", { duration: formatDuration(view.blockedAppSecs) })}</span>}
          </div>

          <div className="focus-hero__actions">
            {view.status === "running" ? (
              <button onClick={() => withReload(async () => (await getTimerBindings()).Pause(view.id))}>
                {t("focus.pauseButton")}
              </button>
            ) : (
              <button onClick={() => withReload(async () => (await getTimerBindings()).Resume(view.id))}>
                {t("focus.resumeButton")}
              </button>
            )}
            <button onClick={() => withReload(async () => (await getTimerBindings()).Finish(view.id))}>
              {t("focus.finishButton")}
            </button>
            <button onClick={() => withReload(async () => (await getTimerBindings()).Cancel(view.id))}>
              {t("focus.cancelButton")}
            </button>
          </div>
        </div>
      ) : (
        <section className="settings__section focus-form">
          <h2>{t("focus.startSessionHeading")}</h2>
          <div className="project-form">
            <select value={projectId} onChange={(e) => setProjectId(Number(e.target.value))}>
              <option value={0}>{t("focus.chooseProjectOption")}</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
            <input placeholder={t("focus.labelPlaceholder")} value={label} onChange={(e) => setLabel(e.target.value)} />
          </div>

          <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
            <label className="focus-form__checkbox">
              <input type="checkbox" checked={openEnded} onChange={(e) => setOpenEnded(e.target.checked)} />
              {t("focus.noTimeLimit")}
            </label>
            {!openEnded && (
              <input
                type="number"
                min={1}
                value={durationMinutes}
                onChange={(e) => setDurationMinutes(Number(e.target.value))}
                style={{ width: 70 }}
                title={t("focus.durationTitle")}
              />
            )}
          </div>

          <div style={{ marginTop: "var(--space-3)" }}>
            <p className="settings__hint">{t("focus.blockedAppsHint")}</p>
            <textarea
              value={blockedAppsRaw}
              onChange={(e) => setBlockedAppsRaw(e.target.value)}
              placeholder={t("focus.blockedAppsPlaceholder")}
            />
          </div>

          <label className="focus-form__checkbox" style={{ marginTop: "var(--space-3)" }}>
            <input
              type="checkbox"
              checked={suppressNotifications}
              onChange={(e) => setSuppressNotifications(e.target.checked)}
            />
            {t("focus.suppressNotifications")}
          </label>

          <div style={{ marginTop: "var(--space-4)" }}>
            <button onClick={handleStart}>{t("focus.startButton")}</button>
          </div>
        </section>
      )}

      <section className="settings__section">
        <h2>{t("focus.recentSessionsHeading")}</h2>
        {history.length === 0 ? (
          <p className="settings__hint">{t("focus.noFocusSessions")}</p>
        ) : (
          <div className="alarm-list">
            {history.map((h) => (
              <div key={h.id} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">
                    {projectName(h.projectId)}
                    {h.label && ` · ${h.label}`}
                  </div>
                  <div className="alarm-row__meta">
                    {formatDuration(h.elapsedSecs)} · {h.status}
                    {h.qualityKnown && ` · ${t("focus.qualitySuffix", { percent: Math.round(h.qualityPercent) })}`}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
