package goal_test

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func init() {
	goal.AnnounceHolderForTest = func(root, session string, pid, start, startTicks int64, bootID, tag, runtime, lineage string) error {
		_, err := lease.AnnounceWithPair(root, session, pid, start, startTicks, bootID, tag, runtime, lineage)
		return err
	}
}
