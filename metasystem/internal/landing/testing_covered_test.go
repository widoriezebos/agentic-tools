package landing

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Landing accepts an owned group that passed because its expected tests
// passed natively in another owned group of the same result, and refuses the
// same claim once the covering evidence no longer matches.
func TestTestingOwnersAcceptCoveredPassOnlyWithMatchingNativeSource(t *testing.T) {
	t.Parallel()
	f := &observeFixture{t: t, root: t.TempDir()}
	f.write("metasystem.conf", "dispatch.cap-max=120\nmetasystem.runtimes=fake\n")
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	identity, err := proofrun.BuildProofIdentity(f.root, filepath.Join(f.root, "metasystem.conf"), "selected", "testing", nil, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	sourceIdentity, coveredIdentity := strings.Repeat("3", 64), strings.Repeat("4", 64)
	attempt, decision, err := proofrun.ReserveLocked(proofrun.WithTestHostAdmissionDirectory(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: f.root, ExecutionRoot: f.root,
		GoalID: "goal", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 5, Identity: identity, Launcher: launcher,
		Now: now, SharedComponents: true, ComponentIdentities: map[string]string{"source": sourceIdentity, "covered": coveredIdentity}}, strings.Repeat("b", 40)),
		filepath.Join(t.TempDir(), "host-admission")))
	if err != nil || decision.Disposition != proofrun.DispositionExecuted || len(attempt.TestOwned) != 2 {
		t.Fatalf("reservation: %+v %+v %v", attempt, decision, err)
	}
	exit := 0
	context := strings.Repeat("c", 64)
	expected := proofrun.NativeTestIdentity{Classname: "fixture", Name: "TestShared", Status: "expected"}
	passed := proofrun.NativeTestIdentity{Report: "go-test-json", Classname: "fixture", Name: "TestShared", Status: "passed"}
	result := proofrun.TestResult{AttemptID: attempt.AttemptID, Groups: []proofrun.GroupResult{
		{ID: "source", ExecutionIdentity: sourceIdentity, Status: "passed", NativeLaunched: true, NativeExitStatus: &exit, CollectionComplete: true,
			NativeContext: context, Expected: []proofrun.NativeTestIdentity{expected}, Observed: []proofrun.NativeTestIdentity{passed}},
		{ID: "covered", ExecutionIdentity: coveredIdentity, Status: "passed", CollectionComplete: true, NativeContext: context,
			Expected: []proofrun.NativeTestIdentity{expected}, Observed: []proofrun.NativeTestIdentity{passed},
			CoveredByGroups: []string{"source"}, CoveredTests: []proofrun.NativeTestIdentity{passed}},
	}}
	if _, err := validateTestingAttemptOwnersAt(f.root, result, true, now); err != nil {
		t.Fatalf("covered pass with its native source refused: %v", err)
	}
	for name, change := range map[string]func(*proofrun.GroupResult){
		"different context":  func(group *proofrun.GroupResult) { group.NativeContext = strings.Repeat("d", 64) },
		"uncovered test":     func(group *proofrun.GroupResult) { group.CoveredTests[0].Name = "TestOther" },
		"no covering source": func(group *proofrun.GroupResult) { group.CoveredByGroups = nil },
	} {
		changed := result
		changed.Groups = append([]proofrun.GroupResult(nil), result.Groups...)
		changed.Groups[1].CoveredTests = append([]proofrun.NativeTestIdentity(nil), result.Groups[1].CoveredTests...)
		change(&changed.Groups[1])
		if _, err := validateTestingAttemptOwnersAt(f.root, changed, true, now); err == nil || !strings.Contains(err.Error(), "covered") {
			t.Fatalf("%s: covered claim accepted or refused for another reason: %v", name, err)
		}
	}
}
