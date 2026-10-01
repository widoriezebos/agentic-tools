package lane

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// retiredOwnerNames are the old batch owner's entry points and the machinery
// only it ran, which the lane's hard cutover deleted (design r10 §5, unit
// D): its process and lease, its supervised component, its keeper, its pass
// and state machine, its diagnosis and verifier retries, its re-arm through
// up, its cadence tick driver and its replay reconstruction.
var retiredOwnerNames = []string{
	"EnsureBatchOwner", "AcquireBatchOwner", "BatchOwnerLease", "RunBatchOwnerPass", "BatchOwnerCadence",
	"setupLandingOwner", "LandingOwnerLaneRoot", "LandingLaneOwnerProbe", "EndLaneOwner", "LandingLaneKeeper",
	"batchowner.LandingOwnerLineage", "LandingOwnerInvocation", "BridgeNudges",
	"batch.NewOwner", "batch.Owner", "OwnerOptions", "ExecuteBatchDiagnosis", "DiagnoseRed",
	"ExecuteBatchProof", "LaunchBatchTipProof", "RearmBatchTip", "RearmBatchBase", "OwnerUpLandedEngine",
	"UpLandedEngine", "cadence.RunTick", "RunCadenceTick", "LandSeries", "BatchLandSeams", "ExecuteBatchLanding",
	"RecoverMovedBatchPushWith", "ReopenMovedBase",
}

// The old owner is gone (design r10 §5, unit D): no production source of
// the module names one of its entry points, and its lineage is named only
// where the cutover refuses while a claim under it remains.
func TestTheOldBatchOwnerIsGone(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	err = filepath.WalkDir(module, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "testdata", "node_modules", "artifacts", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		read++
		relative, _ := filepath.Rel(module, path)
		source := string(data)
		for _, name := range retiredOwnerNames {
			if strings.Contains(source, name) {
				t.Errorf("%s names %s, which the lane's cutover deleted with the old batch owner", relative, name)
			}
		}
		if strings.Contains(source, `"`+OldOwnerLineage+`"`) && filepath.ToSlash(relative) != "internal/landing/lane/identity.go" {
			t.Errorf("%s names the old owner's lineage %q; only lane.OldOwnerLineage may, for the cutover's refusal", relative, OldOwnerLineage)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if read < 100 {
		t.Fatalf("the audit read %d production files; it is not looking at the module", read)
	}
}
