import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getPatternBindings, type PatternsView } from "@/lib/bindings";

export function Patterns() {
  const { t } = useTranslation();
  const [view, setView] = useState<PatternsView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dismissing, setDismissing] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const bindings = await getPatternBindings();
      setView(await bindings.GetInsights());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function dismiss(insightType: string) {
    setDismissing(insightType);
    try {
      const bindings = await getPatternBindings();
      await bindings.Dismiss(insightType);
      // Remove locally rather than re-fetching everything - the person
      // just asked for this one insight to go away; a full reload would
      // also be correct but adds a round-trip for no visible benefit.
      setView((prev) => (prev ? { ...prev, insights: prev.insights.filter((i) => i.insightType !== insightType) } : prev));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setDismissing(null);
    }
  }

  return (
    <div className="page">
      <h1>{t("patterns.heading")}</h1>
      <p className="settings__hint">{t("patterns.hint")}</p>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : view && view.insights.length > 0 ? (
        <section className="settings__section">
          <ul className="rule-list">
            {/* insight.text is Go-side generated prose from
                internal/patterns (median start times, etc.) - same
                deliberate English-only scoping as DigitalWaste's
                f.text above. */}
            {view.insights.map((insight) => (
              <li key={insight.insightType} className="rule-list__item">
                <span className="rule-list__pattern">{insight.text}</span>
                <button onClick={() => dismiss(insight.insightType)} disabled={dismissing === insight.insightType}>
                  {dismissing === insight.insightType ? "…" : t("patterns.dismiss")}
                </button>
              </li>
            ))}
          </ul>

          {view.aiNote && (
            <>
              <h2 style={{ marginTop: "var(--space-4)" }}>{t("patterns.aiNoteHeading")}</h2>
              <p className="settings__hint">{t("patterns.aiNoteHint")}</p>
              <p>{view.aiNote}</p>
            </>
          )}
        </section>
      ) : (
        <section className="settings__section">
          <p className="settings__hint">{t("patterns.emptyHint")}</p>
        </section>
      )}
    </div>
  );
}
