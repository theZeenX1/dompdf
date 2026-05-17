package fonts

import (
	"encoding/binary"
	"fmt"
)

// gpdf implementation of subsetting
// zeros out unused glyphs and returns ttf font data with recalculated signatures
func (ttf *TrueTypeFont) SubsetTrueType(gids []uint16) ([]byte, error) {
	dat := ttf.dat
	if len(dat) < 12 {
		return nil, fmt.Errorf("font: dat length too short")
	}

	glyfSubtable, hasGlyf := ttf.tables[tagGlyf]
	locaSubtable, hasLoca := ttf.tables[tagLoca]
	headSubtable, hasHead := ttf.tables[tagHead]

	if !hasGlyf || !hasLoca || !hasHead {
		return nil, fmt.Errorf("font: subtables not present in font data")
	}

	// loca format (0 -> uint16, 1 -> uint32)
	indexToLocFormat := int16(0)
	locaOff := int(headSubtable.offset) + 50
	if locaOff+2 <= len(ttf.dat) {
		indexToLocFormat = int16(binary.BigEndian.Uint16(ttf.dat[locaOff : locaOff+2]))
	}
	// not using maxp numGlyphs (ttf.numGlyphs)
	// loca size -> numGlyphs + 1
	// uint16 -> 2 bytes per offset
	numGlyphs := int(locaSubtable.length)/2 - 1
	if indexToLocFormat == 1 {
		// uint32 -> 4 bytes per offset
		numGlyphs = int(locaSubtable.length)/4 - 1
	}

	// contains all glyphs and composites for the glyphs
	allGids := map[uint16]bool{}

	// fill allGids with the glyphs present in the arguments
	for _, gid := range gids {
		allGids[gid] = true
	}

	getGlyphOffsets := func(gid uint16) (int, int) {
		glyfBase := int(glyfSubtable.offset)
		glyfEnd := glyfBase + int(glyfSubtable.length)

		offStart, offEnd := glyfBase, glyfBase
		locaBase := int(locaSubtable.offset)

		if indexToLocFormat == 0 {
			// location of glyf offset is stored at gid * 2 (uint16)
			glyfIdx := locaBase + int(gid)*2
			if glyfIdx+4 > len(ttf.dat) {
				return glyfBase, glyfBase
			}

			// dat * 2 -> uint16
			offStart += int(binary.BigEndian.Uint16(ttf.dat[glyfIdx:glyfIdx+2])) * 2
			offEnd += int(binary.BigEndian.Uint16(ttf.dat[glyfIdx+2:glyfIdx+4])) * 2
		} else {
			// location of glyf offset is stored at gid * 4 (uint32)
			glyfIdx := locaBase + int(gid)*4
			if glyfIdx+8 > len(ttf.dat) {
				return glyfBase, glyfBase
			}

			// dat * 4 -> uint32
			offStart += int(binary.BigEndian.Uint32(ttf.dat[glyfIdx : glyfIdx+4]))
			offEnd += int(binary.BigEndian.Uint32(ttf.dat[glyfIdx+4 : glyfIdx+8]))
		}

		// clamp
		if offStart > glyfEnd {
			offStart = glyfEnd
		}
		if offEnd > glyfEnd {
			offEnd = glyfEnd
		}
		if offEnd < offStart {
			offEnd = offStart
		}

		return offStart, offEnd
	}

	// if numberOfContours in glyfSubtable == -1, the glyph is a composite glyph
	// recursively fill composites for all existing gids
	for {
		added := false

		for gid := range allGids {
			if int(gid) >= numGlyphs {
				continue
			}
			offStart, offEnd := getGlyphOffsets(gid)

			if offStart >= offEnd {
				continue
			}

			if offStart+2 > len(ttf.dat) {
				continue
			}

			numberOfContours := int16(binary.BigEndian.Uint16(ttf.dat[offStart : offStart+2]))
			// simple glyph
			if numberOfContours >= 0 {
				continue
			}

			// component gids slice:
			components := []uint16{}
			// skip glyf table headers:
			pos := offStart + 10
			// get all component gids:
			for pos+4 <= offEnd {
				flags := binary.BigEndian.Uint16(ttf.dat[pos : pos+2])
				componentGID := binary.BigEndian.Uint16(ttf.dat[pos+2 : pos+4])
				components = append(components, componentGID)
				pos += 4
				// get pos skip length:

				// arg size
				if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
					pos += 4
				} else {
					pos += 2
				}
				// transform data size
				switch {
				case flags&0x0008 != 0: // WE_HAVE_A_SCALE
					pos += 2
				case flags&0x0040 != 0: // WE_HAVE_AN_X_AND_Y_SCALE
					pos += 4
				case flags&0x0080 != 0: // WE_HAVE_A_TWO_BY_TWO
					pos += 8
				default:
					pos += 0
				}

				if flags&0x0020 == 0 { // MORE_COMPONENTS flag
					break
				}
			}

			// add component gids:
			for _, cid := range components {
				if !allGids[cid] {
					allGids[cid] = true
					added = true
				}
			}
		}

		if !added {
			break
		}
	}

	// create a data copy for subset:
	result := make([]byte, len(ttf.dat))
	copy(result, ttf.dat)

	// zero all unused glyfs
	for gid := uint16(0); int(gid) < numGlyphs; gid++ {
		if allGids[gid] {
			continue
		}

		offStart, offEnd := getGlyphOffsets(gid)
		offEnd = min(offEnd, len(result))
		if offStart >= offEnd {
			continue
		}

		for i := offStart; i < offEnd; i++ {
			result[i] = 0
		}
	}

	// recalc signatures
	// per table:
	offset := 12
	numTables := binary.BigEndian.Uint16(result[4:6])
	for i := 0; i < int(numTables); i++ {
		if offset+16 > len(result) {
			break
		}
		tableOff := binary.BigEndian.Uint32(result[offset+8 : offset+12])
		tableLen := binary.BigEndian.Uint32(result[offset+12 : offset+16])

		checkSum := uint32(0)
		end := tableOff + ((tableLen + 3) &^ 3)
		if int(end) > len(result) {
			end = uint32(len(result))
		}

		for j := int(tableOff); j < int(end); j += 4 {
			var tmp [4]byte
			copy(tmp[:], result[j:min(j+4, len(result))])
			checkSum += binary.BigEndian.Uint32(tmp[:])
		}

		binary.BigEndian.PutUint32(result[offset+4:offset+8], checkSum)
		offset += 16
	}
	// complete font:
	headOff := int(headSubtable.offset)
	if headOff+12 > len(result) {
		return nil, fmt.Errorf("font: invalid head table")
	}

	// zero adjustment field first
	binary.BigEndian.PutUint32(result[headOff+8:headOff+12], 0)

	// calculate for entire font:
	checkSum := uint32(0)
	paddedLen := (len(result) + 3) &^ 3
	for i := 0; i < paddedLen; i += 4 {
		tmp := make([]byte, 4)
		end := min(len(result), i+4)
		copy(tmp, result[i:end])
		checkSum += binary.BigEndian.Uint32(tmp[:])
	}

	adjustment := uint32(0xB1B0AFBA) - checkSum
	binary.BigEndian.PutUint32(
		result[headOff+8:headOff+12],
		adjustment,
	)

	return result, nil
}
