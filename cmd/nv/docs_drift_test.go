package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cuongtranba/narrated-video/internal/scaffold"
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

// A `references/…` path is written as bare text in prose and in check remedies,
// so nothing but a reader following it discovers that it points at no file. The
// two directions fail differently and both are silent: a dangling link sends
// someone looking for guidance that does not exist, and an unrouted file is
// guidance nobody is sent to. SKILL.md is the only routing surface, so being
// named there is what makes a reference reachable.
func TestDocs_ReferenceLinksResolve(t *testing.T) {
	root := repoRoot(t)
	link := regexp.MustCompile(`references/([a-z0-9-]+\.md)`)

	for _, path := range documentationFiles(t, root) {
		assertReferencesExist(t, root, path, link)
	}
	for _, path := range goSourceFiles(t, root) {
		assertReferencesExist(t, root, path, link)
	}
}

func TestDocs_EveryReferenceIsRoutedFromSKILL(t *testing.T) {
	root := repoRoot(t)
	skill, err := os.ReadFile(filepath.Join(root, skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(root, skillDir, "references"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if !strings.Contains(string(skill), "references/"+entry.Name()) {
			t.Errorf("references/%s is not named in SKILL.md, so nothing routes a reader to it",
				entry.Name())
		}
	}
}

// Both the README and SKILL.md print a sample `nv init` transcript, and the
// file count in it is the kind of number nobody re-checks: the docs said 30
// while the scaffold wrote 39, and the gap only surfaced when someone ran the
// command and read the output. It is a small lie, but it is the first output a
// new user compares against their terminal.
func TestDocs_ScaffoldFileCountIsCurrent(t *testing.T) {
	root := repoRoot(t)

	var out strings.Builder
	if err := scaffold.Project(t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}
	actual := regexp.MustCompile(`wrote (\d+) files`).FindStringSubmatch(out.String())
	if actual == nil {
		t.Fatalf("scaffold no longer prints a file count: %q", out.String())
	}

	claim := regexp.MustCompile(`wrote (\d+) files`)
	for _, path := range []string{"README.md", filepath.Join(skillDir, "SKILL.md")} {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range claim.FindAllStringSubmatch(string(body), -1) {
			if match[1] != actual[1] {
				t.Errorf("%s says `wrote %s files`, but `nv init` writes %s",
					path, match[1], actual[1])
			}
		}
	}
}

const skillDir = "skills/narrated-video"

func assertReferencesExist(t *testing.T, root, path string, link *regexp.Regexp) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range link.FindAllStringSubmatch(string(body), -1) {
		target := filepath.Join(root, skillDir, "references", match[1])
		if _, err := os.Stat(target); err != nil {
			t.Errorf("%s links references/%s, which does not exist",
				relative(root, path), match[1])
		}
	}
}

func goSourceFiles(t *testing.T, root string) []string {
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
		if filepath.Ext(path) == ".go" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
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
