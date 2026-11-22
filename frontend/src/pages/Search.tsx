import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { getSearchBindings, type SearchResultView } from "@/lib/bindings";

// Deliberately NOT translated. internal/search is an explicitly
// English-only bounded-vocabulary parser (real date phrases, whole-word
// entity names, superlatives - no NLP, no synonyms, see that package's
// own doc comment). These example buttons set the query to this exact
// literal text and submit it - translating them would either silently
// break (the backend wouldn't parse a Persian or German date phrase) or
// require building real multi-language query parsing, which spec
// section 69 never asked for and this pass doesn't attempt. Same
// reasoning applies to the placeholder below.
const EXAMPLES = [
  "today",
  "last month",
  "August 10",
  "longest session",
  "gaming last week",
];

export function Search() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [interpretation, setInterpretation] = useState<string | null>(null);
  const [results, setResults] = useState<SearchResultView[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function runSearch(q: string) {
    if (!q.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const bindings = await getSearchBindings();
      const view = await bindings.Search(q);
      setInterpretation(view.interpretation);
      // Normalize: an empty result set arrives as null (Go nil slice),
      // and .length/.map on null crashes the page - same class of bug
      // that black-screened Time Receipt (see its own comment).
      setResults(view.results ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    runSearch(query);
  }

  return (
    <div className="page">
      <h1>{t("search.heading")}</h1>
      <p className="settings__hint">
        {t("search.hintIntro")}{" "}
        {EXAMPLES.map((ex, i) => (
          <span key={ex}>
            {i > 0 && ", "}
            <button
              type="button"
              className="rule-list__link"
              onClick={() => {
                setQuery(ex);
                runSearch(ex);
              }}
            >
              {ex}
            </button>
          </span>
        ))}
      </p>

      <form onSubmit={handleSubmit} className="settings__options">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("search.placeholder")}
          className="settings__select"
        />
        <button type="submit" disabled={loading}>
          {loading ? t("search.searchingButton") : t("search.searchButton")}
        </button>
      </form>

      {error && <p className="settings__error">{error}</p>}

      {interpretation && (
        <section className="settings__section">
          {/* interpretation is Go-side generated ("Showing sessions from
              today", etc.) - same English-only scoping as EXAMPLES
              above and DigitalWaste/Patterns' finding text. */}
          <p className="settings__hint">{interpretation}</p>

          {results.length > 0 && (
            <ul className="rule-list">
              {results.map((r) => (
                <li key={r.sessionId} className="rule-list__item">
                  <button
                    className="rule-list__result-title"
                    onClick={() => navigate(`/timeline?date=${r.dateKey}&zoom=day`)}
                    title={t("search.openInTimelineTitle")}
                  >
                    {r.title}
                  </button>
                  <span>{r.subtitle}</span>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </div>
  );
}
