//go:build linux

package proofrun

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// custodyGroupMembers lists the members of group in table, the leader
// omitted; unknown membership is an error.
func custodyGroupMembers(table identity.ProcessTable, group int64) ([]int64, error) {
	return tableGroupMembers(table, group)
}
