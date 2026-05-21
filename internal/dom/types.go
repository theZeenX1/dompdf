package dom

import (
	"github.com/theZeenX1/dompdf/internal/colors"
	"github.com/theZeenX1/dompdf/internal/fonts"
	"github.com/theZeenX1/dompdf/internal/image"
	"github.com/theZeenX1/dompdf/internal/pdf"
)

type DOMElement interface {
	// justify itself in the page and set the coordinates, height, etc.
	Layout(ctx LayoutContext)
	//
	NodePointers() *NodePointers
	//
	Style() *Style
}

type RendderNode interface {
	DOMElement
	// create stream objects from the given dom node
	ToStream() ([]*pdf.StreamObj, []*pdf.Ref, error)
}

// block component
type BlockElement interface {
	DOMElement
	//
	block()
}

// inline component (is also a render node)
type InlineElement interface {
	RendderNode
	//
	inline()
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
	Child    DOMElement
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
	BorderRadiusType                   MeasurementType
	BorderRadius                       float64
	BorderStyle                        BorderStyle
	Color                              colors.Color
}

// Commons:
type NodePointers struct {
	nodeId       int
	parentNodeId int
	nextNodeId   int
	prevNodeId   int
}

type Style struct {
	Position        BoxCoordinates
	Height, Width   float64
	Padding, Margin BoxCoordinates
	ZIndex          int
	BgColor         colors.Color
	Border          ElementBorder
	Color           colors.Color
	Font            *fonts.RegisteredFont
	FontSize        float64
	FontStyle       fonts.FontStyle
	FontWeight      fonts.FontWeight
}

type LayoutBox struct {
	X, Y          float64
	Width, Height float64
}

// DOMElements:
type Container struct {
	LayoutBox
	NodePointers
	Style

	Child DOMElement
}

func (c *Container) block() {}

type FlexItem struct {
	LayoutBox
	NodePointers
	Style

	AlignSelf   Align
	JustifySelf Justify
	Child       DOMElement
}

type Flex struct {
	LayoutBox
	NodePointers
	Style

	FlexDirection  FlexDirection
	Gap            float64
	AlignItems     Align
	JustifyContent Justify
	Children       []FlexItem
}

func (f *Flex) block() {}

// auto flows nodes from one "pipe" to the next.
type AutoFlow struct {
	LayoutBox
	NodePointers
	Style

	FlowDirection  FlexDirection
	PipeCount      int16
	Gap            float64
	AlignItems     Align
	JustifyContent Justify
	Children       []FlexItem
}

func (a *AutoFlow) block() {}

type Table struct {
	LayoutBox
	NodePointers
	Style

	Columns []TableColumn
	Rows    []TableRow

	BorderCollapse bool
	CellSpacing    float64
}

func (t *Table) block() {}

type TableColumn struct {
	Width float64
}

type TableRow struct {
	Height float64
	Cells  []TableCell
}

type TableCell struct {
	LayoutBox
	NodePointers
	Style

	ColSpan int
	RowSpan int

	Child DOMElement
}

// Render Nodes:

type ParagraphNode struct {
	LayoutBox
	NodePointers
	Style

	TextAlign   TextAlign
	LineHeight  float64
	TextIndent  float64
	WordWrap    bool
	Hyphenation bool

	Inlines []InlineElement
}

func (p *ParagraphNode) block() {}

type TextNode struct {
	LayoutBox
	NodePointers
	Style

	Text string
}

func (t TextNode) Layout(ctx LayoutContext) {}
func (t TextNode) NodePointers()            { return &t.NodePointers() }
func (t TextNode) inline()                  {}

type AnnotationNode struct {
	LayoutBox
	NodePointers
	Style

	TextNode TextNode
	Href     string
}

func (a *AnnotationNode) inline() {}

type ImageNode struct {
	LayoutBox
	NodePointers
	Style

	Details image.Image
}

func (i *ImageNode) inline() {}
