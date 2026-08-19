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

// CHK-38. Remotion refuses to work across a version boundary: every one of its
// packages reaches for the same React context, and two copies in one tree do not
// share it. npm's own resolution is what puts them there — `@remotion/three@4.0.513`
// depends on `remotion@4.0.513` exactly, so a single `^` beside an otherwise
// pinned family installs a second Remotion underneath it.
//
// The reason this needs a check rather than a convention: `remotion render` and
// `remotion compositions` print a mismatch banner and then **exit 0**. The video
// is produced, the pipeline is green, and the frames are wrong — hooks read from
// the copy that is not driving the render. Nothing downstream notices.
//
// CHK-34 cannot see this. It knows the three packages the scene kinds install and
// judges each against what its kind declares; it has nothing to say about
// @remotion/cli, fonts, media, transitions or the eslint config, and a family
// that is uniformly wrong passes it. This check judges the family against itself.
func remotionFamilyPinned(kit *Kit) Result {
	const id, title = "CHK-38", "every Remotion package is on one exact version"

	file, err := pkgscripts.Load(kit.Root)
	if err != nil || file == nil {
		return pass(id, title)
	}
	deps, readable := file.Deps()
	if !readable {
		return pass(id, title)
	}

	// remotion itself is the anchor. Without it there is no version for the rest
	// of the family to be wrong about, and inventing one would put this check in
	// disagreement with CHK-34 about what the project is meant to install.
	pinned := deps["remotion"]
	if pinned == "" {
		return pass(id, title)
	}

	// A slice rather than a map: the order findings come out in is the order the
	// blocks are listed here, with no sort to keep congruent with it. The dev
	// block is optional — a project without one is ordinary, not broken — but
	// what lives there, the eslint config, is still part of the family.
	blocks := []struct {
		where string
		deps  map[string]string
	}{{pkgscripts.FileName + " dependencies", deps}}
	if dev, ok := file.DevDeps(); ok {
		blocks = append(blocks, struct {
			where string
			deps  map[string]string
		}{pkgscripts.FileName + " devDependencies", dev})
	}

	var findings []Finding
	for _, block := range blocks {
		for _, name := range sortedStringKeys(block.deps) {
			if name != "remotion" && !strings.HasPrefix(name, "@remotion/") {
				continue
			}
			if version := block.deps[name]; version != pinned {
				findings = append(findings, Finding{
					Where:  block.where,
					Detail: fmt.Sprintf("%s is %q, but remotion is pinned to %q", name, version, pinned),
				})
			}
		}
	}
	return fail(id, title,
		"pin every @remotion/* package to the same exact version as remotion — a second copy breaks React context at render time and the render still exits 0",
		sortedFindings(findings))
}

var wallClockPatterns = []struct {
	re      *regexp.Regexp
	call    string
	replace string
}{
	{regexp.MustCompile(`\buseFrame\s*\(`), "useFrame", "useCurrentFrame()"},
	{regexp.MustCompile(`\bDate\.now\s*\(`), "Date.now", "useCurrentFrame()"},
	{regexp.MustCompile(`\bnew\s+Date\s*\(`), "new Date", "useCurrentFrame()"},
	{regexp.MustCompile(`\bperformance\.now\s*\(`), "performance.now", "useCurrentFrame()"},
	{regexp.MustCompile(`\bMath\.random\s*\(`), "Math.random", "random() from remotion"},
	{regexp.MustCompile(`\bsetTimeout\s*\(`), "setTimeout", "useCurrentFrame()"},
	{regexp.MustCompile(`\bsetInterval\s*\(`), "setInterval", "useCurrentFrame()"},
	{regexp.MustCompile(`\brequestAnimationFrame\s*\(`), "requestAnimationFrame", "useCurrentFrame()"},
}

// CHK-28. Two renders of the same frame, in separate browser tabs, must be
// identical. Any call that depends on wall clock time or a per-tab RNG makes
// that impossible — and the video looks plausible in a spot-check, so the gate
// is the only thing that catches it.
func sceneSourcesLackWallClockMotion(kit *Kit) Result {
	const id, title = "CHK-28", "scene modules carry no wall-clock or nondeterministic motion"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for i, raw := range strings.Split(kit.SceneSources[sceneID], "\n") {
			stripped := stripStringsAndComment(raw)
			for _, p := range wallClockPatterns {
				if p.re.MatchString(stripped) {
					findings = append(findings, Finding{
						Where:  fmt.Sprintf("%s:%d", sceneID, i+1),
						Detail: fmt.Sprintf("calls %s — use %s instead", p.call, p.replace),
					})
				}
			}
		}
	}
	return fail(id, title,
		"derive timing from useCurrentFrame() from remotion; for randomness, use random() from remotion (seeded, deterministic per frame)",
		sortedFindings(findings))
}

// stripStringsAndComment removes string literal contents and the // comment
// suffix from a single source line, so callers can match call-position
// identifiers without triggering on tokens that appear inside quotes or after
// a comment marker.
func stripStringsAndComment(line string) string {
	var out []byte
	inString := false
	stringChar := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inString {
			if c == '\\' {
				out = append(out, ' ', ' ')
				i++
			} else if c == stringChar {
				out = append(out, c)
				inString = false
			} else {
				out = append(out, ' ')
			}
		} else {
			if c == '"' || c == '\'' || c == '`' {
				inString = true
				stringChar = c
				out = append(out, c)
			} else if c == '/' && i+1 < len(line) && line[i+1] == '/' {
				break
			} else {
				out = append(out, c)
			}
		}
	}
	return string(out)
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
