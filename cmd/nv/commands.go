package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cuongtranba/narrated-video/internal/checks"
	"github.com/cuongtranba/narrated-video/internal/config"
	"github.com/cuongtranba/narrated-video/internal/gen"
	"github.com/cuongtranba/narrated-video/internal/pipeline"
	"github.com/cuongtranba/narrated-video/internal/pkgscripts"
	"github.com/cuongtranba/narrated-video/internal/project"
	"github.com/cuongtranba/narrated-video/internal/scaffold"
	"github.com/cuongtranba/narrated-video/internal/script"
	"github.com/cuongtranba/narrated-video/internal/voiceover"
)

// errCheckFailed carries the exit code without a second error message: the
// findings have already been printed, in the form the reader can act on.
var errCheckFailed = errors.New("checks failed")

func runInit(args []string) error {
	flags, rest := splitArgs(args)

	if sceneID := flags["scene"]; sceneID != "" && sceneID != "true" {
		root, err := config.Find(".")
		if err != nil {
			return err
		}
		return scaffold.AddScene(root, sceneID, os.Stdout)
	}

	dir := "."
	if len(rest) > 0 {
		dir = rest[0]
	}
	if err := scaffold.Project(dir, os.Stdout); err != nil {
		return err
	}
	return syncAt(dir)
}

func runSync(args []string) error {
	root, err := config.Find(".")
	if err != nil {
		return err
	}
	return syncAt(root)
}

func syncAt(dir string) error {
	root, err := config.Find(dir)
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	files, err := gen.All(p)
	if err != nil {
		return err
	}

	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f.Path, f.Data, 0o644); err != nil {
			return err
		}
		fmt.Printf("  %s\n", relTo(root, f.Path))
	}
	return retargetRenderScripts(root, p.Config.CompositionID(p.Config.Locales.Default))
}

// retargetRenderScripts keeps package.json's render and still scripts pointed at
// the composition the config declares. Sync owns it for the same reason it owns
// src/generated/: it is derived from the config, so a hand-kept copy is a copy
// that eventually disagrees.
func retargetRenderScripts(root, composition string) error {
	file, err := pkgscripts.Load(root)
	if err != nil || file == nil {
		return err
	}
	data, changed := file.Retargeted(composition)
	if !changed {
		return nil
	}
	if err := os.WriteFile(file.Path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("  %s\n", relTo(root, file.Path))
	return nil
}

func runValidate(_ context.Context, args []string) error {
	flags, _ := splitArgs(args)

	root, err := config.Find(".")
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}

	results := checks.Run(checks.LoadKit(p, trackedTextFiles(root)))

	if flags["json"] == "true" {
		if err := printJSON(results); err != nil {
			return err
		}
	} else {
		printResults(results)
	}
	if !checks.Passed(results) {
		return errCheckFailed
	}
	return nil
}

// runStatus always exits 0. Reporting where a project is and gating whether it
// is consistent are two questions, and `nv validate` already owns the second —
// two commands whose exit codes mean different things is how a green run starts
// meaning nothing.
func runStatus(args []string) error {
	flags, _ := splitArgs(args)

	root, err := config.Find(".")
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}

	kit := checks.LoadKit(p, trackedTextFiles(root))
	status := pipeline.Derive(kit, checks.Run(kit))

	if flags["json"] == "true" {
		return printJSON(status)
	}
	status.Write(os.Stdout)
	return nil
}

func runScript(args []string) error {
	_, rest := splitArgs(args)

	root, err := config.Find(".")
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}

	locale := ""
	if len(rest) > 0 {
		locale = rest[0]
	}
	out, err := script.Render(p, locale)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func printResults(results []checks.Result) {
	failed := 0
	for _, r := range results {
		if r.OK {
			fmt.Printf("  ok   %s  %s\n", r.ID, r.Title)
			continue
		}
		failed++
		fmt.Printf("\n  FAIL %s  %s\n", r.ID, r.Title)
		for _, f := range r.Findings {
			fmt.Printf("       %s: %s\n", f.Where, f.Detail)
		}
		fmt.Printf("       → %s\n", r.Remedy)
	}

	fmt.Println()
	if failed == 0 {
		fmt.Printf("%d checks passed\n", len(results))
		return
	}
	fmt.Printf("%d of %d checks failed\n", failed, len(results))
}

func runVoiceover(ctx context.Context, args []string) error {
	flags, locales := splitArgs(args)

	root, err := config.Find(".")
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}

	// The key is read here and passed by value. It is never written to the
	// config, never logged, and never lands in a generated file.
	apiKey := ""
	if name := p.Config.TTS.APIKeyEnv; name != "" {
		apiKey = os.Getenv(name)
	}

	err = voiceover.Run(ctx, p, voiceover.Options{
		Locales: locales,
		Force:   flags["force"] == "true",
		APIKey:  apiKey,
	}, os.Stdout)
	if err != nil {
		return err
	}

	// Measurements just changed, so the generated timeline is stale by
	// definition. Leaving that to the operator is how a project ends up with
	// scenes timed to audio it no longer has.
	fmt.Println("\nregenerating:")
	return syncAt(root)
}

// runVersion prints a content hash rather than a release tag. The binaries are
// committed and CI asserts they match their source by rebuilding and comparing
// bytes; stamping a version into them would make that comparison impossible,
// because writing the stamp changes the thing being compared.
func runVersion() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	fmt.Printf("nv %s\n", hex.EncodeToString(sum[:])[:16])
	return nil
}

// trackedTextFiles feeds the secret scan. Version control decides what counts:
// a key in an untracked scratch file is a local problem, and a key in a tracked
// one is in the history from the moment it lands.
func trackedTextFiles(root string) map[string]string {
	out := map[string]string{}

	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	listing, err := cmd.Output()
	if err != nil {
		return out
	}

	const maxScan = 1 << 20
	for _, name := range strings.Split(string(listing), "\x00") {
		if name == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Size() > maxScan {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		out[name] = string(data)
	}
	return out
}

func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}
