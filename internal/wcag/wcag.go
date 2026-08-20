// Package wcag scores the contrast of a theme's colour pairs.
//
// The config writes colour in OKLCH, which no Go colour library reads, and the
// gate may not execute TypeScript or open a browser — so the conversion lives
// here, next to the ratio it exists to compute.
package wcag

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// WCAG 2.2: 1.4.3 asks 4.5:1 of body text, 1.4.11 asks 3:1 of the visual
// information needed to identify a component. Large text may sit at 3:1, but a
// theme key does not know what size it will be set at, so the stricter figure
// is the safe one to hold a palette to.
const (
	TextMinimum    = 4.5
	NonTextMinimum = 3.0
)

// Pair names two theme keys that land on top of each other in the kit, and the
// component that puts them there — so a failure can say where to look.
type Pair struct {
	Foreground string
	Background string
	Where      string
}

// Which pairs actually occur is a fact about the components, not a guess:
// each Where names the code that composites them.
var (
	TextPairs = []Pair{
		{"foreground", "background", "Stage headings and body copy"},
		{"muted", "background", "secondary paragraphs on the Stage"},
		{"foreground", "surface", "Card copy and Diagram node labels"},
		{"muted", "surface", "Mono, and captions inside a Card"},
	}

	NonTextPairs = []Pair{
		{"accent", "background", "Rule, Emphasis underline, Trace strokes"},
		{"accent", "surface", "accents drawn over a Card or node"},
	}
)

// RGB holds linear-light sRGB in 0..1 — the space luminance is defined in.
type RGB struct{ R, G, B float64 }

var (
	oklchRe = regexp.MustCompile(
		`^oklch\(\s*([0-9.]+%?)\s+([0-9.]+)\s+([0-9.]+)(?:\s*/\s*[0-9.]+%?)?\s*\)$`)
	rgbRe = regexp.MustCompile(
		`^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*[0-9.]+\s*)?\)$`)
	hexRe = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)
)

// Parse reads the notations the config and the kit actually use. Anything else
// is an error rather than a default: a colour silently scored as black would
// report a contrast the render does not have.
func Parse(css string) (RGB, error) {
	s := strings.TrimSpace(css)

	if m := oklchRe.FindStringSubmatch(s); m != nil {
		l, err := parseLightness(m[1])
		if err != nil {
			return RGB{}, err
		}
		c, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return RGB{}, fmt.Errorf("chroma %q: %w", m[2], err)
		}
		h, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			return RGB{}, fmt.Errorf("hue %q: %w", m[3], err)
		}
		return fromOKLCH(l, c, h), nil
	}

	if m := hexRe.FindStringSubmatch(s); m != nil {
		var v [3]float64
		for i := 0; i < 3; i++ {
			n, _ := strconv.ParseUint(m[1][i*2:i*2+2], 16, 8)
			v[i] = linearize(float64(n) / 255)
		}
		return RGB{v[0], v[1], v[2]}, nil
	}

	if m := rgbRe.FindStringSubmatch(s); m != nil {
		var v [3]float64
		for i := 0; i < 3; i++ {
			n, err := strconv.Atoi(m[i+1])
			if err != nil || n < 0 || n > 255 {
				return RGB{}, fmt.Errorf("rgb channel %q out of range", m[i+1])
			}
			v[i] = linearize(float64(n) / 255)
		}
		return RGB{v[0], v[1], v[2]}, nil
	}

	return RGB{}, fmt.Errorf("unreadable colour %q: use oklch(), #rrggbb or rgb()", css)
}

func parseLightness(raw string) (float64, error) {
	if strings.HasSuffix(raw, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
		return v / 100, err
	}
	return strconv.ParseFloat(raw, 64)
}

func fromOKLCH(lightness, chroma, hueDegrees float64) RGB {
	h := hueDegrees * math.Pi / 180
	a := chroma * math.Cos(h)
	b := chroma * math.Sin(h)

	l := math.Pow(lightness+0.3963377774*a+0.2158037573*b, 3)
	m := math.Pow(lightness-0.1055613458*a-0.0638541728*b, 3)
	s := math.Pow(lightness-0.0894841775*a-1.291485548*b, 3)

	return RGB{
		R: clamp(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		G: clamp(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		B: clamp(-0.0041960863*l - 0.7034186147*m + 1.707614701*s),
	}
}

func clamp(v float64) float64 { return math.Min(1, math.Max(0, v)) }

func linearize(channel float64) float64 {
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

// RelativeLuminance is WCAG 2.2's definition, over linear-light sRGB.
func RelativeLuminance(c RGB) float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// ContrastRatio returns the WCAG ratio, 1 to 21. Order does not matter.
func ContrastRatio(a, b RGB) float64 {
	la, lb := RelativeLuminance(a), RelativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Level grades a ratio the way WCAG 2.2 does for text.
func Level(ratio float64) string {
	switch {
	case ratio >= 7:
		return "AAA"
	case ratio >= TextMinimum:
		return "AA"
	case ratio >= NonTextMinimum:
		return "AA-large"
	default:
		return "fail"
	}
}
