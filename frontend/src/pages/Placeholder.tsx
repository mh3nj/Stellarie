interface PlaceholderProps {
  title: string;
  phase?: number;
  phaseLabel: string;
}

/**
 * Every nav destination Phase 1 hasn't built yet renders this instead of
 * mock data or a disabled-looking fake UI. Per section 67 ("design
 * excellent empty states... do not show fake data") and section 83 ("do
 * not pretend the tray/timers/alarms work") - an honest "not built yet"
 * screen is the correct empty state for a section of the app that
 * genuinely doesn't exist yet, not an approximation of it.
 */
export function Placeholder({ title, phase, phaseLabel }: PlaceholderProps) {
  return (
    <div className="page page--placeholder">
      <h1>{title}</h1>
      <p className="placeholder__note">
        {phase ? (
          <>
            Not built yet - this lands in <strong>Phase {phase}</strong> ({phaseLabel}) of the implementation
            plan.
          </>
        ) : (
          <>Not built yet - {phaseLabel}.</>
        )}
      </p>
    </div>
  );
}
