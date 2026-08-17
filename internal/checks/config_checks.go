package checks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/gen"
	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
	"github.com/cuongtranba/narrated-video/internal/tts"
)

// CHK-01. The generated timeline is the one thing in the project nobody reads
// closely, and a scene that drifted by fourteen frames renders perfectly at
// every frame. Only regenerating and comparing bytes finds it.
func generatedFresh(kit *Kit) Result {
	const id, title = "CHK-01", "generated files match the config and the measured audio"

	stale, err := gen.Diff(kit.Project, func(path string) ([]byte, error) {
		data, ok := kit.Generated[path]
		if !ok {
			return nil, os.ErrNotExist
		}
		return data, nil
	})
	if err != nil {
		return fail(id, title, "fix the error above, then run: nv sync",
			[]Finding{{Where: config.FileName, Detail: err.Error()}})
	}

	var findings []Finding
	for _, path := range stale {
		findings = append(findings, Finding{Where: relTo(kit.Root, path), Detail: "differs from a fresh derivation"})
	}
	return fail(id, title, "run: nv sync", sortedFindings(findings))
}

// CHK-02. Load already refuses an invalid config, so reaching here means the
// document parsed; re-running the validation keeps the reported check set
// complete rather than silently absent on the one path that matters.
func schemaValid(kit *Kit) Result {
	const id, title = "CHK-02", "video.config.yaml satisfies the published schema"

	raw, err := os.ReadFile(filepath.Join(kit.Root, config.FileName))
	if err != nil {
		return fail(id, title, "create video.config.yaml, or run: nv init",
			[]Finding{{Where: config.FileName, Detail: err.Error()}})
	}
	if err := config.ValidateSchema(raw, config.FileName); err != nil {
		return fail(id, title, "correct the fields named above — every one is documented in references/config-schema.md",
			[]Finding{{Where: config.FileName, Detail: err.Error()}})
	}
	return pass(id, title)
}

// A key shape, not a key. The project this was built from pasted a live
// ElevenLabs key into a dozen shell commands; scanning only the config would
// have caught none of them.
var secretShapes = []*regexp.Regexp{
	regexp.MustCompile(`\bsk_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`\bxi-[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{30,}`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9]{30,}`),
}

var envVarName = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// CHK-03. Secrets are referenced by env-var name and never stored. A key in a
// tracked file is in the history from the moment it lands, and rotating it is
// the only fix — so the cheapest place to catch it is before the commit.
func noSecretLiterals(kit *Kit) Result {
	const id, title = "CHK-03", "no secret is written down anywhere"

	var findings []Finding
	if name := kit.Config.TTS.APIKeyEnv; name != "" && !envVarName.MatchString(name) {
		findings = append(findings, Finding{
			Where:  config.FileName + " tts.apiKeyEnv",
			Detail: fmt.Sprintf("%q is not an environment variable name", name),
		})
	}

	for _, path := range sortedStringKeys(kit.TrackedFiles) {
		for _, shape := range secretShapes {
			if match := shape.FindString(kit.TrackedFiles[path]); match != "" {
				findings = append(findings, Finding{
					Where:  path,
					Detail: fmt.Sprintf("looks like a live credential (%s…)", match[:min(8, len(match))]),
				})
				break
			}
		}
	}
	return fail(id, title, "reference the secret by name instead — tts.apiKeyEnv: ELEVENLABS_API_KEY — and rotate anything already committed", sortedFindings(findings))
}

// CHK-04. The model is per locale and fails silently: a model that mispronounces
// a language still returns success and audio of a plausible length. An unlisted
// model fails too — one nobody has judged by ear must not default into use.
func ttsModelAllowed(kit *Kit) Result {
	const id, title = "CHK-04", "every locale's TTS model is cleared for that language"

	provider, err := tts.Get(kit.Config.TTS.Provider)
	if err != nil {
		return fail(id, title, "set tts.provider to a provider the tool ships — see references/tts-providers.md",
			[]Finding{{Where: config.FileName + " tts.provider", Detail: err.Error()}})
	}

	var findings []Finding
	for _, locale := range kit.Config.Locales.List {
		voice, ok := kit.Config.TTS.Voices[locale.Code]
		if !ok {
			findings = append(findings, Finding{Where: locale.Code, Detail: "no voice configured"})
			continue
		}
		if err := tts.CheckModel(provider, locale.Code, voice.Model); err != nil {
			findings = append(findings, Finding{Where: locale.Code, Detail: err.Error()})
		}
	}
	return fail(id, title, "pick a model cleared for the language, or clear this one by ear and record it in the provider's policy", sortedFindings(findings))
}

// CHK-19. Some providers exist to make a project runnable without an API key,
// not to produce narration anyone ships. macOS `say` is one: its length varies
// with the OS version, the installed voice and the system speech rate, so audio
// made with it cannot be reproduced on another machine.
func providerCommittable(kit *Kit) Result {
	const id, title = "CHK-19", "committed audio came from a provider whose output may ship"

	var findings []Finding
	for _, locale := range kit.Config.Locales.List {
		manifest := kit.Manifests[locale.Code]
		if manifest == nil || manifest.Provider == "" {
			continue
		}
		provider, err := tts.Get(manifest.Provider)
		if err != nil {
			findings = append(findings, Finding{Where: locale.Code, Detail: fmt.Sprintf("audio records unknown provider %q", manifest.Provider)})
			continue
		}
		if !provider.Committable() {
			findings = append(findings, Finding{
				Where:  locale.Code,
				Detail: fmt.Sprintf("audio was produced by %q, which must not ship", manifest.Provider),
			})
		}
	}
	return fail(id, title, "re-run with the real provider: nv voiceover -- <locale>", sortedFindings(findings))
}

// CHK-20. The schema already insists a ceiling exists, so repeating that here
// would be a check that can never fire. What it is worth knowing before running
// synthesis is whether the script as written would breach it — billing is per
// character and a loop over locales multiplies it quietly.
func costCapDeclared(kit *Kit) Result {
	const id, title = "CHK-20", "the declared spend ceiling covers the current script"

	provider, err := tts.Get(kit.Config.TTS.Provider)
	if err != nil {
		return pass(id, title) // CHK-04 owns an unknown provider.
	}

	estimate := 0.0
	for _, locale := range kit.Config.LocaleCodes() {
		price := provider.PricePer1kChars(kit.Config.TTS.Voices[locale].Model)
		for _, sceneID := range kit.NarratedScenes() {
			line := kit.Content[locale].Narration[sceneID]
			estimate += float64(len([]rune(line))) / 1000 * price
		}
	}

	if cap := kit.Config.TTS.CostCapUSD; estimate > cap {
		return fail(id, title, "raise tts.costCapUsd, shorten the script, or synthesize one locale at a time",
			[]Finding{{
				Where:  config.FileName + " tts.costCapUsd",
				Detail: fmt.Sprintf("synthesizing every locale is estimated at $%.2f against a ceiling of $%.2f", estimate, cap),
			}})
	}
	return pass(id, title)
}

// CHK-12. A narrated scene takes its length from its audio; an unnarrated one
// declares it. Allowing both on one scene is how a hand-maintained frame table
// grows back after the derivation removed it.
func sceneDurationExclusivity(kit *Kit) Result {
	const id, title = "CHK-12", "scene length comes from exactly one source"

	var findings []Finding
	for _, scene := range kit.Config.Scenes {
		switch {
		case scene.Narrated && scene.DurationInFrames > 0:
			findings = append(findings, Finding{Where: scene.ID, Detail: "narrated, but declares durationInFrames"})
		case !scene.Narrated && scene.DurationInFrames <= 0:
			findings = append(findings, Finding{Where: scene.ID, Detail: "unnarrated, but declares no durationInFrames"})
		}
	}
	return fail(id, title, "a narrated scene sets leadFrames/tailFrames only; an unnarrated one sets durationInFrames only", sortedFindings(findings))
}

// CHK-13. The cross-fade overlaps two scenes. A line that starts before its own
// scene is opaque is spoken over the previous one — audible, and invisible in
// any single frame.
func leadCoversTransition(kit *Kit) Result {
	const id, title = "CHK-13", "no narration begins before its scene is fully opaque"

	transition := kit.Config.Video.TransitionFrames
	var findings []Finding
	for i, scene := range kit.Config.Scenes {
		if i == 0 || !scene.Narrated {
			continue
		}
		if scene.LeadFrames < transition {
			findings = append(findings, Finding{
				Where:  scene.ID,
				Detail: fmt.Sprintf("leadFrames %d is under the %d-frame cross-fade", scene.LeadFrames, transition),
			})
		}
	}
	return fail(id, title, fmt.Sprintf("raise leadFrames to at least %d, or lower video.transitionFrames", transition), sortedFindings(findings))
}

// CHK-25. Reveals are cued as fractions of the scene, so a short scene collapses
// them onto each other and the frame flashes through its own content.
func minSceneDuration(kit *Kit) Result {
	const id, title = "CHK-25", "every scene is long enough for its reveals to separate"

	floor := kit.Config.Video.MinSceneFrames
	var findings []Finding
	for _, locale := range kit.Config.LocaleCodes() {
		for _, scene := range kit.Timelines[locale].Scenes {
			if scene.DurationInFrames < floor {
				findings = append(findings, Finding{
					Where:  at(locale, scene.ID),
					Detail: fmt.Sprintf("%d frames, floor is %d", scene.DurationInFrames, floor),
				})
			}
		}
	}
	return fail(id, title, "lengthen the line, raise tailFrames, or lower video.minSceneFrames", sortedFindings(findings))
}

// CHK-24. Reported separately from staleness because the message is what the
// reader needs: a name in the config with no file behind it.
func registryParity(kit *Kit) Result {
	const id, title = "CHK-24", "every declared scene has a module and every module is declared"

	declared := map[string]bool{}
	var findings []Finding
	for _, scene := range kit.Config.Scenes {
		declared[scene.ID] = true
		if _, err := os.Stat(kit.ScenePath(scene.ID)); errors.Is(err, os.ErrNotExist) {
			findings = append(findings, Finding{
				Where:  scene.ID,
				Detail: fmt.Sprintf("declared in the config, but %s does not exist", relTo(kit.Root, kit.ScenePath(scene.ID))),
			})
		}
	}

	entries, err := os.ReadDir(filepath.Join(kit.Root, "src", "scenes"))
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".tsx") || strings.HasPrefix(name, "_") {
				continue
			}
			id := strings.TrimSuffix(name, ".tsx")
			if !declared[id] {
				findings = append(findings, Finding{Where: id, Detail: "a scene module nothing in the config refers to"})
			}
		}
	}
	return fail(id, title, "add the scene to video.config.yaml (nv init --scene <Id> scaffolds one), or delete the orphan module", sortedFindings(findings))
}

func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// CHK-27. The composition id lives in video.config.yaml, but package.json's
// render and still scripts carried a copy typed once by the template. Rename the
// composition and `bun run render` points at one that no longer exists — a
// failure that surfaces after the voiceover has been paid for.
//
// A script written in a form pkgscripts disowns is not a finding: nv claims the
// id, not the command around it.
func renderScriptsTargetComposition(kit *Kit) Result {
	const id, title = "CHK-27", "the render scripts target the composition this config declares"

	file, err := pkgscripts.Load(kit.Root)
	if err != nil || file == nil {
		return pass(id, title)
	}

	want := kit.Config.CompositionID(kit.Config.Locales.Default)
	var findings []Finding
	for _, name := range pkgscripts.Managed {
		got, ok := file.Target(name)
		if !ok || got == want {
			continue
		}
		findings = append(findings, Finding{
			Where:  pkgscripts.FileName + " scripts." + name,
			Detail: fmt.Sprintf("renders %q, but video.id declares %q", got, want),
		})
	}
	return fail(id, title, "run: nv sync", sortedFindings(findings))
}
