package metrics

import (
	"strings"
	"testing"
)

// A specified but relative evidence root is one rejected coverage detail in
// the owner's sentence, never silence; local job records are still read.
func TestJobsReportARefusedEvidenceRoot(t *testing.T) {
	t.Parallel()
	f := newSourceFixture(t)
	f.seedFullWorld()
	f.write("metasystem/metasystem.conf", "evidence.root=relative\n")
	records, coverage := loadJobs(f.root)
	found := false
	for _, record := range records {
		found = found || record.JobID == "j1"
	}
	details := strings.Join(coverage.Details, "\n")
	if !found || coverage.Rejected != 1 || !strings.Contains(details, `evidence.root must be absolute (metasystem.conf reads "relative")`) {
		t.Fatalf("records=%d found=%v coverage=%+v", len(records), found, coverage)
	}
}
