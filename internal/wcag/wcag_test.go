package wcag

import (
	"math"
	"testing"
)

func TestParseRejectsWhatItCannotRead(t *testing.T) {
	for _, css := range []string{"", "chartreuse", "var(--accent)", "oklch(bad)", "#ggg"} {
		if _, err := Parse(css); err == nil {
			t.Errorf("Parse(%q) = nil error, want an error — an unreadable colour must not score as black", css)
		}
	}
}

func TestParseOKLCHEndsOfScale(t *testing.T) {
	black, err := Parse("oklch(0% 0 0)")
	if err != nil {
		t.Fatalf("Parse black: %v", err)
	}
	white, err := Parse("oklch(100% 0 0)")
	if err != nil {
		t.Fatalf("Parse white: %v", err)
	}
	if got := ContrastRatio(black, white); math.Abs(got-21) > 0.1 {
		t.Errorf("ContrastRatio(black, white) = %.2f, want 21", got)
	}
}

func TestContrastIsSymmetricAndBoundedByOne(t *testing.T) {
	a, _ := Parse("oklch(71.2% 0.194 13.428)")
	b, _ := Parse("oklch(16% 0.01 13)")
	forward, backward := ContrastRatio(a, b), ContrastRatio(b, a)
	if math.Abs(forward-backward) > 1e-9 {
		t.Errorf("contrast not symmetric: %.6f vs %.6f", forward, backward)
	}
	if got := ContrastRatio(a, a); math.Abs(got-1) > 1e-9 {
		t.Errorf("ContrastRatio(c, c) = %.6f, want 1", got)
	}
}

// The palette nv ships must clear its own gate, or every fresh scaffold fails.
func TestShippedThemeClearsItsOwnThresholds(t *testing.T) {
	theme := map[string]string{
		"background": "oklch(16% 0.01 13)",
		"foreground": "oklch(98% 0.003 13)",
		"muted":      "oklch(70% 0.012 13)",
		"surface":    "oklch(23% 0.01 13)",
		"accent":     "oklch(71.2% 0.194 13.428)",
	}
	for _, pair := range TextPairs {
		fg, err := Parse(theme[pair.Foreground])
		if err != nil {
			t.Fatalf("parse %s: %v", pair.Foreground, err)
		}
		bg, err := Parse(theme[pair.Background])
		if err != nil {
			t.Fatalf("parse %s: %v", pair.Background, err)
		}
		if got := ContrastRatio(fg, bg); got < TextMinimum {
			t.Errorf("%s on %s = %.2f, below the %.1f the gate demands",
				pair.Foreground, pair.Background, got, TextMinimum)
		}
	}
	for _, pair := range NonTextPairs {
		fg, _ := Parse(theme[pair.Foreground])
		bg, _ := Parse(theme[pair.Background])
		if got := ContrastRatio(fg, bg); got < NonTextMinimum {
			t.Errorf("%s on %s = %.2f, below the %.1f the gate demands",
				pair.Foreground, pair.Background, got, NonTextMinimum)
		}
	}
}

func TestHexAndRGBAreReadToo(t *testing.T) {
	white, err := Parse("#ffffff")
	if err != nil {
		t.Fatalf("hex: %v", err)
	}
	alsoWhite, err := Parse("rgb(255, 255, 255)")
	if err != nil {
		t.Fatalf("rgb: %v", err)
	}
	if math.Abs(ContrastRatio(white, alsoWhite)-1) > 1e-9 {
		t.Error("#ffffff and rgb(255,255,255) should be the same colour")
	}
}

func TestLevelNamesTheGrade(t *testing.T) {
	cases := []struct {
		ratio float64
		want  string
	}{{21, "AAA"}, {7, "AAA"}, {4.5, "AA"}, {3, "AA-large"}, {2.9, "fail"}, {1, "fail"}}
	for _, c := range cases {
		if got := Level(c.ratio); got != c.want {
			t.Errorf("Level(%.2f) = %q, want %q", c.ratio, got, c.want)
		}
	}
}
