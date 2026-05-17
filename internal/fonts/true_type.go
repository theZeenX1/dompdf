package fonts

import "sort"

const (
	sntfVerTrueType uint32 = 0x00010000
	sntfVerTrue     uint32 = 0x74727565
	sntfVerOTTO     uint32 = 0x4F54544F

	tagHead string = "head"
	tagHhea string = "hhea"
	tagMaxp string = "maxp"
	tagOS2  string = "OS/2"
	tagHmtx string = "hmtx"
	tagName string = "name"
	tagPost string = "post"
	tagCmap string = "cmap"
	tagGlyf string = "glyf"
	tagLoca string = "loca"
)

type sntfHeader struct {
	sntfVersion   uint32 // "true" "otto" "0x00010000"
	numTables     uint16
	searchRange   uint16
	entrySelector uint16
	rangeShift    uint16
}

type fontTable struct {
	tag      [4]byte
	checkSum uint32
	offset   uint32
	length   uint32
}

// CMAP table, data regarding glyphs
type cmapSubtable struct {
	platformId         uint16
	platformSpecificId uint16
	offset             uint32
}

type cmapTbl struct {
	format4  *cmapFormat4  // BMP mapping (format 4)
	format12 *cmapFormat12 // Full Unicode mapping (format 12)
}

func (c *cmapTbl) lookup(r rune) (uint16, bool) {
	cp := uint32(r)
	// Prefer format 12 for full Unicode coverage.
	if c.format12 != nil {
		for _, g := range c.format12.groups {
			if cp >= g.startCharCode && cp <= g.endCharCode {
				return uint16(g.startGlyphID + (cp - g.startCharCode)), true
			}
		}
	}
	// Fall back to format 4 for BMP.
	if c.format4 != nil && cp <= 0xFFFF {
		return c.format4.lookup(uint16(cp)), true
	}
	return 0, false
}

type cmapFormat4 struct {
	segCount       int
	endCodes       []uint16
	startCodes     []uint16
	idDeltas       []int16
	idRangeOffsets []uint16
	glyphIDArray   []uint16
	// Raw offset where idRangeOffsets begin, used for glyph ID calculation.
	idRangeOffsetBase int
}

func (f4 *cmapFormat4) lookup(cp uint16) uint16 {
	// Binary search for the segment containing cp.
	idx := sort.Search(f4.segCount, func(i int) bool {
		return f4.endCodes[i] >= cp
	})
	if idx >= f4.segCount {
		return 0
	}
	if cp < f4.startCodes[idx] {
		return 0
	}

	if f4.idRangeOffsets[idx] == 0 {
		return uint16(int16(cp) + f4.idDeltas[idx])
	}

	// Use idRangeOffset to index into glyphIDArray.
	// offset = idRangeOffset[idx]/2 + (cp - startCode[idx]) - (segCount - idx)
	glyphIdx := int(f4.idRangeOffsets[idx]/2) + int(cp-f4.startCodes[idx]) - (f4.segCount - idx)
	if glyphIdx < 0 || glyphIdx >= len(f4.glyphIDArray) {
		return 0
	}
	gid := f4.glyphIDArray[glyphIdx]
	if gid == 0 {
		return 0
	}
	return uint16(int16(gid) + f4.idDeltas[idx])
}

type cmapFormat12 struct {
	groups []cmapFormat12Group
}

type cmapFormat12Group struct {
	startCharCode uint32
	endCharCode   uint32
	startGlyphID  uint32
}

// advanced horizontal widths
type hmtxTbl struct {
	advanceWidths    []uint16
	leftSideBearings []int16
}

type TrueTypeFont struct {
	name       string
	dat        []byte
	tables     map[string]fontTable
	metrics    Metrics
	unitsPerEm int
	cmapTbl    cmapTbl
	hmtxTbl    hmtxTbl
	numGlyphs  int
	usedRunes  map[rune]bool
	kern       map[runepair]int
}

// TTF interfaces:
// Name returns the PostScript name of the font.
func (ttf *TrueTypeFont) Name() string {
	return ttf.name
}

// Metrics returns the font's metric information.
func (ttf *TrueTypeFont) Metrics() Metrics {
	return ttf.metrics
}

// GlyphWidth returns the advance width of the glyph for the given rune
// in font design units.
func (ttf *TrueTypeFont) GlyphWidth(r rune) (int, bool) {
	gid, ok := ttf.cmapTbl.lookup(r)
	if !ok || int(gid) >= len(ttf.hmtxTbl.advanceWidths) {
		return 0, false
	}
	return int(ttf.hmtxTbl.advanceWidths[gid]), true
}

// GlyphID returns the glyph ID for a rune, or 0 if not found.
func (ttf *TrueTypeFont) GlyphID(r rune) uint16 {
	gid, ok := ttf.cmapTbl.lookup(r)
	if !ok {
		return 0
	}
	return gid
}

// Encode encodes text into a byte sequence for PDF content streams.
// For identity-encoded TrueType fonts, each character is mapped to its
// glyph ID and encoded as a big-endian uint16.
func (ttf *TrueTypeFont) Encode(text string) []byte {
	var result []byte
	for _, r := range text {
		ttf.usedRunes[r] = true
		gid, ok := ttf.cmapTbl.lookup(r)
		if !ok {
			gid = 0
		}
		result = append(result, byte(gid>>8), byte(gid&0xFF))
	}
	return result
}

// Subset creates a subsetted font containing only the specified runes.
// It delegates to SubsetTrueType with the appropriate glyph IDs.
func (ttf *TrueTypeFont) Subset(runes []rune) ([]byte, error) {
	glyphIDs := make([]uint16, 0, len(runes)+1)
	glyphIDs = append(glyphIDs, 0) // always include .notdef

	seen := make(map[uint16]bool)
	seen[0] = true
	for _, r := range runes {
		gid, ok := ttf.cmapTbl.lookup(r)
		if ok && !seen[gid] {
			glyphIDs = append(glyphIDs, gid)
			seen[gid] = true
		}
	}

	return ttf.SubsetTrueType(glyphIDs)
}

// UsedRunes returns the set of runes that have been encoded so far.
func (ttf *TrueTypeFont) UsedRunes() map[rune]bool {
	result := make(map[rune]bool, len(ttf.usedRunes))
	for r := range ttf.usedRunes {
		result[r] = true
	}
	return result
}

// RuneToGID returns a mapping from rune to glyph ID for all used runes.
func (ttf *TrueTypeFont) RuneToGID() map[rune]uint16 {
	result := make(map[rune]uint16, len(ttf.usedRunes))
	for r := range ttf.usedRunes {
		gid, ok := ttf.cmapTbl.lookup(r)
		if ok {
			result[r] = gid
		}
	}
	return result
}

// NumGlyphs returns the total number of glyphs in the font.
func (ttf *TrueTypeFont) NumGlyphs() int {
	return ttf.numGlyphs
}

// Data returns the original font file data.
func (ttf *TrueTypeFont) Data() []byte {
	return ttf.dat
}
