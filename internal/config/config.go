// Package config is the single source of truth for a narrated-video project.
//
// Every value the tool acts on arrives through Load, and Load refuses anything
// the published JSON Schema rejects. Nothing downstream re-validates or invents
// a default: a field's meaning is fixed here, once.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"github.com/cuongtranba/narrated-video/internal/schema"
)

const FileName = "video.config.yaml"

type Config struct {
	KitVersion      int                `yaml:"kitVersion"`
	Video           Video              `yaml:"video"`
	Locales         Locales            `yaml:"locales"`
	Fonts           map[string]Font    `yaml:"fonts"`
	Theme           map[string]string  `yaml:"theme"`
	Defaults        Defaults           `yaml:"defaults"`
	Scenes          []Scene            `yaml:"scenes"`
	TTS             TTS                `yaml:"tts"`
	Glossary        []string           `yaml:"glossary"`
	PrintedLiterals []string           `yaml:"printedLiterals"`
	Diagrams        map[string]Diagram `yaml:"diagrams"`
}

type Diagram struct {
	Nodes []DiagramNode `yaml:"nodes"`
	Edges []DiagramEdge `yaml:"edges"`
}

type DiagramNode struct {
	ID   string     `yaml:"id"`
	At   [2]float64 `yaml:"at"`
	Size [2]float64 `yaml:"size"`
}

type DiagramEdge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type Video struct {
	ID               string         `yaml:"id"`
	Width            int            `yaml:"width"`
	Height           int            `yaml:"height"`
	FPS              int            `yaml:"fps"`
	TransitionFrames int            `yaml:"transitionFrames"`
	MinSceneFrames   int            `yaml:"minSceneFrames"`
	Out              string         `yaml:"out"`
	TargetDuration   TargetDuration `yaml:"targetDuration"`
}

// TargetDuration is the length the video was commissioned at. Either bound may
// stand alone, and zero means unbounded — a video with no declared target is a
// normal video, not a misconfigured one.
type TargetDuration struct {
	MinSeconds int `yaml:"minSeconds"`
	MaxSeconds int `yaml:"maxSeconds"`
}

func (t TargetDuration) Declared() bool { return t.MinSeconds > 0 || t.MaxSeconds > 0 }

func (t TargetDuration) Covers(seconds float64) bool {
	if t.MinSeconds > 0 && seconds < float64(t.MinSeconds) {
		return false
	}
	return t.MaxSeconds <= 0 || seconds <= float64(t.MaxSeconds)
}

type Locales struct {
	Default string   `yaml:"default"`
	List    []Locale `yaml:"list"`
}

type Locale struct {
	Code  string `yaml:"code"`
	Label string `yaml:"label"`
	Font  string `yaml:"font"`
	// Used only to estimate scene length before audio exists. Measured against
	// the reference cut: en 16.17, vi 15.55.
	CharsPerSecond float64 `yaml:"charsPerSecond"`
	// Budget multiplier over the default locale's string length. A proxy for
	// visual overflow, not a proof of it.
	ExpansionFactor float64 `yaml:"expansionFactor"`
	// Every codepoint here must exist in this locale's font. Include the
	// script's hardest stacked marks — a face can cover a subset's range and
	// still be missing its most common composed vowels.
	RequiredSample string `yaml:"requiredSample"`
}

type FontKind string

const (
	FontGoogle FontKind = "google"
	FontLocal  FontKind = "local"
)

type Font struct {
	Kind       FontKind   `yaml:"kind"`
	Family     string     `yaml:"family"`
	ImportName string     `yaml:"importName"`
	Subsets    []string   `yaml:"subsets"`
	Weights    []string   `yaml:"weights"`
	Files      []FontFile `yaml:"files"`
}

type FontFile struct {
	Path   string `yaml:"path"`
	Weight string `yaml:"weight"`
	Style  string `yaml:"style"`
}

type Defaults struct {
	LeadFrames int `yaml:"leadFrames"`
	TailFrames int `yaml:"tailFrames"`
}

// Scene is written in YAML either as a bare id or as a table of overrides, so a
// project whose scenes all take the defaults reads as a plain list.
type Scene struct {
	ID         string
	LeadFrames int
	TailFrames int
	// Set only for a scene with no narration line. A narrated scene declaring a
	// duration is refused: that is precisely how a hand-maintained frame table
	// grows back after the derivation removed it.
	DurationInFrames int
	Narrated         bool
}

type TTS struct {
	Provider     string           `yaml:"provider"`
	APIKeyEnv    string           `yaml:"apiKeyEnv"`
	CostCapUSD   float64          `yaml:"costCapUsd"`
	OutputFormat string           `yaml:"outputFormat"`
	Voices       map[string]Voice `yaml:"voices"`
}

type Voice struct {
	VoiceID string `yaml:"voiceId"`
	Model   string `yaml:"model"`
	// Absent means: send no voice_settings field at all. Settings tuned against
	// one model are not tuning for another, and a model that wants none will
	// quietly degrade if handed a neighbour's.
	VoiceSettings map[string]float64 `yaml:"voiceSettings"`
}

// rawScene mirrors the YAML union. The override fields are pointers so an
// explicit 0 is distinguishable from an absent key — both are meaningful for
// frame counts.
type rawScene struct {
	ID               string `yaml:"id"`
	LeadFrames       *int   `yaml:"leadFrames"`
	TailFrames       *int   `yaml:"tailFrames"`
	DurationInFrames *int   `yaml:"durationInFrames"`
	Narrated         *bool  `yaml:"narrated"`
}

// Load reads, schema-validates and resolves a project config. Defaults are
// applied here and nowhere else, so a caller reading Config sees final values.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

func Parse(data []byte, name string) (*Config, error) {
	if err := ValidateSchema(data, name); err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

// UnmarshalYAML accepts either spelling of a scene: a bare id, or a table of
// overrides. The sentinel marks "not stated" so applyDefaults can tell it from
// an explicit 0 — zero is a legal frame count.
func (s *Scene) UnmarshalYAML(node *yaml.Node) error {
	*s = Scene{Narrated: true, LeadFrames: unset, TailFrames: unset}

	if node.Kind == yaml.ScalarNode {
		return node.Decode(&s.ID)
	}

	var raw rawScene
	if err := node.Decode(&raw); err != nil {
		return err
	}
	s.ID = raw.ID
	if raw.LeadFrames != nil {
		s.LeadFrames = *raw.LeadFrames
	}
	if raw.TailFrames != nil {
		s.TailFrames = *raw.TailFrames
	}
	// Deliberately NOT inferred from each other. Letting a declared duration
	// imply `narrated: false` would silence a scene without saying so — the
	// exact class of quiet failure this tool exists to make loud. Stating both
	// is two words; a scene that lost its voice is a re-render to notice.
	if raw.DurationInFrames != nil {
		s.DurationInFrames = *raw.DurationInFrames
	}
	if raw.Narrated != nil {
		s.Narrated = *raw.Narrated
	}
	return nil
}

const unset = -1

// The schema carries these same defaults for editor tooling; they are repeated
// here because a JSON Schema `default` annotates, it does not fill.
func (c *Config) applyDefaults() {
	setIfZero(&c.Video.TransitionFrames, 14)
	setIfZero(&c.Video.MinSceneFrames, 60)
	if c.Video.Out == "" {
		c.Video.Out = "out"
	}
	// The reference cut's scenes carried a ~37-frame pad around the voice.
	// Splitting it 14/24 puts the lead over the cross-fade, so no line starts
	// while the previous scene is still on screen.
	setIfZero(&c.Defaults.LeadFrames, 14)
	setIfZero(&c.Defaults.TailFrames, 24)
	if c.TTS.OutputFormat == "" {
		c.TTS.OutputFormat = "mp3_44100_128"
	}

	for i := range c.Locales.List {
		l := &c.Locales.List[i]
		if l.CharsPerSecond == 0 {
			l.CharsPerSecond = 16
		}
		if l.ExpansionFactor == 0 {
			l.ExpansionFactor = 1.3
		}
	}
	for i := range c.Scenes {
		s := &c.Scenes[i]
		if s.LeadFrames == unset {
			s.LeadFrames = c.Defaults.LeadFrames
		}
		if s.TailFrames == unset {
			s.TailFrames = c.Defaults.TailFrames
		}
	}
}

func setIfZero(field *int, fallback int) {
	if *field == 0 {
		*field = fallback
	}
}

// ValidateSchema reports every violation at once. Stopping at the first would
// cost one edit-and-rerun cycle per mistake in a file people edit by hand.
func ValidateSchema(yamlData []byte, name string) error {
	var doc any
	if err := yaml.Unmarshal(yamlData, &doc); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	// The validator speaks JSON types; YAML's own decoding produces some that
	// look equivalent but compare differently.
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	schema, err := compiledSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func compiledSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.File))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schema.FileName, doc); err != nil {
		return nil, err
	}
	return c.Compile(schema.FileName)
}

// LocaleByCode and the lookups below exist so callers never re-scan the slices
// and disagree about what "the default locale" means.
func (c *Config) LocaleByCode(code string) (Locale, bool) {
	for _, l := range c.Locales.List {
		if l.Code == code {
			return l, true
		}
	}
	return Locale{}, false
}

func (c *Config) DefaultLocale() Locale {
	l, _ := c.LocaleByCode(c.Locales.Default)
	return l
}

func (c *Config) LocaleCodes() []string {
	codes := make([]string, 0, len(c.Locales.List))
	for _, l := range c.Locales.List {
		codes = append(codes, l.Code)
	}
	return codes
}

func (c *Config) SceneIDs() []string {
	ids := make([]string, 0, len(c.Scenes))
	for _, s := range c.Scenes {
		ids = append(ids, s.ID)
	}
	return ids
}

// CompositionID keeps the default locale on the bare id so the composition
// anyone renders by habit stays where it was.
func (c *Config) CompositionID(locale string) string {
	if locale == c.Locales.Default {
		return c.Video.ID
	}
	return c.Video.ID + "-" + locale
}

// Find locates the project root by walking up from dir, so the tool works from
// anywhere inside a project the way git does.
func Find(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, FileName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no %s found in %s or any parent directory", FileName, dir)
		}
		abs = parent
	}
}
