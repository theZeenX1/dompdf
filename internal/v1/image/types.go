package image

import (
	"github.com/theZeenX1/dompdf/internal/v1/colors"
	"github.com/theZeenX1/dompdf/internal/v1/pdf"
)

type Image struct {
	Width, Height    int
	Type             ImageType
	ColorSpace       colors.ColorSpace
	BitsPerComponent int
	Data             []byte
	// if alpha is present, ToStream outputs two objects
	Alpha []byte
}

// stream objects for PDF
//
// if alpha != nil, per pixel alpha is to be stored in a new object
// i.e., two objects, one with pixel data and one with alpha data
// (read more: pdf 1.7 spec -> 11.6.5 Transparency)
func (d *Image) ToStream() ([]*pdf.StreamObj, []*pdf.Ref, error) {
	return nil, nil, nil
}
