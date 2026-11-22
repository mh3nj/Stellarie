import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useTheme } from "@/lib/theme-context";
import { ThemePreview } from "@/components/ThemePreview";
import { validateTheme, type ThemeTokens, type ThemeTokenKey } from "@/themes/themes";

// Group labels are translated (real UI section headings). Individual
// token labels (tokenLabel() below) deliberately are NOT: they're the
// same identifiers spec section 38's semantic-token system and section
// 40's exported theme JSON use verbatim (e.g. "chartPrimary"). Someone
// editing a theme and later inspecting or hand-editing the exported
// JSON benefits from the on-screen label matching the actual key, not
// a translation of it - same reasoning class as leaving Search's
// EXAMPLES and backend enum values untranslated elsewhere in this pass.
const TOKEN_GROUPS: { labelKey: string; keys: ThemeTokenKey[] }[] = [
  { labelKey: "themes.groupSurfaces", keys: ["background", "backgroundElevated", "surface", "surfaceHover", "surfaceActive", "border"] },
  { labelKey: "themes.groupText", keys: ["text", "textMuted"] },
  { labelKey: "themes.groupAccentStatus", keys: ["accent", "accentHover", "success", "warning", "danger", "info"] },
  {
    labelKey: "themes.groupChartsTimeline",
    keys: [
      "chartPrimary",
      "chartSecondary",
      "timelineDevelopment",
      "timelineDesign",
      "timelineMusic",
      "timelineGaming",
      "timelineResearch",
      "timelineIdle",
    ],
  },
];

function tokenLabel(key: string): string {
  return key.replace(/([A-Z])/g, " $1").replace(/^./, (c) => c.toUpperCase());
}

function downloadThemeFile(name: string, tokens: ThemeTokens) {
  const payload = { name, version: 1, author: "STELLAREI", tokens };
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${name.trim().toLowerCase().replace(/\s+/g, "-") || "theme"}.stellarei-theme.json`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

export function Themes() {
  const { t } = useTranslation();
  const { themes, activeThemeName, activateTheme, saveTheme, deleteTheme, loading, error } = useTheme();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [baseThemeName, setBaseThemeName] = useState(activeThemeName);
  const [editorName, setEditorName] = useState("");
  const [editorTokens, setEditorTokens] = useState<ThemeTokens | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [savedMessage, setSavedMessage] = useState<string | null>(null);

  const baseTheme = useMemo(() => themes.find((theme) => theme.name === baseThemeName) ?? themes[0], [themes, baseThemeName]);
  const workingTokens = editorTokens ?? baseTheme?.tokens ?? null;

  function startEditingFrom(name: string) {
    const source = themes.find((theme) => theme.name === name);
    if (!source) return;
    setBaseThemeName(name);
    setEditorTokens({ ...source.tokens });
    setEditorName(`${source.name} copy`);
    setActionError(null);
    setSavedMessage(null);
  }

  function updateToken(key: ThemeTokenKey, value: string) {
    setEditorTokens((prev) => ({ ...(prev ?? baseTheme?.tokens ?? ({} as ThemeTokens)), [key]: value }));
    setSavedMessage(null);
  }

  async function handleActivate(name: string) {
    setBusy(true);
    setActionError(null);
    try {
      await activateTheme(name);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete(name: string) {
    setBusy(true);
    setActionError(null);
    try {
      await deleteTheme(name);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleSave(activateAfter: boolean) {
    if (!workingTokens) return;
    const name = editorName.trim();
    if (!name) {
      setActionError(t("themes.nameYourThemeError"));
      return;
    }
    setBusy(true);
    setActionError(null);
    try {
      await saveTheme(name, workingTokens);
      if (activateAfter) {
        await activateTheme(name);
      }
      setSavedMessage(t("themes.savedMessage", { name }));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function handleExport(name: string) {
    const theme = themes.find((th) => th.name === name);
    if (!theme) return;
    downloadThemeFile(theme.name, theme.tokens);
  }

  async function handleImportFile(file: File) {
    setActionError(null);
    let parsed: unknown;
    try {
      parsed = JSON.parse(await file.text());
    } catch {
      setActionError(t("themes.invalidJsonError"));
      return;
    }
    const result = validateTheme(parsed);
    if (!result.valid) {
      setActionError(result.errors.join(" "));
      return;
    }
    const themeFile = parsed as { name: string; tokens: ThemeTokens };
    setEditorTokens({ ...themeFile.tokens });
    setEditorName(themeFile.name);
    setSavedMessage(null);
  }

  return (
    <div className="page page--wide">
      <h1>{t("themes.heading")}</h1>

      {error && (
        <p className="settings__error">{t("themes.backendErrorNotice", { error })}</p>
      )}

      <section className="settings__section">
        <h2>{t("themes.yourThemesHeading")}</h2>
        {loading ? (
          <p className="settings__hint">{t("common.loading")}</p>
        ) : (
          <ul className="theme-list">
            {themes.map((theme) => (
              <li key={theme.name} className="theme-list__item">
                <span
                  className="theme-list__swatch"
                  style={{ background: theme.tokens.background, borderColor: theme.tokens.accent }}
                />
                <span className="theme-list__name">
                  {theme.name}
                  {theme.isBuiltin && <span className="theme-list__badge">{t("themes.builtinBadge")}</span>}
                  {theme.name === activeThemeName && <span className="theme-list__badge theme-list__badge--active">{t("themes.activeBadge")}</span>}
                </span>
                <span className="theme-list__actions">
                  <button disabled={busy || theme.name === activeThemeName} onClick={() => handleActivate(theme.name)}>
                    {t("themes.activateButton")}
                  </button>
                  <button disabled={busy} onClick={() => startEditingFrom(theme.name)}>
                    {t("themes.editCopyButton")}
                  </button>
                  <button disabled={busy} onClick={() => handleExport(theme.name)}>
                    {t("themes.exportButton")}
                  </button>
                  {!theme.isBuiltin && (
                    <button disabled={busy || theme.name === activeThemeName} onClick={() => handleDelete(theme.name)}>
                      {t("themes.deleteButton")}
                    </button>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}

        <button
          className="theme-list__import"
          disabled={busy}
          onClick={() => fileInputRef.current?.click()}
        >
          {t("themes.importThemeButton")}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept="application/json"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) handleImportFile(file);
            e.target.value = "";
          }}
        />
      </section>

      {workingTokens && (
        <section className="settings__section">
          <h2>{t("themes.editorHeading")}</h2>
          <p className="settings__hint">{t("themes.editingCopyOf", { name: baseTheme?.name ?? "" })}</p>

          <label className="theme-editor__name">
            {t("themes.nameLabel")}
            <input
              type="text"
              value={editorName}
              onChange={(e) => {
                setEditorName(e.target.value);
                setSavedMessage(null);
              }}
              placeholder={t("themes.namePlaceholder")}
            />
          </label>

          <div className="theme-editor__preview-label">{t("themes.livePreviewLabel")}</div>
          <ThemePreview tokens={workingTokens} />

          {TOKEN_GROUPS.map((group) => (
            <div key={group.labelKey} className="theme-editor__group">
              <h3>{t(group.labelKey)}</h3>
              <div className="theme-editor__grid">
                {group.keys.map((key) => (
                  <label key={key} className="theme-editor__field">
                    <span>{tokenLabel(key)}</span>
                    <input type="color" value={workingTokens[key]} onChange={(e) => updateToken(key, e.target.value)} />
                  </label>
                ))}
              </div>
            </div>
          ))}

          {actionError && <p className="settings__error">{actionError}</p>}
          {savedMessage && <p className="settings__hint">{savedMessage}</p>}

          <div className="theme-editor__actions">
            <button disabled={busy} onClick={() => handleSave(false)}>
              {t("themes.saveAsNewButton")}
            </button>
            <button disabled={busy} onClick={() => handleSave(true)}>
              {t("themes.saveAndActivateButton")}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
