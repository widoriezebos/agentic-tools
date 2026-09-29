package launch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// A read attempt's context is a registered store the attempt owns (Part B
// DL-18): recorded in the machine registry while the reader runs, and
// released through that record once its launches ended; the record stays
// as history.
func TestReadContextIsAStoreTheReadAttemptOwns(t *testing.T) {
	t.Parallel()
	fixture := newStandaloneFixture(t, sampleDiff())
	fixture.git.checkoutFileNames = []string{"pkg/a/a.go"}
	armed, err := registry.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	stores := diskstore.MachineRegistry(filepath.Dir(armed))
	find := func(path string) (diskstore.Record, bool) {
		records, _ := stores.Inventory()
		for _, record := range records {
			if record.Path == path {
				return record, true
			}
		}
		return diskstore.Record{}, false
	}
	var running diskstore.Record
	var found bool
	fixture.starter.onStart = func(record Record) error {
		running, found = find(filepath.Dir(record.WorkingDirectory))
		return nil
	}
	result, err := fixture.runner.StartRead(fixture.request)
	if err != nil || !result.Complete {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !found || running.State != diskstore.StateAccepted || running.Class != "read-context" ||
		running.Owner != (diskstore.Owner{Kind: diskstore.OwnerAttempt, Ref: "read:" + result.Ref + "-a1"}) {
		t.Fatalf("while the reader ran its context's record was %+v (found %v)", running, found)
	}
	if _, err := os.Stat(result.Attempt.Context); !errors.Is(err, os.ErrNotExist) || !result.Attempt.ContextRemoved {
		t.Fatalf("context not removed: %v", err)
	}
	if after, ok := find(result.Attempt.Context); !ok || after.State != diskstore.StateReleased {
		t.Fatalf("after the read the record is %+v (found %v); it is released and kept", after, ok)
	}
}
