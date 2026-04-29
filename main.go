package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Point struct{ X, Y float64 }

type StrokeDef struct {
	Outline string
	Median  []Point
}

type DigitDef struct {
	Char    rune
	Strokes []StrokeDef
}

type GraphicsEntry struct {
	Character string     `json:"character"`
	Strokes   []string   `json:"strokes"`
	Medians   [][][2]int `json:"medians"`
}

const (
	outDir       = "svgsNumber"
	graphicsFile = "graphicsNumber.txt"
)

// Glyph outlines extracted from Arphic PL KaitiM GB (Arphic Public License).
// Source: https://ftp.gnu.org/non-gnu/chinese-fonts-truetype/gkai00mp.ttf.gz
// Same font used by animCJK/MakeMeAHanzi for CJK characters.
// unitsPerEm=1024, matching animCJK viewBox. x offset +256 applied for centering.
// Coordinate system: animCJK graphics (y-up, 0-900 range).
var digits = []DigitDef{
	{Char: '0', Strokes: []StrokeDef{
		{Outline: "M357,594Q306,500 306,373Q306,238 364,154Q422,69 509,66Q600,70 664,168Q719,252 719,383Q719,513 667,610Q616,701 517,703L517,704Q516,704 516,703Q515,704 511,703Q415,703 357,594ZM426,610Q457,674 512,675Q563,675 597,608Q643,514 643,383Q643,241 601,156Q571,98 509,94Q454,98 420,154Q383,218 382,373Q382,520 426,610Z",
			Median: []Point{
				// Counterclockwise from ~1 o'clock
				{580, 680}, {512, 700}, {450, 680}, {400, 620},
				{370, 530}, {360, 400}, {370, 270}, {400, 200},
				{450, 140}, {512, 100}, {570, 140}, {620, 200},
				{660, 300}, {670, 400}, {660, 530}, {620, 620}, {580, 680},
			}},
	}},
	{Char: '1', Strokes: []StrokeDef{
		{Outline: "M545,694L534,694L365,651L365,629Q473,629 473,581L473,159Q473,104 365,104L365,76L655,76L655,104Q546,104 545,159Z",
			Median: []Point{
				// Serif from left, then vertical down
				{400, 640}, {470, 670}, {540, 694}, {540, 90},
			}},
	}},
	{Char: '2', Strokes: []StrokeDef{
		{Outline: "M556,338Q722,444 723,531Q723,600 665,654Q611,703 516,703L516,704Q515,704 516,703Q512,704 511,703Q429,703 373,657Q309,606 309,545Q309,520 327,503Q343,488 368,488Q387,488 403,503Q416,518 417,530Q416,557 403,572Q393,585 395,601Q396,623 432,646Q463,668 516,675Q564,675 600,641Q646,596 647,531Q647,455 520,365Q393,271 350,208Q307,156 307,76L674,76L717,194L700,202Q648,124 543,121L360,121Q369,172 401,207Q443,262 556,338Z",
			Median: []Point{
				// Top curve, diagonal down, horizontal right
				{360, 580}, {400, 640}, {470, 680}, {520, 690},
				{580, 670}, {630, 630}, {660, 560}, {640, 470},
				{560, 380}, {470, 310}, {400, 240}, {360, 170},
				{340, 110}, {440, 100}, {560, 100}, {700, 120},
			}},
	}},
	{Char: '3', Strokes: []StrokeDef{
		{Outline: "M716,234Q716,345 616,380Q595,387 576,392Q588,399 604,407Q695,453 696,546Q696,612 642,662Q599,702 516,703L516,704Q515,704 516,703Q512,704 511,703Q438,703 389,660Q331,614 330,552Q330,528 346,511Q362,495 381,495Q403,495 417,511Q430,527 431,552Q428,571 415,588Q405,604 430,636Q460,667 515,675Q554,675 585,645Q619,612 620,546Q620,476 579,436Q546,405 526,400Q508,401 499,406Q484,413 471,413Q443,413 442,386Q442,358 471,357Q481,357 495,365Q507,372 521,372Q567,372 603,341Q640,305 640,234Q640,159 604,129Q561,96 519,94Q464,103 428,126Q388,154 391,171Q392,184 400,194Q407,203 409,215Q409,240 394,255Q379,270 362,270Q340,270 324,255Q308,239 308,215Q308,160 362,115Q422,69 519,66Q598,67 655,108Q716,157 716,234Z",
			Median: []Point{
				// Two bumps, top left to bottom left
				{360, 600}, {420, 660}, {500, 690}, {560, 670},
				{620, 630}, {660, 560}, {630, 470}, {570, 430},
				{520, 400}, {490, 390},
				{540, 380}, {610, 350}, {660, 290}, {660, 230},
				{630, 160}, {570, 110}, {510, 90},
				{440, 100}, {380, 140}, {330, 200},
			}},
	}},
	{Char: '4', Strokes: []StrokeDef{
		// Stroke 1: diagonal + horizontal
		{Outline: "M618,235L618,694L585,694L580,687L580,671L289,242L289,207L546,207L546,150Q546,104 476,104L476,76L689,76L689,104Q619,104 618,150L618,207L735,207L735,235ZM546,235L322,235L546,583Z",
			Median: []Point{
				{600, 690}, {430, 420}, {310, 230}, {500, 220}, {730, 220},
			}},
		// Stroke 2: vertical
		{Outline: "M618,235L618,694L585,694L580,687L580,671L289,242L289,207L546,207L546,150Q546,104 476,104L476,76L689,76L689,104Q619,104 618,150L618,207L735,207L735,235ZM546,235L322,235L546,583Z",
			Median: []Point{
				{600, 694}, {600, 90},
			}},
	}},
	{Char: '5', Strokes: []StrokeDef{
		// Stroke 1: horizontal at top (right to left)
		{Outline: "M686,682L360,682L360,388L371,375Q443,421 504,422Q556,422 593,386Q644,337 644,247Q644,174 598,130Q559,96 515,94Q457,101 425,126Q394,150 393,159Q393,166 399,173Q405,179 406,187Q406,206 391,222Q375,238 355,239Q336,239 320,226Q305,211 305,187Q305,147 358,114Q425,68 515,66Q600,67 656,111Q720,163 720,247Q720,350 653,407Q602,450 530,450L530,451Q530,451 530,450Q529,451 527,450Q526,451 526,451L526,450Q448,451 393,414L393,632L664,632Z",
			Median: []Point{
				{680, 660}, {530, 660}, {400, 660},
			}},
		// Stroke 2: down then curve
		{Outline: "M686,682L360,682L360,388L371,375Q443,421 504,422Q556,422 593,386Q644,337 644,247Q644,174 598,130Q559,96 515,94Q457,101 425,126Q394,150 393,159Q393,166 399,173Q405,179 406,187Q406,206 391,222Q375,238 355,239Q336,239 320,226Q305,211 305,187Q305,147 358,114Q425,68 515,66Q600,67 656,111Q720,163 720,247Q720,350 653,407Q602,450 530,450L530,451Q530,451 530,450Q529,451 527,450Q526,451 526,451L526,450Q448,451 393,414L393,632L664,632Z",
			Median: []Point{
				{400, 660}, {390, 500}, {420, 430},
				{500, 440}, {560, 420}, {620, 370},
				{660, 280}, {640, 180}, {580, 120},
				{510, 85}, {430, 100}, {370, 140},
			}},
	}},
	{Char: '6', Strokes: []StrokeDef{
		{Outline: "M380,603Q307,491 306,334Q306,211 365,141Q426,69 512,66Q600,67 657,118Q718,178 719,261Q719,370 661,420Q612,463 548,464L548,465Q548,465 548,464Q545,465 542,464Q466,464 383,395Q390,531 438,600Q484,673 547,675Q577,672 598,650Q617,631 611,620Q604,610 602,593Q602,574 618,559Q631,546 649,546Q662,546 680,557Q696,572 697,593Q697,638 648,671Q602,702 552,703L552,704Q551,704 551,703Q548,704 546,703Q446,703 380,603ZM382,334Q382,347 382,360Q472,423 523,425Q578,425 611,388Q642,349 643,261Q643,173 607,133Q570,97 512,94Q457,97 419,150Q382,207 382,334Z",
			Median: []Point{
				// From top, curve down into bowl
				{650, 630}, {560, 680}, {500, 680}, {440, 640},
				{400, 560}, {380, 440}, {360, 330}, {380, 220},
				{430, 140}, {500, 90}, {570, 100}, {630, 150},
				{670, 230}, {680, 320}, {650, 400}, {590, 440},
				{520, 450}, {450, 420}, {400, 360}, {380, 280},
			}},
	}},
	{Char: '7', Strokes: []StrokeDef{
		{Outline: "M718,684L362,684L306,490L329,483Q377,586 421,608Q472,633 563,634L668,634Q530,459 477,344Q426,227 426,126Q426,89 436,74Q448,59 469,59Q488,59 501,75Q513,91 513,126Q513,286 554,393Q600,504 718,656Z",
			Median: []Point{
				// Horizontal right, then diagonal down
				{330, 590}, {450, 660}, {570, 660}, {700, 660},
				{580, 470}, {510, 310}, {470, 160}, {470, 70},
			}},
	}},
	{Char: '8', Strokes: []StrokeDef{
		// Upper half: split at waist (~y=400)
		{Outline: "M443,394Q352,351 329,313Q298,267 297,228Q297,161 357,116Q423,68 519,66Q613,67 674,123Q726,169 727,238Q727,278 696,317Q662,362 557,410L443,394ZM632,286Q657,255 658,225Q658,165 615,130Q570,96 519,94Q456,95 411,137Q374,171 373,228Q373,265 397,304Q412,332 486,372Q496,368 510,361Q604,319 632,286Z",
			Median: []Point{
				// Upper loop, counterclockwise from top
				{512, 690}, {440, 670}, {380, 630}, {350, 570},
				{370, 510}, {430, 460}, {512, 420},
				{590, 460}, {640, 510}, {660, 580},
				{640, 640}, {580, 680}, {512, 690},
			}},
		// Lower half
		{Outline: "M557,410Q644,458 668,492Q692,525 692,560Q692,626 626,667Q578,703 512,703L512,704Q511,704 511,703Q507,704 506,703Q430,703 382,662Q325,616 325,548Q325,502 361,458Q386,427 443,394L557,410ZM415,495Q388,528 388,565Q388,610 422,643Q458,674 507,675Q559,675 593,647Q629,614 629,570Q629,530 602,493Q577,460 516,428Q440,465 415,495Z",
			Median: []Point{
				// Lower loop, counterclockwise from center
				{512, 420}, {440, 380}, {380, 320}, {370, 240},
				{390, 160}, {440, 110}, {512, 85},
				{580, 110}, {640, 160}, {670, 240},
				{650, 330}, {590, 380}, {512, 420},
			}},
	}},
	{Char: '9', Strokes: []StrokeDef{
		{Outline: "M649,154Q718,253 718,399Q718,553 663,626Q605,702 509,703L509,704Q508,704 508,703Q504,704 503,703Q418,703 364,648Q301,581 301,495Q301,407 353,348Q407,293 493,292L493,293Q575,294 641,367Q634,234 586,164Q535,97 476,94Q445,100 426,113Q407,125 411,150Q410,184 396,200Q380,216 358,216Q340,215 327,201Q314,186 315,161Q315,128 363,98Q409,68 476,66Q588,67 649,154ZM413,628Q447,674 504,675Q565,675 605,617Q642,563 641,398Q617,373 595,355Q546,322 487,320Q444,323 412,363Q378,406 377,495Q377,579 413,628Z",
			Median: []Point{
				// Circle counterclockwise from ~2 o'clock, then tail down
				{640, 480}, {620, 580}, {560, 650}, {500, 680},
				{430, 660}, {380, 600}, {360, 520},
				{380, 420}, {430, 350}, {500, 310},
				{570, 320}, {630, 380}, {650, 450},
				{650, 350}, {620, 230}, {570, 140},
				{510, 90}, {440, 90},
			}},
	}},
}

// --- Coordinate transform (graphics -> SVG) ---

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

// --- SVG generation ---

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
	fill:none;
	stroke:#000;
}
svg.acjk path[id] {fill:#ccc;}
]]>
</style>
`
}

func buildMedianPath(median [][2]int) string {
	var sb strings.Builder
	for i, p := range median {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("M%d %d", p[0], p[1]))
		} else {
			sb.WriteString(fmt.Sprintf("L%d %d", p[0], p[1]))
		}
	}
	return sb.String()
}

func buildSVG(entry GraphicsEntry) string {
	u := int([]rune(entry.Character)[0])
	id := fmt.Sprintf("z%d", u)
	svgStyle := buildStyle()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg id="%s" class="acjk" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg">`, id))
	sb.WriteString("\n")
	sb.WriteString(svgStyle)

	for i, stroke := range entry.Strokes {
		transformed := transformPath(stroke)
		sb.WriteString(fmt.Sprintf(`<path id="%sd%d" d="%s"/>`, id, i+1, transformed))
		sb.WriteString("\n")
	}

	sb.WriteString("<defs>\n")
	for i := range entry.Strokes {
		sb.WriteString(fmt.Sprintf("\t"+`<clipPath id="%sc%d"><use href="#%sd%d"/></clipPath>`, id, i+1, id, i+1))
		sb.WriteString("\n")
	}
	sb.WriteString("</defs>\n")

	for i, median := range entry.Medians {
		medianPath := buildMedianPath(median)
		transformedMedian := transformPath(medianPath)
		sb.WriteString(fmt.Sprintf(`<path style="--d:%ds;" pathLength="3333" clip-path="url(#%sc%d)" d="%s"/>`, i+1, id, i+1, transformedMedian))
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}

func svgComment() string {
	return `<!--
animNumber Copyright 2026- k1LoW, https://github.com/k1LoW/animNumber
Glyph outlines derived from Arphic PL KaitiM GB font.
You can redistribute and/or modify this file under the terms of the Arphic Public License
as published by Arphic Technology Co., Ltd.
You should have received a copy of this license along with this file.
If not, see https://ftp.gnu.org/non-gnu/chinese-fonts-truetype/LICENSE.
-->
`
}

func main() {
	entries := make([]GraphicsEntry, 0, len(digits))

	for _, d := range digits {
		entry := GraphicsEntry{
			Character: string(d.Char),
		}
		for _, s := range d.Strokes {
			entry.Strokes = append(entry.Strokes, s.Outline)

			intMedian := make([][2]int, len(s.Median))
			for i, p := range s.Median {
				intMedian[i] = [2]int{int(math.Round(p.X)), int(math.Round(p.Y))}
			}
			entry.Medians = append(entry.Medians, intMedian)
		}
		entries = append(entries, entry)
	}

	f, err := os.Create(graphicsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", graphicsFile, err)
		os.Exit(1)
	}
	for _, entry := range entries {
		line, _ := json.Marshal(entry)
		fmt.Fprintln(f, string(line))
	}
	f.Close()
	fmt.Printf("Written %s\n", graphicsFile)

	os.MkdirAll(outDir, 0o755)
	comment := svgComment()
	for _, entry := range entries {
		u := int([]rune(entry.Character)[0])
		svgContent := comment + buildSVG(entry)
		svgPath := filepath.Join(outDir, fmt.Sprintf("%d.svg", u))
		if err := os.WriteFile(svgPath, []byte(svgContent), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", svgPath, err)
			os.Exit(1)
		}
		fmt.Printf("Written %s (%s)\n", svgPath, entry.Character)
	}

	fmt.Println("Done!")
}
