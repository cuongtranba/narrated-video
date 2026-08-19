package kit

import (
	"encoding/json"
	"regexp"
	"testing"
)

// The lockfile ships to every scaffolded project, so it is the file that decides
// what a user actually installs. A lock that disagrees with package.json is a
// scaffold whose tree is not the one this repo tested — and the disagreement is
// silent, because `bun install` simply re-resolves and moves on.
//
// This is not hypothetical. The npm bump in #32 moved package.json to Remotion
// 4.0.513, ESLint 10 and TypeScript 6, and left the lock recording 4.0.512,
// ESLint 9.19 and TypeScript 5.9.3 — with no entry at all for the three packages
// a space scene needs. Dependabot maintains both together; a hand edit is what
// splits them, and this is what says so.
func TestLockfile_AgreesWithPackageJSON(t *testing.T) {
	declared := packageJSONDeps(t)
	locked := lockfileDeps(t)

	for block, want := range declared {
		got := locked[block]
		for name, version := range want {
			if got[name] != version {
				t.Errorf("%s.%s: package.json says %q, bun.lock says %q — run `bun install --lockfile-only` in kit/",
					block, name, version, got[name])
			}
		}
		for name := range got {
			if _, ok := want[name]; !ok {
				t.Errorf("%s.%s is in bun.lock but not in package.json — run `bun install --lockfile-only` in kit/", block, name)
			}
		}
	}
}

// Every package the template installs must be in the lock's resolution table, or
// the scaffold ships a lock that cannot satisfy its own package.json.
func TestLockfile_ResolvesEveryDeclaredPackage(t *testing.T) {
	var doc struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(lockfileJSON(t), &doc); err != nil {
		t.Fatal(err)
	}

	for block, deps := range packageJSONDeps(t) {
		for name := range deps {
			if _, ok := doc.Packages[name]; !ok {
				t.Errorf("%s.%s is declared but not resolved in bun.lock — run `bun install --lockfile-only` in kit/", block, name)
			}
		}
	}
}

func packageJSONDeps(t *testing.T) map[string]map[string]string {
	t.Helper()

	raw, err := FS.ReadFile("package.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return map[string]map[string]string{
		"dependencies":    doc.Dependencies,
		"devDependencies": doc.DevDependencies,
	}
}

func lockfileDeps(t *testing.T) map[string]map[string]string {
	t.Helper()

	var doc struct {
		Workspaces map[string]struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(lockfileJSON(t), &doc); err != nil {
		t.Fatal(err)
	}

	root, ok := doc.Workspaces[""]
	if !ok {
		t.Fatal("bun.lock has no root workspace")
	}
	return map[string]map[string]string{
		"dependencies":    root.Dependencies,
		"devDependencies": root.DevDependencies,
	}
}

// bun.lock is JSON with trailing commas, which encoding/json refuses. Dropping
// them is enough to read it: the format is otherwise ordinary JSON, and this is
// a test asserting on content rather than a parser anything depends on.
var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

func lockfileJSON(t *testing.T) []byte {
	t.Helper()

	raw, err := FS.ReadFile("bun.lock")
	if err != nil {
		t.Fatalf("bun.lock is not embedded: %v", err)
	}
	return trailingComma.ReplaceAll(raw, []byte("$1"))
}
