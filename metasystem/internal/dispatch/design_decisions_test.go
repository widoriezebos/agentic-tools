package dispatch

import (
	"strings"
	"testing"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// TestDesignDecisionsReachTheRegister (g1-s66 D4, S66-06): the author's
// validated decisions reach the register before the close. A refuted material
// finding on an unchanged round-1 design refuses the real register close
// until the refutation is applied, and then closes it; the refutation is its
// own resolution, never an accepted risk.
func TestDesignDecisionsReachTheRegister(t *testing.T) {
	t.Parallel()
	finding := closeFinding("F1", "invariant", "", "the claim", critiqueModel.Bounded)
	repo, root, path := writeCloseRoot(t, "design-critic", 1, []registerFinding{finding}, materialHistory(1, 0), 2, 1)
	if _, err := CritiqueRegisterClose(repo, root); err == nil {
		t.Fatalf("an open refuted finding closed before its refutation reached the register")
	}
	if err := CritiqueRegisterApplyDecisions(repo, root, map[string]string{"F1": "refuted", "NOT-REGISTERED": "refuted"}); err != nil {
		t.Fatalf("apply the refutation: %v", err)
	}
	outcome, err := CritiqueRegisterClose(repo, root)
	check(t, err == nil && outcome == "closed", "a refuted finding on an unchanged design did not close: outcome=%q err=%v", outcome, err)
	record, _ := readObject(path)
	register, err := decodeFindingRegister(record[findingRegisterField])
	check(t, err == nil && register[0].Status == "resolved" && register[0].Resolution == "refuted" && register[0].DecisionOpID == "",
		"the refutation is not its own resolution: %+v %v", register, err)
	clean, err := readsubject.CleanRegister(encodeFindingRegister(register))
	check(t, err == nil && !clean, "a refuted register reads as a clean read: clean=%v err=%v", clean, err)
	// A repeat changes nothing; a finding already resolved another way keeps
	// its resolution.
	check(t, CritiqueRegisterApplyDecisions(repo, root, map[string]string{"F1": "refuted"}) == nil, "a repeated refutation refused")
	check(t, CritiqueRegisterApplyDecisions(repo, root, map[string]string{"F1": "accepted"}) == nil, "a later decision on a resolved finding refused")
	record, _ = readObject(path)
	register, _ = decodeFindingRegister(record[findingRegisterField])
	check(t, register[0].Resolution == "refuted", "a resolved finding was resolved again: %+v", register)
}

// TestDesignDecisionsKeepTheEngineClassification (S66-07): an accepted
// finding of an earlier round the follow-up did not raise again is resolved
// as amended; an out-of-scope severe finding is refused as before; and
// nothing else is decided here, so the final round's own accepted findings
// stay open for the close's classification.
func TestDesignDecisionsKeepTheEngineClassification(t *testing.T) {
	t.Parallel()
	earlier := closeFinding("E1", "invariant", "", "earlier", critiqueModel.Bounded)
	severe := closeFinding("S1", "invariant", "", "severe", critiqueModel.Severe)
	repo, root, path := writeCloseRoot(t, "design-critic", 2, []registerFinding{earlier, severe}, materialHistory(2, 1), 2, 2)
	err := CritiqueRegisterApplyDecisions(repo, root, map[string]string{"E1": "accepted", "S1": "out-of-scope"})
	check(t, err != nil && strings.Contains(err.Error(), "S1"), "an out-of-scope severe finding was admitted: %v", err)
	record, _ := readObject(path)
	register, _ := decodeFindingRegister(record[findingRegisterField])
	check(t, register[0].Status == "open" && register[1].Status == "open", "a refused application changed the register: %+v", register)
	check(t, CritiqueRegisterApplyDecisions(repo, root, map[string]string{"E1": "accepted"}) == nil, "an amended earlier finding refused")
	record, _ = readObject(path)
	register, _ = decodeFindingRegister(record[findingRegisterField])
	check(t, register[0].Status == "resolved" && register[0].Resolution == "accepted" && register[1].Status == "open",
		"the amended finding is not resolved as accepted, or the untouched one moved: %+v", register)
	_, err = CritiqueRegisterClose(repo, root)
	assertHumanRaise(t, err, "S1")
	check(t, CritiqueRegisterApplyDecisions(repo, root, map[string]string{"E1": "noted"}) != nil, "a fifth resolution was admitted")
}
