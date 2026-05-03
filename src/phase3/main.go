package main

// phase3 reads svgsNumber/*.svg and rewrites graphicsNumber.txt by treating
// the SVG as the source of truth. Each <path id="z<cp>d<phase>[<part>]">
// outline becomes one entry in `strokes`, and each animated <path clip-path>
// median becomes the matching entry in `medians`. y-coordinates are flipped
// from SVG (y-down) back to animCJK source space (y-up). Run after editing
// outlines (and after phase2 refreshed the median wiring) to keep
// graphicsNumber.txt in sync with the SVGs.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	svgDir       = "svgsNumber"
	graphicsFile = "graphicsNumber.txt"
)

type GraphicsEntry struct {
	Character string     `json:"character"`
	Strokes   []string   `json:"strokes"`
	Medians   [][][2]int `json:"medians"`
}

var (
	idRe       = regexp.MustCompile(`^z(\d+)d(\d+)([a-z]*)$`)
	outlineRe  = regexp.MustCompile(`(?s)<path\s+id="([^"]+)"\s+d="([^"]+?)"\s*/>`)
	medianRe   = regexp.MustCompile(`(?s)<path\s+style="[^"]*"\s+pathLength="[^"]*"\s+clip-path="url\(#([^)]+)\)"\s+d="([^"]+?)"\s*/>`)
	cmdSpaceRe = regexp.MustCompile(`\s?([MQCLZ])\s?`)
	negSpaceRe = regexp.MustCompile(`([^ ])-`)
	xfRe       = regexp.MustCompile(`([MQCLZ ]+)(-?[0-9.]+) (-?[0-9.]+)`)
	medSegRe   = regexp.MustCompile(`[ML]\s*(-?[\d.]+)\s+(-?[\d.]+)`)
)

// flipOutline takes an SVG outline path (y-down, may have float coords and
// commas) and returns the equivalent path in animCJK source space (y-up,
// integer coords, normalized whitespace) — the inverse of phase1's
// transformPath, which applies the same y=900-y flip.
func flipOutline(p string) string {
	p = strings.ReplaceAll(p, ",", " ")
	p = cmdSpaceRe.ReplaceAllString(p, "$1")
	p = negSpaceRe.ReplaceAllString(p, "$1 -")

	hasZ := strings.Contains(p, "Z")
	out := xfRe.ReplaceAllStringFunc(p, func(s string) string {
		m := xfRe.FindStringSubmatch(s)
		if len(m) < 4 {
			return s
		}
		x := roundCoord(m[2])
		y := 900 - roundCoord(m[3])
		return m[1] + strconv.Itoa(x) + " " + strconv.Itoa(y)
	})
	if hasZ && !strings.Contains(out, "Z") {
		out += "Z"
	}
	return out
}

func roundCoord(s string) int {
	f, _ := strconv.ParseFloat(s, 64)
	return int(math.Round(f))
}

// SVG canvas is 0-1024; in source y-up that maps to y in [-124, 900].
func isOffCanvas(p [2]int) bool {
	return p[0] < 0 || p[0] > 1024 || p[1] < -124 || p[1] > 900
}

// flipMedian parses an SVG median d="M x y L x y L x y..." path and returns
// the points in animCJK source space (y-up). The trailing tail (LeadOut
// in phase2's Part — used for stroke-dashoffset timing in the SVG only)
// is stripped so graphicsNumber.txt only contains the clean centerline.
//
// LeadOut always begins with at least one off-canvas point (acting as a
// boundary marker), even when the rest of the LeadOut is on-canvas (e.g.
// "6" d1a where the post-visible trace stays inside the canvas). The
// marker lets us locate the centerline / tail boundary: we walk past the
// leading lead-in (if any), and once we find the first trailing
// off-canvas point, we strip everything from that index to the end.
//
// Leading off-canvas points (the dual-clip lead-in) are kept — kakitori
// uses them to delay the visible-portion start of subsequent stroke-group
// members.
func flipMedian(d string) [][2]int {
	d = strings.ReplaceAll(d, ",", " ")
	matches := medSegRe.FindAllStringSubmatch(d, -1)
	pts := make([][2]int, 0, len(matches))
	for _, m := range matches {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		pts = append(pts, [2]int{int(math.Round(x)), 900 - int(math.Round(y))})
	}
	firstOnCanvas := -1
	for i, p := range pts {
		if !isOffCanvas(p) {
			firstOnCanvas = i
			break
		}
	}
	if firstOnCanvas < 0 {
		return pts
	}
	for i := firstOnCanvas + 1; i < len(pts); i++ {
		if isOffCanvas(pts[i]) {
			pts = pts[:i]
			break
		}
	}
	return pts
}

// medianLength returns the sum of segment lengths in a median (in animCJK
// source units).
func medianLength(pts [][2]int) float64 {
	total := 0.0
	for i := 0; i < len(pts)-1; i++ {
		dx := float64(pts[i+1][0] - pts[i][0])
		dy := float64(pts[i+1][1] - pts[i][1])
		total += math.Sqrt(dx*dx + dy*dy)
	}
	return total
}

// leadInLength returns the length of the leading off-canvas portion of a
// median (the segments traversed before the median first enters the SVG
// canvas).
func leadInLength(pts [][2]int) float64 {
	firstOn := -1
	for i, p := range pts {
		if !isOffCanvas(p) {
			firstOn = i
			break
		}
	}
	if firstOn <= 0 {
		return 0
	}
	total := 0.0
	for i := 0; i < firstOn; i++ {
		dx := float64(pts[i+1][0] - pts[i][0])
		dy := float64(pts[i+1][1] - pts[i][1])
		total += math.Sqrt(dx*dx + dy*dy)
	}
	return total
}

// validateStrokeGroupTiming reports informational notes about kakitori's
// strokeGroup playback timing. kakitori's animateWithGroups
// (Kakitori.ts:710) starts every group member at delay=0 and animates
// each for a duration proportional to its median length. For path B's
// visible drawing to start after path A's visible drawing finishes, the
// invariant is leadIn(B) >= visible_portion_of(A in c1A).
//
// Computing visible_portion_of(A) accurately requires intersecting A's
// median with c1A's clip polygon (out of scope here). As a coarse
// heuristic this validator compares leadIn(B) against total(A); when
// total(A) > visible(A) (e.g. when m_a traces the full single-stroke
// trajectory rather than just its visible portion — the animCJK
// "あ"-stroke-3 / "ね" convention), the heuristic over-reports. The
// notes are printed for review but do not fail the run.
func validateStrokeGroupTiming(char rune, rows []partRow) []string {
	groups := map[int][]partRow{}
	for _, r := range rows {
		groups[r.Phase] = append(groups[r.Phase], r)
	}
	var problems []string
	phases := make([]int, 0, len(groups))
	for p := range groups {
		phases = append(phases, p)
	}
	sort.Ints(phases)
	for _, p := range phases {
		g := groups[p]
		if len(g) < 2 {
			continue
		}
		for i := 0; i < len(g)-1; i++ {
			a, b := g[i], g[i+1]
			ta := medianLength(a.Median)
			lb := leadInLength(b.Median)
			if lb < ta {
				problems = append(problems,
					fmt.Sprintf("%s d%d%s -> d%d%s: leadIn(d%d%s)=%.0f < total(d%d%s)=%.0f — kakitori would render this group as overlapping strokes (Kakitori.ts:710)",
						string(char), p, a.Part, p, b.Part, p, b.Part, lb, p, a.Part, ta))
			}
		}
	}
	return problems
}

func clipToOutlineID(clip string) string {
	// z48c1a -> z48d1a
	return strings.Replace(clip, "c", "d", 1)
}

// listSVGCodepoints returns codepoints of every "<int>.svg" in svgDir.
func listSVGCodepoints() ([]int, error) {
	entries, err := os.ReadDir(svgDir)
	if err != nil {
		return nil, err
	}
	var cps []int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".svg")
		cp, err := strconv.Atoi(base)
		if err != nil {
			continue
		}
		cps = append(cps, cp)
	}
	sort.Ints(cps)
	return cps, nil
}

type partRow struct {
	Phase   int
	Part    string
	Outline string
	Median  [][2]int
}

func processSVG(path string) ([]partRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)

	outlines := map[string]string{}
	medians := map[string][][2]int{}

	for _, m := range outlineRe.FindAllStringSubmatch(text, -1) {
		id, d := m[1], m[2]
		if !idRe.MatchString(id) {
			continue
		}
		outlines[id] = flipOutline(d)
	}
	for _, m := range medianRe.FindAllStringSubmatch(text, -1) {
		clipID, d := m[1], m[2]
		outlineID := clipToOutlineID(clipID)
		medians[outlineID] = flipMedian(d)
	}

	rows := make([]partRow, 0, len(outlines))
	for id, outline := range outlines {
		match := idRe.FindStringSubmatch(id)
		phase, _ := strconv.Atoi(match[2])
		part := match[3]
		rows = append(rows, partRow{
			Phase:   phase,
			Part:    part,
			Outline: outline,
			Median:  medians[id],
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Phase != rows[j].Phase {
			return rows[i].Phase < rows[j].Phase
		}
		return rows[i].Part < rows[j].Part
	})
	return rows, nil
}

func main() {
	f, err := os.Create(graphicsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", graphicsFile, err)
		os.Exit(1)
	}
	defer f.Close()

	cps, err := listSVGCodepoints()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing %s: %v\n", svgDir, err)
		os.Exit(1)
	}
	var allProblems []string
	for _, cp := range cps {
		path := filepath.Join(svgDir, fmt.Sprintf("%d.svg", cp))
		rows, err := processSVG(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
			os.Exit(1)
		}
		if len(rows) == 0 {
			fmt.Fprintf(os.Stderr, "No <path id> found in %s\n", path)
			continue
		}
		entry := GraphicsEntry{Character: string(rune(cp))}
		for _, r := range rows {
			entry.Strokes = append(entry.Strokes, r.Outline)
			entry.Medians = append(entry.Medians, r.Median)
		}
		line, _ := json.Marshal(entry)
		fmt.Fprintln(f, string(line))
		fmt.Printf("Processed %s (%s, %d parts)\n", path, entry.Character, len(rows))
		if probs := validateStrokeGroupTiming(rune(cp), rows); len(probs) > 0 {
			allProblems = append(allProblems, probs...)
		}
	}
	fmt.Printf("Written %s\n", graphicsFile)

	if len(allProblems) > 0 {
		fmt.Fprintln(os.Stderr, "\nKakitori timing notes (heuristic — see validateStrokeGroupTiming):")
		for _, p := range allProblems {
			fmt.Fprintln(os.Stderr, "  "+p)
		}
		fmt.Fprintln(os.Stderr, "\nThese flags compare leadIn(b) against total(a). They over-report when m_a traces the full single-stroke trajectory (animCJK / kakitori \"medians[0] is the whole stroke\" convention). Verify visually in the animNumber preview and against kakitori's quiz — the operative invariant is leadIn(b) >= visible_portion_of(a in clip_a). See CLAUDE.md.")
	}
}
