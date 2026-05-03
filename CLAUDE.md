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

### 1. `Median` boundary

- Each `Median` ends at its own **visible-end pickup point** (where the next part's visible portion begins). Do NOT force a shared endpoint across the stroke group via `Median` (that breaks the dual-clip "b picks up exactly where a finishes" invariant).
- Closed loops (e.g. `0`) are an exception: a's median may dive past the natural close into b's lead-in pickup so that a/b genuinely share an endpoint.

### 2. `Median` magnitude (≤500 outside bbox)

For every point `(x, y)` in `Median`:
- `x` within `[bbox.left - 500, bbox.right + 500]`
- `y` within `[bbox.bottom - 500, bbox.top + 500]`

Values like `-968` / `1237` / `-558` are too far outside bbox and are NOT acceptable in graphicsNumber.txt.

### 3. kakitori timing invariant

`kakitori`'s `animateWithGroups` (`Kakitori.ts:710`) starts every path in a strokeGroup at **delay=0** and animates each for a duration **proportional to its median length** (in `HANZI_COORD_SIZE=900` units). For path B's visible drawing not to start before path A's visible drawing has finished, the median data must satisfy:

```
leadIn(B) >= total(A)
```

where:
- `total(X)` = sum of segment lengths in `X.Median`
- `leadIn(B)` = sum of segment lengths in `B.Median` from index 0 up to (and including) the first segment whose endpoint is on-canvas (i.e. the prefix that is invisible due to clip occlusion / off-canvas position)

For three-path groups (e.g. `9`), also `leadIn(C) >= total(B)`.

Concretely:

| digit | parts | invariant | values |
|---|---|---|---|
| 8 | a, b | `leadIn(b) >= total(a)` | `1150 >= 1119` ✓ |
| 9 | a, b, c | `leadIn(b) >= total(a)`, `leadIn(c) >= total(b)` | `670 >= 644`, `1430 >= 1126` ✓ |

If you change `Median` lengths, **recompute these inequalities** and adjust lead-ins. Failing the inequality makes the strokes appear to draw in parallel in kakitori (the user-visible "1画として描画されない" symptom).

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

## Verification snippet

```python
import json, math
with open('graphicsNumber.txt') as f:
    for line in f:
        e = json.loads(line)
        if e['character'] not in '89': continue
        meds = e['medians']
        totals = [sum(math.hypot(m[i+1][0]-m[i][0], m[i+1][1]-m[i][1])
                       for i in range(len(m)-1)) for m in meds]
        # leadIn = sum of segments from start up to first on-canvas point
        def leadin(m):
            for i, p in enumerate(m):
                if 0 <= p[0] <= 1024 and -124 <= p[1] <= 900:
                    return sum(math.hypot(m[j+1][0]-m[j][0], m[j+1][1]-m[j][1])
                               for j in range(i)) if i > 0 else 0
            return 0
        leadins = [leadin(m) for m in meds]
        for i in range(len(totals)-1):
            ok = '✓' if leadins[i+1] >= totals[i] else '✗ kakitori OVERLAP'
            print(f'{e["character"]} d{i}->d{i+1}: leadIn={leadins[i+1]:.0f} >= total(prev)={totals[i]:.0f} {ok}')
```

Run after every edit. Both inequalities must hold.

## Stroke counts (do not change without README updates)

`0, 1, 2, 3, 6, 8, 9` are 1-stroke; `4, 5, 7` are 2-stroke. Multi-path splits (`d1a`/`d1b`/`d1c`) are an internal mechanism for the dual-clip technique within a single logical stroke — they are NOT additional strokes. The `strokes` count in graphicsNumber.txt reflects data paths, but the logical stroke count is exposed via the consumer's `strokeGroups` config.
