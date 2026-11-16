package diskstore

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const testMemoryPrefix = "mst-"

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
