package main

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The runtime family is the shell's ONLY window onto the runtime
// registry: the declaration lives in Go, plumbing asks the binary.
// Exit codes are pinned: 0 ok, 1 unknown
// runtime or undeclared capability, 2 usage.

func runtimeArg(args []string, verbName string) (runtimes.Declaration, int) {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: metasystem runtime %s <runtime>\n", verbName)
		return runtimes.Declaration{}, 2
	}
	declaration, ok := runtimes.Lookup(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown runtime: %s\n", args[0])
		return runtimes.Declaration{}, 1
	}
	return declaration, 0
}

func runRuntimeList(args []string) int {
	var names []string
	switch {
	case len(args) == 0:
		names = runtimes.Names()
	case len(args) == 1 && args[0] == "--adoptable":
		names = runtimes.Adoptable()
	case len(args) == 1 && args[0] == "--with-adapter":
		names = runtimes.WithAdapter()
	case len(args) == 1 && args[0] == "--with-host":
		names = runtimes.WithHost()
	case len(args) == 1 && args[0] == "--with-common-lifecycle":
		names = runtimes.WithCommonLifecycle()
	default:
		fmt.Fprintln(os.Stderr, "usage: metasystem internal runtime list [--adoptable|--with-adapter|--with-host|--with-common-lifecycle]")
		return 2
	}
	for _, name := range names {
		fmt.Println(name)
	}
	return 0
}

func runRuntimeCollisionRoots(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal runtime collision-roots")
		return 2
	}
	for _, root := range runtimes.CollisionRootsAll() {
		fmt.Println(root)
	}
	return 0
}

func runRuntimeDirs(args []string) int {
	declaration, code := runtimeArg(args, "dirs")
	if code != 0 {
		return code
	}
	for _, dir := range declaration.RegistrationDirs {
		fmt.Println(dir)
	}
	return 0
}

func runRuntimeRegistration(args []string) int {
	declaration, code := runtimeArg(args, "registration")
	if code != 0 {
		return code
	}
	fmt.Print(runtimes.RegistrationV1(declaration.Name))
	return 0
}
