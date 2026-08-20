package checks

import (
	"fmt"
	"regexp"
	"strings"
)

// A colour written into a scene is a colour the palette does not know about:
// CHK-39 and CHK-40 cannot score it, a theme change cannot reach it, and a
// second scene reaching for "roughly the same green" makes two.
var colourLiteralPatterns = []struct {
	re   *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`), "a hex colour"},
	{regexp.MustCompile(`\brgba?\s*\(`), "an rgb() colour"},
	{regexp.MustCompile(`\bhsla?\s*\(`), "an hsl() colour"},
	{regexp.MustCompile(`\bokl(ch|ab)\s*\(`), "an oklch() colour"},
}

// CHK-41. One accent doing one job is what makes eight scenes read as one
// video. The moment a scene names its own colour, the config stops being the
// palette — and the contrast gate goes blind, because it scores `theme` and a
// literal is not in it.
//
// Comments are stripped; string bodies are not, since a colour literal almost
// always lives inside quotes.
func scenesUseThemeColours(kit *Kit) Result {
	const id, title = "CHK-41", "scene modules take colour from the theme, never a literal"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for i, raw := range strings.Split(kit.SceneSources[sceneID], "\n") {
			line := stripComment(raw)
			for _, p := range colourLiteralPatterns {
				if match := p.re.FindString(line); match != "" {
					findings = append(findings, Finding{
						Where:  fmt.Sprintf("%s:%d", sceneID, i+1),
						Detail: fmt.Sprintf("%s (%s) — read it from THEME, or THEME_3D on a three.js material", p.kind, strings.TrimSpace(match)),
					})
					break
				}
			}
		}
	}

	return fail(id, title,
		"add the colour to `theme` in video.config.yaml and read it from THEME — a literal is unreachable by a theme change and unscorable by CHK-39",
		sortedFindings(findings))
}

// Matches a font size given as a bare number rather than a step on the scale.
var magicFontSize = regexp.MustCompile(`fontSize\s*[:=]\s*\{?\s*(\d+(?:\.\d+)?)`)

// CHK-42. A type scale is what stops six scenes from arriving at six slightly
// different heading sizes. `SIZE` holds the six steps the kit is designed
// around; a bare number is a seventh that nothing else will ever match, and at
// 1080p the difference between 44 and 46 is invisible in review and obvious in
// a cut.
func scenesUseTypeScale(kit *Kit) Result {
	const id, title = "CHK-42", "scene modules size type from the shared scale"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for i, raw := range strings.Split(kit.SceneSources[sceneID], "\n") {
			line := stripComment(raw)
			if match := magicFontSize.FindStringSubmatch(line); match != nil {
				findings = append(findings, Finding{
					Where:  fmt.Sprintf("%s:%d", sceneID, i+1),
					Detail: fmt.Sprintf("fontSize: %s — use a step from SIZE (display, heading, lead, body, mono, label)", match[1]),
				})
			}
		}
	}

	return fail(id, title,
		"size type with SIZE from ../components/primitives; if no step fits, the scale is what should change",
		sortedFindings(findings))
}

// stripComment removes a // suffix and any /* */ span on one line, leaving
// string bodies intact — the opposite trade to stripStringsAndComment, because
// the tokens these checks hunt live inside quotes.
func stripComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		line = line[:i]
	}
	for {
		open := strings.Index(line, "/*")
		if open < 0 {
			break
		}
		close := strings.Index(line[open:], "*/")
		if close < 0 {
			return line[:open]
		}
		line = line[:open] + line[open+close+2:]
	}
	return line
}
