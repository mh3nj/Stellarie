import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Events } from "@wailsio/runtime";
import { Icon } from "@/components/icons";
import { getAlarmBindings, getNotificationBindings } from "@/lib/bindings";

// An alarm or a timer running out is the one thing in STELLAREI that has
// to be impossible to miss, and the OS notification banner is not
// something the app controls: a person can have notifications switched
// off for STELLAREI, have Windows's Do Not Disturb on, have denied
// authorization on macOS, or simply be looking at a full-screen window.
// So the same event that triggers the native toast also raises this
// banner inside the app. The toast is the "look over here" channel, this
// is the guaranteed one, and internal/alert's tone is the audible one.
type FiredKind = "alarm" | "timer";

interface Fired {
  kind: FiredKind;
  title: string;
  body: string;
  /** notifications row id, so Dismiss can clear it from the in-app history too (0 = none found) */
  notificationId: number;
  /** alarms row id, only for alarms - enables Snooze (0 = not an alarm / unknown) */
  alarmId: number;
}

export function AlertBanner() {
  const navigate = useNavigate();
  const [fired, setFired] = useState<Fired | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // main.go's Scheduler emits these the instant it notices a timer
    // reached its target or an alarm came due, after it has already
    // written the notification to history - so ListRecent(1) is the row
    // that just fired (title/body/label all come from the backend rather
    // than being reinvented here).
    function onFired(kind: FiredKind) {
      return async () => {
        const fallbackTitle = kind === "alarm" ? "Alarm" : "Timer finished";
        try {
          const notifications = await getNotificationBindings();
          const [latest] = await notifications.ListRecent(1);
          setFired({
            kind,
            title: latest?.title || fallbackTitle,
            body: latest?.body ?? "",
            notificationId: latest?.id ?? 0,
            alarmId: latest?.source === "alarm" ? latest.sourceId : 0,
          });
        } catch (err) {
          // Reading the history back is a nicety; the alert itself must
          // still appear if that read fails, or the person is left with
          // only the audible tone and no idea what it was.
          setFired({ kind, title: fallbackTitle, body: "", notificationId: 0, alarmId: 0 });
          setError(err instanceof Error ? err.message : String(err));
          return;
        }
        setError(null);
      };
    }

    const offAlarm = Events.On("alarm:fired", onFired("alarm"));
    const offTimer = Events.On("timer:completed", onFired("timer"));
    return () => {
      offAlarm();
      offTimer();
    };
  }, []);

  async function dismissNotification(notificationId: number) {
    if (notificationId <= 0) return;
    try {
      await (await getNotificationBindings()).Dismiss(notificationId);
    } catch (err) {
      // Not fatal: the banner has already been taken down by the caller,
      // and a history row that stays unread is not worth blocking on.
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDismiss() {
    if (!fired) return;
    setBusy(true);
    // The banner goes away regardless of whether the history write
    // succeeded - the person asked for it to be gone.
    await dismissNotification(fired.notificationId);
    setFired(null);
    setBusy(false);
  }

  async function handleSnooze() {
    if (!fired || fired.alarmId <= 0) return;
    setBusy(true);
    try {
      // minutes 0 = use the alarm's own configured snooze length, the
      // same fallback AlarmService.Snooze documents.
      await (await getAlarmBindings()).Snooze(fired.alarmId, 0);
      setFired(null);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleOpen() {
    if (!fired) return;
    const target = fired.kind === "alarm" ? "/alarms" : "/timers";
    await handleDismiss();
    navigate(target);
  }

  if (!fired) return null;

  return (
    <div className="alert-banner" role="alert" aria-live="assertive">
      <span className="alert-banner__icon" aria-hidden="true">
        <Icon name={fired.kind === "alarm" ? "alarms" : "timers"} size={22} />
      </span>
      <div className="alert-banner__text">
        <strong className="alert-banner__title">{fired.title}</strong>
        {fired.body && <span className="alert-banner__body">{fired.body}</span>}
        {error && <span className="alert-banner__error">{error}</span>}
      </div>
      <div className="alert-banner__actions">
        {fired.alarmId > 0 && (
          <button type="button" disabled={busy} onClick={handleSnooze}>
            Snooze
          </button>
        )}
        <button type="button" disabled={busy} onClick={handleOpen}>
          Open
        </button>
        <button type="button" className="alert-banner__dismiss" disabled={busy} onClick={handleDismiss}>
          <Icon name="close" size={16} />
          <span className="alert-banner__dismiss-label">Dismiss</span>
        </button>
      </div>
    </div>
  );
}
