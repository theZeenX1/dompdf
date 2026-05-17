package fonts

import (
	"encoding/binary"
	"fmt"
)

func ParseTrueTypeFont(dat []byte) (*TrueTypeFont, error) {
	if len(dat) < 12 {
		return nil, fmt.Errorf("font: not enough bytes present")
	}

	header := sntfHeader{}

	header.sntfVersion = binary.BigEndian.Uint32(dat[0:4])
	header.numTables = binary.BigEndian.Uint16(dat[4:6])
	header.searchRange = binary.BigEndian.Uint16(dat[6:8])
	header.entrySelector = binary.BigEndian.Uint16(dat[8:10])
	header.rangeShift = binary.BigEndian.Uint16(dat[10:12])

	switch header.sntfVersion {
	case sntfVerTrueType, sntfVerTrue, sntfVerOTTO:
	default:
		return nil, fmt.Errorf("font: sntfVersion not supported")
	}

	tables := map[string]fontTable{}
	offset := 12
	for i := 0; i < int(header.numTables); i++ {
		if offset+16 < len(dat) {
			return nil, fmt.Errorf("font: table not complete")
		}
		table := fontTable{}
		var tag [4]byte
		copy(tag[:], dat[offset:offset+4])
		table.tag = tag
		table.checkSum = binary.BigEndian.Uint32(dat[offset+4 : offset+8])
		table.offset = binary.BigEndian.Uint32(dat[offset+8 : offset+12])
		table.length = binary.BigEndian.Uint32(dat[offset+12 : offset+16])
		tables[string(tag[:])] = table
	}

	ttf := &TrueTypeFont{
		dat:       dat,
		usedRunes: make(map[rune]bool),
	}

	ttf.tables = tables

	if err := ttf.parseHead(dat); err != nil {
		return nil, err
	}

	var noOfHMetrics *uint16
	if err := ttf.parseHhea(dat, noOfHMetrics); err != nil {
		return nil, err
	}

	if err := ttf.parseMaxp(dat); err != nil {
		return nil, err
	}

	ttf.parseOS2(dat)

	ttf.parseName(dat)

	if err := ttf.parseHmtx(dat, noOfHMetrics); err != nil {
		return nil, err
	}

	if err := ttf.parseCmap(dat); err != nil {
		return nil, err
	}

	ttf.parsePost(dat)

	return ttf, nil
}

func (ttf *TrueTypeFont) findFontTable(tagName string, dat []byte) ([]byte, error) {
	table, ok := ttf.tables[tagName]
	if !ok {
		return nil, fmt.Errorf("font: tagName (%s) not present in font tables", tagName)
	}
	end := int(table.offset) + int(table.length)
	if end > len(dat) {
		return nil, fmt.Errorf("font: font table for %s truncated", tagName)
	}
	return dat[table.offset:end], nil
}

func (ttf *TrueTypeFont) parseHead(dat []byte) error {
	table, err := ttf.findFontTable(tagHead, dat)
	if err != nil {
		return err
	}
	if len(table) < 54 {
		return fmt.Errorf("font: table head too short")
	}
	// the unitsPerEm is from 18:20
	ttf.unitsPerEm = int(binary.BigEndian.Uint16(table[18:20]))
	ttf.metrics.UnitsPerEm = ttf.unitsPerEm
	return nil
}

func (ttf *TrueTypeFont) parseHhea(dat []byte, noOfHMetrics *uint16) error {
	table, err := ttf.findFontTable(tagHhea, dat)
	if err != nil {
		return err
	}
	if len(table) < 36 {
		return fmt.Errorf("font: table hhea too short")
	}
	ttf.metrics.Ascender = int(int16(binary.BigEndian.Uint16(table[4:6])))
	ttf.metrics.Descender = int(int16(binary.BigEndian.Uint16(table[6:8])))
	ttf.metrics.LineGap = int(int16(binary.BigEndian.Uint16(table[8:10])))
	*noOfHMetrics = binary.BigEndian.Uint16(table[34:36])
	return nil
}

func (ttf *TrueTypeFont) parseMaxp(dat []byte) error {
	table, err := ttf.findFontTable(tagMaxp, dat)
	if err != nil {
		return err
	}
	if len(table) < 6 {
		return fmt.Errorf("font: maxp table truncated")
	}
	ttf.numGlyphs = int(binary.BigEndian.Uint16(table[4:6]))
	return nil
}

func (ttf *TrueTypeFont) parseOS2(dat []byte) {
	table, err := ttf.findFontTable(tagOS2, dat)
	if err != nil || len(table) < 72 {
		return
	}
	// sCapHeight is at offset 88 in version 2+ of OS/2.
	if len(table) >= 90 {
		ttf.metrics.CapHeight = int(int16(binary.BigEndian.Uint16(table[88:90])))
	}
	// sxHeight is at offset 86 in version 2+.
	if len(table) >= 88 {
		ttf.metrics.XHeight = int(int16(binary.BigEndian.Uint16(table[86:88])))
	}
}

type recordTable struct {
	platformId   uint16
	encodingId   uint16
	languageId   uint16
	nameId       uint16
	length       uint16
	stringOffset uint16
}

// readNameRecord reads a name record from the given offset in the table data.
func readNameRecord(table []byte, offset int) recordTable {
	return recordTable{
		platformId:   binary.BigEndian.Uint16(table[offset : offset+2]),
		nameId:       binary.BigEndian.Uint16(table[offset+6 : offset+8]),
		length:       binary.BigEndian.Uint16(table[offset+8 : offset+10]),
		stringOffset: binary.BigEndian.Uint16(table[offset+10 : offset+12]),
	}
}

func decodeUTF16BE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		r := rune(binary.BigEndian.Uint16(b[i : i+2]))
		runes = append(runes, r)
	}
	return string(runes)
}

// decodeNameString decodes a name string from the table storage area.
func decodeNameString(table []byte, storageOffset int, rec recordTable) string {
	start := storageOffset + int(rec.stringOffset)
	end := start + int(rec.length)
	if end > len(table) {
		return ""
	}
	if rec.platformId == 3 || rec.platformId == 0 {
		return decodeUTF16BE(table[start:end])
	}
	return string(table[start:end])
}

func (ttf *TrueTypeFont) parseName(dat []byte) {
	table, err := ttf.findFontTable(tagName, dat)
	if err != nil || len(table) < 6 {
		return
	}

	count := binary.BigEndian.Uint16(table[2:4])
	storageOffset := binary.BigEndian.Uint16(table[4:6])

	var postScriptName, fullName string
	offset := 6
	for i := 0; i < int(count) && offset+12 <= len(table); i++ {
		rec := readNameRecord(table, offset)
		offset += 12

		if rec.nameId != 6 && rec.nameId != 4 {
			continue
		}
		name := decodeNameString(table, int(storageOffset), rec)
		if rec.nameId == 6 && postScriptName == "" {
			postScriptName = name
		}
		if rec.nameId == 4 && fullName == "" {
			fullName = name
		}
	}

	switch {
	case postScriptName != "":
		ttf.name = postScriptName
	case fullName != "":
		ttf.name = fullName
	default:
		ttf.name = "Unknown"
	}

}

func (ttf *TrueTypeFont) parseHmtx(dat []byte, noOfHMetrics *uint16) error {
	table, err := ttf.findFontTable(tagHmtx, dat)
	if err != nil {
		return err
	}
	numH := int(*noOfHMetrics)
	if len(table) < numH*4 {
		return fmt.Errorf("font: hmtx table truncated")
	}

	ttf.hmtxTbl.advanceWidths = make([]uint16, ttf.numGlyphs)
	ttf.hmtxTbl.leftSideBearings = make([]int16, ttf.numGlyphs)

	for i := 0; i < numH; i++ {
		off := i * 4
		ttf.hmtxTbl.advanceWidths[i] = binary.BigEndian.Uint16(table[off : off+2])
		ttf.hmtxTbl.leftSideBearings[i] = int16(binary.BigEndian.Uint16(table[off+2 : off+4]))
	}

	// remaining glyphs share the last advance width.
	lastWidth := uint16(0)
	if numH > 0 {
		lastWidth = ttf.hmtxTbl.advanceWidths[numH-1]
	}

	remainingOffset := numH * 4
	for i := numH; i < ttf.numGlyphs; i++ {
		ttf.hmtxTbl.advanceWidths[i] = lastWidth
		lsbOff := remainingOffset + (i-numH)*2
		if lsbOff+2 <= len(table) {
			ttf.hmtxTbl.leftSideBearings[i] = int16(binary.BigEndian.Uint16(table[lsbOff : lsbOff+2]))
		}
	}

	return nil
}

func (ttf *TrueTypeFont) parseCmap(dat []byte) error {
	table, err := ttf.findFontTable(tagCmap, dat)
	if err != nil {
		return err
	}
	if len(table) < 4 {
		return fmt.Errorf("font: cmap table truncated")
	}

	noOfSubtables := binary.BigEndian.Uint16(table[2:4])
	subtables := []cmapSubtable{}
	off := 4
	for i := 0; i < int(noOfSubtables); i++ {
		if off+8 > len(table) {
			break
		}
		subtable := cmapSubtable{}
		subtable.platformId = binary.BigEndian.Uint16(table[off : off+2])
		subtable.platformSpecificId = binary.BigEndian.Uint16(table[off+2 : off+4])
		subtable.offset = binary.BigEndian.Uint32(table[off+4 : off+8])
		subtables = append(subtables, subtable)
		off += 8
	}

	for _, st := range subtables {
		if int(st.offset)+2 > len(table) {
			continue
		}
		format := binary.BigEndian.Uint16(table[st.offset : st.offset+2])
		switch format {
		case 4:
			if ttf.cmapTbl.format4 == nil {
				f4, err := parseCmapFormat4(table, int(st.offset))
				if err == nil {
					ttf.cmapTbl.format4 = f4
				}
			}
		case 12:
			if ttf.cmapTbl.format12 == nil {
				f12, err := parseCmapFormat12(table, int(st.offset))
				if err == nil {
					ttf.cmapTbl.format12 = f12
				}
			}
		}
	}

	if ttf.cmapTbl.format4 == nil && ttf.cmapTbl.format12 == nil {
		return fmt.Errorf("font: no supported cmap subtable found (need format 4 or 12)")
	}

	return nil
}

func parseCmapFormat4(table []byte, offset int) (*cmapFormat4, error) {
	if offset+14 > len(table) {
		return nil, fmt.Errorf("font: cmap format 4 header too short")
	}

	length := int(binary.BigEndian.Uint16(table[offset+2 : offset+4]))
	if offset+length > len(table) {
		return nil, fmt.Errorf("font: cmap format 4 extends beyond table")
	}

	segCount := int(binary.BigEndian.Uint16(table[offset+6:offset+8])) / 2
	f4 := &cmapFormat4{
		segCount: segCount,
	}

	// endcodes:
	start := offset + 14
	f4.endCodes = []uint16{}
	for i := 0; i < int(segCount); i++ {
		pos := start + i*2
		if pos+2 > len(table) {
			return nil, fmt.Errorf("font: cmap format 4 endCode array truncated")
		}
		f4.endCodes = append(f4.endCodes, binary.BigEndian.Uint16(table[pos:pos+2]))
	}

	// reservedPad (2 bytes) for startCode
	start += segCount*2 + 2
	f4.startCodes = []uint16{}
	for i := 0; i < int(segCount); i++ {
		pos := start + i*2
		if pos+2 > len(table) {
			return nil, fmt.Errorf("font: cmap format 4 startCode array truncated")
		}
		f4.startCodes = append(f4.startCodes, binary.BigEndian.Uint16(table[pos:pos+2]))
	}

	// idDeltas
	start += segCount * 2
	f4.idDeltas = []int16{}
	for i := 0; i < int(segCount); i++ {
		pos := start + i*2
		if pos+2 > len(table) {
			return nil, fmt.Errorf("font: cmap format 4 idDelta array truncated")
		}
		f4.idDeltas = append(f4.idDeltas, int16(binary.BigEndian.Uint16(table[pos:pos+2])))
	}

	// idRangeOffsets
	start += segCount * 2
	f4.idRangeOffsetBase = start
	f4.idRangeOffsets = []uint16{}
	for i := 0; i < int(segCount); i++ {
		pos := start + i*2
		if pos+2 > len(table) {
			return nil, fmt.Errorf("font: cmap format 4 idRangeOffset array truncated")
		}
		f4.idRangeOffsets = append(f4.idRangeOffsets, binary.BigEndian.Uint16(table[pos:pos+2]))
	}

	// glyphIdArray
	start += segCount * 2
	glyphIDEnd := offset + length
	if start < glyphIDEnd {
		numGlyphIDs := (glyphIDEnd - start) / 2
		f4.glyphIDArray = make([]uint16, numGlyphIDs)
		for i := 0; i < numGlyphIDs; i++ {
			pos := start + i*2
			if pos+2 > len(table) {
				break
			}
			f4.glyphIDArray[i] = binary.BigEndian.Uint16(table[pos : pos+2])
		}
	}

	return f4, nil
}

func parseCmapFormat12(table []byte, offset int) (*cmapFormat12, error) {
	if offset+16 > len(table) {
		return nil, fmt.Errorf("font: cmap format 12 header too short")
	}

	// bytes 0-1: format (12), 2-3: reserved, 4-7: length, 8-11: language, 12-15: numGroups.
	numGroups := binary.BigEndian.Uint32(table[offset+12 : offset+16])

	f12 := &cmapFormat12{
		groups: make([]cmapFormat12Group, numGroups),
	}

	groupStart := offset + 16
	for i := 0; i < int(numGroups); i++ {
		pos := groupStart + i*12
		if pos+12 > len(table) {
			return nil, fmt.Errorf("font: cmap format 12 group array truncated")
		}
		f12.groups[i] = cmapFormat12Group{
			startCharCode: binary.BigEndian.Uint32(table[pos : pos+4]),
			endCharCode:   binary.BigEndian.Uint32(table[pos+4 : pos+8]),
			startGlyphID:  binary.BigEndian.Uint32(table[pos+8 : pos+12]),
		}
	}

	return f12, nil
}

func (ttf *TrueTypeFont) parsePost(dat []byte) {
	table, err := ttf.findFontTable(tagPost, dat)
	if err != nil || len(table) < 32 {
		return
	}
	// italicAngle is a Fixed (16.16) at offset 4.
	intPart := int16(binary.BigEndian.Uint16(table[4:6]))
	fracPart := binary.BigEndian.Uint16(table[6:8])
	ttf.metrics.ItalicAngle = float64(intPart) + float64(fracPart)/65536.0
}
