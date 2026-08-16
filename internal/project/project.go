// Package project is the one place that knows where a narrated-video project
// keeps things. Every other package asks it rather than joining paths, so the
// layout is a single decision rather than a convention repeated in a dozen
// call sites and eventually contradicted in one of them.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/timing"
)

type Project struct {
	Root      string
	Config    *config.Config
	Content   map[string]Content
	Manifests map[string]*timing.Manifest
	Timelines map[string]timing.LocaleTimeline
}

// Content is what a translator edits: the spoken lines, and everything the
// frame puts on screen.
type Content struct {
	Narration map[string]string `yaml:"narration" json:"narration"`
	Copy      map[string]any    `yaml:"copy" json:"copy"`
}

func (p *Project) ContentPath(locale string) string {
	return filepath.Join(p.Root, "content", locale+".yaml")
}

func (p *Project) VoiceoverDir(locale string) string {
	return filepath.Join(p.Root, "public", "voiceover", locale)
}

func (p *Project) ManifestPath(locale string) string {
	return filepath.Join(p.VoiceoverDir(locale), "manifest.json")
}

func (p *Project) AudioPath(locale, sceneID string) string {
	return filepath.Join(p.VoiceoverDir(locale), sceneID+".mp3")
}

func (p *Project) ScenePath(sceneID string) string {
	return filepath.Join(p.Root, "src", "scenes", sceneID+".tsx")
}

func (p *Project) GeneratedPath(name string) string {
	return filepath.Join(p.Root, "src", "generated", name)
}

func (p *Project) PublicPath(rel string) string {
	return filepath.Join(p.Root, "public", rel)
}

// Load reads everything on disk and derives the timelines. Missing content or
// a missing manifest is not an error — a project mid-translation, or one whose
// audio has not been made yet, still has to open in Studio and render. The
// checks report those states; loading does not refuse them.
func Load(root string) (*Project, error) {
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		return nil, err
	}

	p := &Project{
		Root:      root,
		Config:    cfg,
		Content:   map[string]Content{},
		Manifests: map[string]*timing.Manifest{},
		Timelines: map[string]timing.LocaleTimeline{},
	}

	for _, locale := range cfg.Locales.List {
		content, err := readContent(p.ContentPath(locale.Code))
		if err != nil {
			return nil, err
		}
		p.Content[locale.Code] = content

		manifest, err := readManifest(p.ManifestPath(locale.Code))
		if err != nil {
			return nil, err
		}
		p.Manifests[locale.Code] = manifest

		p.Timelines[locale.Code] = timing.Derive(timing.Input{
			Locale:           locale.Code,
			FPS:              cfg.Video.FPS,
			TransitionFrames: cfg.Video.TransitionFrames,
			Scenes:           cfg.Scenes,
			Manifest:         manifest,
			Narration:        content.Narration,
			CharsPerSecond:   locale.CharsPerSecond,
			AudioPresent:     p.audioPresence(locale.Code),
		})
	}
	return p, nil
}

func (p *Project) audioPresence(locale string) func(string) bool {
	return func(sceneID string) bool {
		info, err := os.Stat(p.AudioPath(locale, sceneID))
		return err == nil && info.Size() > 0
	}
}

func readContent(path string) (Content, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Content{Narration: map[string]string{}, Copy: map[string]any{}}, nil
	}
	if err != nil {
		return Content{}, err
	}

	var c Content
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return Content{}, fmt.Errorf("%s: %w", path, err)
	}
	if c.Narration == nil {
		c.Narration = map[string]string{}
	}
	if c.Copy == nil {
		c.Copy = map[string]any{}
	}
	return c, nil
}

func readManifest(path string) (*timing.Manifest, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var m timing.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m.Scenes == nil {
		m.Scenes = map[string]timing.ManifestEntry{}
	}
	return &m, nil
}

// NarratedScenes is the set a line is expected for. An unnarrated scene has no
// audio, no hash and no cost, and every check that walks narration skips it.
func (p *Project) NarratedScenes() []string {
	var ids []string
	for _, s := range p.Config.Scenes {
		if s.Narrated {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// SortedKeys keeps generated output byte-stable. Authoring order is not
// preserved anywhere, deliberately: the generated files are compared against a
// fresh derivation, and a comparison that depended on map iteration would fail
// at random.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
