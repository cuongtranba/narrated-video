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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuongtranba/narrated-video/kit"
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
	// Packages names what the kind imports. The range each one resolves to is
	// not written here: it lives in the template's package.json, which is the
	// file a scaffolded project actually receives. A version repeated here
	// could disagree with the one installed, and CHK-34 compares them exactly —
	// so the disagreement would surface as a check failure in the user's
	// project, with `nv sync` "fixing" it by undoing their upgrade.
	Packages []string
	// Reference is where the author reads about this kind once it is scaffolded.
	Reference string
}

// Dependencies pairs each of the kind's packages with the range the template
// installs it at.
func (k Kind) Dependencies() []Dep {
	deps := make([]Dep, 0, len(k.Packages))
	for _, name := range k.Packages {
		deps = append(deps, Dep{Name: name, Version: templateVersions[name]})
	}
	return deps
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
		Packages:     []string{"@xyflow/react"},
		Reference:    "references/scene-registry.md#flow",
	},
	{
		Name:         "space",
		TemplatePath: "src/scenes/_template.space.tsx",
		Packages:     []string{"@react-three/fiber", "@remotion/three", "three"},
		Reference:    "references/scene-registry.md#space",
	},
}

// templateVersions is what the embedded package.json installs, keyed by package
// name. Reading it at init makes a kind that names a package the template does
// not install a build-time failure rather than a scaffolded project whose
// import cannot resolve.
var templateVersions = mustTemplateVersions()

func mustTemplateVersions() map[string]string {
	raw, err := kit.FS.ReadFile("package.json")
	if err != nil {
		panic(fmt.Sprintf("scenekind: reading the template package.json: %v", err))
	}
	versions, err := templateVersionsFrom(raw)
	if err != nil {
		panic("scenekind: " + err.Error())
	}
	return versions
}

// templateVersionsFrom reads the dependency ranges out of a package.json and
// refuses one that does not install every package the kinds import — the case
// that would otherwise scaffold a scene whose import cannot resolve, and leave
// CHK-34 comparing against an empty version.
func templateVersionsFrom(raw []byte) (map[string]string, error) {
	var doc struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing the template package.json: %w", err)
	}

	for _, kind := range kinds {
		for _, name := range kind.Packages {
			if doc.Dependencies[name] == "" {
				return nil, fmt.Errorf("the %s kind needs %s, which kit/package.json does not install", kind.Name, name)
			}
		}
	}
	return doc.Dependencies, nil
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
		for _, name := range kind.Packages {
			if importsPackage(source, name) {
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
		for _, dep := range Of(source).Dependencies() {
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
