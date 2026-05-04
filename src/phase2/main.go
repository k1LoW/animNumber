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
				{415, 630}, {380, 580}, {360, 500}, {350, 380},
				{370, 270}, {410, 180}, {470, 110}, {510, 85},
				{580, 110}, {620, 170}, {650, 250}, {660, 380},
				{650, 500}, {620, 600}, {580, 660}, {512, 680},
				{450, 670}, {415, 630}, {405, 555},
			}},
			// b: off-canvas lead-in delays b's visible drawing until
			// after a's first visible portion (left arc, ~614 units)
			// finishes. The final two points {425, 625}, {405, 555} trace
			// into the upper-left wedge of c1b so the stroke covers that
			// thin closure region.
			{Letter: "b", Median: []Point{
				{-150, 85}, {510, 85}, {580, 110}, {620, 170},
				{650, 250}, {660, 380}, {650, 500}, {620, 600},
				{580, 660}, {512, 680}, {450, 670}, {425, 625},
				{405, 555},
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
			// a: full single-stroke "3" trajectory (upper bump CCW from
			// upper-left, around top, down to the middle pinch, then
			// CW around the lower bump to the lower-left curl tail).
			// kakitori matches the user's drawing against medians[0] so
			// m_a must be the whole "3" centerline. Clip c1a covers only
			// the upper bump — the lower-bump portion of m_a draws into
			// c1b's territory and is occluded by c1a.
			{Letter: "a", Median: []Point{
				{380, 610}, {430, 650}, {520, 650}, {590, 630}, {640, 600},
				{650, 540}, {650, 470}, {620, 430}, {570, 410}, {480, 410},
				{550, 390}, {620, 360}, {670, 310}, {680, 240},
				{660, 170}, {610, 110}, {520, 90}, {430, 100}, {370, 140},
				{340, 170},
			}},
			// b: off-canvas lead-in keeps b invisible until a finishes
			// drawing the upper bump (~617 units). LeadOut on b balances
			// b's SVG length against a's so the concurrent --d:1s preview
			// animation has b's visible lower-bump pick up where a's
			// visible upper-bump finishes.
			{Letter: "b",
				Median: []Point{
					{480, 1149},
					{480, 410}, {550, 390}, {620, 360}, {670, 310}, {680, 240},
					{660, 170}, {610, 110}, {520, 90}, {430, 100}, {370, 140},
					{340, 170},
				},
				LeadOut: []Point{{-166, 170}},
			},
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
			// a: full single-stroke "6" trajectory (upper-right tail,
			// down-left through left side, around the bottom, up the
			// right side, closing into the bowl interior at the lower-
			// left junction). kakitori matches the user's drawing
			// against medians[0] so m_a must trace the whole "6"
			// centerline. Clip c1a covers left half + tail — the
			// right-half portion of m_a is occluded.
			{Letter: "a", Median: []Point{
				{570, 660}, {490, 580}, {430, 480}, {380, 380},
				{340, 290}, {320, 220}, {350, 150}, {410, 100},
				{490, 80}, {570, 100}, {640, 140}, {680, 220},
				{680, 290}, {640, 370}, {570, 420}, {490, 430},
				{420, 405}, {390, 380}, {350, 340},
			}},
			// b: off-canvas lead-in keeps b invisible until a's first
			// visible portion (tail through bottom-mid, ~750 units)
			// finishes.
			{Letter: "b", Median: []Point{
				{-200, 100}, {570, 100}, {640, 140}, {680, 220},
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
				{640, 620}, {610, 650}, {510, 650}, {430, 650}, {400, 620},
				{370, 560}, {360, 500}, {380, 450}, {420, 420}, {490, 400},
				{550, 390}, {620, 360}, {660, 300}, {680, 230}, {660, 160},
				{610, 110}, {490, 80}, {440, 80},
				{380, 110}, {350, 170}, {340, 230}, {340, 290}, {360, 350},
				{410, 380}, {460, 410}, {530, 430}, {580, 460}, {620, 490},
				{660, 510}, {680, 540}, {685, 570},
			}},
			// b: off-canvas lead-in via [-160, -420] -> [200, -420] ->
			// [490, -420] -> [490, 80] keeps b invisible until a's
			// first visible portion (right S, ~1119 units) finishes.
			// After picking up at [490, 80], b traces the LEFT wall of
			// the lower loop UP, through the waist X-crossing up-right,
			// ending at the upper-right tab.
			{Letter: "b", Median: []Point{
				{-160, -420}, {200, -420}, {490, -420}, {490, 80},
				{440, 80}, {380, 110}, {350, 170}, {340, 230}, {340, 290},
				{360, 350}, {410, 380}, {460, 410}, {530, 430}, {580, 460},
				{620, 490}, {660, 510}, {680, 540}, {685, 570},
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
				{640, 600}, {570, 660}, {490, 670}, {420, 660}, {370, 620},
				{350, 560}, {340, 480}, {340, 410}, {370, 360}, {400, 330},
				{420, 320}, {440, 320}, {490, 320}, {530, 350}, {560, 380},
				{580, 410}, {620, 440}, {640, 470}, {660, 500}, {680, 540},
				{685, 580}, {685, 620}, {680, 650}, {665, 580}, {650, 510},
				{640, 440}, {625, 380}, {615, 310}, {610, 240}, {605, 170},
				{595, 100}, {585, 50},
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
					{440, -350},
					{440, 320}, {490, 320}, {530, 350}, {560, 380}, {580, 410},
					{620, 440}, {640, 470}, {660, 500}, {680, 540}, {685, 580},
					{685, 620}, {680, 650},
				},
				LeadOut: []Point{{1365, 650}},
			},
			// c: off-canvas left lead-in via [-150, 50] -> [-150, 650] ->
			// [680, 650] keeps c invisible until b's visible closure
			// finishes. c then traces the descender down to the foot
			// [585, 50]. LeadOut balances c's SVG length so its visible
			// descender picks up where b's visible closure finishes.
			{Letter: "c",
				Median: []Point{
					{-150, 50}, {-150, 650},
					{680, 650}, {665, 580}, {650, 510}, {640, 440}, {625, 380},
					{615, 310}, {610, 240}, {605, 170}, {595, 100}, {585, 50},
				},
				LeadOut: []Point{{585, -217}},
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
				{403, 630}, {363, 580}, {340, 500}, {329, 380}, {352, 270},
				{397, 180}, {465, 110}, {510, 85}, {589, 110}, {634, 170},
				{668, 250}, {679, 380}, {668, 500}, {634, 600}, {589, 660},
				{512, 680}, {442, 670}, {403, 630}, {391, 555},
			}},
			{Letter: "b", Median: []Point{
				{-166, 85}, {510, 85}, {589, 110}, {634, 170}, {668, 250},
				{679, 380}, {668, 500}, {634, 600}, {589, 660}, {512, 680},
				{442, 670}, {414, 625}, {391, 555},
			}},
		}},
	}},
	{Char: '１', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{433, 580}, {499, 630}, {575, 670}, {575, 80},
			}},
		}},
	}},
	{Char: '２', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{345, 580}, {400, 630}, {466, 660}, {543, 660},
				{608, 630}, {652, 580}, {663, 530}, {641, 470},
				{576, 400}, {488, 320}, {400, 230}, {334, 150},
				{302, 100}, {422, 90}, {565, 90}, {718, 100},
			}},
		}},
	}},
	{Char: '３', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{325, 590}, {367, 610}, {422, 650}, {521, 650}, {598, 630}, {653, 600},
				{664, 540}, {664, 470}, {631, 430}, {576, 410}, {477, 410},
				{457, 369}, {422, 393},
				{554, 390}, {631, 360}, {686, 310}, {697, 240}, {675, 170},
				{620, 110}, {521, 90}, {422, 100}, {356, 140}, {323, 170},
			}},
			{Letter: "b",
				Median: []Point{
					{477, 1149}, {477, 410}, {457, 369}, {422, 393},
					{554, 390}, {631, 360}, {686, 310},
					{697, 240}, {675, 170}, {620, 110}, {521, 90}, {422, 100},
					{356, 140}, {323, 170},
				},
				LeadOut: []Point{{-101, 170}},
			},
		}},
	}},
	{Char: '４', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{592, 660}, {422, 400}, {291, 230}, {499, 230}, {740, 240},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{608, 670}, {608, 80},
			}},
		}},
	}},
	{Char: '５', Phases: []Phase{
		{Number: 1, Parts: []Part{
			// Bar centerline at source y=640 (= SVG y=260, mid of bar
			// y range 227-292). Half-width "5" uses the same y. x range
			// adjusted to full-width's slightly wider bar.
			{Letter: "", Median: []Point{
				{415, 640}, {521, 640}, {685, 640},
			}},
		}},
		{Number: 2, Parts: []Part{
			// Body trace: scaled from half-width "5" phase 2 median (17
			// pts) so the bend at the upper-left and the lower-left
			// curl tail are explicitly traced (otherwise they leave gray
			// gaps where the median doesn't reach the centerline).
			{Letter: "", Median: []Point{
				{377, 660}, {366, 580}, {355, 480}, {344, 440},
				{344, 380}, {377, 390}, {421, 400},
				{488, 420}, {565, 410}, {631, 380},
				{675, 320}, {675, 230}, {631, 160},
				{554, 110}, {455, 100}, {366, 130},
				{300, 170},
			}},
		}},
	}},
	{Char: '６', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{576, 660}, {488, 580}, {422, 480}, {367, 380}, {323, 290},
				{301, 220}, {334, 150}, {400, 100}, {488, 80}, {576, 100},
				{653, 140}, {697, 220}, {697, 290}, {653, 370}, {576, 420},
				{488, 430}, {411, 405}, {378, 380}, {334, 340},
			}},
			{Letter: "b", Median: []Point{
				{-71, -50}, {-71, 100}, {576, 100}, {653, 140}, {697, 220},
				{697, 290}, {653, 370}, {576, 420}, {488, 430}, {411, 405},
				{378, 380}, {334, 340},
			}},
		}},
	}},
	{Char: '７', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "", Median: []Point{
				{317, 645}, {329, 560}, {340, 480},
			}},
		}},
		{Number: 2, Parts: []Part{
			{Letter: "", Median: []Point{
				{383, 615}, {521, 625}, {676, 625},
				{587, 450}, {499, 280}, {432, 130}, {410, 80},
			}},
		}},
	}},
	{Char: '８', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{653, 620}, {620, 650}, {510, 650}, {421, 650}, {388, 620},
				{355, 560}, {344, 500}, {366, 450}, {410, 420}, {488, 400},
				{554, 390}, {631, 360}, {675, 300}, {697, 230}, {675, 160},
				{620, 110}, {488, 80}, {433, 80}, {366, 110}, {333, 170},
				{322, 230}, {322, 290}, {344, 350}, {399, 380}, {455, 410},
				{532, 430}, {587, 460}, {631, 490}, {675, 510}, {697, 540},
				{703, 570},
			}},
			{Letter: "b", Median: []Point{
				{-9, -200}, {-9, -420}, {168, -420}, {488, -420}, {488, 80},
				{433, 80}, {366, 110}, {333, 170}, {322, 230}, {322, 290},
				{344, 350}, {399, 380}, {455, 410}, {532, 430}, {587, 460},
				{631, 490}, {675, 510}, {697, 540}, {703, 570},
			}},
		}},
	}},
	{Char: '９', Phases: []Phase{
		{Number: 1, Parts: []Part{
			{Letter: "a", Median: []Point{
				{651, 600}, {575, 660}, {488, 670}, {412, 660}, {358, 620},
				{337, 560}, {326, 480}, {326, 410}, {358, 360}, {391, 330},
				{412, 320}, {434, 320}, {488, 320}, {531, 350}, {564, 380},
				{586, 410}, {629, 440}, {651, 470}, {672, 500}, {694, 540},
				{699, 580}, {699, 620}, {694, 650}, {678, 580}, {661, 510},
				{651, 440}, {634, 380}, {624, 310}, {618, 240}, {613, 170},
				{602, 100}, {591, 50},
			}},
			{Letter: "b",
				Median: []Point{
					{434, -430}, {434, 320}, {488, 320}, {531, 350}, {564, 380},
					{586, 410}, {629, 440}, {651, 470}, {672, 500}, {694, 540},
					{699, 580}, {699, 620}, {694, 650},
				},
				LeadOut: []Point{{1329, 650}},
			},
			{Letter: "c",
				Median: []Point{
					{-205, 50}, {-205, 650}, {694, 650}, {678, 580}, {661, 510},
					{651, 440}, {634, 380}, {624, 310}, {618, 240}, {613, 170},
					{602, 100}, {591, 50},
				},
				LeadOut: []Point{{591, -126}},
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
