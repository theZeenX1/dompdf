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
	infinity      = 1000000
	forcedBreak   = -10000
	hyphenPenalty = 50
)

type kpNodeType int16

const (
	tbox kpNodeType = iota
	tglue
	tpenalty
)

// Calculate prefix sums of all params as mentioned in the Knuth-Plass Line Break algo
type params struct {
	t             kpNodeType
	x, y, w, p, f float64
	textNode      *TextNode
	word          string
}

// active node stores the optimal break points for our paragraph
type activeNode struct {
	index    int
	demerits float64
	prev     *activeNode
}

func (p *ParagraphNode) calculateParams() ([]params, error) {
	var results []params

	// prefix sums
	sumW, sumX, sumY := 0., 0., 0.
	currentBoxWidth := 0.
	var latextTextNode *TextNode
	currentWord := []rune{}

	// helper to flush current box in results
	flushBox := func(t *TextNode) {
		if currentBoxWidth > 0 {
			sumW += currentBoxWidth
			results = append(results, params{
				t:        tbox,
				w:        sumW,
				x:        sumX,
				y:        sumY,
				word:     string(currentWord),
				textNode: t,
			})
			currentBoxWidth = 0
			currentWord = []rune{}
		}
	}

	for _, frag := range p.Fragments {
		fs := frag.style.FontStyle
		fsize := float64(frag.style.FontSize)
		f, ok := frag.style.Font.Fmap[fs]
		latextTextNode = &frag
		if !ok {
			return nil, fmt.Errorf("font map doesn't contain fs: %d", fs)
		}
		unitsPerEm := float64(f.Metrics().UnitsPerEm)

		for _, r := range frag.Text {
			rw := float64(0)
			v, ok := f.GlyphAdvancedWidth(r)
			if !ok {
				whiteSpaceW, ok := f.GlyphAdvancedWidth(' ')
				if !ok {
					return nil, fmt.Errorf("whitespace not present in glyph advanced width")
				}
				rw = float64(whiteSpaceW) * fsize / unitsPerEm // fallback
			} else {
				rw = float64(v) * fsize / unitsPerEm
			}

			switch r {
			case ' ':
				// flush the preceding word into a box
				flushBox(latextTextNode)
				// no consecutive glues
				if len(results) > 0 && results[len(results)-1].t == tglue {
					continue
				}

				// append glue for the space
				// stretch is 1/2 space width, shrink is 1/3
				glueW := rw
				glueStretch := rw / 2
				glueShrink := rw / 3

				sumW += glueW
				sumX += glueStretch
				sumY += glueShrink

				results = append(results, params{
					t:        tglue,
					w:        sumW,
					x:        sumX,
					y:        sumY,
					textNode: latextTextNode,
				})

			case '-':
				// hyphens are part of the word (box), followed by a permissible break (penalty)
				currentBoxWidth += rw
				currentWord = append(currentWord, r)
				flushBox(latextTextNode)

				// add a flagged penalty for the breakpoint
				results = append(results, params{
					t:        tpenalty,
					w:        sumW,
					x:        sumX,
					y:        sumY,
					p:        hyphenPenalty,
					f:        1,
					textNode: latextTextNode,
				})

			case '\n':
				// explicit forced line break
				flushBox(latextTextNode)

				results = append(results, params{
					t:        tpenalty,
					w:        sumW,
					x:        sumX,
					y:        sumY,
					p:        forcedBreak,
					f:        0,
					textNode: latextTextNode,
				})

			default:
				// accumulate normal character's width into the current word box
				currentBoxWidth += rw
				currentWord = append(currentWord, r)
			}
		}
	}

	// Flush the final word of the paragraph
	flushBox(latextTextNode)

	// we append an infinite glue followed by a forced break penalty.
	// this is for the remaining space on the last line so it doesn't get fully justified (stretched out).
	sumX += infinity
	results = append(results, params{
		t:        tglue,
		w:        sumW,
		x:        sumX,
		y:        sumY,
		textNode: latextTextNode,
	})

	results = append(results, params{
		t:        tpenalty,
		w:        sumW,
		x:        sumX,
		y:        sumY,
		p:        forcedBreak,
		f:        0,
		textNode: latextTextNode,
	})

	return results, nil
}

func (p *ParagraphNode) LineBreak(ctx LayoutContext) ([]lineFragment, error) {
	// first get all line break params:
	params, err := p.calculateParams()
	if err != nil {
		fmt.Printf("dom calculateParams: %v", err)
		return nil, err
	}

	// width is an "inside" property, so no need to integrate margin here
	pW := p.layoutBox.Width - (p.style.Padding.Left + p.style.Padding.Right)

	lineWidth := pW
	// for the first line:
	if p.ParagraphIndent != 0 {
		lineWidth -= float64(p.ParagraphIndent)
	}

	// -1 is the start of the paragraph
	activeNodes := []*activeNode{{index: -1, demerits: 0, prev: nil}}

	for j, node := range params {
		// is current node a break point?
		// Knuth-Plass breaks at penalties
		// or at glues immediately preceded by a box
		isBreakpoint := false
		if node.t == tpenalty {
			isBreakpoint = true
		} else if node.t == tglue && j > 0 && params[j-1].t == tbox {
			isBreakpoint = true
		}

		if !isBreakpoint {
			continue
		}

		bestNode := &activeNode{}
		minDemerits := math.MaxFloat64
		nextActiveNodes := []*activeNode{}

		// fallback if current box width is greater than paragraph width
		fallbackNode := &activeNode{}
		minOverfullRatio := math.MaxFloat64

		// go through all active nodes
		for _, active := range activeNodes {
			i := active.index

			w, x, y := 0., 0., 0.
			if i >= 0 {
				w, x, y = params[i].w, params[i].x, params[i].y
			}

			// L is the length between i and j
			L := float64(node.w - w)
			diff := lineWidth - L

			r := float64(0)
			if diff > 0 {
				// L too short
				stretch := float64(node.x - x)
				if stretch > 0 {
					r = diff / stretch
				} else {
					r = 10000
				}
			} else if diff < 0 {
				// L too long
				shrink := float64(node.y - y)
				if shrink > 0 {
					r = diff / shrink
				} else {
					r = -10000
				}
			}

			// if r < -1, line can't shrink enough (too long)
			if r < -1 {
				if math.Abs(r) < minOverfullRatio {
					minOverfullRatio = math.Abs(r)
					fallbackNode = active
				}
				continue
			}

			// valid candidate, keep it for the next iterations
			nextActiveNodes = append(nextActiveNodes, active)

			// calculate badness:
			badness := min(10000, 100*math.Pow(math.Abs(r), 3))

			// demerit calculation:
			demerits := float64(0)
			penalty := float64(node.p)
			if penalty <= forcedBreak {
				demerits = math.Pow(1+badness, 2)
			} else {
				demerits = math.Pow(1+badness+penalty, 2)
			}

			// avoid 2 hyphanated consecutive lines
			if node.f == 1 && i >= 0 && params[i].f == 1 {
				demerits += 3000
			}

			// reassign the bestNode for backtracking
			// (rest all nodes will have prev == nil)
			totalDemerits := active.demerits + demerits
			if totalDemerits < minDemerits {
				minDemerits = totalDemerits
				bestNode = active
			}
		}

		// handle overflow lines
		if bestNode == nil && fallbackNode != nil {
			bestNode = fallbackNode
			minDemerits = fallbackNode.demerits + 100000
			nextActiveNodes = append(nextActiveNodes, fallbackNode)
		}

		if bestNode != nil {
			newNode := &activeNode{
				index:    j,
				demerits: minDemerits,
				prev:     bestNode,
			}
			nextActiveNodes = append(nextActiveNodes, newNode)
		}

		activeNodes = nextActiveNodes
	}

	// find breaks
	breakIndices := []int{}
	if len(activeNodes) > 0 {
		curr := activeNodes[len(activeNodes)-1]
		for curr != nil && curr.index != -1 {
			breakIndices = append([]int{curr.index}, breakIndices...)
			curr = curr.prev
		}
	}

	fragments := []lineFragment{}
	startIdx := 0
	for _, breakIdx := range breakIndices {
		lineParams := params[startIdx : breakIdx+1]

		for _, lp := range lineParams {
			if lp.t == tbox || (lp.t == tpenalty && lp.f == 1) {
				// words/hyphens
				line := []TextNode{}

				fragments = append(fragments, lineFragment{
					Line: line,
				})
			} else if lp.t == tglue && lp != lineParams[len(lineParams)-1] {
				// spaces
			}
		}

	}

	return fragments, nil
}
