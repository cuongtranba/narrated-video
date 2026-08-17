// Command nv builds narrated explainer videos: it scaffolds the project,
// derives every scene's length from measured audio, synthesizes the narration,
// and gates the result.
//
// Everything deterministic lives here rather than in the project it scaffolds.
// That is what lets the gate run in a bare container, on a fresh clone, before
// the video's own dependencies are installed — a broken node_modules can change
// what renders, but it cannot change what this says.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const usage = `nv — narrated explainer videos

  nv init [dir]            scaffold a project (--scene <Id> adds one scene)
  nv status [--json]       where the project is, and the one command to run next
  nv sync                  regenerate src/generated/ from the config and the audio
  nv validate [--json]     run every check; exit 1 if any fails
  nv voiceover [locale…]   synthesize narration and measure it (--force ignores the spend cap)
  nv script [locale]       write the readable script to stdout, from the content file
  nv version               print the build's content hash

Run from anywhere inside a project; the root is found by walking up.
`

// commands is the single list of what `nv` can do. Dispatch reads it, and so
// does the drift test that checks the documentation never names a command that
// does not exist — the docs shipped `nv check` for a while, which reads as a
// perfectly plausible command and is not one.
var commands = map[string]func(context.Context, []string) error{
	"init":      func(_ context.Context, args []string) error { return runInit(args) },
	"status":    func(_ context.Context, args []string) error { return runStatus(args) },
	"sync":      func(_ context.Context, args []string) error { return runSync(args) },
	"validate":  runValidate,
	"voiceover": runVoiceover,
	"script":    func(_ context.Context, args []string) error { return runScript(args) },
	"version":   func(context.Context, []string) error { return runVersion() },
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	command, args := os.Args[1], os.Args[2:]
	if command == "help" || command == "-h" || command == "--help" {
		fmt.Print(usage)
		return
	}

	run, known := commands[command]
	if !known {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, usage)
		os.Exit(2)
	}

	if err := run(ctx, args); err != nil {
		// A validation failure has already printed its findings; anything else
		// is an error the operator has not been told about yet.
		if err != errCheckFailed {
			fmt.Fprintf(os.Stderr, "nv %s: %v\n", command, err)
		}
		os.Exit(1)
	}
}

// Flags that take a value, so `--scene Title` and `--scene=Title` mean the same
// thing. Without this the space form silently reads "Title" as a positional
// argument, and `nv init --scene Title` scaffolds a project into a directory
// called Title instead of adding a scene.
var valueFlags = map[string]bool{"scene": true}

// splitArgs is a deliberately small argument reader. The commands take a
// handful of flags plus a list of locales, and the standard library's flag
// package requires flags to come before positional arguments.
func splitArgs(args []string) (flags map[string]string, rest []string) {
	flags = map[string]string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			continue

		case strings.HasPrefix(arg, "--"):
			name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			if !hasValue {
				if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
					i++
					value = args[i]
				} else {
					value = "true"
				}
			}
			flags[name] = value

		default:
			rest = append(rest, arg)
		}
	}
	return flags, rest
}

func printJSON(v any) error {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
