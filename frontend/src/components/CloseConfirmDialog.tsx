import { useEffect, useState } from "react";
import { Events } from "@wailsio/runtime";
import { getWindowBindings } from "@/lib/bindings";

/**
 * Renders nothing until Go emits "window:confirm-close" (main.go's
 * WindowClosing hook, for the "ask" close-behavior setting), then shows
 * exactly the three choices from section 73: Close STELLAREI, Minimize to
 * Tray, Cancel. Cancel just closes this dialog - the native window was
 * already prevented from closing by e.Cancel() on the Go side, so there's
 * nothing further to undo.
 */
export function CloseConfirmDialog() {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const off = Events.On("window:confirm-close", () => setOpen(true));
    return off;
  }, []);

  if (!open) return null;

  async function choose(action: "close" | "tray") {
    setBusy(true);
    setError(null);
    try {
      const win = await getWindowBindings();
      if (action === "close") {
        await win.QuitApplication();
      } else {
        await win.MinimizeToTray();
        setOpen(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="dialog-overlay" role="dialog" aria-modal="true" aria-labelledby="close-confirm-title">
      <div className="dialog">
        <h2 id="close-confirm-title">What would you like to do?</h2>
        {error && <p className="dialog__error">{error}</p>}
        <div className="dialog__actions">
          <button disabled={busy} onClick={() => choose("close")}>
            Close STELLAREI
          </button>
          <button disabled={busy} onClick={() => choose("tray")}>
            Minimize to Tray
          </button>
          <button disabled={busy} onClick={() => setOpen(false)}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
