package tts

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuongtranba/narrated-video/internal/mp3"
)

func TestVieneu_OnlyTheModelClearedByEarIsAllowedForVietnamese(t *testing.T) {
	p, err := Get(vieneuID)
	if err != nil {
		t.Fatal(err)
	}

	if err := CheckModel(p, "vi", "v3turbo"); err != nil {
		t.Errorf("v3turbo for vi = %v, want it cleared", err)
	}
	if err := CheckModel(p, "vi", "v2"); !errors.Is(err, ErrModelUnlisted) {
		t.Errorf("an uncleared model for vi = %v, want ErrModelUnlisted", err)
	}
	if err := CheckModel(p, "en", "v3turbo"); !errors.Is(err, ErrModelUnlisted) {
		t.Errorf("v3turbo for en = %v, want ErrModelUnlisted: nobody judged it English by ear", err)
	}
}

func TestVieneu_ShipsAsFreeLocalNarration(t *testing.T) {
	p, err := Get(vieneuID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Committable() {
		t.Error("vieneu output is the narration a project ships")
	}
	if p.Deterministic() {
		t.Error("vieneu samples, so the same line differs run to run")
	}
	if got := p.PricePer1kChars("v3turbo"); got != 0 {
		t.Errorf("PricePer1kChars() = %v, want 0: it runs locally", got)
	}
}

// Synthesis needs the interpreter, but nothing that merely reads a project may:
// that is the contract the variable's absence has to leave intact.
func TestVieneu_SynthesisNamesTheMissingInterpreter(t *testing.T) {
	t.Setenv("NV_VIENEU_PYTHON", "")
	p, err := Get(vieneuID)
	if err != nil {
		t.Fatal(err)
	}

	err = p.Synthesize(t.Context(), Request{
		Text: "Xin chào.", Locale: "vi", SceneID: "Title", Model: "v3turbo",
		VoiceID: "Thiện Minh", OutPath: filepath.Join(t.TempDir(), "Title.mp3"),
	})
	if err == nil || !strings.Contains(err.Error(), "NV_VIENEU_PYTHON") {
		t.Errorf("Synthesize() = %v, want an error naming NV_VIENEU_PYTHON", err)
	}
}

// Runs the real model. Skipped unless NV_VIENEU_PYTHON names a Python with the
// `vieneu` package, because it downloads and loads a model on first use.
func TestVieneu_SynthesizesVietnameseIntoAMeasurableMP3(t *testing.T) {
	if os.Getenv("NV_VIENEU_PYTHON") == "" {
		t.Skip("set NV_VIENEU_PYTHON to a Python with `vieneu` installed to run the real model")
	}
	p, err := Get(vieneuID)
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "Title.mp3")
	err = p.Synthesize(t.Context(), Request{
		// The unsupported tag must be dropped, not read aloud.
		Text: "[intro] Xin chào, đây là vòng lặp tự động. [cười]", Locale: "vi", SceneID: "Title",
		Model: "v3turbo", VoiceID: "Thiện Minh", OutPath: out,
	})
	if err != nil {
		t.Fatalf("Synthesize() = %v", err)
	}

	duration, err := mp3.Duration(readFile(t, out))
	if err != nil {
		t.Fatalf("the written file is not a measurable mp3: %v", err)
	}
	if duration < time.Second || duration > 30*time.Second {
		t.Errorf("measured %v for one short sentence, want a few seconds of speech", duration)
	}
}
