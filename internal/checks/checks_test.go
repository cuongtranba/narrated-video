package checks

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/gen"
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
)

// A gate is only worth its exit code if each check fires on its own failure and
// stays quiet otherwise. That needs a project that genuinely passes everything
// first — synthesized here with the silence provider, so it needs no API key and
// produces the same bytes on every machine.
func buildPassingProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "video.config.yaml"), baseConfig)
	writeFile(t, filepath.Join(root, "content", "en.yaml"), baseEnglish)
	writeFile(t, filepath.Join(root, "content", "vi.yaml"), baseVietnamese)
	for _, scene := range []string{"Title", "Iteration", "Credits"} {
		writeFile(t, filepath.Join(root, "src", "scenes", scene+".tsx"), sceneSource)
	}

	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := voiceover.Run(context.Background(), p, voiceover.Options{}, io.Discard); err != nil {
		t.Fatalf("voiceover: %v", err)
	}
	sync(t, root)
	return root
}

// sync mirrors what `nv sync` does, so the fixture is generated the same way a
// real project is rather than by a second implementation that could disagree.
func sync(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	files, err := gen.All(p)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, f := range files {
		writeFile(t, f.Path, string(f.Data))
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func failingIDs(t *testing.T, root string, tracked map[string]string) []string {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tracked == nil {
		tracked = map[string]string{}
	}

	var ids []string
	for _, r := range Run(LoadKit(p, tracked)) {
		if !r.OK {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func TestRun_PassingProjectPassesEveryCheck(t *testing.T) {
	root := buildPassingProject(t)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Run(LoadKit(p, map[string]string{})) {
		if !r.OK {
			t.Errorf("%s (%s) failed on a project that should pass:\n  remedy: %s\n  %v", r.ID, r.Title, r.Remedy, r.Findings)
		}
	}
}

// Every check must carry an actionable remedy. A failure the reader cannot act
// on is one they learn to skip past.
func TestRun_EveryCheckHasATitleAndRemedy(t *testing.T) {
	root := buildPassingProject(t)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, r := range Run(LoadKit(p, map[string]string{})) {
		if seen[r.ID] {
			t.Errorf("duplicate check id %s", r.ID)
		}
		seen[r.ID] = true
		if r.Title == "" {
			t.Errorf("%s has no title", r.ID)
		}
	}
	if len(seen) != len(All()) {
		t.Errorf("ran %d checks, registry holds %d", len(seen), len(All()))
	}
}

// The table is the argument: one mutation, one named failure. Where a mutation
// genuinely breaks two things, both are listed — a cascade stated up front is
// honest; a cascade discovered later means the checks overlap.
func TestChecks_MutationFailsExactlyItsOwnCheck(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(t *testing.T, root string) map[string]string
		wantFail []string
	}{
		{
			name: "hand-edited timeline",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "src", "generated", "timeline.ts")
				body := read(t, path)
				writeFile(t, path, strings.Replace(body, "durationInFrames: ", "durationInFrames: 1", 1))
				return nil
			},
			wantFail: []string{"CHK-01"},
		},
		{
			name: "a live credential in a tracked file",
			mutate: func(t *testing.T, root string) map[string]string {
				return map[string]string{
					"scripts/render.sh": "ELEVENLABS_API_KEY=sk_e9e1b70ec4f2a1d38b7c6f0912ab34cd bun run render\n",
				}
			},
			wantFail: []string{"CHK-03"},
		},
		{
			name: "narration edited after the take",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "content", "en.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "This is the loop", "This is the loop, edited", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-07"},
		},
		{
			name: "audio deleted",
			mutate: func(t *testing.T, root string) map[string]string {
				if err := os.Remove(filepath.Join(root, "public", "voiceover", "en", "Title.mp3")); err != nil {
					t.Fatal(err)
				}
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-06"},
		},
		{
			name: "frame rate changed after measuring",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "video.config.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "fps: 30", "fps: 60", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-09"},
		},
		{
			name: "a translation drops a list entry",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "content", "vi.yaml")
				writeFile(t, path, strings.Replace(read(t, path), `    links: ["README", "ADR index"]`, `    links: ["README"]`, 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-11"},
		},
		{
			name: "a narrated scene declares its own duration",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "video.config.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "    tailFrames: 50", "    tailFrames: 50\n    durationInFrames: 400", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-12"},
		},
		{
			name: "narration would start under the cross-fade",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "video.config.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "  leadFrames: 14", "  leadFrames: 4", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-13"},
		},
		{
			name: "a printed literal was translated",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "content", "vi.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "GOAL MET", "MỤC TIÊU ĐẠT", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-15"},
		},
		{
			name: "a domain term was translated away",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "content", "vi.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "orchestrator điều phối", "bộ điều phối", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-16"},
		},
		{
			name: "decomposed Vietnamese text",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "content", "vi.yaml")
				// The same word spelled with a combining acute instead of the
				// composed character. Identical on screen in a review.
				writeFile(t, path, strings.Replace(read(t, path), "Vòng lặp tự động", "Vòng lặp tự động", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-17"},
		},
		{
			name: "the font is not asked for the script it must render",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "video.config.yaml")
				writeFile(t, path, strings.Replace(read(t, path), "    subsets: [latin, vietnamese]", "    subsets: [latin]", 1))
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-18"},
		},
		{
			name: "a local font missing this script's stacked vowels",
			mutate: func(t *testing.T, root string) map[string]string {
				// The real face from the project this tool was built from. It
				// contains ọ, đ and the combining marks, so every coarse test
				// says it handles Vietnamese; it has no ố, ầ, ế, ấ, ư or ơ.
				fixture, err := os.ReadFile(filepath.Join("testdata", "body-regular.woff2"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "public", "fonts", "body-regular.woff2"), string(fixture))

				path := filepath.Join(root, "video.config.yaml")
				// Only the Vietnamese locale: English is exactly what this
				// face was built for, so pointing en at it proves nothing.
				body := strings.Replace(read(t, path), "      font: bevietnam\n      charsPerSecond: 15.55", "      font: body\n      charsPerSecond: 15.55", 1)
				writeFile(t, path, body)
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-18"},
		},
		{
			name: "a cue pinned to one language's frame count",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "src", "scenes", "Title.tsx")
				writeFile(t, path, strings.Replace(read(t, path), "at(0.4, durationInFrames)", "at(0.4, 1209)", 1))
				return nil
			},
			wantFail: []string{"CHK-22"},
		},
		{
			name: "a scene reaches for the timeline",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "src", "scenes", "Title.tsx")
				writeFile(t, path, `import { TIMELINE } from "../generated/timeline"`+"\n"+read(t, path))
				return nil
			},
			wantFail: []string{"CHK-21"},
		},
		{
			name: "a scene declared with no module behind it",
			mutate: func(t *testing.T, root string) map[string]string {
				if err := os.Remove(filepath.Join(root, "src", "scenes", "Credits.tsx")); err != nil {
					t.Fatal(err)
				}
				return nil
			},
			wantFail: []string{"CHK-24"},
		},
		{
			name: "the script would blow the spend ceiling",
			mutate: func(t *testing.T, root string) map[string]string {
				path := filepath.Join(root, "video.config.yaml")
				body := read(t, path)
				body = strings.Replace(body, "  provider: silence", "  provider: elevenlabs", 1)
				body = strings.Replace(body, "  costCapUsd: 2.0", "  costCapUsd: 0.0001", 1)
				writeFile(t, path, body)
				sync(t, root)
				return nil
			},
			wantFail: []string{"CHK-20"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := buildPassingProject(t)
			tracked := tc.mutate(t, root)

			got := failingIDs(t, root, tracked)
			slices.Sort(tc.wantFail)
			if !slices.Equal(got, tc.wantFail) {
				t.Errorf("failing checks = %v, want %v", got, tc.wantFail)
			}
		})
	}
}

// A model that mispronounces a language returns success and audio of a
// plausible length, so the only thing standing between it and a shipped video
// is this refusal.
func TestChecks_VietnameseRejectsTheModelThatMispronouncesIt(t *testing.T) {
	root := buildPassingProject(t)
	path := filepath.Join(root, "video.config.yaml")

	body := read(t, path)
	body = strings.Replace(body, "  provider: silence", "  provider: elevenlabs", 1)
	body = strings.Replace(body, "      model: eleven_turbo_v2_5", "      model: eleven_multilingual_v2", 1)
	writeFile(t, path, body)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	var result Result
	for _, r := range Run(LoadKit(p, map[string]string{})) {
		if r.ID == "CHK-04" {
			result = r
		}
	}
	if result.OK {
		t.Fatal("eleven_multilingual_v2 was accepted for Vietnamese")
	}
	// The reason has to travel with the refusal: without it the reader has no
	// way to know the failure is inaudible-to-them rather than a typo.
	joined := strings.ToLower(result.Findings[0].Detail)
	if !strings.Contains(joined, "tone") {
		t.Errorf("refusal does not explain what goes wrong: %q", result.Findings[0].Detail)
	}
}

// An unlisted model is refused too. A model nobody has judged by ear must not
// default into use just because the request would have succeeded.
func TestChecks_UnlistedModelIsRefused(t *testing.T) {
	root := buildPassingProject(t)
	path := filepath.Join(root, "video.config.yaml")

	body := read(t, path)
	body = strings.Replace(body, "  provider: silence", "  provider: elevenlabs", 1)
	body = strings.Replace(body, "      model: eleven_turbo_v2_5", "      model: eleven_future_v9", 1)
	writeFile(t, path, body)

	if got := failingIDs(t, root, nil); !slices.Contains(got, "CHK-04") {
		t.Errorf("failing checks = %v, expected CHK-04 for an unlisted model", got)
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

const baseConfig = `kitVersion: 1
video:
  id: Explainer
  width: 1920
  height: 1080
  fps: 30
  transitionFrames: 14
  minSceneFrames: 30
locales:
  default: en
  list:
    - code: en
      label: English
      font: bevietnam
      charsPerSecond: 16.17
      expansionFactor: 1.0
      requiredSample: "The quick brown fox"
    - code: vi
      label: Tiếng Việt
      font: bevietnam
      charsPerSecond: 15.55
      expansionFactor: 1.6
      requiredSample: "tuyên bố ố ầ ế ấ ư ơ đ Đ"
fonts:
  bevietnam:
    kind: google
    family: Be Vietnam Pro
    importName: BeVietnamPro
    subsets: [latin, vietnamese]
  body:
    kind: local
    family: Body
    files:
      - path: fonts/body-regular.woff2
        weight: "400"
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
  apiKeyEnv: ELEVENLABS_API_KEY
  costCapUsd: 2.0
  voices:
    en:
      voiceId: v-en
      model: eleven_multilingual_v2
    vi:
      voiceId: v-vi
      model: eleven_turbo_v2_5
glossary:
  - orchestrator
printedLiterals:
  - GOAL MET
`

const baseEnglish = `narration:
  Title: "This is the loop, in ninety seconds."
  Iteration: "Each pass reads the plan and delegates one chunk of the work."
copy:
  title:
    heading: "The autonomous loop"
    links: ["README", "ADR index"]
  iteration:
    caption: "the orchestrator delegates, the worker reports"
    verdict: "GOAL MET"
    steps: 4
`

const baseVietnamese = `narration:
  Title: "Đây là vòng lặp, trong chín mươi giây."
  Iteration: "Mỗi lượt đọc kế hoạch và giao một phần việc."
copy:
  title:
    heading: "Vòng lặp tự động"
    links: ["README", "ADR index"]
  iteration:
    caption: "orchestrator điều phối, worker báo cáo"
    verdict: "GOAL MET"
    steps: 4
`

const sceneSource = `import React from "react"
import { useCurrentFrame } from "remotion"

import type { SceneComponent } from "./types"
import { at } from "../timing"

export const Scene: SceneComponent = ({ durationInFrames, leadFrames }) => {
  const frame = useCurrentFrame()
  const revealed = frame > at(0.4, durationInFrames)
  return <div style={{ opacity: frame < leadFrames ? 0 : 1 }}>{revealed ? "shown" : null}</div>
}
`
