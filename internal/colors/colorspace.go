package colors

// SRC: https://github.com/gpdf-dev/gpdf

import "fmt"

// ColorSpace identifies the color model used by a Color value.
type ColorSpace int

const (
	// ColorSpaceRGB represents the RGB color space.
	ColorSpaceRGB ColorSpace = iota
	// ColorSpaceGray represents the grayscale color space.
	ColorSpaceGray
	// ColorSpaceCMYK represents the CMYK color space.
	ColorSpaceCMYK
)

// Color represents a color in one of the supported color spaces.
// For RGB, use R, G, B in the range 0.0 to 1.0.
// For Gray, use R as the gray value (0.0 = black, 1.0 = white).
// For CMYK, use R=C, G=M, B=Y, A=K (all 0.0 to 1.0), where A is
// repurposed as the K channel.
type Color struct {
	R, G, B float64    // For RGB; for Gray use R only; for CMYK: C, M, Y
	A       float64    // Alpha (1.0 = opaque); for CMYK this is the K channel
	Space   ColorSpace // Color space identifier
}

// ---------------------------------------------------------------------------
// Convenience constructors
// ---------------------------------------------------------------------------

// RGB creates an RGB color with the given red, green, and blue components.
// All values are in the range 0.0 to 1.0.
func RGB(r, g, b float64) Color {
	return Color{R: r, G: g, B: b, A: 1.0, Space: ColorSpaceRGB}
}

// RGBHex creates an RGB color from a 24-bit hex value.
// For example, 0xFF0000 produces red.
func RGBHex(hex uint32) Color {
	r := float64((hex>>16)&0xFF) / 255.0
	g := float64((hex>>8)&0xFF) / 255.0
	b := float64(hex&0xFF) / 255.0
	return RGB(r, g, b)
}

// Gray creates a grayscale color. v ranges from 0.0 (black) to 1.0 (white).
func Gray(v float64) Color {
	return Color{R: v, G: v, B: v, A: 1.0, Space: ColorSpaceGray}
}

// CMYK creates a CMYK color. All values are in the range 0.0 to 1.0.
func CMYK(c, m, y, k float64) Color {
	return Color{R: c, G: m, B: y, A: k, Space: ColorSpaceCMYK}
}

// ---------------------------------------------------------------------------
// Predefined colors
// ---------------------------------------------------------------------------

var (
	// Black is a predefined black color (grayscale 0).
	Black = Gray(0)
	// White is a predefined white color (grayscale 1).
	White = Gray(1)
	// Red is a predefined red color.
	Red = RGB(1, 0, 0)
	// Green is a predefined green color.
	Green = RGB(0, 1, 0)
	// Blue is a predefined blue color.
	Blue = RGB(0, 0, 1)
	// Yellow is a predefined yellow color.
	Yellow = RGB(1, 1, 0)
	// Cyan is a predefined cyan color.
	Cyan = RGB(0, 1, 1)
	// Magenta is a predefined magenta color.
	Magenta = RGB(1, 0, 1)
)

// ---------------------------------------------------------------------------
// PDF color operators
// ---------------------------------------------------------------------------

// StrokeColorCmd returns the PDF operator string to set this color as the
// current stroking color. The result depends on the color space:
//
//	RGB:  "r g b RG"
//	Gray: "g G"
//	CMYK: "c m y k K"
func (c Color) StrokeColorCmd() string {
	switch c.Space {
	case ColorSpaceGray:
		return fmt.Sprintf("%g G", c.R)
	case ColorSpaceCMYK:
		return fmt.Sprintf("%g %g %g %g K", c.R, c.G, c.B, c.A)
	default: // RGB
		return fmt.Sprintf("%g %g %g RG", c.R, c.G, c.B)
	}
}

// FillColorCmd returns the PDF operator string to set this color as the
// current non-stroking (fill) color. The result depends on the color space:
//
//	RGB:  "r g b rg"
//	Gray: "g g"
//	CMYK: "c m y k k"
func (c Color) FillColorCmd() string {
	switch c.Space {
	case ColorSpaceGray:
		return fmt.Sprintf("%g g", c.R)
	case ColorSpaceCMYK:
		return fmt.Sprintf("%g %g %g %g k", c.R, c.G, c.B, c.A)
	default: // RGB
		return fmt.Sprintf("%g %g %g rg", c.R, c.G, c.B)
	}
}
