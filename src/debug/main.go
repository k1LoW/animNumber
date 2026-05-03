package main

// debug reads svgsNumber/*.svg and writes debug overlay SVGs to
// svgsNumber/debug/*.svg. Each debug SVG renders the outline as a
// semi-transparent gray fill, then overlays every median path as a thin
// coloured line WITHOUT clip-path, plus a small dot at every median
// point. Used to verify that the median traces the outline's centerline
// when adding or revising digits — if the median wanders away from the
// center or skips through dead space, it's immediately visible.

import (
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
	svgDir   = "svgsNumber"
	debugDir = "svgsNumber/debug"
)

var (
	idRe       = regexp.MustCompile(`^z(\d+)d(\d+)([a-z]*)$`)
	outlineRe  = regexp.MustCompile(`(?s)<path\s+id="([^"]+)"\s+d="([^"]+?)"\s*/>`)
	medianRe   = regexp.MustCompile(`(?s)<path\s+style="[^"]*"\s+pathLength="[^"]*"\s+clip-path="url\(#([^)]+)\)"\s+d="([^"]+?)"\s*/>`)
	medPointRe = regexp.MustCompile(`[ML]\s*(-?[\d.]+)\s+(-?[\d.]+)`)
)

// Color per part letter. Phase ignored — multi-phase digits get the same
// rotation so 1a/1b/1c stay visually distinct from 2a (etc.) only when
// it matters; for animNumber's 0-9 the rotation is enough.
var partColors = map[string]string{
	"":  "#d62728",
	"a": "#d62728", // red
	"b": "#1f77b4", // blue
	"c": "#2ca02c", // green
	"d": "#9467bd", // purple
	"e": "#ff7f0e", // orange
}

func clipToOutlineID(clip string) string {
	return strings.Replace(clip, "c", "d", 1)
}

func partLetter(id string) string {
	m := idRe.FindStringSubmatch(id)
	if m == nil {
		return ""
	}
	return m[3]
}

func processSVG(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(data)

	type outline struct {
		ID, D string
	}
	type median struct {
		OutlineID string
		D         string
		Points    [][2]float64
	}

	var outlines []outline
	var medians []median

	for _, m := range outlineRe.FindAllStringSubmatch(text, -1) {
		if !idRe.MatchString(m[1]) {
			continue
		}
		outlines = append(outlines, outline{ID: m[1], D: m[2]})
	}
	for _, m := range medianRe.FindAllStringSubmatch(text, -1) {
		oid := clipToOutlineID(m[1])
		var pts [][2]float64
		for _, pm := range medPointRe.FindAllStringSubmatch(m[2], -1) {
			x, _ := strconv.ParseFloat(pm[1], 64)
			y, _ := strconv.ParseFloat(pm[2], 64)
			pts = append(pts, [2]float64{x, y})
		}
		medians = append(medians, median{OutlineID: oid, D: m[2], Points: pts})
	}

	sort.SliceStable(outlines, func(i, j int) bool { return outlines[i].ID < outlines[j].ID })
	sort.SliceStable(medians, func(i, j int) bool { return medians[i].OutlineID < medians[j].OutlineID })

	var sb strings.Builder
	sb.WriteString(`<svg viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">` + "\n")
	sb.WriteString(`<style><![CDATA[
.outline { fill: #ccc; fill-opacity: 0.4; stroke: #888; stroke-width: 1; stroke-dasharray: 4 4; }
.median { fill: none; stroke-width: 3; }
.median-pt { stroke: white; stroke-width: 1; }
.median-pt-first { stroke: black; stroke-width: 2; }
.median-label { font-family: monospace; font-size: 14px; fill: #333; }
]]></style>` + "\n")

	// Canvas reference (faint border)
	sb.WriteString(`<rect x="0" y="0" width="1024" height="1024" fill="none" stroke="#ddd"/>` + "\n")

	// Outlines
	for _, o := range outlines {
		sb.WriteString(fmt.Sprintf(`<path class="outline" d="%s"/>`+"\n", o.D))
	}

	// Medians and their points
	for _, m := range medians {
		letter := partLetter(m.OutlineID)
		color := partColors[letter]
		if color == "" {
			color = "#000000"
		}
		sb.WriteString(fmt.Sprintf(`<path class="median" stroke="%s" d="%s"/>`+"\n", color, m.D))
		for i, p := range m.Points {
			cls := "median-pt"
			r := "5"
			if i == 0 {
				cls = "median-pt-first"
				r = "8"
			}
			sb.WriteString(fmt.Sprintf(`<circle class="%s" cx="%.0f" cy="%.0f" r="%s" fill="%s"/>`+"\n",
				cls, p[0], p[1], r, color))
		}
	}

	// Legend
	sb.WriteString(`<g transform="translate(10, 20)">` + "\n")
	y := 0
	for _, m := range medians {
		letter := partLetter(m.OutlineID)
		color := partColors[letter]
		if color == "" {
			color = "#000000"
		}
		label := m.OutlineID
		if letter == "" {
			label = m.OutlineID + " (single path)"
		}
		sb.WriteString(fmt.Sprintf(`<text class="median-label" x="0" y="%d" fill="%s">%s — %d pts, length %.0f</text>`+"\n",
			y, color, label, len(m.Points), pathLength(m.Points)))
		y += 18
	}
	sb.WriteString(`</g>` + "\n")

	sb.WriteString(`</svg>` + "\n")
	return sb.String(), nil
}

func pathLength(pts [][2]float64) float64 {
	total := 0.0
	for i := 0; i < len(pts)-1; i++ {
		dx := pts[i+1][0] - pts[i][0]
		dy := pts[i+1][1] - pts[i][1]
		total += math.Sqrt(dx*dx + dy*dy)
	}
	return total
}

func main() {
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", debugDir, err)
		os.Exit(1)
	}
	for cp := 48; cp <= 57; cp++ {
		in := filepath.Join(svgDir, fmt.Sprintf("%d.svg", cp))
		out := filepath.Join(debugDir, fmt.Sprintf("%d.svg", cp))
		svg, err := processSVG(in)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", in, err)
			os.Exit(1)
		}
		if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %s\n", out)
	}
}
