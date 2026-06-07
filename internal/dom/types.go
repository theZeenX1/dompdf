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
	LayoutBox() *LayoutBox
	//
	NodePointers() *NodePointers
	//
	Style() *Style
	//
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
	DOMElement
	//
	inline()
}

type BoxCoordinates struct {
	Top, Left, Right, Bottom float64
}

type DOMPage struct {
	PageNo int
	Width  int
	Height int
	Child  DOMElement
}

type LayoutContext struct {
	CurrentPageNo    int
	CurrentNodeId    int
	CursorX, CursorY float64
	Pages            []*DOMPage
	ElementCache     map[int]*DOMElement
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
	FontSize        int
	FontStyle       fonts.FontStyle
	FontWeight      fonts.FontWeight
}

type LayoutBox struct {
	X, Y          float64
	Width, Height float64
}

// DOMElements:
type Container struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	Child DOMElement
}

func (c *Container) block() {}

type FlexItem struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	AlignSelf   Align
	JustifySelf Justify
	Child       DOMElement
}

type Flex struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	FlexDirection  FlexDirection
	Gap            float64
	AlignItems     Align
	JustifyContent Justify
	Children       []FlexItem
}

func (f *Flex) block() {}

// auto flows nodes from one "pipe" to the next.
type AutoFlow struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	FlowDirection  FlexDirection
	PipeCount      int16
	Gap            float64
	AlignItems     Align
	JustifyContent Justify
	Children       []FlexItem
}

func (a *AutoFlow) block() {}

type Table struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

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
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	ColSpan int
	RowSpan int

	Child DOMElement
}

type FloatBox struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	Children []FloatItem
}

type FloatItem struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	FloatLayout FloatLayout
	Child       InlineElement
}

type ParagraphNode struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	TextAlign       TextAlign
	ParagraphIndent int
	JustifiedText   bool

	WordWrap    bool
	Hyphenation bool

	Fragments []TextNode
}

func (p *ParagraphNode) block() {}

// Render Nodes:
type TextNode struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	Text string
}

func (t *TextNode) Layout(ctx LayoutContext)                        {}
func (t *TextNode) NodePointers() *NodePointers                     { return &t.nodePointers }
func (t *TextNode) LayoutBox() *LayoutBox                           { return &t.layoutBox }
func (t *TextNode) Style() *Style                                   { return &t.style }
func (t *TextNode) ToStream() ([]*pdf.StreamObj, []*pdf.Ref, error) { return nil, nil, nil }
func (t *TextNode) inline()                                         {}

type ImageNode struct {
	layoutBox    LayoutBox
	nodePointers NodePointers
	style        Style

	image.Image
}

func (i *ImageNode) Layout(ctx LayoutContext)                        {}
func (i *ImageNode) NodePointers() *NodePointers                     { return &i.nodePointers }
func (i *ImageNode) LayoutBox() *LayoutBox                           { return &i.layoutBox }
func (i *ImageNode) Style() *Style                                   { return &i.style }
func (i *ImageNode) ToStream() ([]*pdf.StreamObj, []*pdf.Ref, error) { return nil, nil, nil }
func (i *ImageNode) inline()                                         {}
