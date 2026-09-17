package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

const testingMergeDriverUsage = `usage: metasystem testing merge-driver BASE OURS THEIRS

Git passes %O %A %B as BASE OURS THEIRS. The driver writes the merge to OURS.
metasystem/testing.json merge=metasystem-testing
git config merge.metasystem-testing.driver 'metasystem testing merge-driver %O %A %B'`

func runTestingMerge(args []string) int {
	flags := flag.NewFlagSet("testing merge", flag.ContinueOnError)
	base := flags.String("base", "", "merge-base testing contract")
	ours := flags.String("ours", "", "current-side testing contract")
	theirs := flags.String("theirs", "", "incoming-side testing contract")
	out := flags.String("out", "", "merged output path")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *base == "" || *ours == "" || *theirs == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem testing merge --base FILE --ours FILE --theirs FILE --out FILE")
		return 2
	}
	if err := mergeTestingFiles(*base, *ours, *theirs, *out); err != nil {
		fmt.Fprintln(os.Stderr, "metasystem testing merge:", err)
		return 1
	}
	return 0
}

func runTestingMergeDriver(args []string) int {
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
	flags := flag.NewFlagSet("testing add-tests", flag.ContinueOnError)
	path := flags.String("file", "", "testing contract to edit")
	group := flags.String("group", "", "group id")
	tests := flags.String("tests", "", "comma-separated Go test names")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *path == "" || *group == "" || *tests == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem testing add-tests --file FILE --group ID --tests NAME,NAME")
		return 2
	}
	data, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "metasystem testing add-tests:", err)
		return 1
	}
	contract, err := testpolicy.Decode(data)
	if err == nil {
		contract, err = contractmerge.AddTests(contract, *path, *group, strings.Split(*tests, ","))
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
		fmt.Fprintln(os.Stderr, "metasystem testing add-tests:", err)
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
