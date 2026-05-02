# animNumber

Animated SVGs of Arabic numerals (0-9) in [animCJK](https://github.com/parsimonhi/animCJK) format. Each digit is drawn stroke-by-stroke using `stroke-dashoffset` animation, the same technique animCJK uses for CJK characters.

## Demo

- Numerals: <https://k1low.github.io/animNumber/>

## Repository layout

- `svgsNumber/` — generated SVG files, one per digit (named by Unicode codepoint, e.g. `48.svg` for "0")
- `graphicsNumber.txt` — animCJK-format graphics data (one JSON line per digit), derived from the SVGs via phase3
- `phase1/` — bootstraps SVGs (and an initial `graphicsNumber.txt`) from font outlines + median definitions (full regeneration)
- `phase2/` — preserves manually edited `<path id>` outlines and rebuilds everything else (style, clipPath, median wiring). Run after hand-editing outlines
- `phase3/` — reads `svgsNumber/*.svg` and rewrites `graphicsNumber.txt` (treating the SVGs as the source of truth, including `d1a`/`d1b`/`d1c` splits). Run after phase2
- `licenses/` — license texts for redistributed font glyph data
- `demo/` — Vite app for the GitHub Pages site
- `animCJK.html` (under `demo/public/`) — explainer page for the animCJK SVG structure

## Usage

```sh
go run ./phase1   # full regeneration (initial bootstrap)
go run ./phase2   # preserve outlines, refresh medians/style/clipPath in SVGs
go run ./phase3   # rebuild graphicsNumber.txt from current SVGs
```

## Glyph attribution and modifications

Digit glyph outlines are **derived and modified** from [Klee One](https://github.com/fontworks-fonts/Klee), Copyright 2020 The Klee Project Authors, licensed under the [SIL Open Font License, Version 1.1](licenses/OFL.txt).

The following modifications have been applied to the original glyph outlines:

- **All digits** are scaled (×0.85) and re-positioned so the baseline lands at animCJK source y=76, fitting the 1024×1024 viewBox used by animCJK.
- **0, 3, 6, 8**: outline split into two halves (`d1a` / `d1b`) so the closed-loop stroke can be animated as a single continuous motion using the same dual-clip technique animCJK uses for "あ" stroke 3. b's median includes an off-canvas pre-lead-in so its visible portion picks up exactly where a finishes.
- **9**: outline split into three parts (`d1a` / `d1b` / `d1c`) for the bowl + descender, with sequential off-canvas timing so the three medians animate as one continuous stroke.
- **4, 5**: outline split into physically distinct parts (frame / stem for "4"; top bar / body for "5") so each stroke clips to its own region rather than the full digit silhouette. These are written as 2 strokes (`--d:1s`, `--d:2s`).
- **7**: a small downward flag was added at the upper-left so the digit can be written as 2 strokes (flag + horizontal-and-diagonal). The original Klee One "7" does not have this flag.

Stroke counts after these modifications: 0, 1, 2, 3, 6, 8, 9 are 1画 (one-stroke) and 4, 5, 7 are 2画 (two-stroke).

Each generated SVG includes a header comment naming Klee One and the OFL.

The Reserved Font Name "Klee One" is **not** used for the modified glyph data redistributed in this project.

## License

- Code: see repository license.
- Glyph outlines: SIL Open Font License 1.1 (`licenses/OFL.txt`).
