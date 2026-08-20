// Package timing derives how long every scene runs, from measured narration
// rather than from a table someone maintains by hand.
//
// The table it replaces was rewritten seven times in one project — once per
// text-to-speech run, because synthesis is not deterministic and identical text
// re-synthesized shifted lengths by up to fourteen frames. Each rewrite was a
// human recomputing narration + a pad. This package is that function, and the
// pad is the only number left for a person to choose.
package timing

import (
	"maps"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/cuongtranba/narrated-video/internal/config"
)

// Source records where a scene's length came from, so a draft can never be
// mistaken for a finished cut — the render badges anything not Measured.
type Source string

const (
	// Measured: taken from audio that exists on disk.
	Measured Source = "measured"
	// Estimated: predicted from character count because no audio has been made
	// yet. Accurate to roughly ±15%, which is enough to review layout.
	Estimated Source = "estimated"
	// Literal: a scene with no narration line, whose length was declared.
	Literal Source = "literal"
)

// ManifestEntry is one line's measurement, written by `nv voiceover`.
//
// Provider and Model are recorded because checking the config proves only what
// was *intended*: an environment override can synthesize a locale with a model
// that mispronounces it, and the resulting mp3 is indistinguishable from a good
// one without listening.
type ManifestEntry struct {
	SHA256   string  `json:"sha256"`
	Seconds  float64 `json:"seconds"`
	Frames   int     `json:"frames"`
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	VoiceID  string  `json:"voiceId"`
	Bytes    int64   `json:"bytes"`
}

// Manifest stamps the frame rate its measurements were taken at. Changing fps
// in the config otherwise invalidates every committed frame count in silence.
type Manifest struct {
	FPS      int                      `json:"fps"`
	Provider string                   `json:"provider"`
	Scenes   map[string]ManifestEntry `json:"scenes"`
}

type SceneTiming struct {
	ID               string
	LeadFrames       int
	NarrationFrames  int
	TailFrames       int
	DurationInFrames int
	Source           Source
	HasAudio         bool

	// Nil on a scene that belongs to no set, which is every scene in every
	// project written before sets existed.
	Set *SceneSet
}

// SceneSet is one scene's window onto a picture that several consecutive scenes
// share. Each of them redraws the whole picture at OffsetFrames, and because
// the picture is a pure function of that offset the two scenes either side of a
// boundary draw identical pixels there — so the cross-fade between them
// composites to those same pixels and the cut is not hidden but absent.
//
// Beats carries every beat's start, not this scene's index among them, and the
// omission is the guarantee. A picture that mutated on "which beat am I" would
// render its two neighbours differently at the one frame where they must agree,
// and the seam would reappear precisely where the design claims it cannot.
type SceneSet struct {
	Name         string
	OffsetFrames int
	SpanFrames   int
	Beats        []int
}

type LocaleTimeline struct {
	Locale           string
	FPS              int
	TransitionFrames int
	Scenes           []SceneTiming
	TotalFrames      int
	// Complete is false while any narrated scene is estimated or has no audio.
	// The render draws a badge from this rather than trusting the operator to
	// remember which cut they were looking at.
	Complete bool
}

type Input struct {
	Locale           string
	FPS              int
	TransitionFrames int
	Scenes           []config.Scene
	Manifest         *Manifest
	Narration        map[string]string
	AudioPresent     func(sceneID string) bool
	CharsPerSecond   float64
	// Sets maps a set name to the scenes it holds. Membership is declared
	// rather than inferred from imports, for the reason every other pairing in
	// this project is declared: a run that is only implied has nothing to
	// disagree with, so no check can catch the day a scene is inserted into
	// the middle of one and splits it in two.
	Sets map[string][]string
}

// Derive is the whole timing model: lead + narration + tail, with the
// cross-fade repaid once per boundary.
//
// A missing manifest is a normal state, not an error — a fresh clone with no
// API key still has to open in Studio and render. It degrades to an estimate
// and reports itself as one.
func Derive(in Input) LocaleTimeline {
	out := LocaleTimeline{
		Locale:           in.Locale,
		FPS:              in.FPS,
		TransitionFrames: in.TransitionFrames,
		Complete:         true,
	}

	total := 0
	for _, spec := range in.Scenes {
		st := SceneTiming{
			ID:         spec.ID,
			LeadFrames: spec.LeadFrames,
			TailFrames: spec.TailFrames,
		}

		switch {
		case !spec.Narrated:
			st.Source = Literal
			st.LeadFrames = 0
			st.TailFrames = 0
			st.DurationInFrames = spec.DurationInFrames

		default:
			st.HasAudio = in.AudioPresent != nil && in.AudioPresent(spec.ID)
			if entry, ok := measured(in.Manifest, spec.ID); ok {
				st.Source = Measured
				st.NarrationFrames = entry.Frames
			} else {
				st.Source = Estimated
				st.NarrationFrames = estimate(in.Narration[spec.ID], in.CharsPerSecond, in.FPS)
			}
			st.DurationInFrames = st.LeadFrames + st.NarrationFrames + st.TailFrames

			if st.Source != Measured || !st.HasAudio {
				out.Complete = false
			}
		}

		total += st.DurationInFrames
		out.Scenes = append(out.Scenes, st)
	}

	if n := len(out.Scenes); n > 1 {
		total -= in.TransitionFrames * (n - 1)
	}
	out.TotalFrames = total
	assignSets(out.Scenes, setNameByScene(in.Sets), in.TransitionFrames)
	return out
}

// setNameByScene inverts the declaration. A scene named by two sets takes the
// first in sorted order rather than whichever the map yielded, so a malformed
// config still derives the same timeline on every machine; CHK-43 is what
// refuses it.
func setNameByScene(sets map[string][]string) map[string]string {
	byScene := map[string]string{}
	for _, name := range slices.Sorted(maps.Keys(sets)) {
		for _, sceneID := range sets[name] {
			if _, taken := byScene[sceneID]; !taken {
				byScene[sceneID] = name
			}
		}
	}
	return byScene
}

// assignSets walks the running order and gives each run of consecutive scenes
// sharing a set its own clock.
//
// The offset advances by the scene's duration minus the cross-fade, the same
// repayment TotalFrames makes, because a transition overlaps its two
// neighbours: during those frames both scenes are on screen, and only this
// subtraction puts them on the same set frame. Advance by the raw duration and
// every beat after the first draws the picture a transition too late, so it
// jumps backwards at each boundary while every frame still renders.
//
// Scenes a set names out of order, or with a gap, form separate runs here. That
// is deliberate: the arithmetic stays honest about what is actually adjacent on
// screen, and CHK-43 reports the config that said otherwise.
func assignSets(scenes []SceneTiming, setNameByScene map[string]string, transitionFrames int) {
	for start := 0; start < len(scenes); {
		name := setNameByScene[scenes[start].ID]
		if name == "" {
			start++
			continue
		}

		end := start
		for end < len(scenes) && setNameByScene[scenes[end].ID] == name {
			end++
		}
		run := scenes[start:end]

		beats := make([]int, len(run))
		span := 0
		for i := range run {
			beats[i] = span
			span += run[i].DurationInFrames - transitionFrames
		}
		span += transitionFrames

		for i := range run {
			run[i].Set = &SceneSet{
				Name:         name,
				OffsetFrames: beats[i],
				SpanFrames:   span,
				Beats:        beats,
			}
		}
		start = end
	}
}

func measured(m *Manifest, sceneID string) (ManifestEntry, bool) {
	if m == nil {
		return ManifestEntry{}, false
	}
	entry, ok := m.Scenes[sceneID]
	if !ok || entry.Frames <= 0 {
		return ManifestEntry{}, false
	}
	return entry, true
}

// Runes, not bytes: a rate expressed per character means characters, and for
// any non-ASCII script byte length overstates it by half again.
func estimate(text string, charsPerSecond float64, fps int) int {
	if text == "" || charsPerSecond <= 0 {
		return 0
	}
	seconds := float64(utf8.RuneCountInString(text)) / charsPerSecond
	return int(math.Ceil(seconds * float64(fps)))
}

// At converts a reveal cue expressed as a fraction of its scene into a frame.
//
// This is what makes a second language nearly free. Cues tuned against one
// voice, kept as absolute frames, drift out of sync with the sentence
// explaining them the moment a translation changes the scene's length; as
// fractions they re-time themselves.
func At(fraction float64, durationInFrames int) int {
	return int(math.Round(fraction * float64(durationInFrames)))
}
