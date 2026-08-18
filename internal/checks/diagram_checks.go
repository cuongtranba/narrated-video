package checks

import (
	"fmt"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/project"
)

// CHK-29. A typo'd endpoint renders an edge from or to nowhere while the process
// exits 0. The node never appears, the edge converges on a point, and no error
// is produced — a silent visual lie in the final video.
func diagramEdgesReferenceKnownNodes(kit *Kit) Result {
	const id, title = "CHK-29", "every diagram edge references nodes declared in the same diagram"

	var findings []Finding
	for _, name := range project.SortedKeys(kit.Config.Diagrams) {
		d := kit.Config.Diagrams[name]
		known := make(map[string]bool, len(d.Nodes))
		for _, node := range d.Nodes {
			known[node.ID] = true
		}
		for _, edge := range d.Edges {
			if !known[edge.From] {
				findings = append(findings, Finding{
					Where:  name,
					Detail: fmt.Sprintf("edge from %q: node %q is not declared", edge.To, edge.From),
				})
			}
			if !known[edge.To] {
				findings = append(findings, Finding{
					Where:  name,
					Detail: fmt.Sprintf("edge from %q: node %q is not declared", edge.From, edge.To),
				})
			}
		}
	}
	return fail(id, title, "add the missing node to the diagram's nodes list, or fix the edge endpoint", sortedFindings(findings))
}

// CHK-30. Diagram labels are translated content. A node that has copy in one
// locale but not another renders its key as visible text or nothing, in silence,
// while the render exits 0. This is the localization failure the kit already
// guards for narration, applied to the surface a translator is least likely to
// notice.
func diagramNodeLabelsComplete(kit *Kit) Result {
	const id, title = "CHK-30", "every diagram node has a label in every locale"

	var findings []Finding
	for _, name := range project.SortedKeys(kit.Config.Diagrams) {
		d := kit.Config.Diagrams[name]
		for _, locale := range kit.Config.Locales.List {
			labels := diagramLabelsForLocale(kit.Content[locale.Code].Copy, name)
			for _, node := range d.Nodes {
				if labels[node.ID] == "" {
					findings = append(findings, Finding{
						Where:  fmt.Sprintf("%s/%s", locale.Code, name),
						Detail: fmt.Sprintf("node %q has no label", node.ID),
					})
				}
			}
		}
	}
	return fail(id, title, "add the missing label to content/<locale>.yaml under copy.diagrams.<diagram>.<nodeId>", sortedFindings(findings))
}

func diagramLabelsForLocale(copy map[string]any, diagramName string) map[string]string {
	result := map[string]string{}
	diagrams, _ := copy["diagrams"].(map[string]any)
	if diagrams == nil {
		return result
	}
	labels, _ := diagrams[diagramName].(map[string]any)
	for k, v := range labels {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// CHK-32. React Flow measures node dimensions with a ResizeObserver after first
// paint. A headless renderer never fires that observer, so any node without an
// explicit width and height renders as a 0×0 point while the render exits 0.
func diagramNodesDeclaredSize(kit *Kit) Result {
	const id, title = "CHK-32", "every diagram node declares width and height"

	path := kit.GeneratedPath("diagrams.ts")
	source, ok := kit.Generated[path]
	if !ok {
		return pass(id, title)
	}

	findings := nodesWithoutSize(string(source), relTo(kit.Root, path))
	return fail(id, title,
		"declare width/height on every node — React Flow cannot measure a node in a headless render, and an unmeasured node renders 0×0 while the render still exits 0",
		sortedFindings(findings))
}

// nodesWithoutSize scans a diagrams.ts source for node objects that lack width
// and height. It uses brace depth to associate properties with their enclosing
// object, entering node-tracking only inside a nodes: [...] array.
func nodesWithoutSize(source, location string) []Finding {
	var findings []Finding
	lines := strings.Split(source, "\n")

	inNodes := false
	depth := 0
	nodeStart := -1
	var nodeLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inNodes {
			if strings.Contains(trimmed, "nodes:") && strings.Contains(trimmed, "[") {
				inNodes = true
			}
			continue
		}

		opens := strings.Count(trimmed, "{")
		closes := strings.Count(trimmed, "}")
		prevDepth := depth

		if prevDepth == 0 && opens > 0 {
			nodeStart = i
			nodeLines = []string{line}
		} else if nodeStart >= 0 {
			nodeLines = append(nodeLines, line)
		}

		depth += opens - closes

		if nodeStart >= 0 && depth == 0 {
			nodeText := strings.Join(nodeLines, "\n")
			if strings.Contains(nodeText, "id:") {
				if !strings.Contains(nodeText, "width:") || !strings.Contains(nodeText, "height:") {
					findings = append(findings, Finding{
						Where:  fmt.Sprintf("%s:%d", location, nodeStart+1),
						Detail: "node object is missing width or height",
					})
				}
			}
			nodeStart = -1
			nodeLines = nil
		}

		if depth == 0 && strings.Contains(trimmed, "]") {
			inNodes = false
		}
	}

	return findings
}

// CHK-36. A scene that imports @xyflow/react directly bypasses the Diagram
// wrapper, which is where the determinism guards live: fitView disabled, all
// interaction locked, defaultViewport set explicitly. Without the wrapper a
// scene can still render — the check that catches it otherwise exits 0.
func scenesUseDiagramWrapper(kit *Kit) Result {
	const id, title = "CHK-36", "scene modules import React Flow only through the Diagram wrapper"

	var findings []Finding
	for _, sceneID := range sortedStringKeys(kit.SceneSources) {
		if importsPath(kit.SceneSources[sceneID], "@xyflow/react") {
			findings = append(findings, Finding{
				Where:  sceneID,
				Detail: "imports @xyflow/react directly",
			})
		}
	}
	return fail(id, title,
		"use <Diagram> from kit components instead — direct @xyflow/react imports bypass the wrapper's determinism guards",
		sortedFindings(findings))
}
