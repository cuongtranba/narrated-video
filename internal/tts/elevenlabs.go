package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	elevenLabsID            = "elevenlabs"
	elevenLabsBaseURL       = "https://api.elevenlabs.io"
	elevenLabsOutputDefault = "mp3_44100_128"
)

type elevenLabs struct {
	baseURL string
	client  *http.Client
}

func newElevenLabs() *elevenLabs {
	return &elevenLabs{
		baseURL: elevenLabsBaseURL,
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *elevenLabs) ID() string { return elevenLabsID }

func (p *elevenLabs) RequiredEnv() []string { return []string{"ELEVENLABS_API_KEY"} }

// Deterministic is false: the same text resynthesized returns different bytes and
// a different length, which is why scene timing is measured per render.
func (p *elevenLabs) Deterministic() bool { return false }

func (p *elevenLabs) Committable() bool { return true }

func (p *elevenLabs) Policy() ModelPolicy {
	return ModelPolicy{
		Deny: map[string][]DeniedModel{
			"vi": {{
				Model: "eleven_multilingual_v2",
				Why: "speaks Vietnamese with wrong tones; returns 200 with a plausible duration, " +
					"so nothing but listening catches it. Use eleven_turbo_v2_5 with no voice_settings.",
			}},
		},
		Allow: map[string][]string{
			"vi": {"eleven_turbo_v2_5"},
			"en": {"eleven_multilingual_v2", "eleven_turbo_v2_5"},
		},
	}
}

// USD per 1000 characters. Unknown models bill at the highest known tier: an
// estimate that is too low is the one that surprises someone.
var elevenLabsPrices = map[string]float64{
	"eleven_turbo_v2_5":      0.15,
	"eleven_flash_v2_5":      0.15,
	"eleven_multilingual_v2": 0.30,
}

const elevenLabsFallbackPrice = 0.30

func (p *elevenLabs) PricePer1kChars(model string) float64 {
	if price, ok := elevenLabsPrices[model]; ok {
		return price
	}
	return elevenLabsFallbackPrice
}

type elevenLabsBody struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
	// Omitted, not defaulted: settings tuned for one model mistune another, and
	// the models cleared for Vietnamese were cleared with no settings at all.
	VoiceSettings map[string]float64 `json:"voice_settings,omitempty"`
}

func (p *elevenLabs) Synthesize(ctx context.Context, r Request) error {
	if err := CheckModel(p, r.Locale, r.Model); err != nil {
		return err
	}
	if r.APIKey == "" {
		return fmt.Errorf("tts: %s: no API key (set %s)", p.ID(), p.RequiredEnv()[0])
	}
	if r.VoiceID == "" {
		return fmt.Errorf("tts: %s: no voice id", p.ID())
	}
	if r.Text == "" {
		return fmt.Errorf("tts: %s: no text for scene %q", p.ID(), r.SceneID)
	}

	format := r.OutputFormat
	if format == "" {
		format = elevenLabsOutputDefault
	}

	payload, err := json.Marshal(elevenLabsBody{
		Text:          r.Text,
		ModelID:       r.Model,
		VoiceSettings: r.VoiceSettings,
	})
	if err != nil {
		return fmt.Errorf("tts: %s: encode request: %w", p.ID(), err)
	}

	endpoint := fmt.Sprintf("%s/v1/text-to-speech/%s?output_format=%s",
		p.baseURL, url.PathEscape(r.VoiceID), url.QueryEscape(format))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("tts: %s: build request: %w", p.ID(), err)
	}
	req.Header.Set("xi-api-key", r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("tts: %s: scene %q: %w", p.ID(), r.SceneID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &apiError{provider: p.ID(), status: resp.StatusCode, body: readErrorBody(resp.Body)}
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("tts: %s: read audio: %w", p.ID(), err)
	}
	return writeOutput(r.OutPath, audio)
}

// apiError carries the status AND the body: the status alone is rarely enough to
// tell a bad voice id from an exhausted quota.
type apiError struct {
	provider string
	status   int
	body     string
}

func (e *apiError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("tts: %s: HTTP %d %s", e.provider, e.status, http.StatusText(e.status))
	}
	return fmt.Sprintf("tts: %s: HTTP %d %s: %s", e.provider, e.status, http.StatusText(e.status), e.body)
}

// errorBodyLimit keeps an HTML error page out of a one-line log while leaving
// room for any JSON error a sane API returns.
const errorBodyLimit = 4 << 10

func readErrorBody(r io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(r, errorBodyLimit))
	if err != nil && len(body) == 0 {
		return ""
	}
	return string(bytes.TrimSpace(body))
}
