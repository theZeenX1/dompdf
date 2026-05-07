package fonts

// Metrics holds font metric information in font design units.
type Metrics struct {
	UnitsPerEm  int
	Ascender    int // in font units (positive, above baseline)
	Descender   int // in font units (negative, below baseline)
	LineGap     int
	CapHeight   int
	XHeight     int
	ItalicAngle float64
}

type Font interface {
	// return stored font name
	FontName() string
	// metrics
	Metrics() Metrics
	// in em
	GlyphAdvancedWidth(r rune) (float64, bool)
	// encodes to PDF byte data
	Encode(text string) []byte
	// returns a PDF byte font subset
	Subset(runes []rune) ([]byte, error)
}

type FontStyle int16

const (
	FSBold FontStyle = iota
	FSItalic
	FSUnderlined
	FSStrikethrough
)
