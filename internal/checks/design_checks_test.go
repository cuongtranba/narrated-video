package checks

import (
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/kit"
)

func sceneKit(source string) *Kit {
	return &Kit{SceneSources: map[string]string{"Demo": source}}
}

func TestScenesUseThemeColours(t *testing.T) {
	cases := []struct {
		name   string
		source string
		wantOK bool
	}{
		{"theme reference is the sanctioned path",
			`<p style={{ color: THEME.muted }}>x</p>`, true},
		{"THEME_3D on a material is equally fine",
			`<meshStandardMaterial color={THEME_3D.accent} />`, true},
		{"a hex literal is a second palette",
			`<p style={{ color: "#ff0055" }}>x</p>`, false},
		{"short hex too",
			`<p style={{ color: "#f05" }}>x</p>`, false},
		{"rgba is a colour by another name",
			`<div style={{ background: "rgba(0,0,0,0.4)" }} />`, false},
		{"oklch written by hand bypasses the theme just the same",
			`<div style={{ color: "oklch(70% 0.1 20)" }} />`, false},
		{"a hex inside a comment is not a rendered colour",
			`// the old build used #ff0055 here`, true},
		{"a hex inside a block comment is not either",
			`const x = 1 /* was #abcdef */`, true},
		{"an SVG path is not a colour",
			`<path d="M0 0 L10 10" fill="none" />`, true},
		{"color-mix over theme tokens still sources from the theme",
			"border: `2px solid color-mix(in oklch, ${THEME.border}, ${THEME.muted} 45%)`", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scenesUseThemeColours(sceneKit(c.source))
			if got.OK != c.wantOK {
				t.Errorf("OK = %v, want %v (findings: %v)", got.OK, c.wantOK, got.Findings)
			}
		})
	}
}

func TestScenesUseTypeScale(t *testing.T) {
	cases := []struct {
		name   string
		source string
		wantOK bool
	}{
		{"a step from the scale", `style={{ fontSize: SIZE.lead }}`, true},
		{"arithmetic on a step still references the scale",
			`style={{ fontSize: SIZE.body * 0.9 }}`, true},
		{"a bare number is a seventh size", `style={{ fontSize: 46 }}`, false},
		{"as a JSX prop too", `<Count fontSize={72} />`, false},
		{"a decimal is no better", `style={{ fontSize: 33.5 }}`, false},
		{"a commented-out size is not rendered", `// fontSize: 46`, true},
		{"other numeric styles are none of this check's business",
			`style={{ gap: 34, maxWidth: 1400, lineHeight: 1.4 }}`, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scenesUseTypeScale(sceneKit(c.source))
			if got.OK != c.wantOK {
				t.Errorf("OK = %v, want %v (findings: %v)", got.OK, c.wantOK, got.Findings)
			}
		})
	}
}

// The templates a scaffold starts from must clear the gate they ship with —
// read from the embedded FS, so this is the bytes `nv init` actually writes and
// not a copy that can drift from them.
func TestShippedScenesClearTheDesignChecks(t *testing.T) {
	entries, err := kit.FS.ReadDir("src/scenes")
	if err != nil {
		t.Fatalf("read embedded scenes: %v", err)
	}

	sources := map[string]string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tsx") {
			continue
		}
		body, err := kit.FS.ReadFile("src/scenes/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		sources[strings.TrimSuffix(entry.Name(), ".tsx")] = string(body)
	}
	if len(sources) == 0 {
		t.Fatal("no embedded scenes found — the fixture is not reading the kit")
	}

	shipped := &Kit{SceneSources: sources}
	for _, check := range []Check{scenesUseThemeColours, scenesUseTypeScale} {
		if got := check(shipped); !got.OK {
			t.Errorf("%s failed on the shipped kit: %v", got.ID, got.Findings)
		}
	}
}
