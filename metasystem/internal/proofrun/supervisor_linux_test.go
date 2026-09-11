//go:build linux

package proofrun

import (
	"encoding/binary"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPlatformProcessReaderContract(t *testing.T) {
	wordBytes := strconv.IntSize / 8
	data := make([]byte, wordBytes*6)
	put := func(offset int, value uint64) {
		if wordBytes == 8 {
			binary.NativeEndian.PutUint64(data[offset:offset+wordBytes], value)
		} else {
			binary.NativeEndian.PutUint32(data[offset:offset+wordBytes], uint32(value))
		}
	}
	put(0, linuxATClockTicks)
	put(wordBytes, 250)
	put(wordBytes*2, 0)
	ticks, err := parseAuxClockTicks(data)
	if err != nil || ticks != 250 {
		t.Fatalf("AT_CLKTCK = %.0f, err=%v", ticks, err)
	}
	fields := []string{"T", "7"}
	for len(fields) < 11 {
		fields = append(fields, "0")
	}
	fields = append(fields, "11", "12", "13", "14")
	for len(fields) < 19 {
		fields = append(fields, "0")
	}
	fields = append(fields, "99")
	stat, err := parseLinuxProcessStat([]byte("42 (command with ) delimiter) " + strings.Join(fields, " ")))
	if err != nil || stat.ppid != 7 || stat.state != "T" || stat.ownCPUTicks != 23 || stat.reapedChildrenCPUTicks != 27 || stat.startTicks != 99 {
		t.Fatalf("Linux stat = %+v, err=%v", stat, err)
	}
	if memberTicks := linuxGroupMemberCPUTicks(stat); memberTicks != 50 {
		t.Fatalf("Linux live member ticks = %d, want 50", memberTicks)
	}
	if !linuxProcessVanished(os.ErrNotExist) || !linuxProcessVanished(syscall.ESRCH) || linuxProcessVanished(errors.New("other")) {
		t.Fatal("Linux vanished-process errors are not classified as partial samples")
	}
	if suffix := (&linuxProcessTreeReader{ticks: linuxUserHZ, assumed: true}).ProgressRuleSuffix(); suffix != "ticks/assumed-100" {
		t.Fatalf("assumed clock-tick rule suffix = %q", suffix)
	}
	parents := map[int]int{100: 1, 101: 100, 102: 101, 200: 1}
	got := descendantProcessIDs(100, parents)
	if len(got) != 3 || got[0] != 100 || got[1] != 101 || got[2] != 102 {
		t.Fatalf("synthetic descendant tree = %v", got)
	}
}

func testPlatformReapedChildCPU(t *testing.T) {
	reader := availableProcessTreeReader(t)
	helper := newSupervisorHelperFixture(t, "reaped-child")
	observedCPU, beforeNonRootReap, afterNonRootReap := float64(0), float64(0), float64(0)
	releasedGrandchild, observedNonRootReap, releasedNested, observedRootReap, counterDropped := false, false, false, false, false
	options := supervisorOptionsForTest(t, 10, 400*time.Millisecond)
	options.Limits.ZeroConsumptionWindow = 0
	helper.gate(&options, reader, func(_ int, sample processTreeSample) {
		cpu := sampleMemberCPUTotal(sample)
		if cpu > observedCPU {
			observedCPU = cpu
		}
		switch {
		case !releasedGrandchild && len(sample.MemberCPU) >= 3 && cpu >= 0.25:
			beforeNonRootReap = cpu
			releasedGrandchild = true
			helper.releaseGrandchild()
		case releasedGrandchild && !observedNonRootReap && len(sample.MemberCPU) == 2:
			afterNonRootReap = cpu
			counterDropped = cpu+1e-9 < beforeNonRootReap
			observedNonRootReap = true
			releasedNested = true
			helper.releaseNested()
		case releasedNested && len(sample.MemberCPU) == 1:
			counterDropped = counterDropped || cpu+1e-9 < afterNonRootReap
			observedRootReap = true
			helper.closeInput()
		}
	})
	outcome := superviseCommand(helper.command, options)
	if outcome.Verdict != "" || outcome.WaitErr != nil || !observedNonRootReap || !observedRootReap || counterDropped || observedCPU < 0.25 || outcome.CPUSeconds < 0.25 {
		t.Fatalf("Linux nested reap accounting outcome=%+v observedCPU=%.2f beforeNonRoot=%.2f afterNonRoot=%.2f observedNonRoot=%v observedRoot=%v dropped=%v", outcome, observedCPU, beforeNonRootReap, afterNonRootReap, observedNonRootReap, observedRootReap, counterDropped)
	}
}

func processStoppedForTest(pid int) (bool, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	stat, err := parseLinuxProcessStat(data)
	if err != nil {
		return false, err
	}
	return stat.state == "T" || stat.state == "t", nil
}
