// Package pipeline answers "where is this project, and what is the one command
// to run next" by deriving both from the files on disk.
//
// It holds no state of its own. Every stage is a projection of the checks, which
// are the only judge of whether a project is consistent — so a status cannot
// disagree with the gate, and there is no checklist to fall out of date. That is
// the whole point: a plan a model keeps in its head is a plan it will
// eventually invent, and a plan kept in a markdown file is one someone forgets
// to tick.
//
// Nothing here affects an exit code. `nv validate` gates; this reports.
package pipeline

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cuongtranba/narrated-video/internal/checks"
	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/timing"
	"github.com/cuongtranba/narrated-video/internal/tts"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
)

type State string

const (
	Done    State = "done"
	Current State = "current"
	Blocked State = "blocked"
)

type Stage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	State   State  `json:"state"`
	Detail  string `json:"detail"`
	Command string `json:"command,omitempty"`
}

type Duration struct {
	Locale       string  `json:"locale"`
	Seconds      float64 `json:"seconds"`
	Measured     bool    `json:"measured"`
	Target       string  `json:"target,omitempty"`
	WithinTarget bool    `json:"withinTarget"`
}

type Spend struct {
	Provider     string  `json:"provider"`
	Characters   int     `json:"characters"`
	EstimatedUSD float64 `json:"estimatedUsd"`
	CapUSD       float64 `json:"capUsd"`
}

type Next struct {
	Command string `json:"command,omitempty"`
	Why     string `json:"why,omitempty"`
}

type Status struct {
	Project  string     `json:"project"`
	Root     string     `json:"root"`
	Locales  []string   `json:"locales"`
	Stages   []Stage    `json:"stages"`
	Duration []Duration `json:"duration"`
	Spend    Spend      `json:"spend"`
	Failing  []string   `json:"failing"`
	Next     Next       `json:"next"`
}

// Derive projects the check results onto the running order. The caller passes
// results it already has so a status never runs the gate twice.
func Derive(kit *checks.Kit, results []checks.Result) Status {
	ok := map[string]bool{}
	var failing []string
	for _, r := range results {
		ok[r.ID] = r.OK
		if !r.OK {
			failing = append(failing, r.ID)
		}
	}

	status := Status{
		Project:  kit.Config.Video.ID,
		Root:     kit.Root,
		Locales:  kit.Config.LocaleCodes(),
		Failing:  failing,
		Duration: durations(kit),
		Spend:    spend(kit),
	}
	status.Stages = stages(kit, ok, failing)
	status.Next = next(status.Stages)
	return status
}

// stages lists the running order and marks the first unfinished one current.
// Exactly one stage is current, because a checklist offering two next steps is
// one the reader has to arbitrate.
func stages(kit *checks.Kit, ok map[string]bool, failing []string) []Stage {
	built := []Stage{
		scaffoldStage(kit, ok),
		scriptStage(kit),
		scenesStage(kit, ok),
		voiceoverStage(kit, ok),
		gateStage(failing),
		renderStage(kit),
	}

	current := false
	for i := range built {
		if built[i].State == Done {
			continue
		}
		if current {
			built[i].State = Blocked
			continue
		}
		built[i].State = Current
		current = true
	}
	return built
}

func scaffoldStage(kit *checks.Kit, ok map[string]bool) Stage {
	missing := 0
	for _, name := range []string{"timeline.ts", "registry.ts", "theme.ts", "content.ts"} {
		if _, found := kit.Generated[kit.GeneratedPath(name)]; !found {
			missing++
		}
	}
	stage := Stage{ID: "scaffold", Title: "project", Command: "nv sync"}
	switch {
	case !ok["CHK-02"]:
		stage.Detail = "video.config.yaml does not satisfy the schema"
		stage.Command = "nv validate"
	case missing > 0:
		stage.Detail = fmt.Sprintf("%d of 4 generated files missing", missing)
	default:
		stage.State = Done
		stage.Detail = fmt.Sprintf("config valid, %d scenes, %d locale(s)",
			len(kit.Config.Scenes), len(kit.Config.Locales.List))
	}
	return stage
}

// scriptStage counts written lines rather than reading the gate, because a
// missing line is a script that is not finished — not a voiceover that failed.
// Sending the reader to the wrong file is the expensive half of that mistake.
func scriptStage(kit *checks.Kit) Stage {
	narrated := kit.NarratedScenes()
	written, total := 0, 0
	for _, locale := range kit.Config.LocaleCodes() {
		for _, sceneID := range narrated {
			total++
			if strings.TrimSpace(kit.Content[locale].Narration[sceneID]) != "" {
				written++
			}
		}
	}

	stage := Stage{
		ID:      "script",
		Title:   "narration",
		Detail:  fmt.Sprintf("%d/%d lines written", written, total),
		Command: "edit content/" + kit.Config.Locales.Default + ".yaml",
	}
	if written == total {
		stage.State = Done
	}
	return stage
}

func scenesStage(kit *checks.Kit, ok map[string]bool) Stage {
	stage := Stage{
		ID:      "scenes",
		Title:   "scene modules",
		Detail:  fmt.Sprintf("%d declared", len(kit.Config.Scenes)),
		Command: "nv init --scene <Id>",
	}
	if ok["CHK-24"] {
		stage.State = Done
		stage.Detail = fmt.Sprintf("%d modules, all declared", len(kit.Config.Scenes))
	}
	return stage
}

func voiceoverStage(kit *checks.Kit, ok map[string]bool) Stage {
	measured, total := 0, 0
	for _, locale := range kit.Config.LocaleCodes() {
		for _, scene := range kit.Timelines[locale].Scenes {
			if scene.Source == timing.Literal {
				continue
			}
			total++
			if scene.Source == timing.Measured && scene.HasAudio {
				measured++
			}
		}
	}

	stage := Stage{
		ID:      "voiceover",
		Title:   "measured audio",
		Detail:  fmt.Sprintf("%d/%d scenes measured", measured, total),
		Command: "nv voiceover",
	}
	if ok["CHK-05"] && ok["CHK-06"] {
		stage.State = Done
	}
	return stage
}

func gateStage(failing []string) Stage {
	stage := Stage{ID: "gate", Title: "nv validate", Command: "nv validate"}
	if len(failing) == 0 {
		stage.State = Done
		stage.Detail = "every check passing"
		return stage
	}
	stage.Detail = fmt.Sprintf("%d failing: %s", len(failing), strings.Join(failing, ", "))
	return stage
}

// renderStage is the one place a modification time is consulted, and it can
// never move an exit code: a render that predates its audio is a fact about the
// file, not about whether the project is consistent.
func renderStage(kit *checks.Kit) Stage {
	stage := Stage{ID: "render", Title: "rendered file", Command: renderCommand(kit.Root)}

	newest, at, found := newestRender(filepath.Join(kit.Root, kit.Config.Video.Out))
	if !found {
		stage.Detail = "nothing in " + kit.Config.Video.Out + "/"
		return stage
	}
	if audioAt, ok := newestManifest(kit); ok && at.Before(audioAt) {
		stage.Detail = newest + " predates the current audio"
		return stage
	}
	stage.State = Done
	stage.Detail = newest
	return stage
}

// renderCommand names the dependency install when there is nothing to render
// with. The scaffold points at `nv status` rather than listing steps, so the
// step it no longer lists has to be derivable.
func renderCommand(root string) string {
	if _, err := os.Stat(filepath.Join(root, "node_modules")); err != nil {
		return "bun install && bun run render"
	}
	return "bun run render"
}

func newestRender(dir string) (string, time.Time, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", time.Time{}, false
	}
	var name string
	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if at := info.ModTime(); name == "" || at.After(newest) {
			name, newest = filepath.Join(filepath.Base(dir), entry.Name()), at
		}
	}
	return name, newest, name != ""
}

func newestManifest(kit *checks.Kit) (time.Time, bool) {
	var newest time.Time
	found := false
	for _, locale := range kit.Config.LocaleCodes() {
		info, err := os.Stat(kit.ManifestPath(locale))
		if err != nil {
			continue
		}
		if at := info.ModTime(); !found || at.After(newest) {
			newest, found = at, true
		}
	}
	return newest, found
}

func next(stages []Stage) Next {
	for _, stage := range stages {
		if stage.State != Current {
			continue
		}
		return Next{Command: stage.Command, Why: stage.Detail}
	}
	return Next{}
}

func durations(kit *checks.Kit) []Duration {
	target := kit.Config.Video.TargetDuration

	var out []Duration
	for _, locale := range kit.Config.LocaleCodes() {
		timeline := kit.Timelines[locale]
		if timeline.FPS <= 0 {
			continue
		}
		seconds := float64(timeline.TotalFrames) / float64(timeline.FPS)
		out = append(out, Duration{
			Locale:       locale,
			Seconds:      seconds,
			Measured:     timeline.Complete,
			Target:       window(target),
			WithinTarget: target.Covers(seconds),
		})
	}
	return out
}

func window(t config.TargetDuration) string {
	switch {
	case t.MinSeconds > 0 && t.MaxSeconds > 0:
		return Clock(float64(t.MinSeconds)) + "–" + Clock(float64(t.MaxSeconds))
	case t.MaxSeconds > 0:
		return "at most " + Clock(float64(t.MaxSeconds))
	case t.MinSeconds > 0:
		return "at least " + Clock(float64(t.MinSeconds))
	default:
		return ""
	}
}

func spend(kit *checks.Kit) Spend {
	out := Spend{Provider: kit.Config.TTS.Provider, CapUSD: kit.Config.TTS.CostCapUSD}
	provider, err := tts.Get(kit.Config.TTS.Provider)
	if err != nil {
		return out
	}
	out.Characters, out.EstimatedUSD = voiceover.EstimateSpend(kit.Project, provider, kit.Config.Locales.List)
	return out
}

func Clock(seconds float64) string {
	total := int(seconds + 0.5)
	return fmt.Sprintf("%dm%02ds", total/60, total%60)
}

// Write prints the checklist. It is the same data --json carries, in the shape a
// person reads: the running order top to bottom, one marker per state, and the
// next command spelled out rather than implied.
func (s Status) Write(w io.Writer) {
	fmt.Fprintf(w, "%s · %s\n\n", s.Project, strings.Join(s.Locales, ", "))

	for _, stage := range s.Stages {
		fmt.Fprintf(w, "  %s %-10s %s\n", marker(stage.State), stage.ID, stage.Detail)
	}

	fmt.Fprintln(w)
	for _, d := range s.Duration {
		source := "estimated"
		if d.Measured {
			source = "measured"
		}
		line := fmt.Sprintf("  duration   %s  %s %s", d.Locale, Clock(d.Seconds), source)
		if d.Target != "" {
			line += fmt.Sprintf("  (target %s)%s", d.Target, verdict(d.WithinTarget))
		}
		fmt.Fprintln(w, line)
	}
	if s.Spend.Characters > 0 {
		fmt.Fprintf(w, "  spend      %d characters, $%.2f of $%.2f via %s\n",
			s.Spend.Characters, s.Spend.EstimatedUSD, s.Spend.CapUSD, s.Spend.Provider)
	}

	if s.Next.Command == "" {
		fmt.Fprintf(w, "\n  nothing outstanding\n")
		return
	}
	fmt.Fprintf(w, "\n  next: %s\n        %s\n", s.Next.Command, s.Next.Why)
}

func marker(state State) string {
	switch state {
	case Done:
		return "✔"
	case Current:
		return "▸"
	default:
		return "·"
	}
}

func verdict(ok bool) string {
	if ok {
		return "  ok"
	}
	return "  OUTSIDE"
}
