package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cuongtranba/narrated-video/internal/mp3"
)

// oneFrame is the resolution of the silence provider: 1152 samples at 44.1 kHz.
const oneFrame = time.Duration(silenceSamplesPerFrame) * time.Second / silenceSampleRate

func TestSilence_DeterministicAndMeasurable(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		charsPerSecond float64
	}{
		{name: "short english", text: "Hello there.", charsPerSecond: 15},
		{
			name:           "paragraph",
			text:           strings.Repeat("The quick brown fox jumps over the lazy dog. ", 12),
			charsPerSecond: 15,
		},
		{
			name:           "vietnamese counts runes not bytes",
			text:           "Xin chào, đây là một câu tiếng Việt có dấu đầy đủ.",
			charsPerSecond: 14,
		},
		{name: "slow pacing", text: strings.Repeat("a", 300), charsPerSecond: 5},
		{name: "fast pacing", text: strings.Repeat("a", 300), charsPerSecond: 40},
		{name: "single character", text: "a", charsPerSecond: 15},
		{name: "long", text: strings.Repeat("narration ", 500), charsPerSecond: 16},
	}

	provider, err := Get(silenceID)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", silenceID, err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			first := filepath.Join(dir, "nested", "scene-01.mp3")
			second := filepath.Join(dir, "other", "scene-01.mp3")

			req := Request{
				Text:           tt.text,
				Locale:         "vi",
				SceneID:        "scene-01",
				CharsPerSecond: tt.charsPerSecond,
			}

			req.OutPath = first
			if err := provider.Synthesize(t.Context(), req); err != nil {
				t.Fatalf("Synthesize() error = %v", err)
			}
			req.OutPath = second
			if err := provider.Synthesize(t.Context(), req); err != nil {
				t.Fatalf("Synthesize() second run error = %v", err)
			}

			a := readFile(t, first)
			b := readFile(t, second)
			if !bytes.Equal(a, b) {
				t.Fatalf("output is not byte-identical across runs: %d bytes vs %d bytes", len(a), len(b))
			}

			// A whole number of fixed-size frames, each carrying the documented
			// header — anything else means the constant drifted.
			if len(a)%silenceFrameSize != 0 {
				t.Errorf("output is %d bytes, not a multiple of the %d-byte frame", len(a), silenceFrameSize)
			}
			for off := 0; off+4 <= len(a); off += silenceFrameSize {
				if got := a[off : off+4]; !bytes.Equal(got, []byte{0xFF, 0xFB, 0x92, 0xC0}) {
					t.Fatalf("frame at byte %d has header %x, want ff fb 92 c0", off, got)
				}
			}

			got, err := mp3.Duration(a)
			if err != nil {
				t.Fatalf("mp3.Duration() error = %v", err)
			}
			want := time.Duration(float64(utf8.RuneCountInString(tt.text)) / tt.charsPerSecond * float64(time.Second))
			diff := absDuration(got - want)
			if diff > oneFrame {
				t.Errorf("measured %v, want %v (off by %v, tolerance one frame %v)", got, want, diff, oneFrame)
			}
			t.Logf("%d runes @ %.0f chars/s: want %v, measured %v, error %v (%.1f%% of a frame)",
				utf8.RuneCountInString(tt.text), tt.charsPerSecond, want, got, diff,
				100*float64(diff)/float64(oneFrame))
		})
	}
}

func TestSilence_EmptyTextStillProducesMeasurableAudio(t *testing.T) {
	out := filepath.Join(t.TempDir(), "empty.mp3")
	if err := (silence{}).Synthesize(t.Context(), Request{OutPath: out, CharsPerSecond: 15}); err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	got, err := mp3.Duration(readFile(t, out))
	if err != nil {
		t.Fatalf("mp3.Duration() error = %v", err)
	}
	if got != oneFrame {
		t.Errorf("Duration() = %v, want exactly one frame %v", got, oneFrame)
	}
}

func TestSilence_UnsetCharsPerSecondUsesTheDefault(t *testing.T) {
	dir := t.TempDir()
	text := strings.Repeat("a", 150)

	unset := filepath.Join(dir, "unset.mp3")
	explicit := filepath.Join(dir, "explicit.mp3")

	if err := (silence{}).Synthesize(t.Context(), Request{Text: text, OutPath: unset}); err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if err := (silence{}).Synthesize(t.Context(), Request{
		Text: text, OutPath: explicit, CharsPerSecond: silenceDefaultCharsPerSecond,
	}); err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if !bytes.Equal(readFile(t, unset), readFile(t, explicit)) {
		t.Error("unset CharsPerSecond did not fall back to the documented default")
	}
}

func TestSilence_IsDeterministicAndCommittable(t *testing.T) {
	p := silence{}
	if !p.Deterministic() {
		t.Error("silence must be deterministic; it is what makes CI diffable")
	}
	if !p.Committable() {
		t.Error("silence output is a legitimate placeholder narration")
	}
	if got := p.PricePer1kChars("anything"); got != 0 {
		t.Errorf("PricePer1kChars() = %v, want 0", got)
	}
	if got := p.RequiredEnv(); len(got) != 0 {
		t.Errorf("RequiredEnv() = %v, want none — silence is the no-API-key default", got)
	}
}

func TestCheckModel_RejectsMultilingualForVietnamese(t *testing.T) {
	p, err := Get(elevenLabsID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	err = CheckModel(p, "vi", "eleven_multilingual_v2")
	if err == nil {
		t.Fatal("CheckModel() = nil, want a denial")
	}
	if !errors.Is(err, ErrModelDenied) {
		t.Errorf("CheckModel() error = %v, want ErrModelDenied", err)
	}

	// The reason is the whole point: a status code cannot catch this failure, so
	// the error has to say what listening revealed and what to use instead.
	for _, want := range []string{"wrong tones", "eleven_turbo_v2_5", "eleven_multilingual_v2", "vi"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestCheckModel_RejectsUnlistedModel(t *testing.T) {
	p, err := Get(elevenLabsID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	tests := []struct {
		name          string
		locale, model string
	}{
		{name: "unknown model for vi", locale: "vi", model: "eleven_v3_alpha"},
		{name: "empty model for vi", locale: "vi", model: ""},
		{name: "model cleared for another locale only", locale: "vi", model: "eleven_multilingual_v1"},
		{name: "locale nobody has judged", locale: "ja", model: "eleven_turbo_v2_5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckModel(p, tt.locale, tt.model)
			if err == nil {
				t.Fatalf("CheckModel(%q, %q) = nil, want an error", tt.locale, tt.model)
			}
			if !errors.Is(err, ErrModelUnlisted) {
				t.Errorf("error = %v, want ErrModelUnlisted", err)
			}
		})
	}
}

func TestCheckModel_AcceptsClearedModels(t *testing.T) {
	p, err := Get(elevenLabsID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	cleared := []struct{ locale, model string }{
		{"vi", "eleven_turbo_v2_5"},
		{"en", "eleven_turbo_v2_5"},
		{"en", "eleven_multilingual_v2"},
	}
	for _, c := range cleared {
		if err := CheckModel(p, c.locale, c.model); err != nil {
			t.Errorf("CheckModel(%q, %q) = %v, want nil", c.locale, c.model, err)
		}
	}

	// Providers whose output does not vary by model clear everything.
	for _, id := range []string{silenceID, sayID} {
		open, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", id, err)
		}
		if err := CheckModel(open, "vi", "whatever"); err != nil {
			t.Errorf("CheckModel(%s) = %v, want nil", id, err)
		}
	}
}

func TestElevenLabs_OmitsVoiceSettingsWhenNil(t *testing.T) {
	var body map[string]any
	var apiKey string

	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("xi-api-key")
		decodeJSON(t, r, &body)
		w.Write(silentFrame[:])
	})
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "vi.mp3")
	err := p.Synthesize(t.Context(), Request{
		Text:    "Xin chào",
		Locale:  "vi",
		Model:   "eleven_turbo_v2_5",
		VoiceID: "voice-123",
		OutPath: out,
		APIKey:  "secret-key",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if apiKey != "secret-key" {
		t.Errorf("xi-api-key header = %q, want the request's key", apiKey)
	}
	if _, present := body["voice_settings"]; present {
		t.Errorf("voice_settings present with nil settings: %v — the field must be absent, not defaulted", body)
	}
	want := map[string]any{"text": "Xin chào", "model_id": "eleven_turbo_v2_5"}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("request body = %v, want %v", body, want)
	}
	if _, err := mp3.Duration(readFile(t, out)); err != nil {
		t.Errorf("written file is not measurable audio: %v", err)
	}
}

func TestElevenLabs_SendsThemWhenSet(t *testing.T) {
	var body map[string]any
	var gotPath, gotQuery, gotMethod string

	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		decodeJSON(t, r, &body)
		w.Write(silentFrame[:])
	})
	defer srv.Close()

	err := p.Synthesize(t.Context(), Request{
		Text:          "Hello",
		Locale:        "en",
		Model:         "eleven_multilingual_v2",
		VoiceID:       "voice-123",
		OutputFormat:  "mp3_22050_32",
		VoiceSettings: map[string]float64{"stability": 0.5, "similarity_boost": 0.75},
		OutPath:       filepath.Join(t.TempDir(), "en.mp3"),
		APIKey:        "secret-key",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if want := "/v1/text-to-speech/voice-123"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if want := "output_format=mp3_22050_32"; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}

	want := map[string]any{
		"text":     "Hello",
		"model_id": "eleven_multilingual_v2",
		"voice_settings": map[string]any{
			"stability":        0.5,
			"similarity_boost": 0.75,
		},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("request body = %v, want %v", body, want)
	}
}

func TestElevenLabs_DefaultsOutputFormat(t *testing.T) {
	var gotQuery string
	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write(silentFrame[:])
	})
	defer srv.Close()

	err := p.Synthesize(t.Context(), Request{
		Text: "Hello", Locale: "en", Model: "eleven_turbo_v2_5",
		VoiceID: "v", OutPath: filepath.Join(t.TempDir(), "a.mp3"), APIKey: "k",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if want := "output_format=" + elevenLabsOutputDefault; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}
}

func TestElevenLabs_ErrorCarriesStatusAndBody(t *testing.T) {
	const detail = `{"detail":{"status":"voice_not_found","message":"A voice with voice_id v was not found."}}`

	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(detail))
	})
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "fail.mp3")
	err := p.Synthesize(t.Context(), Request{
		Text: "Hello", Locale: "en", Model: "eleven_turbo_v2_5",
		VoiceID: "v", OutPath: out, APIKey: "secret-key",
	})
	if err == nil {
		t.Fatal("Synthesize() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Errorf("error %q does not carry the status", err)
	}
	if !strings.Contains(err.Error(), "voice_not_found") {
		t.Errorf("error %q does not carry the response body", err)
	}
	if strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("error leaked the API key: %v", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Error("a failed synthesis left a file behind for the renderer to measure")
	}
}

func TestElevenLabs_RefusesDeniedModelBeforeCallingTheAPI(t *testing.T) {
	called := false
	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write(silentFrame[:])
	})
	defer srv.Close()

	err := p.Synthesize(t.Context(), Request{
		Text: "Xin chào", Locale: "vi", Model: "eleven_multilingual_v2",
		VoiceID: "v", OutPath: filepath.Join(t.TempDir(), "vi.mp3"), APIKey: "k",
	})
	if !errors.Is(err, ErrModelDenied) {
		t.Fatalf("Synthesize() error = %v, want ErrModelDenied", err)
	}
	if called {
		t.Error("the API was called with a model that must not be used")
	}
}

func TestElevenLabs_RequiresCredentialsAndVoice(t *testing.T) {
	p, srv := elevenLabsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the API must not be called without credentials")
	})
	defer srv.Close()

	base := Request{
		Text: "Hello", Locale: "en", Model: "eleven_turbo_v2_5",
		VoiceID: "v", OutPath: filepath.Join(t.TempDir(), "a.mp3"), APIKey: "k",
	}

	missingKey := base
	missingKey.APIKey = ""
	if err := p.Synthesize(t.Context(), missingKey); err == nil {
		t.Error("Synthesize() without an API key = nil, want an error")
	}

	missingVoice := base
	missingVoice.VoiceID = ""
	if err := p.Synthesize(t.Context(), missingVoice); err == nil {
		t.Error("Synthesize() without a voice id = nil, want an error")
	}
}

func TestElevenLabs_Metadata(t *testing.T) {
	p := newElevenLabs()

	if p.Deterministic() {
		t.Error("elevenlabs output varies between calls; claiming otherwise would let a stale render be reused")
	}
	if !p.Committable() {
		t.Error("elevenlabs is the provider whose output ships")
	}
	if want := []string{"ELEVENLABS_API_KEY"}; !reflect.DeepEqual(p.RequiredEnv(), want) {
		t.Errorf("RequiredEnv() = %v, want %v", p.RequiredEnv(), want)
	}
	if got := p.PricePer1kChars("eleven_turbo_v2_5"); got != 0.15 {
		t.Errorf("PricePer1kChars(turbo) = %v, want 0.15", got)
	}
	if got := p.PricePer1kChars("some_future_model"); got != elevenLabsFallbackPrice {
		t.Errorf("PricePer1kChars(unknown) = %v, want the documented fallback %v", got, elevenLabsFallbackPrice)
	}
}

func TestSay_IsNotCommittable(t *testing.T) {
	p, err := Get(sayID)
	if err != nil {
		t.Fatalf("Get(%q) error = %v", sayID, err)
	}
	if p.Committable() {
		t.Error("`say` length varies with OS version, installed voice and speech rate; its output must never ship")
	}
	if p.Deterministic() {
		t.Error("`say` is not reproducible across machines")
	}
}

func TestRegistry(t *testing.T) {
	got := make([]string, 0, len(All()))
	for _, p := range All() {
		got = append(got, p.ID())
	}
	want := []string{elevenLabsID, sayID, silenceID}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("All() = %v, want %v (sorted by id)", got, want)
	}

	for _, id := range want {
		p, err := Get(id)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", id, err)
		}
		if p.ID() != id {
			t.Errorf("Get(%q).ID() = %q", id, p.ID())
		}
	}

	if _, err := Get("nope"); err == nil {
		t.Error("Get(unknown) = nil error, want one naming the known providers")
	} else if !strings.Contains(err.Error(), silenceID) {
		t.Errorf("error %q does not list the known providers", err)
	}
}

// TestSynthesizeReturnsNoDuration pins the contract that makes measurement
// uniform: a provider reports success or failure and nothing about length.
func TestSynthesizeReturnsNoDuration(t *testing.T) {
	// Compile-time: the interface method has exactly this shape.
	var _ = func(p Provider) func(context.Context, Request) error { return p.Synthesize }

	errType := reflect.TypeOf((*error)(nil)).Elem()
	for _, p := range All() {
		m, ok := reflect.TypeOf(p).MethodByName("Synthesize")
		if !ok {
			t.Fatalf("%s has no Synthesize method", p.ID())
		}
		if got := m.Type.NumOut(); got != 1 {
			t.Errorf("%s.Synthesize returns %d values, want 1 (error only)", p.ID(), got)
		}
		if out := m.Type.Out(0); out != errType {
			t.Errorf("%s.Synthesize returns %v, want error", p.ID(), out)
		}
	}
}

func elevenLabsTestServer(t *testing.T, h http.HandlerFunc) (*elevenLabs, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	return &elevenLabs{baseURL: srv.URL, client: srv.Client()}, srv
}

func decodeJSON(t *testing.T, r *http.Request, into any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		t.Errorf("decode request body: %v", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
