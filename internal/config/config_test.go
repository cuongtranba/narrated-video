package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadValid(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatalf("load valid config: %v", err)
	}
	return cfg
}

// The scene list is a union so a project whose scenes all take the defaults
// reads as a plain list of names. All three spellings must land on the same
// resolved shape.
func TestParse_SceneUnionSpellings(t *testing.T) {
	cfg := loadValid(t)

	if len(cfg.Scenes) != 3 {
		t.Fatalf("scenes = %d, want 3", len(cfg.Scenes))
	}

	bare := cfg.Scenes[0]
	if bare.ID != "Title" || !bare.Narrated {
		t.Errorf("bare scene = %+v, want narrated Title", bare)
	}
	if bare.LeadFrames != 14 || bare.TailFrames != 24 {
		t.Errorf("bare scene took %d/%d, want the defaults 14/24", bare.LeadFrames, bare.TailFrames)
	}

	override := cfg.Scenes[1]
	if override.TailFrames != 50 {
		t.Errorf("Iteration tail = %d, want its override 50", override.TailFrames)
	}
	if override.LeadFrames != 14 {
		t.Errorf("Iteration lead = %d, want the default 14", override.LeadFrames)
	}

	// Declaring a duration is what makes a scene unnarrated; the two are one
	// decision, so a config cannot half-express it.
	unnarrated := cfg.Scenes[2]
	if unnarrated.Narrated || unnarrated.DurationInFrames != 120 {
		t.Errorf("Credits = %+v, want unnarrated with duration 120", unnarrated)
	}
}

// Zero is a legal frame count, so an absent key and an explicit 0 must not
// collapse into each other.
func TestParse_ExplicitZeroSurvivesDefaulting(t *testing.T) {
	src := mutateValid(t, "    tailFrames: 50 # holds after the voice stops — the silence is the argument", "    tailFrames: 0")
	cfg, err := Parse(src, "explicit-zero.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := cfg.Scenes[1].TailFrames; got != 0 {
		t.Errorf("explicit tailFrames: 0 became %d", got)
	}
}

func TestParse_AppliesDocumentedDefaults(t *testing.T) {
	cfg, err := Parse([]byte(minimalConfig), "minimal.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, tc := range []struct {
		name string
		got  any
		want any
	}{
		{"transitionFrames", cfg.Video.TransitionFrames, 14},
		{"minSceneFrames", cfg.Video.MinSceneFrames, 60},
		{"out", cfg.Video.Out, "out"},
		{"leadFrames", cfg.Defaults.LeadFrames, 14},
		{"tailFrames", cfg.Defaults.TailFrames, 24},
		{"outputFormat", cfg.TTS.OutputFormat, "mp3_44100_128"},
		{"charsPerSecond", cfg.Locales.List[0].CharsPerSecond, 16.0},
		{"expansionFactor", cfg.Locales.List[0].ExpansionFactor, 1.3},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// Every violation is reported at once. Stopping at the first costs one
// edit-and-rerun cycle per mistake in a file people write by hand.
func TestValidateSchema_ReportsEveryViolation(t *testing.T) {
	broken := []byte(strings.ReplaceAll(minimalConfig, "fps: 30", "fps: 0") + "\nunknownTopLevel: nope\n")

	err := ValidateSchema(broken, "broken.yaml")
	if err == nil {
		t.Fatal("expected the schema to refuse fps 0 and an unknown top-level key")
	}
	for _, want := range []string{"fps", "unknownTopLevel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%s", want, err)
		}
	}
}

func TestValidateSchema_RejectsMalformedIdentifiers(t *testing.T) {
	for name, mutation := range map[string][2]string{
		"lowercase scene id": {"  - Title", "  - title"},
		"non-name api key":   {"  apiKeyEnv: ELEVENLABS_API_KEY", "  apiKeyEnv: sk_live_abc123"},
		"unknown provider":   {"  provider: silence", "  provider: mystery"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSchema(mutateValid(t, mutation[0], mutation[1]), "bad.yaml"); err == nil {
				t.Errorf("schema accepted %s", name)
			}
		})
	}
}

func TestCompositionID_DefaultLocaleKeepsBareID(t *testing.T) {
	cfg := loadValid(t)
	if got := cfg.CompositionID("en"); got != "Explainer" {
		t.Errorf("default locale composition = %q, want the bare id", got)
	}
	if got := cfg.CompositionID("vi"); got != "Explainer-vi" {
		t.Errorf("vi composition = %q", got)
	}
}

func TestFind_WalksUpToProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(minimalConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "scenes")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Find(nested)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	// t.TempDir can hand back a symlinked path on macOS; compare resolved.
	if resolve(t, got) != resolve(t, root) {
		t.Errorf("found %q, want %q", got, root)
	}

	if _, err := Find(t.TempDir()); err == nil {
		t.Error("expected an error when no config exists in any parent")
	}
}

func mutateValid(t *testing.T, old, new string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), old) {
		t.Fatalf("fixture no longer contains %q — update the test with it", old)
	}
	return []byte(strings.Replace(string(raw), old, new, 1))
}

func resolve(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

const minimalConfig = `kitVersion: 1
video:
  id: Demo
  width: 1920
  height: 1080
  fps: 30
locales:
  default: en
  list:
    - code: en
      label: English
      font: body
fonts:
  body:
    kind: google
    family: Inter
    importName: Inter
    subsets: [latin]
defaults: {}
scenes:
  - Title
tts:
  provider: silence
  costCapUsd: 1.0
  voices:
    en:
      voiceId: v1
      model: eleven_turbo_v2_5
`

// The schema and the struct are maintained by hand and could disagree; the
// fixture is where they are made to meet. A window read as 0 would silence
// CHK-26 without saying so.
func TestParse_ReadsTheDurationTarget(t *testing.T) {
	cfg := loadValid(t)
	target := cfg.Video.TargetDuration
	if !target.Declared() || target.MinSeconds != 120 || target.MaxSeconds != 180 {
		t.Fatalf("targetDuration = %+v", target)
	}
	for _, tc := range []struct {
		seconds float64
		want    bool
	}{{119, false}, {120, true}, {150, true}, {180, true}, {181, false}} {
		if got := target.Covers(tc.seconds); got != tc.want {
			t.Errorf("Covers(%v) = %v, want %v", tc.seconds, got, tc.want)
		}
	}
}

func TestTargetDuration_UndeclaredCoversEverything(t *testing.T) {
	var target TargetDuration
	if target.Declared() {
		t.Fatal("an empty target reports itself declared")
	}
	if !target.Covers(0) || !target.Covers(99999) {
		t.Fatal("an undeclared target rejected a length")
	}
}
