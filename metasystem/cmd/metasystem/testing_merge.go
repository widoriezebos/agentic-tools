package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

const testingMergeDriverUsage = `usage: metasystem internal testing merge-driver BASE OURS THEIRS

Git passes %O %A %B as BASE OURS THEIRS. The driver writes the merge to OURS.
It is git's merge driver for the testing contract, not a command for people
or agents: metasystem system setup registers it.`

func runTestingMergeDriver(args []string) int {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return refuseUnknownOption(nil, "testing merge-driver", arg, "it takes BASE OURS THEIRS, the three paths git passes")
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, testingMergeDriverUsage)
		return 2
	}
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, testingMergeDriverUsage)
		return 2
	}
	if err := mergeTestingFiles(args[0], args[1], args[2], args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem internal testing merge-driver:", err)
		return 1
	}
	return 0
}

func mergeTestingFiles(basePath, oursPath, theirsPath, outPath string) error {
	inputs := make([][]byte, 3)
	for i, path := range []string{basePath, oursPath, theirsPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		inputs[i] = data
	}
	merged, err := contractmerge.MergeBytes(inputs[0], inputs[1], inputs[2])
	if err != nil {
		return err
	}
	return writeTestingContract(outPath, merged)
}

func runTestingAddTests(args []string) int {
	return editTestingContract("add", args, testingContractEdits{
		tests: func(contract testpolicy.Contract, path, group string, tests []string) (testpolicy.Contract, error) {
			return contractmerge.AddTests(contract, path, group, tests)
		},
		inputs: contractmerge.AddInputs,
		paths:  contractmerge.AddSurfacePaths,
	})
}

// runTestingRemoveTests takes named tests, inputs or surface paths out of the
// contract, or a whole group or surface: the follow-up of deleting or moving
// what they name.
func runTestingRemoveTests(args []string) int {
	return editTestingContract("remove", args, testingContractEdits{
		tests: func(contract testpolicy.Contract, _ string, group string, tests []string) (testpolicy.Contract, error) {
			return contractmerge.RemoveTests(contract, group, tests)
		},
		group:   contractmerge.RemoveGroup,
		inputs:  contractmerge.RemoveInputs,
		paths:   contractmerge.RemoveSurfacePaths,
		surface: contractmerge.RemoveSurface,
	})
}

// testingContractEdits are one action's edits, one per form; a nil edit is a
// form the action does not have.
type testingContractEdits struct {
	tests   func(testpolicy.Contract, string, string, []string) (testpolicy.Contract, error)
	group   func(testpolicy.Contract, string) (testpolicy.Contract, error)
	inputs  func(testpolicy.Contract, string, []string) (testpolicy.Contract, error)
	paths   func(testpolicy.Contract, string, []string) (testpolicy.Contract, error)
	surface func(testpolicy.Contract, string) (testpolicy.Contract, error)
}

// editTestingContract is test add and test remove: decode the contract, apply
// one group or surface edit, render and write it back.
func editTestingContract(action string, args []string, edits testingContractEdits) int {
	flags := newFlagSet("test " + action)
	path := flags.String("file", "", "testing contract to edit")
	group := flags.String("group", "", "group id")
	surface := flags.String("surface", "", "surface id")
	tests := flags.String("tests", "", "comma-separated Go test names")
	inputs := flags.String("inputs", "", "comma-separated group input paths")
	paths := flags.String("paths", "", "comma-separated surface paths")
	list := func(value string) []string {
		if value == "" {
			return nil
		}
		return strings.Split(value, ",")
	}
	var edit func(testpolicy.Contract) (testpolicy.Contract, error)
	if flags.Parse(args) == nil && flags.NArg() == 0 && *path != "" {
		switch {
		case *group != "" && *surface == "" && *paths == "" && *tests != "" && *inputs == "":
			edit = func(c testpolicy.Contract) (testpolicy.Contract, error) {
				return edits.tests(c, *path, *group, list(*tests))
			}
		case *group != "" && *surface == "" && *paths == "" && *tests == "" && *inputs != "" && edits.inputs != nil:
			edit = func(c testpolicy.Contract) (testpolicy.Contract, error) {
				return edits.inputs(c, *group, list(*inputs))
			}
		case *group != "" && *surface == "" && *paths == "" && *tests == "" && *inputs == "" && edits.group != nil:
			edit = func(c testpolicy.Contract) (testpolicy.Contract, error) { return edits.group(c, *group) }
		case *surface != "" && *group == "" && *tests == "" && *inputs == "" && *paths != "" && edits.paths != nil:
			edit = func(c testpolicy.Contract) (testpolicy.Contract, error) {
				return edits.paths(c, *surface, list(*paths))
			}
		case *surface != "" && *group == "" && *tests == "" && *inputs == "" && *paths == "" && edits.surface != nil:
			edit = func(c testpolicy.Contract) (testpolicy.Contract, error) { return edits.surface(c, *surface) }
		}
	}
	if edit == nil {
		fmt.Fprintf(os.Stderr, "usage: metasystem test %s --file FILE --group ID --tests NAME,NAME | --group ID --inputs PATH,PATH | --surface ID --paths PATH,PATH\n", action)
		return 2
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metasystem test %s: %v\n", action, err)
		return 1
	}
	contract, err := testpolicy.Decode(data)
	if err == nil {
		contract, err = edit(contract)
	}
	if err == nil {
		data, err = contractmerge.Render(contract)
	}
	if err == nil {
		_, err = testpolicy.Decode(data)
	}
	if err == nil {
		err = writeTestingContract(*path, data)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "metasystem test %s: %v\n", action, err)
		return 1
	}
	return 0
}

func writeTestingContract(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, data, mode)
}
