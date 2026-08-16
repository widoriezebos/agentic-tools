package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The runtime family is the shell's ONLY window onto the runtime
// registry (agnosticism audit, D74 phase A): the declaration lives in
// Go, plumbing asks the binary. Exit codes are pinned: 0 ok, 1 unknown
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
		fmt.Fprintln(os.Stderr, "usage: metasystem runtime list [--adoptable|--with-adapter|--with-host|--with-common-lifecycle]")
		return 2
	}
	for _, name := range names {
		fmt.Println(name)
	}
	return 0
}

func runRuntimeSignatureVectors(args []string) int {
	declaration, code := runtimeArg(args, "signature-vectors")
	if code != 0 {
		return code
	}
	if declaration.SignatureVectors.Positive == "" {
		fmt.Fprintln(os.Stderr, "no signature vectors declared for "+declaration.Name)
		return 1
	}
	payload, _ := json.Marshal(map[string]string{
		"positive":  declaration.SignatureVectors.Positive,
		"lookalike": declaration.SignatureVectors.Lookalike,
	})
	fmt.Println(string(payload))
	return 0
}

func runRuntimeCollisionRoots(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem runtime collision-roots")
		return 2
	}
	for _, root := range runtimes.CollisionRootsAll() {
		fmt.Println(root)
	}
	return 0
}

func runRuntimeEnforcementMap(args []string) int {
	declaration, code := runtimeArg(args, "enforcement-map")
	if code != 0 {
		return code
	}
	if declaration.ExpectedEnvelopeEnforcement == nil {
		fmt.Fprintln(os.Stderr, "no static enforcement map declared for "+declaration.Name)
		return 1
	}
	ordered := map[string]string{}
	for field, value := range declaration.ExpectedEnvelopeEnforcement {
		ordered[field] = string(value)
	}
	payload, _ := json.Marshal(ordered)
	fmt.Println(string(payload))
	return 0
}

func runRuntimeAdoptionDefault(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem runtime adoption-default")
		return 2
	}
	fmt.Println(runtimes.AdoptionDefault())
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

// singleValue prints a declared value, or refuses with exit 1 and
// empty output when the runtime does not declare the capability — the
// pinned absent semantics every shell consumer relies on.
func singleValue(value, absentNote string) int {
	if value == "" {
		fmt.Fprintln(os.Stderr, absentNote)
		return 1
	}
	fmt.Println(value)
	return 0
}

func runRuntimeEnforcementConfig(args []string) int {
	declaration, code := runtimeArg(args, "enforcement-config")
	if code != 0 {
		return code
	}
	return singleValue(declaration.ShippedEnforcementConfig,
		"no shipped enforcement config declared for "+declaration.Name)
}

func runRuntimeSelfCheck(args []string) int {
	declaration, code := runtimeArg(args, "self-check")
	if code != 0 {
		return code
	}
	if declaration.SelfCheck == nil {
		fmt.Fprintln(os.Stderr, "no live self-check declared for "+declaration.Name)
		return 1
	}
	fmt.Println(declaration.SelfCheck.VendoredMarker)
	return 0
}

func runRuntimeInstructionFile(args []string) int {
	declaration, code := runtimeArg(args, "instruction-file")
	if code != 0 {
		return code
	}
	return singleValue(declaration.InstructionFile,
		"no instruction file declared for "+declaration.Name)
}

func runRuntimeSessionEnv(args []string) int {
	declaration, code := runtimeArg(args, "session-env")
	if code != 0 {
		return code
	}
	return singleValue(declaration.SessionEnv,
		"no session environment declared for "+declaration.Name)
}

func runRuntimeConfigIdentityFilter(args []string) int {
	declaration, code := runtimeArg(args, "config-identity-filter")
	if code != 0 {
		return code
	}
	return singleValue(declaration.ConfigIdentityFilter,
		"no config identity filter declared for "+declaration.Name)
}

func runRuntimeRegistration(args []string) int {
	declaration, code := runtimeArg(args, "registration")
	if code != 0 {
		return code
	}
	fmt.Print(runtimes.RegistrationV1(declaration.Name))
	return 0
}
