package dom

import (
	"fmt"
	"math"
	"strings"
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

// textRun is a struct to hold a string without whitespaces or break-points with a common style
type textRun struct {
	x, y  float64 // x -> left edge of the word box, y -> height from the baseline (baseline + asc of the font)
	width float64 // width of the run
	style *Style  // style
	text  string  // word
}

type kpNode struct {
	t                  kpNodeType // type of node (box, glue, penalty)
	runs               []*textRun // all font styles present in a single node
	width, height, asc float64    // width of all runs, max height, ascender present in the run
	y, z               float64    // stretch, shrink
	penalty            float64    // penalty for a hyphen (common for forced and actual hyphen (e.g., con-side-rate and break-point))
	flagged            bool       // here flagged is true if a word is hyphenated without the presence of a '-' character in the word. if flagged, insert a "-" while reconstructing the word
}

// active node linked list for break points
type kpLinkedList struct {
	head, tail *kpLinkedListNode
	size       uint
}

type kpLinkedListNode struct {
	prev, next *kpLinkedListNode
	data       *kpBreakPoint
}

// potential break point for a paragraph
type kpBreakPoint struct {
	index            int           // index of the node in the createKpNodes array
	parent           *kpBreakPoint // the parent for the current node / previous break point
	fitnessClass     int           // fitness of the line
	line             int           // line number / index
	ratio            float64       // adjustment ratio of the line ending at the current break point
	demerits         float64       // total demerits of the line
	sumW, sumY, sumZ float64       // width, stretch, shrink
}

type kpCandidate struct {
	active          *kpBreakPoint
	ratio, demerits float64
}

type line struct {
	x, y       float64 // top-left corner for the start of the line
	maxAsc     float64 // maxAsc of the line
	lineHeight float64 // height of the line
	runs       []*textRun
}

func (p *ParagraphNode) createKpNodes() ([]kpNode, error) {
	// store all nodes for kp algorithm
	nodes := []kpNode{}
	// store current run details:z
	currentRuns := []*textRun{}
	var currentRun []rune
	currRunWidth, currRunsWidth, currentHeight, currentAsc := 0., 0., 0., 0.
	var currentStyle *Style

	flushRun := func() {
		if len(currentRun) == 0 {
			return
		}

		currentRuns = append(currentRuns, &textRun{
			width: currRunWidth,
			style: currentStyle,
			text:  string(currentRun),
		})

		currentRun = nil
		currRunWidth = 0.
	}

	flushBox := func() {
		if len(currentRun) > 0 {
			flushRun()
		}
		if len(currentRuns) == 0 {
			return
		}
		nodes = append(nodes, kpNode{
			t:       tbox,
			width:   currRunsWidth,
			height:  currentHeight,
			asc:     currentAsc,
			runs:    currentRuns,
			penalty: 0,
			flagged: false,
		})

		currentRuns = nil
		currentHeight, currRunsWidth, currentAsc = 0., 0., 0.
	}

	// flagged on soft-penalty (\u00AD)
	flushPenalty := func(flagged bool) {
		nodes = append(nodes, kpNode{
			t:       tpenalty,
			penalty: kpPenalty,
			flagged: flagged,
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

	if p.Hyphenation {
		var newFragments []TextNode
		for _, frag := range p.Fragments {
			words := strings.Split(frag.Text, " ")
			var newText strings.Builder
			for wi, word := range words {
				hyphenated, err := hyphenateWord(word, frag.LangCode)
				if err != nil {
					hyphenated = word
				}
				newText.WriteString(hyphenated)
				if wi < len(words)-1 {
					newText.WriteRune(' ')
				}
			}
			newFragments = append(newFragments, TextNode{
				layoutBox:    frag.layoutBox,
				nodePointers: frag.nodePointers,
				style:        frag.style,

				Text:     newText.String(),
				LangCode: frag.LangCode,
			})
		}
		p.Fragments = newFragments

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
			// store the max height, asc for each run
			currentAsc = max(currentAsc, frag.style.LineHeight*float64(font.Metrics().Ascender)*fsize/unitsPerEm)
			// lineHeight = LineGap + Ascender + abs(Descender)
			currentHeight = max(
				currentHeight,
				frag.style.LineHeight*(float64(font.Metrics().LineGap+font.Metrics().Ascender)+math.Abs(float64(font.Metrics().Descender)))*fsize/unitsPerEm,
			)

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
			case '-':
				// hard-hyphen (already existing)
				// add "-" to the current run:
				currentRun = append(currentRun, r)
				currRunWidth += rw
				currRunsWidth += rw
				// then flushBox and flushPenalty
				flushBox()
				flushPenalty(false)
			case '\u00AD':
				// soft-hyphen (from hyphenation)
				// if flagged == true, we add hyphen on paint
				flushBox()
				flushPenalty(true)
			default:
				currentRun = append(currentRun, r)
				currRunWidth += rw
				currRunsWidth += rw
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
	sumW, sumY, sumZ float64,
	activeNodes *kpLinkedList,
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
			currentLine := activeNode.data.line + 1

			La_b := sumW - activeNode.data.sumW
			// TODO: somehow get L using context and index of line (currentLine)!
			L := p.style.Width - (p.style.Padding.Left + p.style.Padding.Right)
			if p.ParagraphIndent != 0 && currentLine == 1 {
				L -= float64(p.ParagraphIndent)
			}

			// adjustment ratio:
			r := 0.
			diff := L - La_b
			if diff > 0 {
				// line too short
				stretch := sumY - activeNode.data.sumY
				if stretch > 0 {
					r = diff / stretch
				} else {
					r = kpInfinity
				}
			} else {
				// line too long
				shrink := sumZ - activeNode.data.sumZ
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

				if nodes[index].t == tpenalty && nodes[index].flagged &&
					nodes[activeNode.data.index].t == tpenalty && nodes[activeNode.data.index].flagged {
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

				if math.Abs(float64(c-activeNode.data.fitnessClass)) > 1 {
					demerits += kpDemeritsFitness
				}

				// if this is a better candidate for the algo
				if demerits < candidates[c].demerits {
					candidates[c] = kpCandidate{
						active:   activeNode.data,
						ratio:    r,
						demerits: demerits,
					}
				}
			}

			activeNode = next

			if !activeNodes.isNil(activeNode) && activeNode.data.line >= currentLine {
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
				newActiveNode := &kpBreakPoint{
					index:        index,
					parent:       candidate.active,
					fitnessClass: fitnessClass,
					line:         candidate.active.line + 1,
					demerits:     candidate.demerits,
					ratio:        candidate.ratio,
					sumW:         sumW + w,
					sumY:         sumY + y,
					sumZ:         sumZ + z,
				}

				if activeNodes.isNil(activeNode) {
					activeNodes.pushBack(&kpLinkedListNode{
						data: newActiveNode,
					})
				} else {
					activeNodes.insertBefore(activeNode, &kpLinkedListNode{
						data: newActiveNode,
					})
				}
			}
		}
	}
}

func (p *ParagraphNode) kpLineBreak(ctx LayoutContext) ([]*line, error) {
	// create kp nodes:
	nodes, err := p.createKpNodes()
	if err != nil {
		return nil, err
	}

	sumW, sumY, sumZ := 0., 0., 0.

	activeNodes := newList()
	activeNodes.pushBack(&kpLinkedListNode{
		data: &kpBreakPoint{
			index:        0,
			parent:       nil,
			line:         0,
			fitnessClass: 0,
			demerits:     0,
			ratio:        0,
		},
	})

	for i := range nodes {
		if nodes[i].t == tbox {
			sumW += nodes[i].width
		} else if nodes[i].t == tglue {
			if i > 0 && nodes[i-1].t == tbox {
				p.kpMainLoop(i, nodes, sumW, sumY, sumZ, activeNodes)
			}
			sumW += nodes[i].width
			sumY += nodes[i].y
			sumZ += nodes[i].z
		} else if nodes[i].t == tpenalty && nodes[i].penalty != kpInfinity {
			p.kpMainLoop(i, nodes, sumW, sumY, sumZ, activeNodes)
		}
	}

	best := activeNodes.getFirst()
	node := activeNodes.getFirst()
	for !activeNodes.isNil(node) {
		if node.data.demerits < best.data.demerits {
			best = node
		}
		node = node.next
	}

	// trace back breakpoints
	breaks := []*kpBreakPoint{}
	for cur := best; cur != nil && !activeNodes.isNil(cur); cur = cur.prev {
		breaks = append(breaks, cur.data)
	}
	// reverse
	for i, j := 0, len(breaks)-1; i < j; i, j = i+1, j-1 {
		breaks[i], breaks[j] = breaks[j], breaks[i]
	}

	// breaks[i].index gives you the node index of each breakpoint
	lines := []*line{}
	// cursor x, y
	cx, cy := (p.layoutBox.X + p.style.Padding.Left), (p.layoutBox.Y + p.style.Padding.Top)
	if p.ParagraphIndent != 0 {
		cx += float64(p.ParagraphIndent)
	}
	for i := 0; i < len(breaks)-1; i++ {
		startIdx := breaks[i].index
		endIdx := breaks[i+1].index

		// calculate maxAsc, maxHeight for the current line
		maxAsc, maxHeight := 0., 0.
		for j := startIdx; j < endIdx; j++ {
			node := nodes[j]
			if node.t == tbox {
				maxAsc = max(maxAsc, node.asc)
				maxHeight = max(maxHeight, node.height)
			}
		}

		runs := []*textRun{}
		// set line x, y:
		lx, ly := cx, cy
		// now calculate the x, y for each run
		for j := startIdx; j < endIdx; j++ {
			node := nodes[j]
			switch node.t {
			case tbox:
				for _, run := range nodes[j].runs {
					font := run.style.Font.Fmap[run.style.FontStyle]
					fasc := float64(font.Metrics().Ascender) * float64(run.style.FontSize) / float64(font.Metrics().UnitsPerEm)
					// ascent is the height from the baseline to the top of the tallest character
					// therefore for a common baseline in a line,
					// we subtract maxAsc from the "cy" to get the common baseline for the line
					r := &textRun{
						x:     cx,
						y:     cy - maxAsc + fasc,
						style: run.style,
						text:  run.text,
					}
					cx += run.width
					runs = append(runs, r)
				}
			case tglue:
				// ratio for the current line is stored in i + 1
				cx += node.width * breaks[i+1].ratio
			case tpenalty:
				if j == endIdx-1 && node.flagged {
					// soft-hyphen breakpoint: render a "-" using the style of the last run
					if len(runs) > 0 {
						lastRun := runs[len(runs)-1]
						font := lastRun.style.Font.Fmap[lastRun.style.FontStyle]
						unitsPerEm := float64(font.Metrics().UnitsPerEm)
						if unitsPerEm == 0 {
							unitsPerEm = 1000.
						}
						hyphenW, ok := font.GlyphAdvancedWidth('-')
						hyphenWidth := 0.
						if ok {
							hyphenWidth = float64(hyphenW) * float64(lastRun.style.FontSize) / unitsPerEm
						}
						fasc := float64(font.Metrics().Ascender) * float64(lastRun.style.FontSize) / unitsPerEm
						runs = append(runs, &textRun{
							x:     cx,
							y:     cy - maxAsc + fasc,
							style: lastRun.style,
							text:  "-",
						})
						cx += hyphenWidth
					}
				}
			}

			lines = append(lines, &line{
				x:          lx,
				y:          ly,
				maxAsc:     maxAsc,
				lineHeight: maxHeight,
				runs:       runs,
			})

			// increment cy by maxHeight
			cy += maxHeight

			// reset cursor x
			cx = (p.layoutBox.X + p.style.Padding.Left)
		}
	}

	return lines, nil
}

// ! utility functions for linked list:

func (list *kpLinkedList) isNil(a *kpLinkedListNode) bool {
	return a == nil || a == list.head || a == list.tail
}

func newList() *kpLinkedList {
	head, tail := &kpLinkedListNode{}, &kpLinkedListNode{}
	head.next = tail
	tail.prev = head
	return &kpLinkedList{
		head: head,
		tail: tail,
		size: 0,
	}
}

func (a *kpLinkedList) pushBack(node *kpLinkedListNode) {
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

func (a *kpLinkedList) pushFront(node *kpLinkedListNode) {
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

func (a *kpLinkedList) insertBefore(node, newNode *kpLinkedListNode) {
	if a.isNil(node) || a.isNil(newNode) {
		return
	}

	newNode.next = node
	newNode.prev = node.prev
	node.prev.next = newNode
	node.prev = newNode

	a.size++
}

func (a *kpLinkedList) insertAfter(node, newNode *kpLinkedListNode) {
	if a.isNil(node) || a.isNil(newNode) {
		return
	}

	newNode.next = node.next
	newNode.prev = node
	node.next.prev = newNode
	node.next = newNode

	a.size++
}

func (a *kpLinkedList) delete(node *kpLinkedListNode) {
	if a.isNil(node) {
		return
	}

	next, prev := node.next, node.prev
	next.prev = prev
	prev.next = next
	a.size--
}

func (a *kpLinkedList) getFirst() *kpLinkedListNode {
	if a.size == 0 {
		return nil
	} else {
		return a.head.next
	}
}

func (a *kpLinkedList) getLast() *kpLinkedListNode {
	if a.size == 0 {
		return nil
	} else {
		return a.tail.prev
	}
}
