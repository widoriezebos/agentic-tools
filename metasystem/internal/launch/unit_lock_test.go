package launch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// TestUnitLockReleaseSurvivesAnInheritedDescriptor: a duplicate of a unit
// lock's descriptor, as a concurrently forked child holds until it execs,
// keeps neither the run lock nor the named lock once its holder releases
// it. While the holder has it, a second caller is still refused as busy.
func TestUnitLockReleaseSurvivesAnInheritedDescriptor(t *testing.T) {
	t.Parallel()
	runner := &UnitRunner{Manager: &Manager{Store: Store{Root: t.TempDir()}}, Root: filepath.Join(t.TempDir(), "unit")}
	plan := UnitPlan{Unit: "u", Goal: "g"}
	for _, lock := range []struct {
		name    string
		acquire func() (*os.File, error)
	}{
		{"run", func() (*os.File, error) { return runner.lock("20260925t120000-0123456789") }},
		{"named", func() (*os.File, error) { return runner.namedLock("0123456789abcdef0123456789abcdef", plan) }},
	} {
		held, err := lock.acquire()
		if err != nil {
			t.Fatalf("%s lock: %v", lock.name, err)
		}
		inherited, err := unix.Dup(int(held.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		if second, err := lock.acquire(); err == nil || !strings.HasPrefix(err.Error(), "UNIT_RUN_BUSY") {
			if second != nil {
				releaseUnitLock(second)
			}
			t.Fatalf("%s lock while held: err=%v, want UNIT_RUN_BUSY", lock.name, err)
		}
		releaseUnitLock(held)
		again, err := lock.acquire()
		if err != nil {
			t.Fatalf("%s lock after its holder released it, with an inherited descriptor still open: %v", lock.name, err)
		}
		releaseUnitLock(again)
		if err := unix.Close(inherited); err != nil {
			t.Fatal(err)
		}
	}
}
