export type TimelineZoom = "day" | "week" | "month";

/** Parses a "YYYY-MM-DD" search param (e.g. a link from Calendar's day
 * click or a Search result - spec sections 29/30's own "clicking a day/
 * result opens its timeline" requirement) into a local Date. Falls back
 * to today for a missing or malformed value, so a bad or old bookmarked
 * link never crashes the page it lands on, just shows the same default
 * it always would have. */
export function parseDateParam(value: string | null): Date {
  if (!value) return new Date();
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return new Date();
  const [, y, m, d] = match;
  const parsed = new Date(Number(y), Number(m) - 1, Number(d));
  return Number.isNaN(parsed.getTime()) ? new Date() : parsed;
}

/** Returns [startUnix, endUnix) covering the zoom level containing
 * `anchor`, in the browser's local timezone (matching spec section 70:
 * store/reason in UTC seconds, but every boundary here is computed from
 * local wall-clock midnight, which is what "day" means to a person). */
export function rangeFor(zoom: TimelineZoom, anchor: Date, weekStartDay = 1): [number, number] {
  const start = new Date(anchor);
  start.setHours(0, 0, 0, 0);

  if (zoom === "day") {
    const end = new Date(start);
    end.setDate(end.getDate() + 1);
    return [Math.floor(start.getTime() / 1000), Math.floor(end.getTime() / 1000)];
  }

  if (zoom === "week") {
    // Week start is the person's own setting (see lib/week-start.ts's
    // useWeekStartDay) - Monday by default, matching the locale default
    // for English/German and every pre-override caller, which passes
    // nothing. The math below with weekStartDay=1 reduces exactly to the
    // old hardcoded diffToMonday, so the default path is unchanged.
    // (An earlier comment here cited "section 71" for this setting
    // wrong, section 71 is Settings > AI; the number is left out rather
    // than guessed at, since the correct one is unknown without the
    // source spec document.)
    const day = start.getDay(); // 0 = Sunday
    const diffToWeekStart = (day - weekStartDay + 7) % 7;
    start.setDate(start.getDate() - diffToWeekStart);
    const end = new Date(start);
    end.setDate(end.getDate() + 7);
    return [Math.floor(start.getTime() / 1000), Math.floor(end.getTime() / 1000)];
  }

  // month
  start.setDate(1);
  const end = new Date(start);
  end.setMonth(end.getMonth() + 1);
  return [Math.floor(start.getTime() / 1000), Math.floor(end.getTime() / 1000)];
}

/** Moves `anchor` forward or backward by one unit of `zoom`. */
export function shiftAnchor(zoom: TimelineZoom, anchor: Date, direction: 1 | -1): Date {
  const next = new Date(anchor);
  if (zoom === "day") next.setDate(next.getDate() + direction);
  else if (zoom === "week") next.setDate(next.getDate() + 7 * direction);
  else next.setMonth(next.getMonth() + direction);
  return next;
}

export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  if (h === 0) return `${m}m`;
  if (m === 0) return `${h}h`;
  return `${h}h ${m}m`;
}

/** Formats a live countdown/stopwatch face as HH:MM:SS (or MM:SS under an
 * hour) - second-precision, unlike formatDuration above, which rounds to
 * the minute and reads naturally in a budget/goal summary but would make
 * a running timer look frozen for up to 59 seconds at a time. */
export function formatClock(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = String(m).padStart(2, "0");
  const ss = String(s).padStart(2, "0");
  if (h === 0) return `${mm}:${ss}`;
  return `${h}:${mm}:${ss}`;
}

export function formatClockTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function formatRangeLabel(zoom: TimelineZoom, anchor: Date, weekStartDay = 1): string {
  if (zoom === "day") {
    return anchor.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" });
  }
  if (zoom === "week") {
    const [startUnix, endUnix] = rangeFor("week", anchor, weekStartDay);
    const start = new Date(startUnix * 1000);
    const end = new Date((endUnix - 1) * 1000);
    return `${start.toLocaleDateString([], { month: "short", day: "numeric" })} - ${end.toLocaleDateString([], { month: "short", day: "numeric" })}`;
  }
  return anchor.toLocaleDateString([], { month: "long", year: "numeric" });
}
