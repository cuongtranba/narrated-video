package scaffold

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/checks"
	"github.com/cuongtranba/narrated-video/internal/gen"
	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/scenekind"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
	"github.com/cuongtranba/narrated-video/kit"
)

func scaffoldInto(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Project(dir, io.Discard); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	return dir
}

func failingChecks(t *testing.T, root string) []string {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var ids []string
	for _, r := range checks.Run(checks.LoadKit(p, map[string]string{})) {
		if !r.OK {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func syncGenerated(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := gen.All(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(f.Path, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The first minute with the tool, as a test. A fresh project must be honest
// about exactly one thing — that it has no audio yet — and wrong about nothing
// else. Anything more failing here is a template that ships broken.
func TestProject_FreshScaffoldFailsOnlyOnMissingAudio(t *testing.T) {
	root := scaffoldInto(t)

	if got := failingChecks(t, root); !slices.Equal(got, []string{"CHK-05", "CHK-06"}) {
		t.Errorf("a fresh scaffold fails %v, want exactly [CHK-05 CHK-06]", got)
	}
}

// The committed generated files in the template must be what the tool would
// produce. If they are not, the first `nv sync` rewrites files the user never
// touched, and the diff teaches them the generated output is untrustworthy.
func TestProject_TemplateGeneratedFilesAreAlreadyInSync(t *testing.T) {
	root := scaffoldInto(t)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := gen.Diff(p, os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) > 0 {
		t.Errorf("the template ships generated files that differ from a fresh derivation: %v", stale)
	}
}

// The whole point of the silence provider: a project reaches a clean gate with
// no API key, no network, and no account anywhere.
func TestProject_ReachesACleanGateWithoutAnAPIKey(t *testing.T) {
	root := scaffoldInto(t)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := voiceover.Run(context.Background(), p, voiceover.Options{}, io.Discard); err != nil {
		t.Fatalf("voiceover: %v", err)
	}
	syncGenerated(t, root)

	if got := failingChecks(t, root); len(got) > 0 {
		t.Errorf("checks still failing after synthesis: %v", got)
	}
}

func TestProject_RefusesToOverwriteAnExistingProject(t *testing.T) {
	root := scaffoldInto(t)

	if err := Project(root, io.Discard); err == nil {
		t.Error("scaffolding over an existing project was allowed")
	}
}

func TestProject_WritesDotfilesUnderTheirRealNames(t *testing.T) {
	root := scaffoldInto(t)

	for _, name := range []string{".gitignore", ".env.example", "video.schema.json", "video.config.yaml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s missing from a scaffolded project", name)
		}
	}
	// Embedding a dotfile under its real name is not possible, so it travels
	// renamed; shipping the placeholder as well would be confusing.
	if _, err := os.Stat(filepath.Join(root, "gitignore")); err == nil {
		t.Error("the pre-rename placeholder was written too")
	}
}

// Writing the module without registering it leaves a project that compiles and
// renders nothing — the failure is silent, so both halves happen together.
func TestAddScene_WritesTheModuleAndRegistersIt(t *testing.T) {
	root := scaffoldInto(t)

	if err := AddScene(root, "Marathon", "", io.Discard); err != nil {
		t.Fatalf("add scene: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "src", "scenes", "Marathon.tsx")); err != nil {
		t.Errorf("scene module not written: %v", err)
	}

	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("config no longer loads after adding a scene: %v", err)
	}
	if !slices.Contains(p.Config.SceneIDs(), "Marathon") {
		t.Errorf("scene ids = %v, want Marathon among them", p.Config.SceneIDs())
	}
}

// The config is edited as text so the comments explaining why a scene holds
// longer than the rest survive. A YAML round-trip would reflow the document and
// drop exactly the lines worth keeping.
func TestAddScene_PreservesCommentsAndLayout(t *testing.T) {
	root := scaffoldInto(t)
	before := read(t, filepath.Join(root, "video.config.yaml"))

	if err := AddScene(root, "Marathon", "", io.Discard); err != nil {
		t.Fatal(err)
	}
	after := read(t, filepath.Join(root, "video.config.yaml"))

	for _, line := range strings.Split(before, "\n") {
		if strings.Contains(line, "#") && !strings.Contains(after, line) {
			t.Errorf("comment lost: %q", strings.TrimSpace(line))
		}
	}
	if !strings.Contains(after, "\n  - Marathon") {
		t.Errorf("scene not appended to the list:\n%s", after)
	}
}

func TestAddScene_RefusesBadNamesAndDuplicates(t *testing.T) {
	root := scaffoldInto(t)

	for _, id := range []string{"marathon", "1Scene", "Bad-Name", ""} {
		if err := AddScene(root, id, "", io.Discard); err == nil {
			t.Errorf("accepted %q as a scene id", id)
		}
	}

	if err := AddScene(root, "Marathon", "", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := AddScene(root, "Marathon", "", io.Discard); err == nil {
		t.Error("adding the same scene twice was allowed")
	}
}

// A kind is a promise: the module comes from that kind's template, the config
// gains the scene, and the project's package.json lists whatever the kind
// cannot render without (pre-installed in the kit's own package.json for kinds
// whose components ship in kit/src).
func TestAddScene_ScaffoldsTheKindItWasAskedFor(t *testing.T) {
	for _, kind := range scenekind.All() {
		t.Run(kind.Name, func(t *testing.T) {
			root := scaffoldInto(t)

			if err := AddScene(root, "Marathon", kind.Name, io.Discard); err != nil {
				t.Fatalf("add scene: %v", err)
			}

			template, err := kit.FS.ReadFile(kind.TemplatePath)
			if err != nil {
				t.Fatal(err)
			}
			if got := read(t, filepath.Join(root, "src", "scenes", "Marathon.tsx")); got != string(template) {
				t.Errorf("the module was not copied from %s:\n%s", kind.TemplatePath, got)
			}

			p, err := project.Load(root)
			if err != nil {
				t.Fatalf("config no longer loads: %v", err)
			}
			if !slices.Contains(p.Config.SceneIDs(), "Marathon") {
				t.Errorf("scene ids = %v, want Marathon among them", p.Config.SceneIDs())
			}

			installed := dependencies(t, root)
			for _, dep := range kind.Dependencies {
				if installed[dep.Name] != dep.Version {
					t.Errorf("package.json has %s = %q, want %q", dep.Name, installed[dep.Name], dep.Version)
				}
			}
		})
	}
}

// The default is the kind every existing project was written in, so an author
// who has never heard of `--kind` gets exactly what they got before.
func TestAddScene_DefaultsToText(t *testing.T) {
	root := scaffoldInto(t)

	if err := AddScene(root, "Marathon", "", io.Discard); err != nil {
		t.Fatal(err)
	}

	template, err := kit.FS.ReadFile("src/scenes/_template.tsx")
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, "src", "scenes", "Marathon.tsx")); got != string(template) {
		t.Error("the default kind did not copy _template.tsx")
	}
	if before, after := read(t, filepath.Join(root, "package.json")), string(kitPackageJSON(t)); before != after {
		t.Error("a text scene changed package.json")
	}
}

// The kind decides the scene module. Everything else about the project is the
// same decision it was before — a kind that quietly rewrote a config or a
// component would make `--kind` a fork rather than a template.
//
// Dependencies ship in the kit's own package.json, so AddScene for a flow kind
// need not change package.json: @xyflow/react is already declared there.
func TestAddScene_ChangesNothingBeyondItsModule(t *testing.T) {
	text, flow := scaffoldInto(t), scaffoldInto(t)

	if err := AddScene(text, "Marathon", "text", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := AddScene(flow, "Marathon", "flow", io.Discard); err != nil {
		t.Fatal(err)
	}

	expected := map[string]bool{
		filepath.Join("src", "scenes", "Marathon.tsx"): true,
	}
	for name, body := range tree(t, text) {
		other, present := tree(t, flow)[name]
		switch {
		case !present:
			t.Errorf("%s exists only in the text project", name)
		case body != other && !expected[name]:
			t.Errorf("%s differs between a text and a flow project", name)
		case body == other && expected[name]:
			t.Errorf("%s is identical, but a flow scene should have changed it", name)
		}
	}
}

func TestAddScene_RefusesAnUnknownKindBeforeWritingAnything(t *testing.T) {
	root := scaffoldInto(t)
	before := tree(t, root)

	if err := AddScene(root, "Marathon", "nope", io.Discard); err == nil {
		t.Fatal("accepted an unknown kind")
	}
	if got := len(tree(t, root)); got != len(before) {
		t.Errorf("the project holds %d files after a refused kind, want %d", got, len(before))
	}
	if _, err := os.Stat(filepath.Join(root, "src", "scenes", "Marathon.tsx")); err == nil {
		t.Error("a module was written for a kind that does not exist")
	}
}

func dependencies(t *testing.T, root string) map[string]string {
	t.Helper()
	file, err := pkgscripts.Load(root)
	if err != nil || file == nil {
		t.Fatalf("Load(package.json) = %v, %v", file, err)
	}
	deps, ok := file.Deps()
	if !ok {
		t.Fatal("the scaffolded package.json has no readable dependencies")
	}
	return deps
}

func kitPackageJSON(t *testing.T) []byte {
	t.Helper()
	data, err := kit.FS.ReadFile("package.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func tree(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[rel] = read(t, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
