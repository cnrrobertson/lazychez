// lazychez is a lazygit-style TUI for chezmoi.
//
// Run from any directory. Requires chezmoi to be installed and configured.
package main

import (
	"fmt"
	"os"

	"lazychez/pkg/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lazychez: %v\n", err)
		os.Exit(1)
	}
}
