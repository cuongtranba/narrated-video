package checks

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cuongtranba/narrated-video/internal/fonts"
	"github.com/cuongtranba/narrated-video/internal/project"
)

// CHK-11. TypeScript already makes a missing KEY a compile error. What it
// cannot see is array LENGTH: a list of five gates in one language and three in
// another satisfies the same type, and the scene renders a short table in
// silence. The reference project caught this once, by hand-writing an assertion
// about one specific field — which does not generalise. A structural walk does.
func copyCongruent(kit *Kit) Result {
	const id, title = "CHK-11", "every locale has the same copy structure as the default"

	defaultLocale := kit.Config.Locales.Default
	reference := kit.Content[defaultLocale].Copy

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		if locale == defaultLocale {
			continue
		}
		for _, mismatch := range compareShape(reference, kit.Content[locale].Copy, "copy") {
			findings = append(findings, Finding{Where: locale + " " + mismatch.path, Detail: mismatch.detail})
		}
	}
	return fail(id, title, fmt.Sprintf("bring the locale's content file into the same shape as %s.yaml", defaultLocale), sortedFindings(findings))
}

type shapeMismatch struct{ path, detail string }

func compareShape(want, got any, path string) []shapeMismatch {
	switch wantTyped := want.(type) {
	case map[string]any:
		gotTyped, ok := got.(map[string]any)
		if !ok {
			return []shapeMismatch{{path, fmt.Sprintf("expected an object, found %s", describe(got))}}
		}
		var out []shapeMismatch
		for _, key := range project.SortedKeys(wantTyped) {
			child, present := gotTyped[key]
			if !present {
				out = append(out, shapeMismatch{path + "." + key, "missing"})
				continue
			}
			out = append(out, compareShape(wantTyped[key], child, path+"."+key)...)
		}
		for _, key := range project.SortedKeys(gotTyped) {
			if _, expected := wantTyped[key]; !expected {
				out = append(out, shapeMismatch{path + "." + key, "not present in the default locale"})
			}
		}
		return out

	case []any:
		gotTyped, ok := got.([]any)
		if !ok {
			return []shapeMismatch{{path, fmt.Sprintf("expected a list, found %s", describe(got))}}
		}
		if len(wantTyped) != len(gotTyped) {
			return []shapeMismatch{{path, fmt.Sprintf("has %d entries, the default locale has %d", len(gotTyped), len(wantTyped))}}
		}
		var out []shapeMismatch
		for i := range wantTyped {
			out = append(out, compareShape(wantTyped[i], gotTyped[i], fmt.Sprintf("%s[%d]", path, i))...)
		}
		return out
	}

	if describe(want) != describe(got) {
		return []shapeMismatch{{path, fmt.Sprintf("expected %s, found %s", describe(want), describe(got))}}
	}
	return nil
}

func describe(v any) string {
	switch v.(type) {
	case string:
		return "text"
	case bool:
		return "a boolean"
	case int, int64, float64:
		return "a number"
	case []any:
		return "a list"
	case map[string]any:
		return "an object"
	case nil:
		return "nothing"
	}
	return fmt.Sprintf("%T", v)
}

// CHK-14. A translation runs longer than its source and pushes a card off the
// frame. This is a proxy for that, not a proof of it — the budget is derived
// from the default locale so nobody maintains a per-slot table, and the only
// tuned number is one factor per language.
func copyFitsBudget(kit *Kit) Result {
	const id, title = "CHK-14", "translated copy stays within its length budget"

	defaultLocale := kit.Config.Locales.Default
	reference := map[string]string{}
	collectStrings(kit.Content[defaultLocale].Copy, "copy", reference)

	var findings []Finding
	for _, locale := range kit.Config.Locales.List {
		if locale.Code == defaultLocale {
			continue
		}
		translated := map[string]string{}
		collectStrings(kit.Content[locale.Code].Copy, "copy", translated)

		for _, path := range sortedStringKeys(translated) {
			source, ok := reference[path]
			if !ok {
				continue
			}
			budget := int(float64(utf8.RuneCountInString(source)) * locale.ExpansionFactor)
			if actual := utf8.RuneCountInString(translated[path]); actual > budget {
				findings = append(findings, Finding{
					Where:  locale.Code + " " + path,
					Detail: fmt.Sprintf("%d characters against a budget of %d", actual, budget),
				})
			}
		}
	}
	return fail(id, title, "shorten the line, or raise this locale's expansionFactor if the layout really does fit", sortedFindings(findings))
}

// CHK-15. Some strings are things the system itself prints. Translating one
// makes the video contradict the product it explains, and the reader who greps
// for what they saw finds nothing.
func printedLiterals(kit *Kit) Result {
	const id, title = "CHK-15", "strings the system prints survive translation verbatim"

	literals := kit.Config.PrintedLiterals
	if len(literals) == 0 {
		return pass(id, title)
	}

	defaultLocale := kit.Config.Locales.Default
	reference := map[string]string{}
	collectStrings(kit.Content[defaultLocale].Copy, "copy", reference)

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		if locale == defaultLocale {
			continue
		}
		translated := map[string]string{}
		collectStrings(kit.Content[locale].Copy, "copy", translated)

		for _, path := range sortedStringKeys(reference) {
			for _, literal := range literals {
				if !strings.Contains(reference[path], literal) {
					continue
				}
				if !strings.Contains(translated[path], literal) {
					findings = append(findings, Finding{
						Where:  locale + " " + path,
						Detail: fmt.Sprintf("%q is a string the system prints, and it was translated away", literal),
					})
				}
			}
		}
	}
	return fail(id, title, "restore the literal exactly as the system prints it", sortedFindings(findings))
}

// CHK-16. Domain vocabulary stays in the source language because the audience
// speaks it that way, and because a translated term stops being greppable.
//
// The reference project's version of this check was close to vacuous: it
// searched a JSON dump of the whole copy object, which embedded code literals
// the translator never touches, and it matched without word boundaries, so
// "turn" was satisfied by "returns". This one searches prose only, whole words
// only, and skips any string that is itself a printed literal.
func glossaryUntranslated(kit *Kit) Result {
	const id, title = "CHK-16", "domain terms are not translated away"

	if len(kit.Config.Glossary) == 0 {
		return pass(id, title)
	}

	defaultLocale := kit.Config.Locales.Default
	reference := map[string]string{}
	collectStrings(kit.Content[defaultLocale].Copy, "copy", reference)

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		if locale == defaultLocale {
			continue
		}
		translated := map[string]string{}
		collectStrings(kit.Content[locale].Copy, "copy", translated)

		for _, term := range kit.Config.Glossary {
			word := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
			for _, path := range sortedStringKeys(reference) {
				if isPrintedLiteral(kit, reference[path]) || !word.MatchString(reference[path]) {
					continue
				}
				if !word.MatchString(translated[path]) {
					findings = append(findings, Finding{
						Where:  locale + " " + path,
						Detail: fmt.Sprintf("%q does not survive here — the audience says it in the source language and greps for it", term),
					})
				}
			}
		}
	}
	return fail(id, title, "keep the term in the source language, or drop it from the glossary if it really should be translated", sortedFindings(findings))
}

func isPrintedLiteral(kit *Kit, s string) bool {
	for _, literal := range kit.Config.PrintedLiterals {
		if s == literal {
			return true
		}
	}
	return false
}

// CHK-17. A composed character and its decomposed spelling look identical in a
// review and behave differently on screen: the decomposed form depends on the
// font's mark attachment, which is where stacked diacritics go missing.
func copyIsNFC(kit *Kit) Result {
	const id, title = "CHK-17", "text is normalized so stacked marks render as one glyph"

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		text := map[string]string{}
		collectStrings(kit.Content[locale].Copy, "copy", text)
		for scene, line := range kit.Content[locale].Narration {
			text["narration."+scene] = line
		}

		for _, path := range sortedStringKeys(text) {
			if !fonts.IsNFC(text[path]) {
				findings = append(findings, Finding{
					Where:  locale + " " + path,
					Detail: "text is decomposed; it looks correct in review and may render without its marks",
				})
			}
		}
	}
	return fail(id, title, "normalize the file to NFC", sortedFindings(findings))
}

func collectStrings(v any, path string, out map[string]string) {
	switch typed := v.(type) {
	case string:
		out[path] = typed
	case map[string]any:
		for _, key := range project.SortedKeys(typed) {
			collectStrings(typed[key], path+"."+key, out)
		}
	case []any:
		for i, item := range typed {
			collectStrings(item, fmt.Sprintf("%s[%d]", path, i), out)
		}
	}
}
