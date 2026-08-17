package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every `nv <word>` written anywhere in the repo has to name a command that
// exists. The documentation shipped `nv check` in four places for a while —
// plausible, memorable, and not a command. A reader who tries it gets exit 2
// and concludes the tool is broken, which is a worse first impression than any
// missing feature.
func TestDocs_NameOnlyRealCommands(t *testing.T) {
	// Words that follow `nv` in prose without being commands.
	prose := []string{"binary", "is", "was", "reads", "writes", "on", "in", "and", "to", "from", "version"}

	mention := regexp.MustCompile(`\bnv ([a-z][a-z-]+)`)
	root := repoRoot(t)

	for _, path := range documentationFiles(t, root) {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range mention.FindAllStringSubmatch(string(body), -1) {
			word := match[1]
			if _, real := commands[word]; real || slices.Contains(prose, word) {
				continue
			}
			t.Errorf("%s names `nv %s`, which is not a command — the real ones are %v",
				relative(root, path), word, commandNames())
		}
	}
}

// The usage text is what someone reads after getting a command wrong, so it is
// the one place that must list everything.
func TestUsage_ListsEveryCommand(t *testing.T) {
	for name := range commands {
		if !strings.Contains(usage, "nv "+name) {
			t.Errorf("usage text does not mention %q", name)
		}
	}
}

func commandNames() []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func documentationFiles(t *testing.T, root string) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".worktrees", "node_modules", "bin", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".md", ".yaml", ".yml", ".ts", ".tsx", ".example":
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func relative(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
