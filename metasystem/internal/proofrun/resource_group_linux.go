//go:build linux

package proofrun

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func custodyGroupMembers(group int64) ([]int64, error) {
	pids, err := identity.AllPids()
	if err != nil {
		return nil, fmt.Errorf("resource group membership is indeterminable: %w", err)
	}
	var members []int64
	for _, pid := range pids {
		if pid == group {
			continue
		}
		pgid, err := syscall.Getpgid(int(pid))
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("resource group membership is indeterminable at pid %d: %w", pid, err)
		}
		if int64(pgid) == group {
			members = append(members, pid)
		}
	}
	return members, nil
}
