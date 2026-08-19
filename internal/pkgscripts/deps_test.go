package pkgscripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/scenekind"
)

var (
	flowDep  = scenekind.Dep{Name: "@xyflow/react", Version: "^12.0.0"}
	threeDep = scenekind.Dep{Name: "three", Version: "^0.175.0"}
)

func reconcile(t *testing.T, body string, required ...scenekind.Dep) string {
	t.Helper()
	f, err := Load(writePackage(t, body))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}
	out, changed := f.ReconcileDeps(required)
	if !changed {
		return body
	}
	return string(out)
}

// A kind's dependency lands in alphabetical order, and every other byte of the
// document — the scripts, the formatting, the packages nv knows nothing about —
// is exactly where the author left it.
func TestReconcileDeps_AddsInAlphabeticalOrder(t *testing.T) {
	got := reconcile(t, kitPackageJSON, flowDep, threeDep)

	want := `{
  "name": "narrated-video-project",
  "private": true,
  "scripts": {
    "studio": "remotion studio",
    "render": "remotion render Explainer out/explainer.mp4",
    "still": "remotion still Explainer out/explainer.png",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "@xyflow/react": "^12.0.0",
    "remotion": "4.0.512",
    "three": "^0.175.0"
  }
}
`
	if got != want {
		t.Fatalf("ReconcileDeps()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// `nv sync` runs on every voiceover and every scene edit. A reconciliation that
// rewrote the file each time would put a diff in front of the author that says
// nothing, and they would stop reading the ones that do.
func TestReconcileDeps_IsIdempotent(t *testing.T) {
	once := reconcile(t, kitPackageJSON, flowDep, threeDep)

	f, err := Load(writePackage(t, once))
	if err != nil {
		t.Fatal(err)
	}
	if out, changed := f.ReconcileDeps([]scenekind.Dep{flowDep, threeDep}); changed {
		t.Fatalf("a second reconciliation rewrote the file:\n%s", out)
	}
}

func TestReconcileDeps_CorrectsAVersionThatDriftedFromTheKind(t *testing.T) {
	body := `{
  "scripts": { "render": "remotion render Explainer out/x.mp4" },
  "dependencies": {
    "@xyflow/react": "^11.0.0",
    "remotion": "4.0.512"
  }
}
`
	deps := depsOf(t, reconcile(t, body, flowDep))

	if deps["@xyflow/react"] != flowDep.Version {
		t.Fatalf("@xyflow/react = %q, want %q", deps["@xyflow/react"], flowDep.Version)
	}
	if deps["remotion"] != "4.0.512" {
		t.Fatalf("remotion = %q, want it untouched", deps["remotion"])
	}
}

// The dependencies nv does not own outnumber the ones it does. Reconciliation
// adds and corrects; a package it has never heard of is the author's.
func TestReconcileDeps_LeavesUnmanagedDependenciesAlone(t *testing.T) {
	body := `{
  "scripts": { "render": "remotion render Explainer out/x.mp4" },
  "dependencies": {
    "d3": "^7.9.0",
    "zustand": "^5.0.0"
  }
}
`
	deps := depsOf(t, reconcile(t, body, flowDep))

	for name, want := range map[string]string{
		"d3":            "^7.9.0",
		"zustand":       "^5.0.0",
		"@xyflow/react": flowDep.Version,
	} {
		if deps[name] != want {
			t.Errorf("%s = %q, want %q", name, deps[name], want)
		}
	}
	if len(deps) != 3 {
		t.Errorf("dependencies = %v, want exactly the two originals plus the one added", deps)
	}
}

func TestReconcileDeps_WritesIntoAnEmptyDependenciesObject(t *testing.T) {
	body := `{
  "scripts": { "render": "remotion render Explainer out/x.mp4" },
  "dependencies": {}
}
`
	if deps := depsOf(t, reconcile(t, body, flowDep)); deps["@xyflow/react"] != flowDep.Version {
		t.Fatalf("dependencies = %v, want @xyflow/react added", deps)
	}
}

// A document with no dependencies object is one nv cannot edit without inventing
// a shape for it, so it is disowned rather than guessed at — and CHK-34 stays
// quiet for the same reason.
func TestDeps_DisownsAPackageWithNoDependencies(t *testing.T) {
	f, err := Load(writePackage(t, `{"scripts": {"render": "remotion render Explainer out/x.mp4"}}`))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}
	if deps, ok := f.Deps(); ok {
		t.Fatalf("Deps() = %v, true; want false", deps)
	}
	if out, changed := f.ReconcileDeps([]scenekind.Dep{flowDep}); changed {
		t.Fatalf("ReconcileDeps() rewrote a document it cannot read:\n%s", out)
	}
}

func TestDeps_ReadsWhatThePackageInstalls(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil {
		t.Fatal(err)
	}
	deps, ok := f.Deps()
	if !ok {
		t.Fatal("Deps() disowned the kit's own package.json")
	}
	if deps["remotion"] != "4.0.512" || len(deps) != 1 {
		t.Fatalf("Deps() = %v, want just remotion", deps)
	}
}

// CHK-38 judges the whole Remotion family, and one of them — the eslint config —
// is a devDependency. The two blocks are read the same way; only the key differs.
func TestDevDeps_ReadsTheDevBlockWithoutConfusingItForDependencies(t *testing.T) {
	f, err := Load(writePackage(t, `{
  "scripts": { "render": "remotion render Explainer out/x.mp4" },
  "devDependencies": {
    "@remotion/eslint-config-flat": "4.0.512",
    "typescript": "5.9.3"
  },
  "dependencies": {
    "remotion": "4.0.512"
  }
}
`))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}

	dev, ok := f.DevDeps()
	if !ok {
		t.Fatal("DevDeps() disowned a readable devDependencies object")
	}
	if dev["@remotion/eslint-config-flat"] != "4.0.512" || len(dev) != 2 {
		t.Fatalf("DevDeps() = %v, want the two dev entries", dev)
	}

	deps, ok := f.Deps()
	if !ok || deps["remotion"] != "4.0.512" || len(deps) != 1 {
		t.Fatalf("Deps() = %v, %v; want just remotion — devDependencies must not be mistaken for it", deps, ok)
	}
}

// A package.json with no dev block is ordinary, not broken. CHK-38 has to treat
// it as "nothing to judge" rather than a finding.
func TestDevDeps_DisownsAPackageWithNoDevDependencies(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}
	if dev, ok := f.DevDeps(); ok {
		t.Fatalf("DevDeps() = %v, true; want false", dev)
	}
}

func TestReconcileDepsAt_WritesOnlyWhenSomethingChanged(t *testing.T) {
	root := writePackage(t, kitPackageJSON)
	path := filepath.Join(root, FileName)

	changed, err := ReconcileDepsAt(root, []scenekind.Dep{flowDep})
	if err != nil || !changed {
		t.Fatalf("ReconcileDepsAt() = %v, %v; want true, nil", changed, err)
	}
	after := read(t, path)

	changed, err = ReconcileDepsAt(root, []scenekind.Dep{flowDep})
	if err != nil || changed {
		t.Fatalf("a second run reported %v, %v; want false, nil", changed, err)
	}
	if read(t, path) != after {
		t.Fatal("a run that reported no change still rewrote the file")
	}

	if changed, err := ReconcileDepsAt(t.TempDir(), []scenekind.Dep{flowDep}); changed || err != nil {
		t.Fatalf("with no package.json: %v, %v; want false, nil", changed, err)
	}
}

func depsOf(t *testing.T, body string) map[string]string {
	t.Helper()
	var doc struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("the result is not valid JSON: %v\n%s", err, body)
	}
	return doc.Dependencies
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
