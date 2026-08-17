// Package checks is the gate. Its exit code is the contract: a project that
// passes is one where nothing the tool can decide mechanically is inconsistent.
//
// Two properties are load-bearing and easy to erode. First, every check runs —
// there is no fail-fast, because a validator that stops at the first problem
// costs one edit-and-rerun cycle per problem, and the project this was built
// from spent twenty-eight of them. Second, no check reads pixels, asks a model
// to judge prose, or touches the network. Each of those would make the answer
// depend on something other than the files, and then the exit code means
// nothing.
package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/project"
)

type Finding struct {
	Where  string
	Detail string
}

type Result struct {
	ID       string
	Title    string
	OK       bool
	Findings []Finding
	// Remedy is one imperative line. A failure the reader cannot act on is a
	// failure they will learn to ignore.
	Remedy string
}

type Check func(*Kit) Result

// Kit is the project plus the files the checks read but the renderer does not.
type Kit struct {
	*project.Project
	// Generated holds what is on disk now, so staleness is a comparison rather
	// than a guess about mtimes.
	Generated map[string][]byte
	// SceneSources is scene id -> TSX source.
	SceneSources map[string]string
	// TrackedFiles is every text file under version control, read once for the
	// secret scan.
	TrackedFiles map[string]string
}

func pass(id, title string) Result { return Result{ID: id, Title: title, OK: true} }

func fail(id, title, remedy string, findings []Finding) Result {
	return Result{ID: id, Title: title, OK: len(findings) == 0, Findings: findings, Remedy: remedy}
}

// All is the registry. A check exists here or it does not run; there is no
// discovery by convention, so the set is auditable by reading one list.
func All() []Check {
	return []Check{
		generatedFresh,
		schemaValid,
		noSecretLiterals,
		ttsModelAllowed,
		narrationMeasured,
		audioPresent,
		narrationHash,
		manifestModelRecorded,
		manifestFPS,
		narrationKeys,
		copyCongruent,
		sceneDurationExclusivity,
		leadCoversTransition,
		copyFitsBudget,
		printedLiterals,
		glossaryUntranslated,
		copyIsNFC,
		fontCoversLocale,
		providerCommittable,
		costCapDeclared,
		sceneSourcesAreTimingFree,
		fractionCuesOnly,
		timelineIsBundlerFree,
		registryParity,
		minSceneDuration,
		durationWithinTarget,
		renderScriptsTargetComposition,
	}
}

// Run evaluates every check and returns results in registry order, so the
// output reads the same way twice.
func Run(kit *Kit) []Result {
	results := make([]Result, 0, len(All()))
	for _, check := range All() {
		results = append(results, check(kit))
	}
	return results
}

func Passed(results []Result) bool {
	for _, r := range results {
		if !r.OK {
			return false
		}
	}
	return true
}

// LoadKit reads the extra state the checks need. A file it cannot read is left
// absent rather than fatal: the check that cares reports it with a remedy, and
// the other twenty-four still run.
func LoadKit(p *project.Project, trackedFiles map[string]string) *Kit {
	kit := &Kit{
		Project:      p,
		Generated:    map[string][]byte{},
		SceneSources: map[string]string{},
		TrackedFiles: trackedFiles,
	}

	for _, name := range []string{"timeline.ts", "registry.ts", "theme.ts", "content.ts"} {
		path := p.GeneratedPath(name)
		if data, err := os.ReadFile(path); err == nil {
			kit.Generated[path] = data
		}
	}

	// The config and the content files are scanned for secrets whether or not
	// they are under version control. Keying the scan solely on git would mean
	// a project that has not been committed yet — the moment a key is most
	// likely to be pasted in "just to try it" — is the one project the check
	// cannot see.
	kit.TrackedFiles[config.FileName] = readOrEmpty(filepath.Join(p.Root, config.FileName))
	for _, locale := range p.Config.LocaleCodes() {
		path := p.ContentPath(locale)
		kit.TrackedFiles[relTo(p.Root, path)] = readOrEmpty(path)
	}
	for _, scene := range p.Config.Scenes {
		if data, err := os.ReadFile(p.ScenePath(scene.ID)); err == nil {
			kit.SceneSources[scene.ID] = string(data)
		}
	}
	return kit
}

func readOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func sortedFindings(f []Finding) []Finding {
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].Where != f[j].Where {
			return f[i].Where < f[j].Where
		}
		return f[i].Detail < f[j].Detail
	})
	return f
}

func at(locale, scene string) string { return fmt.Sprintf("%s/%s", locale, scene) }
