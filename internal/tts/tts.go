// Package tts turns narration text into MP3 files.
//
// Providers write bytes and nothing else. Duration is measured from those bytes
// by the caller (internal/mp3), uniformly for every provider, because a
// provider's self-reported duration is not the timing contract — the audio is.
package tts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Request is one scene's narration job.
type Request struct {
	Text    string
	Locale  string
	SceneID string
	OutPath string // provider MUST write an mp3 here

	VoiceID string
	Model   string
	// VoiceSettings nil means: send no voice_settings field at all. Settings
	// tuned against one model are not tuning for another, so the absence has to
	// survive all the way to the wire rather than being filled with defaults.
	VoiceSettings map[string]float64
	OutputFormat  string

	CharsPerSecond float64 // offline providers only

	APIKey string // never logged, never persisted
}

// DeniedModel records a model that must not be used, and the observation that
// disqualified it. The reason travels with the denial because these failures are
// inaudible to the machine: the caller can only act on prose.
type DeniedModel struct {
	Model string
	Why   string
}

// ModelPolicy is which models a native speaker's ear has cleared, per locale.
//
// The locale key "*" applies to every locale, and the model entry "*" in Allow
// clears every model — for providers whose output does not vary by model.
type ModelPolicy struct {
	Deny  map[string][]DeniedModel // locale -> models that must not be used
	Allow map[string][]string      // locale -> models cleared by ear
}

// Provider synthesizes narration audio.
type Provider interface {
	ID() string
	RequiredEnv() []string // env var NAMES only
	Deterministic() bool   // same text => same bytes on any machine
	Committable() bool     // may its output ship as the narration?
	Policy() ModelPolicy
	PricePer1kChars(model string) float64
	// Synthesize writes r.OutPath and returns no duration, by design: the only
	// duration the renderer trusts is the one measured from the written bytes.
	Synthesize(ctx context.Context, r Request) error
}

// Model policy failures, distinguishable because they mean different things to a
// caller: one model is known bad, the other is merely unproven.
var (
	ErrModelDenied   = errors.New("model denied for locale")
	ErrModelUnlisted = errors.New("model not cleared for locale")
)

// CheckModel reports why a model is not usable for a locale. An UNLISTED model is
// a failure too: a model never judged by a native speaker's ear must not default
// into use just because the request would succeed.
func CheckModel(p Provider, locale, model string) error {
	policy := p.Policy()

	denials := append(append([]DeniedModel(nil), policy.Deny[locale]...), policy.Deny[anyKey]...)
	for _, denied := range denials {
		if denied.Model == model {
			return fmt.Errorf("tts: %s: %w: model %q for locale %q: %s",
				p.ID(), ErrModelDenied, model, locale, denied.Why)
		}
	}

	allowed := append(append([]string(nil), policy.Allow[locale]...), policy.Allow[anyKey]...)
	for _, a := range allowed {
		if a == model || a == anyKey {
			return nil
		}
	}

	if len(allowed) == 0 {
		return fmt.Errorf("tts: %s: %w: no model has been cleared by ear for locale %q, so %q cannot be used",
			p.ID(), ErrModelUnlisted, locale, model)
	}
	return fmt.Errorf("tts: %s: %w: model %q for locale %q; cleared by ear: %s",
		p.ID(), ErrModelUnlisted, model, locale, strings.Join(allowed, ", "))
}

const anyKey = "*"

var registry = map[string]Provider{
	elevenLabsID: newElevenLabs(),
	silenceID:    silence{},
	sayID:        say{},
}

// Get returns the provider registered under id.
func Get(id string) (Provider, error) {
	p, ok := registry[id]
	if !ok {
		ids := make([]string, 0, len(registry))
		for k := range registry {
			ids = append(ids, k)
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("tts: unknown provider %q (known: %s)", id, strings.Join(ids, ", "))
	}
	return p, nil
}

// All returns every provider, ordered by id so listings are stable.
func All() []Provider {
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// writeOutput publishes audio through a rename so a measured file is never a
// partial one: the caller measures duration from whatever is at OutPath, and a
// truncated read would silently shorten a scene.
func writeOutput(path string, data []byte) error {
	if path == "" {
		return errors.New("tts: no output path")
	}
	if len(data) == 0 {
		return fmt.Errorf("tts: refusing to write empty audio to %s", path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("tts: create output dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tts-*.mp3.tmp")
	if err != nil {
		return fmt.Errorf("tts: create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("tts: write audio: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("tts: close audio: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("tts: publish audio: %w", err)
	}
	return nil
}
