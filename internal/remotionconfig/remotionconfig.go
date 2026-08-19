// Package remotionconfig owns the one setting in remotion.config.ts that is
// derived rather than authored: the Chromium GL backend.
//
// It exists for the reason pkgscripts exists. Whether a project renders 3D is
// already written down — in what its scenes import — and the GL backend follows
// from it. A copy kept by hand is one that eventually disagrees, and this
// particular disagreement is invisible: headless Chromium without a WebGL
// backend renders the 3D content as a black rectangle, at the right size, for
// the right duration, and exits 0.
//
// Only that one call is claimed. Every other setting is the author's, and a file
// this package cannot read confidently is left exactly as it was.
package remotionconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const FileName = "remotion.config.ts"

// The angle backend specifically. A config that selects some other renderer has
// not made the choice a space scene needs, so it reads as absent rather than as
// something to leave alone.
var glRenderer = regexp.MustCompile(`Config\.setChromiumOpenGlRenderer\s*\(\s*["']angle["']\s*\)`)

// The exact line this package writes, and the only form it removes.
const glRendererCall = `Config.setChromiumOpenGlRenderer("angle")`

// HasGLRenderer reports whether the config selects the ANGLE backend. A
// commented-out call does not count: it is an example, not a setting, and
// reading it as one would pass a project that renders a black rectangle.
func HasGLRenderer(source []byte) bool {
	for _, line := range strings.Split(string(source), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}
		if glRenderer.MatchString(line) {
			return true
		}
	}
	return false
}

// Reconciled returns source with the ANGLE selection present or absent as wantGL
// requires, and false when it already matches.
//
// Adding appends, rather than placing the call among the others, because the
// order of Config calls is the author's and there is no position this one
// belongs in more than the end. Removing takes only a line this package would
// have written; a call the author put somewhere else, or wrote differently, is
// left for them.
func Reconciled(source []byte, wantGL bool) ([]byte, bool) {
	if HasGLRenderer(source) == wantGL {
		return nil, false
	}
	if wantGL {
		return withGLRenderer(source), true
	}
	return withoutGLRenderer(source), true
}

func withGLRenderer(source []byte) []byte {
	body := string(source)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return []byte(body + glRendererCall + "\n")
}

func withoutGLRenderer(source []byte) []byte {
	lines := strings.Split(string(source), "\n")

	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == glRendererCall {
			continue
		}
		kept = append(kept, line)
	}

	// Appending the call left a blank line behind it only if the file already
	// ended in one; collapse the pair back so removing restores what was there.
	out := strings.Join(kept, "\n")
	return []byte(strings.TrimRight(out, "\n") + "\n")
}

// ReconcileAt brings root's remotion.config.ts in line with whether the project
// renders 3D, and reports whether the file changed. A config that is absent or
// unreadable is not written: it belongs to the JavaScript project, and CHK-33
// reports it with a remedy the author can act on.
func ReconcileAt(root string, wantGL bool) (bool, error) {
	path := filepath.Join(root, FileName)
	source, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	out, changed := Reconciled(source, wantGL)
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, out, 0o644)
}
