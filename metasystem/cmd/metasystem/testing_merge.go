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
		fmt.Fprintln(os.Stderr, "metasystem testing merge-driver:", err)
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
	return editTestingContract("add", args, func(contract testpolicy.Contract, path, group string, tests []string) (testpolicy.Contract, error) {
		return contractmerge.AddTests(contract, path, group, tests)
	})
}

// runTestingRemoveTests takes named tests out of a group, the follow-up of
// deleting them.
func runTestingRemoveTests(args []string) int {
	return editTestingContract("remove", args, func(contract testpolicy.Contract, _ string, group string, tests []string) (testpolicy.Contract, error) {
		if len(tests) == 0 {
			return contractmerge.RemoveGroup(contract, group)
		}
		return contractmerge.RemoveTests(contract, group, tests)
	})
}

// editTestingContract is test add and test remove: decode the contract, apply
// one group edit, render and write it back.
func editTestingContract(action string, args []string, edit func(testpolicy.Contract, string, string, []string) (testpolicy.Contract, error)) int {
	flags := newFlagSet("test " + action)
	path := flags.String("file", "", "testing contract to edit")
	group := flags.String("group", "", "group id")
	tests := flags.String("tests", "", "comma-separated Go test names")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *group == "" || *tests == "" && action != "remove" {
		fmt.Fprintf(os.Stderr, "usage: metasystem test %s --file FILE --group ID --tests NAME,NAME\n", action)
		return 2
	}
	names := []string{}
	if *tests != "" {
		names = strings.Split(*tests, ",")
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metasystem test %s: %v\n", action, err)
		return 1
	}
	contract, err := testpolicy.Decode(data)
	if err == nil {
		contract, err = edit(contract, *path, *group, names)
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
