// Command tq is the stock tui-q binary. It runs the core with no instance
// code, against the demo profile beside this file unless TQ_PROFILE names
// another.
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/lawsmd/tui-q/tuiq"
)

//go:embed profile.toml
var demoProfile string

func main() {
	if err := tuiq.Run(tuiq.WithProfile(demoProfile)); err != nil {
		fmt.Fprintln(os.Stderr, "tq:", err)
		os.Exit(1)
	}
}
