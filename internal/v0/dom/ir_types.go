package dom

import "github.com/theZeenX1/dompdf/internal/v0/pdf"

// Intermediate representation of DOM Elements before turning them to PDF objects
type IR interface {
	ToPDFStream() ([]*pdf.StreamObj, []*pdf.Ref, error)
}

type TextIR struct{}

type ImageIR struct{}

type AnnotationIR struct{}

type ShapeIR struct{}
