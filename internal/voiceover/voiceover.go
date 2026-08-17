// Package voiceover synthesizes narration and records what it measured.
//
// The measurement is the point. Scene lengths derive from audio duration, so
// that number is the timing contract for the whole video — and it is taken from
// the bytes that were written, never from what a provider claims it produced.
package voiceover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/mp3"
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/timing"
	"github.com/cuongtranba/narrated-video/internal/tts"
)

type Options struct {
	// Locales limits the run. Empty means every locale: re-synthesizing one
	// language must never disturb another's measurements, since each is timed
	// from its own audio.
	Locales []string
	// Force proceeds past the declared spend ceiling.
	Force bool
	// APIKey is read from the environment by the caller and never persisted.
	APIKey string
}

func Run(ctx context.Context, p *project.Project, opts Options, out io.Writer) error {
	provider, err := tts.Get(p.Config.TTS.Provider)
	if err != nil {
		return err
	}

	locales, err := resolveLocales(p, opts.Locales)
	if err != nil {
		return err
	}

	if err := checkSpend(p, provider, locales, opts.Force, out); err != nil {
		return err
	}

	for _, locale := range locales {
		if err := synthesizeLocale(ctx, p, provider, locale, opts.APIKey, out); err != nil {
			return fmt.Errorf("%s: %w", locale.Code, err)
		}
	}
	return nil
}

func resolveLocales(p *project.Project, requested []string) ([]config.Locale, error) {
	if len(requested) == 0 {
		return p.Config.Locales.List, nil
	}
	var out []config.Locale
	for _, code := range requested {
		locale, ok := p.Config.LocaleByCode(code)
		if !ok {
			return nil, fmt.Errorf("unknown locale %q — the project declares %v", code, p.Config.LocaleCodes())
		}
		out = append(out, locale)
	}
	return out, nil
}

// EstimateSpend reports the characters a run would send and what it would cost.
// Exported so `nv status` quotes the same number `nv voiceover` will enforce:
// two implementations of a price would eventually disagree, and the one the
// operator read is not the one that bills.
func EstimateSpend(p *project.Project, provider tts.Provider, locales []config.Locale) (int, float64) {
	characters := 0
	estimate := 0.0
	for _, locale := range locales {
		voice := p.Config.TTS.Voices[locale.Code]
		for _, sceneID := range p.NarratedScenes() {
			line := p.Content[locale.Code].Narration[sceneID]
			characters += len([]rune(line))
			estimate += float64(len([]rune(line))) / 1000 * provider.PricePer1kChars(voice.Model)
		}
	}
	return characters, estimate
}

// checkSpend estimates before the first request rather than after the last.
// Synthesis is billed per character and a loop over locales multiplies it
// quietly; the ceiling is the config's, so it is reviewed with everything else.
func checkSpend(p *project.Project, provider tts.Provider, locales []config.Locale, force bool, out io.Writer) error {
	characters, estimate := EstimateSpend(p, provider, locales)

	fmt.Fprintf(out, "%d characters across %d locale(s), estimated $%.2f (cap $%.2f)\n",
		characters, len(locales), estimate, p.Config.TTS.CostCapUSD)

	if estimate > p.Config.TTS.CostCapUSD && !force {
		return fmt.Errorf("estimated $%.2f exceeds tts.costCapUsd of $%.2f — raise the cap or pass --force",
			estimate, p.Config.TTS.CostCapUSD)
	}
	return nil
}

func synthesizeLocale(ctx context.Context, p *project.Project, provider tts.Provider, locale config.Locale, apiKey string, out io.Writer) error {
	voice, ok := p.Config.TTS.Voices[locale.Code]
	if !ok {
		return fmt.Errorf("no voice configured")
	}
	// Refused here as well as in validate, because this is the step that would
	// otherwise write a file nothing downstream can tell is wrong.
	if err := tts.CheckModel(provider, locale.Code, voice.Model); err != nil {
		return err
	}

	dir := p.VoiceoverDir(locale.Code)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n── %s — %s via %s ──\n", locale.Code, voice.Model, provider.ID())

	manifest := timing.Manifest{
		FPS:      p.Config.Video.FPS,
		Provider: provider.ID(),
		Scenes:   map[string]timing.ManifestEntry{},
	}

	for _, sceneID := range p.NarratedScenes() {
		line := p.Content[locale.Code].Narration[sceneID]
		if line == "" {
			return fmt.Errorf("%s has no narration line", sceneID)
		}

		path := p.AudioPath(locale.Code, sceneID)
		err := provider.Synthesize(ctx, tts.Request{
			Text:           line,
			Locale:         locale.Code,
			SceneID:        sceneID,
			OutPath:        path,
			VoiceID:        voice.VoiceID,
			Model:          voice.Model,
			VoiceSettings:  voice.VoiceSettings,
			OutputFormat:   p.Config.TTS.OutputFormat,
			CharsPerSecond: locale.CharsPerSecond,
			APIKey:         apiKey,
		})
		if err != nil {
			return fmt.Errorf("%s: %w", sceneID, err)
		}

		entry, err := measure(path, line, provider.ID(), voice, p.Config.Video.FPS)
		if err != nil {
			return fmt.Errorf("%s: %w", sceneID, err)
		}
		manifest.Scenes[sceneID] = entry

		fmt.Fprintf(out, "%-16s %6.2fs  %5df\n", sceneID, entry.Seconds, entry.Frames)
	}

	return writeManifest(p.ManifestPath(locale.Code), manifest)
}

func measure(path, line, providerID string, voice config.Voice, fps int) (timing.ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return timing.ManifestEntry{}, err
	}
	duration, err := mp3.Duration(data)
	if err != nil {
		return timing.ManifestEntry{}, err
	}

	seconds := duration.Seconds()
	sum := sha256.Sum256([]byte(line))
	return timing.ManifestEntry{
		SHA256:   hex.EncodeToString(sum[:]),
		Seconds:  math.Round(seconds*100) / 100,
		Frames:   int(math.Ceil(seconds * float64(fps))),
		Provider: providerID,
		Model:    voice.Model,
		VoiceID:  voice.VoiceID,
		Bytes:    int64(len(data)),
	}, nil
}

func writeManifest(path string, manifest timing.Manifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}
