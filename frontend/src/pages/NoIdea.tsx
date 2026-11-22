import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/components/icons";
import { getIdeaBindings, getTimerBindings, type SuggestionView, type ProjectIdeaView } from "@/lib/bindings";

const TIME_OPTIONS: { minutes: number; labelKey: string }[] = [
  { minutes: 15, labelKey: "noIdea.time15" },
  { minutes: 30, labelKey: "noIdea.time30" },
  { minutes: 60, labelKey: "noIdea.time60" },
  { minutes: 120, labelKey: "noIdea.time120" },
  { minutes: 0, labelKey: "noIdea.timeWhatever" },
];

const INTENT_OPTIONS: { value: string; labelKey: string }[] = [
  { value: "productive", labelKey: "noIdea.intentProductive" },
  { value: "creative", labelKey: "noIdea.intentCreative" },
  { value: "fun", labelKey: "noIdea.intentFun" },
  { value: "learn", labelKey: "noIdea.intentLearn" },
  { value: "surprise", labelKey: "noIdea.intentSurprise" },
];

const PLATFORM_OPTIONS = [
  { value: "any", labelKey: "noIdea.platformAny" },
  { value: "desktop", labelKey: "noIdea.platformDesktop" },
  { value: "web", labelKey: "noIdea.platformWeb" },
  { value: "cli", labelKey: "noIdea.platformCli" },
];

type Mode = "start" | "suggest" | "build";

/** The little "1 2 3" trail at the top of each flow. Purely presentational:
 * it reflects where the person is, so a multi-step flow never feels like a
 * form with a hidden ending. */
function Stepper({ steps, current }: { steps: string[]; current: number }) {
  return (
    <ol className="no-idea__stepper">
      {steps.map((label, i) => {
        const state = i < current ? "done" : i === current ? "active" : "todo";
        return (
          <li key={label} className={`no-idea__step no-idea__step--${state}`}>
            <span className="no-idea__step-bullet">
              {state === "done" ? <Icon name="check" size={12} /> : i + 1}
            </span>
            <span className="no-idea__step-label">{label}</span>
          </li>
        );
      })}
    </ol>
  );
}

export function NoIdea() {
  const { t } = useTranslation();
  const [mode, setMode] = useState<Mode>("start");
  const [error, setError] = useState<string | null>(null);

  // Suggest flow state
  const [minutes, setMinutes] = useState<number | null>(null);
  const [intent, setIntent] = useState<string | null>(null);
  const [historyId, setHistoryId] = useState<number | null>(null);
  const [suggestions, setSuggestions] = useState<SuggestionView[]>([]);
  const [loadingSuggestions, setLoadingSuggestions] = useState(false);
  const [startedIndex, setStartedIndex] = useState<number | null>(null);

  // Build-something flow state
  const [buildMinutes, setBuildMinutes] = useState(60);
  const [platform, setPlatform] = useState("any");
  const [idea, setIdea] = useState<ProjectIdeaView | null>(null);
  const [loadingIdea, setLoadingIdea] = useState(false);
  const [starting, setStarting] = useState(false);
  const [started, setStarted] = useState(false);

  async function generateSuggestions(m: number, i: string) {
    setLoadingSuggestions(true);
    setError(null);
    setStartedIndex(null);
    try {
      const bindings = await getIdeaBindings();
      const result = await bindings.Generate(m, i);
      setHistoryId(result.historyId);
      setSuggestions(result.suggestions);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingSuggestions(false);
    }
  }

  function pickTime(m: number) {
    setMinutes(m);
    if (intent) generateSuggestions(m, intent);
  }

  function pickIntent(i: string) {
    setIntent(i);
    if (minutes !== null) generateSuggestions(minutes, i);
  }

  async function handleAccept(s: SuggestionView, index: number) {
    setError(null);
    try {
      const ideaBindings = await getIdeaBindings();
      if (historyId) await ideaBindings.RecordAcceptance(historyId, index);

      const timerBindings = await getTimerBindings();
      const duration = minutes && minutes > 0 ? minutes : 30;
      if (s.kind === "project" && s.projectId) {
        await timerBindings.StartCountdown("project", s.projectId, s.title, duration);
      } else {
        await timerBindings.StartCountdown("custom", 0, s.title, duration);
      }
      setStartedIndex(index);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  function resetSuggestFlow() {
    setMinutes(null);
    setIntent(null);
    setSuggestions([]);
    setHistoryId(null);
    setStartedIndex(null);
  }

  async function generateIdea() {
    setLoadingIdea(true);
    setError(null);
    setStarted(false);
    try {
      const bindings = await getIdeaBindings();
      const result = await bindings.GenerateProjectIdea(buildMinutes, platform);
      setIdea(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingIdea(false);
    }
  }

  async function handleStartProject() {
    if (!idea) return;
    setStarting(true);
    setError(null);
    try {
      const bindings = await getIdeaBindings();
      await bindings.StartProjectFromIdea(idea.title, idea.description);
      setStarted(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setStarting(false);
    }
  }

  const timeLabelKey = TIME_OPTIONS.find((opt) => opt.minutes === minutes)?.labelKey;
  const intentLabelKey = INTENT_OPTIONS.find((opt) => opt.value === intent)?.labelKey;

  if (mode === "start") {
    return (
      <div className="page page--wide no-idea">
        <div className="no-idea__hero">
          <span className="no-idea__hero-icon">
            <Icon name="noIdea" size={30} />
          </span>
          <h1 className="no-idea__hero-title">{t("noIdea.heroTitle")}</h1>
          <p className="no-idea__hero-text">{t("noIdea.heroText")}</p>
          <div className="no-idea__paths">
            <button className="no-idea__path no-idea__path--primary" onClick={() => setMode("suggest")}>
              <span className="no-idea__path-icon">
                <Icon name="noIdea" size={22} />
              </span>
              <span className="no-idea__path-title">{t("noIdea.bigButtonLabel")}</span>
              <span className="no-idea__path-text">{t("noIdea.suggestPathText")}</span>
            </button>
            <button className="no-idea__path" onClick={() => setMode("build")}>
              <span className="no-idea__path-icon">
                <Icon name="package" size={22} />
              </span>
              <span className="no-idea__path-title">{t("noIdea.buildSomethingButton")}</span>
              <span className="no-idea__path-text">{t("noIdea.buildPathText")}</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (mode === "build") {
    return (
      <div className="page page--wide no-idea">
        <h1>{t("noIdea.buildHeading")}</h1>
        <Stepper steps={[t("noIdea.stepDetails"), t("noIdea.stepIdeas")]} current={idea ? 1 : 0} />
        {error && <p className="settings__error">{error}</p>}

        <section className="no-idea__panel">
          <div className="no-idea__field">
            <span className="no-idea__field-label">{t("noIdea.timeBudgetLabel")}</span>
            <div className="chip-grid">
              {TIME_OPTIONS.map((opt) => (
                <button
                  key={opt.minutes}
                  className={`chip${buildMinutes === opt.minutes ? " chip--selected" : ""}`}
                  aria-pressed={buildMinutes === opt.minutes}
                  onClick={() => setBuildMinutes(opt.minutes)}
                >
                  <Icon name="clock" size={15} />
                  <span className="chip__label">{t(opt.labelKey)}</span>
                </button>
              ))}
            </div>
          </div>

          <div className="no-idea__field">
            <span className="no-idea__field-label">{t("noIdea.platformLabel")}</span>
            <div className="chip-grid">
              {PLATFORM_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  className={`chip${platform === opt.value ? " chip--selected" : ""}`}
                  aria-pressed={platform === opt.value}
                  onClick={() => setPlatform(opt.value)}
                >
                  {t(opt.labelKey)}
                </button>
              ))}
            </div>
          </div>

          <div className="no-idea__actions">
            <button className="no-idea__cta" onClick={generateIdea} disabled={loadingIdea}>
              <Icon name="refresh" size={15} />
              {idea ? t("noIdea.regenerateButton") : t("noIdea.generateIdeaButton")}
            </button>
            <button className="no-idea__ghost" onClick={() => setMode("start")}>
              {t("noIdea.backButton")}
            </button>
          </div>
        </section>

        {loadingIdea ? (
          <p className="settings__hint">{t("noIdea.thinkingLabel")}</p>
        ) : idea ? (
          <div className="project-idea">
            <span className="project-idea__label">{t("noIdea.projectIdeaLabel")}</span>
            <input
              className="project-idea__title"
              value={idea.title}
              aria-label={t("noIdea.projectIdeaLabel")}
              onChange={(e) => setIdea({ ...idea, title: e.target.value })}
            />
            <textarea
              className="project-idea__description"
              value={idea.description}
              aria-label={t("noIdea.projectIdeaLabel")}
              onChange={(e) => setIdea({ ...idea, description: e.target.value })}
            />
            <div className="project-idea__meta">
              <span>{t("noIdea.estimatedLabel", { value: idea.estimatedLabel })}</span>
              <span>{t("noIdea.difficultyLabel", { value: idea.difficulty })}</span>
            </div>
            <p className="project-idea__why">{t("noIdea.whyLabel", { value: idea.why })}</p>
            <p className="project-idea__tweak">{t("noIdea.tweakHint")}</p>
            <div className="focus-hero__actions">
              <button onClick={handleStartProject} disabled={starting || started}>
                {started ? (
                  <>
                    <Icon name="check" size={15} />
                    {t("noIdea.projectStartedButton")}
                  </>
                ) : starting ? (
                  t("noIdea.startingButton")
                ) : (
                  <>
                    <Icon name="package" size={15} />
                    {t("noIdea.startProjectButton")}
                  </>
                )}
              </button>
            </div>
            {started && <p className="settings__hint">{t("noIdea.startedHint")}</p>}
          </div>
        ) : (
          <p className="settings__hint">{t("noIdea.pickTimeAndPlatformHint")}</p>
        )}
      </div>
    );
  }

  // mode === "suggest"
  const step = minutes === null ? 0 : intent === null ? 1 : 2;
  return (
    <div className="page page--wide no-idea">
      <h1>{t("noIdea.noIdeaHeading")}</h1>
      <Stepper steps={[t("noIdea.stepTime"), t("noIdea.stepVibe"), t("noIdea.stepIdeas")]} current={step} />
      {error && <p className="settings__error">{error}</p>}

      <section className="no-idea__panel">
        {minutes === null ? (
          <>
            <p className="no-idea__hint">{t("noIdea.pickTimeHint")}</p>
            <div className="chip-grid">
              {TIME_OPTIONS.map((opt) => (
                <button key={opt.minutes} className="chip" onClick={() => pickTime(opt.minutes)}>
                  <Icon name="clock" size={15} />
                  <span className="chip__label">{t(opt.labelKey)}</span>
                </button>
              ))}
            </div>
          </>
        ) : intent === null ? (
          <>
            <p className="no-idea__hint">{t("noIdea.pickVibeHint")}</p>
            <div className="chip-grid">
              {INTENT_OPTIONS.map((opt) => (
                <button key={opt.value} className="chip" onClick={() => pickIntent(opt.value)}>
                  {t(opt.labelKey)}
                </button>
              ))}
            </div>
            <div className="no-idea__actions">
              <button className="no-idea__ghost" onClick={resetSuggestFlow}>
                {t("noIdea.backButton")}
              </button>
            </div>
          </>
        ) : (
          <>
            <div className="no-idea__summary">
              <span className="chip chip--static">
                <Icon name="clock" size={15} />
                <span className="chip__label">{timeLabelKey ? t(timeLabelKey) : t("noIdea.whateverYouveGot")}</span>
              </span>
              <span className="chip chip--static">{intentLabelKey ? t(intentLabelKey) : intent}</span>
            </div>

            <h2 className="no-idea__section-head">{t("noIdea.hereAreIdeas")}</h2>
            {loadingSuggestions ? (
              <p className="settings__hint">{t("noIdea.thinkingLabel")}</p>
            ) : suggestions.length === 0 ? (
              <p className="settings__hint">{t("noIdea.noSuggestions")}</p>
            ) : (
              <div className="suggestion-list">
                {suggestions.map((s, i) => (
                  <div key={i} className={`suggestion-card${s.kind === "chaos" ? " suggestion-card--chaos" : ""}`}>
                    <span className="suggestion-card__index">
                      {s.kind === "chaos" ? <Icon name="patterns" size={18} /> : i + 1}
                    </span>
                    <div className="suggestion-card__body">
                      <div className="suggestion-card__top">
                        <span className="suggestion-card__title">{s.title}</span>
                        {s.estimatedMinutesLabel && (
                          <span className="suggestion-card__estimate">{s.estimatedMinutesLabel}</span>
                        )}
                      </div>
                      {s.description && <p className="suggestion-card__description">{s.description}</p>}
                      {s.reasons && s.reasons.length > 0 && (
                        <div className="suggestion-card__reasons">
                          {s.reasons.map((reason) => (
                            <span key={reason} className="suggestion-card__reason">
                              {reason}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                    <button
                      className="suggestion-card__start"
                      onClick={() => handleAccept(s, i)}
                      disabled={startedIndex !== null}
                    >
                      {startedIndex === i ? (
                        <>
                          <Icon name="check" size={15} />
                          {t("noIdea.startedButton")}
                        </>
                      ) : (
                        <>
                          <Icon name="timers" size={15} />
                          {t("noIdea.startButton")}
                        </>
                      )}
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="no-idea__actions">
              <button
                className="no-idea__cta"
                onClick={() => generateSuggestions(minutes, intent)}
                disabled={loadingSuggestions}
              >
                <Icon name="refresh" size={15} />
                {t("noIdea.regenerateButton")}
              </button>
              <button className="no-idea__ghost" onClick={resetSuggestFlow}>
                {t("noIdea.startOverButton")}
              </button>
            </div>
          </>
        )}
      </section>
    </div>
  );
}
