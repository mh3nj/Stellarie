import { useEffect, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Events } from "@wailsio/runtime";
import { getActivityBindings, type ActivityStatus } from "@/lib/bindings";

const QUICK_LINKS = [
  { to: "/receipt", key: "overview.quickLinkReceipt" },
  { to: "/patterns", key: "overview.quickLinkPatterns" },
  { to: "/digital-waste", key: "overview.quickLinkWaste" },
  { to: "/my-year", key: "overview.quickLinkMyYear" },
  { to: "/no-idea", key: "overview.quickLinkNoIdea" },
];

function formatElapsed(runStartedAt: string): string {
  if (!runStartedAt) return "00:00";
  const started = new Date(runStartedAt).getTime();
  const minutes = Math.max(0, Math.round((Date.now() - started) / 60000));
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

/**
 * Shows real, live Activity Engine status, plus quick links into the
 * app's now-substantial feature set (Time Receipt, Patterns, Digital
 * Waste, MY YEAR, "I Have No Idea").
 *
 * This page used to end with hardcoded "Working" / "Next" lists from
 * very early development ("Next: Phase 5 - Timers + alarms") that had
 * gone badly stale - Phases 5 through 13 are now substantially built,
 * so that list was actively misleading about the app's real state by
 * the time this rewrite happened. Removed rather than updated-in-place,
 * since a running tally of "what phase are we on" was always a
 * development-time convenience, not a real section 12 Daily Home
 * Screen feature - the quick links below are a more honest, more
 * useful thing for this page to show a person who's actually using the
 * app day to day.
 */
export function Overview() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<ActivityStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const activity = await getActivityBindings();
      setStatus(await activity.GetStatus());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    refresh();
    // Push-based updates from Go (see main.go's engine.SetOnStateChange)
    // plus a slow poll as a safety net - the poll alone would work, the
    // event alone would miss the elapsed-time display ticking forward
    // between app switches.
    const off = Events.On("activity:changed", refresh);
    const interval = setInterval(refresh, 15000);
    return () => {
      off();
      clearInterval(interval);
    };
  }, [refresh]);

  async function pause(minutes: number) {
    setBusy(true);
    try {
      const activity = await getActivityBindings();
      await activity.Pause(minutes);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function resume() {
    setBusy(true);
    try {
      const activity = await getActivityBindings();
      await activity.Resume();
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <h1>{t("overview.greeting")}</h1>

      <section className="overview__section">
        <h2>{t("overview.currentlyHeading")}</h2>
        {error && <p className="settings__error">{error}</p>}
        {!error && status && (
          <div className="overview__currently">
            {status.paused ? (
              <p className="overview__status-line">{t("overview.trackingPaused")}</p>
            ) : status.processName ? (
              <p className="overview__status-line">
                {status.idle ? t("overview.idle") : status.processName}
                {!status.idle && <span className="overview__elapsed"> · {formatElapsed(status.runStartedAt)}</span>}
              </p>
            ) : (
              <p className="overview__status-line settings__hint">{t("overview.waitingForFirstSample")}</p>
            )}

            <div className="overview__controls">
              {status.paused ? (
                <button disabled={busy} onClick={resume}>
                  {t("overview.resumeTracking")}
                </button>
              ) : (
                <>
                  <button disabled={busy} onClick={() => pause(15)}>
                    {t("overview.pause15")}
                  </button>
                  <button disabled={busy} onClick={() => pause(60)}>
                    {t("overview.pause60")}
                  </button>
                  <button disabled={busy} onClick={() => pause(0)}>
                    {t("overview.pauseUntilResumed")}
                  </button>
                </>
              )}
            </div>
          </div>
        )}
      </section>

      <section className="overview__section">
        <h2>{t("overview.quickLinksHeading")}</h2>
        <ul className="overview__quick-links">
          {QUICK_LINKS.map((link) => (
            <li key={link.to}>
              <Link to={link.to}>{t(link.key)}</Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
