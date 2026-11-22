import type { ThemeTokens } from "@/themes/themes";

const TIMELINE_KEYS = [
  "timelineDevelopment",
  "timelineDesign",
  "timelineMusic",
  "timelineGaming",
  "timelineResearch",
  "timelineIdle",
] as const;

const STATUS_KEYS = ["success", "warning", "danger", "info"] as const;

/** section 39's "LIVE PREVIEW: mini STELLAREI interface, mini cards, mini
 * timeline, mini chart, mini buttons" - a small mockup of the real UI
 * (nav strip, a status card, a button, timeline swatches, status dots)
 * styled directly from `tokens` via inline styles. */
export function ThemePreview({ tokens }: { tokens: ThemeTokens }) {
  return (
    <div
      style={{
        background: tokens.background,
        border: `1px solid ${tokens.border}`,
        borderRadius: 8,
        padding: 12,
        display: "flex",
        gap: 8,
      }}
    >
      <div
        style={{
          width: 56,
          flexShrink: 0,
          background: tokens.backgroundElevated,
          border: `1px solid ${tokens.border}`,
          borderRadius: 6,
          padding: 8,
        }}
      >
        <div style={{ width: 14, height: 14, borderRadius: 4, background: tokens.accent, marginBottom: 10 }} />
        <div style={{ width: "100%", height: 4, background: tokens.surfaceActive, borderRadius: 2, marginBottom: 5 }} />
        <div style={{ width: "70%", height: 4, background: tokens.surfaceHover, borderRadius: 2, marginBottom: 5 }} />
        <div style={{ width: "85%", height: 4, background: tokens.surfaceHover, borderRadius: 2 }} />
      </div>

      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            background: tokens.surface,
            border: `1px solid ${tokens.border}`,
            borderRadius: 6,
            padding: 10,
            marginBottom: 8,
          }}
        >
          <div style={{ color: tokens.text, fontSize: 12, fontWeight: 600, marginBottom: 4 }}>Currently</div>
          <div style={{ color: tokens.textMuted, fontSize: 11, marginBottom: 8 }}>Code.exe · 01:17</div>
          <span
            style={{
              display: "inline-block",
              background: tokens.accent,
              color: tokens.background,
              borderRadius: 4,
              padding: "4px 10px",
              fontSize: 11,
              fontWeight: 500,
            }}
          >
            Primary Button
          </span>
        </div>

        <div style={{ display: "flex", gap: 3, marginBottom: 6 }}>
          {TIMELINE_KEYS.map((key) => (
            <div key={key} title={key} style={{ flex: 1, height: 16, background: tokens[key], borderRadius: 3 }} />
          ))}
        </div>

        <div style={{ display: "flex", gap: 6 }}>
          {STATUS_KEYS.map((key) => (
            <div
              key={key}
              title={key}
              style={{ width: 10, height: 10, borderRadius: "50%", background: tokens[key] }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
