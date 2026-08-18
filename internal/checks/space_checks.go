package checks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
)

var glRendererRe = regexp.MustCompile(`setChromiumOpenGlRenderer\s*\(\s*["']angle["']`)

// CHK-33. Headless Chromium does not enable a WebGL backend by default. A
// space scene rendered without one produces a black rectangle at the correct
// size and length with exit code 0 — nothing in the output signals the failure.
func glRendererConfigured(kit *Kit) Result {
	const id, title = "CHK-33", "space scenes have the GL renderer configured"

	if !HasSpaceScene(kit.SceneSources) {
		return pass(id, title)
	}

	var findings []Finding

	configPath := filepath.Join(kit.Root, "remotion.config.ts")
	configContent, err := os.ReadFile(configPath)
	if err != nil || !glRendererRe.Match(configContent) {
		findings = append(findings, Finding{
			Where:  "remotion.config.ts",
			Detail: `missing Config.setChromiumOpenGlRenderer("angle")`,
		})
	}

	file, err := pkgscripts.Load(kit.Root)
	if err == nil && file != nil {
		scripts := file.Scripts()
		for _, name := range pkgscripts.Managed {
			script, ok := scripts[name]
			if !ok {
				continue
			}
			if !strings.Contains(script, "--gl=angle") {
				findings = append(findings, Finding{
					Where:  pkgscripts.FileName + " scripts." + name,
					Detail: "missing --gl=angle",
				})
			}
		}
	}

	return fail(id, title,
		`add --gl=angle to every render/still script in package.json and Config.setChromiumOpenGlRenderer("angle") to remotion.config.ts — without it a 3D scene renders as a blank rectangle and the render still exits 0`,
		sortedFindings(findings))
}

func HasSpaceScene(sources map[string]string) bool {
	for _, source := range sources {
		if importsPath(source, "../components/space") {
			return true
		}
	}
	return false
}

// CHK-37. A scene that imports @react-three/fiber, three, or @remotion/three
// directly bypasses the Space wrapper, which is where the frameloop-disable and
// camera/lighting defaults live. A bypass is silent: the frame renders, the check
// exits 0, the video differs between two headless processes.
func scenesUseSpaceWrapper(kit *Kit) Result {
	const id, title = "CHK-37", "scene modules import Three.js only through the Space wrapper"

	forbidden := []string{"@react-three/fiber", "three", "@remotion/three"}

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for _, pkg := range forbidden {
			if importsPackageExact(kit.SceneSources[sceneID], pkg) {
				findings = append(findings, Finding{
					Where:  sceneID,
					Detail: "imports " + pkg + " directly",
				})
				break
			}
		}
	}
	return fail(id, title,
		"use <Space> from kit components instead — direct three/r3f imports bypass the wrapper's determinism guards",
		sortedFindings(findings))
}

// importsPackageExact matches the whole module specifier so that "three" does
// not match inside "@remotion/three", and a deep import like "three/examples/…"
// still counts as three.
func importsPackageExact(source, pkg string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import") && !strings.HasPrefix(trimmed, "export") {
			continue
		}
		spec, ok := moduleSpecifierFromLine(trimmed)
		if !ok {
			continue
		}
		if spec == pkg || strings.HasPrefix(spec, pkg+"/") {
			return true
		}
	}
	return false
}

func moduleSpecifierFromLine(line string) (string, bool) {
	end := strings.LastIndexAny(line, `"'`)
	if end < 0 {
		return "", false
	}
	start := strings.LastIndexByte(line[:end], line[end])
	if start < 0 {
		return "", false
	}
	return line[start+1 : end], true
}
