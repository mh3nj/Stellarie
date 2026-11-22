"""Generates every STELLAREI brand asset from the root stellarie.png mark.

Run from the repo root:  python scripts/generate_brand_assets.py

Outputs (all derived from the one source mark, so they can never drift):
  build/appicon.png                   1024x1024 transparent, 6% padding
                                      (wails3 task common:generate:icons turns
                                      this into the .ico/.icns)
  frontend/public/favicon.ico         multi-size .ico (16/32/48/256)
  frontend/public/favicon-16.png      tiny favicon sizes as PNG, for
  frontend/public/favicon-32.png      browsers/WebView2 that prefer them
  frontend/public/favicon-48.png
  frontend/public/apple-touch-icon.png  180x180, mark on Midnight navy tile
  frontend/public/icon-192.png        maskable-style web tiles: mark on the
  frontend/public/icon-512.png        Midnight navy tile with safe-area padding
  frontend/src/assets/logo-dark.png   128x128, mark on Midnight navy tile
  frontend/src/assets/logo-light.png  128x128, mark on Light off-white tile
  assets/tray-icon-dark.png           32x32, mark on Midnight navy tile
  assets/tray-icon-light.png          32x32, mark on Light off-white tile
  stellarie-icon-source.png           square 3820x3820 canvas-locked copy of
                                      the raw mark (see 3821 note below)

Downscaling uses a premultiplied-alpha resample (split alpha, resize each
channel against a black matte, recompose) - a naive RGBA Lanczos on a
transparent image lets invisible RGB bleed through the resized alpha and
produces dark halos around the mark. All sizes are exact square squares.
"""

from PIL import Image

SRC = "stellarie.png"

# Midnight (default theme) tokens - frontend/src/themes/themes.ts is the
# source of truth; these are the same values.
NAVY = (11, 13, 16, 255)  # --background: #0b0d10
LIGHT = (245, 244, 251, 255)  # --background: #f5f4fb


def load_mark():
    """Loads stellarie.png and re-locks it onto a square canvas."""
    im = Image.open(SRC).convert("RGBA")
    # Source is 3820x3821 (one pixel taller than wide). Pad symmetrically to
    # square so the mark can never end up off-center or subtly non-round.
    w, h = im.size
    side = max(w, h)
    canvas = Image.new("RGBA", (side, side), (0, 0, 0, 0))
    canvas.paste(im, ((side - w) // 2, (side - h) // 2))
    return canvas


def shrink(im, size):
    """Premultiplied-alpha Lanczos downscale (see module docstring)."""
    a = im.getchannel("A").resize((size, size), Image.LANCZOS)
    rgb = im.copy()
    rgb.putalpha(Image.new("L", im.size, 255))
    black = Image.new("RGBA", im.size, (0, 0, 0, 255))
    premul = Image.alpha_composite(black, rgb).convert("RGB")
    premul = premul.resize((size, size), Image.LANCZOS)
    out = premul.convert("RGBA")
    out.putalpha(a)
    return out


def on_tile(mark, size, tile, pad_frac):
    """Returns `mark` scaled onto a solid `tile` square with padding."""
    inner = max(1, round(size * (1.0 - 2 * pad_frac)))
    mark_sized = shrink(mark, inner)
    out = Image.new("RGBA", (size, size), tile)
    out.alpha_composite(mark_sized, ((size - inner) // 2, (size - inner) // 2))
    return out


def padded(mark, size, pad_frac):
    """Returns `mark` on a transparent square with padding fraction `pad_frac`."""
    inner = max(1, round(size * (1.0 - 2 * pad_frac)))
    mark_sized = shrink(mark, inner)
    out = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    out.alpha_composite(mark_sized, ((size - inner) // 2, (size - inner) // 2))
    return out


def main():
    mark = load_mark()

    # --- Square source lock, kept in the repo as the stable input for any
    # --- future asset runs (the raw file's 3820x3821 would off-center pads).
    mark.save("stellarie-icon-source.png")

    # --- App icon (wails3 generate icons input) ----------------------------
    # Transparent, 6% padding - the OS compositors add their own masks.
    padded(mark, 1024, 0.06).save("build/appicon.png")

    # --- Favicons -----------------------------------------------------------
    for size in (16, 32, 48):
        shrink(mark, size).save(f"frontend/public/favicon-{size}.png")
    # Multi-resolution .ico (256 must be included or Windows Explorer and
    # WebView2 fall back to blurry upscaled 32px in some surfaces).
    shrink(mark, 256).save(
        "frontend/public/favicon.ico",
        sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )

    # --- Web app tiles: mark on the Midnight navy tile, maskable-safe -------
    for size in (192, 512):
        on_tile(mark, size, NAVY, 0.16).save(f"frontend/public/icon-{size}.png")

    # --- Apple touch: iOS composites onto white, needs opaque tile ----------
    on_tile(mark, 180, NAVY, 0.12).save("frontend/public/apple-touch-icon.png")

    # --- Frontend logo variants (Logo.tsx, theme-aware) ----------------------
    on_tile(mark, 128, NAVY, 0.14).save("frontend/src/assets/logo-dark.png")
    on_tile(mark, 128, LIGHT, 0.14).save("frontend/src/assets/logo-light.png")

    # --- Tray icons (32px, embedded by main.go) ------------------------------
    on_tile(mark, 32, NAVY, 0.08).save("assets/tray-icon-dark.png")
    on_tile(mark, 32, LIGHT, 0.08).save("assets/tray-icon-light.png")

    print("brand assets generated from", SRC)


if __name__ == "__main__":
    main()
