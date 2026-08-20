package checks

import (
	"fmt"
	"sort"

	"github.com/cuongtranba/narrated-video/internal/wcag"
)

// CHK-39. Contrast is the one accessibility property a video cannot recover
// from: there is no zoom, no reader mode, no user stylesheet, and no way to ask
// the frame again. It is also invisible to every other check here — a palette
// with 2:1 body text renders perfectly, exits 0, and is unreadable on a laptop
// in daylight or through a projector.
//
// Scored per WCAG 2.2 SC 1.4.3, against the pairs the components actually
// composite rather than every combination the map allows.
func themeTextContrast(kit *Kit) Result {
	const id, title = "CHK-39", "text colours meet WCAG AA contrast against what they sit on"
	const remedy = "raise the lightness gap between the pair named above — see references/ui-consistency.md"

	return fail(id, title, remedy, contrastFindings(kit, wcag.TextPairs, wcag.TextMinimum))
}

// CHK-40. SC 1.4.11: the accent is not decoration. It is the Rule that marks
// where the voice starts, the underline under the term being defined, and the
// stroke of every diagram edge — all of it information carried by one colour
// against one background. Below 3:1 that information is gone for a viewer with
// low vision, and gone for everyone on a washed-out projector.
func themeAccentContrast(kit *Kit) Result {
	const id, title = "CHK-40", "accent meets WCAG contrast for the information it carries"
	const remedy = "raise the accent's lightness against the surface named above, or darken that surface"

	return fail(id, title, remedy, contrastFindings(kit, wcag.NonTextPairs, wcag.NonTextMinimum))
}

// Only declared pairs are scored. `theme` is optional and a project may set
// three keys and let the components' own defaults cover the rest, so an absent
// colour is a choice rather than a fault — and inventing a value to score
// against would report a contrast the render does not have.
func contrastFindings(kit *Kit, pairs []wcag.Pair, minimum float64) []Finding {
	var findings []Finding
	for _, pair := range pairs {
		rawFg, rawBg := kit.Config.Theme[pair.Foreground], kit.Config.Theme[pair.Background]
		if rawFg == "" || rawBg == "" {
			continue
		}

		fg, err := wcag.Parse(rawFg)
		if err != nil {
			findings = append(findings, Finding{
				Where:  "video.config.yaml theme." + pair.Foreground,
				Detail: err.Error(),
			})
			continue
		}
		bg, err := wcag.Parse(rawBg)
		if err != nil {
			findings = append(findings, Finding{
				Where:  "video.config.yaml theme." + pair.Background,
				Detail: err.Error(),
			})
			continue
		}

		ratio := wcag.ContrastRatio(fg, bg)
		if ratio < minimum {
			findings = append(findings, Finding{
				Where: fmt.Sprintf("theme.%s on theme.%s", pair.Foreground, pair.Background),
				Detail: fmt.Sprintf("%.2f:1 (%s) — %s needs at least %.1f:1",
					ratio, wcag.Level(ratio), pair.Where, minimum),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Where < findings[j].Where })
	return findings
}
