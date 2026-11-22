import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { Icon } from "@/components/icons";
import {
  getReceiptBindings,
  getCategoryBindings,
  type TimeReceiptView,
  type CategoryRecord,
} from "@/lib/bindings";
import { shiftAnchor, formatDuration, parseDateParam } from "@/lib/time-ranges";

export function TimeReceipt() {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [anchor, setAnchor] = useState(() => parseDateParam(searchParams.get("date")));
  const [receipt, setReceipt] = useState<TimeReceiptView | null>(null);
  const [categories, setCategories] = useState<CategoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Follow the URL when it changes under this page (Calendar day clicks
  // and Search/Timeline links navigate date→date without remounting,
  // and keep-alive tabs are never remounted at all). Without this, the
  // anchor - and everything rendered from it - silently stays on the
  // previously shown day.
  const dateParam = searchParams.get("date");
  useEffect(() => {
    setAnchor(parseDateParam(dateParam));
  }, [dateParam]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [receiptBindings, categoryBindings] = await Promise.all([getReceiptBindings(), getCategoryBindings()]);
      const [view, cats] = await Promise.all([
        receiptBindings.GetTimeReceipt(Math.floor(anchor.getTime() / 1000)),
        categoryBindings.ListCategories(),
      ]);
      // Normalize nullable arrays: Go marshals empty slices as null,
      // and .length/.map on null unmounts the whole app (this exact
      // crash black-screened the app from Calendar day clicks on fresh
      // databases with no goals/budgets). The backend now also
      // guarantees [] (see receipt_service.go), this stays as
      // belt-and-braces at the proven crash site.
      setReceipt({
        ...view,
        categories: view.categories ?? [],
        goals: view.goals ?? [],
        budgets: view.budgets ?? [],
      });
      setCategories(cats ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [anchor]);

  useEffect(() => {
    load();
  }, [load]);

  const sortedCategories = receipt ? [...receipt.categories].sort((a, b) => b.seconds - a.seconds) : [];

  return (
    <div className="page">
      <h1>{t("timeReceipt.heading")}</h1>

      <div className="timeline__controls">
        <div className="timeline__nav">
          <button onClick={() => setAnchor(shiftAnchor("day", anchor, -1))} aria-label={t("common.previousLabel")}>‹</button>
          <span className="timeline__range-label">{receipt?.dateLabel ?? "-"}</span>
          <button onClick={() => setAnchor(shiftAnchor("day", anchor, 1))} aria-label={t("common.nextLabel")}>›</button>
          <button onClick={() => setAnchor(new Date())}>{t("common.today")}</button>
        </div>
      </div>

      {error && <p className="settings__error">{error}</p>}

      {loading ? (
        <p className="settings__hint">{t("common.loading")}</p>
      ) : receipt ? (
        <>
          <section className="settings__section">
            <h2>{t("timeReceipt.trackedHeading")}</h2>
            {sortedCategories.length === 0 ? (
              <p className="settings__hint">{t("timeReceipt.nothingTracked")}</p>
            ) : (
              <ul className="rule-list">
                {sortedCategories.map((c) => {
                  const cat = categories.find((cc) => cc.id === c.categoryId);
                  return (
                    <li key={c.categoryId} className="rule-list__item">
                      <span
                        className="theme-list__swatch"
                        style={{
                          width: 14,
                          height: 14,
                          background: cat?.colorToken ? `var(--${cat.colorToken})` : "var(--chartPrimary)",
                          borderColor: cat?.colorToken ? `var(--${cat.colorToken})` : "var(--chartPrimary)",
                        }}
                      />
                      <span className="rule-list__pattern">{cat?.name ?? t("timeReceipt.uncategorized")}</span>
                      <span>{formatDuration(c.seconds)}</span>
                    </li>
                  );
                })}
              </ul>
            )}
            <p className="analytics__trend-value" style={{ marginTop: "var(--space-3)" }}>
              {t("timeReceipt.totalTracked", { duration: formatDuration(receipt.totalTrackedSeconds) })}
            </p>
          </section>

          {receipt.goals.length > 0 && (
            <section className="settings__section">
              <h2>{t("timeReceipt.goalsHeading")}</h2>
              <p className="settings__hint">{t("timeReceipt.goalsProgressHint")}</p>
              <ul className="rule-list">
                {receipt.goals.map((g) => (
                  <li key={g.name} className="rule-list__item">
                    <span className="rule-list__pattern">{g.name}</span>
                    <span className={`goal-status${g.met ? " goal-status--met" : ""}`} aria-hidden="true">
                      <Icon name={g.met ? "check" : "close"} size={15} />
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {receipt.budgets.length > 0 && (
            <section className="settings__section">
              <h2>{t("timeReceipt.budgetsHeading")}</h2>
              <ul className="rule-list">
                {receipt.budgets.map((b) => (
                  <li key={b.name} className="rule-list__item">
                    <span className="rule-list__pattern">{b.name}</span>
                    <span>
                      {b.overMinutes > 0 ? `+${b.overMinutes}m` : `${b.overMinutes}m`}
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {receipt.aiNote && (
            <section className="settings__section">
              <h2>{t("timeReceipt.aiNoteHeading")}</h2>
              <p className="settings__hint">{t("timeReceipt.aiNoteHint")}</p>
              <p>{receipt.aiNote}</p>
            </section>
          )}
        </>
      ) : null}
    </div>
  );
}
