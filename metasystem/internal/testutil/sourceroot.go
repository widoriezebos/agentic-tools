package testutil

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ModulePath is the metasystem module's import path, the prefix a
// -trimpath build records on every source file of this module.
const ModulePath = "github.com/widoriezebos/agentic-tools/metasystem"

// SourceRoot names the metasystem module root: FIXTURE_SOURCE_ROOT when set,
// else the nearest ancestor of the working directory (the package directory
// under go test) whose go.mod declares ModulePath. It never reads
// runtime.Caller, whose file names are module-relative under -trimpath.
func SourceRoot() (string, error) {
	working, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return resolveSourceRoot(os.Getenv("FIXTURE_SOURCE_ROOT"), working)
}

func resolveSourceRoot(override, working string) (string, error) {
	if override != "" {
		return override, nil
	}
	for dir := working; ; dir = filepath.Dir(dir) {
		if declaresModule(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", errors.New("no go.mod declaring " + ModulePath + " above " + working)
		}
	}
}

func declaresModule(goMod string) bool {
	file, err := os.Open(goMod)
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`) == ModulePath
		}
	}
	return false
}

// MustSourceRoot is SourceRoot for tests: it fails the test when the root
// cannot be found.
func MustSourceRoot(t testing.TB) string {
	t.Helper()
	root, err := SourceRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
