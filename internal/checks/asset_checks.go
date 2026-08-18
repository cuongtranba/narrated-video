package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bareAssetRe = regexp.MustCompile(`["']([^"']*\.(?:glb|gltf|hdr|ktx2?|exr|basis))["']`)

	httpsAssetURLRe = regexp.MustCompile(`["'](https?://[^"'\s]+)["']`)

	delayRenderRe    = regexp.MustCompile(`\bdelayRender\s*\(`)
	continueRenderRe = regexp.MustCompile(`\bcontinueRender\s*\(`)

	staticFileArgRe = regexp.MustCompile(`staticFile\(["']([^"']*)["']\)`)
)

// CHK-31.
func assetExistsInPublic(kit *Kit) Result {
	const id, title = "CHK-31", "assets referenced via staticFile exist under public/"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		for _, assetPath := range extractStaticFilePaths(kit.SceneSources[sceneID]) {
			full := filepath.Join(kit.Root, "public", assetPath)
			if _, err := os.Stat(full); os.IsNotExist(err) {
				findings = append(findings, Finding{
					Where:  sceneID,
					Detail: fmt.Sprintf("staticFile(%q) — public/%s does not exist", assetPath, assetPath),
				})
			}
		}
	}
	return fail(id, title,
		"add the missing asset to public/ or correct the path passed to staticFile()",
		sortedFindings(findings))
}

// CHK-35.
func scenesHoldFrameOnAssetLoad(kit *Kit) Result {
	const id, title = "CHK-35", "scene modules load assets via staticFile and hold the frame"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		findings = append(findings, assetLoadFindings(sceneID, kit.SceneSources[sceneID])...)
	}
	return fail(id, title,
		"load assets with staticFile(path) and wrap in delayRender/continueRender — missing handles let frames render before assets arrive",
		sortedFindings(findings))
}

func assetLoadFindings(sceneID, source string) []Finding {
	var findings []Finding
	lines := strings.Split(source, "\n")

	hasDelay, hasContinue := false, false
	for _, line := range lines {
		stripped := callLine(line)
		if delayRenderRe.MatchString(stripped) {
			hasDelay = true
		}
		if continueRenderRe.MatchString(stripped) {
			hasContinue = true
		}
	}
	if hasDelay && !hasContinue {
		findings = append(findings, Finding{
			Where:  sceneID,
			Detail: "calls delayRender but never calls continueRender — a failed load hangs the render",
		})
	}

	for i, line := range lines {
		if assetScanSkip(line) {
			continue
		}
		lineRef := fmt.Sprintf("%s:%d", sceneID, i+1)

		for _, m := range httpsAssetURLRe.FindAllStringSubmatch(line, -1) {
			findings = append(findings, Finding{
				Where:  lineRef,
				Detail: fmt.Sprintf("references external URL %q — asset must live under public/ and be loaded via staticFile()", m[1]),
			})
		}

		for _, m := range bareAssetRe.FindAllStringSubmatchIndex(line, -1) {
			prefix := strings.TrimRight(line[:m[0]], " \t")
			if strings.HasSuffix(prefix, "staticFile(") {
				continue
			}
			assetPath := line[m[2]:m[3]]
			findings = append(findings, Finding{
				Where:  lineRef,
				Detail: fmt.Sprintf("references %q without staticFile() — use staticFile(%q) and wrap load in delayRender/continueRender", assetPath, assetPath),
			})
		}
	}
	return findings
}

func extractStaticFilePaths(source string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, m := range staticFileArgRe.FindAllStringSubmatch(source, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			paths = append(paths, m[1])
		}
	}
	return paths
}

func assetScanSkip(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "//") ||
		strings.HasPrefix(t, "*") ||
		strings.HasPrefix(t, "/*") ||
		strings.HasPrefix(t, "import ")
}

// callLine strips // comment suffixes and string literal contents from a
// single line so call-position patterns (delayRender, continueRender) do not
// match tokens that appear inside quotes or after a comment marker.
func callLine(line string) string {
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
			switch {
			case c == '"' || c == '\'' || c == '`':
				inString = true
				stringChar = c
				out = append(out, c)
			case c == '/' && i+1 < len(line) && line[i+1] == '/':
				return string(out)
			default:
				out = append(out, c)
			}
		}
	}
	return string(out)
}
