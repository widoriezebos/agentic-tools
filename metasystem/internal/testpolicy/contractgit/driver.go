// Package contractgit supplies the temporary Git configuration that makes
// testing contract merges use the metasystem merge driver.
package contractgit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	driverName                 = "merge.metasystem-testing.name=metasystem testing.json merge by surface"
	driverKey                  = "merge.metasystem-testing.driver="
	DriverUnresolvedCode       = "TESTING_MERGE_DRIVER_UNRESOLVED"
	AttributeMisuseCode        = "TESTING_MERGE_ATTRIBUTE_MISUSE"
	ContractHandMergedCode     = "TESTING_CONTRACT_HAND_MERGED"
	TestingContractPath        = "metasystem/testing.json"
	testingContractMergeDriver = "metasystem-testing"
)

// Refusal reports a transition that cannot safely enter a content merge.
type Refusal struct {
	Code, Detail string
}

func (r *Refusal) Error() string { return fmt.Sprintf("%s: %s", r.Code, r.Detail) }

// DriverArgs returns command-line Git configuration for the running
// metasystem binary. Content-merging verbs must stop when the executable
// cannot be resolved because Git's text merge does not preserve the testing
// contract's semantic identities.
func DriverArgs(executable func() (string, error)) ([]string, error) {
	path, err := executable()
	if err != nil || path == "" {
		return nil, unresolved(err)
	}
	path, err = filepath.Abs(path)
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err != nil {
		return nil, unresolved(err)
	}
	command := shellQuote(path) + " testing merge-driver %O %A %B"
	return []string{"-c", driverName, "-c", driverKey + command}, nil
}

// RuntimeDriverArgs resolves the current process only when a Git invocation
// is about to be assembled, which keeps the executable source injectable in
// focused tests.
func RuntimeDriverArgs() ([]string, error) { return DriverArgs(os.Executable) }

func unresolved(cause error) error {
	detail := "cannot resolve the metasystem merge-driver executable; run the verb from an installed metasystem binary"
	if cause != nil {
		detail += ": " + cause.Error()
	}
	return &Refusal{Code: DriverUnresolvedCode, Detail: detail}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
