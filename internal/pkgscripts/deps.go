package pkgscripts

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/scenekind"
)

const depsKey = "dependencies"

// Deps reports what package.json installs. The second result is false when the
// document has no dependencies object this package can read confidently — the
// same disowning the scripts get, so nv never rewrites a shape it guessed at.
func (f *File) Deps() (map[string]string, bool) {
	sp, _, ok := objectSpan(f.raw, depsKey)
	if !ok {
		return nil, false
	}
	entries, ok := depEntries(string(f.raw[sp.start:sp.end]))
	if !ok {
		return nil, false
	}

	deps := make(map[string]string, len(entries))
	for _, entry := range entries {
		deps[entry.name] = entry.version
	}
	return deps, true
}

// ReconcileDeps returns the file's bytes with every required dependency present
// at the version its kind declares, and false when nothing needed to change.
//
// It adds and corrects; it never removes. A package.json holds packages nv knows
// nothing about — one a component reached for, one the author is trying out —
// and deleting what it does not recognise would be a destructive answer to a
// question nobody asked. That is also what keeps CHK-34 congruent with this:
// everything the check reports is exactly what running `nv sync` fixes.
func (f *File) ReconcileDeps(required []scenekind.Dep) ([]byte, bool) {
	out, changed := f.raw, false
	for _, dep := range required {
		next, ok := withDep(out, dep)
		if !ok {
			continue
		}
		out, changed = next, true
	}
	if !changed {
		return nil, false
	}
	return out, true
}

// ReconcileDepsAt brings root's package.json in line with what the scene kinds
// in use require, and reports whether the file changed.
func ReconcileDepsAt(root string, required []scenekind.Dep) (bool, error) {
	if len(required) == 0 {
		return false, nil
	}
	file, err := Load(root)
	if err != nil || file == nil {
		return false, err
	}
	data, changed := file.ReconcileDeps(required)
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(file.Path, data, 0o644)
}

// withDep edits one dependency into the document, leaving every other byte —
// including the author's formatting — where it was. The span is recomputed per
// call because the previous edit moved everything after it.
func withDep(raw []byte, dep scenekind.Dep) ([]byte, bool) {
	sp, keyAt, ok := objectSpan(raw, depsKey)
	if !ok {
		return nil, false
	}
	region := string(raw[sp.start:sp.end])
	entries, ok := depEntries(region)
	if !ok {
		return nil, false
	}

	for _, entry := range entries {
		if entry.name != dep.Name {
			continue
		}
		if entry.version == dep.Version {
			return nil, false
		}
		return splice(raw, sp.start+entry.valueStart, sp.start+entry.valueEnd, quote(dep.Version)), true
	}

	start, end, text := insertion(region, entries, indentAt(raw, keyAt), dep)
	return splice(raw, sp.start+start, sp.start+end, text), true
}

// insertion places a new dependency in alphabetical order, matching the layout
// it finds. Sorted keys are not cosmetic: the block is read in review, and a
// package appended to the end of a sorted list is one a reader scans past.
func insertion(region string, entries []depEntry, baseIndent string, dep scenekind.Dep) (start, end int, text string) {
	pair := quote(dep.Name) + ": " + quote(dep.Version)

	if len(entries) == 0 {
		return 0, len(region), "{\n" + baseIndent + "  " + pair + "\n" + baseIndent + "}"
	}

	for _, entry := range entries {
		if entry.name <= dep.Name {
			continue
		}
		if indent, ownsLine := lineIndent(region, entry); ownsLine {
			return entry.lineStart, entry.lineStart, indent + pair + ",\n"
		}
		return entry.keyStart, entry.keyStart, pair + ", "
	}

	last := entries[len(entries)-1]
	if indent, ownsLine := lineIndent(region, last); ownsLine {
		return last.valueEnd, last.valueEnd, ",\n" + indent + pair
	}
	return last.valueEnd, last.valueEnd, ", " + pair
}

// depEntry is one `"name": "version"` pair and where it sits inside the
// dependencies object, so an edit can be spliced rather than reprinted.
type depEntry struct {
	name, version        string
	lineStart, keyStart  int
	valueStart, valueEnd int
}

// depEntries reads the object at the start of region. It refuses anything that
// is not a flat map of strings — a nested object or an array means this is not
// the shape nv understands, and guessing is how a rewrite corrupts a file.
func depEntries(region string) ([]depEntry, bool) {
	var entries []depEntry

	i := 1
	for i < len(region) {
		for i < len(region) && (isSpace(region[i]) || region[i] == ',') {
			i++
		}
		if i >= len(region) || region[i] == '}' {
			return entries, true
		}
		if region[i] != '"' {
			return nil, false
		}

		keyStart := i
		keyEnd, ok := stringEnd(region, keyStart)
		if !ok {
			return nil, false
		}
		var name string
		if json.Unmarshal([]byte(region[keyStart:keyEnd]), &name) != nil {
			return nil, false
		}

		i = keyEnd
		for i < len(region) && isSpace(region[i]) {
			i++
		}
		if i >= len(region) || region[i] != ':' {
			return nil, false
		}
		i++
		for i < len(region) && isSpace(region[i]) {
			i++
		}

		valueStart := i
		valueEnd, ok := stringEnd(region, valueStart)
		if !ok {
			return nil, false
		}
		var version string
		if json.Unmarshal([]byte(region[valueStart:valueEnd]), &version) != nil {
			return nil, false
		}

		entries = append(entries, depEntry{
			name:       name,
			version:    version,
			lineStart:  strings.LastIndexByte(region[:keyStart], '\n') + 1,
			keyStart:   keyStart,
			valueStart: valueStart,
			valueEnd:   valueEnd,
		})
		i = valueEnd
	}
	return nil, false
}

// lineIndent returns the whitespace an entry sits behind, and false when the
// entry shares its line with something else — a compact object, where inserting
// a line would reformat what the author wrote.
func lineIndent(region string, entry depEntry) (string, bool) {
	prefix := region[entry.lineStart:entry.keyStart]
	if strings.TrimSpace(prefix) != "" {
		return "", false
	}
	return prefix, true
}

func indentAt(raw []byte, at int) string {
	doc := string(raw)
	start := strings.LastIndexByte(doc[:at], '\n') + 1
	if strings.TrimSpace(doc[start:at]) != "" {
		return ""
	}
	return doc[start:at]
}

func splice(raw []byte, start, end int, text string) []byte {
	out := make([]byte, 0, len(raw)+len(text))
	out = append(out, raw[:start]...)
	out = append(out, text...)
	return append(out, raw[end:]...)
}

func quote(s string) string {
	encoded, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(encoded)
}
