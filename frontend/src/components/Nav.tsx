import { NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Logo } from "@/components/Logo";
import { Icon, type IconName } from "@/components/icons";

interface NavItem {
  to: string;
  key: string;
  icon: IconName;
  end?: boolean;
}

const PRIMARY: NavItem[] = [
  { to: "/", key: "nav.overview", icon: "overview", end: true },
  { to: "/timeline", key: "nav.timeline", icon: "timeline" },
  { to: "/calendar", key: "nav.calendar", icon: "calendar" },
  { to: "/projects", key: "nav.projects", icon: "projects" },
  { to: "/analytics", key: "nav.analytics", icon: "analytics" },
  { to: "/receipt", key: "nav.receipt", icon: "receipt" },
  { to: "/patterns", key: "nav.patterns", icon: "patterns" },
  { to: "/digital-waste", key: "nav.digitalWaste", icon: "waste" },
  { to: "/my-year", key: "nav.myYear", icon: "myYear" },
  { to: "/goals", key: "nav.goals", icon: "goals" },
  { to: "/focus", key: "nav.focus", icon: "focus" },
  { to: "/timers", key: "nav.timers", icon: "timers" },
  { to: "/alarms", key: "nav.alarms", icon: "alarms" },
  { to: "/no-idea", key: "nav.noIdea", icon: "noIdea" },
];

const SECONDARY: NavItem[] = [
  { to: "/search", key: "nav.search", icon: "search" },
  { to: "/themes", key: "nav.themes", icon: "themes" },
  { to: "/settings", key: "nav.settings", icon: "settings" },
  { to: "/privacy", key: "nav.privacy", icon: "privacy" },
  { to: "/backups", key: "nav.backups", icon: "backups" },
  { to: "/diagnostics", key: "nav.diagnostics", icon: "diagnostics" },
  { to: "/about", key: "nav.about", icon: "about" },
];

function linkClass({ isActive }: { isActive: boolean }): string {
  return isActive ? "nav-link nav-link--active" : "nav-link";
}

// The sidebar carries an icon AND a label for every destination. Which of
// the two is visible is decided entirely in CSS (see app.css's nav media
// queries): wide shows both, a mid-width window shows the labels alone,
// and a narrow window shows the icons alone. The markup is identical in
// all three so nothing re-renders when the window is resized, and the
// label stays in the accessibility tree even when it is visually hidden
// by keeping the anchor's own title/aria-label populated.
//
// Spec section 69: "mirrored navigation where appropriate" - the nav item
// ORDER doesn't change for RTL (reordering menu items by language would
// move the same feature to a different list position depending on which
// language is active, which is more confusing than helpful). What mirrors
// is the whole layout, via dir="rtl" on <html> (see lib/language.ts),
// which flips which side the nav sits on and how its own flex/text
// alignment reads.
function NavRow({ item }: { item: NavItem }) {
  const { t } = useTranslation();
  const label = t(item.key);
  return (
    <li>
      <NavLink to={item.to} end={item.end} className={linkClass} title={label} aria-label={label}>
        <Icon name={item.icon} className="nav-link__icon" />
        <span className="nav-link__label">{label}</span>
      </NavLink>
    </li>
  );
}

export function Nav() {
  return (
    <nav className="nav">
      <div className="nav__brand">
        <Logo size={22} />
        <span className="nav__brand-name">STELLAREI</span>
      </div>
      <ul className="nav__group">
        {PRIMARY.map((item) => (
          <NavRow key={item.to} item={item} />
        ))}
      </ul>
      <ul className="nav__group nav__group--secondary">
        {SECONDARY.map((item) => (
          <NavRow key={item.to} item={item} />
        ))}
      </ul>
    </nav>
  );
}
