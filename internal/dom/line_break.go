package dom

import "fmt"

type lineFragment struct {
	X, Y          float64
	Height, Width int
	Line          []TextNode
}

// Helper constants for Knuth-Plass defaults
const (
	infinity      = 1000000
	forcedBreak   = -10000
	hyphenPenalty = 50
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
type candidates struct {
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
	index    int
	demerits float64
	prev     *activeNode
}

func (p *ParagraphNode) calculateCandidates() ([]candidates, error) {
	var results []candidates

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

		box := candidates{
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

		box := candidates{
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
		results = append(results, candidates{
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
	results = append(results, candidates{
		t:       tpenalty,
		p:       -10000,
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
	return lines, nil
}
