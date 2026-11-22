import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getProjectBindings, type ProjectRecord, type RuleRecord } from "@/lib/bindings";

const COLOR_TOKENS = ["accent", "chartSecondary", "success", "warning", "danger", "info"];
// Same naming-collision precedent as Settings.tsx (see handbook's
// self-correction log): "rt" not "t", since useTranslation() already
// binds t.
const RULE_TYPE_KEYS: { value: string; labelKey: string }[] = [
  { value: "process", labelKey: "projects.ruleTypeProcess" },
  { value: "window_title_contains", labelKey: "projects.ruleTypeWindowTitle" },
  { value: "path_contains", labelKey: "projects.ruleTypePathContains" },
  { value: "domain", labelKey: "projects.ruleTypeDomain" },
];

export function Projects() {
  const { t } = useTranslation();
  const [projects, setProjects] = useState<ProjectRecord[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [newName, setNewName] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [newColor, setNewColor] = useState(COLOR_TOKENS[0]);

  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [rules, setRules] = useState<RuleRecord[]>([]);
  const [ruleType, setRuleType] = useState(RULE_TYPE_KEYS[0].value);
  const [rulePattern, setRulePattern] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const bindings = await getProjectBindings();
      setProjects(await bindings.ListProjects(showArchived));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [showArchived]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleCreate() {
    if (!newName.trim()) return;
    try {
      const bindings = await getProjectBindings();
      await bindings.CreateProject(newName.trim(), newDescription.trim(), newColor);
      setNewName("");
      setNewDescription("");
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleArchiveToggle(p: ProjectRecord) {
    try {
      const bindings = await getProjectBindings();
      if (p.archived) await bindings.UnarchiveProject(p.id);
      else await bindings.ArchiveProject(p.id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function toggleExpanded(p: ProjectRecord) {
    if (expandedId === p.id) {
      setExpandedId(null);
      return;
    }
    setExpandedId(p.id);
    setRulePattern("");
    try {
      const bindings = await getProjectBindings();
      setRules(await bindings.ListRulesForProject(p.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleAddRule(projectId: number) {
    if (!rulePattern.trim()) return;
    try {
      const bindings = await getProjectBindings();
      await bindings.AddRule(projectId, ruleType, rulePattern.trim(), rules.length * 10);
      setRulePattern("");
      setRules(await bindings.ListRulesForProject(projectId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDeleteRule(projectId: number, ruleId: number) {
    try {
      const bindings = await getProjectBindings();
      await bindings.DeleteRule(ruleId);
      setRules(await bindings.ListRulesForProject(projectId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="page page--wide">
      <h1>{t("projects.heading")}</h1>
      <p className="settings__hint">{t("projects.hint")}</p>

      {error && <p className="settings__error">{error}</p>}

      <section className="settings__section">
        <h2>{t("projects.newProjectHeading")}</h2>
        <div className="project-form">
          <input placeholder={t("projects.namePlaceholder")} value={newName} onChange={(e) => setNewName(e.target.value)} />
          <input
            placeholder={t("projects.descriptionPlaceholder")}
            value={newDescription}
            onChange={(e) => setNewDescription(e.target.value)}
          />
          <select value={newColor} onChange={(e) => setNewColor(e.target.value)}>
            {COLOR_TOKENS.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
          <button onClick={handleCreate}>{t("projects.createButton")}</button>
        </div>
      </section>

      <section className="settings__section">
        <div className="theme-list__header-row">
          <h2>{t("projects.yourProjectsHeading")}</h2>
          <label className="settings__checkbox">
            <input type="checkbox" checked={showArchived} onChange={(e) => setShowArchived(e.target.checked)} />
            {t("projects.showArchived")}
          </label>
        </div>

        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : (
          <ul className="theme-list">
            {projects.map((p) => (
              <li key={p.id} className="project-list__item">
                <div
                  className="theme-list__item"
                  role="button"
                  tabIndex={0}
                  aria-expanded={expandedId === p.id}
                  onClick={() => toggleExpanded(p)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      toggleExpanded(p);
                    }
                  }}
                >
                  <span className="theme-list__swatch" style={{ background: `var(--${p.colorToken})`, borderColor: `var(--${p.colorToken})` }} />
                  <span className="theme-list__name">
                    {p.name}
                    {p.archived && <span className="theme-list__badge">{t("projects.archivedBadge")}</span>}
                  </span>
                  <span className="theme-list__actions">
                    <button onClick={(e) => { e.stopPropagation(); handleArchiveToggle(p); }}>
                      {p.archived ? t("projects.unarchiveButton") : t("projects.archiveButton")}
                    </button>
                  </span>
                </div>

                {expandedId === p.id && (
                  <div className="project-list__rules">
                    <h3>{t("projects.rulesHeading")}</h3>
                    {rules.length === 0 && <p className="settings__hint">{t("projects.noRulesHint")}</p>}
                    <ul className="rule-list">
                      {rules.map((r) => (
                        <li key={r.id} className="rule-list__item">
                          <span className="rule-list__type">{r.type}</span>
                          <span className="rule-list__pattern">{r.pattern}</span>
                          <button onClick={() => handleDeleteRule(p.id, r.id)}>{t("projects.removeButton")}</button>
                        </li>
                      ))}
                    </ul>
                    <div className="project-form project-form--rule">
                      <select value={ruleType} onChange={(e) => setRuleType(e.target.value)}>
                        {RULE_TYPE_KEYS.map((rt) => (
                          <option key={rt.value} value={rt.value}>
                            {t(rt.labelKey)}
                          </option>
                        ))}
                      </select>
                      <input
                        placeholder={t("projects.rulePatternPlaceholder")}
                        value={rulePattern}
                        onChange={(e) => setRulePattern(e.target.value)}
                      />
                      <button onClick={() => handleAddRule(p.id)}>{t("projects.addRuleButton")}</button>
                    </div>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
