// Package scaffold writes a working project out of the binary.
//
// It exists to delete a specific cost. Assembling this project by hand took
// roughly twenty-nine separate steps in the session this tool came from —
// creating a template, then undoing the parts of it that were wrong, adding
// four dependency groups one at a time, rewriting three config files, and
// hunting for font subsets. None of that is a decision anyone wants to make
// twice.
package scaffold

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
	"github.com/cuongtranba/narrated-video/internal/scenekind"
	"github.com/cuongtranba/narrated-video/internal/schema"
	"github.com/cuongtranba/narrated-video/kit"
)

// Files whose names must change on the way out. A dotfile cannot be embedded
// under its real name without tooling swallowing it before it ever ships.
var renameOnWrite = map[string]string{
	"gitignore":   ".gitignore",
	"env.example": ".env.example",
}

func Project(dir string, out io.Writer) error {
	if existing, _ := os.ReadDir(dir); len(existing) > 0 {
		if _, err := os.Stat(filepath.Join(dir, "video.config.yaml")); err == nil {
			return fmt.Errorf("%s already holds a project", dir)
		}
	}

	written := 0
	err := fs.WalkDir(kit.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || strings.HasSuffix(path, ".go") {
			return err
		}
		data, err := kit.FS.ReadFile(path)
		if err != nil {
			return err
		}
		if renamed, ok := renameOnWrite[path]; ok {
			path = renamed
		}
		written++
		return write(filepath.Join(dir, path), data)
	})
	if err != nil {
		return err
	}

	// The schema travels with the project so an editor can complete the config
	// against the same document the tool validates it with.
	if err := write(filepath.Join(dir, schema.FileName), schema.File); err != nil {
		return err
	}
	written++

	// One next step, not five. The running order is derived, so pointing at the
	// command that derives it beats a list that goes stale the moment the
	// pipeline gains a stage.
	fmt.Fprintf(out, "wrote %d files to %s\n\nNext:\n  cd %s\n  nv status        # the stage you are on, and the command that follows\n",
		written, dir, dir)
	return nil
}

// AddScene writes the module, registers it, and installs what its kind needs.
// The three happen together because any two without the third leave a project
// that fails somewhere other than here: unregistered renders nothing, and
// registered without its dependency fails to resolve an import at bundle time.
func AddScene(root, id, kindName string, out io.Writer) error {
	if kindName == "" {
		kindName = scenekind.Text
	}
	kind, known := scenekind.Lookup(kindName)
	if !known {
		return fmt.Errorf("unknown scene kind %q — valid kinds: %s", kindName, strings.Join(scenekind.Names(), ", "))
	}
	if !isSceneID(id) {
		return fmt.Errorf("%q is not a scene id — use PascalCase letters and digits", id)
	}

	path := filepath.Join(root, "src", "scenes", id+".tsx")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}

	template, err := kit.FS.ReadFile(kind.TemplatePath)
	if err != nil {
		return err
	}
	if err := write(path, template); err != nil {
		return err
	}

	configPath := filepath.Join(root, "video.config.yaml")
	current, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	updated, err := appendScene(string(current), id)
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		return err
	}

	// Only this kind's dependencies, not the whole project's: reconciliation
	// adds and never removes, so scenes already in the project have already
	// contributed theirs.
	installed, err := pkgscripts.ReconcileDepsAt(root, kind.Dependencies)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "wrote src/scenes/%s.tsx from the %s template and added it to video.config.yaml\n", id, kind.Name)
	if installed {
		fmt.Fprintf(out, "added to package.json: %s\n", strings.Join(depNames(kind), ", "))
	}
	fmt.Fprintf(out, "\nNext:\n  add its narration line to content/<locale>.yaml\n")
	if installed {
		fmt.Fprintf(out, "  bun install\n")
	}
	fmt.Fprintf(out, "  read %s\n  nv status\n", kind.Reference)
	return nil
}

func depNames(kind scenekind.Kind) []string {
	names := make([]string, 0, len(kind.Dependencies))
	for _, dep := range kind.Dependencies {
		names = append(names, dep.Name+"@"+dep.Version)
	}
	return names
}

// appendScene edits the document as text rather than reserialising it. A YAML
// round-trip would reflow the whole file and drop the comments explaining why a
// particular scene holds longer than the rest, which are the most valuable
// lines in it.
func appendScene(source, id string) (string, error) {
	lines := strings.Split(source, "\n")

	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "scenes:") {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("video.config.yaml has no scenes: block")
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "-") {
			continue
		}
		end = i
		break
	}
	for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	inserted := append([]string{}, lines[:end]...)
	inserted = append(inserted, "  - "+id)
	inserted = append(inserted, lines[end:]...)
	return strings.Join(inserted, "\n"), nil
}

func isSceneID(id string) bool {
	if id == "" || id[0] < 'A' || id[0] > 'Z' {
		return false
	}
	for _, r := range id {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum {
			return false
		}
	}
	return true
}

func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
