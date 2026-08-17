package script_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/script"
)

func load(t *testing.T) *project.Project {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "video.config.yaml"), fixtureConfig)
	write(t, filepath.Join(root, "content", "en.yaml"), fixtureEnglish)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
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

func TestRender_CarriesTheSpokenLinesVerbatim(t *testing.T) {
	out, err := script.Render(load(t), "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Explainer — English (en)",
		"This is the loop, in ninety seconds.",
		"That is the whole cycle.",
		"The autonomous loop",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

// The doc exists so narration has one home. A renderer that reordered a map
// would make two runs disagree and invite someone to keep the file by hand.
func TestRender_IsByteStable(t *testing.T) {
	p := load(t)
	first, err := script.Render(p, "en")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := script.Render(p, "en")
		if err != nil {
			t.Fatal(err)
		}
		if again != first {
			t.Fatalf("run %d differs from the first", i)
		}
	}
}

// Timecodes have to compose the way the video does: the cross-fade is repaid
// once per boundary, so scene two starts before scene one's frames run out.
func TestRender_TimecodesRepayTheCrossFade(t *testing.T) {
	out, err := script.Render(load(t), "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0:00–") {
		t.Fatalf("the first scene does not start at zero:\n%s", out)
	}

	p := load(t)
	timeline := p.Timelines["en"]
	first := timeline.Scenes[0].DurationInFrames
	second := (first - timeline.TransitionFrames) / timeline.FPS
	if !strings.Contains(out, clock(second)+"–") {
		t.Fatalf("scene two does not start at %s:\n%s", clock(second), out)
	}
}

func clock(seconds int) string {
	return string(rune('0'+seconds/60)) + ":" + pad(seconds%60)
}

func pad(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func TestRender_UnknownLocaleNamesTheDeclaredOnes(t *testing.T) {
	_, err := script.Render(load(t), "fr")
	if err == nil {
		t.Fatal("an undeclared locale rendered")
	}
	if !strings.Contains(err.Error(), "en") {
		t.Fatalf("the error does not name the declared locales: %v", err)
	}
}

func TestRender_EmptyLocaleUsesTheDefault(t *testing.T) {
	out, err := script.Render(load(t), "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "(en)") {
		t.Fatalf("did not fall back to the default locale:\n%s", out)
	}
}

// An unnarrated scene has no line and no audio. Printing an empty quote would
// read as a missing line rather than a deliberate one.
func TestRender_MarksScenesThatAreNotNarrated(t *testing.T) {
	out, err := script.Render(load(t), "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no narration") {
		t.Fatalf("the silent scene is not marked:\n%s", out)
	}
}

const fixtureConfig = `kitVersion: 1
video:
  id: Explainer
  width: 1920
  height: 1080
  fps: 30
  transitionFrames: 14
locales:
  default: en
  list:
    - code: en
      label: English
      font: body
      charsPerSecond: 16.17
fonts:
  body:
    kind: google
    family: Be Vietnam Pro
    importName: BeVietnamPro
    subsets: [latin]
defaults:
  leadFrames: 14
  tailFrames: 24
scenes:
  - Title
  - id: Card
    narrated: false
    durationInFrames: 90
  - Outro
tts:
  provider: silence
  costCapUsd: 2.0
  voices:
    en:
      voiceId: v1
      model: eleven_multilingual_v2
`

const fixtureEnglish = `narration:
  Title: "This is the loop, in ninety seconds."
  Outro: "That is the whole cycle."

copy:
  title:
    heading: "The autonomous loop"
    subheading: "An explainer"
  outro:
    links: ["README", "ADR index"]
`
