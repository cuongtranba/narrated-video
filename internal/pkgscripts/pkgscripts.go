// Package pkgscripts owns the composition id inside package.json's render and
// still scripts.
//
// It exists because the id had two homes. `video.config.yaml` declares it and
// `nv sync` writes it into src/generated/, while package.json carried a copy
// typed once by the template and never revisited — so renaming the composition
// left `bun run render` pointing at one that no longer exists. The failure
// arrives after the voiceover has been paid for, which is the expensive moment
// to discover it.
//
// Only the id is claimed. Flags, the output path and every other script are the
// author's, and a script this package cannot read confidently is left exactly
// as it was.
package pkgscripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "package.json"

// Managed are the scripts whose composition id is derived from the config.
var Managed = []string{"render", "still"}

// Target reports the composition id a script renders, and whether the script is
// one this package understands.
//
// The rule is deliberately narrow: `remotion render` or `remotion still`
// immediately followed by a non-flag token. A script that puts anything else
// there is not refused, it is disowned — reading it as an id would let `nv sync`
// rewrite a flag.
func Target(script string) (string, bool) {
	tokens := tokenize(script)
	for i, tok := range tokens {
		if tok.text != "remotion" {
			continue
		}
		if i+2 >= len(tokens) {
			return "", false
		}
		if sub := tokens[i+1].text; sub != "render" && sub != "still" {
			return "", false
		}
		if id := tokens[i+2].text; !strings.HasPrefix(id, "-") {
			return id, true
		}
		return "", false
	}
	return "", false
}

// Retarget rewrites the composition id and leaves every other byte, including
// the spacing, where it was. A script Target disowns comes back unchanged.
func Retarget(script, id string) string {
	tokens := tokenize(script)
	for i, tok := range tokens {
		if tok.text != "remotion" || i+2 >= len(tokens) {
			continue
		}
		if sub := tokens[i+1].text; sub != "render" && sub != "still" {
			return script
		}
		cur := tokens[i+2]
		if strings.HasPrefix(cur.text, "-") {
			return script
		}
		return script[:cur.start] + id + script[cur.end:]
	}
	return script
}

type token struct {
	text       string
	start, end int
}

func tokenize(s string) []token {
	var tokens []token
	start := -1
	for i, r := range s {
		if r == ' ' || r == '\t' {
			if start >= 0 {
				tokens = append(tokens, token{s[start:i], start, i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		tokens = append(tokens, token{s[start:], start, len(s)})
	}
	return tokens
}

// File is a package.json nv can read. Its raw bytes are kept so a rewrite
// preserves the author's formatting rather than reprinting the document.
type File struct {
	Path    string
	raw     []byte
	scripts map[string]string
	span    span
}

type span struct{ start, end int }

// Load reads the project's package.json. A file that is absent, unreadable, not
// JSON, or has no scripts object yields (nil, nil): it belongs to the JavaScript
// project, and nv validates only what it derives.
func Load(root string) (*File, error) {
	path := filepath.Join(root, FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var doc struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Scripts == nil {
		return nil, nil
	}

	sp, ok := scriptsSpan(raw)
	if !ok {
		return nil, nil
	}
	return &File{Path: path, raw: raw, scripts: doc.Scripts, span: sp}, nil
}

// Target reports the composition a managed script renders. The second result is
// false when the script is absent or written in a form this package disowns.
func (f *File) Target(name string) (string, bool) {
	script, ok := f.scripts[name]
	if !ok {
		return "", false
	}
	return Target(script)
}

// Retargeted returns the file's bytes with every managed script pointed at id,
// and false when nothing needed to change.
func (f *File) Retargeted(id string) ([]byte, bool) {
	out := f.raw
	changed := false
	for _, name := range Managed {
		script, ok := f.scripts[name]
		if !ok {
			continue
		}
		want := Retarget(script, id)
		if want == script {
			continue
		}
		next, ok := replaceScript(out, f.span, name, want)
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

// replaceScript swaps one script's JSON string literal inside the scripts
// object. Anchoring to that span keeps a `"render"` key elsewhere in the
// document — a dependency of that name, say — out of reach.
func replaceScript(raw []byte, sp span, name, value string) ([]byte, bool) {
	region := string(raw[sp.start:sp.end])
	key := `"` + name + `"`

	at := strings.Index(region, key)
	if at < 0 {
		return nil, false
	}
	i := at + len(key)
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
	end, ok := stringEnd(region, i)
	if !ok {
		return nil, false
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	out := make([]byte, 0, len(raw)+len(encoded))
	out = append(out, raw[:sp.start+i]...)
	out = append(out, encoded...)
	out = append(out, raw[sp.start+end:]...)
	return out, true
}

// scriptsSpan locates the byte range of the scripts object's value.
func scriptsSpan(raw []byte) (span, bool) {
	doc := string(raw)
	at := strings.Index(doc, `"scripts"`)
	if at < 0 {
		return span{}, false
	}
	i := at + len(`"scripts"`)
	for i < len(doc) && isSpace(doc[i]) {
		i++
	}
	if i >= len(doc) || doc[i] != ':' {
		return span{}, false
	}
	i++
	for i < len(doc) && isSpace(doc[i]) {
		i++
	}
	if i >= len(doc) || doc[i] != '{' {
		return span{}, false
	}

	depth, inString, escaped := 0, false, false
	for j := i; j < len(doc); j++ {
		c := doc[j]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return span{i, j + 1}, true
			}
		}
	}
	return span{}, false
}

// stringEnd returns the index just past the JSON string literal starting at i.
func stringEnd(s string, i int) (int, bool) {
	if i >= len(s) || s[i] != '"' {
		return 0, false
	}
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1, true
		}
	}
	return 0, false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
