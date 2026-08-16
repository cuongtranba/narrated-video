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
	"math"
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
	return out
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
