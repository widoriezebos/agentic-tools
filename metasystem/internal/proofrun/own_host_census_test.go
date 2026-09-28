package proofrun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// A batch owner counts its own runs itself; the census must name the part of
// its host count that is the owner's own children, or each own run counts
// twice and a ceiling of three admits two.
func TestSampleLoadNamesTheSlotsHeldByTheCallersOwnFamily(t *testing.T) {
	previousDirectory, previousReaders, previousOptions := hostAdmissionDirectoryForTest, loadSeams, commandLoadOptions
	t.Cleanup(func() {
		hostAdmissionDirectoryForTest, loadSeams, commandLoadOptions = previousDirectory, previousReaders, previousOptions
	})
	commandLoadOptions = nil
	directory := filepath.Join(t.TempDir(), "host-admission")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	hostAdmissionDirectoryForTest = directory
	const self, child, grandchild, foreign = 500, 501, 502, 601
	parents := map[int64]int64{self: 1, child: self, grandchild: child, foreign: 1}
	loadSeams.host = func(time.Time) hostload.Sample { return hostload.Sample{Available: true, Cores: 16, Load1m: 1} }
	loadSeams.launchers = func(int64) (int, bool) { return 0, true }
	loadSeams.fixtureNamespaceLaunchers = loadSeams.launchers
	loadSeams.prober = &censusProber{processes: map[int64]identity.Exact{}, calls: map[int64]int{}}
	loadSeams.pids = func() ([]int64, error) { return []int64{1, self, child, grandchild, foreign}, nil }
	loadSeams.parent = func(pid int64) (int64, bool) { parent, ok := parents[pid]; return parent, ok }

	var held []*os.File
	t.Cleanup(func() { closeHostFiles(held) })
	lease := func(index int, owner int64, cleared bool) {
		t.Helper()
		slot, acquired, err := tryHostFile(filepath.Join(directory, "slot-0"+string(rune('0'+index))))
		if err != nil || !acquired {
			t.Fatalf("hold slot %d: acquired=%t err=%v", index, acquired, err)
		}
		held = append(held, slot)
		exact := identity.Exact{Pid: owner, StartedAt: time.UnixMicro(1790530358214211)}
		if runtime.GOOS == "linux" {
			exact.StartTicks, exact.BootID = 1, "boot"
		}
		encoded, err := json.Marshal(hostLeaseRecord{Schema: 1, Owner: processIdentity(exact, owner), Class: "heavy", Slot: "slot-0" + string(rune('0'+index)), Resources: []string{}, Cleared: cleared})
		if err != nil {
			t.Fatal(err)
		}
		// A live owner holds its marker's flock, as AcquireHostResources does.
		marker, acquired, err := tryHostFile(filepath.Join(directory, "lease-heavy-"+strings.Repeat(string(rune('a'+index)), 32)))
		if err != nil || !acquired {
			t.Fatalf("hold marker %d: acquired=%t err=%v", index, acquired, err)
		}
		held = append(held, marker)
		if _, err := marker.WriteAt(encoded, 0); err != nil {
			t.Fatal(err)
		}
	}
	lease(0, child, false)
	lease(1, grandchild, false)
	lease(2, foreign, false)
	sample := sampleLoad(t.TempDir(), "", self, time.Unix(7, 0))
	if !sample.OverlapKnown || sample.OverlappingHost != 3 || sample.OwnHost != 2 {
		t.Fatalf("sample=%+v, want 3 slots on the host of which 2 are the caller's own children", sample)
	}
	if other := sampleLoad(t.TempDir(), "", foreign, time.Unix(7, 0)); other.OwnHost != 1 {
		t.Fatalf("a different caller's own share = %d, want its one slot", other.OwnHost)
	}
	if none := sampleLoad(t.TempDir(), "", 0, time.Unix(7, 0)); none.OwnHost != 0 {
		t.Fatalf("a caller without a pid claimed %d own slots", none.OwnHost)
	}
}
