package scenekind

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/kit"
)

// Every kind promises a template. A kind whose template is not in the binary
// scaffolds an error message instead of a scene, and the first person to find
// out is the author who asked for that kind.
func TestAll_TemplatesShipInsideTheBinary(t *testing.T) {
	for _, kind := range All() {
		if _, err := kit.FS.ReadFile(kind.TemplatePath); err != nil {
			t.Errorf("kind %q names %s, which is not in the kit: %v", kind.Name, kind.TemplatePath, err)
		}
	}
}

// The export name is load-bearing: the generated registry imports
// `{ Scene as <Id> }`, so a template that exports anything else is a compile
// error in a generated file the author never wrote.
func TestAll_TemplatesExportExactlyOneScene(t *testing.T) {
	exported := regexp.MustCompile(`(?m)^export\s+(const|function|default)\s+(\w+)?`)

	for _, kind := range All() {
		source := templateSource(t, kind)

		var names []string
		for _, match := range exported.FindAllStringSubmatch(source, -1) {
			names = append(names, match[1]+" "+match[2])
		}
		if len(names) != 1 || names[0] != "const Scene" {
			t.Errorf("kind %q exports %v, want exactly [const Scene]", kind.Name, names)
		}
	}
}

// CHK-21 and CHK-22 hold for every scene in a project, so a template that
// violates either ships a project that fails its own gate on the first sync.
func TestAll_TemplatesSatisfyTheSceneInvariants(t *testing.T) {
	absoluteCue := regexp.MustCompile(`\bat\(\s*[^,()]+,\s*\d+(?:\.\d+)?\s*\)`)

	for _, kind := range All() {
		source := templateSource(t, kind)

		for _, forbidden := range []string{"generated/timeline", "generated/registry", "video.config"} {
			for _, line := range strings.Split(source, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "import") && strings.Contains(trimmed, forbidden) {
					t.Errorf("kind %q imports %s — CHK-21 fails the moment it is scaffolded", kind.Name, forbidden)
				}
			}
		}
		if match := absoluteCue.FindString(source); match != "" {
			t.Errorf("kind %q hardcodes a frame count in %s — CHK-22 fails on it", kind.Name, match)
		}
	}
}

// A kind that needs a package the project does not install renders a blank
// frame at best, so every declared dependency must carry a version to install.
func TestAll_DependenciesAreVersioned(t *testing.T) {
	for _, kind := range All() {
		for _, dep := range kind.Dependencies() {
			if dep.Name == "" || dep.Version == "" {
				t.Errorf("kind %q declares %+v, want both a name and a version", kind.Name, dep)
			}
		}
	}
}

func TestLookup(t *testing.T) {
	text, ok := Lookup(Text)
	if !ok {
		t.Fatal("the default kind is not in the registry")
	}
	if text.TemplatePath != "src/scenes/_template.tsx" || len(text.Packages) != 0 {
		t.Errorf("text = %+v, want the original template and no extra dependencies", text)
	}

	if kind, ok := Lookup("nope"); ok {
		t.Errorf("Lookup(\"nope\") = %+v, true; want false", kind)
	}
}

func TestOf_ReadsTheKindBackOutOfTheImports(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "nothing kind-specific",
			source: `import { Stage } from "../components/stage"`,
			want:   Text,
		},
		{
			name:   "react flow",
			source: "import { ReactFlow } from \"@xyflow/react\"\n",
			want:   "flow",
		},
		{
			name:   "three",
			source: "import { Mesh } from \"three\"\n",
			want:   "space",
		},
		{
			name:   "a deep import still names its package",
			source: "import { OrbitControls } from \"three/examples/jsm/controls/OrbitControls\"\n",
			want:   "space",
		},
		{
			// The substring trap: "@remotion/three" ends in "three", and a
			// project that only imports it is not a space scene.
			name:   "a package that merely ends in a dependency name",
			source: "import { ThreeCanvas } from \"@remotion/media\"\n",
			want:   Text,
		},
		{
			name:   "a relative path that mentions a package name",
			source: "import { three } from \"../components/three\"\n",
			want:   Text,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Of(tc.source).Name; got != tc.want {
				t.Fatalf("Of() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequired_UnionsTheKindsInUse(t *testing.T) {
	required := Required(map[string]string{
		"Intro":    `import { Stage } from "../components/stage"`,
		"Pipeline": `import { ReactFlow } from "@xyflow/react"`,
		"Graph":    `import { ReactFlow } from "@xyflow/react"`,
		"World":    `import { Mesh } from "three"`,
	})

	var names []string
	for _, dep := range required {
		names = append(names, dep.Name)
	}
	want := []string{"@react-three/fiber", "@remotion/three", "@xyflow/react", "three"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("Required() = %v, want %v sorted and deduplicated", names, want)
	}

	if got := Required(map[string]string{"Intro": "export const Scene = () => null"}); len(got) != 0 {
		t.Fatalf("a project of text scenes requires %v, want nothing", got)
	}
}

// The kit's shipped reference cut must include at least one scene that uses the
// Diagram component. Templates are excluded — a template that demonstrates what
// to fill in is not the same as an example that demonstrates the vocabulary.
func TestKit_ReferenceScenesDemonstrateFlowKind(t *testing.T) {
	entries, err := kit.FS.ReadDir("src/scenes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Type()&fs.ModeDir != 0 {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") || !strings.HasSuffix(entry.Name(), ".tsx") {
			continue
		}
		data, err := kit.FS.ReadFile("src/scenes/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `from "../components/diagram"`) {
			return
		}
	}
	t.Error(`kit reference cut ships no scene using the Diagram component; add a flow scene to kit/src/scenes/`)
}

// The template is where every version now lives, so it is where a Remotion
// package can drift away from the rest of its family — and Remotion will not
// work across a version boundary. `@remotion/three@4.0.513` depends on
// `remotion@4.0.513` exactly, so a range here installs a second copy of Remotion
// beside the pinned one, and the render that follows is wrong rather than
// broken. This is CHK-38 applied to the template itself, without a network
// round-trip: Dependabot bumps the family in a batch, and a range is exactly
// what its batch cannot hold.
func TestTemplateVersions_EveryRemotionPackageIsOnOneExactVersion(t *testing.T) {
	raw, err := kit.FS.ReadFile("package.json")
	if err != nil {
		t.Fatal(err)
	}
	// Read the file rather than templateVersions, which holds only the runtime
	// dependencies: @remotion/eslint-config-flat is a devDependency, and a
	// version boundary is one wherever it is declared.
	var doc struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	want := doc.Dependencies["remotion"]
	if want == "" {
		t.Fatal("kit/package.json installs no remotion")
	}

	for block, deps := range map[string]map[string]string{
		"dependencies":    doc.Dependencies,
		"devDependencies": doc.DevDependencies,
	} {
		for name, version := range deps {
			if name != "remotion" && !strings.HasPrefix(name, "@remotion/") {
				continue
			}
			if version != want {
				t.Errorf("%s.%s is %q, want %q — every Remotion package must share one exact version",
					block, name, version, want)
			}
		}
	}
}

func templateSource(t *testing.T, kind Kind) string {
	t.Helper()
	data, err := kit.FS.ReadFile(kind.TemplatePath)
	if err != nil {
		t.Fatalf("kind %q: %v", kind.Name, err)
	}
	return string(data)
}
