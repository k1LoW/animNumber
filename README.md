# animNumber

Animated SVGs of Arabic numerals (0-9) in [animCJK](https://github.com/parsimonhi/animCJK) format. Each digit is drawn stroke-by-stroke using `stroke-dashoffset` animation, the same technique animCJK uses for CJK characters.

## Demo

- Numerals: <https://k1low.github.io/animNumber/>
- How an animCJK SVG works (using "あ" as example): <https://k1low.github.io/animNumber/animCJK.html>

## Repository layout

- `svgsNumber/` — generated SVG files, one per digit (named by Unicode codepoint, e.g. `48.svg` for "0")
- `phase1/` — bootstraps SVGs from font outlines + median definitions (full regeneration)
- `phase2/` — preserves manually edited `<path id>` outlines and rebuilds everything else (style, clipPath, median wiring). Run after hand-editing outlines
- `licenses/` — license texts for redistributed font glyph data
- `demo/` — Vite app for the GitHub Pages site
- `animCJK.html` (under `demo/public/`) — explainer page for the animCJK SVG structure

## Usage

```sh
go run ./phase1   # full regeneration
go run ./phase2   # preserve outlines, refresh medians/style/clipPath
```

## Glyph attribution and modifications

Digit glyph outlines are **derived and modified** from [Klee One Regular](https://fonts.google.com/specimen/Klee+One) by Fontworks Inc., licensed under the [SIL Open Font License, Version 1.1](licenses/OFL.txt).

The following modifications have been applied to the original glyph outlines:

- **All digits** are scaled (×0.85) and re-positioned so the baseline lands at animCJK source y=76, fitting the 1024×1024 viewBox used by animCJK.
- **0, 6**: outline split into two halves (`d1a` / `d1b`) so the closed-loop stroke can be animated as a single continuous motion using the same dual-clip technique animCJK uses for "あ" stroke 3.
- **4, 5**: outline split into physically distinct parts (frame / stem for "4"; top bar / body for "5") so each stroke clips to its own region rather than the full digit silhouette.
- **7**: a small downward flag was added at the upper-left so the digit can be written as 2 strokes (flag + horizontal-and-diagonal). The original Klee One "7" does not have this flag.

Each generated SVG includes a header comment naming Klee One and the OFL.

The Reserved Font Name "Klee One" is **not** used for the modified glyph data redistributed in this project.

## License

- Code: see repository license.
- Glyph outlines: SIL Open Font License 1.1 (`licenses/OFL.txt`).
