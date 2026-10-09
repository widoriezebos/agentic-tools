package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/repoproof"
)

func main() { os.Exit(run()) }

func run() int {
	if exit, handled := repoproof.Custodian(os.Args[1:], os.Stderr); handled {
		return exit
	}
	// The live marker identifies this proof's descendants to admission.
	// The suite launcher owns their resource custody and stop-fence checks.
	root, err := os.Getwd()
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	marker, err := gaterun.Register(root, int64(os.Getpid()), "repository proof")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.Remove(marker)
	if len(os.Args) == 3 && os.Args[1] == "--section" {
		return repoproof.RunSection(os.Stdout, os.Stderr, os.Args[2])
	}
	if len(os.Args) == 2 && os.Args[1] == "--host" {
		return repoproof.RunHost(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute)
	}
	return repoproof.Run(os.Stdout, os.Stderr, os.Getenv, repoproof.Execute)
}

// The native command keeps its registered process identity through exec on Linux.
func init() { runtime.LockOSThread() }
