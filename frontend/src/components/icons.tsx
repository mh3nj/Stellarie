import type { ReactNode } from "react";

// Hand-built line icons, one per navigation destination. They are plain
// inline SVG rather than an icon package: STELLAREI ships no web fonts
// and no icon dependency, and twenty small paths cost nothing at bundle
// size while staying perfectly crisp at any DPI. Every icon is drawn on
// the same 24x24 grid with the same 1.75 stroke so the sidebar reads as
// one set, not twenty borrowed drawings.
export type IconName =
  | "overview"
  | "timeline"
  | "calendar"
  | "projects"
  | "analytics"
  | "receipt"
  | "patterns"
  | "waste"
  | "myYear"
  | "goals"
  | "focus"
  | "timers"
  | "alarms"
  | "noIdea"
  | "search"
  | "themes"
  | "settings"
  | "privacy"
  | "backups"
  | "diagnostics"
  | "about"
  | "clock"
  | "check"
  | "close"
  | "refresh"
  | "package";

const PATHS: Record<IconName, ReactNode> = {
  overview: (
    <>
      <path d="M3 10.5 12 3l9 7.5" />
      <path d="M5 9.5V20a1 1 0 0 0 1 1h3.5v-6h5v6H18a1 1 0 0 0 1-1V9.5" />
    </>
  ),
  timeline: (
    <>
      <circle cx="4.5" cy="6" r="1.6" />
      <circle cx="4.5" cy="12" r="1.6" />
      <circle cx="4.5" cy="18" r="1.6" />
      <path d="M9 6h11" />
      <path d="M9 12h7" />
      <path d="M9 18h9" />
    </>
  ),
  calendar: (
    <>
      <rect x="3" y="4.5" width="18" height="16.5" rx="2" />
      <path d="M3 9.5h18" />
      <path d="M8 2.5v4" />
      <path d="M16 2.5v4" />
    </>
  ),
  projects: (
    <>
      <path d="M3 7.5a2 2 0 0 1 2-2h3.6l1.8 2.4H19a2 2 0 0 1 2 2v7.6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
    </>
  ),
  analytics: (
    <>
      <path d="M4 20V10" />
      <path d="M10 20V4" />
      <path d="M16 20v-7" />
      <path d="M22 20H2" />
    </>
  ),
  receipt: (
    <>
      <path d="M6 2.5h12v19l-3-2-3 2-3-2-3 2z" />
      <path d="M9 7.5h6" />
      <path d="M9 12h6" />
    </>
  ),
  patterns: (
    <>
      <path d="M12 3l1.9 5.1L19 10l-5.1 1.9L12 17l-1.9-5.1L5 10l5.1-1.9z" />
      <path d="M18.5 16.5l.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" />
    </>
  ),
  waste: (
    <>
      <path d="M3.5 6.5h17" />
      <path d="M8 6.5V4.5a1.5 1.5 0 0 1 1.5-1.5h5A1.5 1.5 0 0 1 16 4.5v2" />
      <path d="M18.5 6.5 17.7 20a2 2 0 0 1-2 1.9H8.3a2 2 0 0 1-2-1.9L5.5 6.5" />
      <path d="M10 10.5v7" />
      <path d="M14 10.5v7" />
    </>
  ),
  myYear: (
    <>
      <circle cx="12" cy="8.5" r="5.5" />
      <path d="M8.4 13.2 7 22.5l5-2.6 5 2.6-1.4-9.3" />
    </>
  ),
  goals: (
    <>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="5" />
      <circle cx="12" cy="12" r="1.4" />
    </>
  ),
  focus: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 2.5v4" />
      <path d="M12 17.5v4" />
      <path d="M2.5 12h4" />
      <path d="M17.5 12h4" />
    </>
  ),
  timers: (
    <>
      <circle cx="12" cy="13" r="8" />
      <path d="M12 9v4l2.6 1.6" />
      <path d="M9 2.5h6" />
      <path d="M12 2.5V5" />
    </>
  ),
  alarms: (
    <>
      <path d="M18 9a6 6 0 1 0-12 0c0 5-2 6.5-2 6.5h16S18 14 18 9z" />
      <path d="M13.7 19a2 2 0 0 1-3.4 0" />
      <path d="M4.5 4 2.5 6" />
      <path d="M19.5 4 21.5 6" />
    </>
  ),
  noIdea: (
    <>
      <path d="M9 17.5a6 6 0 1 1 6 0c-.6.5-1 1.3-1 2.1H10c0-.8-.4-1.6-1-2.1z" />
      <path d="M10 21.5h4" />
      <path d="M12 2v1.5" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="7.5" />
      <path d="M16.5 16.5 21 21" />
    </>
  ),
  themes: (
    <>
      <path d="M12 2.5a9.5 9.5 0 1 0 0 19c.9 0 1.6-.7 1.6-1.6 0-.4-.2-.8-.4-1.05-.3-.3-.4-.65-.4-1.05 0-.9.7-1.6 1.6-1.6h1.9a5.2 5.2 0 0 0 5.2-5.2C21.5 7 17.2 2.5 12 2.5z" />
      <circle cx="7.5" cy="10" r="1.15" />
      <circle cx="9.8" cy="6.6" r="1.15" />
      <circle cx="14.2" cy="6.6" r="1.15" />
      <circle cx="16.7" cy="10" r="1.15" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2.5l1.3 2.1 2.4-.5.6 2.4 2.1 1.3-1.3 2.1 1.3 2.1-2.1 1.3-.6 2.4-2.4-.5L12 21.5l-1.3-2.1-2.4.5-.6-2.4L5.6 16.2l1.3-2.1-1.3-2.1 2.1-1.3.6-2.4 2.4.5z" />
    </>
  ),
  privacy: (
    <>
      <path d="M12 21.5s7.5-3.7 7.5-9.5V5.4L12 2.5 4.5 5.4v6.6c0 5.8 7.5 9.5 7.5 9.5z" />
      <path d="M9.2 11.8l2 2 3.6-3.8" />
    </>
  ),
  backups: (
    <>
      <ellipse cx="12" cy="5.5" rx="7.5" ry="2.8" />
      <path d="M4.5 5.5v13c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8v-13" />
      <path d="M4.5 12c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8" />
    </>
  ),
  diagnostics: (
    <>
      <path d="M2.5 12h4l2.5-7 4 14 2.5-7h6" />
    </>
  ),
  about: (
    <>
      <circle cx="12" cy="12" r="9.5" />
      <path d="M12 11v6" />
      <path d="M12 7.5h.01" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5.2l3.4 2" />
    </>
  ),
  check: <path d="M20 6 9 17l-5-5" />,
  close: (
    <>
      <path d="M18 6 6 18" />
      <path d="M6 6l12 12" />
    </>
  ),
  refresh: (
    <>
      <path d="M20.5 11.5a8.5 8.5 0 1 0-.7 4.3" />
      <path d="M20.5 5.5v6h-6" />
    </>
  ),
  package: (
    <>
      <path d="M21 8.2 12 3 3 8.2v7.6L12 21l9-5.2z" />
      <path d="M3 8.2 12 13.4l9-5.2" />
      <path d="M12 13.4V21" />
    </>
  ),
};

interface IconProps {
  name: IconName;
  size?: number;
  className?: string;
}

/** A single monochrome, currentColor icon. Decorative by default
 * (aria-hidden) because every icon here sits beside its own visible,
 * translated label; the label is what assistive tech should read, not a
 * second announcement of the picture next to it. */
export function Icon({ name, size = 18, className }: IconProps) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {PATHS[name]}
    </svg>
  );
}
