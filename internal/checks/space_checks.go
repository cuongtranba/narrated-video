package checks

import "strings"

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
