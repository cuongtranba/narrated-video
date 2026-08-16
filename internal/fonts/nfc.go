package fonts

import "golang.org/x/text/unicode/norm"

// IsNFC reports whether s is already in Unicode Normalization Form C.
//
// Non-NFC text renders unpredictably, and worse, it hides a coverage failure.
// "b" + U+00F4 + U+0301 and "b" + U+1ED1 are the same word, but the decomposed
// spelling passes a per-codepoint coverage check on any font holding U+00F4 and
// U+0301 separately, while what actually gets drawn then depends on the font's
// mark-attachment tables -- which a subset webfont need not carry. Normalize to
// NFC before asking CoveredRunes or Missing anything, so the question being
// answered is the one the reader will see.
func IsNFC(s string) bool { return norm.NFC.IsNormalString(s) }

// NormalizeNFC returns s in Unicode Normalization Form C.
func NormalizeNFC(s string) string { return norm.NFC.String(s) }
