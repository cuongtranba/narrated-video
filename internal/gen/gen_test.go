package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/project"
)

// newProject writes a small but complete project on disk. Generation reads the
// filesystem, so a fixture that only half-exists would test the wrong thing.
func newProject(t *testing.T) *project.Project {
	t.Helper()
	root := t.TempDir()

	write(t, filepath.Join(root, "video.config.yaml"), projectConfig)
	write(t, filepath.Join(root, "content", "en.yaml"), englishContent)
	write(t, filepath.Join(root, "content", "vi.yaml"), vietnameseContent)

	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return p
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func generated(t *testing.T, p *project.Project, name string) string {
	t.Helper()
	files, err := All(p)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, f := range files {
		if filepath.Base(f.Path) == name {
			return string(f.Data)
		}
	}
	t.Fatalf("%s was not generated", name)
	return ""
}

// Byte stability is the property the staleness check rests on. Map iteration
// order in Go is randomised per run, so this would fail immediately if any
// emitter walked a map without sorting.
func TestAll_IsByteStableAcrossRuns(t *testing.T) {
	p := newProject(t)

	first, err := All(p)
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		next, err := All(p)
		if err != nil {
			t.Fatal(err)
		}
		for i := range first {
			if string(first[i].Data) != string(next[i].Data) {
				t.Fatalf("%s differs between two generations of identical input", first[i].Path)
			}
		}
	}
}

// The timeline is loaded by tests with no bundler, so it must stay free of
// imports entirely — the moment it pulls in remotion it can only be read
// through a build.
func TestTimelineTS_HasNoImports(t *testing.T) {
	out := generated(t, newProject(t), "timeline.ts")
	if strings.Contains(out, "import ") {
		t.Errorf("timeline.ts must not import anything:\n%s", out)
	}
	for _, want := range []string{"export const TIMELINE", "export const LOCALES", `"en"`, `"vi"`} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline.ts missing %q", want)
		}
	}
}

// Without a manifest every narrated scene is an estimate, and the timeline has
// to say so — the render draws its draft badge from this flag.
func TestTimelineTS_ReportsIncompleteWithoutAudio(t *testing.T) {
	out := generated(t, newProject(t), "timeline.ts")
	if !strings.Contains(out, "complete: false") {
		t.Error("a project with no audio must generate complete: false")
	}
	if !strings.Contains(out, `source: "estimated"`) {
		t.Error("a narrated scene with no measurement must be marked estimated")
	}
	if !strings.Contains(out, `source: "literal"`) {
		t.Error("an unnarrated scene must be marked literal")
	}
}

func TestRegistryTS_BindsComponentsAtModuleScope(t *testing.T) {
	out := generated(t, newProject(t), "registry.ts")
	for _, want := range []string{
		`import { Scene as Title } from "../scenes/Title"`,
		`import { Scene as Credits } from "../scenes/Credits"`,
		`export const SCENE_ORDER = ["Title", "Iteration", "Credits"] as const`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("registry.ts missing:\n  %s\ngot:\n%s", want, out)
		}
	}
}

// Scene order is the author's, not the alphabet's — it is the order of the
// video. Sorting here would silently re-cut it.
func TestRegistryTS_PreservesConfigOrder(t *testing.T) {
	out := generated(t, newProject(t), "registry.ts")
	title, iteration := strings.Index(out, `"Title"`), strings.Index(out, `"Iteration"`)
	if title < 0 || iteration < 0 || title > iteration {
		t.Errorf("scene order not preserved:\n%s", out)
	}
}

// The Copy type is derived from the default locale, so a field added in YAML
// arrives in TypeScript typed, and a translation that drops it fails to build.
func TestContentTS_DerivesCopyTypeFromDefaultLocale(t *testing.T) {
	out := generated(t, newProject(t), "content.ts")
	for _, want := range []string{
		"export interface Copy {",
		"readonly heading: string",
		"readonly links: readonly string[]",
		"readonly steps: number",
		"readonly forceable: boolean",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("content.ts missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `import { `) {
		t.Error("content.ts must use type-only imports")
	}
}

func TestContentTS_EmitsEveryLocale(t *testing.T) {
	out := generated(t, newProject(t), "content.ts")
	for _, want := range []string{`"The autonomous loop"`, `"Vòng lặp tự động"`, "export const NARRATION", "export const COPY"} {
		if !strings.Contains(out, want) {
			t.Errorf("content.ts missing %q", want)
		}
	}
}

// An empty list gives nothing to infer from. Guessing string[] would be wrong
// as often as right, and wrong here means a build that compiles and a frame
// that renders nothing.
func TestContentTS_EmptyListIsUnknownRatherThanGuessed(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "video.config.yaml"), projectConfig)
	write(t, filepath.Join(root, "content", "en.yaml"), "narration:\n  Title: hi\ncopy:\n  outro:\n    links: []\n")
	write(t, filepath.Join(root, "content", "vi.yaml"), "narration:\n  Title: chào\ncopy:\n  outro:\n    links: []\n")

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if out := generated(t, p, "content.ts"); !strings.Contains(out, "readonly unknown[]") {
		t.Errorf("expected an empty list to derive unknown[]:\n%s", out)
	}
}

func TestDiff_DetectsStaleAndMissingFiles(t *testing.T) {
	p := newProject(t)
	files, err := All(p)
	if err != nil {
		t.Fatal(err)
	}

	fresh := map[string][]byte{}
	for _, f := range files {
		fresh[f.Path] = f.Data
	}
	read := func(path string) ([]byte, error) { return fresh[path], nil }

	stale, err := Diff(p, read)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Errorf("freshly generated files reported stale: %v", stale)
	}

	// One frame is all it takes: the drift that motivated this whole design was
	// invisible in every rendered frame.
	target := files[0].Path
	fresh[target] = []byte(strings.Replace(string(files[0].Data), "durationInFrames: ", "durationInFrames: 1", 1))

	stale, err = Diff(p, read)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || stale[0] != target {
		t.Errorf("stale = %v, want exactly [%s]", stale, target)
	}
}

func TestDiagramsTS_EmitsNodesWithLabelsAndSize(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "video.config.yaml"), projectConfigWithDiagram)
	write(t, filepath.Join(root, "content", "en.yaml"), englishContentWithDiagram)
	write(t, filepath.Join(root, "content", "vi.yaml"), vietnameseContentWithDiagram)

	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	out := generated(t, p, "diagrams.ts")

	for _, want := range []string{
		"export const DIAGRAMS",
		`en:`,
		`vi:`,
		`pipeline:`,
		`id: "config"`,
		`width: 320`,
		`height: 96`,
		`x: 0`,
		`y: 0`,
		`label: "video.config.yaml"`,
		`label: "cấu hình video"`,
		`id: "config-sync"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diagrams.ts missing %q:\n%s", want, out)
		}
	}
}

func TestDiagramsTS_AbsentWhenNoDiagrams(t *testing.T) {
	p := newProject(t)
	files, err := All(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f.Path) == "diagrams.ts" {
			t.Errorf("diagrams.ts should not be generated when no diagrams are declared, got:\n%s", string(f.Data))
		}
	}
}

const projectConfigWithDiagram = `kitVersion: 1
video:
  id: Explainer
  width: 1920
  height: 1080
  fps: 30
locales:
  default: en
  list:
    - code: en
      label: English
      font: body
      charsPerSecond: 16.17
    - code: vi
      label: Tiếng Việt
      font: body
      charsPerSecond: 15.55
fonts:
  body:
    kind: google
    family: Inter
    importName: Inter
    subsets: [latin]
theme:
  background: oklch(0.16 0.012 13)
  foreground: oklch(0.96 0.005 13)
defaults:
  leadFrames: 14
  tailFrames: 24
scenes:
  - Title
tts:
  provider: silence
  costCapUsd: 1.0
  voices:
    en:
      voiceId: v1
      model: eleven_multilingual_v2
    vi:
      voiceId: v2
      model: eleven_turbo_v2_5
diagrams:
  pipeline:
    nodes:
      - id: config
        at: [0, 0]
        size: [320, 96]
      - id: sync
        at: [420, 0]
        size: [320, 96]
    edges:
      - from: config
        to: sync
`

const englishContentWithDiagram = `narration:
  Title: "hello"
copy:
  diagrams:
    pipeline:
      config: "video.config.yaml"
      sync: "nv sync"
`

const vietnameseContentWithDiagram = `narration:
  Title: "xin chào"
copy:
  diagrams:
    pipeline:
      config: "cấu hình video"
      sync: "đồng bộ"
`

const projectConfig = `kitVersion: 1
video:
  id: Explainer
  width: 1920
  height: 1080
  fps: 30
locales:
  default: en
  list:
    - code: en
      label: English
      font: body
      charsPerSecond: 16.17
    - code: vi
      label: Tiếng Việt
      font: body
      charsPerSecond: 15.55
fonts:
  body:
    kind: google
    family: Inter
    importName: Inter
    subsets: [latin]
theme:
  background: oklch(0.16 0.012 13)
  foreground: oklch(0.96 0.005 13)
defaults:
  leadFrames: 14
  tailFrames: 24
scenes:
  - Title
  - id: Iteration
    tailFrames: 50
  - id: Credits
    durationInFrames: 120
    narrated: false
tts:
  provider: silence
  costCapUsd: 1.0
  voices:
    en:
      voiceId: v1
      model: eleven_multilingual_v2
    vi:
      voiceId: v2
      model: eleven_turbo_v2_5
`

const englishContent = `narration:
  Title: "This is the loop, in ninety seconds."
  Iteration: "Each pass reads the plan and delegates one chunk."
copy:
  title:
    heading: "The autonomous loop"
    links: ["README", "ADR index"]
  iteration:
    steps: 4
    gate:
      forceable: true
`

const vietnameseContent = `narration:
  Title: "Đây là vòng lặp, trong chín mươi giây."
  Iteration: "Mỗi lượt đọc kế hoạch và giao một phần việc."
copy:
  title:
    heading: "Vòng lặp tự động"
    links: ["README", "ADR index"]
  iteration:
    steps: 4
    gate:
      forceable: true
`
