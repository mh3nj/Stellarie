/**
 * This file is the ONE place in the frontend that knows the generated
 * bindings might not exist yet.
 *
 * `wails3 generate bindings` produces
 * frontend/bindings/stellarei/app/services/*.ts (one module per bound
 * Go service, generated - never hand-edited, never checked in). This
 * loader imports them lazily (one chunk per service) and throws a
 * clear, actionable error if a chunk can't load, instead of the app
 * silently doing nothing or (worse) showing fabricated data. See
 * README.md "Generating bindings".
 *
 * Everything downstream of this file - pages, components - should import
 * from here, not from "../../bindings/..." directly, so this is the only
 * spot that needs to change if the generated import paths differ slightly
 * from what's guessed below.
 */

import type { CloseBehavior, StartupBehavior } from "@/types/settings";

export interface SettingsBindings {
  GetString(key: string): Promise<string>;
  SetString(key: string, value: string): Promise<void>;
  GetBool(key: string): Promise<boolean>;
  SetBool(key: string, value: boolean): Promise<void>;
  GetInt(key: string, fallback: number): Promise<number>;
  SetInt(key: string, value: number): Promise<void>;
  GetCloseBehavior(): Promise<CloseBehavior>;
  SetCloseBehavior(value: CloseBehavior): Promise<void>;
}

export interface WindowBindings {
  MinimizeToTray(): Promise<void>;
  QuitApplication(): Promise<void>;
  Show(): Promise<void>;
}

export interface StartupBindings {
  IsEnabled(): Promise<boolean>;
  SetEnabled(enabled: boolean, behavior: StartupBehavior): Promise<void>;
}

class BindingsNotGeneratedError extends Error {
  constructor(serviceName: string, cause: unknown) {
    super(
      `STELLAREI: bindings for ${serviceName} haven't been generated yet. ` +
        `Run "wails3 generate bindings" (or "wails3 dev") from the project ` +
        `root with the Wails v3 CLI installed, then reload.`,
    );
    this.name = "BindingsNotGeneratedError";
    this.cause = cause;
  }
}

// The exact generated path is `frontend/bindings/<module>/<go-package-path>/<service>`
// per the v3 binding generator's "clearer imports" layout (see README.md).
// Each caller passes a closure wrapping a LITERAL import() specifier
// never a variable, never a Vite ignore comment. The literal is what lets
// Vite/Rollup statically resolve the generated module and bundle it into
// dist (one lazy chunk per service); a variable specifier is left as a
// runtime URL that 404s in production builds, where frontend/bindings/
// is NOT served from disk (it works in `wails3 dev` only because the dev
// server serves the whole frontend/ directory). `wails3 generate
// bindings` must run before `vite build` - build:frontend already depends
// on generate:bindings, so a missing binding fails the build loudly
// instead of failing every tab at runtime. The try/catch stays so a
// genuinely unreachable backend still surfaces the actionable error
// below rather than an opaque chunk-load failure.
async function loadService<T>(importer: () => Promise<unknown>, serviceName: string): Promise<T> {
  try {
    const mod = await importer();
    return mod as T;
  } catch (cause) {
    throw new BindingsNotGeneratedError(serviceName, cause);
  }
}

export const getSettingsBindings = (): Promise<SettingsBindings> =>
  loadService<SettingsBindings>(
    () => import("../../bindings/stellarei/app/services/settingsservice.js"),
    "SettingsService",
  );

export const getWindowBindings = (): Promise<WindowBindings> =>
  loadService<WindowBindings>(
    () => import("../../bindings/stellarei/app/services/windowservice.js"),
    "WindowService",
  );

export const getStartupBindings = (): Promise<StartupBindings> =>
  loadService<StartupBindings>(
    () => import("../../bindings/stellarei/app/services/startupservice.js"),
    "StartupService",
  );

// Spec section 45: global keyboard shortcuts, user-configurable. Mirrors
// internal/shortcuts.Action's five string values and
// app/services/shortcuts_service.go's ShortcutView exactly - isCustom
// tells the UI whether to show a "reset to default" affordance for that
// row.
export interface ShortcutView {
  action: string;
  accelerator: string;
  isCustom: boolean;
}

export interface ShortcutsBindings {
  ListShortcuts(): Promise<ShortcutView[]>;
  // SetShortcut validates syntax, checks for a conflict with
  // STELLAREI's other shortcuts, and attempts live OS re-registration
  // before persisting - a rejected value throws with a real, specific
  // message (malformed syntax, already assigned to another STELLAREI
  // action, or already owned by another application at the OS level).
  SetShortcut(action: string, accelerator: string): Promise<void>;
  ResetShortcut(action: string): Promise<void>;
}

export const getShortcutsBindings = (): Promise<ShortcutsBindings> =>
  loadService<ShortcutsBindings>(
    () => import("../../bindings/stellarei/app/services/shortcutsservice.js"),
    "ShortcutsService",
  );

export interface ActivityStatus {
  processName: string;
  windowTitle: string;
  idle: boolean;
  paused: boolean;
  privacyMode: boolean; // spec section 26 - distinct from an ordinary manual pause
  runStartedAt: string; // RFC 3339, empty if nothing in progress
}

export interface ActivityBindings {
  GetStatus(): Promise<ActivityStatus>;
  Pause(minutes: number): Promise<void>;
  Resume(): Promise<void>;
}

export const getActivityBindings = (): Promise<ActivityBindings> =>
  loadService<ActivityBindings>(
    () => import("../../bindings/stellarei/app/services/activityservice.js"),
    "ActivityService",
  );

export interface ThemeRecord {
  name: string;
  isBuiltin: boolean;
  tokensJson: string;
}

export interface ThemeBindings {
  ListThemes(): Promise<ThemeRecord[]>;
  GetTokensJSON(name: string): Promise<string>;
  SaveTheme(name: string, tokensJson: string): Promise<void>;
  DeleteTheme(name: string): Promise<void>;
  GetActiveThemeName(): Promise<string>;
  SetActiveThemeName(name: string): Promise<void>;
}

export const getThemeBindings = (): Promise<ThemeBindings> =>
  loadService<ThemeBindings>(
    () => import("../../bindings/stellarei/app/services/themeservice.js"),
    "ThemeService",
  );

export interface SessionView {
  id: number;
  startedAt: number; // unix seconds
  endedAt: number;
  applicationId: number;
  applicationName: string;
  projectId: number; // 0 = unassigned
  projectName: string;
  projectColor: string; // a theme token name, e.g. "accent" - not a literal color
  categoryId: number; // 0 = uncategorized
  categoryName: string;
  categoryColor: string;
  confidence: number;
  manuallyEdited: boolean;
  excluded: boolean;
  note: string;
}

export interface SessionBindings {
  ListForRange(startUnix: number, endUnix: number): Promise<SessionView[]>;
  UpdateCorrection(id: number, projectId: number, categoryId: number, excluded: boolean, note: string): Promise<void>;
  Split(id: number, splitAtUnix: number): Promise<number>;
  Merge(keepId: number, mergeId: number): Promise<void>;
}

export const getSessionBindings = (): Promise<SessionBindings> =>
  loadService<SessionBindings>(
    () => import("../../bindings/stellarei/app/services/sessionservice.js"),
    "SessionService",
  );

export interface ProjectRecord {
  id: number;
  name: string;
  description: string;
  status: string;
  colorToken: string;
  archived: boolean;
}

export interface RuleRecord {
  id: number;
  projectId: number;
  type: string;
  pattern: string;
  priority: number;
}

export interface ProjectBindings {
  ListProjects(includeArchived: boolean): Promise<ProjectRecord[]>;
  CreateProject(name: string, description: string, colorToken: string): Promise<number>;
  UpdateProject(id: number, name: string, description: string, colorToken: string, status: string): Promise<void>;
  ArchiveProject(id: number): Promise<void>;
  UnarchiveProject(id: number): Promise<void>;
  ListRulesForProject(projectId: number): Promise<RuleRecord[]>;
  AddRule(projectId: number, ruleType: string, pattern: string, priority: number): Promise<number>;
  DeleteRule(ruleId: number): Promise<void>;
}

export const getProjectBindings = (): Promise<ProjectBindings> =>
  loadService<ProjectBindings>(
    () => import("../../bindings/stellarei/app/services/projectservice.js"),
    "ProjectService",
  );

export interface CategoryRecord {
  id: number;
  name: string;
  colorToken: string;
  isBuiltin: boolean;
}

export interface ApplicationCategoryRecord {
  applicationId: number;
  processName: string;
  categoryId: number;
  defaultCategoryId: number;
}

// Named distinctly from RuleRecord (which is project-scoped) even
// though the shape is otherwise identical - matches the Go-side
// CategoryRuleRecord naming, itself renamed from a first-draft
// RuleRecord after a real same-package collision with
// project_service.go's RuleRecord (see the handbook's self-correction
// log).
export interface CategoryRuleRecord {
  id: number;
  categoryId: number;
  type: string;
  pattern: string;
  priority: number;
}

export interface CategoryBindings {
  ListCategories(): Promise<CategoryRecord[]>;
  CreateCategory(name: string, colorToken: string): Promise<number>;
  DeleteCategory(id: number): Promise<void>;
  ListApplications(): Promise<ApplicationCategoryRecord[]>;
  SetApplicationCategory(applicationId: number, categoryId: number): Promise<void>;
  ListRulesForCategory(categoryId: number): Promise<CategoryRuleRecord[]>;
  AddRule(categoryId: number, ruleType: string, pattern: string, priority: number): Promise<number>;
  DeleteRule(ruleId: number): Promise<void>;
}

export const getCategoryBindings = (): Promise<CategoryBindings> =>
  loadService<CategoryBindings>(
    () => import("../../bindings/stellarei/app/services/categoryservice.js"),
    "CategoryService",
  );

export interface BudgetStatus {
  id: number;
  name: string;
  categoryId: number;
  projectId: number;
  budgetType: string;
  limitMinutes: number;
  schedule: string;
  warnAtMinutes: number;
  hardLimit: boolean;
  actualMinutes: number;
  periodStart: number;
  periodEnd: number;
}

export interface BudgetBindings {
  ListWithStatus(): Promise<BudgetStatus[]>;
  CreateBudget(
    name: string,
    categoryId: number,
    projectId: number,
    budgetType: string,
    limitMinutes: number,
    schedule: string,
    warnAtMinutes: number,
    hardLimit: boolean,
  ): Promise<number>;
  ArchiveBudget(id: number): Promise<void>;
}

export const getBudgetBindings = (): Promise<BudgetBindings> =>
  loadService<BudgetBindings>(
    () => import("../../bindings/stellarei/app/services/budgetservice.js"),
    "BudgetService",
  );

export interface GoalStatus {
  id: number;
  name: string;
  goalType: string;
  projectId: number;
  targetValue: number;
  unit: string;
  period: string;
  actualHours: number;
  percentComplete: number;
  periodStart: number;
  periodEnd: number;
}

export interface GoalBindings {
  ListWithStatus(): Promise<GoalStatus[]>;
  CreateGoal(name: string, goalType: string, projectId: number, targetHours: number, period: string): Promise<number>;
  ArchiveGoal(id: number): Promise<void>;
}

export const getGoalBindings = (): Promise<GoalBindings> =>
  loadService<GoalBindings>(
    () => import("../../bindings/stellarei/app/services/goalservice.js"),
    "GoalService",
  );

export interface CategoryTotal {
  categoryId: number;
  seconds: number;
}

export interface ProjectTotal {
  projectId: number;
  seconds: number;
}

export interface ContextSwitchStats {
  switches: number;
  sessionCount: number;
  averageSeconds: number;
  longestSeconds: number;
}

export interface Trend {
  currentSeconds: number;
  previousSeconds: number;
  percentChange: number;
}

export interface AnalyticsBindings {
  GetCategoryBreakdown(startUnix: number, endUnix: number): Promise<CategoryTotal[]>;
  GetProjectBreakdown(startUnix: number, endUnix: number): Promise<ProjectTotal[]>;
  GetContextSwitchStats(startUnix: number, endUnix: number): Promise<ContextSwitchStats>;
  GetTrend(startUnix: number, endUnix: number): Promise<Trend>;
}

export const getAnalyticsBindings = (): Promise<AnalyticsBindings> =>
  loadService<AnalyticsBindings>(
    () => import("../../bindings/stellarei/app/services/analyticsservice.js"),
    "AnalyticsService",
  );

export interface TimerView {
  id: number;
  timerType: string; // stopwatch | countdown | pomodoro | interval | focus | project | custom
  projectId: number;
  label: string;
  durationSecs: number; // 0 = no target (stopwatch)
  status: string; // running | paused | completed | cancelled
  startedAt: number; // unix seconds
  elapsedSecs: number;
  remainingSecs: number;
  pomodoroPhase?: string; // work | short_break | long_break - only set for pomodoro timers
  pomodoroCyclesDone?: number;
}

export interface TimerBindings {
  ListActive(): Promise<TimerView[]>;
  ListRecent(limit: number): Promise<TimerView[]>;
  StartStopwatch(projectId: number, label: string): Promise<number>;
  StartCountdown(timerType: string, projectId: number, label: string, durationMinutes: number): Promise<number>;
  StartPomodoro(
    projectId: number,
    label: string,
    workMinutes: number,
    shortBreakMinutes: number,
    longBreakMinutes: number,
    cyclesUntilLongBreak: number,
  ): Promise<number>;
  Pause(id: number): Promise<void>;
  Resume(id: number): Promise<void>;
  Finish(id: number): Promise<void>;
  Cancel(id: number): Promise<void>;
}

export const getTimerBindings = (): Promise<TimerBindings> =>
  loadService<TimerBindings>(
    () => import("../../bindings/stellarei/app/services/timerservice.js"),
    "TimerService",
  );

export interface AlarmView {
  id: number;
  alarmType: string; // one_time | recurring | relative | activity_based | schedule
  label: string;
  fireAt?: number; // unix seconds - one_time/relative/schedule
  recurrenceRule?: string;
  triggerCondition?: string;
  enabled: boolean;
  snoozeMinutes: number;
  sound?: string;
  createdAt: number;
  lastFiredAt?: number;
  snoozedUntil?: number;
  nextFireAt?: number; // best-effort next occurrence; 0/absent for activity_based
}

export interface AlarmBindings {
  List(): Promise<AlarmView[]>;
  CreateOneTime(label: string, fireAtUnix: number, snoozeMinutes: number, sound: string): Promise<number>;
  CreateRelative(label: string, minutesFromNow: number, snoozeMinutes: number, sound: string): Promise<number>;
  CreateRecurring(
    alarmType: string,
    label: string,
    recurrenceRule: string,
    snoozeMinutes: number,
    sound: string,
  ): Promise<number>;
  CreateActivityBased(label: string, triggerCondition: string, snoozeMinutes: number, sound: string): Promise<number>;
  SetEnabled(id: number, enabled: boolean): Promise<void>;
  Delete(id: number): Promise<void>;
  Snooze(id: number, minutes: number): Promise<void>;
  Dismiss(id: number): Promise<void>;
}

export const getAlarmBindings = (): Promise<AlarmBindings> =>
  loadService<AlarmBindings>(
    () => import("../../bindings/stellarei/app/services/alarmservice.js"),
    "AlarmService",
  );

export interface NotificationView {
  id: number;
  source: string; // alarm | budget | goal | system
  sourceId: number;
  title: string;
  body: string;
  createdAt: number;
  dismissedAt?: number;
}

export interface NotificationBindings {
  ListRecent(limit: number): Promise<NotificationView[]>;
  Dismiss(id: number): Promise<void>;
}

export const getNotificationBindings = (): Promise<NotificationBindings> =>
  loadService<NotificationBindings>(
    () => import("../../bindings/stellarei/app/services/notificationservice.js"),
    "NotificationService",
  );

export interface FocusView {
  id: number;
  projectId: number;
  label: string;
  durationSecs: number; // 0 = open-ended
  status: string; // running | paused | completed | cancelled
  startedAt: number;
  elapsedSecs: number;
  remainingSecs: number;
  blockedApps?: string[];
  blockedWebsites?: string[];
  suppressNotifications: boolean;
  qualityPercent: number;
  qualityKnown: boolean;
  onTargetSecs: number;
  trackedSecs: number;
  blockedAppSecs: number;
  currentAppBlocked: boolean;
}

export interface FocusBindings {
  Start(
    projectId: number,
    label: string,
    durationMinutes: number,
    blockedApps: string[],
    blockedWebsites: string[],
    suppressNotifications: boolean,
  ): Promise<number>;
  GetActive(): Promise<FocusView | null>;
  History(limit: number): Promise<FocusView[]>;
}

export const getFocusBindings = (): Promise<FocusBindings> =>
  loadService<FocusBindings>(
    () => import("../../bindings/stellarei/app/services/focusservice.js"),
    "FocusService",
  );

export interface SuggestionView {
  kind: string; // project | goal | chaos | generic
  projectId?: number;
  goalId?: number;
  title: string;
  description?: string;
  estimatedMinutesLabel?: string;
  reasons?: string[];
}

export interface IdeaResult {
  historyId: number;
  suggestions: SuggestionView[];
  aiNote?: string; // Phase 11's OPTIONAL AI commentary - only ever present when the person has turned AI on AND opted the "i_have_no_idea" scope in. See AIBindings below and Settings > AI. Always render this clearly labeled as AI-generated, never blended in with the suggestions above.
}

export interface ProjectIdeaView {
  title: string;
  description: string;
  estimatedMinutes: number;
  estimatedLabel: string;
  difficulty: string;
  why: string;
  aiNote?: string; // same OPTIONAL-AI contract as IdeaResult.aiNote, gated on the "project_ideas" scope instead
}

export interface IdeaBindings {
  Generate(availableMinutes: number, intent: string): Promise<IdeaResult>;
  RecordAcceptance(historyId: number, index: number): Promise<void>;
  GenerateProjectIdea(availableMinutes: number, platform: string): Promise<ProjectIdeaView>;
  StartProjectFromIdea(title: string, description: string): Promise<number>;
}

export const getIdeaBindings = (): Promise<IdeaBindings> =>
  loadService<IdeaBindings>(
    () => import("../../bindings/stellarei/app/services/ideaservice.js"),
    "IdeaService",
  );

export interface ExclusionView {
  id: number;
  type: string; // application | window_title_pattern | domain
  pattern: string;
  appliesTo: string; // all | screenshots | browser_tracking
  createdAt: number;
}

export interface ApplicationPrivacyView {
  applicationId: number;
  processName: string;
  trackingEnabled: boolean;
  screenshotAllowed: boolean;
  browserUrlTracking: boolean;
  privacySensitive: boolean;
}

export interface ScreenshotView {
  id: number;
  takenAt: number;
  filePath: string;
  applicationId?: number;
  deleteAfter?: number;
}

export interface PrivacyBindings {
  GetPrivacyMode(): Promise<boolean>;
  SetPrivacyMode(enabled: boolean): Promise<void>;
  ListExclusions(): Promise<ExclusionView[]>;
  CreateExclusion(exclusionType: string, pattern: string, appliesTo: string): Promise<number>;
  DeleteExclusion(id: number): Promise<void>;
  ListApplicationPrivacy(): Promise<ApplicationPrivacyView[]>;
  SetApplicationPrivacy(
    applicationId: number,
    trackingEnabled: boolean,
    screenshotAllowed: boolean,
    browserUrlTracking: boolean,
    privacySensitive: boolean,
  ): Promise<void>;
  ListScreenshots(limit: number): Promise<ScreenshotView[]>;
  DeleteScreenshot(id: number): Promise<void>;
}

export const getPrivacyBindings = (): Promise<PrivacyBindings> =>
  loadService<PrivacyBindings>(
    () => import("../../bindings/stellarei/app/services/privacyservice.js"),
    "PrivacyService",
  );

export interface BackupInfoView {
  path: string;
  filename: string;
  createdAt: number;
  sizeBytes: number;
  includesScreenshots: boolean;
}

export interface BackupBindings {
  BackupNow(includeScreenshots: boolean): Promise<BackupInfoView>;
  ListBackups(): Promise<BackupInfoView[]>;
  DeleteBackup(path: string): Promise<void>;
  RestoreBackup(path: string): Promise<void>;
  RestartToApplyRestore(): Promise<void>;
  ExportSQLite(): Promise<string>;
  Export(dataset: string, format: string): Promise<string>;
}

export const getBackupBindings = (): Promise<BackupBindings> =>
  loadService<BackupBindings>(
    () => import("../../bindings/stellarei/app/services/backupservice.js"),
    "BackupService",
  );

// ── Phase 11: Optional Local AI (spec section 37 / Settings > AI, ─────────
// section 71) ────────────────────────────────────────────────────────────
// Disabled by default and opt-in per feature. AIConfigView.scopes is keyed
// by the same scope strings the Go side uses ("summaries", "patterns",
// "suggestions", "project_ideas", "i_have_no_idea", "daily_reviews") - see
// internal/ai.AllScopes. Every one of these bindings is safe to call even
// when AI is off: GetConfig/TestConnection/Diagnostics degrade gracefully
// (see AIResultView.unavailable / AIDiagnosticsView.hasRun) rather than
// throwing.

export const AI_SCOPES = [
  "summaries",
  "patterns",
  "suggestions",
  "project_ideas",
  "i_have_no_idea",
  "daily_reviews",
] as const;
export type AIScope = (typeof AI_SCOPES)[number];

export const AI_SCOPE_LABELS: Record<AIScope, string> = {
  summaries: "Daily / Weekly Review summaries",
  patterns: "Pattern Discovery phrasing",
  suggestions: "General suggestion text",
  project_ideas: "\"I want to build something\" project ideas",
  i_have_no_idea: "\"I HAVE NO IDEA\" suggestions",
  daily_reviews: "Time Receipt commentary",
};

export interface AIConfigView {
  enabled: boolean;
  provider: string; // "none" | "ollama" | "openai_compatible"
  endpoint: string;
  model: string;
  scopes: Record<string, boolean>;
}

export interface AIResultView {
  text: string;
  unavailable: boolean;
  reason: string;
  latencyMs: number;
}

export interface AIDiagnosticsView {
  hasRun: boolean;
  at: string;
  scope: string;
  provider: string;
  success: boolean;
  reason: string;
  latencyMs: number;
}

export interface AIBindings {
  GetConfig(): Promise<AIConfigView>;
  SetEnabled(enabled: boolean): Promise<void>;
  SetProvider(provider: string): Promise<void>;
  SetEndpoint(endpoint: string): Promise<void>;
  SetModel(model: string): Promise<void>;
  SetScope(scope: string, enabled: boolean): Promise<void>;
  TestConnection(): Promise<AIResultView>;
  Diagnostics(): Promise<AIDiagnosticsView>;
}

export const getAIBindings = (): Promise<AIBindings> =>
  loadService<AIBindings>(
    () => import("../../bindings/stellarei/app/services/aiservice.js"),
    "AIService",
  );

// ── Daily Time Receipt (spec section 60) ───────────────────────────────
// A real, deterministic feature - categories/total come straight from
// session data; goals/budgets reuse GoalBindings/BudgetBindings' own
// status math (see app/services/receipt_service.go). aiNote is the same
// OPTIONAL, clearly-labeled Phase 11 contract as IdeaResult.aiNote,
// gated on the "daily_reviews" scope.

export interface TimeReceiptGoalLine {
  name: string;
  met: boolean;
  percentComplete: number;
}

export interface TimeReceiptBudgetLine {
  name: string;
  limitMinutes: number;
  actualMinutes: number;
  overMinutes: number; // positive = over, negative = under/remaining
}

export interface TimeReceiptView {
  dateLabel: string;
  categories: CategoryTotal[];
  totalTrackedSeconds: number;
  goals: TimeReceiptGoalLine[];
  budgets: TimeReceiptBudgetLine[];
  aiNote?: string;
}

export interface ReceiptBindings {
  GetTimeReceipt(atUnix: number): Promise<TimeReceiptView>;
}

export const getReceiptBindings = (): Promise<ReceiptBindings> =>
  loadService<ReceiptBindings>(
    () => import("../../bindings/stellarei/app/services/timereceiptservice.js"),
    "TimeReceiptService",
  );

// ── Pattern Discovery (spec section 31) ────────────────────────────────
// A real, deterministic feature - every Insight comes from
// internal/patterns' tested median/average/trend math over real session
// data (see app/services/pattern_service.go). aiNote is the same
// OPTIONAL, clearly-labeled Phase 11 contract as elsewhere, gated on the
// "patterns" scope, and can only ever summarize the Insights already
// found - never introduce a new one (spec section 66: never fabricate
// patterns).

export interface PatternInsightView {
  kind: string;
  categoryId?: number;
  categoryName?: string;
  text: string;
  insightType: string; // pass back to Dismiss/Undismiss unchanged - an opaque key, not meant to be parsed on the frontend
}

export interface PatternsView {
  insights: PatternInsightView[];
  aiNote?: string;
}

export interface PatternBindings {
  GetInsights(): Promise<PatternsView>;
  Dismiss(insightType: string): Promise<void>;
  Undismiss(insightType: string): Promise<void>;
}

export const getPatternBindings = (): Promise<PatternBindings> =>
  loadService<PatternBindings>(
    () => import("../../bindings/stellarei/app/services/patternservice.js"),
    "PatternService",
  );

// ── Digital Waste Detection (spec section 33) ──────────────────────────
// A real, deterministic feature - see app/services/waste_service.go and
// internal/waste's package doc for exactly which 3 of the spec's 6
// example patterns this covers (and why the other 3 are deliberately
// not reimplemented here). aiNote is gated on the "suggestions" scope
// the one AI use case from spec section 37 with no other home - and is
// explicitly asked for a forward-looking, non-judgmental suggestion
// rather than a backward-looking summary (contrast with
// PatternsView.aiNote).

export interface WasteFindingView {
  kind: string;
  categoryId?: number;
  categoryName?: string;
  text: string;
}

export interface WasteView {
  findings: WasteFindingView[];
  aiNote?: string;
}

export interface WasteBindings {
  GetFindings(): Promise<WasteView>;
}

export const getWasteBindings = (): Promise<WasteBindings> =>
  loadService<WasteBindings>(
    () => import("../../bindings/stellarei/app/services/wasteservice.js"),
    "WasteService",
  );

// ── Calendar (spec section 30) ─────────────────────────────────────────
// A real month-view day-by-day category breakdown - see
// app/services/calendar_service.go and internal/calendar's package doc
// for the midnight-crossing session-splitting logic. Categories mirrors
// CategoryTotal (id + seconds only - the frontend joins names from the
// category list it already has loaded).

export interface CalendarDayView {
  date: string; // "2026-08-16"
  categories: CategoryTotal[];
  totalSeconds: number;
}

export interface CalendarMonthView {
  year: number;
  month: number; // 1-12
  days: CalendarDayView[];
}

export interface CalendarBindings {
  GetMonth(year: number, month: number): Promise<CalendarMonthView>;
}

export const getCalendarBindings = (): Promise<CalendarBindings> =>
  loadService<CalendarBindings>(
    () => import("../../bindings/stellarei/app/services/calendarservice.js"),
    "CalendarService",
  );

// ── Search (spec sections 29/62) ───────────────────────────────────────
// A real, bounded-vocabulary query parser (internal/search) composed
// with real session/project/category/application data - see that
// package's own doc comment for exactly what it understands (date
// phrases, whole-word entity names, "longest"/"shortest") and what it
// deliberately doesn't (no synonym dictionary, no full NLP).
// Interpretation is always present and honest about what was actually
// recognized, so a query outside the supported vocabulary is explained
// rather than silently mishandled.

export interface SearchResultView {
  sessionId: number;
  title: string;
  subtitle: string;
  dateKey: string; // "YYYY-MM-DD" - pass straight to `/timeline?date=<dateKey>&zoom=day`
  startedAt: number;
  endedAt: number;
}

export interface SearchView {
  interpretation: string;
  results: SearchResultView[];
}

export interface SearchBindings {
  Search(query: string): Promise<SearchView>;
}

export const getSearchBindings = (): Promise<SearchBindings> =>
  loadService<SearchBindings>(
    () => import("../../bindings/stellarei/app/services/searchservice.js"),
    "SearchService",
  );

// ── MY YEAR (spec section 61) ───────────────────────────────────────────
// A real yearly overview composing existing aggregations plus new logic
// for longest sessions, busiest month, and abandoned projects - see
// app/services/myyear_service.go and internal/myyear's package doc.
// GoalsCurrentlyComplete is current status, not necessarily completed
// THIS year (no completion timestamp exists to know that honestly
// same caveat as TimeReceiptView.goals). AbandonedProjects mixes
// explicit (person set Status="abandoned") and inferred (active but
// untouched 90+ days with real historical time) entries - see
// MyYearAbandonedProject.explicit.

export interface MyYearAbandonedProject {
  name: string;
  explicit: boolean;
  daysSinceActivity: number;
  historicalMinutes: number;
}

export interface MyYearLongestSession {
  title: string;
  dateKey: string;
  durationMinutes: number;
}

export interface MyYearView {
  year: number;
  totalTrackedSeconds: number;
  categories: CategoryTotal[];
  topProjects: ProjectTotal[];
  longestSessions: MyYearLongestSession[];
  busiestMonthLabel: string;
  busiestMonthSeconds: number;
  abandonedProjects: MyYearAbandonedProject[];
  goalsCurrentlyComplete: string[];
  hasPriorYearData: boolean;
  priorYearTotalSeconds: number;
  aiNote?: string;
}

export interface MyYearBindings {
  GetYear(year: number): Promise<MyYearView>;
}

export const getMyYearBindings = (): Promise<MyYearBindings> =>
  loadService<MyYearBindings>(
    () => import("../../bindings/stellarei/app/services/myyearservice.js"),
    "MyYearService",
  );

// ── Diagnostics (spec sections 55/80) ──────────────────────────────────
// Real, currently-observable facts - see app/services/diagnostics_service.go's
// own doc comment for exactly which parts of those two spec sections are
// covered vs. genuinely still missing (tray status - investigated for
// real; no such API exists in Wails v3 today). Startup-registration
// status, previously also unwired, now reads the OS's own record via
// StartupService.IsEnabled().

export interface DiagnosticsView {
  appVersion: string;
  os: string;
  arch: string;
  databasePath: string;
  databaseReachable: boolean;
  databaseSizeBytes: number;
  eventsStored: number;
  sessionsStored: number;
  activityEngineRunning: boolean;
  activityEngineIdle: boolean;
  activityEnginePrivacyMode: boolean;
  activityEngineLastUpdate: string;
  startupRegistered: boolean;
  startupCheckError: string;
  processUptimeSeconds: number;
  goroutineCount: number;
  memoryAllocMb: number;
}

export interface DiagnosticsBindings {
  GetDiagnostics(): Promise<DiagnosticsView>;
}

export const getDiagnosticsBindings = (): Promise<DiagnosticsBindings> =>
  loadService<DiagnosticsBindings>(
    () => import("../../bindings/stellarei/app/services/diagnosticsservice.js"),
    "DiagnosticsService",
  );
