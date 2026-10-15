package commandenv

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// EngineEnvironment gives declared commands the running engine before the
// caller's search path. The supplied environment remains unchanged.
func EngineEnvironment(environment []string) []string {
	if environment == nil {
		environment = os.Environ()
	}
	executable, err := os.Executable()
	if err != nil {
		return slices.Clone(environment)
	}
	return EngineEnvironmentInDirectory(environment, filepath.Dir(executable))
}

// EngineEnvironmentInDirectory prefixes PATH with the resolved engine directory.
// A frozen command records this environment and reuses it without resolving again.
func EngineEnvironmentInDirectory(environment []string, directory string) []string {
	if environment == nil {
		environment = os.Environ()
	}
	if directory == "" {
		return slices.Clone(environment)
	}
	// An environment without a PATH is left as it is: a declared explicit environment that names no
	// PATH keeps the shell's own default search and never gains an empty entry (the working directory).
	// With a PATH (the last one wins when it is repeated), the engine's directory goes first.
	path, found := "", false
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path, found = value, true
		} else {
			result = append(result, entry)
		}
	}
	if !found {
		return slices.Clone(environment)
	}
	return append(result, "PATH="+directory+string(os.PathListSeparator)+path)
}

// InstallationEnvironment binds lane children to the registered installation.
func InstallationEnvironment(environment []string, install string) ([]string, error) {
	directory := filepath.Join(install, "bin")
	binary := filepath.Join(directory, "metasystem")
	info, err := os.Stat(binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("the installation's engine %s is missing or is not executable; no command was started", binary)
	}
	for _, entry := range environment {
		if strings.HasPrefix(entry, "PATH=") {
			return EngineEnvironmentInDirectory(environment, directory), nil
		}
	}
	return append(slices.Clone(environment), "PATH="+directory+string(os.PathListSeparator)+"/usr/bin:/bin"), nil
}
