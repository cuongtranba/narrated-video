// Package fonts answers, from the font file's own bytes, which characters a
// font can actually draw.
//
// The question matters because the failure mode is silent. A Vietnamese
// localization shipped with "tuyên bố" rendered as "tuyên bô": the webfont had
// no glyph for U+1ED1 and drew nothing at all -- no missing-glyph box, no
// warning, nothing to notice until a full-resolution still was cropped. The
// font's declared subset was no help. It maps U+1ECD, which sits squarely
// inside Google's vietnamese range U+1EA0-1EF9, so every range-containment test
// passes, while U+1ED1, U+1EA7, U+1EBF, U+1EA5, U+01B0 and U+01A1 are simply
// not in its cmap. Only per-codepoint cmap membership is a real answer.
package fonts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"unicode"

	"github.com/andybalholm/brotli"
)

// CoveredRunes returns the set of runes a font file actually maps to a glyph.
// Accepts woff2 and plain sfnt (ttf/otf).
func CoveredRunes(data []byte) (map[rune]bool, error) {
	table, err := cmapTable(data)
	if err != nil {
		return nil, err
	}
	return parseCmap(table)
}

// Missing returns the runes of sample absent from the font, in first-appearance
// order. Normalize sample with NormalizeNFC first: a decomposed cluster reports
// as covered whenever the base and the combining mark exist separately, which
// says nothing about whether the pair renders.
func Missing(data []byte, sample string) ([]rune, error) {
	covered, err := CoveredRunes(data)
	if err != nil {
		return nil, err
	}
	return absentRunes(sample, func(r rune) bool { return covered[r] }), nil
}

// absentRunes is the single definition of what Missing and RangesCover report:
// first appearance order, repeats dropped. The caller is a human reading a
// "characters this font cannot draw" list, which is a set and not a tally.
func absentRunes(sample string, covered func(rune) bool) []rune {
	var missing []rune
	seen := make(map[rune]bool)
	for _, r := range sample {
		if seen[r] || covered(r) {
			continue
		}
		seen[r] = true
		missing = append(missing, r)
	}
	return missing
}

const (
	sigWOFF2      = "wOF2"
	sigWOFF1      = "wOFF"
	sigCollection = "ttcf"
)

func cmapTable(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("fonts: file is %d bytes, too short to hold a font signature", len(data))
	}
	switch string(data[:4]) {
	case sigWOFF2:
		return woff2CmapTable(data)
	case sigWOFF1:
		return nil, fmt.Errorf("fonts: %s is woff1, which this package does not read; convert it to woff2, ttf or otf first", sigWOFF1)
	case sigCollection:
		return nil, fmt.Errorf("fonts: %s is a font collection holding several faces, and coverage is a property of one face; convert the face you mean into a single-face ttf, otf or woff2 first", sigCollection)
	default:
		return sfntCmapTable(data)
	}
}

// --- sfnt (ttf/otf) ---

func sfntCmapTable(data []byte) ([]byte, error) {
	const (
		versionTrueType = 0x00010000
		versionOTTO     = 0x4F54544F
		versionTrue     = 0x74727565
		headerSize      = 12
		recordSize      = 16
	)
	if len(data) < headerSize {
		return nil, fmt.Errorf("fonts: file is %d bytes, too short for an sfnt header", len(data))
	}
	switch version := be32(data); version {
	case versionTrueType, versionOTTO, versionTrue:
	default:
		return nil, fmt.Errorf("fonts: unrecognised font signature %q (%#08x); expected woff2, ttf or otf", data[:4], version)
	}

	numTables := int(be16(data[4:]))
	if end := headerSize + numTables*recordSize; end > len(data) {
		return nil, fmt.Errorf("fonts: sfnt declares %d tables needing %d bytes of directory, but the file is %d bytes", numTables, end, len(data))
	}
	for i := 0; i < numTables; i++ {
		record := headerSize + i*recordSize
		if string(data[record:record+4]) != "cmap" {
			continue
		}
		offset := int(be32(data[record+8:]))
		length := int(be32(data[record+12:]))
		if offset+length > len(data) {
			return nil, fmt.Errorf("fonts: sfnt cmap at offset %d length %d runs past the %d-byte file", offset, length, len(data))
		}
		return data[offset : offset+length], nil
	}
	return nil, fmt.Errorf("fonts: sfnt font has no cmap table (%d tables present)", numTables)
}

// --- woff2 ---

type woff2Table struct {
	tag string
	// offset and length locate the table inside the single decompressed brotli
	// stream. length is the TRANSFORMED length where a transform applies, since
	// that is what the table occupies in the stream.
	offset int
	length int
}

func woff2CmapTable(data []byte) ([]byte, error) {
	tables, stream, err := woff2Tables(data)
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		if table.tag != "cmap" {
			continue
		}
		// woff2 defines transforms only for glyf, loca and hmtx, so cmap is
		// always stored verbatim. After the one brotli pass it is readable in
		// place at its accumulated offset -- reading coverage needs no glyph
		// reconstruction at all.
		return stream[table.offset : table.offset+table.length], nil
	}
	return nil, fmt.Errorf("fonts: woff2 font has no cmap table (%d tables present)", len(tables))
}

// woff2KnownTags is the spec's 63-entry table; a flags tag index of 63 means an
// explicit 4-byte tag follows the flags byte instead.
var woff2KnownTags = [63]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
	"cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
	"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea",
	"vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
	"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar",
	"bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
	"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
	"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

func woff2Tables(data []byte) ([]woff2Table, []byte, error) {
	const (
		headerSize      = 48
		arbitraryTagIdx = 63
		tagTTCF         = 0x74746366
	)
	if len(data) < headerSize {
		return nil, nil, fmt.Errorf("fonts: woff2 file is %d bytes, too short for a %d-byte header", len(data), headerSize)
	}
	if flavor := be32(data[4:]); flavor == tagTTCF {
		return nil, nil, fmt.Errorf("fonts: woff2 wraps a %s font collection holding several faces, and coverage is a property of one face; convert the face you mean into a single-face ttf, otf or woff2 first", sigCollection)
	}
	numTables := int(be16(data[12:]))
	if numTables == 0 {
		return nil, nil, fmt.Errorf("fonts: woff2 declares no tables")
	}
	totalCompressedSize := int(be32(data[20:]))

	tables := make([]woff2Table, 0, numTables)
	at := headerSize
	streamOffset := 0
	for i := 0; i < numTables; i++ {
		if at >= len(data) {
			return nil, nil, fmt.Errorf("fonts: woff2 table directory entry %d starts at byte %d, past the %d-byte file", i, at, len(data))
		}
		flags := data[at]
		at++

		tagIndex := int(flags & 0x3F)
		transformVersion := (flags >> 6) & 0x03
		var tag string
		if tagIndex == arbitraryTagIdx {
			if at+4 > len(data) {
				return nil, nil, fmt.Errorf("fonts: woff2 table directory entry %d claims an explicit tag at byte %d, past the %d-byte file", i, at, len(data))
			}
			tag = string(data[at : at+4])
			at += 4
		} else {
			tag = woff2KnownTags[tagIndex]
		}

		origLength, next, err := readBase128(data, at)
		if err != nil {
			return nil, nil, fmt.Errorf("fonts: woff2 table %q (directory entry %d): origLength: %w", tag, i, err)
		}
		at = next

		length := origLength
		if woff2Transformed(tag, transformVersion) {
			transformLength, next, err := readBase128(data, at)
			if err != nil {
				return nil, nil, fmt.Errorf("fonts: woff2 table %q (directory entry %d): transformLength: %w", tag, i, err)
			}
			at = next
			length = transformLength
		}

		tables = append(tables, woff2Table{tag: tag, offset: streamOffset, length: int(length)})
		streamOffset += int(length)
	}

	compressedEnd := len(data)
	if totalCompressedSize > 0 {
		if at+totalCompressedSize > len(data) {
			return nil, nil, fmt.Errorf("fonts: woff2 compressed block at byte %d claims %d bytes, past the %d-byte file", at, totalCompressedSize, len(data))
		}
		compressedEnd = at + totalCompressedSize
	}
	// Every table shares ONE brotli stream: decompress once, then the tables are
	// slices of the result at the offsets accumulated above.
	stream, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data[at:compressedEnd])))
	if err != nil {
		return nil, nil, fmt.Errorf("fonts: woff2 brotli stream at byte %d (%d bytes): %w", at, compressedEnd-at, err)
	}

	for i, table := range tables {
		if table.offset+table.length > len(stream) {
			return nil, nil, fmt.Errorf("fonts: woff2 table %q (directory entry %d) spans %d..%d of a %d-byte decompressed stream; the directory walk is desynchronised", table.tag, i, table.offset, table.offset+table.length, len(stream))
		}
	}
	return tables, stream, nil
}

// woff2Transformed decides whether a directory entry carries a transformLength
// and therefore occupies a different number of bytes in the stream than its
// original length.
//
// The flag semantics are INVERTED for glyf and loca: there version 3 is the
// null transform, so 0 means transformed, while for every other table 0 is the
// null transform. Reading this backwards consumes the wrong number of varints
// and shifts every subsequent table offset, after which the parse reads
// plausible garbage rather than failing.
func woff2Transformed(tag string, version byte) bool {
	if tag == "glyf" || tag == "loca" {
		return version != 3
	}
	return version != 0
}

// readBase128 decodes a UIntBase128: up to five big-endian 7-bit groups, high
// bit set on every byte but the last.
func readBase128(data []byte, at int) (uint32, int, error) {
	const maxBytes = 5
	var value uint32
	for i := 0; i < maxBytes; i++ {
		if at+i >= len(data) {
			return 0, 0, fmt.Errorf("UIntBase128 at byte %d runs past the %d-byte file", at, len(data))
		}
		b := data[at+i]
		// A leading 0x80 is a redundant leading zero. The spec forbids it, and
		// accepting it would let two byte strings denote the same number.
		if i == 0 && b == 0x80 {
			return 0, 0, fmt.Errorf("UIntBase128 at byte %d has a leading zero group", at)
		}
		if value > (1<<32-1)>>7 {
			return 0, 0, fmt.Errorf("UIntBase128 at byte %d overflows 32 bits", at)
		}
		value = value<<7 | uint32(b&0x7F)
		if b&0x80 == 0 {
			return value, at + i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("UIntBase128 at byte %d is longer than %d bytes", at, maxBytes)
}

// --- cmap ---

func parseCmap(table []byte) (map[rune]bool, error) {
	const (
		headerSize = 4
		recordSize = 8
	)
	if len(table) < headerSize {
		return nil, fmt.Errorf("fonts: cmap table is %d bytes, too short for a header", len(table))
	}
	numTables := int(be16(table[2:]))
	if end := headerSize + numTables*recordSize; end > len(table) {
		return nil, fmt.Errorf("fonts: cmap declares %d encoding records needing %d bytes, but the table is %d bytes", numTables, end, len(table))
	}

	type candidate struct {
		rank     int
		offset   int
		platform int
		encoding int
	}
	candidates := make([]candidate, 0, numTables)
	for i := 0; i < numTables; i++ {
		record := headerSize + i*recordSize
		platform := int(be16(table[record:]))
		encoding := int(be16(table[record+2:]))
		offset := int(be32(table[record+4:]))
		if offset < 0 || offset >= len(table) {
			continue
		}
		candidates = append(candidates, candidate{
			rank:     encodingRank(platform, encoding),
			offset:   offset,
			platform: platform,
			encoding: encoding,
		})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("fonts: cmap has no usable encoding record (%d present)", numTables)
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].rank > candidates[j].rank })

	var lastErr error
	for _, c := range candidates {
		covered, err := parseCmapSubtable(table[c.offset:])
		if err != nil {
			lastErr = fmt.Errorf("fonts: cmap subtable (platform %d, encoding %d) at offset %d: %w", c.platform, c.encoding, c.offset, err)
			continue
		}
		return covered, nil
	}
	return nil, lastErr
}

// encodingRank prefers Windows UCS-4 then Windows BMP, the two encodings a
// modern font is built around, then anything Unicode, then whatever is left.
// Every record scores at least 1 so a font carrying only an unusual encoding
// still gets read.
func encodingRank(platform, encoding int) int {
	switch {
	case platform == 3 && encoding == 10:
		return 4
	case platform == 3 && encoding == 1:
		return 3
	case platform == 0:
		return 2
	default:
		return 1
	}
}

func parseCmapSubtable(sub []byte) (map[rune]bool, error) {
	if len(sub) < 2 {
		return nil, fmt.Errorf("subtable is %d bytes, too short for a format field", len(sub))
	}
	switch format := be16(sub); format {
	case 4:
		return parseCmapFormat4(sub)
	case 12:
		return parseCmapFormat12(sub)
	default:
		return nil, fmt.Errorf("unsupported subtable format %d (only 4 and 12 are read)", format)
	}
}

func parseCmapFormat4(sub []byte) (map[rune]bool, error) {
	const headerSize = 14
	if len(sub) < headerSize {
		return nil, fmt.Errorf("format 4 header needs %d bytes, subtable holds %d", headerSize, len(sub))
	}
	segCountX2 := int(be16(sub[6:]))
	if segCountX2 == 0 || segCountX2%2 != 0 {
		return nil, fmt.Errorf("format 4 segCountX2 is %d, want a positive even number", segCountX2)
	}
	segCount := segCountX2 / 2

	endCodes := headerSize
	startCodes := endCodes + segCountX2 + 2 // the reservedPad UInt16 sits between them
	idDeltas := startCodes + segCountX2
	idRangeOffsets := idDeltas + segCountX2
	if end := idRangeOffsets + segCountX2; end > len(sub) {
		return nil, fmt.Errorf("format 4 with %d segments needs %d bytes, subtable holds %d", segCount, end, len(sub))
	}

	covered := make(map[rune]bool)
	for i := 0; i < segCount; i++ {
		start := int(be16(sub[startCodes+i*2:]))
		end := int(be16(sub[endCodes+i*2:]))
		delta := be16(sub[idDeltas+i*2:])
		idRangeOffset := int(be16(sub[idRangeOffsets+i*2:]))
		if start > end {
			continue
		}
		for c := start; c <= end; c++ {
			var glyph uint16
			if idRangeOffset == 0 {
				glyph = uint16(c) + delta
			} else {
				// idRangeOffset is a byte distance from its OWN slot in the
				// idRangeOffsets array, not from the start of the subtable, so
				// the slot's address has to be added back in.
				at := idRangeOffsets + i*2 + idRangeOffset + (c-start)*2
				if at < 0 || at+2 > len(sub) {
					continue
				}
				glyph = be16(sub[at:])
				if glyph == 0 {
					continue
				}
				glyph += delta
			}
			// Glyph 0 is .notdef. A codepoint mapped there is the font saying it
			// has nothing to draw, which is absence, not coverage of a blank.
			if glyph == 0 {
				continue
			}
			covered[rune(c)] = true
		}
	}
	if len(covered) == 0 {
		return nil, fmt.Errorf("format 4 subtable with %d segments maps no codepoint to a glyph", segCount)
	}
	return covered, nil
}

func parseCmapFormat12(sub []byte) (map[rune]bool, error) {
	const (
		headerSize = 16
		groupSize  = 12
	)
	if len(sub) < headerSize {
		return nil, fmt.Errorf("format 12 header needs %d bytes, subtable holds %d", headerSize, len(sub))
	}
	numGroups := int(be32(sub[12:]))
	// numGroups is a UInt32, so the byte count it implies is computed in int64:
	// a font declaring billions of groups must be rejected, not wrapped into a
	// small positive length that passes the bounds check.
	if end := int64(headerSize) + int64(numGroups)*groupSize; end > int64(len(sub)) {
		return nil, fmt.Errorf("format 12 with %d groups needs %d bytes, subtable holds %d", numGroups, end, len(sub))
	}

	covered := make(map[rune]bool)
	for i := 0; i < numGroups; i++ {
		group := headerSize + i*groupSize
		start := int64(be32(sub[group:]))
		end := int64(be32(sub[group+4:]))
		startGlyph := be32(sub[group+8:])
		if start > end || start > unicode.MaxRune {
			continue
		}
		if end > unicode.MaxRune {
			end = unicode.MaxRune
		}
		for c := start; c <= end; c++ {
			if startGlyph+uint32(c-start) == 0 {
				continue
			}
			covered[rune(c)] = true
		}
	}
	if len(covered) == 0 {
		return nil, fmt.Errorf("format 12 subtable with %d groups maps no codepoint to a glyph", numGroups)
	}
	return covered, nil
}

func be16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }
