package proofrun

import (
	"os"
	"path/filepath"
	"testing"
)

// A person's --leases settles by the admission path's own proof and says
// when a proof holds admission.lock: then nothing is reclaimed; free, the
// dead, settled lease is reclaimed with its record.
func TestAPersonsLeaseSettlementSaysWhenAdmissionIsBusy(t *testing.T) {
	t.Parallel()
	directory := reclaimDirectory(t)
	path := writeDirtyReclaimLease(t, directory)
	seams := newReclaimSeams(reclaimProber{})
	guard, acquired, err := tryHostFile(filepath.Join(directory, "admission.lock"))
	if err != nil || !acquired {
		t.Fatalf("hold admission.lock: acquired=%t err=%v", acquired, err)
	}
	reports, locked, err := inspectHostLeasesLocked(directory, seams.reclaimer())
	if err != nil || locked || len(reports) != 1 || reports[0].State != HostLeaseDead {
		t.Fatalf("busy: reports=%+v locked=%t err=%v", reports, locked, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("a busy settlement reclaimed: %v", err)
	}
	if err := releaseHostProbe(guard); err != nil {
		t.Fatal(err)
	}
	reports, locked, err = inspectHostLeasesLocked(directory, seams.reclaimer())
	if err != nil || !locked || len(reports) != 1 || reports[0].State != HostLeaseReclaimed || len(reclaimRecords(t, directory)) != 1 {
		t.Fatalf("free: reports=%+v locked=%t err=%v", reports, locked, err)
	}
}
