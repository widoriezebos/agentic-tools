package proofrun

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func admissionReadPath(directory string) func() (string, bool, error) {
	return func() (string, bool, error) { return hostAdmissionPathFrom(directory) }
}

func admissionDirectorySnapshot(t *testing.T, directory string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := map[string]string{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		snapshot[entry.Name()] = fmt.Sprintf("%d:%s:%s", info.Size(), info.ModTime().Format(time.RFC3339Nano), data)
	}
	return snapshot
}

func TestTheAdmissionReadCreatesNothingAndListsOnlyHeldMarkers(t *testing.T) {
	t.Parallel()
	// TestMain selects this binary's private admission directory.
	if _, err := ReadHostAdmission(); err != nil {
		t.Fatal(err)
	}
	directory, conf := isolatedHostResources(t)
	// The acquisition return is the phase seam: its claim and flock exist.
	lease, err := acquireHostResourcesIn(WithLaneCheckPriority(t.Context()), directory, directory, conf, "heavy", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	markers, err := filepath.Glob(filepath.Join(directory, "lease-heavy-*"))
	if err != nil || len(markers) != 1 {
		t.Fatalf("lane claims = %v, %v", markers, err)
	}
	reader, err := os.Open(markers[0])
	if err != nil {
		t.Fatal(err)
	}
	record, _, err := readHostLeaseRecord(reader)
	reader.Close()
	if err != nil || record.Priority != HostPriorityLane {
		t.Fatalf("lane claim = %+v, %v", record, err)
	}
	cleared, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(directory, "lease-heavy-"+strings.Repeat("a", 32)), cleared, 0o600)
	writeTestFile(t, filepath.Join(directory, "lease-heavy-"+strings.Repeat("b", 32)), []byte("unreadable"), 0o600)
	waiter, err := holdWaitMarker(directory, hostPriorityRun)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseHostProbe(waiter)
	stale, err := holdWaitMarker(directory, HostPriorityLane)
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseHostProbe(stale); err != nil {
		t.Fatal(err)
	}
	before := admissionDirectorySnapshot(t, directory)
	exact := identity.Exact{Pid: record.Owner.Pid, StartedAt: time.UnixMicro(record.Owner.PidStartedAtMicro),
		StartTicks: record.Owner.PidStartTicks, BootID: record.Owner.BootID}
	live := functionIdentityProber(func(int64) (identity.Exact, identity.Liveness, error) {
		return exact, identity.Alive, nil
	})
	view, err := readHostAdmission(admissionReadPath(directory), live)
	if err != nil || len(view.Held) != 1 || len(view.Waiting) != 1 || view.Unreadable != 1 {
		t.Fatalf("admission = %+v, %v", view, err)
	}
	if got := view.Held[0]; got.Owner != record.Owner || got.Slot != record.Slot || got.Priority != HostPriorityLane || got.OwnerEnded {
		t.Fatalf("holder = %+v, claim = %+v", got, record)
	}
	info, err := waiter.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Waiting[0]; got.Owner != record.Owner || got.Priority != hostPriorityRun || !got.Since.Equal(info.ModTime()) {
		t.Fatalf("waiter = %+v", got)
	}
	if after := admissionDirectorySnapshot(t, directory); !reflect.DeepEqual(before, after) {
		t.Fatalf("read changed names, contents, sizes or modification times: before=%v after=%v", before, after)
	}
	reused := exact
	if reused.StartTicks != 0 {
		reused.StartTicks++
	} else {
		reused.StartedAt = reused.StartedAt.Add(time.Microsecond)
	}
	for _, row := range []struct {
		name  string
		state identity.Liveness
		exact identity.Exact
		ended bool
	}{
		{"ended owner", identity.Dead, identity.Exact{}, true},
		{"reused pid", identity.Alive, reused, true},
		{"unknown owner", identity.Unknown, identity.Exact{}, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			prober := functionIdentityProber(func(int64) (identity.Exact, identity.Liveness, error) {
				return row.exact, row.state, nil
			})
			got, err := readHostAdmission(admissionReadPath(directory), prober)
			if err != nil || len(got.Held) != 1 || got.Held[0].OwnerEnded != row.ended {
				t.Fatalf("owner-ended view = %+v, %v", got, err)
			}
		})
	}
	t.Run("absent directory", func(t *testing.T) {
		absent := filepath.Join(t.TempDir(), "absent", "admission")
		got, err := readHostAdmission(admissionReadPath(absent), deadTestProber{})
		if err != nil || !reflect.DeepEqual(got, HostAdmission{}) {
			t.Fatalf("absent admission = %+v, %v", got, err)
		}
		if _, err := os.Lstat(filepath.Dir(absent)); !os.IsNotExist(err) {
			t.Fatalf("read created the absent directory's parent: %v", err)
		}
	})
	t.Run("unreadable directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		writeTestFile(t, path, nil, 0o600)
		if _, err := readHostAdmission(admissionReadPath(path), deadTestProber{}); err == nil {
			t.Fatal("a file in place of the admission directory was accepted")
		}
		want := errors.New("path cannot be resolved")
		if _, err := readHostAdmission(func() (string, bool, error) { return "", false, want }, deadTestProber{}); !errors.Is(err, want) {
			t.Fatalf("path read error = %v", err)
		}
	})
}
