package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// runConfigCanonicalModel prints the canonical model key for a name given as
// the sole positional argument.
func runConfigCanonicalModel(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem config canonical-model <name>")
		return 2
	}
	fmt.Println(config.CanonicalModel(args[0]))
	return 0
}

// runConfigIdentity prints one adapter's canonical configuration identity as
// JSON.
func runConfigIdentity(args []string) int {
	flags := flag.NewFlagSet("config identity", flag.ContinueOnError)
	runtime := flags.String("runtime", "", "runtime name")
	version := flags.String("version", "", "CLI version")
	if flags.Parse(args) != nil {
		return 2
	}
	if *runtime == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem config identity --runtime R --version V [sources...]")
		return 2
	}
	identity, err := config.BuildConfigIdentity(*runtime, *version, flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, err := config.CanonicalConfigJSON(identity)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(encoded)
	return 0
}
