package checks

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
	"github.com/cuongtranba/narrated-video/internal/scenekind"
)

// A scene learns its length from props. Reaching into the timeline gives it a
// second, contradictory answer, and reaching into the config gives it a third.
var forbiddenSceneImports = []string{
	"generated/timeline",
	"generated/registry",
	"video.config",
}

// CHK-21.
func sceneSourcesAreTimingFree(kit *Kit) Result {
	const id, title = "CHK-21", "scenes learn their length from props only"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		source := kit.SceneSources[sceneID]
		for _, forbidden := range forbiddenSceneImports {
			if importsPath(source, forbidden) {
				findings = append(findings, Finding{
					Where:  sceneID,
					Detail: fmt.Sprintf("imports %s", forbidden),
				})
			}
		}
	}
	return fail(id, title, "take durationInFrames and leadFrames from SceneProps instead", sortedFindings(findings))
}

// The second argument to `at` is the scene's duration. A literal there pins a
// cue to one language's audio, and the translation drifts away from the
// sentence that explains it — silently, because every frame still renders.
//
// Only the second argument is constrained: the first is expected to be a
// literal fraction. That is what makes this decidable by inspection. A blanket
// ban on numeric literals in scene sources is not — they legitimately carry
// five to fourteen of them each for layout, and a rule that flags those flags
// everything while missing this.
var absoluteCue = regexp.MustCompile(`\bat\(\s*[^,()]+,\s*(\d+(?:\.\d+)?)\s*\)`)

// CHK-22.
func fractionCuesOnly(kit *Kit) Result {
	const id, title = "CHK-22", "reveals are cued as fractions, so a translation re-times itself"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for i, line := range strings.Split(kit.SceneSources[sceneID], "\n") {
			for _, match := range absoluteCue.FindAllStringSubmatch(line, -1) {
				findings = append(findings, Finding{
					Where:  fmt.Sprintf("%s:%d", sceneID, i+1),
					Detail: fmt.Sprintf("at(…, %s) hardcodes a frame count", match[1]),
				})
			}
		}
	}
	return fail(id, title, "pass durationInFrames as the second argument", sortedFindings(findings))
}

// CHK-23. The timeline is read by tests and tools with no bundler. One import
// of remotion and it can only be loaded through a build, which puts the numbers
// the whole video depends on behind the thing most likely to be broken.
func timelineIsBundlerFree(kit *Kit) Result {
	const id, title = "CHK-23", "the generated timeline loads without a bundler"

	banned := []string{"remotion", "react", "@remotion/"}
	var findings []Finding

	for _, path := range sortedByteKeys(kit.Generated) {
		base := path[strings.LastIndex(path, "/")+1:]
		if base != "timeline.ts" && base != "content.ts" && base != "diagrams.ts" {
			continue
		}
		source := string(kit.Generated[path])
		for _, pkg := range banned {
			// A type-only import is erased before anything runs, so it cannot
			// drag a bundler in.
			if importsPath(source, pkg) && !onlyTypeImports(source, pkg) {
				findings = append(findings, Finding{Where: relTo(kit.Root, path), Detail: "imports " + pkg})
			}
		}
	}
	return fail(id, title, "regenerate with: nv sync", sortedFindings(findings))
}

// CHK-34. A scene's kind is visible only in what it imports, and the packages
// that back it live in package.json — two surfaces, and the failure when they
// disagree lands at bundle time as an unresolved import, after the voiceover has
// been paid for. `nv sync` reconciles them; this catches a package.json edited
// out from under it, and a scene whose kind changed by hand.
//
// A package.json nv cannot read is not a finding. It belongs to the JavaScript
// project, and a check whose remedy would not work is one readers learn to skip.
func pkgDepsMatchSceneKinds(kit *Kit) Result {
	const id, title = "CHK-34", "package.json installs what the scene kinds in use require"

	file, err := pkgscripts.Load(kit.Root)
	if err != nil || file == nil {
		return pass(id, title)
	}
	installed, readable := file.Deps()
	if !readable {
		return pass(id, title)
	}

	var findings []Finding
	for _, dep := range scenekind.Required(kit.SceneSources) {
		got, present := installed[dep.Name]
		switch {
		case !present:
			findings = append(findings, Finding{
				Where:  pkgscripts.FileName + " dependencies",
				Detail: fmt.Sprintf("%s is missing; a scene in this project needs %s", dep.Name, dep.Version),
			})
		case got != dep.Version:
			findings = append(findings, Finding{
				Where:  pkgscripts.FileName + " dependencies",
				Detail: fmt.Sprintf("%s is %q, but the scene kinds in use need %q", dep.Name, got, dep.Version),
			})
		}
	}
	return fail(id, title, "run: nv sync", sortedFindings(findings))
}

func importsPath(source, needle string) bool {
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") && !strings.HasPrefix(trimmed, "export ") {
			continue
		}
		if strings.Contains(trimmed, `"`+needle) || strings.Contains(trimmed, `'`+needle) ||
			strings.Contains(trimmed, needle+`"`) || strings.Contains(trimmed, needle+`'`) {
			return true
		}
	}
	return false
}

func onlyTypeImports(source, needle string) bool {
	found := false
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, needle) || !strings.HasPrefix(trimmed, "import") {
			continue
		}
		found = true
		if !strings.HasPrefix(trimmed, "import type ") {
			return false
		}
	}
	return found
}

func sortedByteKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
