// Package contractgit supplies the temporary Git configuration that makes
// testing contract merges use the metasystem merge driver.
package contractgit

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	driverName = "merge.metasystem-testing.name=metasystem testing.json merge by surface"
	driverKey  = "merge.metasystem-testing.driver="
)

// DriverArgs returns command-line Git configuration for the running
// metasystem binary. An unreadable executable leaves Git's default merge in
// place so a missing driver never turns an otherwise valid operation into a
// refusal.
func DriverArgs(executable func() (string, error)) []string {
	path, err := executable()
	if err != nil || path == "" {
		return nil
	}
	path, err = filepath.Abs(path)
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err != nil {
		return nil
	}
	command := shellQuote(path) + " testing merge-driver %O %A %B"
	return []string{"-c", driverName, "-c", driverKey + command}
}

// RuntimeDriverArgs resolves the current process only when a Git invocation
// is about to be assembled, which keeps the executable source injectable in
// focused tests.
func RuntimeDriverArgs() []string { return DriverArgs(os.Executable) }

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
