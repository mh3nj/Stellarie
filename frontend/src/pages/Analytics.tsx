import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  getAnalyticsBindings,
  getCategoryBindings,
  getProjectBindings,
  type ApplicationCategoryRecord,
  type CategoryRecord,
  type CategoryRuleRecord,
  type CategoryTotal,
  type ContextSwitchStats,
  type ProjectRecord,
  type ProjectTotal,
  type Trend,
} from "@/lib/bindings";
import { rangeFor, shiftAnchor, formatDuration, formatRangeLabel, type TimelineZoom } from "@/lib/time-ranges";
import { useWeekStartDay } from "@/lib/week-start";

const ZOOMS: { value: TimelineZoom; labelKey: string }[] = [
  { value: "day", labelKey: "analytics.zoomDay" },
  { value: "week", labelKey: "analytics.zoomWeek" },
  { value: "month", labelKey: "analytics.zoomMonth" },
];

// Reuses Projects.tsx's rule-type vocabulary verbatim - same four rule
// types, same underlying internal/categories.RuleType constants (spec
// §65's category-rules extension mirrors the project rule engine
// exactly), so the same translated labels apply without duplication.
// Named "rt" in the .map below per this codebase's established
// naming-collision precedent (see the handbook) - "t" would shadow
// useTranslation()'s t.
const RULE_TYPE_KEYS: { value: string; labelKey: string }[] = [
  { value: "process", labelKey: "projects.ruleTypeProcess" },
  { value: "window_title_contains", labelKey: "projects.ruleTypeWindowTitle" },
  { value: "path_contains", labelKey: "projects.ruleTypePathContains" },
  { value: "domain", labelKey: "projects.ruleTypeDomain" },
];

interface BreakdownRow {
  id: number;
  label: string;
  colorToken: string;
  seconds: number;
}

function BreakdownBars({ title, rows, total }: { title: string; rows: BreakdownRow[]; total: number }) {
  const { t } = useTranslation();
  const sorted = [...rows].sort((a, b) => b.seconds - a.seconds);
  return (
    <section className="settings__section">
      <h2>{title}</h2>
      {sorted.length === 0 || total === 0 ? (
        <p className="settings__hint">{t("analytics.nothingTrackedRange")}</p>
      ) : (
        <div className="breakdown-bars">
          {sorted.map((row) => (
            <div key={row.id} className="breakdown-bars__row">
              <span className="breakdown-bars__label">{row.label}</span>
              <div className="breakdown-bars__track">
                <div
                  className="breakdown-bars__fill"
                  style={{
                    width: `${Math.max((row.seconds / total) * 100, 1)}%`,
                    background: row.colorToken ? `var(--${row.colorToken})` : "var(--chartPrimary)",
                  }}
                />
              </div>
              <span className="breakdown-bars__value">{formatDuration(row.seconds)}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

export function Analytics() {
  const { t } = useTranslation();
  const [zoom, setZoom] = useState<TimelineZoom>("week");
  const [anchor, setAnchor] = useState(new Date());
  const [categoryTotals, setCategoryTotals] = useState<CategoryTotal[]>([]);
  const [projectTotals, setProjectTotals] = useState<ProjectTotal[]>([]);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [contextStats, setContextStats] = useState<ContextSwitchStats | null>(null);
  const [trend, setTrend] = useState<Trend | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [applications, setApplications] = useState<ApplicationCategoryRecord[]>([]);
  const [showAppMapping, setShowAppMapping] = useState(false);
  const [newCategoryName, setNewCategoryName] = useState("");

  const [expandedCategoryId, setExpandedCategoryId] = useState<number | null>(null);
  const [categoryRules, setCategoryRules] = useState<CategoryRuleRecord[]>([]);
  const [categoryRuleType, setCategoryRuleType] = useState(RULE_TYPE_KEYS[0].value);
  const [categoryRulePattern, setCategoryRulePattern] = useState("");

  const weekStartDay = useWeekStartDay();
  const [rangeStart, rangeEnd] = rangeFor(zoom, anchor, weekStartDay);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [analytics, categoryBindings, projectBindings] = await Promise.all([
        getAnalyticsBindings(),
        getCategoryBindings(),
        getProjectBindings(),
      ]);
      const [catTotals, projTotals, cats, projs, stats, trendResult] = await Promise.all([
        analytics.GetCategoryBreakdown(rangeStart, rangeEnd),
        analytics.GetProjectBreakdown(rangeStart, rangeEnd),
        categoryBindings.ListCategories(),
        projectBindings.ListProjects(false),
        analytics.GetContextSwitchStats(rangeStart, rangeEnd),
        analytics.GetTrend(rangeStart, rangeEnd),
      ]);
      setCategoryTotals(catTotals);
      setProjectTotals(projTotals);
      setCategories(cats);
      setProjects(projs);
      setContextStats(stats);
      setTrend(trendResult);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rangeStart, rangeEnd]);

  useEffect(() => {
    load();
  }, [load]);

  async function loadApplications() {
    try {
      const bindings = await getCategoryBindings();
      setApplications(await bindings.ListApplications());
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleToggleAppMapping() {
    const next = !showAppMapping;
    setShowAppMapping(next);
    if (next) await loadApplications();
  }

  async function handleSetApplicationCategory(applicationId: number, categoryId: number) {
    try {
      const bindings = await getCategoryBindings();
      await bindings.SetApplicationCategory(applicationId, categoryId);
      await loadApplications();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleCreateCategory() {
    if (!newCategoryName.trim()) return;
    try {
      const bindings = await getCategoryBindings();
      await bindings.CreateCategory(newCategoryName.trim(), "chartSecondary");
      setNewCategoryName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function toggleCategoryExpanded(c: CategoryRecord) {
    if (expandedCategoryId === c.id) {
      setExpandedCategoryId(null);
      return;
    }
    setExpandedCategoryId(c.id);
    setCategoryRulePattern("");
    try {
      const bindings = await getCategoryBindings();
      setCategoryRules(await bindings.ListRulesForCategory(c.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleAddCategoryRule(categoryId: number) {
    if (!categoryRulePattern.trim()) return;
    try {
      const bindings = await getCategoryBindings();
      await bindings.AddRule(categoryId, categoryRuleType, categoryRulePattern.trim(), categoryRules.length * 10);
      setCategoryRulePattern("");
      setCategoryRules(await bindings.ListRulesForCategory(categoryId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDeleteCategoryRule(categoryId: number, ruleId: number) {
    try {
      const bindings = await getCategoryBindings();
      await bindings.DeleteRule(ruleId);
      setCategoryRules(await bindings.ListRulesForCategory(categoryId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  const categoryRows: BreakdownRow[] = categoryTotals.map((ct) => {
    const cat = categories.find((c) => c.id === ct.categoryId);
    return { id: ct.categoryId, label: cat?.name ?? t("analytics.uncategorizedOption"), colorToken: cat?.colorToken ?? "", seconds: ct.seconds };
  });
  const projectRows: BreakdownRow[] = projectTotals.map((pt) => {
    const proj = projects.find((p) => p.id === pt.projectId);
    return { id: pt.projectId, label: proj?.name ?? t("analytics.unassignedOption"), colorToken: proj?.colorToken ?? "", seconds: pt.seconds };
  });

  const categoryTotal = categoryRows.reduce((sum, r) => sum + r.seconds, 0);
  const projectTotal = projectRows.reduce((sum, r) => sum + r.seconds, 0);

  const activeZoom = ZOOMS.find((z) => z.value === zoom);

  return (
    <div className="page page--wide">
      <h1>{t("analytics.heading")}</h1>

      <div className="timeline__controls">
        <div className="timeline__zoom">
          {ZOOMS.map((z) => (
            <button key={z.value} className={z.value === zoom ? "timeline__zoom-btn--active" : ""} onClick={() => setZoom(z.value)}>
              {t(z.labelKey)}
            </button>
          ))}
        </div>
        <div className="timeline__nav">
          <button onClick={() => setAnchor(shiftAnchor(zoom, anchor, -1))} aria-label={t("common.previousLabel")}>‹</button>
          <span className="timeline__range-label">{formatRangeLabel(zoom, anchor, weekStartDay)}</span>
          <button onClick={() => setAnchor(shiftAnchor(zoom, anchor, 1))} aria-label={t("common.nextLabel")}>›</button>
          <button onClick={() => setAnchor(new Date())}>{t("common.today")}</button>
        </div>
      </div>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : (
        <>
          {trend && (
            <section className="settings__section">
              <h2>{t("analytics.totalTrackedHeading")}</h2>
              <p className="analytics__trend-value">{formatDuration(trend.currentSeconds)}</p>
              {trend.previousSeconds > 0 && (
                <p className="settings__hint">
                  {t("analytics.vsPrevious", {
                    arrow: trend.percentChange >= 0 ? "↑" : "↓",
                    percent: Math.abs(Math.round(trend.percentChange)),
                    zoom: activeZoom ? t(activeZoom.labelKey) : zoom,
                    amount: formatDuration(trend.previousSeconds),
                  })}
                </p>
              )}
            </section>
          )}

          <BreakdownBars title={t("analytics.byCategoryTitle")} rows={categoryRows} total={categoryTotal} />
          <BreakdownBars title={t("analytics.byProjectTitle")} rows={projectRows} total={projectTotal} />

          {contextStats && (
            <section className="settings__section">
              <h2>{t("analytics.digitalFrictionHeading")}</h2>
              <p className="settings__hint">{t("analytics.digitalFrictionHint")}</p>
              <div className="analytics__stat-grid">
                <div className="analytics__stat">
                  <span className="analytics__stat-value">{contextStats.switches}</span>
                  <span className="analytics__stat-label">{t("analytics.contextSwitchesLabel")}</span>
                </div>
                <div className="analytics__stat">
                  <span className="analytics__stat-value">{formatDuration(contextStats.averageSeconds)}</span>
                  <span className="analytics__stat-label">{t("analytics.averageSessionLabel")}</span>
                </div>
                <div className="analytics__stat">
                  <span className="analytics__stat-value">{formatDuration(contextStats.longestSeconds)}</span>
                  <span className="analytics__stat-label">{t("analytics.longestSessionLabel")}</span>
                </div>
                <div className="analytics__stat">
                  <span className="analytics__stat-value">{contextStats.sessionCount}</span>
                  <span className="analytics__stat-label">{t("analytics.totalSessionsLabel")}</span>
                </div>
              </div>
            </section>
          )}

          <section className="settings__section">
            <h2>{t("analytics.categoriesHeading")}</h2>
            <div className="project-form">
              <input
                placeholder={t("analytics.newCategoryPlaceholder")}
                value={newCategoryName}
                onChange={(e) => setNewCategoryName(e.target.value)}
              />
              <button onClick={handleCreateCategory}>{t("analytics.createButton")}</button>
            </div>
            <ul className="rule-list" style={{ marginTop: "var(--space-3)" }}>
              {categories.map((c) => (
                <li key={c.id} className="project-list__item">
                  <div
                    className="theme-list__item"
                    role="button"
                    tabIndex={0}
                    aria-expanded={expandedCategoryId === c.id}
                    onClick={() => toggleCategoryExpanded(c)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        toggleCategoryExpanded(c);
                      }
                    }}
                  >
                    <span
                      className="theme-list__swatch"
                      style={{ width: 14, height: 14, background: `var(--${c.colorToken})`, borderColor: `var(--${c.colorToken})` }}
                    />
                    <span className="rule-list__pattern">{c.name}</span>
                    {c.isBuiltin && <span className="theme-list__badge">{t("analytics.builtinBadge")}</span>}
                  </div>

                  {expandedCategoryId === c.id && (
                    <div className="project-list__rules">
                      <h3>{t("projects.rulesHeading")}</h3>
                      {categoryRules.length === 0 && <p className="settings__hint">{t("projects.noRulesHint")}</p>}
                      <ul className="rule-list">
                        {categoryRules.map((r) => (
                          <li key={r.id} className="rule-list__item">
                            <span className="rule-list__type">{r.type}</span>
                            <span className="rule-list__pattern">{r.pattern}</span>
                            <button onClick={() => handleDeleteCategoryRule(c.id, r.id)}>{t("projects.removeButton")}</button>
                          </li>
                        ))}
                      </ul>
                      <div className="project-form project-form--rule">
                        <select value={categoryRuleType} onChange={(e) => setCategoryRuleType(e.target.value)}>
                          {RULE_TYPE_KEYS.map((rt) => (
                            <option key={rt.value} value={rt.value}>
                              {t(rt.labelKey)}
                            </option>
                          ))}
                        </select>
                        <input
                          placeholder={t("projects.rulePatternPlaceholder")}
                          value={categoryRulePattern}
                          onChange={(e) => setCategoryRulePattern(e.target.value)}
                        />
                        <button onClick={() => handleAddCategoryRule(c.id)}>{t("projects.addRuleButton")}</button>
                      </div>
                    </div>
                  )}
                </li>
              ))}
            </ul>

            <button onClick={handleToggleAppMapping} style={{ marginTop: "var(--space-3)" }}>
              {showAppMapping ? t("analytics.hideAppMappingButton") : t("analytics.showAppMappingButton")}
            </button>

            {showAppMapping && (
              <ul className="rule-list" style={{ marginTop: "var(--space-3)" }}>
                {applications.map((a) => (
                  <li key={a.applicationId} className="rule-list__item">
                    <span className="rule-list__pattern">{a.processName}</span>
                    <select
                      value={a.categoryId}
                      onChange={(e) => handleSetApplicationCategory(a.applicationId, Number(e.target.value))}
                    >
                      <option value={0}>
                        {a.defaultCategoryId
                          ? t("analytics.defaultCategoryOption", { name: categories.find((c) => c.id === a.defaultCategoryId)?.name ?? "-" })
                          : t("analytics.uncategorizedOption")}
                      </option>
                      {categories.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name}
                        </option>
                      ))}
                    </select>
                  </li>
                ))}
                {applications.length === 0 && <p className="settings__hint">{t("analytics.noApplicationsTracked")}</p>}
              </ul>
            )}
          </section>
        </>
      )}
    </div>
  );
}
