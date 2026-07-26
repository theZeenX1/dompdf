package paragraph

import (
	"fmt"
	"math"
	"strings"

	"github.com/theZeenX1/dompdf/internal/v0/dom"
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
	x, y  float64    // x -> left edge of the word box, y -> height from the baseline (baseline + asc of the font)
	width float64    // width of the run
	style *dom.Style // style
	text  string     // word
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
	lineHeight       float64       // line height for the current breakpoint-parent pair
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

func createKpNodes(p *dom.ParagraphNode) ([]kpNode, error) {
	// store all nodes for kp algorithm
	nodes := []kpNode{}
	// store current run details:z
	currentRuns := []*textRun{}
	var currentRun []rune
	currRunWidth, currRunsWidth, currentHeight, currentAsc := 0., 0., 0., 0.
	var currentStyle *dom.Style

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
	flushPenalty := func(flagged bool, hyphenW float64) {
		nodes = append(nodes, kpNode{
			t:       tpenalty,
			penalty: kpPenalty,
			flagged: flagged,
			width:   hyphenW,
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
		var newFragments []dom.TextNode
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

			newFrag := frag
			newFrag.Text = newText.String()
			newFrag.LangCode = frag.LangCode

			newFragments = append(newFragments, newFrag)
		}
		p.Fragments = newFragments

	}

	for _, frag := range p.Fragments {
		// on a new fragment, flush the previous run:
		flushRun()
		// set current style
		tmpStyle := frag.Style()
		currentStyle = &tmpStyle

		fs := frag.Style().FontStyle
		fsize := frag.Style().FontSize
		fraglhm := frag.Style().LineHeight // frag line-height multiplier
		if fraglhm == 0 {
			fraglhm = 1
		}
		font, ok := frag.Style().Font.Fmap[fs]
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
			currentAsc = max(
				currentAsc,
				fraglhm*frag.Style().LineHeight*float64(font.Metrics().Ascender)*fsize/unitsPerEm,
			)
			// lineHeight = LineGap + Ascender + abs(Descender)
			currentHeight = max(
				currentHeight,
				fraglhm*frag.Style().LineHeight*(float64(font.Metrics().LineGap+font.Metrics().Ascender)+math.Abs(float64(font.Metrics().Descender)))*fsize/unitsPerEm,
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
				flushPenalty(false, 0)
			case '\u00AD':
				// soft-hyphen (from hyphenation)
				// if flagged == true, we add hyphen on paint
				flushBox()
				flushPenalty(true, rw)
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

// follows kp main loop as defined in the paper
// active nodes represents the set of all plausible break-points that are considered valid by the algorithm
// if this set is empty (i.e., no break-point is found), we go for the node with the least demerits
// if an overfull break-point is also not possible, then the algorithm panics and exits
func kpMainLoop(
	ctx *dom.LayoutContext,
	p *dom.ParagraphNode,
	nodeHeightsST []float64,
	b int, nodes []kpNode,
	sumW, sumY, sumZ float64,
	activeNodes *kpLinkedList,
) {
	a := activeNodes.getFirst()
	var overfullNode *kpBreakPoint = nil
	for {
		// this outer for loop is redundant as we don't use the j < j0 check
		// as j0 here is "0" (we assume all lines to be of the same length, unless the TODO is completed)
		if activeNodes.isNil(a) {
			break
		}

		// list of all possible parents with least demerits for the given class
		candidates := []kpCandidate{{demerits: kpInfinity}, {demerits: kpInfinity}, {demerits: kpInfinity}, {demerits: kpInfinity}}
		D := kpInfinity

		for {
			// here we go through all the previously plausible break-points (nodes a_i)
			// if the current node b is a valid break-point for node a, we insert this in the activeNodes set with the calculated class (c)
			// thus for each new node b we find upto 4 possible new break-points (b_c0, b_c1, b_c2, b_c3), which are then inserted in the activeNodes set
			nextA := a.next

			// line width from a -> b
			La_b := sumW - a.data.sumW
			// actual width available (L)
			// checks the current cursor position for the line a->b and then checks if paragraphs width fits or not
			var prevY float64
			if a.data.index >= 0 {
				prevY = a.data.lineHeight
			} else {
				prevY = p.LayoutBox().Y
			}
			currLineH, ok := querySegmentTree(&nodeHeightsST, len(nodes), max(0, a.data.index+1), b)
			if !ok {
				// fallback to zero
				currLineH = 0
			}
			pageWidthAtCurrentPosition := dom.GetCurrentPageWidth(ctx, prevY+currLineH)
			L := min(pageWidthAtCurrentPosition, p.LayoutBox().Width) - (p.Style().Padding.Left + p.Style().Padding.Right)
			if p.ParagraphIndent != 0 && a.data.line != 0 {
				L -= float64(p.ParagraphIndent)
			}

			// adjustment ratio calculation
			r := 0.
			diff := L - La_b
			if diff > 0 {
				// line too short
				stretch := sumY - a.data.sumY
				if stretch > 0 {
					r = diff / stretch
				} else {
					r = kpInfinity
				}
			} else {
				// line too long
				shrink := sumZ - a.data.sumZ
				if shrink > 0 {
					r = diff / shrink
				} else {
					r = kpInfinity
				}
			}

			if r < -1 || (nodes[b].t == tpenalty && nodes[b].penalty == -kpInfinity) {
				// the line is too long, therefore we remove this break-point from activeNodes set,
				// this will prevent any future nodes from adopting this break-point as a parent
				// (re-inserted if activeNodes is empty)
				if overfullNode == nil || r > overfullNode.ratio {
					overfullNode = &kpBreakPoint{
						index:        b,
						parent:       a.data,
						fitnessClass: 3,
						line:         a.data.line + 1,
						demerits:     0,
						ratio:        r,
						sumW:         sumW,
						sumY:         sumY,
						sumZ:         sumZ,
					}
				}
				activeNodes.delete(a)
			}

			if r >= -1 && r < kpRho {
				// demerit calculation:
				badness := 100 * math.Pow(math.Abs(r), 3)
				currDemerits := 0.
				if nodes[b].t == tpenalty && nodes[b].penalty >= 0 {
					currDemerits = math.Pow(kpDemeritsLine+badness, 2) + math.Pow(nodes[b].penalty, 2)
				} else if nodes[b].t == tpenalty && nodes[b].penalty < 0 {
					currDemerits = math.Pow(kpDemeritsLine+badness, 2) - math.Pow(nodes[b].penalty, 2)
				} else {
					currDemerits = math.Pow(kpDemeritsLine+badness, 2)
				}

				if nodes[b].t == tpenalty && nodes[b].flagged &&
					nodes[a.data.index].t == tpenalty && nodes[a.data.index].flagged {
					currDemerits += kpDemeritsFlagged
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

				// if too much change in fitness class
				if math.Abs(float64(c-a.data.fitnessClass)) > 1 {
					currDemerits += kpDemeritsFitness
				}

				// if this is a better candidate for the algo
				if currDemerits < candidates[c].demerits {
					candidates[c] = kpCandidate{
						active:   a.data,
						ratio:    r,
						demerits: currDemerits,
					}
					if currDemerits < D {
						D = currDemerits
					}
				}
			}

			a = nextA
			if activeNodes.isNil(a) {
				break
			}
		}

		if D < kpInfinity {
			// calculate sums till next box or penalty:
			w, y, z := 0., 0., 0.
			for i := b; i < len(nodes); i++ {
				if nodes[i].t == tglue {
					w += nodes[i].width
					y += nodes[i].y
					z += nodes[i].z
				} else if nodes[i].t == tpenalty && nodes[i].flagged {
					w += nodes[i].width
				} else {
					break
				}
			}

			for fitnessClass := 0; fitnessClass < len(candidates); fitnessClass++ {
				candidate := candidates[fitnessClass]
				if candidate.demerits < kpInfinity {
					var prevY float64
					if candidate.active.index >= 0 {
						prevY = candidate.active.lineHeight
					} else {
						prevY = p.LayoutBox().Y
					}
					currLineH, ok := querySegmentTree(&nodeHeightsST, len(nodes), max(0, candidate.active.index+1), b)
					if !ok {
						currLineH = 0
					}
					newActiveNode := &kpBreakPoint{
						index:        b,
						parent:       candidate.active,
						fitnessClass: fitnessClass,
						line:         candidate.active.line + 1,
						lineHeight:   prevY + currLineH,
						demerits:     candidate.demerits,
						ratio:        candidate.ratio,
						sumW:         sumW + w,
						sumY:         sumY + y,
						sumZ:         sumZ + z,
					}
					if activeNodes.isNil(a) {
						activeNodes.pushBack(&kpLinkedListNode{
							data: newActiveNode,
						})
					} else {
						activeNodes.insertBefore(a, &kpLinkedListNode{
							data: newActiveNode,
						})
					}
				}
			}
		}
	}

	if activeNodes.size == 0 {
		if overfullNode != nil {
			activeNodes.pushBack(&kpLinkedListNode{data: overfullNode})
		} else {
			panic("no feasible breakpoints and no overfull fallback found")
		}
	}
}

func KpLineBreak(p *dom.ParagraphNode, ctx *dom.LayoutContext) ([]*line, error) {
	// create kp nodes:
	nodes, err := createKpNodes(p)
	if err != nil {
		return nil, err
	}

	nodeHeightsST := createSegmentTree(&nodes)

	sumW, sumY, sumZ := 0., 0., 0.

	activeNodes := newList()
	activeNodes.pushBack(&kpLinkedListNode{
		data: &kpBreakPoint{
			index:        -1,
			parent:       nil,
			line:         -1,
			fitnessClass: 0,
			demerits:     0,
			ratio:        0,
		},
	})

	for b := range nodes {
		if nodes[b].t == tbox {
			sumW += nodes[b].width
		} else if nodes[b].t == tglue {
			if b > 0 && nodes[b-1].t == tbox {
				kpMainLoop(ctx, p, nodeHeightsST, b, nodes, sumW, sumY, sumZ, activeNodes)
			}
			sumW += nodes[b].width
			sumY += nodes[b].y
			sumZ += nodes[b].z
		} else if nodes[b].t == tpenalty && nodes[b].penalty != kpInfinity {
			kpMainLoop(ctx, p, nodeHeightsST, b, nodes, sumW, sumY, sumZ, activeNodes)
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
	cx, cy := (p.LayoutBox().X + p.Style().Padding.Left), (p.LayoutBox().Y + p.Style().Padding.Top)
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
					fasc := float64(font.Metrics().Ascender) * run.style.FontSize / float64(font.Metrics().UnitsPerEm)
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
							hyphenWidth = float64(hyphenW) * lastRun.style.FontSize / unitsPerEm
						}
						fasc := float64(font.Metrics().Ascender) * lastRun.style.FontSize / unitsPerEm
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
		cx = (p.LayoutBox().X + p.Style().Padding.Left)
	}

	return lines, nil
}

// ! utility functions for querying heights:

func createSegmentTree(nodes *[]kpNode) []float64 {
	n := len(*nodes)
	stlen := len(*nodes) * 4
	st := make([]float64, stlen)
	createSTHelper(nodes, &st, 0, n-1, 0)
	return st
}

func createSTHelper(nodes *[]kpNode, st *[]float64, ss, se, si int) float64 {
	if ss > se {
		return -1
	}
	if ss == se {
		(*st)[si] = (*nodes)[ss].height
		return (*st)[si]
	}
	smid := ss + ((se - ss) >> 1)
	(*st)[si] = max(
		createSTHelper(nodes, st, ss, smid, si*2+1),
		createSTHelper(nodes, st, smid+1, se, si*2+2),
	)
	return (*st)[si]
}

func querySegmentTree(st *[]float64, nodesLen, l, r int) (float64, bool) {
	if l > r || l < 0 || r > nodesLen {
		return -1, false
	}
	return querySTHelper(st, l, r, 0, len(*st)-1, 0)
}

func querySTHelper(st *[]float64, l, r, ss, se, si int) (float64, bool) {
	if se < l || ss > r {
		return 0, false
	}
	if l <= ss && se <= r {
		return (*st)[si], true
	}
	smid := ss + ((se - ss) >> 1)
	lv, lok := querySTHelper(st, l, r, ss, smid, 2*si+1)
	rv, rok := querySTHelper(st, l, r, smid+1, se, 2*si+2)
	switch {
	case lok && rok:
		return max(lv, rv), true
	case lok:
		return lv, true
	case rok:
		return rv, true
	default:
		return 0, false
	}
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
