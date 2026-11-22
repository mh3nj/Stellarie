import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { getCalendarBindings, getCategoryBindings, type CalendarMonthView, type CategoryRecord } from "@/lib/bindings";
import { formatDuration } from "@/lib/time-ranges";
import { useWeekStartDay } from "@/lib/week-start";

// Full weekday key set indexed by JS Date.getDay() (0=Sun...6=Sat)
// weekdayKeysFrom rotates this to start at the effective week-start day
// (the person's own setting when set, else the active locale's real
// convention - see lib/week-start.ts) rather than assuming Monday-first
// for every language. Persian starts on Saturday; the locale half of
// this used to be a named, honest gap (see the handbook), closed without
// adding any new locale keys - the existing weekdayMon..weekdaySun
// labels are simply reordered per locale.
const WEEKDAY_KEYS_BY_DAY = [
  "calendar.weekdaySun",
  "calendar.weekdayMon",
  "calendar.weekdayTue",
  "calendar.weekdayWed",
  "calendar.weekdayThu",
  "calendar.weekdayFri",
  "calendar.weekdaySat",
];

function weekdayKeysFrom(startDay: number): string[] {
  return Array.from({ length: 7 }, (_, i) => WEEKDAY_KEYS_BY_DAY[(startDay + i) % 7]);
}
const MAX_CATEGORY_LINES = 3;

function monthLabel(year: number, month: number): string {
  return new Date(year, month - 1, 1).toLocaleDateString(undefined, { month: "long", year: "numeric" });
}

export function CalendarPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const now = new Date();
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1); // 1-12
  const [view, setView] = useState<CalendarMonthView | null>(null);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [calendarBindings, categoryBindings] = await Promise.all([getCalendarBindings(), getCategoryBindings()]);
      const [monthView, cats] = await Promise.all([calendarBindings.GetMonth(year, month), categoryBindings.ListCategories()]);
      setView(monthView);
      setCategories(cats);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [year, month]);

  useEffect(() => {
    load();
  }, [load]);

  function shiftMonth(delta: number) {
    const d = new Date(year, month - 1 + delta, 1);
    setYear(d.getFullYear());
    setMonth(d.getMonth() + 1);
  }

  function goToday() {
    const today = new Date();
    setYear(today.getFullYear());
    setMonth(today.getMonth() + 1);
  }

  const weekStartDay = useWeekStartDay();
  const daysInMonth = new Date(year, month, 0).getDate();
  const firstWeekday = (new Date(year, month - 1, 1).getDay() - weekStartDay + 7) % 7;
  const dayByDate = new Map(view?.days.map((d) => [d.date, d]) ?? []);
  const categoryName = (id: number) => categories.find((c) => c.id === id)?.name ?? t("calendar.uncategorized");

  const cells: (number | null)[] = [...Array.from({ length: firstWeekday }, () => null), ...Array.from({ length: daysInMonth }, (_, i) => i + 1)];

  return (
    <div className="page">
      <h1>{t("calendar.heading")}</h1>

      <div className="timeline__controls">
        <div className="timeline__nav">
          <button onClick={() => shiftMonth(-1)} aria-label={t("common.previousLabel")}>‹</button>
          <span className="timeline__range-label">{monthLabel(year, month)}</span>
          <button onClick={() => shiftMonth(1)} aria-label={t("common.nextLabel")}>›</button>
          <button onClick={goToday}>{t("common.thisMonth")}</button>
        </div>
      </div>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : (
        <div className="calendar-grid">
          {weekdayKeysFrom(weekStartDay).map((key) => (
            <div key={key} className="calendar-grid__weekday">
              {t(key)}
            </div>
          ))}
          {cells.map((day, i) => {
            if (day === null) {
              return <div key={`blank-${i}`} className="calendar-grid__cell calendar-grid__cell--blank" />;
            }
            const dateStr = `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
            const dayView = dayByDate.get(dateStr);
            const sorted = dayView ? [...dayView.categories].sort((a, b) => b.seconds - a.seconds) : [];
            const shown = sorted.slice(0, MAX_CATEGORY_LINES);
            const remaining = sorted.length - shown.length;

            return (
              <button
                key={day}
                className={`calendar-grid__cell${dayView ? "" : " calendar-grid__cell--empty"}`}
                onClick={() => navigate(`/receipt?date=${dateStr}`)}
                title={t("calendar.openReceiptTitle", { date: dateStr })}
              >
                <span className="calendar-grid__day-number">{day}</span>
                {shown.map((c) => (
                  <span key={c.categoryId} className="calendar-grid__category-line">
                    <span>{categoryName(c.categoryId)}</span>
                    <span>{formatDuration(c.seconds)}</span>
                  </span>
                ))}
                {remaining > 0 && <span className="calendar-grid__more">{t("calendar.moreCount", { count: remaining })}</span>}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
