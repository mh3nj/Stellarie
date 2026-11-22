import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { Icon } from "@/components/icons";
import {
  getMyYearBindings,
  getCategoryBindings,
  getProjectBindings,
  type MyYearView,
  type CategoryRecord,
  type ProjectRecord,
} from "@/lib/bindings";
import { formatDuration } from "@/lib/time-ranges";

export function MyYear() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [year, setYear] = useState(new Date().getFullYear());
  const [view, setView] = useState<MyYearView | null>(null);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [myYearBindings, categoryBindings, projectBindings] = await Promise.all([
        getMyYearBindings(),
        getCategoryBindings(),
        getProjectBindings(),
      ]);
      const [yearView, cats, projs] = await Promise.all([
        myYearBindings.GetYear(year),
        categoryBindings.ListCategories(),
        projectBindings.ListProjects(true),
      ]);
      setView(yearView);
      setCategories(cats);
      setProjects(projs);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [year]);

  useEffect(() => {
    load();
  }, [load]);

  const categoryName = (id: number) => categories.find((c) => c.id === id)?.name ?? t("myYear.uncategorized");
  const projectName = (id: number) => projects.find((p) => p.id === id)?.name ?? t("myYear.unknownProject");

  const percentChange =
    view && view.hasPriorYearData && view.priorYearTotalSeconds > 0
      ? ((view.totalTrackedSeconds - view.priorYearTotalSeconds) / view.priorYearTotalSeconds) * 100
      : null;

  return (
    <div className="page">
      <h1>{t("myYear.heading")}</h1>

      <div className="timeline__controls">
        <div className="timeline__nav">
          <button onClick={() => setYear((y) => y - 1)} aria-label={t("common.previousLabel")}>‹</button>
          <span className="timeline__range-label">{year}</span>
          <button onClick={() => setYear((y) => y + 1)} aria-label={t("common.nextLabel")}>›</button>
          <button onClick={() => setYear(new Date().getFullYear())}>{t("common.thisYear")}</button>
        </div>
      </div>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : view ? (
        <>
          <section className="settings__section">
            <h2>{t("myYear.totalTrackedHeading")}</h2>
            <p className="analytics__trend-value">{formatDuration(view.totalTrackedSeconds)}</p>
            {percentChange !== null && (
              <p className="settings__hint">
                {t("myYear.vsPriorYear", {
                  percent: `${percentChange >= 0 ? "+" : ""}${percentChange.toFixed(0)}`,
                  year: year - 1,
                  amount: formatDuration(view.priorYearTotalSeconds),
                })}
              </p>
            )}
            {view.busiestMonthLabel && (
              <p className="settings__hint">
                {t("myYear.busiestMonth", { month: view.busiestMonthLabel, amount: formatDuration(view.busiestMonthSeconds) })}
              </p>
            )}
          </section>

          {view.categories.length > 0 && (
            <section className="settings__section">
              <h2>{t("myYear.categoriesHeading")}</h2>
              <ul className="rule-list">
                {view.categories.map((c) => (
                  <li key={c.categoryId} className="rule-list__item">
                    <span className="rule-list__pattern">{categoryName(c.categoryId)}</span>
                    <span>{formatDuration(c.seconds)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {view.topProjects.length > 0 && (
            <section className="settings__section">
              <h2>{t("myYear.topProjectsHeading")}</h2>
              <ul className="rule-list">
                {view.topProjects.map((p) => (
                  <li key={p.projectId} className="rule-list__item">
                    <span className="rule-list__pattern">{projectName(p.projectId)}</span>
                    <span>{formatDuration(p.seconds)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {view.longestSessions.length > 0 && (
            <section className="settings__section">
              <h2>{t("myYear.longestSessionsHeading")}</h2>
              <ul className="rule-list">
                {view.longestSessions.map((s, i) => (
                  <li key={i} className="rule-list__item">
                    <button
                      className="rule-list__result-title"
                      onClick={() => navigate(`/timeline?date=${s.dateKey}&zoom=day`)}
                      title={t("myYear.openInTimelineTitle")}
                    >
                      {s.title}
                    </button>
                    <span>{formatDuration(s.durationMinutes * 60)}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {view.goalsCurrentlyComplete.length > 0 && (
            <section className="settings__section">
              <h2>{t("myYear.goalsCompleteHeading")}</h2>
              <p className="settings__hint">{t("myYear.goalsCompleteHint", { year })}</p>
              <ul className="rule-list">
                {view.goalsCurrentlyComplete.map((name) => (
                  <li key={name} className="rule-list__item">
                    <span className="rule-list__pattern">{name}</span>
                    <span className="goal-status goal-status--met" aria-hidden="true">
                      <Icon name="check" size={15} />
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {view.abandonedProjects.length > 0 && (
            <section className="settings__section">
              <h2>{t("myYear.abandonedProjectsHeading")}</h2>
              <p className="settings__hint">{t("myYear.abandonedProjectsHint")}</p>
              <ul className="rule-list">
                {view.abandonedProjects.map((p) => (
                  <li key={p.name} className="rule-list__item">
                    <span className="rule-list__pattern">{p.name}</span>
                    <span>{p.explicit ? t("myYear.explicit") : t("myYear.inferred")}</span>
                    <span>{t("myYear.daysAgo", { days: p.daysSinceActivity })}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {view.aiNote && (
            <section className="settings__section">
              <h2>{t("myYear.aiNoteHeading")}</h2>
              <p className="settings__hint">{t("myYear.aiNoteHint")}</p>
              <p>{view.aiNote}</p>
            </section>
          )}
        </>
      ) : null}
    </div>
  );
}
