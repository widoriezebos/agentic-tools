package diskstore

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const testMemoryPrefix = "mst-"
const testRegistryPrefix = "metasystem-test-registry-"

// InTestRegistryHome recognizes the reserved test registry name in any
// component of a cleaned path, including homes under a fallback TMPDIR.
// It compares spellings without filesystem I/O.
func InTestRegistryHome(path string) bool {
	for _, component := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if strings.HasPrefix(component, testRegistryPrefix) {
			return true
		}
	}
	return false
}

// OnTestMemoryRoot reports whether path is on the current user's throwaway
// test memory root. It compares cleaned spellings without filesystem I/O;
// macOS also spells /tmp as /private/tmp.
func OnTestMemoryRoot(path string) bool {
	path = filepath.Clean(path)
	root := "/tmp/" + testMemoryPrefix + strconv.Itoa(os.Geteuid())
	for _, spelling := range []string{root, "/private" + root} {
		if path == spelling || strings.HasPrefix(path, spelling+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
