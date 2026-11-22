import logoDark from "@/assets/logo-dark.png";
import logoLight from "@/assets/logo-light.png";
import { useTheme } from "@/lib/theme-context";

interface LogoProps {
  size?: number;
}

/** Switches between the dark-background and light-background logo
 * variants based on the ACTIVE THEME's computed mode (useTheme().mode)
 * - not the OS's dark-mode setting, matching the same reasoning as the
 * Go-side tray icon (internal/tray.Tray.SetIconForMode). */
export function Logo({ size = 28 }: LogoProps) {
  const { mode } = useTheme();
  const src = mode === "dark" ? logoDark : logoLight;
  return <img src={src} alt="" width={size} height={size} className="logo" />;
}
