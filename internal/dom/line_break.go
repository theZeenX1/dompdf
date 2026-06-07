package dom

import (
	"fmt"
	"math"
)

type lineFragment struct {
	X, Y          float64
	Height, Width int
	Line          []TextNode
}

// Helper constants for Knuth-Plass defaults
const (
	infinity      = 10000.
	forcedBreak   = -10000.
	hyphenPenalty = 50.
)

type kpBoxType int16

const (
	tword kpBoxType = iota
	tglue
	tpenalty
)

// store a text run inside a box consisting of multiple fonts
//
// for e.g., "Hel" (style A) + "lo" (style B)
type textRun struct {
	text  string
	style *Style
	w     float64
}

// all probable candidates (potentially active nodes) as mentioned in the Knuth-Plass Line Break algo
type candidate struct {
	t       kpBoxType
	w, y, z float64   // individual dimensions of this item
	sumW    float64   // accumulated width up to this point
	sumY    float64   // accumulated stretchability
	sumZ    float64   // accumulated shrinkability
	p       float64   // penalty value (if penalty node)
	flagged bool      // flagged penalty (for consecutive hyphen checks)
	runs    []textRun // set of continuous characters without whitespaces in between
}

// active node stores the optimal break points for our paragraph
type activeNode struct {
	index    int         // index of the candidate in candidates array
	demerits float64     // minimization criteria
	prev     *activeNode // previous activeNode for backtracking
}

func (p *ParagraphNode) calculateCandidates() ([]candidate, error) {
	var results []candidate

	// prefix sums
	sumW, sumY, sumZ := 0., 0., 0.

	// to store the width of the current word
	currRun := []rune{}
	currRunWidth := 0.

	currBoxRuns := []textRun{}
	wordWidth := 0.
	currStyle := (*Style)(nil)

	// flushes currRun in currBoxRuns
	//
	// sets currRunWidth = 0
	//
	// and sets currRun back to empty
	flushRun := func() {
		if len(currRun) == 0 {
			return
		}

		currBoxRuns = append(currBoxRuns, textRun{
			text:  string(currRun),
			style: currStyle,
			w:     currRunWidth,
		})

		currRun = currRun[:0]
		currRunWidth = 0
	}

	// adds wordWidth to sumW
	//
	// sets wordWidth to zero
	//
	// sets currWord to []rune{}
	flushWordBox := func() {
		flushRun()

		if len(currBoxRuns) == 0 {
			return
		}

		sumW += wordWidth

		box := candidate{
			t:    tword,
			w:    wordWidth,
			sumW: sumW,
			sumY: sumY,
			sumZ: sumZ,
			runs: append([]textRun(nil), currBoxRuns...),
		}

		results = append(results, box)

		wordWidth = 0
		currBoxRuns = currBoxRuns[:0]
	}

	// sets stretch and shrink as `whiteSpaceWidth / 2` and `whiteSpaceWidth / 3`
	//
	// sets runs with a single " "
	flushGlueBox := func(whiteSpaceWidth float64, textStyle *Style) {
		stretch := whiteSpaceWidth / 2
		shrink := whiteSpaceWidth / 3

		sumW += whiteSpaceWidth
		sumY += stretch
		sumZ += shrink

		box := candidate{
			t:    tglue,
			w:    whiteSpaceWidth,
			y:    stretch,
			z:    shrink,
			sumW: sumW,
			sumY: sumY,
			sumZ: sumZ,
			runs: []textRun{
				{
					text:  " ",
					style: textStyle,
					w:     whiteSpaceWidth,
				},
			},
		}

		results = append(results, box)
	}

	// appends a penalty box with p=50
	//
	// sets runs as a single word: "-"
	flushPenaltyBox := func(hyphenWidth float64, textStyle *Style) {
		results = append(results, candidate{
			t:       tpenalty,
			w:       hyphenWidth,
			sumW:    sumW,
			sumY:    sumY,
			sumZ:    sumZ,
			p:       50,
			flagged: true,
			runs: []textRun{
				{
					text:  "-",
					style: textStyle,
					w:     hyphenWidth,
				},
			},
		})
	}

	for _, frag := range p.Fragments {
		fs := frag.style.FontStyle
		fsize := float64(frag.style.FontSize)

		f, ok := frag.style.Font.Fmap[fs]
		if !ok {
			return nil, fmt.Errorf("font map doesn't contain %d", fs)
		}

		unitsPerEm := float64(f.Metrics().UnitsPerEm)
		if unitsPerEm == 0 {
			unitsPerEm = 1000
		}

		whiteSpaceW, ok := f.GlyphAdvancedWidth(' ')
		if !ok {
			return nil, fmt.Errorf("whitespace not present in glyph advanced width for the font: %s", f.FontName())
		}

		// flush a "common styled" run in runs
		if currStyle != &frag.style {
			flushRun()
			currStyle = &frag.style
		}

		for _, r := range frag.Text {
			rw := 0.

			v, ok := f.GlyphAdvancedWidth(r)
			if !ok {
				rw = float64(whiteSpaceW) * fsize / unitsPerEm
			} else {
				rw = float64(v) * fsize / unitsPerEm
			}

			switch r {
			case ' ':
				// glue
				// if len(currWord) > 0, append it in a word box
				// else or after, append ' ' in a glue box
				if wordWidth > 0 {
					flushWordBox()
				}
				flushGlueBox(rw, &frag.style)
			case '-':
				// penalty
				// first flush the preceding word
				if wordWidth > 0 {
					flushWordBox()
				}
				// then flush "-"
				flushPenaltyBox(rw, &frag.style)
			default:
				// word
				currRun = append(currRun, r)
				currRunWidth += rw
				wordWidth += rw
			}
		}
	}

	if wordWidth > 0 {
		flushWordBox()
	}

	// append paragraph end
	results = append(results, candidate{
		t:       tpenalty,
		p:       forcedBreak,
		flagged: false,
		sumW:    sumW,
		sumY:    sumY,
		sumZ:    sumZ,
	})

	return results, nil
}

func (p *ParagraphNode) LineBreak(ctx LayoutContext) ([]lineFragment, error) {
	// first get all line break candidates:
	candidates, err := p.calculateCandidates()
	if err != nil {
		fmt.Printf("dom calculate: %v", err)
		return nil, err
	}
	lines := []lineFragment{}
	fmt.Println(candidates)

	// set of all activeNode sequences
	activeNodes := []*activeNode{{index: -1, demerits: 0, prev: nil}}

	// here b is the potential break node
	for b, candidate := range candidates {
		// first check if the box is a breakpoint
		// glue -> ok, if prev box is a word
		// penalties -> ok
		// word -> not ok
		isBreakPoint := false
		if candidate.t == tpenalty {
			isBreakPoint = true
		} else if candidate.t == tglue && b > 0 && candidates[b-1].t == tword {
			isBreakPoint = true
		}

		if !isBreakPoint {
			continue
		}

		var bestNode *activeNode
		minDemerits := math.MaxFloat64
		nextActiveNodes := append([]*activeNode(nil), activeNodes...)

		// fallback
		fallbackNode := &activeNode{}
		minOverflowRatio := math.MaxFloat64

		// go through all active nodes:
		for _, activeNode := range activeNodes {
			i := activeNode.index
			prevSumW, prevSumZ, prevSumY := 0., 0., 0.
			prevW := 0.

			if i >= 0 {
				prevSumW, prevSumZ, prevSumY = candidates[i].sumW, candidates[i].sumZ, candidates[i].sumY
				prevW = candidates[i].w
			}

			// TODO: something like this: L := scanLines(ctx)
			L := p.layoutBox.Width - (p.style.Padding.Left + p.style.Padding.Right)
			// paragraph indent is only on the first line
			if p.ParagraphIndent != 0 && i < 0 {
				L -= float64(p.ParagraphIndent)
			}

			// La_b -> length of new line post break
			// include the current box in the line (i.e., candidate.sumW only, don't subtract its width)
			La_b := candidate.sumW - (prevSumW - prevW)

			diff := L - La_b
			// adj ratio:
			r := 0.
			if diff > 0 {
				// stretch
				stretch := candidate.sumY - prevSumY
				if stretch > 0 {
					r = (L - La_b) / stretch
				} else {
					r = infinity
				}
			} else {
				// shrink
				shrink := candidate.sumZ - prevSumZ
				if shrink > 0 {
					r = (L - La_b) / shrink
				} else {
					r = infinity
				}
			}

			// line too long
			if r < -1 {
				if math.Abs(r) < minOverflowRatio {
					minOverflowRatio = math.Abs(r)
					fallbackNode = activeNode
				}
				continue
			}

			// valid candidate:
			nextActiveNodes = append(nextActiveNodes, activeNode)

			// calc badness:
			badness := min(infinity, 100*math.Pow(math.Abs(r), 3))

			// demerits:
			demerits := 0.
			penalty := candidate.p
			if penalty >= 0 {
				demerits = math.Pow(1+badness+penalty, 2)
			} else if penalty > -infinity {
				demerits = math.Pow(1+badness, 2) - math.Pow(penalty, 2)
			} else {
				demerits = math.Pow(1+badness, 2)
			}

			// avoid 2 hyphenated consecutive lines
			if candidate.flagged && i >= 0 && candidates[i].flagged {
				demerits += 3000
			}

			// reassign bestnode
			totalDemerits := activeNode.demerits + demerits
			if totalDemerits < minDemerits {
				minDemerits = totalDemerits
				bestNode = activeNode
			}
		}

		// handle overflow lines
		if bestNode == nil && fallbackNode != nil {
			bestNode = fallbackNode
			minDemerits = fallbackNode.demerits + infinity
			nextActiveNodes = append(nextActiveNodes, fallbackNode)
		}

		if bestNode != nil {
			newNode := &activeNode{
				index:    b,
				demerits: minDemerits,
				prev:     bestNode,
			}
			nextActiveNodes = append(nextActiveNodes, newNode)
		}

		activeNodes = nextActiveNodes
	}

	for range activeNodes {
	}

	return lines, nil
}
