package dom

import (
	"fmt"
	"math"
)

const (
	kpInfinity        = 10000.
	kpForcedBreak     = -10000.
	kpPenalty         = 50.
	kpDemeritsLine    = 10.
	kpDemeritsFlagged = 100.
	kpDemeritsFitness = 3000.
	kpRho             = 3
)

type kpNodeType int16

const (
	tbox kpNodeType = iota
	tglue
	tpenalty
)

type kpTextrun struct {
	x, y  float64
	style *Style
	text  string
}

type kpNode struct {
	t             kpNodeType  // type of node (box, glue, penalty)
	runs          []kpTextrun // all font styles present in a single run
	width, height float64     // width of all runs, max height present in the run
	y, z          float64     // stretch, shrink
	penalty       float64     // penalty for a hyphen (common for forced and actual hyphen (e.g., con-side-rate and break-point))
	flagged       bool        // here flagged is true if a word is hyphenated without the presence of a '-' character in the word. if flagged, insert a "-" while reconstructing the word
}

type kpActiveNode struct {
	index                  int
	fitnessClass           int
	line                   int
	ratio                  float64
	demerits               float64
	sumW, sumY, sumZ, sumH float64 // width, stretch, shrink, height
	prev                   *kpActiveNode
	next                   *kpActiveNode
}

type kpCandidate struct {
	active          *kpActiveNode
	ratio, demerits float64
}

type line struct {
	x, y float64
	runs []kpTextrun
}

// https://github.com/hyphenation/tex-hyphen/blob/master/webpage/hyphenator/patterns/en_us.js

func (p *ParagraphNode) createKpNodes() ([]kpNode, error) {
	// store all nodes for kp algorithm
	nodes := []kpNode{}
	// store current run details:z
	currentRuns := []kpTextrun{}
	var currentRun []rune
	currentWidth, currentHeight := 0., 0.
	var currentStyle *Style

	flushRun := func() {
		if currentRun == nil || len(currentRun) == 0 {
			return
		}

		currentRuns = append(currentRuns, kpTextrun{
			style: currentStyle,
			text:  string(currentRun),
		})

		currentRun = nil
	}

	flushBox := func() {
		if currentRun != nil && len(currentRun) > 0 {
			flushRun()
		}
		if currentRuns == nil || len(currentRuns) == 0 {
			return
		}
		nodes = append(nodes, kpNode{
			t:       tbox,
			width:   currentWidth,
			height:  currentHeight,
			runs:    currentRuns,
			penalty: 0,
			flagged: false,
		})

		currentRuns = nil
		currentHeight, currentWidth = 0., 0.
	}

	flushPenalty := func() {
		nodes = append(nodes, kpNode{
			t:       tpenalty,
			penalty: kpPenalty,
			flagged: false,
		})
	}

	flushGlue := func(rw float64) {
		nodes = append(nodes, kpNode{
			t:     tglue,
			width: rw,
			y:     rw / 3,
			z:     rw / 6,
		})
	}

	for _, frag := range p.Fragments {
		// on a new fragment, flush the previous run:
		flushRun()
		// set current style
		currentStyle = &frag.style

		fs := frag.style.FontStyle
		fsize := float64(frag.style.FontSize)
		font, ok := frag.style.Font.Fmap[fs]
		if !ok {
			return nil, fmt.Errorf("no font found in fmap for font style: %d", fs)
		}
		unitsPerEm := float64(font.Metrics().UnitsPerEm)
		if unitsPerEm == 0 {
			unitsPerEm = 1000.
		}
		whiteSpaceW, ok := font.GlyphAdvancedWidth(' ')
		if !ok {
			return nil, fmt.Errorf("whitespace not present in glyph advanced width for the font: %s", font.FontName())
		}

		for _, r := range frag.Text {
			// store the max height for each run
			currentHeight = max(currentHeight, float64(font.Metrics().CapHeight)*fsize/unitsPerEm)

			rw := 0.
			v, ok := font.GlyphAdvancedWidth(r)
			if !ok {
				rw = float64(whiteSpaceW) * fsize / unitsPerEm
			} else {
				rw = float64(v) * fsize / unitsPerEm
			}

			switch r {
			case ' ':
				// first flush box
				flushBox()
				// then flush the glue
				flushGlue(rw)
			case '-', '\u00AD':
				// TODO: \u00ad is a soft hyphen, flagged == true for that, enable a true/false flag in flush penalty
				// add "-" to the current run:
				currentRun = append(currentRun, r)
				currentWidth += rw
				// then flushBox and flushPenalty
				flushBox()
				flushPenalty()
			default:
				currentRun = append(currentRun, r)
				currentWidth += rw
			}
		}
	}

	// flush any remaining box:
	flushBox()

	// insert forced break at the end
	nodes = append(nodes, kpNode{
		t:       tpenalty,
		penalty: kpForcedBreak,
		flagged: false,
	})

	return nodes, nil
}

func (p *ParagraphNode) kpMainLoop(
	index int, nodes []kpNode,
	sumW, sumH, sumY, sumZ float64,
	activeNodes *kpActiveNodeList,
) {
	activeNode := activeNodes.getFirst()
	demerits := 0.

	for {
		if activeNodes.isNil(activeNode) {
			break
		}

		// candidates per class:
		candidates := []kpCandidate{{demerits: kpInfinity}, {demerits: kpInfinity}, {demerits: kpInfinity}, {demerits: kpInfinity}}

		for {
			if activeNodes.isNil(activeNode) {
				break
			}

			next := activeNode.next
			currentLine := activeNode.line + 1

			La_b := sumW - activeNode.sumW
			// TODO: somehow get L using context and sumH!
			L := p.style.Width - (p.style.Padding.Left + p.style.Padding.Right)
			if p.ParagraphIndent != 0 && currentLine == 0 {
				L -= float64(p.ParagraphIndent)
			}

			// adjustment ratio:
			r := 0.Then the loop runs as-is. The - chars in the hyphenated text hit the existing case '-', '\u00AD': branch, producing box + penalty naturally.
			diff := L - La_b
			if diff > 0 {
				// line too short
				stretch := sumY - activeNode.sumY
				if stretch > 0 {
					r = diff / stretch
				} else {
					r = kpInfinity
				}
			} else {
				// line too long
				shrink := sumZ - activeNode.sumZ
				if shrink > 0 {
					r = diff / shrink
				} else {
					r = kpInfinity
				}
			}

			if r < -1 || (nodes[index].t == tpenalty && nodes[index].penalty == -kpInfinity) {
				activeNodes.delete(activeNode)
			}

			if r >= -1 && r <= kpRho {
				// demerit calculation:
				badness := 100 * math.Pow(math.Abs(r), 3)

				if nodes[index].t == tpenalty && nodes[index].penalty >= 0 {
					demerits = math.Pow(kpDemeritsLine+badness, 2) + math.Pow(nodes[index].penalty, 2)
				} else if nodes[index].t == tpenalty && nodes[index].penalty < 0 {
					demerits = math.Pow(kpDemeritsLine+badness, 2) - math.Pow(nodes[index].penalty, 2)
				} else {
					demerits = math.Pow(kpDemeritsLine+badness, 2)
				}

				if nodes[index].t == tpenalty && nodes[activeNode.index].t == tpenalty {
					demerits += kpDemeritsFlagged
				}

				// current class:
				c := 0
				if r < -.5 {
					c = 0
				} else if r <= .5 {
					c = 1
				} else if r <= 1 {
					c = 2
				} else {
					c = 3
				}

				if math.Abs(float64(c-activeNode.fitnessClass)) > 1 {
					demerits += kpDemeritsFitness
				}

				// if this is a better candidate for the algo
				if demerits < candidates[c].demerits {
					candidates[c] = kpCandidate{
						active:   activeNode,
						ratio:    r,
						demerits: demerits,
					}
				}
			}

			activeNode = next

			if !activeNodes.isNil(activeNode) && activeNode.line >= currentLine {
				break
			}
		}

		// calculate sums till next box or penalty:
		w, y, z := 0., 0., 0.
		for i := index; i < len(nodes); i++ {
			if nodes[i].t == tglue {
				w += nodes[i].width
				y += nodes[i].y
				z += nodes[i].z
			} else {
				break
			}
		}

		for fitnessClass := 0; fitnessClass < len(candidates); fitnessClass++ {
			candidate := candidates[fitnessClass]
			if candidate.demerits < kpInfinity {
				newActiveNode := &kpActiveNode{
					index:        index,
					fitnessClass: fitnessClass,
					line:         candidate.active.line + 1,
					demerits:     candidate.demerits,
					ratio:        candidate.ratio,
					sumW:         sumW + w,
					sumY:         sumY + y,
					sumZ:         sumZ + z,
				}

				if activeNodes.isNil(activeNode) {
					activeNodes.pushBack(newActiveNode)
				} else {
					activeNodes.insertBefore(activeNode, newActiveNode)
				}
			}
		}
	}
}

func (p *ParagraphNode) kpLineBreak(ctx LayoutContext) ([]line, error) {
	// create kp nodes:
	nodes, err := p.createKpNodes()
	if err != nil {
		return nil, err
	}

	sumW, sumH, sumY, sumZ := 0., 0., 0., 0.

	activeNodes := newList()
	activeNodes.pushBack(&kpActiveNode{
		index:        0,
		line:         0,
		fitnessClass: 0,
		demerits:     0,
		ratio:        0,
		prev:         nil,
		next:         nil,
	})

	for i := range nodes {
		if nodes[i].t == tbox {
			sumW += nodes[i].width
			sumH = math.Max(sumH, nodes[i].height)
		} else if nodes[i].t == tglue {
			if i > 0 && nodes[i-1].t == tbox {
				p.kpMainLoop(i, nodes, sumW, sumH, sumY, sumZ, activeNodes)
			}
			sumW += nodes[i].width
			sumY += nodes[i].y
			sumZ += nodes[i].z
		} else if nodes[i].t == tpenalty && nodes[i].penalty != kpInfinity {
			p.kpMainLoop(i, nodes, sumW, sumH, sumY, sumZ, activeNodes)
		}
	}

	best := activeNodes.getFirst()
	node := activeNodes.getFirst()
	for !activeNodes.isNil(node) {
		if node.demerits < best.demerits {
			best = node
		}
		node = node.next
	}

	// trace back breakpoints
	breaks := []*kpActiveNode{}
	for cur := best; cur != nil && !activeNodes.isNil(cur); cur = cur.prev {
		breaks = append(breaks, cur)
	}
	// reverse
	for i, j := 0, len(breaks)-1; i < j; i, j = i+1, j-1 {
		breaks[i], breaks[j] = breaks[j], breaks[i]
	}

	// breaks[i].index gives you the node index of each breakpoint
	lines := []line{}
	prevIdx := 0
	for _, breakNode := range breaks {

	}

	return []line{}, nil
}

// active node linked list:
type kpActiveNodeList struct {
	head, tail *kpActiveNode
	size       uint
}

func (list *kpActiveNodeList) isNil(a *kpActiveNode) bool {
	return a == nil || a == list.head || a == list.tail
}

func newList() *kpActiveNodeList {
	head, tail := &kpActiveNode{}, &kpActiveNode{}
	head.next = tail
	tail.prev = head
	return &kpActiveNodeList{
		head: head,
		tail: tail,
		size: 0,
	}
}

func (a *kpActiveNodeList) pushBack(node *kpActiveNode) {
	if a.isNil(node) {
		return
	}
	tail := a.tail
	prev := tail.prev
	// set prev and next for new node:
	node.prev = prev
	node.next = tail
	// remove old connections:
	tail.prev = node
	prev.next = node

	a.size++
}

func (a *kpActiveNodeList) pushFront(node *kpActiveNode) {
	if a.isNil(node) {
		return
	}
	head := a.head
	next := head.next
	// set prev and next for the new node:
	node.next = next
	node.prev = head
	// remove old connections:
	head.next = node
	next.prev = node

	a.size++
}

func (a *kpActiveNodeList) insertBefore(node, newNode *kpActiveNode) {
	if a.isNil(node) || a.isNil(newNode) {
		return
	}

	newNode.next = node
	newNode.prev = node.prev
	node.prev.next = newNode
	node.prev = newNode

	a.size++
}

func (a *kpActiveNodeList) insertAfter(node, newNode *kpActiveNode) {
	if a.isNil(node) || a.isNil(newNode) {
		return
	}

	newNode.next = node.next
	newNode.prev = node
	node.next.prev = newNode
	node.next = newNode

	a.size++
}

func (a *kpActiveNodeList) delete(node *kpActiveNode) {
	if a.isNil(node) {
		return
	}

	next, prev := node.next, node.prev
	next.prev = prev
	prev.next = next
	a.size--
}

func (a *kpActiveNodeList) getFirst() *kpActiveNode {
	if a.size == 0 {
		return nil
	} else {
		return a.head.next
	}
}

func (a *kpActiveNodeList) getLast() *kpActiveNode {
	if a.size == 0 {
		return nil
	} else {
		return a.tail.prev
	}
}
