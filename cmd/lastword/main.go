// Command lastword pins the verdict of a finished Claude Code session.
package main

import (
	"os"

	"github.com/SuperMarioYL/lastword/internal/cli"
)

// version is the dev default; goreleaser injects the release tag at build
// time (-X main.version=...).
var version = "0.2.0"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
