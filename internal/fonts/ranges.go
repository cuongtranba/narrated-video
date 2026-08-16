package fonts

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// RuneRange is an inclusive codepoint interval.
type RuneRange struct{ Lo, Hi rune }

// ParseUnicodeRanges parses CSS unicode-range syntax: "U+0102-0103",
// "U+1EA0-1EF9", "U+00??", or a comma-separated list of those.
//
// What this answers is what a font DECLARES, which is a weaker claim than what
// it contains: the font that motivated this package declares the vietnamese
// subset and is still missing six of the language's commonest vowels. Use it to
// pick a @font-face src, never to conclude that text will render -- that is
// CoveredRunes' question.
func ParseUnicodeRanges(spec string) ([]RuneRange, error) {
	elements := strings.Split(spec, ",")
	ranges := make([]RuneRange, 0, len(elements))
	for _, element := range elements {
		r, err := parseUnicodeRange(strings.TrimSpace(element))
		if err != nil {
			return nil, fmt.Errorf("fonts: unicode-range %q: %w", spec, err)
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func parseUnicodeRange(element string) (RuneRange, error) {
	if element == "" {
		return RuneRange{}, fmt.Errorf("empty element")
	}
	if len(element) < 3 || (element[0] != 'U' && element[0] != 'u') || element[1] != '+' {
		return RuneRange{}, fmt.Errorf("element %q does not start with U+", element)
	}
	body := element[2:]

	if wildcard := strings.IndexByte(body, '?'); wildcard >= 0 {
		digits, marks := body[:wildcard], body[wildcard:]
		if strings.Trim(marks, "?") != "" {
			return RuneRange{}, fmt.Errorf("element %q: ? wildcards must all be trailing and cannot be combined with a range", element)
		}
		lo, err := parseHexCodepoint(digits + strings.Repeat("0", len(marks)))
		if err != nil {
			return RuneRange{}, fmt.Errorf("element %q: %w", element, err)
		}
		hi, err := parseHexCodepoint(digits + strings.Repeat("F", len(marks)))
		if err != nil {
			return RuneRange{}, fmt.Errorf("element %q: %w", element, err)
		}
		return RuneRange{Lo: lo, Hi: hi}, nil
	}

	loText, hiText, isRange := strings.Cut(body, "-")
	lo, err := parseHexCodepoint(loText)
	if err != nil {
		return RuneRange{}, fmt.Errorf("element %q: %w", element, err)
	}
	if !isRange {
		return RuneRange{Lo: lo, Hi: lo}, nil
	}
	hi, err := parseHexCodepoint(hiText)
	if err != nil {
		return RuneRange{}, fmt.Errorf("element %q: %w", element, err)
	}
	if hi < lo {
		return RuneRange{}, fmt.Errorf("element %q ends at U+%04X, before it starts at U+%04X", element, hi, lo)
	}
	return RuneRange{Lo: lo, Hi: hi}, nil
}

func parseHexCodepoint(text string) (rune, error) {
	if len(text) == 0 || len(text) > 6 {
		return 0, fmt.Errorf("%q is not 1 to 6 hex digits", text)
	}
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not hexadecimal", text)
	}
	if value > unicode.MaxRune {
		return 0, fmt.Errorf("U+%X is beyond the Unicode maximum U+10FFFF", value)
	}
	return rune(value), nil
}

// RangesCover returns the runes of sample not covered by any range, in
// first-appearance order.
func RangesCover(ranges []RuneRange, sample string) (missing []rune) {
	return absentRunes(sample, func(r rune) bool {
		for _, candidate := range ranges {
			if r >= candidate.Lo && r <= candidate.Hi {
				return true
			}
		}
		return false
	})
}
