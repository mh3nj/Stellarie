import { Component, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

// Catches render-time crashes in ONE page (a bad backend payload, a bad
// date param, any TypeError during render) and shows a recovery screen
// instead of unmounting the entire app. Before this existed, a crash in
// any route left a black, unusable window - worse, the crashing hash
// route persisted, so every reload crashed again until the person
// cleared app state by hand. The boundary resets automatically on
// navigation (resetKey) and always offers a way home.
interface BoundaryProps {
  resetKey: string;
  onGoHome: () => void;
  children: ReactNode;
}

interface BoundaryState {
  error: Error | null;
}

class ErrorBoundary extends Component<BoundaryProps, BoundaryState> {
  state: BoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: { componentStack?: string }) {
    // Logged, never sent anywhere - local-first means crash details
    // stay on this machine (spec section 1: no telemetry).
    console.error("STELLAREI page crash:", error, info.componentStack);
  }

  componentDidUpdate(prevProps: BoundaryProps) {
    if (prevProps.resetKey !== this.props.resetKey && this.state.error) {
      this.setState({ error: null });
    }
  }

  render() {
    if (!this.state.error) return this.props.children;
    return <CrashFallback error={this.state.error} onGoHome={this.props.onGoHome} />;
  }
}

function CrashFallback({ error, onGoHome }: { error: Error; onGoHome: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="page" role="alert">
      <h1>{t("crash.heading")}</h1>
      <p className="settings__hint">{t("crash.body")}</p>
      <p className="settings__hint">
        <code>{error.message}</code>
      </p>
      <div className="timeline__nav">
        <button type="button" onClick={onGoHome}>
          {t("crash.back")}
        </button>
        <button type="button" onClick={() => window.location.reload()}>
          {t("crash.reload")}
        </button>
      </div>
    </div>
  );
}

// Wraps the page area: resets the boundary on every location change and
// routes "back home" through the real router (a plain link would also
// work, but navigate("/") keeps the recovery path identical to the
// tray's own "show app" behavior).
export function RouteErrorBoundary({ children }: { children: ReactNode }) {
  const { pathname, search } = useLocation();
  const navigate = useNavigate();
  return (
    <ErrorBoundary resetKey={`${pathname}${search}`} onGoHome={() => navigate("/")}>
      {children}
    </ErrorBoundary>
  );
}
