import { useEffect, useState, type ReactNode } from "react";
import { HashRouter, useLocation, useNavigate } from "react-router-dom";
import { Events } from "@wailsio/runtime";
import { Nav } from "@/components/Nav";
import { AlertBanner } from "@/components/AlertBanner";
import { CloseConfirmDialog } from "@/components/CloseConfirmDialog";
import { RouteErrorBoundary } from "@/components/ErrorBoundary";
import { ThemeProvider } from "@/lib/theme-context";
import { Overview } from "@/pages/Overview";
import { Settings } from "@/pages/Settings";
import { About } from "@/pages/About";
import { Themes } from "@/pages/Themes";
import { Timeline } from "@/pages/Timeline";
import { Projects } from "@/pages/Projects";
import { Analytics } from "@/pages/Analytics";
import { TimeReceipt } from "@/pages/TimeReceipt";
import { Patterns } from "@/pages/Patterns";
import { DigitalWaste } from "@/pages/DigitalWaste";
import { CalendarPage } from "@/pages/Calendar";
import { Search } from "@/pages/Search";
import { MyYear } from "@/pages/MyYear";
import { Diagnostics } from "@/pages/Diagnostics";
import { Goals } from "@/pages/Goals";
import { Timers } from "@/pages/Timers";
import { Alarms } from "@/pages/Alarms";
import { Focus } from "@/pages/Focus";
import { NoIdea } from "@/pages/NoIdea";
import { Privacy } from "@/pages/Privacy";
import { Backups } from "@/pages/Backups";

/** Listens for the tray's "Settings"/"Timers"/"Alarms"/"Focus"/
 * "I HAVE NO IDEA" clicks (main.go emits "tray:open-settings"/
 * "tray:open-timers"/"tray:open-alarms"/"tray:open-focus"/
 * "tray:open-no-idea" after showing/focusing the window) and routes
 * there. Split out from AppShell so useNavigate has a Router ancestor. */
function TrayEventBridge() {
  const navigate = useNavigate();
  useEffect(() => {
    const offSettings = Events.On("tray:open-settings", () => navigate("/settings"));
    const offTimers = Events.On("tray:open-timers", () => navigate("/timers"));
    const offAlarms = Events.On("tray:open-alarms", () => navigate("/alarms"));
    const offFocus = Events.On("tray:open-focus", () => navigate("/focus"));
    const offNoIdea = Events.On("tray:open-no-idea", () => navigate("/no-idea"));
    return () => {
      offSettings();
      offTimers();
      offAlarms();
      offFocus();
      offNoIdea();
    };
  }, [navigate]);
  return null;
}

// Every nav destination, declared once. KeepAliveRoutes mounts each
// page on first visit and keeps it mounted afterwards (hidden while
// inactive) - see its own comment for why.
const ROUTE_DEFS: { path: string; element: ReactNode }[] = [
  { path: "/", element: <Overview /> },
  { path: "/timeline", element: <Timeline /> },
  { path: "/calendar", element: <CalendarPage /> },
  { path: "/projects", element: <Projects /> },
  { path: "/analytics", element: <Analytics /> },
  { path: "/receipt", element: <TimeReceipt /> },
  { path: "/patterns", element: <Patterns /> },
  { path: "/digital-waste", element: <DigitalWaste /> },
  { path: "/goals", element: <Goals /> },
  { path: "/focus", element: <Focus /> },
  { path: "/timers", element: <Timers /> },
  { path: "/alarms", element: <Alarms /> },
  { path: "/no-idea", element: <NoIdea /> },
  { path: "/search", element: <Search /> },
  { path: "/my-year", element: <MyYear /> },
  { path: "/diagnostics", element: <Diagnostics /> },
  { path: "/themes", element: <Themes /> },
  { path: "/privacy", element: <Privacy /> },
  { path: "/backups", element: <Backups /> },
  { path: "/settings", element: <Settings /> },
  { path: "/about", element: <About /> },
];

// Tab state survives tab switches: React Router unmounts a route's
// element the moment you navigate away, discarding everything the
// person typed or toggled on that page (search queries, idea intents,
// timer labels, form drafts). Instead of accepting that, each visited
// page stays mounted and is merely hidden (display:none removes it
// from the tab order and the layout, so a hidden page can't steal
// focus or clicks). Pages mount lazily on first visit, so startup cost
// is unchanged; the only ongoing cost is background refreshes in pages
// that poll (Overview re-reads activity status every 15s - cheap,
// local, and arguably desirable so the tab is current on return).
// Unknown paths render nothing new, exactly as before.
function KeepAliveRoutes() {
  const { pathname } = useLocation();
  const [visited, setVisited] = useState<string[]>([]);
  if (!visited.includes(pathname) && ROUTE_DEFS.some((r) => r.path === pathname)) {
    // Render-phase update (the documented "adjust state during render"
    // pattern): React discards this pass and re-renders with the new
    // page included before committing anything.
    setVisited([...visited, pathname]);
  }
  return (
    <>
      {visited.map((p) => {
        const def = ROUTE_DEFS.find((r) => r.path === p);
        if (!def) return null;
        const active = p === pathname;
        return (
          <div key={p} hidden={!active} style={active ? undefined : { display: "none" }} aria-hidden={!active}>
            {def.element}
          </div>
        );
      })}
    </>
  );
}

function AppShell() {
  return (
    <div className="app-shell">
      <Nav />
      <main className="app-shell__content">
        <RouteErrorBoundary>
          <KeepAliveRoutes />
        </RouteErrorBoundary>
      </main>
      <AlertBanner />
      <CloseConfirmDialog />
    </div>
  );
}

export function App() {
  return (
    <ThemeProvider>
      <HashRouter>
        <TrayEventBridge />
        <AppShell />
      </HashRouter>
    </ThemeProvider>
  );
}
