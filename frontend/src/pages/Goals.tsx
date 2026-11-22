import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  getBudgetBindings,
  getGoalBindings,
  getProjectBindings,
  getCategoryBindings,
  type BudgetStatus,
  type CategoryRecord,
  type GoalStatus,
  type ProjectRecord,
} from "@/lib/bindings";
import { formatDuration } from "@/lib/time-ranges";

const GOAL_TYPES = ["time", "project", "activity", "custom"] as const;
// Reuses Analytics' Day/Week/Month labels - same concept (a period
// name), so no separate translation set for it here.
const PERIODS = ["day", "week", "month"] as const;
const BUDGET_TYPES = ["maximum", "target"] as const;

const GOAL_TYPE_LABEL_KEYS: Record<(typeof GOAL_TYPES)[number], string> = {
  time: "goals.goalTypeTime",
  project: "goals.goalTypeProject",
  activity: "goals.goalTypeActivity",
  custom: "goals.goalTypeCustom",
};

const PERIOD_LABEL_KEYS: Record<string, string> = {
  day: "analytics.zoomDay",
  week: "analytics.zoomWeek",
  month: "analytics.zoomMonth",
};

const BUDGET_TYPE_LABEL_KEYS: Record<(typeof BUDGET_TYPES)[number], string> = {
  maximum: "goals.budgetTypeMaximum",
  target: "goals.budgetTypeTarget",
};

const SCHEDULE_LABEL_KEYS: Record<string, string> = {
  daily: "goals.scheduleDaily",
  weekly: "goals.scheduleWeekly",
  monthly: "goals.scheduleMonthly",
};

export function Goals() {
  const { t } = useTranslation();
  const [goals, setGoals] = useState<GoalStatus[]>([]);
  const [budgets, setBudgets] = useState<BudgetStatus[]>([]);
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [goalName, setGoalName] = useState("");
  const [goalType, setGoalType] = useState<(typeof GOAL_TYPES)[number]>(GOAL_TYPES[0]);
  const [goalProjectId, setGoalProjectId] = useState(0);
  const [goalTargetHours, setGoalTargetHours] = useState(10);
  const [goalPeriod, setGoalPeriod] = useState<(typeof PERIODS)[number]>("month");

  const [budgetName, setBudgetName] = useState("");
  const [budgetScope, setBudgetScope] = useState<"category" | "project">("category");
  const [budgetTargetId, setBudgetTargetId] = useState(0);
  const [budgetType, setBudgetType] = useState<(typeof BUDGET_TYPES)[number]>(BUDGET_TYPES[0]);
  const [budgetLimitMinutes, setBudgetLimitMinutes] = useState(60);
  const [budgetSchedule, setBudgetSchedule] = useState("daily");
  const [budgetWarnAt, setBudgetWarnAt] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [goalBindings, budgetBindings, projectBindings, categoryBindings] = await Promise.all([
        getGoalBindings(),
        getBudgetBindings(),
        getProjectBindings(),
        getCategoryBindings(),
      ]);
      const [goalRows, budgetRows, projectRows, categoryRows] = await Promise.all([
        goalBindings.ListWithStatus(),
        budgetBindings.ListWithStatus(),
        projectBindings.ListProjects(false),
        categoryBindings.ListCategories(),
      ]);
      setGoals(goalRows);
      setBudgets(budgetRows);
      setProjects(projectRows);
      setCategories(categoryRows);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleCreateGoal() {
    if (!goalName.trim()) return;
    try {
      const bindings = await getGoalBindings();
      await bindings.CreateGoal(goalName.trim(), goalType, goalProjectId, goalTargetHours, goalPeriod);
      setGoalName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleArchiveGoal(id: number) {
    try {
      const bindings = await getGoalBindings();
      await bindings.ArchiveGoal(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleCreateBudget() {
    if (!budgetName.trim() || !budgetTargetId) {
      setError(t("goals.pickCategoryOrProjectError"));
      return;
    }
    try {
      const bindings = await getBudgetBindings();
      const categoryId = budgetScope === "category" ? budgetTargetId : 0;
      const projectId = budgetScope === "project" ? budgetTargetId : 0;
      await bindings.CreateBudget(
        budgetName.trim(),
        categoryId,
        projectId,
        budgetType,
        budgetLimitMinutes,
        budgetSchedule,
        budgetWarnAt,
        false,
      );
      setBudgetName("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleArchiveBudget(id: number) {
    try {
      const bindings = await getBudgetBindings();
      await bindings.ArchiveBudget(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function projectName(id: number): string {
    return projects.find((p) => p.id === id)?.name ?? "-";
  }
  function categoryName(id: number): string {
    return categories.find((c) => c.id === id)?.name ?? "-";
  }
  function periodLabel(period: string): string {
    return PERIOD_LABEL_KEYS[period] ? t(PERIOD_LABEL_KEYS[period]) : period;
  }
  function scheduleLabel(schedule: string): string {
    return SCHEDULE_LABEL_KEYS[schedule] ? t(SCHEDULE_LABEL_KEYS[schedule]) : schedule;
  }

  return (
    <div className="page page--wide">
      <h1>{t("goals.heading")}</h1>

      {error && <p className="settings__error">{error}</p>}

      <section className="settings__section">
        <h2>{t("goals.yourGoalsHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : goals.length === 0 ? (
          <p className="settings__hint">{t("goals.noGoalsYet")}</p>
        ) : (
          <div className="goal-list">
            {goals.map((g) => (
              <div key={g.id} className="goal-card">
                <div className="goal-card__header">
                  <span>
                    {g.name}
                    {g.projectId > 0 && <span className="settings__hint"> · {projectName(g.projectId)}</span>}
                  </span>
                  <button onClick={() => handleArchiveGoal(g.id)}>{t("goals.archiveButton")}</button>
                </div>
                <div className="goal-card__bar">
                  <div className="goal-card__bar-fill" style={{ width: `${Math.min(g.percentComplete, 100)}%` }} />
                </div>
                <p className="settings__hint">
                  {t("goals.goalProgressLine", {
                    actual: g.actualHours.toFixed(1),
                    target: g.targetValue,
                    period: periodLabel(g.period),
                    percent: Math.round(g.percentComplete),
                  })}
                </p>
              </div>
            ))}
          </div>
        )}

        <div className="project-form" style={{ marginTop: "var(--space-4)" }}>
          <input placeholder={t("goals.goalNamePlaceholder")} value={goalName} onChange={(e) => setGoalName(e.target.value)} />
          <select value={goalType} onChange={(e) => setGoalType(e.target.value as (typeof GOAL_TYPES)[number])}>
            {GOAL_TYPES.map((gt) => (
              <option key={gt} value={gt}>
                {t(GOAL_TYPE_LABEL_KEYS[gt])}
              </option>
            ))}
          </select>
          <select value={goalProjectId} onChange={(e) => setGoalProjectId(Number(e.target.value))}>
            <option value={0}>{t("goals.noProjectOption")}</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
          <input
            type="number"
            min={0.5}
            step={0.5}
            value={goalTargetHours}
            onChange={(e) => setGoalTargetHours(Number(e.target.value))}
            style={{ width: 70 }}
          />
          <select value={goalPeriod} onChange={(e) => setGoalPeriod(e.target.value as (typeof PERIODS)[number])}>
            {PERIODS.map((p) => (
              <option key={p} value={p}>
                {t(PERIOD_LABEL_KEYS[p])}
              </option>
            ))}
          </select>
          <button onClick={handleCreateGoal}>{t("goals.addGoalButton")}</button>
        </div>
      </section>

      <section className="settings__section">
        <h2>{t("goals.yourBudgetsHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : budgets.length === 0 ? (
          <p className="settings__hint">{t("goals.noBudgetsYet")}</p>
        ) : (
          <div className="goal-list">
            {budgets.map((b) => {
              const overBudget = b.budgetType === "maximum" && b.actualMinutes > b.limitMinutes;
              const scopeName = b.categoryId ? categoryName(b.categoryId) : projectName(b.projectId);
              return (
                <div key={b.id} className="goal-card">
                  <div className="goal-card__header">
                    <span>
                      {b.name} <span className="settings__hint">· {scopeName}</span>
                    </span>
                    <button onClick={() => handleArchiveBudget(b.id)}>{t("goals.archiveButton")}</button>
                  </div>
                  <div className="goal-card__bar">
                    <div
                      className="goal-card__bar-fill"
                      style={{
                        width: `${Math.min((b.actualMinutes / Math.max(b.limitMinutes, 1)) * 100, 100)}%`,
                        background: overBudget ? "var(--danger)" : undefined,
                      }}
                    />
                  </div>
                  <p className="settings__hint">
                    {t("goals.budgetLine", {
                      actual: formatDuration(b.actualMinutes * 60),
                      limit: formatDuration(b.limitMinutes * 60),
                      schedule: scheduleLabel(b.schedule),
                      kind: b.budgetType === "maximum" ? t("goals.limitWord") : t("goals.targetWord"),
                    })}
                    {overBudget && t("goals.overBySuffix", { duration: formatDuration((b.actualMinutes - b.limitMinutes) * 60) })}
                  </p>
                </div>
              );
            })}
          </div>
        )}

        <div className="project-form" style={{ marginTop: "var(--space-4)" }}>
          <input placeholder={t("goals.budgetNamePlaceholder")} value={budgetName} onChange={(e) => setBudgetName(e.target.value)} />
          <select value={budgetScope} onChange={(e) => { setBudgetScope(e.target.value as "category" | "project"); setBudgetTargetId(0); }}>
            <option value="category">{t("goals.categoryOption")}</option>
            <option value="project">{t("goals.projectOption")}</option>
          </select>
          <select value={budgetTargetId} onChange={(e) => setBudgetTargetId(Number(e.target.value))}>
            <option value={0}>{t("goals.chooseOption")}</option>
            {(budgetScope === "category" ? categories : projects).map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <select value={budgetType} onChange={(e) => setBudgetType(e.target.value as (typeof BUDGET_TYPES)[number])}>
            {BUDGET_TYPES.map((bt) => (
              <option key={bt} value={bt}>
                {t(BUDGET_TYPE_LABEL_KEYS[bt])}
              </option>
            ))}
          </select>
          <input
            type="number"
            min={5}
            value={budgetLimitMinutes}
            onChange={(e) => setBudgetLimitMinutes(Number(e.target.value))}
            style={{ width: 70 }}
            title={t("goals.limitMinutesTitle")}
          />
          <input
            type="number"
            min={0}
            value={budgetWarnAt}
            onChange={(e) => setBudgetWarnAt(Number(e.target.value))}
            style={{ width: 70 }}
            title={t("goals.warnAtTitle")}
            placeholder={t("goals.warnAtPlaceholder")}
          />
          <select value={budgetSchedule} onChange={(e) => setBudgetSchedule(e.target.value)}>
            <option value="daily">{t("goals.dailyOption")}</option>
            <option value="weekly">{t("goals.weeklyOption")}</option>
            <option value="monthly">{t("goals.monthlyOption")}</option>
          </select>
          <button onClick={handleCreateBudget}>{t("goals.addBudgetButton")}</button>
        </div>
      </section>
    </div>
  );
}
