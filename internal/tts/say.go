package tts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const sayID = "say"

// say drives macOS's built-in speech synthesizer. It is the zero-setup way to
// hear a script read back; it is not a way to ship one.
type say struct{}

func (say) ID() string { return sayID }

func (say) RequiredEnv() []string { return nil }

func (say) Deterministic() bool { return false }

// Committable is false: the same text yields a different length on a different
// macOS version, with a different installed voice, or at a different system
// speech rate — so a scene timed against it on one machine is mistimed on the
// next. Its output must never become committed narration.
func (say) Committable() bool { return false }

// Policy clears everything: `say` selects a voice, not a model.
func (say) Policy() ModelPolicy {
	return ModelPolicy{Allow: map[string][]string{anyKey: {anyKey}}}
}

func (say) PricePer1kChars(string) float64 { return 0 }

func (s say) Synthesize(ctx context.Context, r Request) error {
	if err := CheckModel(s, r.Locale, r.Model); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("tts: %s: only available on macOS (running %s)", s.ID(), runtime.GOOS)
	}
	if r.Text == "" {
		return fmt.Errorf("tts: %s: no text for scene %q", s.ID(), r.SceneID)
	}
	sayBin, err := exec.LookPath("say")
	if err != nil {
		return fmt.Errorf("tts: %s: `say` not found on PATH: %w", s.ID(), err)
	}
	encoder, err := findEncoder()
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp("", "kanna-say-*")
	if err != nil {
		return fmt.Errorf("tts: %s: temp dir: %w", s.ID(), err)
	}
	defer os.RemoveAll(workDir)

	aiff := filepath.Join(workDir, "speech.aiff")
	args := []string{"-o", aiff}
	if r.VoiceID != "" {
		args = append(args, "-v", r.VoiceID)
	}
	args = append(args, "--", r.Text)
	if err := run(ctx, sayBin, args...); err != nil {
		return fmt.Errorf("tts: %s: %w", s.ID(), err)
	}

	mp3 := filepath.Join(workDir, "speech.mp3")
	if err := run(ctx, encoder.bin, encoder.args(aiff, mp3)...); err != nil {
		return fmt.Errorf("tts: %s: encode with %s: %w", s.ID(), encoder.name, err)
	}

	audio, err := os.ReadFile(mp3)
	if err != nil {
		return fmt.Errorf("tts: %s: read encoded audio: %w", s.ID(), err)
	}
	return writeOutput(r.OutPath, audio)
}

type encoder struct {
	name string
	bin  string
	args func(in, out string) []string
}

func findEncoder() (encoder, error) {
	if bin, err := exec.LookPath("ffmpeg"); err == nil {
		return encoder{
			name: "ffmpeg",
			bin:  bin,
			args: func(in, out string) []string {
				return []string{"-nostdin", "-loglevel", "error", "-y", "-i", in, "-codec:a", "libmp3lame", "-q:a", "2", out}
			},
		}, nil
	}
	if bin, err := exec.LookPath("lame"); err == nil {
		return encoder{
			name: "lame",
			bin:  bin,
			args: func(in, out string) []string { return []string{"--quiet", in, out} },
		}, nil
	}
	return encoder{}, fmt.Errorf(
		"tts: %s: needs an mp3 encoder on PATH, found neither ffmpeg nor lame (try `brew install ffmpeg`)", sayID)
}

// run reports the tool's own stderr, which is the only useful part of a failed
// encode.
func run(ctx context.Context, bin string, args ...string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if detail := strings.TrimSpace(string(out)); detail != "" {
		return fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, detail)
	}
	return fmt.Errorf("%s: %w", filepath.Base(bin), err)
}
