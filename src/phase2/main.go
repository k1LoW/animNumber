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
	Letter  string  // "" for single-part, "a"/"b"/... for splits
	Median  []Point // clean centerline (output to graphicsNumber.txt as-is; lead-in points kept here are valid for hanzi-writer dual-clip technique)
	LeadOut []Point // SVG-only, appended after Median in the SVG <path> d attribute. Used to lengthen the SVG stroke-dashoffset path so the visible drawing of this part finishes before the next part's visible drawing starts. Stripped from graphicsNumber.txt.
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
			// a: full CCW loop trajectory (animCJK "あ"-stroke-3 / kana "ね"
			// convention). kakitori's strokeMatches compares the user's
			// 1-stroke drawing against medians[0], so m_a must trace the
			// whole "0" centerline (clip c1a covers left half + closure
			// wedge — only that subset draws visibly, but the median data
			// itself is the full trajectory).
			{Letter: "a", Median: []Point{
				{398, 680}, {357, 621}, {333, 527}, {321, 386},
				{345, 256}, {392, 150}, {463, 68}, {510, 39},
				{592, 68}, {639, 139}, {674, 233}, {686, 386},
				{674, 527}, {639, 644}, {592, 715}, {512, 739},
				{439, 727}, {398, 680}, {386, 592},
			}},
			// b: off-canvas lead-in delays b's visible drawing until
			// after a's first visible portion (left arc, ~614 units)
			// finishes. The final two points {410, 674}, {386, 592} trace
			// into the upper-left wedge of c1b so the stroke covers that
			// thin closure region.
			{Letter: "b", Median: []Point{
				{-267, 39}, {510, 39}, {592, 68}, {639, 139},
				{674, 233}, {686, 386}, {674, 527}, {639, 644},
				{592, 715}, {512, 739}, {439, 727}, {410, 674},
				{386, 592},
			}},
		}},
	}},
	{Char: '1', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{514, 727}, {514, 33},
			}},
		}},
	}},
	{Char: '2', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{333, 621}, {392, 680}, {463, 715}, {545, 715},
				{616, 680}, {663, 621}, {674, 562}, {651, 492},
				{580, 409}, {486, 315}, {392, 209}, {321, 115},
				{286, 56}, {416, 44}, {568, 44}, {733, 56},
			}},
		}},
	}},
	{Char: '3', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full single-stroke "3" trajectory (upper bump CCW from
			// upper-left, around top, down to the middle pinch, then
			// CW around the lower bump to the lower-left curl tail).
			// kakitori matches the user's drawing against medians[0] so
			// m_a must be the whole "3" centerline. Clip c1a covers only
			// the upper bump — the lower-bump portion of m_a draws into
			// c1b's territory and is occluded by c1a.
			{Letter: "a", Median: []Point{
				{357, 656}, {416, 703}, {521, 703}, {604, 680}, {663, 644},
				{674, 574}, {674, 492}, {639, 444}, {580, 421}, {474, 421},
				{557, 397}, {639, 362}, {698, 303}, {710, 221},
				{686, 139}, {627, 68}, {521, 44}, {416, 56}, {345, 103},
				{310, 139},
			}},
			// b: off-canvas lead-in keeps b invisible until a finishes
			// drawing the upper bump (~617 units). LeadOut on b balances
			// b's SVG length against a's so the concurrent --d:1s preview
			// animation has b's visible lower-bump pick up where a's
			// visible upper-bump finishes.
			{Letter: "b",
				Median: []Point{
					{474, 1290},
					{474, 421}, {557, 397}, {639, 362}, {698, 303}, {710, 221},
					{686, 139}, {627, 68}, {521, 44}, {416, 56}, {345, 103},
					{310, 139},
				},
				LeadOut: []Point{{-286, 139}},
			},
		}},
	}},
	{Char: '4', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{598, 715}, {416, 409}, {274, 209}, {498, 209}, {757, 221},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{616, 727}, {616, 33},
			}},
		}},
	}},
	{Char: '5', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{380, 692}, {521, 692}, {698, 692},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{368, 715}, {357, 621}, {345, 503}, {333, 456},
				{333, 386}, {368, 397}, {416, 409},
				{486, 433}, {568, 421}, {639, 386},
				{686, 315}, {686, 209}, {639, 127},
				{557, 68}, {451, 56}, {357, 92},
				{286, 139},
			}},
		}},
	}},
	{Char: '6', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full single-stroke "6" trajectory (upper-right tail,
			// down-left through left side, around the bottom, up the
			// right side, closing into the bowl interior at the lower-
			// left junction). kakitori matches the user's drawing
			// against medians[0] so m_a must trace the whole "6"
			// centerline. Clip c1a covers left half + tail — the
			// right-half portion of m_a is occluded.
			{Letter: "a", Median: []Point{
				{580, 715}, {486, 621}, {416, 503}, {357, 386},
				{310, 280}, {286, 197}, {321, 115}, {392, 56},
				{486, 33}, {580, 56}, {663, 103}, {710, 197},
				{710, 280}, {663, 374}, {580, 433}, {486, 444},
				{404, 415}, {368, 386}, {321, 339},
			}},
			// b: off-canvas lead-in keeps b invisible until a's first
			// visible portion (tail through bottom-mid, ~750 units)
			// finishes.
			{Letter: "b", Median: []Point{
				{-326, 56}, {580, 56}, {663, 103}, {710, 197},
				{710, 280}, {663, 374}, {580, 433}, {486, 444},
				{404, 415}, {368, 386}, {321, 339},
			}},
		}},
	}},
	{Char: '7', Phases: []Phase{
		// 1画目: small downward flag at the upper-left
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{305, 697}, {318, 597}, {330, 503},
			}},
		}},
		// 2画目: top horizontal continuing into the diagonal down-left
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{374, 662}, {521, 674}, {686, 674},
				{592, 468}, {498, 268}, {427, 92}, {404, 33},
			}},
		}},
	}},
	{Char: '8', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full figure-8 trajectory (right S — upper-right tab CCW
			// around the upper loop, down the left wall through the
			// waist X-crossing, down the right wall of the lower loop to
			// the bottom-mid; then continuing left across the bottom and
			// up the left wall of the lower loop, back through the waist
			// to the upper-right tab). kakitori matches the user's
			// drawing against medians[0] so m_a must trace the whole
			// figure-8 centerline. Clip c1a covers the right S region —
			// the left S portion of m_a draws into c1b's territory and
			// is occluded by c1a.
			{Letter: "a", Median: []Point{
				{686, 644}, {663, 668}, {627, 703}, {510, 703}, {416, 703}, {380, 668},
				{345, 597}, {333, 527}, {357, 468}, {404, 433}, {486, 409},
				{557, 397}, {639, 362}, {686, 292}, {710, 209}, {686, 127},
				{627, 68}, {486, 33}, {427, 33},
				{357, 68}, {321, 139}, {310, 209}, {310, 280}, {333, 350},
				{392, 386}, {451, 421}, {533, 444}, {592, 480}, {639, 515},
				{686, 539}, {710, 574}, {716, 609},
			}},
			// b: off-canvas lead-in via [-160, -420] -> [200, -420] ->
			// [490, -420] -> [490, 80] keeps b invisible until a's
			// first visible portion (right S, ~1119 units) finishes.
			// After picking up at [490, 80], b traces the LEFT wall of
			// the lower loop UP, through the waist X-crossing up-right,
			// ending at the upper-right tab.
			{Letter: "b", Median: []Point{
				{-312, -556}, {145, -556}, {486, -556}, {486, 33},
				{427, 33}, {357, 68}, {321, 139}, {310, 209}, {310, 280},
				{333, 350}, {392, 386}, {451, 421}, {533, 444}, {592, 480},
				{639, 515}, {686, 539}, {710, 574}, {716, 609},
			}},
		}},
	}},
	{Char: '9', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// a: full single-stroke "9" trajectory (bowl CCW from
			// upper-right around to bowl-bottom, then up the closure to
			// the upper-right corner, then down the descender to the
			// foot). kakitori matches the user's drawing against
			// medians[0] so m_a must trace the whole "9" centerline.
			// Clip c1a covers only the bowl — closure + descender
			// portions of m_a draw into c1b/c1c's territory and are
			// occluded by c1a.
			{Letter: "a", Median: []Point{
				{663, 644}, {580, 715}, {486, 727}, {404, 715}, {345, 668},
				{321, 597}, {310, 503}, {310, 421}, {345, 362}, {380, 327},
				{404, 315}, {427, 315}, {486, 315}, {533, 350}, {568, 386},
				{592, 421}, {639, 456}, {663, 492}, {686, 527}, {710, 574},
				{716, 621}, {716, 668}, {710, 703}, {692, 621}, {674, 539},
				{663, 456}, {645, 386}, {633, 303}, {627, 221}, {621, 139},
				{610, 56}, {598, -3},
			}},
			// b: vertical off-canvas lead-in from [440, -350] keeps b
			// invisible until a's first visible portion (bowl, ~645
			// units) finishes. b then traces the closure up to the
			// upper-right corner [680, 650]. LeadOut balances b's SVG
			// length against a's so the concurrent --d:1s preview
			// animation has b's visible closure pick up where a's
			// visible bowl finishes, and end at c's visible-start.
			{Letter: "b",
				Median: []Point{
					{427, -473},
					{427, 315}, {486, 315}, {533, 350}, {568, 386}, {592, 421},
					{639, 456}, {663, 492}, {686, 527}, {710, 574}, {716, 621},
					{716, 668}, {710, 703},
				},
				LeadOut: []Point{{1516, 703}},
			},
			// c: off-canvas left lead-in via [-150, 50] -> [-150, 650] ->
			// [680, 650] keeps c invisible until b's visible closure
			// finishes. c then traces the descender down to the foot
			// [585, 50]. LeadOut balances c's SVG length so its visible
			// descender picks up where b's visible closure finishes.
			{Letter: "c",
				Median: []Point{
					{-267, -3}, {-267, 703},
					{710, 703}, {692, 621}, {674, 539}, {663, 456}, {645, 386},
					{633, 303}, {627, 221}, {621, 139}, {610, 56}, {598, -3},
				},
				LeadOut: []Point{{598, -317}},
			},
		}},
	}},
	// Full-width digits (U+FF10-FF19) — Klee One has independent glyphs
	// for these (slightly wider than half-width). Bootstrap medians:
	// scaled half-width medians (around x=512). Closed-loop digits
	// (０/３/６/８/９) start as single-path; split into d1a/d1b/[d1c]
	// in the SVG outlines via Affinity Designer when ready, then add
	// matching multi-Part entries here mirroring the half-width design.
	{Char: '０', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{384, 680}, {337, 621}, {310, 527}, {297, 386}, {324, 256},
				{377, 150}, {457, 68}, {510, 39}, {603, 68}, {656, 139},
				{696, 233}, {708, 386}, {696, 527}, {656, 644}, {603, 715},
				{512, 739}, {430, 727}, {384, 680}, {370, 592},
			}},
			{Letter: "b", Median: []Point{
				{-286, 39}, {510, 39}, {603, 68}, {656, 139}, {696, 233},
				{708, 386}, {696, 527}, {656, 644}, {603, 715}, {512, 739},
				{430, 727}, {397, 674}, {370, 592},
			}},
		}},
	}},
	{Char: '１', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{514, 727}, {514, 33},
			}},
		}},
	}},
	{Char: '２', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{316, 621}, {380, 680}, {458, 715}, {548, 715},
				{625, 680}, {677, 621}, {690, 562}, {664, 492},
				{587, 409}, {484, 315}, {380, 209}, {303, 115},
				{265, 56}, {406, 44}, {574, 44}, {754, 56},
			}},
		}},
	}},
	{Char: '３', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{292, 633}, {341, 656}, {406, 703}, {523, 703}, {613, 680}, {678, 644},
				{691, 574}, {691, 492}, {652, 444}, {587, 421}, {471, 421},
				{447, 373}, {406, 401},
				{561, 397}, {652, 362}, {717, 303}, {730, 221}, {704, 139},
				{639, 68}, {523, 44}, {406, 56}, {328, 103}, {290, 139},
			}},
			{Letter: "b",
				Median: []Point{
					{471, 1290}, {471, 421}, {447, 373}, {406, 401},
					{561, 397}, {652, 362}, {717, 303},
					{730, 221}, {704, 139}, {639, 68}, {523, 44}, {406, 56},
					{328, 103}, {290, 139},
				},
				LeadOut: []Point{{-209, 139}},
			},
		}},
	}},
	{Char: '４', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{606, 715}, {406, 409}, {252, 209}, {497, 209}, {780, 221},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{625, 727}, {625, 33},
			}},
		}},
	}},
	{Char: '５', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// Bar centerline at source y=640 (= SVG y=260, mid of bar
			// y range 227-292). Half-width "5" uses the same y. x range
			// adjusted to full-width's slightly wider bar.
			{Letter: "", Median: []Point{
				{398, 692}, {523, 692}, {716, 692},
			}},
		}},
		{Number: 2, Parts: []Part{
			// Body trace: scaled from half-width "5" phase 2 median (17
			// pts) so the bend at the upper-left and the lower-left
			// curl tail are explicitly traced (otherwise they leave gray
			// gaps where the median doesn't reach the centerline).
			{Letter: "", Median: []Point{
				{353, 715}, {340, 621}, {327, 503}, {314, 456},
				{314, 386}, {353, 397}, {405, 409},
				{484, 433}, {574, 421}, {652, 386},
				{704, 315}, {704, 209}, {652, 127},
				{561, 68}, {445, 56}, {340, 92},
				{263, 139},
			}},
		}},
	}},
	{Char: '６', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{587, 715}, {484, 621}, {406, 503}, {341, 386}, {290, 280},
				{264, 197}, {303, 115}, {380, 56}, {484, 33}, {587, 56},
				{678, 103}, {730, 197}, {730, 280}, {678, 374}, {587, 433},
				{484, 444}, {393, 415}, {354, 386}, {303, 339},
			}},
			{Letter: "b", Median: []Point{
				{-174, -120}, {-174, 56}, {587, 56}, {678, 103}, {730, 197},
				{730, 280}, {678, 374}, {587, 433}, {484, 444}, {393, 415},
				{354, 386}, {303, 339},
			}},
		}},
	}},
	{Char: '７', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{283, 697}, {297, 597}, {310, 503},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{360, 662}, {523, 674}, {705, 674},
				{600, 468}, {497, 268}, {418, 92}, {392, 33},
			}},
		}},
	}},
	{Char: '８', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{698, 644}, {678, 668}, {639, 703}, {510, 703}, {405, 703}, {366, 668},
				{327, 597}, {314, 527}, {340, 468}, {392, 433}, {484, 409},
				{561, 397}, {652, 362}, {704, 292}, {730, 209}, {704, 127},
				{639, 68}, {484, 33}, {419, 33}, {340, 68}, {301, 139},
				{288, 209}, {288, 280}, {314, 350}, {379, 386}, {445, 421},
				{536, 444}, {600, 480}, {652, 515}, {704, 539}, {730, 574},
				{737, 609},
			}},
			{Letter: "b", Median: []Point{
				{-132, -297}, {-132, -556}, {107, -556}, {484, -556}, {484, 33},
				{419, 33}, {340, 68}, {301, 139}, {288, 209}, {288, 280},
				{314, 350}, {379, 386}, {445, 421}, {536, 444}, {600, 480},
				{652, 515}, {704, 539}, {730, 574}, {737, 609},
			}},
		}},
	}},
	{Char: '９', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{676, 644}, {586, 715}, {484, 727}, {394, 715}, {331, 668},
				{306, 597}, {293, 503}, {293, 421}, {331, 362}, {370, 327},
				{394, 315}, {420, 315}, {484, 315}, {534, 350}, {573, 386},
				{599, 421}, {650, 456}, {676, 492}, {700, 527}, {726, 574},
				{732, 621}, {732, 668}, {726, 703}, {707, 621}, {687, 539},
				{676, 456}, {656, 386}, {644, 303}, {637, 221}, {631, 139},
				{618, 56}, {605, -3},
			}},
			{Letter: "b",
				Median: []Point{
					{420, -567}, {420, 315}, {484, 315}, {534, 350}, {573, 386},
					{599, 421}, {650, 456}, {676, 492}, {700, 527}, {726, 574},
					{732, 621}, {732, 668}, {726, 703},
				},
				LeadOut: []Point{{1473, 703}},
			},
			{Letter: "c",
				Median: []Point{
					{-332, -3}, {-332, 703}, {726, 703}, {707, 621}, {687, 539},
					{676, 456}, {656, 386}, {644, 303}, {637, 221}, {631, 139},
					{618, 56}, {605, -3},
				},
				LeadOut: []Point{{605, -210}},
			},
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

func findPart(char rune, phase int, part string) (Part, bool) {
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
					return pt, true
				}
			}
		}
	}
	return Part{}, false
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
		part, ok := findPart(char, o.Phase, o.Part)
		if !ok {
			sb.WriteString(fmt.Sprintf("<!-- missing median for %s -->\n", o.ID))
			continue
		}
		// SVG path includes LeadOut so the stroke-dashoffset animation
		// stays inside its clip during the off-canvas tail (sequencing
		// the visible drawing across multi-part stroke groups). LeadOut
		// is omitted from graphicsNumber.txt below.
		full := append(append([]Point(nil), part.Median...), part.LeadOut...)
		medianPath := buildMedianPath(full)
		transformed := transformPath(medianPath)
		sb.WriteString(fmt.Sprintf(`<path style="--d:%ds;" pathLength="3333" clip-path="url(#%s)" d="%s"/>`,
			o.Phase, clipID(o.ID), transformed))
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

// listSVGCodepoints returns the codepoints of every "<int>.svg" file in
// svgsNumber/ (so phase2/phase3/debug iterate over whatever phase1
// emitted, including both half-width 0-9 and full-width ０-９).
func listSVGCodepoints() ([]int, error) {
	entries, err := os.ReadDir(outDir)
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

func main() {
	cps, err := listSVGCodepoints()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing %s: %v\n", outDir, err)
		os.Exit(1)
	}
	for _, cp := range cps {
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
