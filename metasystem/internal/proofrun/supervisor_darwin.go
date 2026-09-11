//go:build darwin

package proofrun

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type darwinProcessTreeReader struct{}

func newProcessTreeReader() processTreeReader { return &darwinProcessTreeReader{} }

func (*darwinProcessTreeReader) ProgressRuleSuffix() string { return "" }

func (*darwinProcessTreeReader) Sample(rootPID int) (processTreeSample, error) {
	treeOutput, err := exec.Command("ps", "-axo", "pid=,ppid=,state=,lstart=").Output()
	if err != nil {
		return processTreeSample{}, fmt.Errorf("read Darwin process tree: %w", err)
	}
	parents := map[int]int{}
	states := map[int]string{}
	started := map[int]string{}
	for lineNumber, line := range strings.Split(string(treeOutput), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 8 {
			return processTreeSample{}, fmt.Errorf("parse Darwin process tree line %d", lineNumber+1)
		}
		pid, pidErr := strconv.Atoi(fields[0])
		ppid, ppidErr := strconv.Atoi(fields[1])
		if pidErr != nil || ppidErr != nil {
			return processTreeSample{}, fmt.Errorf("parse Darwin process identifiers on line %d", lineNumber+1)
		}
		parents[pid], states[pid], started[pid] = ppid, fields[2], strings.Join(fields[3:], " ")
	}
	members := descendantProcessIDs(rootPID, parents)
	memberText := make([]string, len(members))
	sample := processTreeSample{Members: append([]int(nil), members...), MemberCPU: make([]processMemberCPU, 0, len(members)), RetainVanishedMembers: true}
	for index, pid := range members {
		memberText[index] = strconv.Itoa(pid)
		state := states[pid]
		sample.Stopped = sample.Stopped || strings.ContainsRune(state, 'T')
		sample.Waiting = sample.Waiting || strings.ContainsRune(state, 'U')
	}
	cpuOutput, cpuErr := exec.Command("ps", "-S", "-o", "pid=,cputime=", "-p", strings.Join(memberText, ",")).Output()
	if cpuErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cpuErr, &exitErr) && len(strings.TrimSpace(string(cpuOutput))) == 0 {
			sample.Partial = true
			return sample, nil
		}
		return processTreeSample{}, fmt.Errorf("read Darwin process CPU: %w", cpuErr)
	}
	read := 0
	for _, line := range strings.Split(string(cpuOutput), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return processTreeSample{}, fmt.Errorf("parse Darwin process CPU line %q", line)
		}
		pid, pidErr := strconv.Atoi(fields[0])
		if pidErr != nil {
			return processTreeSample{}, fmt.Errorf("parse Darwin process CPU identifier %q", fields[0])
		}
		seconds, parseErr := parseCPUTime(fields[1])
		if parseErr != nil {
			return processTreeSample{}, parseErr
		}
		sample.MemberCPU = append(sample.MemberCPU, processMemberCPU{PID: pid, Started: started[pid], CPUSeconds: seconds})
		read++
	}
	if read < len(members) {
		sample.Partial = true
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
