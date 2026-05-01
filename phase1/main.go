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

// Glyph outlines extracted from Klee One Regular (SIL Open Font License 1.1).
// Source: https://github.com/google/fonts/tree/main/ofl/kleeone
// unitsPerEm=1000; scaled by 0.85, baseline placed at animCJK y=76.
// Coordinate system: animCJK graphics (y-up, 0-900 range).
var digits = []DigitDef{
	{Char: '0', Strokes: []StrokeDef{
		{Outline: "M421 623Q419 628 414 631Q410 634 404 634Q394 634 380 612Q366 590 353 552Q339 515 330 466Q320 417 320 362Q320 301 331 247Q341 192 363 151Q386 110 422 86Q459 62 512 62Q560 62 596 88Q632 113 656 158Q680 202 692 258Q704 315 704 377Q704 464 684 533Q664 601 624 641Q584 680 523 680Q494 680 468 665Q443 649 421 623ZM509 107Q468 107 441 129Q415 150 399 186Q384 222 378 269Q371 315 371 364Q371 430 386 480Q401 531 424 565Q448 600 473 617Q499 635 519 635Q556 635 581 614Q607 593 622 556Q638 520 645 475Q653 430 653 382Q653 307 637 245Q621 182 589 145Q557 107 509 107Z",
			Median: []Point{
				// Counterclockwise from ~1 o'clock
				{600, 660}, {512, 680}, {440, 660}, {390, 600},
				{360, 510}, {350, 380}, {360, 250}, {390, 170},
				{440, 110}, {512, 80}, {580, 110}, {620, 170},
				{650, 260}, {660, 380}, {650, 510}, {620, 600}, {600, 660},
			}},
	}},
	{Char: '1', Strokes: []StrokeDef{
		{Outline: "M547 600L547 165Q547 157 546 141Q546 126 546 112Q545 97 545 90Q545 79 559 73Q573 67 585 67Q599 67 599 75Q599 87 598 105Q598 123 597 141Q597 159 597 170L597 586Q597 605 600 620Q602 636 602 646Q602 658 591 668Q580 678 571 678Q565 678 557 671Q550 665 544 658Q538 651 534 648Q515 634 488 618Q461 602 431 590Q422 587 422 582Q422 575 434 569Q446 563 458 563Q462 563 464 564Q484 568 506 578Q528 589 547 600Z",
			Median: []Point{
				// Flag from upper-left, then vertical stem down
				{440, 580}, {500, 630}, {570, 670}, {570, 80},
			}},
	}},
	{Char: '2', Strokes: []StrokeDef{
		{Outline: "M342 74L346 74Q374 79 397 81Q420 83 444 83Q468 84 501 84Q536 84 571 84Q606 84 632 82Q655 81 675 79Q695 77 704 77Q716 77 716 90Q716 102 707 115Q698 128 688 128Q679 128 662 127Q644 127 632 127Q606 127 573 127Q540 128 508 128Q492 128 466 127Q440 127 415 126Q389 125 372 124Q402 173 451 226Q501 280 565 334Q628 386 665 436Q702 487 702 541Q702 603 658 642Q614 680 539 680Q488 680 451 665Q414 650 390 628Q366 606 355 587Q343 568 343 560Q343 547 354 536Q365 525 373 525Q381 525 386 531Q391 538 393 545Q408 583 447 609Q485 634 537 634Q591 634 620 607Q649 580 649 541Q649 499 617 456Q586 413 529 367Q460 310 407 251Q355 192 331 150Q327 144 322 138Q317 133 311 124Q308 119 308 113Q308 100 318 87Q328 74 342 74Z",
			Median: []Point{
				// Top curve, diagonal down, horizontal right
				{360, 580}, {410, 630}, {470, 660}, {540, 660},
				{600, 630}, {640, 580}, {650, 530}, {630, 470},
				{570, 400}, {490, 320}, {410, 230}, {350, 150},
				{320, 100}, {430, 90}, {560, 90}, {700, 100},
			}},
	}},
	{Char: '3', Strokes: []StrokeDef{
		{Outline: "M564 408L554 408Q611 422 648 459Q686 496 686 550Q686 612 642 646Q599 680 527 680Q487 680 453 669Q419 658 394 643Q369 627 355 611Q341 595 341 586Q341 585 344 577Q347 568 353 560Q358 553 367 553Q373 553 378 558Q383 564 385 566Q418 603 453 620Q488 636 529 636Q579 636 608 612Q637 587 637 544Q637 513 612 486Q587 458 542 439Q497 420 438 414Q429 413 429 402Q429 393 437 382Q445 370 456 370Q459 370 461 371Q463 371 481 374Q499 378 527 378Q586 378 625 345Q663 313 663 260Q663 221 645 186Q626 152 588 130Q549 107 490 107Q440 107 406 127Q373 147 345 181Q341 186 335 186Q326 186 318 174Q311 161 311 147Q311 136 316 130Q351 97 393 80Q435 62 490 62Q567 62 617 90Q666 117 690 162Q713 208 713 261Q713 306 693 339Q672 372 638 390Q604 408 564 408Z",
			Median: []Point{
				// Two bumps, top to bottom (S curve)
				{360, 600}, {420, 650}, {500, 670}, {560, 660},
				{620, 620}, {650, 560}, {640, 480}, {580, 420},
				{510, 400}, {480, 395},
				{540, 390}, {610, 360}, {650, 300}, {650, 230},
				{620, 150}, {560, 110}, {490, 90},
				{420, 100}, {370, 140}, {330, 180},
			}},
	}},
	{Char: '4', Strokes: []StrokeDef{
		// Stroke 1: diagonal + horizontal (the "4" frame)
		{Outline: "M622 214L643 215Q668 215 691 214Q714 213 722 213Q732 213 732 224Q732 236 724 249Q716 262 705 262Q696 262 678 260Q659 258 642 257L622 256L622 431Q622 442 622 457Q623 472 623 486Q624 500 624 507Q624 518 611 524Q597 531 586 531Q571 531 571 522Q571 513 572 494Q573 476 573 458Q574 440 574 431L574 254L350 244Q386 296 425 357Q464 419 501 477Q537 536 564 580L595 628Q597 632 597 637Q597 651 584 665Q571 680 562 680Q557 680 554 674Q548 659 540 642Q533 625 519 599Q505 573 481 534Q457 494 420 435Q382 375 328 289Q322 281 314 271Q306 261 298 250Q292 242 292 233Q292 219 305 207Q317 195 330 195Q333 195 335 196Q344 198 353 200Q362 201 367 201L574 211L574 165Q574 159 573 144Q573 129 572 113Q572 97 572 90Q572 79 586 73Q599 67 611 67Q625 67 625 75Q625 90 624 113Q623 137 622 164Z",
			Median: []Point{
				// Diagonal from upper-right of frame down-left to bottom, then horizontal right
				{585, 660}, {430, 400}, {310, 230}, {500, 230}, {720, 240},
			}},
		// Stroke 2: vertical stem on the right
		{Outline: "M622 214L643 215Q668 215 691 214Q714 213 722 213Q732 213 732 224Q732 236 724 249Q716 262 705 262Q696 262 678 260Q659 258 642 257L622 256L622 431Q622 442 622 457Q623 472 623 486Q624 500 624 507Q624 518 611 524Q597 531 586 531Q571 531 571 522Q571 513 572 494Q573 476 573 458Q574 440 574 431L574 254L350 244Q386 296 425 357Q464 419 501 477Q537 536 564 580L595 628Q597 632 597 637Q597 651 584 665Q571 680 562 680Q557 680 554 674Q548 659 540 642Q533 625 519 599Q505 573 481 534Q457 494 420 435Q382 375 328 289Q322 281 314 271Q306 261 298 250Q292 242 292 233Q292 219 305 207Q317 195 330 195Q333 195 335 196Q344 198 353 200Q362 201 367 201L574 211L574 165Q574 159 573 144Q573 129 572 113Q572 97 572 90Q572 79 586 73Q599 67 611 67Q625 67 625 75Q625 90 624 113Q623 137 622 164Z",
			Median: []Point{
				{600, 670}, {600, 80},
			}},
	}},
	{Char: '5', Strokes: []StrokeDef{
		// Stroke 1: top horizontal (left to right is more natural in writing, but kept as defined)
		{Outline: "M495 108L493 108Q444 109 407 134Q370 159 348 192Q344 198 338 198Q330 198 321 185Q313 171 313 157Q313 146 319 140Q350 107 394 84Q438 62 493 62Q570 62 618 90Q666 119 689 166Q711 213 711 266Q711 324 685 365Q659 405 618 427Q577 448 530 449L523 449Q482 449 449 436Q416 423 389 402L422 607L609 621Q628 623 647 623Q665 623 676 624Q687 624 687 636Q687 646 679 659Q671 671 661 671L659 671Q648 669 632 668Q615 667 603 666L425 653Q406 663 392 663Q379 663 379 653Q379 646 379 636Q379 626 377 617L345 410L336 380Q336 377 336 372Q336 357 345 347Q355 337 365 337Q370 337 373 340Q392 356 412 371Q432 386 458 396Q484 405 523 405Q583 404 622 368Q661 331 661 266Q661 225 643 189Q625 152 589 130Q552 108 495 108Z",
			Median: []Point{
				{670, 640}, {520, 640}, {400, 640},
			}},
		// Stroke 2: vertical down then bowl curve
		{Outline: "M495 108L493 108Q444 109 407 134Q370 159 348 192Q344 198 338 198Q330 198 321 185Q313 171 313 157Q313 146 319 140Q350 107 394 84Q438 62 493 62Q570 62 618 90Q666 119 689 166Q711 213 711 266Q711 324 685 365Q659 405 618 427Q577 448 530 449L523 449Q482 449 449 436Q416 423 389 402L422 607L609 621Q628 623 647 623Q665 623 676 624Q687 624 687 636Q687 646 679 659Q671 671 661 671L659 671Q648 669 632 668Q615 667 603 666L425 653Q406 663 392 663Q379 663 379 653Q379 646 379 636Q379 626 377 617L345 410L336 380Q336 377 336 372Q336 357 345 347Q355 337 365 337Q370 337 373 340Q392 356 412 371Q432 386 458 396Q484 405 523 405Q583 404 622 368Q661 331 661 266Q661 225 643 189Q625 152 589 130Q552 108 495 108Z",
			Median: []Point{
				{400, 640}, {380, 480}, {400, 400},
				{490, 420}, {560, 410}, {620, 380},
				{660, 320}, {660, 230}, {620, 160},
				{550, 110}, {460, 100}, {380, 130},
			}},
	}},
	{Char: '6', Strokes: []StrokeDef{
		{Outline: "M380 366Q415 439 456 506Q497 572 560 645Q564 649 567 654Q571 658 571 662Q571 674 557 677Q543 681 534 681Q522 681 515 675Q509 668 498 653Q447 576 405 506Q362 436 336 370Q311 305 311 242Q311 192 336 152Q361 112 406 87Q451 63 511 63Q579 63 624 92Q669 120 691 167Q713 214 713 269Q713 320 688 359Q662 398 620 421Q578 443 527 443Q482 443 444 423Q405 402 380 366ZM511 107Q439 107 400 146Q361 186 361 243Q361 286 381 322Q402 357 438 379Q475 401 525 401Q565 401 596 382Q628 362 645 332Q663 301 663 266Q663 222 646 186Q628 149 595 128Q561 107 511 107Z",
			Median: []Point{
				// From top-right tail, curve down-left into bowl, looping clockwise
				{560, 660}, {490, 580}, {430, 480}, {380, 380},
				{340, 290}, {320, 220}, {350, 150},
				{410, 100}, {490, 80}, {570, 100},
				{640, 140}, {680, 220}, {680, 290},
				{640, 370}, {570, 420}, {490, 430}, {410, 410},
				{370, 380},
			}},
	}},
	{Char: '7', Strokes: []StrokeDef{
		{Outline: "M388 601L640 623Q606 555 573 482Q539 409 510 341Q481 272 459 216Q437 160 424 125Q412 90 412 85Q412 73 425 70Q438 68 447 68Q466 68 469 80Q475 102 480 120Q485 139 490 152Q516 224 547 300Q577 376 611 451Q645 526 681 593Q685 600 693 612Q700 623 702 632Q702 633 703 634Q703 634 703 635Q703 647 690 659Q677 670 667 671L664 671Q656 671 648 668Q639 666 628 665L370 646Q367 646 363 646Q359 646 356 646Q345 646 339 646Q332 647 325 647Q321 647 321 642Q321 637 326 627Q331 617 341 608Q350 599 363 599Q369 599 376 600Q382 600 388 601Z",
			Median: []Point{
				// Horizontal slightly up to right, then diagonal down-left in one continuous motion
				{350, 600}, {500, 615}, {660, 630},
				{580, 450}, {500, 280}, {440, 130}, {420, 80},
			}},
	}},
	{Char: '8', Strokes: []StrokeDef{
		{Outline: "M551 414L541 419Q559 430 587 451Q615 471 642 494Q670 517 689 538Q708 558 708 571Q708 587 694 587Q685 587 674 577L670 572Q675 580 675 588Q675 593 665 607Q654 622 634 639Q613 656 583 668Q554 680 516 680Q463 680 423 660Q384 640 362 609Q340 577 340 539Q340 498 367 468Q393 439 450 410Q411 385 379 356Q348 328 330 295Q311 263 311 223Q311 178 334 142Q356 106 399 84Q441 63 498 63Q564 63 612 84Q660 106 686 144Q713 182 713 234Q713 297 670 340Q627 383 551 414ZM495 439Q441 464 415 485Q390 506 390 538Q390 564 405 586Q420 609 449 623Q477 637 517 637Q557 637 585 618Q613 600 632 569Q639 558 649 558Q654 558 656 560Q623 528 583 498Q543 469 495 439ZM497 389Q552 368 585 347Q617 325 634 304Q651 283 656 265Q662 246 662 231Q662 194 640 167Q618 139 582 124Q545 108 498 108Q452 108 422 126Q392 143 377 170Q362 197 362 226Q362 271 400 310Q439 349 497 389Z",
			Median: []Point{
				// Continuous figure-8: upper loop down-right through waist, lower loop, back up through waist, close upper loop
				{512, 660}, {430, 640}, {370, 580}, {360, 510},
				{410, 450}, {490, 410},
				{570, 370}, {630, 310}, {660, 230},
				{620, 150}, {550, 100}, {490, 90},
				{420, 100}, {360, 150}, {340, 230},
				{370, 320}, {440, 380}, {520, 420},
				{600, 460}, {650, 520}, {660, 580},
				{620, 640}, {560, 660}, {512, 660},
			}},
	}},
	{Char: '9', Strokes: []StrokeDef{
		{Outline: "M575 67L578 67Q611 68 611 84Q614 110 621 155Q628 201 638 257Q648 312 659 370Q670 428 681 480Q692 532 702 570Q711 608 717 623Q717 624 717 625Q718 626 718 627Q718 636 705 645Q692 654 682 655L680 655Q668 655 668 640Q667 624 662 606Q656 617 639 635Q622 652 592 666Q562 680 520 680Q476 680 438 660Q399 641 369 609Q339 576 323 535Q306 494 306 452Q306 403 325 369Q345 335 377 317Q409 299 448 299Q495 299 543 325Q590 352 619 398Q618 394 614 371Q610 347 603 313Q596 278 588 239Q581 199 574 163Q566 127 561 101Q560 96 559 90Q558 85 558 81Q558 75 561 71Q565 67 575 67ZM655 567L645 526Q630 479 602 438Q573 396 534 371Q495 345 447 345Q406 345 380 373Q355 400 355 454Q355 484 367 515Q379 547 401 574Q424 601 455 618Q486 635 525 635Q555 635 577 624Q598 613 611 598Q624 583 629 571Q634 560 642 560Q648 560 655 567Z",
			Median: []Point{
				// Bowl counterclockwise from upper-right, then descending tail
				{650, 500}, {630, 580}, {580, 640}, {510, 660},
				{440, 640}, {380, 580}, {350, 500},
				{370, 410}, {430, 360}, {510, 350},
				{580, 380}, {630, 430}, {650, 480},
				{640, 390}, {620, 280}, {600, 180}, {580, 80},
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
Glyph outlines derived from Klee One Regular by Fontworks Inc.
Klee One is licensed under the SIL Open Font License, Version 1.1.
See https://openfontlicense.org/ for the license text.
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
