import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getSessionBindings, type CategoryRecord, type ProjectRecord, type SessionView } from "@/lib/bindings";
import { formatClockTime, formatDuration } from "@/lib/time-ranges";

interface SessionDetailPanelProps {
  session: SessionView;
  projects: ProjectRecord[];
  categories: CategoryRecord[];
  previousSession: SessionView | null;
  nextSession: SessionView | null;
  onClose: () => void;
  onChanged: () => void; // parent refetches the range after any mutation
}

export function SessionDetailPanel({
  session,
  projects,
  categories,
  previousSession,
  nextSession,
  onClose,
  onChanged,
}: SessionDetailPanelProps) {
  const { t } = useTranslation();
  const [projectId, setProjectId] = useState(session.projectId);
  const [categoryId, setCategoryId] = useState(session.categoryId);
  const [excluded, setExcluded] = useState(session.excluded);
  const [note, setNote] = useState(session.note);
  const [splitTime, setSplitTime] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setProjectId(session.projectId);
    setCategoryId(session.categoryId);
    setExcluded(session.excluded);
    setNote(session.note);
    setError(null);
    setSplitTime("");
  }, [session]);

  async function handleSave() {
    setBusy(true);
    setError(null);
    try {
      const bindings = await getSessionBindings();
      await bindings.UpdateCorrection(session.id, projectId, categoryId, excluded, note);
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleSplit() {
    if (!splitTime) {
      setError(t("sessionPanel.pickSplitTimeError"));
      return;
    }
    const [h, m] = splitTime.split(":").map(Number);

    // A session can span midnight (e.g. 23:30-00:45), so the same HH:MM
    // could land on more than one calendar day within its range. Try
    // each day the session actually touches and use whichever candidate
    // falls strictly inside (startedAt, endedAt), instead of assuming
    // the chosen time is always on the session's start date.
    const startDate = new Date(session.startedAt * 1000);
    const spanDays = Math.ceil((session.endedAt - session.startedAt) / 86400) + 1;

    let splitAtUnix: number | null = null;
    for (let dayOffset = 0; dayOffset <= spanDays; dayOffset++) {
      const candidate = new Date(startDate);
      candidate.setDate(candidate.getDate() + dayOffset);
      candidate.setHours(h, m, 0, 0);
      const candidateUnix = Math.floor(candidate.getTime() / 1000);
      if (candidateUnix > session.startedAt && candidateUnix < session.endedAt) {
        splitAtUnix = candidateUnix;
        break;
      }
    }

    if (splitAtUnix === null) {
      setError(t("sessionPanel.splitTimeOutOfRangeError"));
      return;
    }

    setBusy(true);
    setError(null);
    try {
      const bindings = await getSessionBindings();
      await bindings.Split(session.id, splitAtUnix);
      onChanged();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleMerge(otherId: number) {
    setBusy(true);
    setError(null);
    try {
      const bindings = await getSessionBindings();
      await bindings.Merge(session.id, otherId);
      onChanged();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const adjacentToPrevious = previousSession && previousSession.endedAt === session.startedAt;
  const adjacentToNext = nextSession && nextSession.startedAt === session.endedAt;

  return (
    <aside className="session-panel">
      <div className="session-panel__header">
        <h2>{session.applicationName || t("sessionPanel.unknownApplication")}</h2>
        <button className="session-panel__close" onClick={onClose} aria-label={t("sessionPanel.closeAriaLabel")}>
          ×
        </button>
      </div>

      <p className="settings__hint">
        {formatClockTime(session.startedAt)} - {formatClockTime(session.endedAt)} ·{" "}
        {formatDuration(session.endedAt - session.startedAt)}
      </p>
      <p className="settings__hint">
        {t("sessionPanel.confidenceLabel", { percent: Math.round(session.confidence * 100) })}
        {session.manuallyEdited && <span className="theme-list__badge"> {t("sessionPanel.manuallyEditedBadge")}</span>}
      </p>

      {error && <p className="settings__error">{error}</p>}

      <label className="settings__select">
        <span>{t("sessionPanel.projectLabel")}</span>
        <select value={projectId} onChange={(e) => setProjectId(Number(e.target.value))}>
          <option value={0}>{t("sessionPanel.unassignedOption")}</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </label>

      <label className="settings__select">
        <span>{t("sessionPanel.categoryLabel")}</span>
        <select value={categoryId} onChange={(e) => setCategoryId(Number(e.target.value))}>
          <option value={0}>{t("sessionPanel.uncategorizedOption")}</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>

      <label className="settings__checkbox">
        <input type="checkbox" checked={excluded} onChange={(e) => setExcluded(e.target.checked)} />
        {t("sessionPanel.excludeCheckbox")}
      </label>

      <label className="session-panel__note">
        {t("sessionPanel.noteLabel")}
        <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
      </label>

      <button disabled={busy} onClick={handleSave}>
        {t("sessionPanel.saveButton")}
      </button>

      <div className="session-panel__section">
        <h3>{t("sessionPanel.splitHeading")}</h3>
        <div className="session-panel__split-row">
          <input type="time" value={splitTime} onChange={(e) => setSplitTime(e.target.value)} />
          <button disabled={busy} onClick={handleSplit}>
            {t("sessionPanel.splitHereButton")}
          </button>
        </div>
      </div>

      {(adjacentToPrevious || adjacentToNext) && (
        <div className="session-panel__section">
          <h3>{t("sessionPanel.mergeHeading")}</h3>
          <div className="session-panel__split-row">
            {adjacentToPrevious && previousSession && (
              <button disabled={busy} onClick={() => handleMerge(previousSession.id)}>
                {t("sessionPanel.mergeWithPreviousButton")}
              </button>
            )}
            {adjacentToNext && nextSession && (
              <button disabled={busy} onClick={() => handleMerge(nextSession.id)}>
                {t("sessionPanel.mergeWithNextButton")}
              </button>
            )}
          </div>
        </div>
      )}
    </aside>
  );
}
