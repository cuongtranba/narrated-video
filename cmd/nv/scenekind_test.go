package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/checks"
	"github.com/cuongtranba/narrated-video/internal/project"
)

// The whole loop for a kind, end to end: scaffold it, break the half that lives
// in package.json, and prove the remedy CHK-34 prints is the one that clears it.
// A check whose remedy does not work is worse than no check.
func TestInit_KindInstallsItsDependencyAndSyncRestoresIt(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if err := runInit([]string{"."}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := runInit([]string{"--scene", "Pipeline", "--kind", "flow"}); err != nil {
		t.Fatalf("init --scene --kind flow: %v", err)
	}

	pkgPath := filepath.Join(root, "package.json")
	if !strings.Contains(readFile(t, pkgPath), `"@xyflow/react": "^12.0.0"`) {
		t.Fatalf("a flow scene did not install its dependency:\n%s", readFile(t, pkgPath))
	}

	// What the author does next: the kind is only a starting point until the
	// module actually reaches for the library, and the module is what CHK-34
	// derives from.
	scenePath := filepath.Join(root, "src", "scenes", "Pipeline.tsx")
	writeString(t, scenePath, `import { ReactFlow } from "@xyflow/react"`+"\n"+readFile(t, scenePath))
	if failing := failingIDs(t, root); contains(failing, "CHK-34") {
		t.Fatalf("CHK-34 fails on a project nv itself scaffolded: %v", failing)
	}

	stripped := strings.Replace(readFile(t, pkgPath), `"@xyflow/react": "^12.0.0",`, "", 1)
	writeString(t, pkgPath, stripped)
	if failing := failingIDs(t, root); !contains(failing, "CHK-34") {
		t.Fatalf("failing checks = %v, want CHK-34 once the dependency was removed", failing)
	}

	if err := runSync(nil); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if failing := failingIDs(t, root); contains(failing, "CHK-34") {
		t.Fatalf("the remedy CHK-34 prints did not clear it: %v", failing)
	}
}

func TestInit_UnknownKindFailsBeforeWritingAnything(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if err := runInit([]string{"."}); err != nil {
		t.Fatalf("init: %v", err)
	}

	err := runInit([]string{"--scene", "Pipeline", "--kind", "nope"})
	if err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	for _, want := range []string{"nope", "text", "flow", "space"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "src", "scenes", "Pipeline.tsx")); err == nil {
		t.Error("a module was written for a kind that does not exist")
	}
}

func failingIDs(t *testing.T, root string) []string {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, r := range checks.Run(checks.LoadKit(p, map[string]string{})) {
		if !r.OK {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func contains(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeString(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
