import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import {
  getCategoryBindings,
  getProjectBindings,
  getSessionBindings,
  type CategoryRecord,
  type ProjectRecord,
  type SessionView,
} from "@/lib/bindings";
import { rangeFor, shiftAnchor, formatDuration, formatClockTime, formatRangeLabel, parseDateParam, type TimelineZoom } from "@/lib/time-ranges";
import { SessionDetailPanel } from "@/components/SessionDetailPanel";
import { useWeekStartDay } from "@/lib/week-start";

// Reuses Analytics' Day/Week/Month labels - same concept.
const ZOOMS: { value: TimelineZoom; labelKey: string }[] = [
  { value: "day", labelKey: "analytics.zoomDay" },
  { value: "week", labelKey: "analytics.zoomWeek" },
  { value: "month", labelKey: "analytics.zoomMonth" },
];

function sessionColor(session: SessionView): string {
  if (session.excluded) return "var(--textMuted)";
  if (session.projectColor) return `var(--${session.projectColor})`;
  if (session.categoryColor) return `var(--${session.categoryColor})`;
  return "var(--chartPrimary)";
}

export function Timeline() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [zoom, setZoom] = useState<TimelineZoom>(() => {
    const z = searchParams.get("zoom");
    return z === "day" || z === "week" || z === "month" ? z : "day";
  });
  const [anchor, setAnchor] = useState(() => parseDateParam(searchParams.get("date")));
  // Follow the URL when it changes under this page (Search results and
  // Calendar-adjacent links navigate date→date / zoom→zoom without
  // remounting, and keep-alive tabs are never remounted at all) - same
  // reasoning as TimeReceipt's date-param sync. A manual zoom/anchor
  // choice still wins until the URL itself changes again.
  const dateParam = searchParams.get("date");
  const zoomParam = searchParams.get("zoom");
  useEffect(() => {
    setAnchor(parseDateParam(dateParam));
  }, [dateParam]);
  useEffect(() => {
    setZoom(zoomParam === "day" || zoomParam === "week" || zoomParam === "month" ? zoomParam : "day");
  }, [zoomParam]);
  const [sessions, setSessions] = useState<SessionView[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [selected, setSelected] = useState<SessionView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const weekStartDay = useWeekStartDay();
  const [rangeStart, rangeEnd] = rangeFor(zoom, anchor, weekStartDay);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [sessionBindings, projectBindings, categoryBindings] = await Promise.all([
        getSessionBindings(),
        getProjectBindings(),
        getCategoryBindings(),
      ]);
      const [rows, projectRows, categoryRows] = await Promise.all([
        sessionBindings.ListForRange(rangeStart, rangeEnd),
        projectBindings.ListProjects(false),
        categoryBindings.ListCategories(),
      ]);
      setSessions(rows);
      setProjects(projectRows);
      setCategories(categoryRows);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rangeStart, rangeEnd]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    // Keep the selected session's data fresh if the list reloads under it.
    if (!selected) return;
    const fresh = sessions.find((s) => s.id === selected.id);
    setSelected(fresh ?? null);
  }, [sessions, selected]);

  function selectAdjacent(session: SessionView, direction: -1 | 1): SessionView | null {
    const idx = sessions.findIndex((s) => s.id === session.id);
    if (idx === -1) return null;
    return sessions[idx + direction] ?? null;
  }

  const totalTracked = sessions.reduce((sum, s) => sum + (s.endedAt - s.startedAt), 0);

  return (
    <div className="page page--wide">
      <h1>{t("timeline.heading")}</h1>

      <div className="timeline__controls">
        <div className="timeline__zoom">
          {ZOOMS.map((z) => (
            <button key={z.value} className={z.value === zoom ? "timeline__zoom-btn--active" : ""} onClick={() => setZoom(z.value)}>
              {t(z.labelKey)}
            </button>
          ))}
        </div>
        <div className="timeline__nav">
          <button onClick={() => setAnchor(shiftAnchor(zoom, anchor, -1))} aria-label={t("common.previousLabel")}>‹</button>
          <span className="timeline__range-label">{formatRangeLabel(zoom, anchor, weekStartDay)}</span>
          <button onClick={() => setAnchor(shiftAnchor(zoom, anchor, 1))} aria-label={t("common.nextLabel")}>›</button>
          <button onClick={() => setAnchor(new Date())}>{t("common.today")}</button>
        </div>
      </div>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : sessions.length === 0 ? (
        <p className="settings__hint">{t("analytics.nothingTrackedRange")}</p>
      ) : (
        <>
          <p className="settings__hint">{t("timeline.trackedSuffix", { duration: formatDuration(totalTracked) })}</p>

          {zoom === "day" && (
            <DayTimeline sessions={sessions} rangeStart={rangeStart} rangeEnd={rangeEnd} onSelect={setSelected} />
          )}
          {zoom === "week" && (
            <WeekTimeline sessions={sessions} rangeStart={rangeStart} onSelect={setSelected} zoomTo={setZoom} setAnchor={setAnchor} />
          )}
          {zoom === "month" && (
            <MonthTimeline sessions={sessions} rangeStart={rangeStart} zoomTo={setZoom} setAnchor={setAnchor} />
          )}
        </>
      )}

      {selected && (
        <SessionDetailPanel
          session={selected}
          projects={projects}
          categories={categories}
          previousSession={selectAdjacent(selected, -1)}
          nextSession={selectAdjacent(selected, 1)}
          onClose={() => setSelected(null)}
          onChanged={load}
        />
      )}
    </div>
  );
}

function DayTimeline({
  sessions,
  rangeStart,
  rangeEnd,
  onSelect,
}: {
  sessions: SessionView[];
  rangeStart: number;
  rangeEnd: number;
  onSelect: (s: SessionView) => void;
}) {
  const span = rangeEnd - rangeStart;
  const hourMarks = Array.from({ length: 25 }, (_, h) => h);

  return (
    <div className="day-timeline">
      <div className="day-timeline__hours">
        {hourMarks.map((h) => (
          <span key={h} className="day-timeline__hour-label" style={{ left: `${(h / 24) * 100}%` }}>
            {h % 3 === 0 ? `${h}:00` : ""}
          </span>
        ))}
      </div>
      <div className="day-timeline__track">
        {sessions.map((s) => {
          const left = ((Math.max(s.startedAt, rangeStart) - rangeStart) / span) * 100;
          const width = ((Math.min(s.endedAt, rangeEnd) - Math.max(s.startedAt, rangeStart)) / span) * 100;
          return (
            <button
              key={s.id}
              className="day-timeline__session"
              style={{ left: `${left}%`, width: `${Math.max(width, 0.3)}%`, background: sessionColor(s) }}
              onClick={() => onSelect(s)}
              title={`${s.projectName || s.applicationName} - ${formatClockTime(s.startedAt)} - ${formatClockTime(s.endedAt)} (${formatDuration(s.endedAt - s.startedAt)})`}
            />
          );
        })}
      </div>
    </div>
  );
}

function WeekTimeline({
  sessions,
  rangeStart,
  onSelect,
  zoomTo,
  setAnchor,
}: {
  sessions: SessionView[];
  rangeStart: number;
  onSelect: (s: SessionView) => void;
  zoomTo: (z: TimelineZoom) => void;
  setAnchor: (d: Date) => void;
}) {
  const days = Array.from({ length: 7 }, (_, i) => rangeStart + i * 86400);

  return (
    <div className="week-timeline">
      {days.map((dayStart) => {
        const dayEnd = dayStart + 86400;
        const daySessions = sessions.filter((s) => s.startedAt < dayEnd && s.endedAt > dayStart);
        const date = new Date(dayStart * 1000);
        const total = daySessions.reduce(
          (sum, s) => sum + (Math.min(s.endedAt, dayEnd) - Math.max(s.startedAt, dayStart)),
          0,
        );
        return (
          <div key={dayStart} className="week-timeline__row">
            <button
              className="week-timeline__day-label"
              onClick={() => {
                setAnchor(date);
                zoomTo("day");
              }}
            >
              {date.toLocaleDateString([], { weekday: "short", day: "numeric" })}
            </button>
            <div className="week-timeline__track">
              {daySessions.map((s) => {
                const left = ((Math.max(s.startedAt, dayStart) - dayStart) / 86400) * 100;
                const width = ((Math.min(s.endedAt, dayEnd) - Math.max(s.startedAt, dayStart)) / 86400) * 100;
                return (
                  <button
                    key={s.id}
                    className="week-timeline__session"
                    style={{ left: `${left}%`, width: `${Math.max(width, 0.3)}%`, background: sessionColor(s) }}
                    onClick={() => onSelect(s)}
                    title={`${s.projectName || s.applicationName} - ${formatDuration(s.endedAt - s.startedAt)}`}
                  />
                );
              })}
            </div>
            <span className="week-timeline__total">{total > 0 ? formatDuration(total) : ""}</span>
          </div>
        );
      })}
    </div>
  );
}

function MonthTimeline({
  sessions,
  rangeStart,
  zoomTo,
  setAnchor,
}: {
  sessions: SessionView[];
  rangeStart: number;
  zoomTo: (z: TimelineZoom) => void;
  setAnchor: (d: Date) => void;
}) {
  const { t } = useTranslation();
  const firstOfMonth = new Date(rangeStart * 1000);
  const daysInMonth = new Date(firstOfMonth.getFullYear(), firstOfMonth.getMonth() + 1, 0).getDate();
  const weekStartDay = useWeekStartDay();
  const leadingBlank = (firstOfMonth.getDay() - weekStartDay + 7) % 7; // effective grid start - see lib/week-start.ts

  const totalsByDay = new Map<number, number>();
  for (const s of sessions) {
    const day = new Date(s.startedAt * 1000).getDate();
    totalsByDay.set(day, (totalsByDay.get(day) ?? 0) + (s.endedAt - s.startedAt));
  }
  const maxTotal = Math.max(1, ...Array.from(totalsByDay.values()));

  const cells = [
    ...Array.from({ length: leadingBlank }, () => null),
    ...Array.from({ length: daysInMonth }, (_, i) => i + 1),
  ];

  // Same weekday set as Calendar.tsx's grid header and Alarms.tsx's
  // toggle buttons - reused rather than duplicated a third time.
  // Indexed by JS Date.getDay() (0=Sun...6=Sat) and rotated to start
  // at the effective week-start day, same as Calendar.tsx - the
  // locale-aware half of this used to be hardcoded Monday-first
  // regardless of locale, a named honest gap now closed (no new locale
  // keys needed, the existing weekdayMon..weekdaySun labels are just
  // reordered).
  const WEEKDAY_KEYS_BY_DAY = [
    "calendar.weekdaySun",
    "calendar.weekdayMon",
    "calendar.weekdayTue",
    "calendar.weekdayWed",
    "calendar.weekdayThu",
    "calendar.weekdayFri",
    "calendar.weekdaySat",
  ];
  const weekdayKeys = Array.from({ length: 7 }, (_, i) => WEEKDAY_KEYS_BY_DAY[(weekStartDay + i) % 7]);

  return (
    <div className="month-timeline">
      {weekdayKeys.map((key) => (
        <div key={key} className="month-timeline__weekday">
          {t(key)}
        </div>
      ))}
      {cells.map((day, i) => {
        if (day === null) return <div key={`blank-${i}`} className="month-timeline__cell month-timeline__cell--blank" />;
        const total = totalsByDay.get(day) ?? 0;
        const intensity = total > 0 ? 0.15 + 0.75 * (total / maxTotal) : 0;
        return (
          <button
            key={day}
            className="month-timeline__cell"
            style={total > 0 ? { background: `rgba(110, 168, 254, ${intensity})` } : undefined}
            onClick={() => {
              setAnchor(new Date(firstOfMonth.getFullYear(), firstOfMonth.getMonth(), day));
              zoomTo("day");
            }}
          >
            <span className="month-timeline__day-number">{day}</span>
            {total > 0 && <span className="month-timeline__day-total">{formatDuration(total)}</span>}
          </button>
        );
      })}
    </div>
  );
}
