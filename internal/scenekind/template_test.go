package scenekind

import (
	"strings"
	"testing"
)

// The version a kind requires and the version a scaffolded project installs are
// the same number read from one file. A kind that carried its own copy would
// disagree with the template the moment either moved — and CHK-34 compares them
// exactly, so the disagreement would surface in the user's project, with
// `nv sync` reverting the bump rather than adopting it.
func TestDependenciesCarryTheTemplatesVersions(t *testing.T) {
	for _, kind := range All() {
		deps := kind.Dependencies()
		if len(deps) != len(kind.Packages) {
			t.Errorf("the %s kind names %d packages but resolves %d", kind.Name, len(kind.Packages), len(deps))
		}
		for _, dep := range deps {
			if dep.Version != templateVersions[dep.Name] {
				t.Errorf("the %s kind requires %s at %q, but the template installs %q",
					kind.Name, dep.Name, dep.Version, templateVersions[dep.Name])
			}
			if dep.Version == "" {
				t.Errorf("the %s kind requires %s at no version", kind.Name, dep.Name)
			}
		}
	}
}

// Dropping a package from the template while a kind still imports it is the one
// way this arrangement can break, and it breaks silently: the scene scaffolds,
// and the bundle fails on an unresolved import. It has to be a build failure.
func TestTemplateMissingAKindsPackageIsRefused(t *testing.T) {
	_, err := templateVersionsFrom([]byte(`{"dependencies": {"remotion": "4.0.513"}}`))
	if err == nil {
		t.Fatal("a template installing none of the kinds' packages was accepted")
	}
	if !strings.Contains(err.Error(), "does not install") {
		t.Errorf("error = %q, want it to name the missing package", err)
	}
}

func TestTemplateThatIsNotJSONIsRefused(t *testing.T) {
	if _, err := templateVersionsFrom([]byte("not json")); err == nil {
		t.Fatal("an unparseable package.json was accepted")
	}
}

// A kind that names no package is text: it renders with what every project
// already installs. Any other kind must name at least one, or `nv init --kind`
// would scaffold a scene and add nothing to package.json for it.
func TestOnlyTextNeedsNoPackages(t *testing.T) {
	for _, kind := range All() {
		switch {
		case kind.Name == Text && len(kind.Packages) > 0:
			t.Errorf("the text kind names %v, but it is the kind that needs nothing", kind.Packages)
		case kind.Name != Text && len(kind.Packages) == 0:
			t.Errorf("the %s kind names no package, so nothing would be added to package.json", kind.Name)
		}
	}
}
