import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import {
  getAlarmBindings,
  getCategoryBindings,
  getProjectBindings,
  type AlarmView,
  type CategoryRecord,
  type ProjectRecord,
} from "@/lib/bindings";

const ALARM_TYPES = ["one_time", "relative", "recurring", "schedule", "activity_based"] as const;
type AlarmTypeOption = (typeof ALARM_TYPES)[number];

const ALARM_TYPE_LABEL_KEYS: Record<AlarmTypeOption, string> = {
  one_time: "alarms.typeOneTime",
  relative: "alarms.typeRelative",
  recurring: "alarms.typeRecurring",
  schedule: "alarms.typeSchedule",
  activity_based: "alarms.typeActivityBased",
};

// Same abbreviated weekday concept as Calendar.tsx's grid header
// reuse those keys rather than duplicating seven more translations for
// the identical Mon/Tue/.../Sun set.
const WEEKDAYS = [
  { code: "MO", labelKey: "calendar.weekdayMon" },
  { code: "TU", labelKey: "calendar.weekdayTue" },
  { code: "WE", labelKey: "calendar.weekdayWed" },
  { code: "TH", labelKey: "calendar.weekdayThu" },
  { code: "FR", labelKey: "calendar.weekdayFri" },
  { code: "SA", labelKey: "calendar.weekdaySat" },
  { code: "SU", labelKey: "calendar.weekdaySun" },
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

// datetime-local inputs give/want "YYYY-MM-DDTHH:MM" in the browser's
// local time with no timezone suffix - Date's constructor already
// interprets that as local time, so no manual offset math is needed.
function localDateTimeToUnix(value: string): number {
  const ms = new Date(value).getTime();
  return Number.isNaN(ms) ? 0 : Math.floor(ms / 1000);
}

function defaultDateTimeLocal(): string {
  const d = new Date(Date.now() + 60 * 60 * 1000); // an hour from now, a reasonable default
  d.setSeconds(0, 0);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function Alarms() {
  const { t } = useTranslation();
  const [alarms, setAlarms] = useState<AlarmView[]>([]);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [label, setLabel] = useState("");
  const [alarmType, setAlarmType] = useState<AlarmTypeOption>("one_time");
  const [snoozeMinutes, setSnoozeMinutes] = useState(5);

  const [oneTimeAt, setOneTimeAt] = useState(defaultDateTimeLocal());
  const [relativeMinutes, setRelativeMinutes] = useState(45);
  const [freq, setFreq] = useState<"DAILY" | "WEEKDAYS" | "WEEKLY">("DAILY");
  const [byDay, setByDay] = useState<string[]>([]);
  const [timeOfDay, setTimeOfDay] = useState("09:00");
  const [condScope, setCondScope] = useState<"category" | "project">("category");
  const [condName, setCondName] = useState("");
  const [condMinutes, setCondMinutes] = useState(60);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [alarmBindings, categoryBindings, projectBindings] = await Promise.all([
        getAlarmBindings(),
        getCategoryBindings(),
        getProjectBindings(),
      ]);
      const [alarmRows, categoryRows, projectRows] = await Promise.all([
        alarmBindings.List(),
        categoryBindings.ListCategories(),
        projectBindings.ListProjects(false),
      ]);
      setAlarms(alarmRows);
      setCategories(categoryRows);
      setProjects(projectRows);
      setCondName((prev) => prev || (categoryRows[0]?.name ?? ""));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // main.go's Scheduler emits "alarm:fired" the instant an alarm goes
  // off - reload so LastFiredAt/NextFireAt and (for a one-shot alarm)
  // its now-disabled state show up without waiting for the person to
  // revisit this page.
  useEffect(() => {
    return Events.On("alarm:fired", () => load());
  }, [load]);

  function toggleByDay(code: string) {
    setByDay((prev) => (prev.includes(code) ? prev.filter((d) => d !== code) : [...prev, code]));
  }

  function buildRecurrenceRule(): string {
    if (freq === "WEEKLY" && byDay.length > 0) {
      return `FREQ=WEEKLY;BYDAY=${byDay.join(",")};TIME=${timeOfDay}`;
    }
    return `FREQ=${freq};TIME=${timeOfDay}`;
  }

  async function withReload(action: () => Promise<unknown>) {
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleCreate() {
    if (!label.trim()) {
      setError(t("alarms.labelRequiredError"));
      return;
    }
    await withReload(async () => {
      const bindings = await getAlarmBindings();
      switch (alarmType) {
        case "one_time": {
          const fireAtUnix = localDateTimeToUnix(oneTimeAt);
          if (!fireAtUnix) throw new Error(t("alarms.invalidDateTimeError"));
          await bindings.CreateOneTime(label.trim(), fireAtUnix, snoozeMinutes, "");
          break;
        }
        case "relative":
          await bindings.CreateRelative(label.trim(), relativeMinutes, snoozeMinutes, "");
          break;
        case "recurring":
        case "schedule":
          await bindings.CreateRecurring(alarmType, label.trim(), buildRecurrenceRule(), snoozeMinutes, "");
          break;
        case "activity_based": {
          if (!condName) throw new Error(t("alarms.pickCategoryOrProjectError"));
          await bindings.CreateActivityBased(
            label.trim(),
            `${condScope}=${condName};minutes=${condMinutes}`,
            snoozeMinutes,
            "",
          );
          break;
        }
      }
      setLabel("");
    });
  }

  const scopeOptions = condScope === "category" ? categories.map((c) => c.name) : projects.map((p) => p.name);

  return (
    <div className="page page--wide">
      <h1>{t("alarms.heading")}</h1>

      {error && <p className="settings__error">{error}</p>}

      <section className="settings__section">
        <h2>{t("alarms.yourAlarmsHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : alarms.length === 0 ? (
          <p className="settings__hint">{t("alarms.noAlarmsYet")}</p>
        ) : (
          <div className="alarm-list">
            {alarms.map((a) => (
              <div key={a.id} className={`alarm-row${a.enabled ? "" : " alarm-row--disabled"}`}>
                <button
                  className={`alarm-row__toggle${a.enabled ? " alarm-row__toggle--on" : ""}`}
                  aria-label={a.enabled ? t("alarms.disableAriaLabel") : t("alarms.enableAriaLabel")}
                  onClick={() => withReload(async () => (await getAlarmBindings()).SetEnabled(a.id, !a.enabled))}
                />
                <div className="alarm-row__main">
                  <div className="alarm-row__label">{a.label || t("alarms.untitledAlarm")}</div>
                  <div className="alarm-row__meta">
                    {a.alarmType === "activity_based"
                      ? a.triggerCondition
                      : a.nextFireAt
                        ? t("alarms.nextPrefix", { when: formatDateTime(a.nextFireAt) })
                        : a.lastFiredAt
                          ? t("alarms.lastFiredPrefix", { when: formatDateTime(a.lastFiredAt) })
                          : t("alarms.notScheduled")}
                    {a.snoozedUntil ? t("alarms.snoozedUntilSuffix", { when: formatDateTime(a.snoozedUntil) }) : ""}
                  </div>
                </div>
                <span className="alarm-row__type-badge">
                  {ALARM_TYPE_LABEL_KEYS[a.alarmType as AlarmTypeOption] ? t(ALARM_TYPE_LABEL_KEYS[a.alarmType as AlarmTypeOption]) : a.alarmType}
                </span>
                <div className="alarm-row__actions">
                  {a.snoozedUntil ? (
                    <button onClick={() => withReload(async () => (await getAlarmBindings()).Dismiss(a.id))}>
                      {t("alarms.clearSnoozeButton")}
                    </button>
                  ) : (
                    <button
                      onClick={() =>
                        withReload(async () => (await getAlarmBindings()).Snooze(a.id, a.snoozeMinutes || 5))
                      }
                    >
                      {t("alarms.snoozeButton")}
                    </button>
                  )}
                  <button onClick={() => withReload(async () => (await getAlarmBindings()).Delete(a.id))}>
                    {t("alarms.deleteButton")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="settings__section">
        <h2>{t("alarms.newAlarmHeading")}</h2>

        <div className="project-form">
          <input placeholder={t("alarms.labelPlaceholder")} value={label} onChange={(e) => setLabel(e.target.value)} />
          <select value={alarmType} onChange={(e) => setAlarmType(e.target.value as AlarmTypeOption)}>
            {ALARM_TYPES.map((at) => (
              <option key={at} value={at}>
                {t(ALARM_TYPE_LABEL_KEYS[at])}
              </option>
            ))}
          </select>
          <input
            type="number"
            min={1}
            value={snoozeMinutes}
            onChange={(e) => setSnoozeMinutes(Number(e.target.value))}
            style={{ width: 70 }}
            title={t("alarms.snoozeDurationTitle")}
          />
        </div>

        {alarmType === "one_time" && (
          <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
            <input type="datetime-local" value={oneTimeAt} onChange={(e) => setOneTimeAt(e.target.value)} />
          </div>
        )}

        {alarmType === "relative" && (
          <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
            <span className="settings__hint">{t("alarms.inLabel")}</span>
            <input
              type="number"
              min={1}
              value={relativeMinutes}
              onChange={(e) => setRelativeMinutes(Number(e.target.value))}
              style={{ width: 70 }}
            />
            <span className="settings__hint">{t("alarms.minutesLabel")}</span>
          </div>
        )}

        {(alarmType === "recurring" || alarmType === "schedule") && (
          <div style={{ marginTop: "var(--space-3)" }}>
            <div className="project-form">
              <select value={freq} onChange={(e) => setFreq(e.target.value as typeof freq)}>
                <option value="DAILY">{t("alarms.everyDayOption")}</option>
                <option value="WEEKDAYS">{t("alarms.weekdaysOption")}</option>
                <option value="WEEKLY">{t("alarms.specificDaysOption")}</option>
              </select>
              <input type="time" value={timeOfDay} onChange={(e) => setTimeOfDay(e.target.value)} />
            </div>
            {freq === "WEEKLY" && (
              <div className="timer-launcher" style={{ marginTop: "var(--space-2)" }}>
                {WEEKDAYS.map((d) => (
                  <button
                    key={d.code}
                    onClick={() => toggleByDay(d.code)}
                    style={
                      byDay.includes(d.code) ? { background: "var(--accent)", color: "var(--background)" } : undefined
                    }
                  >
                    {t(d.labelKey)}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {alarmType === "activity_based" && (
          <div className="project-form" style={{ marginTop: "var(--space-3)" }}>
            <select
              value={condScope}
              onChange={(e) => {
                setCondScope(e.target.value as "category" | "project");
                setCondName("");
              }}
            >
              <option value="category">{t("alarms.categoryOption")}</option>
              <option value="project">{t("alarms.projectOption")}</option>
            </select>
            <select value={condName} onChange={(e) => setCondName(e.target.value)}>
              <option value="">{t("alarms.chooseOption")}</option>
              {scopeOptions.map((name) => (
                <option key={name} value={name}>
                  {name}
                </option>
              ))}
            </select>
            <span className="settings__hint">{t("alarms.afterLabel")}</span>
            <input
              type="number"
              min={1}
              value={condMinutes}
              onChange={(e) => setCondMinutes(Number(e.target.value))}
              style={{ width: 70 }}
            />
            <span className="settings__hint">{t("alarms.minutesLabel")}</span>
          </div>
        )}

        <div style={{ marginTop: "var(--space-3)" }}>
          <button onClick={handleCreate}>{t("alarms.addAlarmButton")}</button>
        </div>
      </section>
    </div>
  );
}
