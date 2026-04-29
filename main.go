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
	Median    []Point
	HalfWidth float64
	Closed    bool
}

type DigitDef struct {
	Char    rune
	Strokes []StrokeDef
}

type GraphicsEntry struct {
	Character string       `json:"character"`
	Strokes   []string     `json:"strokes"`
	Medians   [][][2]int   `json:"medians"`
}

const (
	defaultHW    = 70.0
	kBezier      = 0.5522847498
	outDir       = "svgsNumber"
	graphicsFile = "graphicsNumber.txt"
)

var digits = []DigitDef{
	{Char: '0', Strokes: []StrokeDef{{Closed: true, HalfWidth: defaultHW, Median: []Point{
		{500, 780}, {560, 770}, {620, 740}, {660, 700}, {690, 640},
		{710, 570}, {710, 490}, {700, 410}, {680, 340}, {640, 280},
		{600, 240}, {550, 215}, {500, 210}, {450, 215}, {400, 240},
		{360, 280}, {320, 340}, {300, 410}, {290, 490}, {290, 570},
		{310, 640}, {340, 700}, {380, 740}, {440, 770}, {500, 780},
	}}}},
	{Char: '1', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{420, 700}, {460, 735}, {500, 770}, {500, 180},
	}}}},
	{Char: '2', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{310, 650}, {340, 710}, {400, 755}, {480, 770}, {560, 755},
		{620, 720}, {655, 670}, {660, 610}, {640, 555}, {600, 500},
		{540, 440}, {470, 375}, {400, 305}, {355, 245}, {335, 200},
		{390, 190}, {480, 185}, {580, 185}, {680, 190},
	}}}},
	{Char: '3', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{310, 720}, {370, 760}, {450, 775}, {540, 765}, {610, 730},
		{650, 680}, {660, 625}, {640, 575}, {600, 535}, {540, 505},
		{490, 495},
		{540, 475}, {600, 440}, {645, 390}, {660, 335}, {655, 275},
		{625, 225}, {575, 195}, {510, 180}, {440, 185}, {370, 205},
	}}}},
	{Char: '4', Strokes: []StrokeDef{
		{HalfWidth: defaultHW, Median: []Point{
			{400, 770}, {370, 640}, {340, 510}, {320, 440}, {420, 440},
			{540, 440}, {680, 440},
		}},
		{HalfWidth: defaultHW, Median: []Point{
			{560, 770}, {560, 180},
		}},
	}},
	{Char: '5', Strokes: []StrokeDef{
		{HalfWidth: defaultHW, Median: []Point{
			{640, 770}, {530, 770}, {420, 770}, {350, 770},
		}},
		{HalfWidth: defaultHW, Median: []Point{
			{360, 770}, {345, 650}, {340, 560}, {355, 485}, {400, 425},
			{465, 395}, {540, 385}, {615, 400}, {660, 445},
			{675, 370}, {660, 285}, {615, 225}, {555, 195},
			{480, 180}, {405, 195}, {350, 230},
		}},
	}},
	{Char: '6', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{620, 770}, {560, 700}, {490, 610}, {430, 510}, {385, 410},
		{365, 310}, {375, 230}, {415, 185}, {475, 170}, {545, 180},
		{610, 220}, {650, 285}, {660, 365}, {635, 440}, {585, 485},
		{520, 500}, {455, 485}, {405, 435}, {380, 370},
	}}}},
	{Char: '7', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{300, 770}, {400, 770}, {500, 770}, {600, 770}, {690, 770},
		{640, 620}, {580, 470}, {520, 330}, {460, 200},
	}}}},
	{Char: '8', Strokes: []StrokeDef{
		{Closed: true, HalfWidth: 65.0, Median: []Point{
			{500, 765}, {560, 755}, {615, 730}, {650, 695}, {665, 650},
			{660, 605}, {635, 565}, {595, 535}, {545, 515}, {500, 510},
			{455, 515}, {405, 535}, {365, 565}, {340, 605}, {335, 650},
			{350, 695}, {385, 730}, {440, 755}, {500, 765},
		}},
		{Closed: true, HalfWidth: 65.0, Median: []Point{
			{500, 500}, {565, 490}, {625, 460}, {665, 415}, {680, 365},
			{675, 305}, {650, 255}, {610, 220}, {560, 200}, {500, 195},
			{440, 200}, {390, 220}, {350, 255}, {325, 305}, {320, 365},
			{335, 415}, {375, 460}, {435, 490}, {500, 500},
		}},
	}},
	{Char: '9', Strokes: []StrokeDef{{HalfWidth: defaultHW, Median: []Point{
		{570, 530}, {610, 580}, {630, 645}, {625, 710}, {595, 755},
		{545, 775}, {485, 775}, {425, 755}, {390, 715}, {380, 660},
		{395, 600}, {430, 555}, {480, 530}, {540, 525}, {580, 535},
		{570, 465}, {550, 385}, {520, 300}, {490, 225}, {460, 175},
	}}}},
}

// --- Vector math ---

func sub(a, b Point) Point   { return Point{a.X - b.X, a.Y - b.Y} }
func add(a, b Point) Point   { return Point{a.X + b.X, a.Y + b.Y} }
func scale(p Point, s float64) Point { return Point{p.X * s, p.Y * s} }

func length(p Point) float64 {
	return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

func normalize(p Point) Point {
	l := length(p)
	if l < 1e-9 {
		return Point{0, 0}
	}
	return Point{p.X / l, p.Y / l}
}

func perpLeft(p Point) Point { return Point{-p.Y, p.X} }

// --- Normal computation ---

func computeNormals(points []Point, closed bool) []Point {
	n := len(points)
	normals := make([]Point, n)
	for i := 0; i < n; i++ {
		var tangent Point
		if closed {
			prev := (i - 1 + n) % n
			next := (i + 1) % n
			t1 := normalize(sub(points[i], points[prev]))
			t2 := normalize(sub(points[next], points[i]))
			tangent = normalize(add(t1, t2))
		} else if i == 0 {
			tangent = normalize(sub(points[1], points[0]))
		} else if i == n-1 {
			tangent = normalize(sub(points[n-1], points[n-2]))
		} else {
			t1 := normalize(sub(points[i], points[i-1]))
			t2 := normalize(sub(points[i+1], points[i]))
			tangent = normalize(add(t1, t2))
		}
		normals[i] = perpLeft(tangent)
	}
	return normals
}

// --- Outline generation ---

func fmtCoord(x, y float64) string {
	return fmt.Sprintf("%d,%d", int(math.Round(x)), int(math.Round(y)))
}

func expandToOutline(s StrokeDef) string {
	points := s.Median
	hw := s.HalfWidth
	n := len(points)
	if n < 2 {
		return ""
	}

	normals := computeNormals(points, s.Closed)

	left := make([]Point, n)
	right := make([]Point, n)
	for i := range points {
		left[i] = add(points[i], scale(normals[i], hw))
		right[i] = add(points[i], scale(normals[i], -hw))
	}

	if s.Closed {
		return buildClosedOutline(left, right, n)
	}
	return buildOpenOutline(left, right, points, normals, hw, n)
}

func buildClosedOutline(left, right []Point, n int) string {
	var sb strings.Builder

	// Outer ring (same direction as median)
	// Skip last point if it duplicates the first (closed loop)
	count := n
	if length(sub(left[0], left[n-1])) < 5 {
		count = n - 1
	}
	sb.WriteString("M" + fmtCoord(left[0].X, left[0].Y))
	for i := 1; i < count; i++ {
		sb.WriteString("L" + fmtCoord(left[i].X, left[i].Y))
	}
	sb.WriteString("Z")

	// Inner ring (reverse direction for hole)
	sb.WriteString("M" + fmtCoord(right[0].X, right[0].Y))
	for i := count - 1; i >= 1; i-- {
		sb.WriteString("L" + fmtCoord(right[i].X, right[i].Y))
	}
	sb.WriteString("Z")

	return sb.String()
}

func buildOpenOutline(left, right []Point, median []Point, normals []Point, hw float64, n int) string {
	var sb strings.Builder

	// Tangent directions at start and end
	tangentStart := normalize(sub(median[1], median[0]))
	tangentEnd := normalize(sub(median[n-1], median[n-2]))
	normalStart := normals[0]
	normalEnd := normals[n-1]

	// Start at left[0]
	sb.WriteString("M" + fmtCoord(left[0].X, left[0].Y))

	// Forward along left side
	for i := 1; i < n; i++ {
		sb.WriteString("L" + fmtCoord(left[i].X, left[i].Y))
	}

	// End cap: semicircle from left[n-1] to right[n-1]
	fwd := add(median[n-1], scale(tangentEnd, hw))
	cp1 := add(left[n-1], scale(tangentEnd, kBezier*hw))
	cp2 := add(fwd, scale(normalEnd, kBezier*hw))
	sb.WriteString("C" + fmtCoord(cp1.X, cp1.Y) + " " + fmtCoord(cp2.X, cp2.Y) + " " + fmtCoord(fwd.X, fwd.Y))
	cp1 = add(fwd, scale(normalEnd, -kBezier*hw))
	cp2 = add(right[n-1], scale(tangentEnd, kBezier*hw))
	sb.WriteString("C" + fmtCoord(cp1.X, cp1.Y) + " " + fmtCoord(cp2.X, cp2.Y) + " " + fmtCoord(right[n-1].X, right[n-1].Y))

	// Backward along right side
	for i := n - 2; i >= 0; i-- {
		sb.WriteString("L" + fmtCoord(right[i].X, right[i].Y))
	}

	// Start cap: semicircle from right[0] to left[0]
	backDir := scale(tangentStart, -1)
	back := add(median[0], scale(backDir, hw))
	cp1 = add(right[0], scale(backDir, kBezier*hw))
	cp2 = add(back, scale(normalStart, -kBezier*hw))
	sb.WriteString("C" + fmtCoord(cp1.X, cp1.Y) + " " + fmtCoord(cp2.X, cp2.Y) + " " + fmtCoord(back.X, back.Y))
	cp1 = add(back, scale(normalStart, kBezier*hw))
	cp2 = add(left[0], scale(backDir, kBezier*hw))
	sb.WriteString("C" + fmtCoord(cp1.X, cp1.Y) + " " + fmtCoord(cp2.X, cp2.Y) + " " + fmtCoord(left[0].X, left[0].Y))

	sb.WriteString("Z")
	return sb.String()
}

// --- Coordinate transform (graphics -> SVG) ---

var reTransform = regexp.MustCompile(`([MQCLZ ]+)([0-9.-]+) ([0-9.-]+)`)

func transformPath(path string) string {
	// Normalize: commas to spaces
	path = strings.ReplaceAll(path, ",", " ")
	// Remove spaces around commands
	re1 := regexp.MustCompile(`\s?([MQCLZ])\s?`)
	path = re1.ReplaceAllString(path, "$1")
	// Add space before negative numbers
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

	// Re-add Z if it was lost
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

	// Stroke shapes
	for i, stroke := range entry.Strokes {
		transformed := transformPath(stroke)
		sb.WriteString(fmt.Sprintf(`<path id="%sd%d" d="%s"/>`, id, i+1, transformed))
		sb.WriteString("\n")
	}

	// Clip paths
	sb.WriteString("<defs>\n")
	for i := range entry.Strokes {
		sb.WriteString(fmt.Sprintf("\t"+`<clipPath id="%sc%d"><use href="#%sd%d"/></clipPath>`, id, i+1, id, i+1))
		sb.WriteString("\n")
	}
	sb.WriteString("</defs>\n")

	// Median paths
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
You can redistribute and/or modify these files under the terms of the GNU
Lesser General Public License as published by the Free Software Foundation,
either version 3 of the license, or (at your option) any later version. You
should have received a copy of this license (the file "licenses/LGPL.txt") along with
these files; if not, see https://www.gnu.org/licenses/.
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
			outline := expandToOutline(s)
			entry.Strokes = append(entry.Strokes, outline)

			median := make([][2]int, len(s.Median))
			for i, p := range s.Median {
				median[i] = [2]int{int(math.Round(p.X)), int(math.Round(p.Y))}
			}
			entry.Medians = append(entry.Medians, median)
		}
		entries = append(entries, entry)
	}

	// Write graphicsNumber.txt
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

	// Write SVG files
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
