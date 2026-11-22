import { useTranslation } from "react-i18next";

const PRINCIPLE_KEYS = [
  "about.principleLocalOnly",
  "about.principleNoAccount",
  "about.principleNoTelemetry",
  "about.principleNoKeylogging",
  "about.principleNoClipboard",
  "about.principleNoMicrophone",
  "about.principleNoCamera",
  "about.principleScreenshotsOff",
  "about.principleBrowserOff",
  "about.principleOffline",
];

/** Static - genuinely complete in Phase 1, unlike everything else that
 * depends on a subsystem that doesn't exist yet. */
export function About() {
  const { t } = useTranslation();
  return (
    <div className="page">
      <h1>STELLAREI</h1>
      <p className="about__tagline">{t("about.tagline")}</p>

      <section className="settings__section">
        <h2>{t("about.principlesHeading")}</h2>
        <ul>
          {PRINCIPLE_KEYS.map((key) => (
            <li key={key}>{t(key)}</li>
          ))}
        </ul>
      </section>

      <section className="settings__section">
        <h2>{t("about.philosophyHeading")}</h2>
        <p>{t("about.philosophyFlow")}</p>
        <p className="settings__hint">{t("about.philosophyHint")}</p>
      </section>
    </div>
  );
}
