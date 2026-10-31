package tts

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	vieneuID = "vieneu"

	// vieneuPythonEnv names the interpreter that has the `vieneu` package
	// installed. VieNeu ships no offline command: its console scripts are a
	// streaming server and a Gradio UI, so synthesis goes through its Python API.
	vieneuPythonEnv = "NV_VIENEU_PYTHON"
)

//go:embed vieneu_speak.py
var vieneuSpeakScript []byte

// vieneu is VieNeu-TTS (github.com/pnnbao97/VieNeu-TTS, Apache-2.0): a Vietnamese
// text-to-speech model that runs locally on CPU. It costs nothing and sends the
// script nowhere, which is why it can voice one locale of a project whose other
// locales use a hosted provider (tts.voices.<locale>.provider).
type vieneu struct{}

func (vieneu) ID() string { return vieneuID }

// RequiredEnv names the interpreter variable. Only `nv voiceover` needs it —
// `nv validate` reads what was already synthesized and must work on a machine
// that has no Python at all.
func (vieneu) RequiredEnv() []string { return []string{vieneuPythonEnv} }

// Deterministic is false: VieNeu samples, so the same line comes back with a
// different length each run. Scene timing is measured from the written bytes on
// every run for exactly this reason.
func (vieneu) Deterministic() bool { return false }

// Committable is true: like every provider, its output is timed by measuring
// the written file, and it is the narration a project ships.
func (vieneu) Committable() bool { return true }

// Policy lists only what a native speaker's ear has cleared.
func (vieneu) Policy() ModelPolicy {
	return ModelPolicy{
		Allow: map[string][]string{
			// v3turbo: cleared by ear by Cuong Tran (native speaker), 2026-10-31,
			// narrator clone. Other models and languages are unjudged, so CHK-04
			// refuses them.
			"vi": {"v3turbo"},
		},
	}
}

func (vieneu) PricePer1kChars(string) float64 { return 0 }

// vieneuSpokenTags are the bracketed cues VieNeu performs. Any other bracketed
// text would be read aloud literally.
var vieneuSpokenTags = map[string]bool{
	"[cười]":       true,
	"[thở dài]":    true,
	"[hắng giọng]": true,
}

var (
	bracketed  = regexp.MustCompile(`\[[^\[\]]*\]`)
	whitespace = regexp.MustCompile(`\s+`)
)

// speakable removes the bracketed tags VieNeu cannot perform, keeping the ones
// it can.
func speakable(text string) string {
	stripped := bracketed.ReplaceAllStringFunc(text, func(tag string) string {
		if vieneuSpokenTags[strings.ToLower(tag)] {
			return tag
		}
		return " "
	})
	return strings.TrimSpace(whitespace.ReplaceAllString(stripped, " "))
}

var referenceClipExtensions = map[string]bool{".wav": true, ".mp3": true, ".flac": true, ".m4a": true}

func (p vieneu) Synthesize(ctx context.Context, r Request) error {
	if err := CheckModel(p, r.Locale, r.Model); err != nil {
		return err
	}
	python := os.Getenv(vieneuPythonEnv)
	if python == "" {
		return fmt.Errorf("tts: %s: set %s to a Python interpreter that has the `vieneu` package installed", p.ID(), vieneuPythonEnv)
	}
	if r.VoiceID == "" {
		return fmt.Errorf("tts: %s: no voice for scene %q: set voiceId to a preset voice name or the path of a reference clip", p.ID(), r.SceneID)
	}
	text := speakable(r.Text)
	if text == "" {
		return fmt.Errorf("tts: %s: no text for scene %q", p.ID(), r.SceneID)
	}
	encoder, err := findEncoder(p.ID())
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "nv-vieneu-*")
	if err != nil {
		return fmt.Errorf("tts: %s: temp dir: %w", p.ID(), err)
	}
	defer os.RemoveAll(workDir)

	wav := filepath.Join(workDir, "speech.wav")
	request := map[string]string{"model": r.Model, "text": text, "out": wav}
	if referenceClipExtensions[strings.ToLower(filepath.Ext(r.VoiceID))] {
		clip := r.VoiceID
		if !filepath.IsAbs(clip) {
			clip = filepath.Join(r.ProjectRoot, clip)
		}
		if _, err := os.Stat(clip); err != nil {
			return fmt.Errorf("tts: %s: reference clip for scene %q: %w", p.ID(), r.SceneID, err)
		}
		request["ref_audio"] = clip
	} else {
		request["voice"] = r.VoiceID
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("tts: %s: encode request: %w", p.ID(), err)
	}

	script := filepath.Join(workDir, "nv_vieneu_speak.py")
	if err := os.WriteFile(script, vieneuSpeakScript, 0o600); err != nil {
		return fmt.Errorf("tts: %s: write helper: %w", p.ID(), err)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, python, script)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "HF_HUB_DISABLE_SYMLINKS=1")
	if err := cmd.Run(); err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("tts: %s: %s: %w\n%s", p.ID(), filepath.Base(python), err, detail)
		}
		return fmt.Errorf("tts: %s: %s: %w", p.ID(), filepath.Base(python), err)
	}

	mp3 := filepath.Join(workDir, "speech.mp3")
	if err := run(ctx, encoder.bin, encoder.cbr44k(wav, mp3)...); err != nil {
		return fmt.Errorf("tts: %s: encode with %s: %w", p.ID(), encoder.name, err)
	}
	audio, err := os.ReadFile(mp3)
	if err != nil {
		return fmt.Errorf("tts: %s: read encoded audio: %w", p.ID(), err)
	}
	return writeOutput(r.OutPath, audio)
}
