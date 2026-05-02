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

// flipMedian parses an SVG median d="M x y L x y L x y..." path and returns
// the points in animCJK source space (y-up). Trailing off-canvas points
// (which phase2 emits as LeadOut for stroke-dashoffset timing in the SVG)
// are stripped so graphicsNumber.txt only contains the clean centerline.
// Leading off-canvas points are kept — they are valid lead-ins that
// hanzi-writer's dual-clip technique relies on.
func flipMedian(d string) [][2]int {
	d = strings.ReplaceAll(d, ",", " ")
	matches := medSegRe.FindAllStringSubmatch(d, -1)
	pts := make([][2]int, 0, len(matches))
	for _, m := range matches {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		pts = append(pts, [2]int{int(math.Round(x)), 900 - int(math.Round(y))})
	}
	for len(pts) > 0 {
		last := pts[len(pts)-1]
		sx, sy := last[0], last[1]
		// SVG canvas is 0-1024; in source y-up that maps to y in [-124, 900].
		offCanvas := sx < 0 || sx > 1024 || sy < -124 || sy > 900
		if !offCanvas {
			break
		}
		pts = pts[:len(pts)-1]
	}
	return pts
}

func clipToOutlineID(clip string) string {
	// z48c1a -> z48d1a
	return strings.Replace(clip, "c", "d", 1)
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

	for cp := 48; cp <= 57; cp++ {
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
	}
	fmt.Printf("Written %s\n", graphicsFile)
}
