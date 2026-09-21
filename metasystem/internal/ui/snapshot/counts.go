package snapshot

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The working tree's goal files are counted only where no accepted tip can be
// read, and only to say how much the checkout holds while the projection
// cannot be built. No goal is ever read from them: the accepted tip is the
// world this interface projects, and a count of files beside it is a fact
// about the checkout, labelled as such.

// goalFileCount counts the goal files directly under the given repository
// directory. A directory that cannot be listed gives no number rather than
// zero: "none" and "not known" are different answers, and only one of them
// would be a lie about a checkout full of records.
func goalFileCount(stateRoot, relative string, skip string) *int {
	entries, err := os.ReadDir(filepath.Join(stateRoot, filepath.FromSlash(relative)))
	if err != nil {
		return nil
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == skip {
			continue
		}
		count++
	}
	return &count
}

// workingTreeCounts reports how many goal files and archived goal records the
// checkout carries. The root record is not a goal and is not counted.
func workingTreeCounts(stateRoot string) (live, archived *int) {
	goalsRoot, err := stateroot.RelativeRoot(stateroot.Goals)
	if err == nil {
		live = goalFileCount(stateRoot, goalsRoot, "backlog.md")
	}
	recordsRoot, err := stateroot.RelativeRoot(stateroot.Records)
	if err == nil {
		archived = goalFileCount(stateRoot, path.Join(recordsRoot, "goals"), "")
	}
	return live, archived
}
