package tts

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	silenceID = "silence"

	// One MPEG-1 Layer III frame carries 1152 samples; at 44100 Hz that is
	// 38.28125 frames per second, and at 128 kbps with the padding bit set each
	// frame is exactly 418 bytes.
	silenceSamplesPerFrame = 1152
	silenceSampleRate      = 44100
	silenceFrameSize       = 418

	// Fallback pacing when a request does not set one: roughly conversational.
	silenceDefaultCharsPerSecond = 15.0
)

// silentFrame is the whole provider. Header bytes, MSB first:
//
//	0xFF 0xFB -> sync, MPEG-1, Layer III, no CRC
//	0x92      -> bitrate index 9 (128 kbps), sample rate index 0 (44100), padded
//	0xC0      -> single channel
//
// The remaining 414 bytes stay zero: zeroed side info declares no Huffman data,
// which every decoder renders as silence. Written as a literal so the bytes
// cannot drift with a helper's behaviour — output must be byte-identical across
// runs and machines for CI to diff it.
var silentFrame = [silenceFrameSize]byte{0xFF, 0xFB, 0x92, 0xC0}

// silence is the CI / no-API-key default: real, measurable MP3 of the right
// length, with no network and no credentials.
type silence struct{}

func (silence) ID() string { return silenceID }

func (silence) RequiredEnv() []string { return nil }

func (silence) Deterministic() bool { return true }

// Committable is true: silence is a legitimate placeholder narration track, and
// its length is a pure function of the script rather than of the machine.
func (silence) Committable() bool { return true }

// Policy clears everything: this provider's output does not vary by model, so
// there is nothing for an ear to judge.
func (silence) Policy() ModelPolicy {
	return ModelPolicy{Allow: map[string][]string{anyKey: {anyKey}}}
}

func (silence) PricePer1kChars(string) float64 { return 0 }

func (s silence) Synthesize(_ context.Context, r Request) error {
	if err := CheckModel(s, r.Locale, r.Model); err != nil {
		return err
	}

	charsPerSecond := r.CharsPerSecond
	if charsPerSecond <= 0 {
		charsPerSecond = silenceDefaultCharsPerSecond
	}

	// Runes, not bytes: a Vietnamese line is spoken in characters, and byte length
	// would stretch it by half again for the same script.
	chars := utf8.RuneCountInString(r.Text)
	seconds := float64(chars) / charsPerSecond

	frames := int(math.Round(seconds * silenceSampleRate / silenceSamplesPerFrame))
	if frames < 1 {
		// An empty scene still has to produce a measurable file rather than one
		// with no frames at all.
		frames = 1
	}

	var buf bytes.Buffer
	buf.Grow(frames * silenceFrameSize)
	for range frames {
		buf.Write(silentFrame[:])
	}

	if err := writeOutput(r.OutPath, buf.Bytes()); err != nil {
		return fmt.Errorf("tts: %s: scene %q: %w", s.ID(), r.SceneID, err)
	}
	return nil
}
