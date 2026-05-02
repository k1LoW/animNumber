package main

// phase2 reads existing svgsNumber/*.svg files, preserves their <path id="...">
// outline elements (including any user-applied splits like z48d1a / z48d1b),
// and rewrites everything else (style, clipPath, median paths). Use this after
// editing outlines by hand to refresh the animation wiring.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Point struct{ X, Y float64 }

type Part struct {
	Letter string  // "" for single-part, "a"/"b"/... for splits
	Median []Point
}

type Phase struct {
	Number int
	Parts  []Part
}

type DigitMedians struct {
	Char   rune
	Phases []Phase
}

const outDir = "svgsNumber"

// Median definitions. Mirrors phase1's medians as the starting point.
// When you split an outline (e.g. z48d1 -> z48d1a + z48d1b), update the
// matching DigitMedians entry to define a Median per Part letter.
var digitMedians = []DigitMedians{
	{Char: '0', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full CCW loop from upper-left split, traced through left
			// arc, bottom, right arc, top, back to the split. Clipped to
			// the left half so only the left arc draws; the right-side
			// portion of the median is occluded by the clip.
			{Letter: "a", Median: []Point{
				{415, 630}, {380, 580}, {360, 500}, {350, 380},
				{370, 270}, {410, 180}, {470, 110}, {510, 85},
				{580, 110}, {620, 170}, {650, 250}, {660, 380},
				{650, 500}, {620, 600}, {580, 660}, {512, 680},
				{450, 670}, {415, 630},
			}},
			// b: off-canvas lead-in delays b's visible drawing until the
			// right half. Final point [415, 630] matches a's endpoint so
			// the loop closure is shared between the two paths (animCJK
			// "あ"-stroke-3 pattern).
			{Letter: "b", Median: []Point{
				{-101, 85}, {510, 85}, {580, 110}, {620, 170},
				{650, 250}, {660, 380}, {650, 500}, {620, 600},
				{580, 660}, {512, 680}, {450, 670}, {415, 630},
			}},
		}},
	}},
	{Char: '1', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{440, 580}, {500, 630}, {570, 670}, {570, 80},
			}},
		}},
	}},
	{Char: '2', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{360, 580}, {410, 630}, {470, 660}, {540, 660},
				{600, 630}, {640, 580}, {650, 530}, {630, 470},
				{570, 400}, {490, 320}, {410, 230}, {350, 150},
				{320, 100}, {430, 90}, {560, 90}, {700, 100},
			}},
		}},
	}},
	{Char: '3', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: upper bump CCW from upper-left, around top, down to the
			// middle pinch. Followed by a vertical off-canvas extension
			// downward so a stays invisible during b's visible portion,
			// then back up to the shared endpoint at the lower-left curl
			// (matching b's end).
			{Letter: "a", Median: []Point{
				{380, 610}, {430, 650}, {520, 650}, {590, 630}, {640, 600},
				{650, 540}, {650, 470}, {620, 430}, {570, 410}, {480, 410},
				{480, -200},
				{340, 170},
			}},
			// b: long vertical lead-in from above the canvas keeps b
			// invisible until a finishes the upper bump, then b traces
			// the lower bump CW around to the lower-left curl tail.
			// Final point shared with a (animCJK pattern).
			{Letter: "b", Median: []Point{
				{480, 1149},
				{480, 410}, {550, 390}, {620, 360}, {670, 310}, {680, 240},
				{660, 170}, {610, 110}, {520, 90}, {430, 100}, {370, 140},
				{340, 170},
			}},
		}},
	}},
	{Char: '4', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{585, 660}, {430, 400}, {310, 230}, {500, 230}, {720, 240},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{600, 670}, {600, 80},
			}},
		}},
	}},
	{Char: '5', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{400, 640}, {520, 640}, {670, 640},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{390, 660}, {380, 580}, {370, 480}, {360, 440},
				{360, 380}, {390, 390}, {430, 400},
				{490, 420}, {560, 410}, {620, 380},
				{660, 320}, {660, 230}, {620, 160},
				{550, 110}, {460, 100}, {380, 130},
				{320, 170},
			}},
		}},
	}},
	{Char: '6', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full single-stroke "6" centerline. Clipped to c1a (left
			// half + tail), only the first portion (tail through bottom
			// arc) is visible; the right-half portion is occluded by the
			// clip. Final point shared with b (animCJK pattern).
			{Letter: "a", Median: []Point{
				{570, 660}, {490, 580}, {430, 480}, {380, 380},
				{340, 290}, {320, 220}, {350, 150}, {410, 100},
				{490, 80}, {570, 100}, {640, 140}, {680, 220},
				{680, 290}, {640, 370}, {570, 420}, {490, 430},
				{420, 405}, {390, 380}, {350, 340},
			}},
			// b: off-canvas lead-in delays b's visible right-bowl portion
			// until a finishes the visible left arc. Capped at -160
			// (≤500 outside bbox). Endpoint matches a.
			{Letter: "b", Median: []Point{
				{-160, 100}, {570, 100}, {640, 140}, {680, 220},
				{680, 290}, {640, 370}, {570, 420}, {490, 430},
				{420, 405}, {390, 380}, {350, 340},
			}},
		}},
	}},
	{Char: '7', Phases: []Phase{
		// 1画目: small downward flag at the upper-left
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{336, 645}, {347, 560}, {357, 480},
			}},
		}},
		// 2画目: top horizontal continuing into the diagonal down-left
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{395, 615}, {520, 625}, {660, 625},
				{580, 450}, {500, 280}, {440, 130}, {420, 80},
			}},
		}},
	}},
	{Char: '8', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: right S — upper-right tab, CCW around upper loop top,
			// down the left wall, waist X-crossing, down right wall of
			// lower loop, to bottom-mid. Then off-canvas-left (capped at
			// -160, ≤500 outside bbox) so a stays invisible during b's
			// visible left S, and finally jumps to the shared endpoint
			// at the upper-right tab.
			{Letter: "a", Median: []Point{
				{640, 620}, {610, 650}, {510, 650}, {430, 650}, {400, 620},
				{370, 560}, {360, 500}, {380, 450}, {420, 420}, {490, 400},
				{550, 390}, {620, 360}, {660, 300}, {680, 230}, {660, 160},
				{610, 110}, {490, 80},
				{-160, 80},
				{685, 570},
			}},
			// b: vertical off-canvas lead-in from below (capped at -420,
			// ≤500 outside bbox) keeps b invisible until a finishes its
			// right S. b then traces the LEFT wall of the lower loop UP,
			// through the waist X-crossing up-right, ending at the shared
			// upper-right tab endpoint.
			{Letter: "b", Median: []Point{
				{490, -420}, {490, 80},
				{440, 80}, {380, 110}, {350, 170}, {340, 230}, {340, 290},
				{360, 350}, {410, 380}, {460, 410}, {530, 430}, {580, 460},
				{620, 490}, {660, 510}, {680, 540}, {685, 570},
			}},
		}},
	}},
	{Char: '9', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: bowl CCW from upper-right around to bowl-bottom. Then a
			// vertical off-canvas dip (capped at -200, ≤500 outside
			// bbox) keeps a invisible during b's closure, and finally
			// jumps to the shared endpoint at the descender foot.
			{Letter: "a", Median: []Point{
				{640, 600}, {570, 660}, {490, 670}, {420, 660}, {370, 620},
				{350, 560}, {340, 480}, {340, 410}, {370, 360}, {400, 330},
				{420, 320},
				{420, -200},
				{585, 50},
			}},
			// b: vertical off-canvas lead-in from below (capped at -180)
			// keeps b invisible during a's bowl drawing. b then traces
			// the closure up to the upper-right corner, then exits to
			// off-canvas right ([800, 650] is 115 outside bbox right) to
			// stay invisible during c's descender, ending at the shared
			// foot endpoint.
			{Letter: "b", Median: []Point{
				{440, -180},
				{440, 320}, {490, 320}, {530, 350}, {560, 380}, {580, 410},
				{620, 440}, {640, 470}, {660, 500}, {680, 540}, {685, 580},
				{685, 620}, {680, 650},
				{800, 650},
				{585, 50},
			}},
			// c: long off-canvas left lead-in (capped at -160, ≤500
			// outside bbox; routed via [-160, 100] -> [-160, 650] to
			// reach the path length needed to delay c until ~67% of the
			// animation), then traces the descender down to the shared
			// foot endpoint.
			{Letter: "c", Median: []Point{
				{-160, 100}, {-160, 650},
				{680, 650}, {665, 580}, {650, 510}, {640, 440}, {625, 380},
				{615, 310}, {610, 240}, {605, 170}, {595, 100}, {585, 50},
			}},
		}},
	}},
}

// SVG path id format: z<codepoint>d<phase>[<partletter>]
// e.g. z48d1, z48d1a, z48d2
var idRe = regexp.MustCompile(`^z(\d+)d(\d+)([a-z]*)$`)

// Match <path id="..." d="..."/> from the existing SVG. We deliberately only
// extract paths with both id and d attributes, ignoring the animated median
// paths (which use clip-path instead of id). Trailing attributes like
// style="..." are allowed and discarded; we only need id and d.
var pathRe = regexp.MustCompile(`(?s)<path\s+id="([^"]+)"\s+d="([^"]+?)"[^>]*/?>`)

type Outline struct {
	ID    string
	Phase int
	Part  string
	D     string
}

func parseSVG(path string) ([]Outline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	var outlines []Outline
	for _, m := range pathRe.FindAllStringSubmatch(text, -1) {
		id, d := m[1], m[2]
		idMatch := idRe.FindStringSubmatch(id)
		if idMatch == nil {
			continue
		}
		phase, _ := strconv.Atoi(idMatch[2])
		outlines = append(outlines, Outline{
			ID:    id,
			Phase: phase,
			Part:  idMatch[3],
			D:     d,
		})
	}
	return outlines, nil
}

func findMedian(char rune, phase int, part string) []Point {
	for _, dm := range digitMedians {
		if dm.Char != char {
			continue
		}
		for _, ph := range dm.Phases {
			if ph.Number != phase {
				continue
			}
			for _, pt := range ph.Parts {
				if pt.Letter == part {
					return pt.Median
				}
			}
		}
	}
	return nil
}

// --- coordinate transform (animCJK source y-up -> SVG y-down) ---

var reTransform = regexp.MustCompile(`([MQCLZ ]+)([0-9.-]+) ([0-9.-]+)`)

func transformPath(path string) string {
	path = strings.ReplaceAll(path, ",", " ")
	re1 := regexp.MustCompile(`\s?([MQCLZ])\s?`)
	path = re1.ReplaceAllString(path, "$1")
	re2 := regexp.MustCompile(`([^ ])-`)
	path = re2.ReplaceAllString(path, "$1 -")

	hasZ := strings.Contains(path, "Z")

	result := reTransform.ReplaceAllStringFunc(path, func(s string) string {
		m := reTransform.FindStringSubmatch(s)
		if len(m) < 4 {
			return s
		}
		x := parseCoord(m[2])
		y := 900 - parseCoord(m[3])
		return m[1] + strconv.Itoa(x) + " " + strconv.Itoa(y)
	})

	if hasZ && !strings.Contains(result, "Z") {
		result += "Z"
	}
	return result
}

func parseCoord(s string) int {
	f, _ := strconv.ParseFloat(s, 64)
	return int(f)
}

func buildMedianPath(median []Point) string {
	var sb strings.Builder
	for i, p := range median {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("M%d %d", int(p.X), int(p.Y)))
		} else {
			sb.WriteString(fmt.Sprintf("L%d %d", int(p.X), int(p.Y)))
		}
	}
	return sb.String()
}

func buildStyle() string {
	return `<style>
<![CDATA[
@keyframes zk {
	to {
		stroke-dashoffset:0;
	}
}
svg.acjk path[clip-path] {
	--t:0.8s;
	animation:zk var(--t) linear forwards var(--d);
	stroke-dasharray:3337;
	stroke-dashoffset:3339;
	stroke-width:128;
	stroke-linecap:round;
	stroke-linejoin:round;
	fill:none;
	stroke:#000;
}
svg.acjk path[id] {fill:#ccc;}
]]>
</style>
`
}

func svgComment() string {
	return `<!--
animNumber 2026 Copyright k1LoW, https://github.com/k1LoW/animNumber
Derived from:
    Klee One - https://github.com/fontworks-fonts/Klee
    Copyright 2020 The Klee Project Authors
You can redistribute and/or modify this file under the terms of the SIL Open Font License, Version 1.1
as published by SIL International.
You should have received a copy of this license along with this file.
If not, see https://openfontlicense.org/.
-->
`
}

func clipID(outlineID string) string {
	// z48d1a -> z48c1a
	return strings.Replace(outlineID, "d", "c", 1)
}

func buildSVG(char rune, outlines []Outline) string {
	cp := int(char)
	svgID := fmt.Sprintf("z%d", cp)

	sort.SliceStable(outlines, func(i, j int) bool {
		if outlines[i].Phase != outlines[j].Phase {
			return outlines[i].Phase < outlines[j].Phase
		}
		return outlines[i].Part < outlines[j].Part
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg id="%s" class="acjk" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">`, svgID))
	sb.WriteString("\n")
	sb.WriteString(buildStyle())

	for _, o := range outlines {
		sb.WriteString(fmt.Sprintf(`<path id="%s" d="%s"/>`, o.ID, o.D))
		sb.WriteString("\n")
	}

	sb.WriteString("<defs>\n")
	for _, o := range outlines {
		sb.WriteString(fmt.Sprintf("\t"+`<clipPath id="%s"><use href="#%s"/></clipPath>`, clipID(o.ID), o.ID))
		sb.WriteString("\n")
	}
	sb.WriteString("</defs>\n")

	for _, o := range outlines {
		median := findMedian(char, o.Phase, o.Part)
		if median == nil {
			sb.WriteString(fmt.Sprintf("<!-- missing median for %s -->\n", o.ID))
			continue
		}
		medianPath := buildMedianPath(median)
		transformed := transformPath(medianPath)
		sb.WriteString(fmt.Sprintf(`<path style="--d:%ds;" pathLength="3333" clip-path="url(#%s)" d="%s"/>`,
			o.Phase, clipID(o.ID), transformed))
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

func main() {
	for cp := 48; cp <= 57; cp++ {
		path := filepath.Join(outDir, fmt.Sprintf("%d.svg", cp))
		outlines, err := parseSVG(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
			os.Exit(1)
		}
		if len(outlines) == 0 {
			fmt.Fprintf(os.Stderr, "No <path id> found in %s\n", path)
			continue
		}
		char := rune(cp)
		svg := svgComment() + buildSVG(char, outlines)
		if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("Updated %s (%d outlines)\n", path, len(outlines))
	}
	fmt.Println("Done!")
}
