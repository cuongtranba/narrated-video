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
  nv sync                  regenerate src/generated/ from the config and the audio
  nv validate [--json]     run every check; exit 1 if any fails
  nv voiceover [locale…]   synthesize narration and measure it (--force ignores the spend cap)
  nv version               print the build's content hash

Run from anywhere inside a project; the root is found by walking up.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	command, args := os.Args[1], os.Args[2:]
	var err error
	switch command {
	case "init":
		err = runInit(args)
	case "sync":
		err = runSync(args)
	case "validate":
		err = runValidate(ctx, args)
	case "voiceover":
		err = runVoiceover(ctx, args)
	case "version":
		err = runVersion()
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", command, usage)
		os.Exit(2)
	}

	if err != nil {
		// A validation failure has already printed its findings; anything else
		// is an error the operator has not been told about yet.
		if err != errCheckFailed {
			fmt.Fprintf(os.Stderr, "nv %s: %v\n", command, err)
		}
		os.Exit(1)
	}
}

// flagSet is a deliberately small argument reader. The commands take a handful
// of flags and a list of locales, and the standard library's flag package
// insists those come in a fixed order.
func splitArgs(args []string) (flags map[string]string, rest []string) {
	flags = map[string]string{}
	for _, arg := range args {
		switch {
		case arg == "--":
			continue
		case strings.HasPrefix(arg, "--"):
			name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			if !hasValue {
				value = "true"
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
