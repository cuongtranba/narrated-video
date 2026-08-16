package fonts

import (
	"encoding/binary"
	"testing"
)

// The fixture carries a single format 4 subtable whose segments all use the
// idDelta branch, so the two remaining ways a cmap can be read -- format 4's
// idRangeOffset pointer arithmetic, and format 12 -- are pinned with subtables
// built by hand here. Neither is exotic: idRangeOffset is how any font with a
// non-contiguous glyph order maps its BMP, and format 12 is the only way to
// reach anything above U+FFFF.

func TestParseCmapFormat4_IdRangeOffsetArithmeticAndNotdef(t *testing.T) {
	const segCount = 3
	const (
		endCodes       = 14
		startCodes     = endCodes + segCount*2 + 2
		idDeltas       = startCodes + segCount*2
		idRangeOffsets = idDeltas + segCount*2
		glyphIDs       = idRangeOffsets + segCount*2
		subtableLen    = glyphIDs + 3*2
	)
	sub := make([]byte, subtableLen)
	put := func(at int, v uint16) { binary.BigEndian.PutUint16(sub[at:], v) }

	put(0, 4)
	put(2, subtableLen)
	put(6, segCount*2)

	// Segment 0 resolves through glyphIdArray. idRangeOffset is a byte distance
	// from the segment's OWN slot, so reaching glyphIDs from slot idRangeOffsets
	// costs exactly the gap between them.
	put(endCodes+0, 0x0043)
	put(startCodes+0, 0x0041)
	put(idDeltas+0, 0)
	put(idRangeOffsets+0, glyphIDs-idRangeOffsets)
	put(glyphIDs+0, 7) // A
	put(glyphIDs+2, 0) // B maps to .notdef
	put(glyphIDs+4, 9) // C

	// Segment 1 resolves through idDelta.
	put(endCodes+2, 0x0062)
	put(startCodes+2, 0x0061)
	put(idDeltas+2, 0x0010)
	put(idRangeOffsets+2, 0)

	// Segment 2 is the mandatory 0xFFFF terminator, which delta-wraps to glyph 0.
	put(endCodes+4, 0xFFFF)
	put(startCodes+4, 0xFFFF)
	put(idDeltas+4, 1)
	put(idRangeOffsets+4, 0)

	covered, err := parseCmapFormat4(sub)
	if err != nil {
		t.Fatalf("parseCmapFormat4: %v", err)
	}
	for _, r := range []rune{'A', 'C', 'a', 'b'} {
		if !covered[r] {
			t.Errorf("U+%04X reported absent, want covered", r)
		}
	}
	for _, r := range []rune{'B', 0xFFFF} {
		if covered[r] {
			t.Errorf("U+%04X reported covered, but it maps to glyph 0 (.notdef), which is the font saying it has nothing to draw", r)
		}
	}
	if len(covered) != 4 {
		t.Errorf("covered %d runes, want 4", len(covered))
	}
}

func TestParseCmapFormat12_BeyondBMPAndNotdef(t *testing.T) {
	groups := []struct{ start, end, startGlyph uint32 }{
		{0x0041, 0x0043, 7},
		{0x1F600, 0x1F601, 40},
		{0x0030, 0x0031, 0}, // startGlyph 0 makes only the FIRST codepoint .notdef
	}
	const headerSize = 16
	sub := make([]byte, headerSize+len(groups)*12)
	binary.BigEndian.PutUint16(sub[0:], 12)
	binary.BigEndian.PutUint32(sub[4:], uint32(len(sub)))
	binary.BigEndian.PutUint32(sub[12:], uint32(len(groups)))
	for i, g := range groups {
		at := headerSize + i*12
		binary.BigEndian.PutUint32(sub[at:], g.start)
		binary.BigEndian.PutUint32(sub[at+4:], g.end)
		binary.BigEndian.PutUint32(sub[at+8:], g.startGlyph)
	}

	covered, err := parseCmapFormat12(sub)
	if err != nil {
		t.Fatalf("parseCmapFormat12: %v", err)
	}
	for _, r := range []rune{'A', 'B', 'C', 0x1F600, 0x1F601, '1'} {
		if !covered[r] {
			t.Errorf("U+%04X reported absent, want covered", r)
		}
	}
	if covered['0'] {
		t.Error("U+0030 reported covered, but its group starts at glyph 0 (.notdef)")
	}
	if len(covered) != 6 {
		t.Errorf("covered %d runes, want 6", len(covered))
	}
}

// The sfnt path is exercised against the fixture's own cmap rather than a
// hand-written one, so ttf/otf and woff2 are proven to reach the same answer
// from the same bytes.
func TestCoveredRunes_SfntContainerMatchesWoff2(t *testing.T) {
	font := bodyFont(t)
	cmap, err := cmapTable(font)
	if err != nil {
		t.Fatalf("cmapTable: %v", err)
	}

	const headerSize, recordSize = 12, 16
	sfnt := make([]byte, headerSize+recordSize)
	binary.BigEndian.PutUint32(sfnt[0:], 0x00010000)
	binary.BigEndian.PutUint16(sfnt[4:], 1)
	copy(sfnt[headerSize:], "cmap")
	binary.BigEndian.PutUint32(sfnt[headerSize+8:], uint32(len(sfnt)))
	binary.BigEndian.PutUint32(sfnt[headerSize+12:], uint32(len(cmap)))
	sfnt = append(sfnt, cmap...)

	fromSfnt, err := CoveredRunes(sfnt)
	if err != nil {
		t.Fatalf("CoveredRunes(sfnt): %v", err)
	}
	fromWoff2, err := CoveredRunes(font)
	if err != nil {
		t.Fatalf("CoveredRunes(woff2): %v", err)
	}
	if len(fromSfnt) != len(fromWoff2) {
		t.Fatalf("sfnt covers %d runes, woff2 covers %d", len(fromSfnt), len(fromWoff2))
	}
	for r := range fromWoff2 {
		if !fromSfnt[r] {
			t.Fatalf("U+%04X covered via woff2 but not via sfnt", r)
		}
	}
	if fromSfnt[0x1ED1] {
		t.Error("U+1ED1 reported covered via the sfnt path")
	}
}

func TestSfntCmapTable_RejectsFontWithoutCmap(t *testing.T) {
	sfnt := make([]byte, 12)
	binary.BigEndian.PutUint32(sfnt[0:], 0x00010000)
	if _, err := CoveredRunes(sfnt); err == nil {
		t.Fatal("CoveredRunes(sfnt with no tables) succeeded, want an error")
	} else if !containsFold(err.Error(), "no cmap") {
		t.Fatalf("error %q should say the font has no cmap table", err)
	}
}

func TestReadBase128(t *testing.T) {
	cases := []struct {
		name    string
		in      []byte
		want    uint32
		wantEnd int
		wantErr bool
	}{
		{name: "single byte", in: []byte{0x00}, want: 0, wantEnd: 1},
		{name: "single byte max", in: []byte{0x7F}, want: 127, wantEnd: 1},
		{name: "GDEF origLength from the fixture", in: []byte{0x81, 0x5E}, want: 222, wantEnd: 2},
		{name: "GPOS origLength from the fixture", in: []byte{0x82, 0xBA, 0x04}, want: 40196, wantEnd: 3},
		{name: "five byte maximum", in: []byte{0x8F, 0xFF, 0xFF, 0xFF, 0x7F}, want: 0xFFFFFFFF, wantEnd: 5},
		{name: "trailing bytes are not consumed", in: []byte{0x60, 0xFF}, want: 96, wantEnd: 1},
		{name: "leading zero group is forbidden", in: []byte{0x80, 0x01}, wantErr: true},
		{name: "overflows 32 bits", in: []byte{0x90, 0x80, 0x80, 0x80, 0x00}, wantErr: true},
		{name: "longer than five bytes", in: []byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x01}, wantErr: true},
		{name: "truncated", in: []byte{0x81}, wantErr: true},
		{name: "empty", in: nil, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, end, err := readBase128(tc.in, 0)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("readBase128(% x) = %d, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("readBase128(% x): %v", tc.in, err)
			}
			if got != tc.want || end != tc.wantEnd {
				t.Fatalf("readBase128(% x) = %d, %d; want %d, %d", tc.in, got, end, tc.want, tc.wantEnd)
			}
		})
	}
}

// The fixture's glyf entry carries transform version 0 and loca carries a
// transformLength of 0, so the fixture only reconciles when the inverted rule is
// applied. This states the rule directly so a regression names itself rather
// than showing up as an offset mismatch.
func TestWoff2Transformed_FlagIsInvertedForGlyfAndLoca(t *testing.T) {
	cases := []struct {
		tag     string
		version byte
		want    bool
	}{
		{"glyf", 0, true},
		{"glyf", 3, false},
		{"loca", 0, true},
		{"loca", 3, false},
		{"cmap", 0, false},
		{"cmap", 3, true},
		{"hmtx", 0, false},
		{"hmtx", 1, true},
	}
	for _, tc := range cases {
		if got := woff2Transformed(tc.tag, tc.version); got != tc.want {
			t.Errorf("woff2Transformed(%q, %d) = %v, want %v", tc.tag, tc.version, got, tc.want)
		}
	}
}
