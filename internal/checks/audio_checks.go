package checks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/timing"
	"github.com/cuongtranba/narrated-video/internal/tts"
)

// CHK-05. An estimated length is a placeholder derived from character count.
// It is close enough to review a layout and not close enough to ship: the frame
// and the voice drift apart over the scene.
func narrationMeasured(kit *Kit) Result {
	const id, title = "CHK-05", "every narrated scene is timed from real audio"

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		for _, scene := range kit.Timelines[locale].Scenes {
			if scene.Source == timing.Estimated {
				findings = append(findings, Finding{Where: at(locale, scene.ID), Detail: "length is an estimate, not a measurement"})
			}
		}
	}
	return fail(id, title, "run: nv voiceover -- <locale>", sortedFindings(findings))
}

// CHK-06. Separate from the hash check because the remedy differs: a clone that
// skipped large files needs them fetched, not re-synthesized.
func audioPresent(kit *Kit) Result {
	const id, title = "CHK-06", "every narrated scene has its audio file"

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		for _, scene := range kit.Timelines[locale].Scenes {
			if scene.Source != timing.Literal && !scene.HasAudio {
				findings = append(findings, Finding{
					Where:  at(locale, scene.ID),
					Detail: fmt.Sprintf("%s is missing", relTo(kit.Root, kit.AudioPath(locale, scene.ID))),
				})
			}
		}
	}
	return fail(id, title, "run: nv voiceover -- <locale> (or fetch the committed audio)", sortedFindings(findings))
}

// CHK-07. The one drift a render cannot reveal. Edit a line, forget to
// regenerate, and the old take still plays: the voice contradicts the frame,
// and there is nothing on screen to show it.
func narrationHash(kit *Kit) Result {
	const id, title = "CHK-07", "every audio file was spoken from the line that is there now"

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		manifest := kit.Manifests[locale]
		if manifest == nil {
			continue
		}
		narration := kit.Content[locale].Narration
		for _, scene := range sortedStringKeys(narration) {
			entry, ok := manifest.Scenes[scene]
			if !ok {
				continue
			}
			if spoken := sha256Hex(narration[scene]); entry.SHA256 != spoken {
				findings = append(findings, Finding{
					Where:  at(locale, scene),
					Detail: "the line was edited after the take was recorded",
				})
			}
		}
	}
	return fail(id, title, "run: nv voiceover -- <locale>", sortedFindings(findings))
}

// CHK-08. Checking the config proves only what was intended. An environment
// override can synthesize a locale with a model that mispronounces it, and the
// resulting file is indistinguishable from a good one without listening — so
// the manifest records what was actually used, and that is what is judged.
func manifestModelRecorded(kit *Kit) Result {
	const id, title = "CHK-08", "the audio on disk was made with a model cleared for its language"

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		manifest := kit.Manifests[locale]
		if manifest == nil {
			continue
		}
		for _, scene := range sortedManifestKeys(manifest.Scenes) {
			entry := manifest.Scenes[scene]
			if entry.Model == "" || entry.Provider == "" {
				findings = append(findings, Finding{
					Where:  at(locale, scene),
					Detail: "the take records no provider or model, so what produced it cannot be judged",
				})
				continue
			}
			provider, err := tts.Get(entry.Provider)
			if err != nil {
				findings = append(findings, Finding{Where: at(locale, scene), Detail: err.Error()})
				continue
			}
			if err := tts.CheckModel(provider, locale, entry.Model); err != nil {
				findings = append(findings, Finding{
					Where:  at(locale, scene),
					Detail: fmt.Sprintf("was synthesized with %s: %v", entry.Model, err),
				})
			}
		}
	}
	return fail(id, title, "re-run: nv voiceover -- <locale>, without a model override", sortedFindings(findings))
}

// CHK-09. Frame counts are measured at one frame rate. Change it in the config
// and every committed number silently means something else.
func manifestFPS(kit *Kit) Result {
	const id, title = "CHK-09", "measurements were taken at the video's frame rate"

	want := kit.Config.Video.FPS
	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		manifest := kit.Manifests[locale]
		if manifest == nil {
			continue
		}
		if manifest.FPS != want {
			findings = append(findings, Finding{
				Where:  locale,
				Detail: fmt.Sprintf("audio was measured at %d fps, the video is %d", manifest.FPS, want),
			})
		}
	}
	return fail(id, title, "re-measure at the current frame rate: nv voiceover -- <locale>", sortedFindings(findings))
}

// CHK-10. A narration key with no scene is a line nobody hears; a narrated
// scene with no line is silence where the argument was. Both are quiet.
func narrationKeys(kit *Kit) Result {
	const id, title = "CHK-10", "narration lines and narrated scenes correspond"

	narrated := map[string]bool{}
	for _, sceneID := range kit.NarratedScenes() {
		narrated[sceneID] = true
	}

	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		narration := kit.Content[locale].Narration
		for _, scene := range sortedStringKeys(narration) {
			if !narrated[scene] {
				findings = append(findings, Finding{
					Where:  at(locale, scene),
					Detail: "a narration line for a scene that is not narrated (or does not exist)",
				})
			}
		}
		for _, sceneID := range kit.NarratedScenes() {
			if narration[sceneID] == "" {
				findings = append(findings, Finding{
					Where:  at(locale, sceneID),
					Detail: fmt.Sprintf("no narration line in %s", relTo(kit.Root, kit.ContentPath(locale))),
				})
			}
		}
	}
	return fail(id, title, "add the missing line, or mark the scene unnarrated with durationInFrames in "+config.FileName, sortedFindings(findings))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedManifestKeys(m map[string]timing.ManifestEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
