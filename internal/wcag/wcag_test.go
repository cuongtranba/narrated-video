package wcag

import (
	"math"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/kit"
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

// shippedTheme reads the palette out of the kit rather than restating it.
//
// A copy here drifted once already: this test went on scoring a red-hue palette
// for the length of a redesign, while the kit shipped navy and teal and the
// config claimed "every pair below is scored by CHK-39/40". A test named for the
// shipped palette that scores a different one is worse than no test, because it
// reports green for a question nobody is asking.
func shippedTheme(t *testing.T) map[string]string {
	t.Helper()
	raw, err := kit.FS.ReadFile("video.config.yaml")
	if err != nil {
		t.Fatalf("read the kit's config: %v", err)
	}
	cfg, err := config.Parse(raw, "video.config.yaml")
	if err != nil {
		t.Fatalf("parse the kit's config: %v", err)
	}
	return cfg.Theme
}

// A pair naming a key the theme does not declare is scored by nothing:
// contrastFindings skips any pair with a missing side, so the colour ships
// unmeasured while the pair list implies it was checked. That is the same
// silence as deleting a key to turn a check green.
func TestEveryPairedKeyIsInTheShippedTheme(t *testing.T) {
	theme := shippedTheme(t)
	for _, pair := range append(append([]Pair{}, TextPairs...), NonTextPairs...) {
		for _, key := range []string{pair.Foreground, pair.Background} {
			if _, ok := theme[key]; !ok {
				t.Errorf("%q is paired for %q but the shipped theme does not declare it", key, pair.Where)
			}
		}
	}
}

// The palette nv ships must clear its own gate, or every fresh scaffold fails.
func TestShippedThemeClearsItsOwnThresholds(t *testing.T) {
	theme := shippedTheme(t)
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
