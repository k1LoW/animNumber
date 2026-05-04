# CLAUDE.md

Repo-specific notes for editing `src/phase2/main.go` median data and keeping `graphicsNumber.txt` correct for both **animNumber's own SVG preview** and the downstream **`hanzi-writer-data-jp` / `kakitori`** consumer.

## Workflow

```sh
go run ./src/phase2   # rewrites svgsNumber/*.svg from digitMedians (preserves manual outline edits)
go run ./src/phase3   # rewrites graphicsNumber.txt from svgsNumber/*.svg
cp svgsNumber/*.svg demo/public/svgsNumber/   # demo/public/ is gitignored, manual sync
```

After editing `digitMedians` always run phase2 → phase3 → sync. The dev preview (`cd demo && npm run dev`) auto-reloads.

`src/glyphs/` holds Affinity Designer source files (`*.af`) and their SVG exports (`*.af.svg`) for digits whose outlines were hand-edited away from Klee One — open these in Affinity to update the outline, export to SVG, and paste the new outline `d=` into the matching `svgsNumber/*.svg` `<path id>` before running phase2. Currently hand-edited: `7` / `７` (added an upper-left flag so the digit can be drawn in 2 strokes) and `1` / `１` (removed the upper-left calligraphic flag, leaving only a small head on the vertical). See README for the rationale of each font modification.

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

## Coordinate system

Every digit is rendered into a 1024×1024 SVG with `viewBox="0 0 1024 1024"`, but the **median data in graphicsNumber.txt is in animCJK source space** (y-up), which is the convention hanzi-writer / kakitori expect.

| | x range | y range | y direction |
|---|---|---|---|
| SVG (y-down) | 0–1024 | 0–1024 | +y goes down |
| animCJK source (y-up) | 0–1024 | -124 to 900 (visible canvas) | +y goes up |

Conversion: `svg_y = 900 - source_y`. `phase1`/`phase2` apply this when emitting outline `d=` and median `d=` into the SVG; `phase3`'s `flipMedian` / `flipOutline` apply the inverse when re-reading.

A point `(x, y)` in source coordinates is **off-canvas** if `x < 0`, `x > 1024`, `y < -124`, or `y > 900`.

The **digit bbox** (x range / y range that the gray fill actually occupies) is roughly `x ∈ [320, 685]`, `y ∈ [50, 680]` for animNumber's Klee-One-derived glyphs — slightly larger than the inked region, smaller than the canvas. Concrete bbox per digit is whatever the outline `d=` describes; eyeball it from `svgsNumber/debug/<cp>.svg` (see "Verification" below).

## SVG structure

`phase2` regenerates the entire `<svg>` body from `digitMedians` plus the outline `<path id>`s preserved from the existing file. The structure for digit codepoint `<cp>`:

```svg
<svg id="z<cp>" class="acjk" viewBox="0 0 1024 1024" xmlns="...">
<style><![CDATA[ ...keyframes / per-path animation rules... ]]></style>

<!-- Outlines: one per data path. id encodes phase + part letter. -->
<path id="z<cp>d<phase>[<part>]" d="..."/>
<path id="z<cp>d<phase>[<part>]" d="..."/>

<!-- Clip paths reference the matching outline. -->
<defs>
  <clipPath id="z<cp>c<phase>[<part>]"><use href="#z<cp>d<phase>[<part>]"/></clipPath>
  ...
</defs>

<!-- Animated medians. style="--d:<phase>s;" delays each phase by 1 logical
     stroke duration; pathLength="3333" normalises every path so the
     concurrent stroke-dashoffset animation reaches 100% in 0.8s
     regardless of physical median length. -->
<path style="--d:1s;" pathLength="3333" clip-path="url(#z<cp>c1[<part>])" d="..."/>
...
```

ID conventions:
- `z<cp>d<phase><part>`: outline (data path). `phase` = logical stroke index (1-based). `part` = `""` if the phase has a single path, or `"a"`/`"b"`/`"c"` for splits.
- `z<cp>c<phase><part>`: clip path; same suffix as the outline.
- The clip path always wraps `<use href="#z<cp>d..."/>`, so changing the outline `d=` automatically changes the clip.

## Adding a new character

1. **Decide the logical stroke count** (= number of `phase`s). For a glyph that's traditionally written in N strokes, set N phases. animNumber's 0/1/2/3/6/8/9 are 1-stroke (phase 1 only); 4/5/7 are 2-stroke (phase 1 + phase 2).

2. **Decide whether each phase needs a multi-path split** (= `d<phase>a` + `d<phase>b` ...). Split is needed when the outline:
   - Encloses a region the user can't draw without crossing themselves (closed loops in 0/3/6/8/9, the 9 descender rejoining the bowl).
   - Has parts that visually look connected but should be drawn as one continuous motion (= one logical stroke) yet each part needs its own clip region for the visible drawing to look right.
   - Otherwise leave it as a single path (`Letter: ""`).

3. **Bootstrap the outline.** Add an entry to `phase1`'s `digits` array with the outline `d=` (in source y-up coords) and a starting median. Run `go run ./src/phase1` — this writes `svgsNumber/<cp>.svg` and appends the digit to `graphicsNumber.txt`.

4. **Hand-edit the outline if needed.** Open `svgsNumber/<cp>.svg` (or import into Affinity Designer; see `src/glyphs/`). Edit the `<path id="z<cp>d<phase>[<part>]">` `d=` attributes — split into a/b/c here if the phase needs multiple paths. Re-export and paste the outline `d=` back into `svgsNumber/<cp>.svg`. Keep the original Klee One attribution comment at the top of the SVG.

5. **Add the median definitions to `phase2`'s `digitMedians`.** One `Part` per `<path id>`. For multi-path stroke groups follow the invariants in the next section (`medians[0]` = full single-stroke trajectory; subsequent parts carry off-canvas pre-lead-in + visible portion + optional `LeadOut` for preview balance).

6. **Regenerate.** `go run ./src/phase2 && go run ./src/phase3 && go run ./src/debug`. Then sync the demo: `cp svgsNumber/*.svg demo/public/svgsNumber/`.

7. **Verify** — see below.

## Verifying a median

After defining or revising a median, run all of these:

### V1. Visual overlay (debug SVG)

`go run ./src/debug` writes `svgsNumber/debug/<cp>.svg` for every digit. Each debug SVG renders:
- The outline at 40% opacity with a dashed border.
- Every median as a thin coloured line, **without** clip-path occlusion (so the full median, including the parts that would normally be clipped out, is visible).
- A black-bordered dot at each median's first point and white-bordered dots at the rest.
- A legend in the top-left with each median's id, point count, and total length.

Open the debug SVG in a browser. Check:
- The coloured median line traces the **center** of the gray outline shape (not an edge or outside).
- Off-canvas points appear outside the dashed canvas border, only at the **start** (lead-in) of subsequent parts. Mid-trajectory off-canvas dots indicate a leaked tail.
- For multi-path stroke groups, `medians[0]` (red, "a") covers the entire stroke trajectory; `medians[1+]` (blue / green / ...) start off-canvas, enter the canvas at the visible-portion pickup point, and end where the digit's stroke would naturally end.

### V2. Segment length sanity

```sh
python3 - <<'EOF'
import json, math
with open('graphicsNumber.txt') as f:
    for line in f:
        e = json.loads(line)
        for i, m in enumerate(e['medians']):
            for j in range(len(m)-1):
                d = math.hypot(m[j+1][0]-m[j][0], m[j+1][1]-m[j][1])
                if d > 300:
                    print(f'{e["character"]} m{i} segment {j}: {m[j]} -> {m[j+1]} = {d:.0f}  (LONG)')
EOF
```

Any segment >300 source units is suspicious — it's usually either an off-canvas lead-in/lead-out (intentional, but should be at the start) or a missing intermediate point.

### V3. Off-canvas-point placement

Off-canvas points (`x<0`, `x>1024`, `y<-124`, `y>900`) in `Median` should appear only at the **start** of `medians[i]` for `i >= 1` (the dual-clip lead-in). If they appear in the middle of a median or at the end, that's a regression — either move them to `LeadOut` or rethink the trajectory.

### V4. animNumber preview

`cd demo && npm run dev`. Each multi-path digit should draw as one continuous stroke without mid-stroke pauses or visible overlaps where two parts draw simultaneously.

### V5. kakitori quiz

After `hanzi-writer-data-jp` is updated and `python stroke_data_parser.py` regenerates per-character JSON, draw the digit in one motion in `kakitori`'s demo. If `medians[0]` correctly traces the full single-stroke centerline, the quiz accepts the drawing.

### V6. animCJK reference cross-check

For multi-path stroke groups, compare the structure with the canonical animCJK references:
- `あ` (U+3042, `graphicsJaKana.txt`): 4 data paths, stroke 3 split into `d3a` + `d3b`. `medians[2]` = full curl trajectory.
- `ね` (U+306D, `graphicsJa.txt`): split similar to `あ`.
- `わ` (U+308F, `graphicsJaKana.txt`): 3-path split for stroke 1, comparable to animNumber's "9".

`phase3`'s heuristic timing check (`leadIn(B) >= total(A)`) over-reports for the full-trajectory convention; that's expected. The exact invariant is `leadIn(B) >= visible_portion_of(A in clip_A)`, which the debug overlay (V1) and kakitori quiz (V5) verify in practice.

## Stroke counts (do not change without README updates)

`0, 1, 2, 3, 6, 8, 9` are 1-stroke; `4, 5, 7` are 2-stroke. Multi-path splits (`d1a`/`d1b`/`d1c`) are an internal mechanism for the dual-clip technique within a single logical stroke — they are NOT additional strokes. The `strokes` count in graphicsNumber.txt reflects data paths, but the logical stroke count is exposed via the consumer's `strokeGroups` config.
