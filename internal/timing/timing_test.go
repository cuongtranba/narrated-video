package timing

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/config"
)

// The reference cut: eight scenes, two locales, and the sixteen frame counts a
// person maintained by hand across seven rewrites. Everything in this file
// checks the derivation against those numbers, because they are the only
// ground truth available — they were tuned by eye against real audio.
var referenceSceneFrames = map[string]map[string]int{
	"en": {
		"Title": 190, "Marathon": 410, "Arm": 437, "Iteration": 1209,
		"BlockedTools": 270, "Decision": 615, "Termination": 530, "Outro": 174,
	},
	"vi": {
		"Title": 185, "Marathon": 432, "Arm": 432, "Iteration": 1350,
		"BlockedTools": 310, "Decision": 662, "Termination": 542, "Outro": 178,
	},
}

var referenceTotalFrames = map[string]int{"en": 3737, "vi": 3993}

var referenceOrder = []string{
	"Title", "Marathon", "Arm", "Iteration",
	"BlockedTools", "Decision", "Termination", "Outro",
}

const referenceTransitionFrames = 14

// loadReferenceManifest reads the manifest the reference project actually
// produced. Its shape predates the fps/provider envelope this package writes,
// so the fixture stays byte-faithful and the test adapts instead.
func loadReferenceManifest(t *testing.T, locale string) *Manifest {
	t.Helper()
	path := filepath.Join("testdata", "reference-"+locale+"-manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var scenes map[string]ManifestEntry
	if err := json.Unmarshal(raw, &scenes); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return &Manifest{FPS: 30, Provider: "elevenlabs", Scenes: scenes}
}

func alwaysPresent(string) bool { return true }

// TestDerive_ReproducesReferenceExactly pins the arithmetic. Given the pad each
// scene actually carried, the derivation must land on the reference's number —
// not near it. If this drifts, every scene in every project shifts.
func TestDerive_ReproducesReferenceExactly(t *testing.T) {
	for _, locale := range []string{"en", "vi"} {
		t.Run(locale, func(t *testing.T) {
			manifest := loadReferenceManifest(t, locale)

			const lead = 14
			scenes := make([]config.Scene, 0, len(referenceOrder))
			for _, id := range referenceOrder {
				pad := referenceSceneFrames[locale][id] - manifest.Scenes[id].Frames
				scenes = append(scenes, config.Scene{
					ID: id, Narrated: true, LeadFrames: lead, TailFrames: pad - lead,
				})
			}

			got := Derive(Input{
				Locale:           locale,
				FPS:              30,
				TransitionFrames: referenceTransitionFrames,
				Scenes:           scenes,
				Manifest:         manifest,
				AudioPresent:     alwaysPresent,
			})

			for _, scene := range got.Scenes {
				want := referenceSceneFrames[locale][scene.ID]
				if scene.DurationInFrames != want {
					t.Errorf("%s: duration = %d, reference = %d", scene.ID, scene.DurationInFrames, want)
				}
				if scene.Source != Measured {
					t.Errorf("%s: source = %q, want %q", scene.ID, scene.Source, Measured)
				}
			}
			if got.TotalFrames != referenceTotalFrames[locale] {
				t.Errorf("total = %d, reference = %d", got.TotalFrames, referenceTotalFrames[locale])
			}
			if !got.Complete {
				t.Error("timeline should be complete: every scene measured with audio present")
			}
		})
	}
}

// TestDerive_OneDefaultPlusOneOverrideApproximatesReference is the design claim
// under test: the sixteen hand-maintained numbers were a default pad plus one
// deliberate hold. The Iteration scene keeps running after its narration stops,
// because the point of that scene is the silence.
func TestDerive_OneDefaultPlusOneOverrideApproximatesReference(t *testing.T) {
	const (
		defaultLead     = 14
		defaultTail     = 24
		iterationTail   = 50
		toleranceFrames = 8
	)

	scenes := make([]config.Scene, 0, len(referenceOrder))
	for _, id := range referenceOrder {
		tail := defaultTail
		if id == "Iteration" {
			tail = iterationTail
		}
		scenes = append(scenes, config.Scene{
			ID: id, Narrated: true, LeadFrames: defaultLead, TailFrames: tail,
		})
	}

	for _, locale := range []string{"en", "vi"} {
		t.Run(locale, func(t *testing.T) {
			got := Derive(Input{
				Locale:           locale,
				FPS:              30,
				TransitionFrames: referenceTransitionFrames,
				Scenes:           scenes,
				Manifest:         loadReferenceManifest(t, locale),
				AudioPresent:     alwaysPresent,
			})

			for _, scene := range got.Scenes {
				want := referenceSceneFrames[locale][scene.ID]
				if diff := abs(scene.DurationInFrames - want); diff > toleranceFrames {
					t.Errorf("%s: duration = %d, reference = %d (off by %d, tolerance %d)",
						scene.ID, scene.DurationInFrames, want, diff, toleranceFrames)
				}
			}
		})
	}
}

// A translation changes every scene's length, so the same config must produce
// different timelines per locale. This is what makes the frame table derivable
// at all rather than one table per language.
func TestDerive_LocalesDifferFromOneConfig(t *testing.T) {
	scenes := []config.Scene{{ID: "Iteration", Narrated: true, LeadFrames: 14, TailFrames: 24}}

	en := Derive(Input{Locale: "en", FPS: 30, Scenes: scenes, Manifest: loadReferenceManifest(t, "en"), AudioPresent: alwaysPresent})
	vi := Derive(Input{Locale: "vi", FPS: 30, Scenes: scenes, Manifest: loadReferenceManifest(t, "vi"), AudioPresent: alwaysPresent})

	if en.Scenes[0].DurationInFrames == vi.Scenes[0].DurationInFrames {
		t.Fatal("locales derived identical durations from different narration")
	}
}

// A fresh clone has no manifest and no API key. It still has to open in Studio
// and render, or the first thing a new contributor meets is a broken project.
func TestDerive_WithoutManifestEstimatesAndReportsIncomplete(t *testing.T) {
	got := Derive(Input{
		Locale:         "en",
		FPS:            30,
		Scenes:         []config.Scene{{ID: "Title", Narrated: true, LeadFrames: 14, TailFrames: 24}},
		Narration:      map[string]string{"Title": "This is the loop."},
		AudioPresent:   func(string) bool { return false },
		CharsPerSecond: 16.17,
	})

	scene := got.Scenes[0]
	if scene.Source != Estimated {
		t.Errorf("source = %q, want %q", scene.Source, Estimated)
	}
	if scene.NarrationFrames <= 0 {
		t.Error("estimate produced no narration frames")
	}
	if got.Complete {
		t.Error("an estimated timeline must not report itself complete")
	}
}

// Audio can be absent while the manifest is present — a clone that skipped the
// large files. The length stays measured; only completeness changes.
func TestDerive_MeasuredButAudioMissingIsIncomplete(t *testing.T) {
	got := Derive(Input{
		Locale:       "en",
		FPS:          30,
		Scenes:       []config.Scene{{ID: "Title", Narrated: true, LeadFrames: 14, TailFrames: 24}},
		Manifest:     loadReferenceManifest(t, "en"),
		AudioPresent: func(string) bool { return false },
	})

	if got.Scenes[0].Source != Measured {
		t.Errorf("source = %q, want %q", got.Scenes[0].Source, Measured)
	}
	if got.Complete {
		t.Error("missing audio must not report itself complete")
	}
}

// An unnarrated scene carries no lead or tail: those pad a voice, and there is
// no voice to pad.
func TestDerive_UnnarratedSceneUsesDeclaredDuration(t *testing.T) {
	got := Derive(Input{
		Locale: "en", FPS: 30,
		Scenes: []config.Scene{{ID: "Credits", Narrated: false, DurationInFrames: 120, LeadFrames: 14, TailFrames: 24}},
	})

	scene := got.Scenes[0]
	if scene.DurationInFrames != 120 {
		t.Errorf("duration = %d, want 120", scene.DurationInFrames)
	}
	if scene.Source != Literal {
		t.Errorf("source = %q, want %q", scene.Source, Literal)
	}
	if scene.LeadFrames != 0 || scene.TailFrames != 0 {
		t.Errorf("lead/tail = %d/%d, want 0/0", scene.LeadFrames, scene.TailFrames)
	}
	if !got.Complete {
		t.Error("a scene that needs no audio must not hold the timeline incomplete")
	}
}

func TestAt(t *testing.T) {
	for _, tc := range []struct {
		fraction float64
		duration int
		want     int
	}{
		{0, 1209, 0},
		{0.5, 1209, 605},
		{1, 1209, 1209},
		{0.4, 437, 175},
	} {
		if got := At(tc.fraction, tc.duration); got != tc.want {
			t.Errorf("At(%v, %d) = %d, want %d", tc.fraction, tc.duration, got, tc.want)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
