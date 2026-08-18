package main

import (
	"slices"
	"testing"
)

// `--scene Title` and `--scene=Title` have to mean the same thing. When only
// the second was accepted, the first read "Title" as a positional argument and
// `nv init --scene Title` scaffolded a whole project into a directory called
// Title — a documented invocation doing something entirely different, quietly.
func TestSplitArgs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantFlags map[string]string
		wantRest  []string
	}{
		{
			name:      "value flag, space form",
			args:      []string{"--scene", "Title"},
			wantFlags: map[string]string{"scene": "Title"},
		},
		{
			name:      "value flag, equals form",
			args:      []string{"--scene=Title"},
			wantFlags: map[string]string{"scene": "Title"},
		},
		{
			name:      "boolean flag does not swallow the next argument",
			args:      []string{"--force", "vi"},
			wantFlags: map[string]string{"force": "true"},
			wantRest:  []string{"vi"},
		},
		{
			name:      "locales after a separator",
			args:      []string{"--", "en", "vi"},
			wantFlags: map[string]string{},
			wantRest:  []string{"en", "vi"},
		},
		{
			name:      "positional directory",
			args:      []string{"my-video"},
			wantFlags: map[string]string{},
			wantRest:  []string{"my-video"},
		},
		{
			name:      "a value flag with nothing after it stays a boolean rather than eating a flag",
			args:      []string{"--scene", "--json"},
			wantFlags: map[string]string{"scene": "true", "json": "true"},
		},
		{
			name:      "the kind travels with the scene",
			args:      []string{"--scene", "Pipeline", "--kind", "flow"},
			wantFlags: map[string]string{"scene": "Pipeline", "kind": "flow"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags, rest := splitArgs(tc.args)

			if len(flags) != len(tc.wantFlags) {
				t.Errorf("flags = %v, want %v", flags, tc.wantFlags)
			}
			for name, want := range tc.wantFlags {
				if flags[name] != want {
					t.Errorf("flags[%q] = %q, want %q", name, flags[name], want)
				}
			}
			if !slices.Equal(rest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}
