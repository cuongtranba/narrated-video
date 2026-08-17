package pipeline_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/checks"
	"github.com/cuongtranba/narrated-video/internal/gen"
	"github.com/cuongtranba/narrated-video/internal/pipeline"
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
)

func scaffold(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "video.config.yaml"), fixtureConfig)
	write(t, filepath.Join(root, "content", "en.yaml"), fixtureEnglish)
	for _, scene := range []string{"Title", "Outro"} {
		write(t, filepath.Join(root, "src", "scenes", scene+".tsx"), sceneSource)
	}
	sync(t, root)
	return root
}

func sync(t *testing.T, root string) {
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
		write(t, f.Path, string(f.Data))
	}
}

func speak(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := voiceover.Run(context.Background(), p, voiceover.Options{}, io.Discard); err != nil {
		t.Fatal(err)
	}
	sync(t, root)
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

func derive(t *testing.T, root string) pipeline.Status {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	kit := checks.LoadKit(p, map[string]string{})
	return pipeline.Derive(kit, checks.Run(kit))
}

func stage(t *testing.T, s pipeline.Status, id string) pipeline.Stage {
	t.Helper()
	for _, st := range s.Stages {
		if st.ID == id {
			return st
		}
	}
	t.Fatalf("no stage %q in %+v", id, s.Stages)
	return pipeline.Stage{}
}

// Before any audio exists the project is genuinely mid-pipeline: everything up
// to the voice is done, and nothing past it can be.
func TestDerive_StopsAtTheFirstUnfinishedStage(t *testing.T) {
	status := derive(t, scaffold(t))

	for _, id := range []string{"scaffold", "script", "scenes"} {
		if got := stage(t, status, id).State; got != pipeline.Done {
			t.Errorf("stage %s = %s, want %s", id, got, pipeline.Done)
		}
	}
	if got := stage(t, status, "voiceover").State; got != pipeline.Current {
		t.Errorf("voiceover = %s, want %s", got, pipeline.Current)
	}
	for _, id := range []string{"gate", "render"} {
		if got := stage(t, status, id).State; got != pipeline.Blocked {
			t.Errorf("stage %s = %s, want %s", id, got, pipeline.Blocked)
		}
	}
	if status.Next.Command != "nv voiceover" {
		t.Errorf("next = %q, want %q", status.Next.Command, "nv voiceover")
	}
}

// Exactly one stage is current at a time: a checklist with two "do this next"
// entries is a checklist the reader has to arbitrate.
func TestDerive_NamesOneCurrentStage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T) string
	}{
		{"fresh", scaffold},
		{"voiced", func(t *testing.T) string {
			root := scaffold(t)
			speak(t, root)
			return root
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := derive(t, tc.build(t))
			current := 0
			for _, st := range status.Stages {
				if st.State == pipeline.Current {
					current++
				}
			}
			if current != 1 {
				t.Fatalf("%d current stages, want 1: %+v", current, status.Stages)
			}
		})
	}
}

func TestDerive_RenderIsCurrentOnceTheGateIsGreen(t *testing.T) {
	root := scaffold(t)
	speak(t, root)

	status := derive(t, root)
	if got := stage(t, status, "gate").State; got != pipeline.Done {
		t.Fatalf("gate = %s, want %s", got, pipeline.Done)
	}
	if got := stage(t, status, "render").State; got != pipeline.Current {
		t.Fatalf("render = %s, want %s", got, pipeline.Current)
	}
	if status.Next.Command != "bun run render" {
		t.Errorf("next = %q, want %q", status.Next.Command, "bun run render")
	}
}

func TestDerive_RenderIsDoneWhenTheFileExists(t *testing.T) {
	root := scaffold(t)
	speak(t, root)
	write(t, filepath.Join(root, "out", "explainer.mp4"), "not really an mp4")

	status := derive(t, root)
	if got := stage(t, status, "render").State; got != pipeline.Done {
		t.Fatalf("render = %s, want %s", got, pipeline.Done)
	}
	if status.Next.Command != "" {
		t.Errorf("a finished pipeline still proposes %q", status.Next.Command)
	}
}

// A missing narration line is a script that is not written yet, and it must not
// read as a voiceover problem — that is the wrong file to open.
func TestDerive_AMissingLineIsAScriptStage(t *testing.T) {
	root := scaffold(t)
	write(t, filepath.Join(root, "content", "en.yaml"), "narration:\n  Title: \"Only one line.\"\ncopy:\n  title:\n    heading: \"h\"\n")

	status := derive(t, root)
	script := stage(t, status, "script")
	if script.State != pipeline.Current {
		t.Fatalf("script = %s, want %s", script.State, pipeline.Current)
	}
	if got := stage(t, status, "voiceover").State; got != pipeline.Blocked {
		t.Errorf("voiceover = %s, want %s", got, pipeline.Blocked)
	}
}

func TestDerive_ReportsDurationAndSpend(t *testing.T) {
	root := scaffold(t)
	speak(t, root)

	status := derive(t, root)
	if len(status.Duration) != 1 || status.Duration[0].Locale != "en" {
		t.Fatalf("duration = %+v", status.Duration)
	}
	if status.Duration[0].Seconds <= 0 || !status.Duration[0].Measured {
		t.Fatalf("duration = %+v, want a measured positive length", status.Duration[0])
	}
	if !status.Duration[0].WithinTarget {
		t.Errorf("a 60–600s target rejected %.1fs", status.Duration[0].Seconds)
	}
	if status.Spend.Characters <= 0 {
		t.Fatalf("spend = %+v", status.Spend)
	}
	if status.Spend.CapUSD != 2.0 {
		t.Errorf("cap = %v, want 2", status.Spend.CapUSD)
	}
}

const fixtureConfig = `kitVersion: 1
video:
  id: Explainer
  width: 1920
  height: 1080
  fps: 30
  transitionFrames: 14
  minSceneFrames: 30
  targetDuration:
    minSeconds: 1
    maxSeconds: 600
locales:
  default: en
  list:
    - code: en
      label: English
      font: body
      charsPerSecond: 16.17
      requiredSample: "The quick brown fox"
fonts:
  body:
    kind: google
    family: Be Vietnam Pro
    importName: BeVietnamPro
    subsets: [latin]
theme:
  background: oklch(16% 0.01 13)
defaults:
  leadFrames: 14
  tailFrames: 24
scenes:
  - Title
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
`

const sceneSource = `import type { SceneComponent } from "./types";

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => {
  return null;
};
`
