package paragraph

import (
	"fmt"

	"github.com/theZeenX1/dompdf/internal/v0/dom"
)

// mnWidth -> least width of the paragraph element without overflow (i.e., largest word)
//
// mxWidth -> total length of the paragraph
func MinMaxWidths(p *dom.ParagraphNode) (mnWidth, mxWidth float64) {
	currW := 0.

	for _, frag := range p.Fragments {
		style := frag.Style()
		fsize := style.FontSize
		fstyle := style.FontStyle
		font, ok := style.Font.Fmap[fstyle]
		if !ok {
			panic(fmt.Sprintf("font style %d not available", fstyle))
		}
		unitsPerEm := float64(font.Metrics().UnitsPerEm)
		if unitsPerEm == 0 {
			unitsPerEm = 1000.
		}
		whiteSpaceW, ok := font.GlyphAdvancedWidth(' ')
		if !ok {
			panic(fmt.Sprintf("whitespace not present in glyph advanced width for the font: %s", font.FontName()))
		}

		for _, r := range frag.Text {
			rw := 0.
			v, ok := font.GlyphAdvancedWidth(r)
			if !ok {
				rw = float64(whiteSpaceW) * fsize / unitsPerEm
			} else {
				rw = float64(v) * fsize / unitsPerEm
			}

			switch r {
			case ' ':
				currW = 0
				mxWidth += rw
			default:
				currW += rw
				mxWidth += rw
				mnWidth = max(mnWidth, currW)
			}
		}
	}

	return
}

// Justify the widths of this element on the basis of MinMaxWidths function by the parent node
//
// If parent node is non-existent, we assume paragraph's width to be equal to the page width - page margins
func Layout(p *dom.ParagraphNode, ctx dom.LayoutContext) {

}
