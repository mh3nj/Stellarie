import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import { getTimerBindings, getProjectBindings, type TimerView, type ProjectRecord } from "@/lib/bindings";
import { formatClock } from "@/lib/time-ranges";

// TypeService.StartCountdown's own accepted set (see
// app/services/timer_service.go) - "pomodoro" and "stopwatch" are
// deliberately excluded here since they have their own dedicated
// start buttons below with different parameters.
const COUNTDOWN_TYPES = ["countdown", "focus", "project", "custom", "interval"];

export function Timers() {
  const { t } = useTranslation();

  function pomodoroPhaseLabel(view: TimerView): string {
    if (view.timerType !== "pomodoro" || !view.pomodoroPhase) return "";
    switch (view.pomodoroPhase) {
      case "work":
        return t("timers.phaseFocus");
      case "short_break":
        return t("timers.phaseShortBreak");
      case "long_break":
        return t("timers.phaseLongBreak");
      default:
        return view.pomodoroPhase;
    }
  }

  const [active, setActive] = useState<TimerView[]>([]);
  const [recent, setRecent] = useState<TimerView[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [, forceTick] = useState(0);
  const fetchedAtMs = useRef(Date.now());

  const [label, setLabel] = useState("");
  const [projectId, setProjectId] = useState(0);
  const [countdownType, setCountdownType] = useState(COUNTDOWN_TYPES[0]);
  const [durationMinutes, setDurationMinutes] = useState(25);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [timerBindings, projectBindings] = await Promise.all([getTimerBindings(), getProjectBindings()]);
      const [activeRows, recentRows, projectRows] = await Promise.all([
        timerBindings.ListActive(),
        timerBindings.ListRecent(10),
        projectBindings.ListProjects(false),
      ]);
      setActive(activeRows);
      setRecent(recentRows);
      setProjects(projectRows);
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

  // Purely client-side second-by-second tick between fetches. The
  // backend only computes elapsed/remaining fresh at request time (see
  // TimerService.toTimerView's own doc comment) - a live-feeling
  // countdown here is the frontend's job, not something the server
  // pushes every second.
  useEffect(() => {
    const id = setInterval(() => forceTick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, []);

  // main.go's Scheduler emits "timer:completed" the moment it notices a
  // countdown/Pomodoro timer reach its target - reload right away rather
  // than waiting for the next visit to this page, so a just-finished
  // timer (and, for Pomodoro, the break it auto-started) shows up
  // immediately.
  useEffect(() => {
    return Events.On("timer:completed", () => load());
  }, [load]);

  function liveView(view: TimerView): TimerView {
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

  async function handleStartStopwatch() {
    await withReload(async () => {
      const bindings = await getTimerBindings();
      await bindings.StartStopwatch(projectId, label.trim());
      setLabel("");
    });
  }

  async function handleStartCountdown() {
    await withReload(async () => {
      const bindings = await getTimerBindings();
      await bindings.StartCountdown(countdownType, projectId, label.trim(), durationMinutes);
      setLabel("");
    });
  }

  async function handleStartPomodoro() {
    await withReload(async () => {
      const bindings = await getTimerBindings();
      await bindings.StartPomodoro(projectId, label.trim(), 25, 5, 15, 4);
      setLabel("");
    });
  }

  function projectName(id: number): string {
    return projects.find((p) => p.id === id)?.name ?? "";
  }

  return (
    <div className="page page--wide">
      <h1>{t("timers.heading")}</h1>

      {error && <p className="settings__error">{error}</p>}

      <section className="settings__section">
        <h2>{t("timers.activeHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : active.length === 0 ? (
          <p className="settings__hint">{t("timers.nothingRunning")}</p>
        ) : (
          <div className="timer-grid">
            {active.map((raw) => {
              const view = liveView(raw);
              const percent = view.durationSecs > 0 ? Math.min(100, (view.elapsedSecs / view.durationSecs) * 100) : 0;
              const phase = pomodoroPhaseLabel(view);
              return (
                <div key={view.id} className={`timer-card${view.status === "paused" ? " timer-card--paused" : ""}`}>
                  <span className="timer-card__type">
                    {view.timerType}
                    {phase && ` · ${phase}`}
                    {view.status === "paused" && ` · ${t("timers.pausedLabel")}`}
                  </span>
                  <span className="timer-card__label">
                    {view.label || t("timers.untitledLabel")}
                    {view.projectId > 0 && ` · ${projectName(view.projectId)}`}
                  </span>
                  <span className="timer-card__clock">
                    {formatClock(view.durationSecs > 0 ? view.remainingSecs : view.elapsedSecs)}
                  </span>
                  {view.durationSecs > 0 && (
                    <div className="timer-card__bar">
                      <div className="timer-card__bar-fill" style={{ width: `${percent}%` }} />
                    </div>
                  )}
                  <div className="timer-card__actions">
                    {view.status === "running" ? (
                      <button onClick={() => withReload(async () => (await getTimerBindings()).Pause(view.id))}>
                        {t("timers.pauseButton")}
                      </button>
                    ) : (
                      <button onClick={() => withReload(async () => (await getTimerBindings()).Resume(view.id))}>
                        {t("timers.resumeButton")}
                      </button>
                    )}
                    <button onClick={() => withReload(async () => (await getTimerBindings()).Finish(view.id))}>
                      {t("timers.finishButton")}
                    </button>
                    <button onClick={() => withReload(async () => (await getTimerBindings()).Cancel(view.id))}>
                      {t("timers.cancelButton")}
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("timers.startTimerHeading")}</h2>
        <div className="project-form">
          <input placeholder={t("timers.labelPlaceholder")} value={label} onChange={(e) => setLabel(e.target.value)} />
          <select value={projectId} onChange={(e) => setProjectId(Number(e.target.value))}>
            <option value={0}>{t("timers.noProjectOption")}</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </div>

        <div className="timer-launcher" style={{ marginTop: "var(--space-3)" }}>
          <button onClick={handleStartStopwatch}>{t("timers.startStopwatchButton")}</button>
          <button onClick={handleStartPomodoro}>{t("timers.startPomodoroButton")}</button>
        </div>

        <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
          <select value={countdownType} onChange={(e) => setCountdownType(e.target.value)}>
            {COUNTDOWN_TYPES.map((ct) => (
              <option key={ct} value={ct}>
                {ct}
              </option>
            ))}
          </select>
          <input
            type="number"
            min={1}
            value={durationMinutes}
            onChange={(e) => setDurationMinutes(Number(e.target.value))}
            style={{ width: 70 }}
            title={t("timers.durationTitle")}
          />
          <button onClick={handleStartCountdown}>{t("timers.startCountdownButton")}</button>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("timers.recentHeading")}</h2>
        {recent.length === 0 ? (
          <p className="settings__hint">{t("timers.nothingCompleted")}</p>
        ) : (
          <div className="alarm-list">
            {recent.map((r) => (
              <div key={r.id} className="alarm-row">
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{r.label || r.timerType}</div>
                  <div className="alarm-row__meta">
                    {r.timerType} · {r.status} · {formatClock(r.elapsedSecs)}
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
