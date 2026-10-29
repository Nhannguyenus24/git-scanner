# unDraw illustrations

Source: [unDraw](https://undraw.co) — open licence, free for commercial use, no
attribution required. The files are the originals from `cdn.undraw.co`, with two
changes made by `scripts/process-undraw.py`:

1. Every flat palette colour is replaced by a CSS custom property
   (`--ud-accent`, `--ud-ink`, `--ud-ink2`, `--ud-mid`, `--ud-soft`,
   `--ud-paper`, `--ud-accent2`). `components/Illustration.tsx` sets those from
   the active theme, so an illustration re-inks itself for the paper, midnight,
   ocean, signal, minimal and high-contrast presets instead of staying unDraw
   violet on whatever surface it lands on. Skin tones are left untouched.
2. The intrinsic `width`/`height` and the generated layer ids are stripped. The
   size goes to the container, and without the ids two illustrations on one page
   cannot collide.

They are inlined (`?raw`) rather than loaded as `<img>` so the custom properties
cascade into them — an external SVG is a separate document and would not inherit
the theme.

To add one: download the SVG, drop it in this directory, re-run the script, and
register it in `components/Illustration.tsx`.
