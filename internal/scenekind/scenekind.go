// Package scenekind names the kinds of scene the tool can scaffold, and what
// each one costs the project that uses it.
//
// A kind is a value rather than a branch. `nv init --kind`, the dependency
// reconciliation in package.json and CHK-34 all read this one list, so none of
// them can learn about a kind the others have not — the failure that pattern
// prevents is a scene that scaffolds fine and cannot resolve its import.
//
// A scene's kind is never recorded anywhere. It is read back out of the source,
// because a field in the config would be a second answer that the imports could
// contradict, and the one that lies is always the one nobody is looking at.
package scenekind

import (
	"sort"
	"strings"
)

// Dep is an npm package a kind cannot render without, at the range the kit is
// known to work against.
type Dep struct {
	Name    string
	Version string
}

type Kind struct {
	Name string
	// TemplatePath is the file inside kit.FS that `nv init --scene` copies.
	TemplatePath string
	Dependencies []Dep
	// Reference is where the author reads about this kind once it is scaffolded.
	Reference string
}

// Text is the kind a scene has when nothing in its source says otherwise: it
// needs nothing beyond what every project already installs.
const Text = "text"

var kinds = []Kind{
	{
		Name:         Text,
		TemplatePath: "src/scenes/_template.tsx",
		Reference:    "references/scene-registry.md",
	},
	{
		Name:         "flow",
		TemplatePath: "src/scenes/_template.flow.tsx",
		Dependencies: []Dep{{Name: "@xyflow/react", Version: "^12.0.0"}},
		Reference:    "references/scene-registry.md#flow",
	},
	{
		Name:         "space",
		TemplatePath: "src/scenes/_template.space.tsx",
		Dependencies: []Dep{
			{Name: "@react-three/fiber", Version: "^9.0.0"},
			{Name: "@remotion/three", Version: "^4.0.0"},
			{Name: "three", Version: "^0.175.0"},
		},
		Reference: "references/scene-registry.md#space",
	},
}

// All returns every kind in definition order.
func All() []Kind { return append([]Kind(nil), kinds...) }

func Lookup(name string) (Kind, bool) {
	for _, kind := range kinds {
		if kind.Name == name {
			return kind, true
		}
	}
	return Kind{}, false
}

// Names is the list a wrong --kind is shown.
func Names() []string {
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		names = append(names, kind.Name)
	}
	return names
}

// Of reports the kind a scene module is written in, judged by the packages it
// imports. A module that imports nothing kind-specific is text.
func Of(source string) Kind {
	for _, kind := range kinds {
		for _, dep := range kind.Dependencies {
			if importsPackage(source, dep.Name) {
				return kind
			}
		}
	}
	text, _ := Lookup(Text)
	return text
}

// Required is the union of what the given scene modules need, sorted by package
// name so the answer reads the same way twice.
func Required(sources map[string]string) []Dep {
	union := map[string]Dep{}
	for _, source := range sources {
		for _, dep := range Of(source).Dependencies {
			union[dep.Name] = dep
		}
	}

	required := make([]Dep, 0, len(union))
	for _, dep := range union {
		required = append(required, dep)
	}
	sort.Slice(required, func(i, j int) bool { return required[i].Name < required[j].Name })
	return required
}

// importsPackage matches the whole module specifier rather than a substring.
// "three" must not be found inside "@remotion/three", and a deep import of
// "three/examples/…" must still count as three.
func importsPackage(source, pkg string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import") && !strings.HasPrefix(trimmed, "export") {
			continue
		}
		spec, ok := moduleSpecifier(trimmed)
		if !ok {
			continue
		}
		if spec == pkg || strings.HasPrefix(spec, pkg+"/") {
			return true
		}
	}
	return false
}

func moduleSpecifier(line string) (string, bool) {
	end := strings.LastIndexAny(line, `"'`)
	if end < 0 {
		return "", false
	}
	start := strings.LastIndexByte(line[:end], line[end])
	if start < 0 {
		return "", false
	}
	return line[start+1 : end], true
}
