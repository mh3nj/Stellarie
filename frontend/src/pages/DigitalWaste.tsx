import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getWasteBindings, type WasteView } from "@/lib/bindings";

export function DigitalWaste() {
  const { t } = useTranslation();
  const [view, setView] = useState<WasteView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError(null);
      try {
        const bindings = await getWasteBindings();
        const v = await bindings.GetFindings();
        if (!cancelled) setView(v);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : String(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page">
      <h1>{t("digitalWaste.heading")}</h1>
      <p className="settings__hint">{t("digitalWaste.hint")}</p>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : view && view.findings.length > 0 ? (
        <section className="settings__section">
          <ul className="rule-list">
            {/* f.text comes from internal/waste's Go-side deterministic
                findings generator - real English sentences built from
                real session data, not a UI label. Translating this would
                mean either a full backend i18n pass (out of scope for
                this frontend-strings extraction) or re-translating
                dynamically-assembled English at render time, which risks
                silently misrepresenting what the finding actually says - left in English deliberately, same reasoning as
                Search's EXAMPLES below. */}
            {view.findings.map((f, i) => (
              <li key={i} className="rule-list__item">
                <span className="rule-list__pattern">{f.text}</span>
              </li>
            ))}
          </ul>

          {view.aiNote && (
            <>
              <h2 style={{ marginTop: "var(--space-4)" }}>{t("digitalWaste.suggestionHeading")}</h2>
              <p className="settings__hint">{t("digitalWaste.suggestionHint")}</p>
              <p>{view.aiNote}</p>
            </>
          )}
        </section>
      ) : (
        <section className="settings__section">
          <p className="settings__hint">{t("digitalWaste.emptyHint")}</p>
        </section>
      )}
    </div>
  );
}
