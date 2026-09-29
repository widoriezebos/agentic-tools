package contractgit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// The outcomes of a merge-driver registration.
const (
	DriverRegistered = "registered"
	DriverUnchanged  = "unchanged"
	DriverNoGit      = "no-git"
)

// driverSectionName is the driver's human name in git configuration.
const driverSectionName = "metasystem testing.json merge by surface"

// DriverCommand is the merge driver git runs for the testing contract: the
// engine's internal entrypoint with git's three version paths.
func DriverCommand(engine string) string {
	return shellquote.Quote(engine) + " testing merge-driver %O %A %B"
}

// AttributeLine is the .gitattributes line that routes the contract at
// contract (a slash path relative to the repository root) to the driver.
func AttributeLine(contract string) string {
	return contract + " merge=" + testingContractMergeDriver
}

// Registration is what a repository still lacks for its testing contract to
// merge through the engine's driver; empty when it lacks nothing.
func Registration(root, contract, engine string, git func(args ...string) (string, error)) ([]string, error) {
	var missing []string
	attributes, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if !hasLine(string(attributes), AttributeLine(contract)) {
		missing = append(missing, fmt.Sprintf(".gitattributes does not route %s to the testing merge driver", contract))
	}
	for key, want := range map[string]string{"name": driverSectionName, "driver": DriverCommand(engine)} {
		got, _ := git("-C", root, "config", "--local", "--get", "merge."+testingContractMergeDriver+"."+key)
		if strings.TrimSpace(got) != want {
			missing = append(missing, fmt.Sprintf("git config merge.%s.%s is not %q", testingContractMergeDriver, key, want))
		}
	}
	return missing, nil
}

// Register makes the repository at root merge its testing contract through
// the engine's driver: the .gitattributes line and the local git
// configuration. A repeat whose registration holds writes nothing.
func Register(root, contract, engine string, git func(args ...string) (string, error)) (string, error) {
	if _, err := git("-C", root, "rev-parse", "--git-dir"); err != nil {
		return DriverNoGit, nil
	}
	missing, err := Registration(root, contract, engine, git)
	if err != nil || len(missing) == 0 {
		return DriverUnchanged, err
	}
	path := filepath.Join(root, ".gitattributes")
	attributes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if line := AttributeLine(contract); !hasLine(string(attributes), line) {
		text := string(attributes)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if _, err := atomicfile.WriteText(path, text+line+"\n", ""); err != nil {
			return "", err
		}
	}
	for key, value := range map[string]string{"name": driverSectionName, "driver": DriverCommand(engine)} {
		if _, err := git("-C", root, "config", "--local", "merge."+testingContractMergeDriver+"."+key, value); err != nil {
			return "", err
		}
	}
	return DriverRegistered, nil
}

func hasLine(text, line string) bool {
	for _, candidate := range strings.Split(text, "\n") {
		if strings.TrimSpace(candidate) == line {
			return true
		}
	}
	return false
}
