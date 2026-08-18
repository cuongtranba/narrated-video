package pkgscripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTarget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
		want   string
		wantOK bool
	}{
		{
			name:   "the shape nv init writes",
			script: "remotion render Explainer out/explainer.mp4",
			want:   "Explainer",
			wantOK: true,
		},
		{
			name:   "still takes the same positional",
			script: "remotion still Explainer out/explainer.png",
			want:   "Explainer",
			wantOK: true,
		},
		{
			name:   "a runner in front of remotion",
			script: "bunx remotion render Explainer-vi out/explainer-vi.mp4",
			want:   "Explainer-vi",
			wantOK: true,
		},
		{
			name:   "npx spelling",
			script: "npx remotion render Demo out/demo.mp4",
			want:   "Demo",
			wantOK: true,
		},
		{
			// Refusing here is what keeps `nv sync` from rewriting a flag into
			// a composition id. An unrecognized script is left alone.
			name:   "a flag sits where the id would be",
			script: "remotion render --concurrency 4 Explainer out.mp4",
			wantOK: false,
		},
		{
			name:   "a subcommand that takes no composition",
			script: "remotion studio",
			wantOK: false,
		},
		{
			name:   "not remotion at all",
			script: "tsc --noEmit",
			wantOK: false,
		},
		{
			name:   "nothing follows the subcommand",
			script: "remotion render",
			wantOK: false,
		},
		{
			name:   "empty",
			script: "",
			wantOK: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Target(tc.script)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("id = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRetarget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script string
		id     string
		want   string
	}{
		{
			name:   "rewrites the composition and nothing else",
			script: "remotion render Explainer out/explainer.mp4",
			id:     "CronExplainer",
			want:   "remotion render CronExplainer out/explainer.mp4",
		},
		{
			// The output path is the author's decision; only the id is derived
			// from the config, so only the id may be rewritten.
			name:   "leaves flags and a custom output path in place",
			script: "bunx remotion render Explainer out/final-cut.mp4 --log verbose",
			id:     "Demo",
			want:   "bunx remotion render Demo out/final-cut.mp4 --log verbose",
		},
		{
			name:   "preserves the spacing it found",
			script: "remotion  render   Explainer   out/x.mp4",
			id:     "Demo",
			want:   "remotion  render   Demo   out/x.mp4",
		},
		{
			name:   "an unrecognized script is returned untouched",
			script: "remotion render --concurrency 4 Explainer out.mp4",
			id:     "Demo",
			want:   "remotion render --concurrency 4 Explainer out.mp4",
		},
		{
			name:   "already correct",
			script: "remotion still Demo out/demo.png",
			id:     "Demo",
			want:   "remotion still Demo out/demo.png",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Retarget(tc.script, tc.id); got != tc.want {
				t.Fatalf("Retarget()\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

const kitPackageJSON = `{
  "name": "narrated-video-project",
  "private": true,
  "scripts": {
    "studio": "remotion studio",
    "render": "remotion render Explainer out/explainer.mp4",
    "still": "remotion still Explainer out/explainer.png",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "remotion": "4.0.512"
  }
}
`

func writePackage(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoad_ReadsTheManagedScripts(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}
	for _, name := range Managed {
		id, ok := f.Target(name)
		if !ok || id != "Explainer" {
			t.Fatalf("Target(%q) = %q, %v", name, id, ok)
		}
	}
}

func TestLoad_DisownsWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"not json", "not json at all"},
		{"no scripts object", `{"name":"x"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Load(writePackage(t, tc.body))
			if f != nil || err != nil {
				t.Fatalf("Load() = %v, %v; want nil, nil", f, err)
			}
		})
	}

	f, err := Load(t.TempDir())
	if f != nil || err != nil {
		t.Fatalf("Load() with no package.json = %v, %v; want nil, nil", f, err)
	}
}

func TestRetargeted_RewritesOnlyTheCompositionIds(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil {
		t.Fatal(err)
	}

	out, changed := f.Retargeted("CronExplainer")
	if !changed {
		t.Fatal("Retargeted() reported no change")
	}
	body := string(out)

	for _, want := range []string{
		`"render": "remotion render CronExplainer out/explainer.mp4"`,
		`"still": "remotion still CronExplainer out/explainer.png"`,
		`"studio": "remotion studio"`,
		`"typecheck": "tsc --noEmit"`,
		`"remotion": "4.0.512"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "render Explainer") || strings.Contains(body, "still Explainer") {
		t.Fatalf("an old composition id survived:\n%s", body)
	}
}

func TestRetargeted_IsSilentWhenAlreadyCorrect(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil {
		t.Fatal(err)
	}
	if out, changed := f.Retargeted("Explainer"); changed {
		t.Fatalf("Retargeted() rewrote a correct file:\n%s", out)
	}
}

// A "render" key outside the scripts object must stay out of reach — a
// dependency of that name is the realistic case.
func TestRetargeted_LeavesIdenticalKeysElsewhereAlone(t *testing.T) {
	body := `{
  "scripts": {
    "render": "remotion render Explainer out/explainer.mp4"
  },
  "dependencies": {
    "render": "^1.2.3"
  }
}
`
	f, err := Load(writePackage(t, body))
	if err != nil {
		t.Fatal(err)
	}
	out, changed := f.Retargeted("Demo")
	if !changed {
		t.Fatal("Retargeted() reported no change")
	}
	if !strings.Contains(string(out), `"render": "^1.2.3"`) {
		t.Fatalf("the dependency was rewritten:\n%s", out)
	}
}

func TestScripts_ReturnsAllScripts(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil || f == nil {
		t.Fatalf("Load() = %v, %v", f, err)
	}
	scripts := f.Scripts()
	for _, name := range []string{"studio", "render", "still", "typecheck"} {
		if _, ok := scripts[name]; !ok {
			t.Errorf("Scripts() missing %q", name)
		}
	}
}

func TestWithGLFlag_AddsFlag(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil {
		t.Fatal(err)
	}
	out, changed := f.WithGLFlag()
	if !changed {
		t.Fatal("WithGLFlag() reported no change when flag was absent")
	}
	body := string(out)
	for _, name := range Managed {
		if !strings.Contains(body, `"`+name+`":`) {
			continue
		}
		if !strings.Contains(body, "--gl=angle") {
			t.Errorf("script %q is missing --gl=angle in:\n%s", name, body)
		}
	}
}

func TestWithGLFlag_PreservesExistingFlag(t *testing.T) {
	body := `{
  "private": true,
  "scripts": {
    "render": "remotion render Explainer out/explainer.mp4 --gl=angle",
    "still": "remotion still Explainer out/explainer.png --gl=angle"
  }
}
`
	f, err := Load(writePackage(t, body))
	if err != nil {
		t.Fatal(err)
	}
	out, changed := f.WithGLFlag()
	if changed {
		t.Fatalf("WithGLFlag() rewrote scripts that already had the flag:\n%s", out)
	}
}

func TestWithGLFlag_SurvivesCompositionRename(t *testing.T) {
	body := `{
  "private": true,
  "scripts": {
    "render": "remotion render Explainer out/explainer.mp4 --gl=angle",
    "still": "remotion still Explainer out/explainer.png --gl=angle"
  }
}
`
	f, err := Load(writePackage(t, body))
	if err != nil {
		t.Fatal(err)
	}
	renamed, ok := f.Retargeted("NewExplainer")
	if !ok {
		t.Fatal("Retargeted() reported no change")
	}
	if !strings.Contains(string(renamed), "--gl=angle") {
		t.Fatalf("--gl=angle did not survive composition rename:\n%s", renamed)
	}
	if strings.Contains(string(renamed), "render Explainer") || strings.Contains(string(renamed), "still Explainer") {
		t.Fatalf("old composition id survived rename:\n%s", renamed)
	}
}

func TestWithoutGLFlag_RemovesFlag(t *testing.T) {
	body := `{
  "private": true,
  "scripts": {
    "render": "remotion render Explainer out/explainer.mp4 --gl=angle",
    "still": "remotion still Explainer out/explainer.png --gl=angle"
  }
}
`
	f, err := Load(writePackage(t, body))
	if err != nil {
		t.Fatal(err)
	}
	out, changed := f.WithoutGLFlag()
	if !changed {
		t.Fatal("WithoutGLFlag() reported no change when flag was present")
	}
	if strings.Contains(string(out), "--gl=angle") {
		t.Fatalf("--gl=angle survived WithoutGLFlag():\n%s", out)
	}
}

func TestWithoutGLFlag_SilentWhenAbsent(t *testing.T) {
	f, err := Load(writePackage(t, kitPackageJSON))
	if err != nil {
		t.Fatal(err)
	}
	out, changed := f.WithoutGLFlag()
	if changed {
		t.Fatalf("WithoutGLFlag() rewrote scripts that had no flag:\n%s", out)
	}
}
