# CLAUDE.md

Repo-specific notes for editing `src/phase2/main.go` median data and keeping `graphicsNumber.txt` correct for both **animNumber's own SVG preview** and the downstream **`hanzi-writer-data-jp` / `kakitori`** consumer.

## Workflow

```sh
go run ./src/phase2   # rewrites svgsNumber/*.svg from digitMedians (preserves manual outline edits)
go run ./src/phase3   # rewrites graphicsNumber.txt from svgsNumber/*.svg
cp svgsNumber/*.svg demo/public/svgsNumber/   # demo/public/ is gitignored, manual sync
```

After editing `digitMedians` always run phase2 → phase3 → sync. The dev preview (`cd demo && npm run dev`) auto-reloads.

`src/glyphs/` holds Affinity Designer source files (`*.af`) and their SVG exports (`*.af.svg`) for digits whose outlines were hand-edited away from Klee One — open these in Affinity to update the outline, export to SVG, and paste the new outline `d=` into the matching `svgsNumber/*.svg` `<path id>` before running phase2.

## Data model: `Part` in `src/phase2/main.go`

```go
type Part struct {
    Letter  string  // "" / "a" / "b" / "c"
    Median  []Point // centerline output to graphicsNumber.txt as-is
    LeadOut []Point // SVG-only points appended after Median in the SVG d-attribute; stripped from graphicsNumber.txt by src/phase3/flipMedian
}
```

- `Median` is what hanzi-writer / kakitori consume. It can include an off-canvas **lead-in at the start** (the dual-clip "b's pre-lead-in" pattern), but must NOT contain a trailing off-canvas tail.
- `LeadOut` is appended to the SVG's `<path d="...">` for the concurrent `--d:1s` `pathLength="3333"` animation. It is invisible (clipped out and/or off-canvas) and never reaches the JSON.

## Required invariants for multi-path stroke groups (`0`, `3`, `6`, `8`, `9`)

### 1. `medians[0]` is the full single-stroke trajectory (animCJK convention)

`hanzi-writer`'s `strokeMatches` compares the user's drawn stroke against `medians[0]` only. For digits split across multiple data strokes, `medians[0]` must trace the **whole logical-stroke centerline** (= what the user is expected to draw in one motion). This mirrors the convention used by `あ` stroke 3 (`graphicsJaKana.txt` U+3042) and `ね` (`graphicsJa.txt` U+306D), where the first split's median covers the entire loop, and subsequent splits use clipPath to expose only their slice of the same trajectory.

Subsequent `medians[i]` (i >= 1) carry an off-canvas pre-lead-in followed by their visible portion of the trajectory. Their job is timing, not matching.

### 2. `Median` magnitude (≤500 outside bbox)

For every point `(x, y)` in `Median`:
- `x` within `[bbox.left - 500, bbox.right + 500]`
- `y` within `[bbox.bottom - 500, bbox.top + 500]`

Values like `-968` / `1237` / `-558` are too far outside bbox and are NOT acceptable in graphicsNumber.txt. (`LeadOut`, which never reaches the JSON, is unconstrained.)

### 3. kakitori timing invariant

`kakitori`'s `animateWithGroups` (`Kakitori.ts:710`) starts every path in a strokeGroup at **delay=0** and animates each for a duration **proportional to its median length** (in `HANZI_COORD_SIZE=900` units). For path B's visible drawing to start after path A's visible drawing has finished, the median data must satisfy:

```
leadIn(B) >= visible_portion_of(A in clip_A)
```

where:
- `visible_portion_of(A in clip_A)` = length of `A.Median`'s prefix that lies inside `clip_A`'s polygon (= the part that is actually drawn before the median exits A's clip into another part's territory).
- `leadIn(B)` = sum of segment lengths in `B.Median` from index 0 up to (and including) the first segment whose endpoint is on-canvas (i.e. the off-canvas pre-lead-in).

For three-path groups (e.g. `9`), also `leadIn(C) >= visible_portion_of(B in clip_B)`. For `9` `B` is structured as "lead-in + closure" so `visible(B) = total(B)`, and the inequality reduces to `leadIn(C) >= total(B)`.

Tracking visible-portion lengths per digit (current state):

| digit | parts | visible_portion_of(A) | leadIn(B) | OK? |
|---|---|---|---|---|
| 0 | a, b | 614 (left arc, A then exits clip into right half) | 660 | ✓ |
| 3 | a, b | 617 (upper bump) | 739 | ✓ |
| 6 | a, b | 750 (tail through bottom-mid) | 770 | ✓ |
| 8 | a, b | 1119 (right S) | 1150 | ✓ |
| 9 | a, b, c | 645 (bowl); 1126 (full B) | 670; 1430 | ✓ |

`src/phase3` does not implement the polygon intersection needed to compute `visible_portion_of` exactly; it falls back to a coarser `leadIn(B) >= total(A)` check and **prints flags but does not fail the build** when the heuristic over-reports (which it does for the full-trajectory `medians[0]` convention). Verify by running `kakitori`'s quiz on each multi-path digit and watching the animNumber preview.

### 4. animNumber preview timing (concurrent `--d:1s`)

The SVG preview animates every `clip-path` path concurrently for `0.8s`, with `pathLength="3333"` normalizing all paths to the same animated length. The visible-portion timing window for each part is:

```
visibleStart% = leadIn(part) / svgTotal(part)
visibleEnd%   = (leadIn(part) + visibleCenterline(part)) / svgTotal(part)
where svgTotal = total(Median) + length(LeadOut)
```

Adjust `LeadOut` (SVG-only) so that consecutive parts' visible windows abut without overlap or perceptible gap. `LeadOut` magnitude is unconstrained (off-canvas, invisible). Typical pattern:

| part | LeadOut |
|---|---|
| earlier parts (a, b for "9") | one point off-canvas, length tuned so visibleEnd% matches next part's visibleStart% |
| last part (c for "9", b for "8") | usually empty — its `Median` already ends at the digit's natural end |

## Verification

`src/phase3` prints heuristic timing notes (`leadIn(B) vs total(A)`) for every multi-path stroke group after writing graphicsNumber.txt. Those notes will flag the full-trajectory `medians[0]` convention as "violating" `leadIn >= total` — that's expected and informational only. The exact invariant is `leadIn(B) >= visible_portion_of(A in clip_A)`, which requires polygon intersection that phase3 doesn't compute.

To verify correctness in practice:

1. Watch the animNumber preview (`cd demo && npm run dev`). Multi-path digits should draw as one continuous stroke without mid-stroke gaps or overlaps.
2. Run kakitori's quiz on each multi-path digit (0, 3, 6, 8, 9). A user drawing the digit in **one continuous stroke** should match `medians[0]`.
3. Confirm the matching reference glyphs in animCJK behave the same: `あ` (U+3042 in `graphicsJaKana.txt`) and `ね` (U+306D in `graphicsJa.txt`) — these are the canonical examples of `medians[0]` = full single-stroke trajectory with subsequent splits providing only animation timing.

## Stroke counts (do not change without README updates)

`0, 1, 2, 3, 6, 8, 9` are 1-stroke; `4, 5, 7` are 2-stroke. Multi-path splits (`d1a`/`d1b`/`d1c`) are an internal mechanism for the dual-clip technique within a single logical stroke — they are NOT additional strokes. The `strokes` count in graphicsNumber.txt reflects data paths, but the logical stroke count is exposed via the consumer's `strokeGroups` config.
