// Package kit carries the Remotion project template that `nv init` writes.
//
// It is embedded rather than shipped as loose files so the binary is the whole
// distributable: a user installs one executable and scaffolds without a network
// round-trip or a second artifact that could arrive at a different version.
package kit

import "embed"

// Adding a top-level template file means adding it here too. That is deliberate
// friction — a pattern broad enough to sweep the directory would also sweep
// whatever a future contributor leaves lying in it.
//
//go:embed all:src all:public all:content video.config.yaml package.json tsconfig.json remotion.config.ts eslint.config.mjs gitignore env.example README.md
var FS embed.FS
