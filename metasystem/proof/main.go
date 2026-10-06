package main

import (
	"os"
	"runtime"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
)

func main() {
	if exit, handled := repoproof.Custodian(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	if len(os.Args) == 3 && os.Args[1] == "--section" {
		os.Exit(repoproof.RunSection(os.Stdout, os.Stderr, os.Args[2]))
	}
	if len(os.Args) == 2 && os.Args[1] == "--host" {
		os.Exit(repoproof.RunHost(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute))
	}
	os.Exit(repoproof.Run(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute))
}

// The native command keeps its registered process identity through exec on Linux.
func init() { runtime.LockOSThread() }
