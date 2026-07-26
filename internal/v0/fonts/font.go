package fonts

// metrics holds font metric information in font design units.
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
	GlyphAdvancedWidth(r rune) (int, bool)
	// encodes to PDF byte data
	Encode(text string) []byte
	// returns a PDF byte font subset
	Subset(runes []rune) ([]byte, error)
}

// FontStyle represents font style flags as bit positions.
// Multiple styles can be combined using bitwise OR.
type FontStyle int16

const (
	FSNormal FontStyle = 1 << iota
	FSBold
	FSItalic
	FSUnderlined
	FSStrikethrough
)

// RegisteredFont stores font variants indexed by style flags.
//
// Example:
//
//	times.Fmap[FSBold|FSItalic]
//
// retrieves the bold-italic variant.
type RegisteredFont struct {
	Fmap map[FontStyle]Font
}

type FontWeight int16

const (
	FWThin   FontWeight = 200
	FWNormal FontWeight = 400
	FWBold   FontWeight = 700

	FW100 FontWeight = 100
	FW200 FontWeight = 200
	FW300 FontWeight = 300
	FW400 FontWeight = 400
	FW500 FontWeight = 500
	FW600 FontWeight = 600
	FW700 FontWeight = 700
	FW800 FontWeight = 800
	FW900 FontWeight = 900
)
