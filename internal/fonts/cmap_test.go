package fonts

import (
	"os"
	"path/filepath"
	"testing"
)

// Non-ASCII samples are spelled with explicit escapes: a precomposed and a
// decomposed cluster are indistinguishable in a source file, and which one a
// test means is the whole point here.
const (
	oCircumflexAcute = "\u1ED1"
	oCircumflex      = "\u00F4"
	oHorn            = "\u01A1"
	uHorn            = "\u01B0"
	aCircumflexGrave = "\u1EA7"
	eCircumflex      = "\u00EA"
	combiningAcute   = "\u0301"
)

func bodyFont(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "body-regular.woff2"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// This test is the bug the package exists for: the Body woff2 shipped a
// Vietnamese video in which "tuyên bố" rendered as "tuyên bô". No missing-glyph
// box was drawn, so the drop was invisible until a full-resolution still was
// cropped. Every declared-subset check passes on this font (U+1ECD is inside
// Google's vietnamese range U+1EA0-1EF9) while six of the most common stacked
// vowels are absent from its cmap. Per-codepoint membership is the only real
// answer, and this test pins it.
func TestCoveredRunes_BodyFontMissingVietnameseStackedMarks(t *testing.T) {
	covered, err := CoveredRunes(bodyFont(t))
	if err != nil {
		t.Fatalf("CoveredRunes: %v", err)
	}

	present := []struct {
		r    rune
		name string
	}{
		{0x1ECD, "LATIN SMALL LETTER O WITH DOT BELOW"},
		{0x0111, "LATIN SMALL LETTER D WITH STROKE"},
		{0x0110, "LATIN CAPITAL LETTER D WITH STROKE"},
		{0x0301, "COMBINING ACUTE ACCENT"},
		{0x0303, "COMBINING TILDE"},
		{0x0323, "COMBINING DOT BELOW"},
	}
	for _, c := range present {
		if !covered[c.r] {
			t.Errorf("U+%04X (%s) reported absent, but the fixture's cmap does map it; the cmap walk is dropping real coverage", c.r, c.name)
		}
	}

	absent := []struct {
		r    rune
		name string
	}{
		{0x1ED1, "LATIN SMALL LETTER O WITH CIRCUMFLEX AND ACUTE"},
		{0x1EA7, "LATIN SMALL LETTER A WITH CIRCUMFLEX AND GRAVE"},
		{0x1EBF, "LATIN SMALL LETTER E WITH CIRCUMFLEX AND ACUTE"},
		{0x1EA5, "LATIN SMALL LETTER A WITH CIRCUMFLEX AND ACUTE"},
		{0x01B0, "LATIN SMALL LETTER U WITH HORN"},
		{0x01A1, "LATIN SMALL LETTER O WITH HORN"},
	}
	for _, c := range absent {
		if covered[c.r] {
			t.Errorf("U+%04X (%s) reported as covered, but the fixture does NOT map it -- this false positive is exactly the silent drop that shipped %q for %q", c.r, c.name, "tuy"+eCircumflex+"n b"+oCircumflex, "tuy"+eCircumflex+"n b"+oCircumflexAcute)
		}
	}
}

func TestMissing_ReturnsRunesInOrder(t *testing.T) {
	sample := "tuy" + eCircumflex + "n b" + oCircumflexAcute
	missing, err := Missing(bodyFont(t), sample)
	if err != nil {
		t.Fatalf("Missing: %v", err)
	}
	if got, want := string(missing), oCircumflexAcute; got != want {
		t.Fatalf("Missing(%+q) = %+q, want %+q", sample, got, want)
	}
}

func TestMissing_FirstAppearanceOrderAndDeduped(t *testing.T) {
	sample := oHorn + uHorn + " " + oHorn + " " + aCircumflexGrave
	missing, err := Missing(bodyFont(t), sample)
	if err != nil {
		t.Fatalf("Missing: %v", err)
	}
	if got, want := string(missing), oHorn+uHorn+aCircumflexGrave; got != want {
		t.Fatalf("Missing(%+q) = %+q, want %+q", sample, got, want)
	}
}

func TestCoveredRunes_RejectsWoff1AndCollections(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"woff1", append([]byte("wOFF"), make([]byte, 40)...), "woff1"},
		{"collection", append([]byte("ttcf"), make([]byte, 40)...), "collection"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CoveredRunes(tc.data)
			if err == nil {
				t.Fatalf("CoveredRunes(%s) succeeded, want an error", tc.name)
			}
			if !containsFold(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
			if !containsFold(err.Error(), "convert") {
				t.Fatalf("error %q should tell the caller what to convert the file to", err)
			}
		})
	}
}

func TestCoveredRunes_RejectsGarbage(t *testing.T) {
	if _, err := CoveredRunes([]byte("not a font at all, just prose")); err == nil {
		t.Fatal("CoveredRunes(garbage) succeeded, want an error")
	}
	if _, err := CoveredRunes(nil); err == nil {
		t.Fatal("CoveredRunes(nil) succeeded, want an error")
	}
}

// The woff2 table directory is walked by accumulating transformed lengths into
// the single decompressed brotli stream. If that walk desynchronises -- the
// classic causes being the inverted transform flag on glyf/loca and a botched
// UIntBase128 -- every later table offset is wrong and the parse silently reads
// garbage instead of failing. The sum reconciling with the stream length is the
// cheap proof that it did not.
func TestWoff2TableWalkReconcilesWithDecompressedStream(t *testing.T) {
	tables, stream, err := woff2Tables(bodyFont(t))
	if err != nil {
		t.Fatalf("woff2Tables: %v", err)
	}
	total := 0
	for _, tab := range tables {
		total += tab.length
	}
	if total != len(stream) {
		t.Fatalf("table lengths sum to %d but the decompressed stream is %d bytes; the directory walk is desynchronised", total, len(stream))
	}
	const fixtureTotal = 133340
	if total != fixtureTotal {
		t.Fatalf("fixture table-length total = %d, want %d", total, fixtureTotal)
	}
	if len(tables) != 17 {
		t.Fatalf("fixture table count = %d, want 17", len(tables))
	}
}

func containsFold(haystack, needle string) bool {
	lower := func(s string) string {
		b := []byte(s)
		for i := range b {
			if b[i] >= 'A' && b[i] <= 'Z' {
				b[i] += 'a' - 'A'
			}
		}
		return string(b)
	}
	h, n := lower(haystack), lower(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
