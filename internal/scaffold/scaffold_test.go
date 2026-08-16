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
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
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

	if err := AddScene(root, "Marathon", io.Discard); err != nil {
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

	if err := AddScene(root, "Marathon", io.Discard); err != nil {
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
		if err := AddScene(root, id, io.Discard); err == nil {
			t.Errorf("accepted %q as a scene id", id)
		}
	}

	if err := AddScene(root, "Marathon", io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := AddScene(root, "Marathon", io.Discard); err == nil {
		t.Error("adding the same scene twice was allowed")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
