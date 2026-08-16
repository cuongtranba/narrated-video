package fonts

import "testing"

func TestIsNFC(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"precomposed o circumflex acute", oCircumflexAcute, true},
		{"o circumflex plus combining acute", oCircumflex + combiningAcute, false},
		{"o plus combining circumflex plus combining acute", "ố", false},
		{"ascii", "tuyen bo", true},
		{"empty", "", true},
		{"precomposed vietnamese sentence", vietnameseSample, true},
		{"sentence with one decomposed cluster", "Tuy" + eCircumflex + "n b" + oCircumflex + combiningAcute, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNFC(tc.s); got != tc.want {
				t.Fatalf("IsNFC(%+q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

func TestNormalizeNFC(t *testing.T) {
	cases := []struct{ in, want string }{
		{oCircumflexAcute, oCircumflexAcute},
		{oCircumflex + combiningAcute, oCircumflexAcute},
		{"ố", oCircumflexAcute},
		{"", ""},
		{vietnameseSample, vietnameseSample},
	}
	for _, tc := range cases {
		got := NormalizeNFC(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeNFC(%+q) = %+q, want %+q", tc.in, got, tc.want)
		}
		if !IsNFC(got) {
			t.Fatalf("NormalizeNFC(%+q) = %+q, which IsNFC rejects", tc.in, got)
		}
	}
}

// A decomposed cluster is not a cosmetic difference: every rune of "b" + o
// circumflex + combining acute is covered by the fixture, so a coverage check
// run over the decomposed text passes while the rendered result still depends
// on mark-attachment tables the font may not have. Normalizing first is what
// turns that into the honest miss the video needed to see.
func TestNormalizeNFC_TurnsAnUncoveredClusterIntoAnHonestMiss(t *testing.T) {
	font := bodyFont(t)
	covered, err := CoveredRunes(font)
	if err != nil {
		t.Fatalf("CoveredRunes: %v", err)
	}
	decomposed := "b" + oCircumflex + combiningAcute
	for _, r := range decomposed {
		if !covered[r] {
			t.Fatalf("U+%04X unexpectedly absent; the decomposed form was supposed to look fully covered", r)
		}
	}
	missing, err := Missing(font, NormalizeNFC(decomposed))
	if err != nil {
		t.Fatalf("Missing: %v", err)
	}
	if got := string(missing); got != oCircumflexAcute {
		t.Fatalf("Missing(NormalizeNFC(%+q)) = %+q, want %+q", decomposed, got, oCircumflexAcute)
	}
}
