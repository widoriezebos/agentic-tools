//go:build darwin

package proofrun

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type darwinProcessTreeReader struct{}

func newProcessTreeReader() processTreeReader { return &darwinProcessTreeReader{} }

func (*darwinProcessTreeReader) ProgressRuleSuffix() string { return "" }

// Sample reads the tree natively (R-138-m1e: Go decides natively; it once
// listed `ps`): parents, start identities and stop status from one kernel
// process-table snapshot, then each member's uninterruptible-wait state and
// own CPU from the kernel. A member that exits between the snapshot and its
// reads makes the sample partial, as a member missing from ps's CPU listing
// did.
func (*darwinProcessTreeReader) Sample(rootPID int) (processTreeSample, error) {
	table, err := identity.TakeProcessTable()
	if err != nil {
		return processTreeSample{}, fmt.Errorf("read Darwin process tree: %w", err)
	}
	parents := map[int]int{}
	entries := map[int]identity.ProcessEntry{}
	for _, entry := range table {
		parents[int(entry.Pid)] = int(entry.Parent)
		entries[int(entry.Pid)] = entry
	}
	members := descendantProcessIDs(rootPID, parents)
	sample := processTreeSample{Members: append([]int(nil), members...), MemberCPU: make([]processMemberCPU, 0, len(members)), RetainVanishedMembers: true}
	for _, pid := range members {
		entry, listed := entries[pid]
		if !listed {
			sample.Partial = true
			continue
		}
		sample.Stopped = sample.Stopped || entry.Stopped
		if waiting, waitErr := identity.ProcessUninterruptible(int64(pid)); waitErr == nil {
			sample.Waiting = sample.Waiting || waiting
		}
		seconds, cpuErr := identity.ProcessCPUSeconds(int64(pid))
		if cpuErr != nil {
			if errors.Is(cpuErr, identity.ErrNoSuchProcess) {
				sample.Partial = true
				continue
			}
			return processTreeSample{}, fmt.Errorf("read Darwin process CPU: %w", cpuErr)
		}
		started := strconv.FormatInt(entry.Started.UnixMicro(), 10)
		sample.MemberCPU = append(sample.MemberCPU, processMemberCPU{PID: pid, Started: started, CPUSeconds: seconds})
	}
	return sample, nil
}

func descendantProcessIDs(rootPID int, parents map[int]int) []int {
	selected := map[int]bool{rootPID: true}
	changed := true
	for changed {
		changed = false
		for pid, ppid := range parents {
			if selected[ppid] && !selected[pid] {
				selected[pid], changed = true, true
			}
		}
	}
	result := make([]int, 0, len(selected))
	for pid := range selected {
		result = append(result, pid)
	}
	return uniqueProcessIDs(result, 0)
}
