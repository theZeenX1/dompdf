package dom

import "fmt"

const (
	kpinfinity    = 10000.
	kpforcedbreak = -10000.
	kppenalty     = 50.
)

type kpNodeType int16

const (
	tbox kpNodeType = iota
	tglue
	tpenalty
)

type textrun struct {
	style *Style
	text  string
}

type kpNode struct {
	t             kpNodeType
	runs          []textrun
	width, height float64
	y, z          float64
	penalty       float64
	flagged       bool
}

type kpCandidate struct {
	index        int
	fitnessClass int
	line         int
	ratio        float64
	demerits     float64
	prev         *kpCandidate
	next         *kpCandidate
}

type kpDemerits struct {
	d0, d1, d2, d3 float64
}

type line struct {
}

func (p *ParagraphNode) createKpNodes() ([]kpNode, error) {
	// store all nodes for kp algorithm
	nodes := []kpNode{}
	// store current run details:
	currentRuns := []textrun{}
	var currentRun []rune
	currentWidth, currentHeight := 0., 0.
	var currentStyle *Style

	flushRun := func() {
		if currentRun == nil || len(currentRun) == 0 {
			return
		}

		currentRuns = append(currentRuns, textrun{
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
			penalty: kppenalty,
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
			case '-':
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
		penalty: kpforcedbreak,
		flagged: false,
	})

	return nodes, nil
}

func kpMainLoop(index int, nodes []kpNode, sumW, sumY, sumZ, totalDemerits float64) float64 {
	demerits := kpDemerits{d0: kpinfinity, d1: kpinfinity, d2: kpinfinity, d3: kpinfinity}
	return 0.
}

func (p *ParagraphNode) kpLineBreak(ctx LayoutContext) ([]line, error) {
	nodes, err := p.createKpNodes()
	if err != nil {
		return nil, err
	}

	sumW, sumY, sumZ := 0., 0., 0.
	totalDemerits := kpinfinity

	activeNode := activeNode{
		// TODO
	}

	for i := range nodes {
		if nodes[i].t == tbox {
			sumW += nodes[i].width
		} else if nodes[i].t == tglue {
			if i > 0 && nodes[i-1].t == tbox {
				totalDemerits += kpMainLoop(i, nodes, sumW, sumY, sumZ, totalDemerits)
			}
			sumW += nodes[i].width
			sumY += nodes[i].y
			sumZ += nodes[i].z
		} else if nodes[i].t == tpenalty && nodes[i].penalty != kpinfinity {
			totalDemerits += kpMainLoop(i, nodes, sumW, sumY, sumZ, totalDemerits)
		}
	}

	return []line{}, nil
}
