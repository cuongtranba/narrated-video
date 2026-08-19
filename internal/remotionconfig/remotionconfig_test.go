package remotionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const base = `import { Config } from "@remotion/cli/config"

Config.setRspack(true)
Config.setVideoImageFormat("jpeg")
Config.setOverwriteOutput(true)
`

func TestHasGLRenderer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   bool
	}{
		{"absent", base, false},
		{"present", base + `Config.setChromiumOpenGlRenderer("angle")` + "\n", true},
		{"single quotes", base + "Config.setChromiumOpenGlRenderer('angle')\n", true},
		{"spaced out", base + `Config.setChromiumOpenGlRenderer ( "angle" )` + "\n", true},
		{
			// A commented example is not a setting. Reading it as one would let
			// CHK-33 pass a project that renders a black rectangle.
			"commented out",
			base + `// Config.setChromiumOpenGlRenderer("angle")` + "\n",
			false,
		},
		{
			// Some other backend is not the one the wrapper needs.
			"a different renderer",
			base + `Config.setChromiumOpenGlRenderer("swangle")` + "\n",
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasGLRenderer([]byte(tc.source)); got != tc.want {
				t.Fatalf("HasGLRenderer() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Sync owns this line the way it owns the --gl=angle script flag: it is derived
// from whether any scene renders 3D, and a hand-kept copy is one that eventually
// disagrees — here as a black rectangle that still exits 0.
func TestReconciled_AddsTheRendererWhenASceneNeedsIt(t *testing.T) {
	out, changed := Reconciled([]byte(base), true)
	if !changed {
		t.Fatal("Reconciled() left a space project without the GL renderer")
	}
	if !HasGLRenderer(out) {
		t.Fatalf("the renderer is still absent:\n%s", out)
	}
	// Every other byte the author wrote stays where it was.
	if got := string(out); got[:len(base)] != base {
		t.Fatalf("the existing config was rewritten:\n%s", got)
	}
}

func TestReconciled_RemovesTheRendererWhenTheLastSpaceSceneGoes(t *testing.T) {
	withGL, _ := Reconciled([]byte(base), true)

	out, changed := Reconciled(withGL, false)
	if !changed {
		t.Fatal("Reconciled() left the GL renderer behind after the last space scene went")
	}
	if HasGLRenderer(out) {
		t.Fatalf("the renderer is still there:\n%s", out)
	}
	if string(out) != base {
		t.Fatalf("removing it did not restore the original:\n%q\nwant\n%q", out, base)
	}
}

// `nv sync` runs on every voiceover and every scene edit. A reconciliation that
// rewrote the file each time would put a diff in front of the author that says
// nothing, and they would stop reading the ones that do.
func TestReconciled_IsIdempotent(t *testing.T) {
	withGL, _ := Reconciled([]byte(base), true)
	if out, changed := Reconciled(withGL, true); changed {
		t.Fatalf("a second reconciliation rewrote the file:\n%s", out)
	}
	if out, changed := Reconciled([]byte(base), false); changed {
		t.Fatalf("a second reconciliation rewrote the file:\n%s", out)
	}
}

// A config that already selects the renderer by hand is already correct, and
// sync must not add a second copy of the line.
func TestReconciled_LeavesAHandWrittenRendererAlone(t *testing.T) {
	hand := base + `Config.setChromiumOpenGlRenderer("angle")` + "\n"
	if out, changed := Reconciled([]byte(hand), true); changed {
		t.Fatalf("Reconciled() rewrote a config that was already right:\n%s", out)
	}
}

// A file with no trailing newline is still a file someone wrote.
func TestReconciled_AddsToAFileWithNoTrailingNewline(t *testing.T) {
	out, changed := Reconciled([]byte("Config.setRspack(true)"), true)
	if !changed || !HasGLRenderer(out) {
		t.Fatalf("Reconciled() = %q, %v", out, changed)
	}
}

func TestReconcileAt_WritesOnlyWhenSomethingChanged(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, FileName)
	if err := os.WriteFile(path, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := ReconcileAt(root, true)
	if err != nil || !changed {
		t.Fatalf("ReconcileAt() = %v, %v; want true, nil", changed, err)
	}
	if !HasGLRenderer(readFile(t, path)) {
		t.Fatal("the GL renderer was not written to disk")
	}

	if changed, err := ReconcileAt(root, true); err != nil || changed {
		t.Fatalf("a second reconciliation rewrote the file: %v, %v", changed, err)
	}
}

// A project with no remotion.config.ts belongs to the JavaScript side, not to
// nv. Sync leaves it alone and CHK-33 reports it with a remedy the author can
// act on — the same stance every other file nv does not own gets.
func TestReconcileAt_LeavesAnAbsentConfigAlone(t *testing.T) {
	root := t.TempDir()

	changed, err := ReconcileAt(root, true)
	if err != nil || changed {
		t.Fatalf("ReconcileAt() = %v, %v; want false, nil", changed, err)
	}
	if _, err := os.Stat(filepath.Join(root, FileName)); !os.IsNotExist(err) {
		t.Fatal("ReconcileAt() created a config the project does not have")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
