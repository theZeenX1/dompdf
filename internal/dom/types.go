package dom

import (
	"github.com/theZeenX1/dompdf/internal/colors"
	"github.com/theZeenX1/dompdf/internal/fonts"
	"github.com/theZeenX1/dompdf/internal/image"
	"github.com/theZeenX1/dompdf/internal/pdf"
)

type DOMElement interface {
	// Justify itself in the page
	Layout(ctx LayoutContext)
}

type DOMNode interface {
	//
	Layout(ctx LayoutContext)
	// create stream objects from the given dom node
	ToStream() ([]*pdf.StreamObj, []*pdf.Ref, error)
}

type MediaBox struct {
	LLX, LLY, Width, Height float64
}

type BoxCoordinates struct {
	Top, Left, Right, Bottom float64
}

type DOMPage struct {
	PageNo   int
	MediaBox MediaBox
	Child    *DOMElement
}

type LayoutContext struct {
	CurrentPageNo    int
	CurrentNodeId    int
	CursorX, CursorY float64
	Pages            []*DOMPage
}

type ElementBorder struct {
	LBorder, RBorder, TBorder, BBorder float64
	Thickness                          float64
	Color                              colors.Color
}

// Element detail:
type ElementLayoutDetail struct {
	NodeId   int
	Position BoxCoordinates
	ZIndex   int
}

// DOMElements:
type Container struct {
	ElementLayoutDetail

	Padding BoxCoordinates
	Margin  BoxCoordinates
	BgColor colors.Color
	Child   *DOMElement
}

type FlexItem struct {
	ElementLayoutDetail

	AlignSelf   Align
	JustifySelf Justify
	Child       *DOMElement
}

type Flex struct {
	ElementLayoutDetail

	FlexDirection  FlexDirection
	Gap            float64
	AlignItems     Align
	JustifyContent Justify
	Children       []*FlexItem
}

type Grid struct {
	ElementLayoutDetail
}

type Table struct {
	ElementLayoutDetail
}

// DOMNodes:
type TextNode struct {
	ElementLayoutDetail

	Text      string
	Color     colors.Color
	Font      *fonts.Font
	FontStyle fonts.FontStyle
}

type AnnotationNode struct {
	ElementLayoutDetail

	TextNode TextNode
	Href     string
}

type ImageNode struct {
	ElementLayoutDetail

	Details image.Image
}
