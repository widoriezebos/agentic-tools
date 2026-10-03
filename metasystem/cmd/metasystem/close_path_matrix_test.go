package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The close-path matrix: one table over every way a critic chain ends,
// for design chains and code chains, driven through the real owners: the
// fold (dispatch's CritiqueRegisterAdvance, as a follow-up dispatch runs it),
// the collect a review or design review prints (collectReview), and the
// close (work finish through the real delegate close owner, its register
// close, close check and chain close). Each row says whether the chain
// closes, and whether it closes to the read a landing takes.

const matrixRoot = "critm"

// closeMatrixBed is one critic chain on a delivery bed whose close owner is
// the real one. Round 1 is the root; round N > 1 is <root>-rN whose parent
// is round N-1, as dispatch writes follow-ups.
type closeMatrixBed struct {
	*deliveryBed
	role   string
	design string
}

func newCloseMatrixBed(t *testing.T, role string) *closeMatrixBed {
	t.Helper()
	b := &closeMatrixBed{deliveryBed: newDeliveryBed(t), role: role, design: "metasystem/plans/designs/matrix.md"}
	realCloseOwner(t, b.deliveryBed)
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	if err := os.MkdirAll(filepath.Join(b.install, "artifacts", "agents", "record-locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(b.install, "metasystem.conf")
	existing, _ := os.ReadFile(conf)
	b.writeFile(conf, string(existing)+"\nevidence.root="+t.TempDir()+"\n")
	return b
}

// artifact is where this chain's findings lie: inside the design's declared
// outputs for a design chain, a changed file for a code chain.
func (b *closeMatrixBed) artifact() string {
	if b.role == "design-critic" {
		return b.design
	}
	return "metasystem/internal/x.go"
}

func (b *closeMatrixBed) jobID(round int) string {
	if round == 1 {
		return matrixRoot
	}
	return fmt.Sprintf("%s-r%d", matrixRoot, round)
}

// matrixFinding is one returned finding with its rigor row; class is the
// rigor the facts earn: severe crosses an authority boundary, bounded is a
// well-formed local recoverable finding, unproven carries no facts.
type matrixFinding struct {
	id, class, artifact string
	material            bool
}

func matrixFacts(class string) any {
	switch class {
	case "severe":
		return map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": true,
			"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
	case "bounded":
		return map[string]any{"local": true, "recoverable": true, "proofBoundaryCrossed": false, "authorityBoundaryCrossed": false,
			"secretsBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false}
	}
	return nil
}

// round writes round n as completed with its return (a schema-v4 return
// bound to the round's subject) and its subject.
func (b *closeMatrixBed) round(n int, findings ...matrixFinding) {
	b.t.Helper()
	job := b.jobID(n)
	record := map[string]any{"jobId": job, "role": b.role, "status": "completed", "round": n,
		"destructiveReach": "DESIGN-BEARING", "dispatchMode": "follow-up", "sessionId": matrixRoot + "-session", "endedAt": "2026-10-03T07:00:00Z",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": b.jobID(n - 1)}
	if n == 1 {
		record["dispatchMode"], record["parentJob"] = "fresh", nil
		record["findingRegister"], record["findingRegisterRound"] = []any{}, 0
		record["goalId"], record["machineId"], record["mainId"], record["claimEpoch"] = bedGoal, "m", "l", 1
		record["reviewRoundLimit"] = 20
		if b.role == "design-critic" {
			record["declaredOutputs"] = []any{b.design}
		}
	}
	b.writeJob(record)
	returned := []any{}
	rigor := []any{}
	for _, f := range findings {
		artifact := f.artifact
		if artifact == "" {
			artifact = b.artifact()
		}
		returned = append(returned, map[string]any{"id": f.id, "material": f.material, "claim": f.id + " claim", "evidence": f.id + " evidence round " + fmt.Sprint(n)})
		if f.material {
			row := map[string]any{"findingId": f.id, "artifact": artifact, "rigorClass": f.class, "facts": matrixFacts(f.class)}
			if f.class != "unproven" {
				row["reopeningTrigger"] = "the facts change"
			}
			rigor = append(rigor, row)
		}
	}
	result := map[string]any{"jobId": job, "round": n, "findings": returned, "rigor": rigor, "schemaVersion": 4, "verdictMaterialCount": len(rigor)}
	subject := readsubject.ReadSubject{Kind: readsubject.SubjectLive, ImplementerRoot: "impl1", ReviewedMember: "impl1",
		ReviewedProjectTree: fmt.Sprintf("tree-%d", n), DiffDigest: "diff"}
	result["reviewedTree"] = subject.ReviewedProjectTree
	if b.role == "design-critic" {
		subject = readsubject.ReadSubject{Kind: readsubject.SubjectDesign, DesignPath: b.design, ContentDigest: fmt.Sprintf("content-%d", n),
			DeclaredOutputsDigest: "outputs", ReviewedCommit: fmt.Sprintf("commit-%d", n)}
		delete(result, "reviewedTree")
		result["reviewedCommit"] = subject.ReviewedCommit
	}
	dir := filepath.Join(b.install, "artifacts", "agents", matrixRoot, "rounds", fmt.Sprint(n))
	b.writeJSON(filepath.Join(dir, "return.json"), result)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := dispatchcore.WriteReadSubject(filepath.Join(dir, "subject.json"), subject); err != nil {
		b.t.Fatal(err)
	}
}

// decide writes round n's own decisions file, the one work review writes
// beside the return and the author fills.
func (b *closeMatrixBed) decide(n int, rows ...string) {
	b.t.Helper()
	body := deliveryDispositionsHeader
	for _, row := range rows {
		body += row + "\n"
	}
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", matrixRoot, "rounds", fmt.Sprint(n), "decisions.md"), body)
}

// fold folds round n into the root's register, as the follow-up dispatch of
// round n+1 does.
func (b *closeMatrixBed) fold(n int) {
	b.t.Helper()
	if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, matrixRoot, b.jobID(n)); err != nil {
		b.t.Fatalf("fold of round %d: %v", n, err)
	}
}

// finish runs work finish on the chain with the author's decisions on the
// terminal round's findings.
func (b *closeMatrixBed) finish(rows ...string) (int, intentResult) {
	b.t.Helper()
	decisions := filepath.Join(b.root(), "decisions.md")
	body := deliveryDispositionsHeader
	for _, row := range rows {
		body += row + "\n"
	}
	b.writeFile(decisions, body)
	return b.do("work", "finish", "j2:"+matrixRoot, "--dispositions", decisions)
}

func (b *closeMatrixBed) closed() bool {
	closed, _ := b.job(matrixRoot)["chainClosed"].(bool)
	return closed
}

func (b *closeMatrixBed) closure() bool {
	_, present := b.job(matrixRoot)["closure"]
	return present
}

func (b *closeMatrixBed) entry(id string) map[string]any {
	b.t.Helper()
	register, _ := b.job(matrixRoot)["findingRegister"].([]any)
	for _, raw := range register {
		if entry, _ := raw.(map[string]any); entry["findingId"] == id {
			return entry
		}
	}
	b.t.Fatalf("finding %s is not in the register: %v", id, register)
	return nil
}

// expectClosed asserts the close was confirmed, the chain is closed, and
// whether it closed to a landable read.
func (b *closeMatrixBed) expectClosed(code int, result intentResult, landable bool) {
	b.t.Helper()
	if result.Outcome != intentConfirmed || code != 0 || !b.closed() {
		b.t.Fatalf("the chain did not close: exit %d %+v data=%v", code, result, result.Data)
	}
	if b.closure() != landable {
		b.t.Fatalf("closure recorded = %v, want %v: %v", b.closure(), landable, b.job(matrixRoot))
	}
}

func TestClosePathMatrix(t *testing.T) {
	t.Parallel()
	severe := func(id string) matrixFinding { return matrixFinding{id: id, class: "severe", material: true} }
	unproven := func(id string) matrixFinding { return matrixFinding{id: id, class: "unproven", material: true} }
	bounded := func(id string) matrixFinding { return matrixFinding{id: id, class: "bounded", material: true} }
	rows := []struct {
		name  string
		roles []string
		run   func(t *testing.T, b *closeMatrixBed)
	}{
		{"final round clean", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1)
			code, result := b.finish()
			b.expectClosed(code, result, true)
		}},
		{"empty confirming round after fixed rounds", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			// Rounds 1 and 2 raised severe and unproven findings the author
			// accepted (a fix is required) in each round's own decisions
			// file; round 3 examined the fixes and found nothing.
			b.round(1, severe("F1"), unproven("F2"))
			b.decide(1, "| F1 | accepted | real | fixed |", "| F2 | accepted | real | fixed |")
			b.fold(1)
			b.round(2, severe("F3"))
			b.decide(2, "| F3 | accepted | real | fixed |")
			b.fold(2)
			b.round(3)
			collected := b.invocation().collectReview(nil, delegateOutcome{Outcome: "REJOINED", JobID: b.jobID(3)})
			if collected.Outcome != intentConfirmed || !strings.Contains(collected.Summary, "0 material") {
				t.Fatalf("the clean confirming round's findings were not read: %+v", collected)
			}
			code, result := b.finish()
			// Accepted (a fix was required) is decided, not a clean read.
			b.expectClosed(code, result, false)
			for _, id := range []string{"F1", "F2", "F3"} {
				if entry := b.entry(id); entry["status"] != "resolved" || entry["resolution"] != "accepted" {
					t.Fatalf("finding %s confirmed fixed is not resolved as accepted: %v", id, entry)
				}
			}
		}},
		{"empty confirming round folded before the fix closes on a repeat", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			// The register seat m1h's close left (2026-10-03): round 3 folded
			// clean, rounds 1-2's accepted findings still open.
			b.round(1, severe("F1"), unproven("F2"))
			b.decide(1, "| F1 | accepted | real | fixed |", "| F2 | accepted | real | fixed |")
			b.fold(1)
			b.round(2, severe("F3"))
			b.decide(2, "| F3 | accepted | real | fixed |")
			b.fold(2)
			b.round(3)
			b.fold(3)
			code, result := b.finish()
			b.expectClosed(code, result, false)
		}},
		{"earlier finding nobody decided stays open after a clean round", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			// An omitted finding stays open (plans/defect-analysis-gate-design.md):
			// only the critic's withdrawal (a same-id material:false row) or
			// a decision resolves it.
			b.round(1, severe("F1"))
			b.fold(1)
			b.round(2)
			code, result := b.finish()
			if result.Outcome == intentConfirmed || code == 0 || b.closed() || !strings.Contains(fmt.Sprint(result.Data), "F1") {
				t.Fatalf("an undecided omitted finding closed or was not named: exit %d %+v", code, result)
			}
		}},
		{"earlier accepted finding raised again refuses", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			// The fix of round 1's F1 did not hold: round 2 raises F1 again
			// and the author accepts it again, so it stays open.
			b.round(1, severe("F1"))
			b.decide(1, "| F1 | accepted | real | fixed |")
			b.fold(1)
			b.round(2, severe("F1"))
			code, result := b.finish("| F1 | accepted | still real | fix again |")
			if result.Outcome == intentConfirmed || code == 0 || b.closed() {
				t.Fatalf("a finding raised again closed on its earlier acceptance: exit %d %+v", code, result)
			}
		}},
		{"critic withdrawal in a clean round", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, severe("F1"))
			b.fold(1)
			b.round(2, matrixFinding{id: "F1"})
			code, result := b.finish("| F1 | noted | withdrawn by the critic | none |")
			b.expectClosed(code, result, true)
		}},
		{"dispositioned refuted and out-of-scope", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, bounded("F1"), bounded("F2"), matrixFinding{id: "N1"})
			code, result := b.finish("| F1 | refuted | the path is unreachable | none |", "| F2 | out-of-scope | the brief scopes it out | none |", "| N1 | noted | a remark | none |")
			// A refuted finding closes without the read a landing takes.
			b.expectClosed(code, result, false)
		}},
		{"demoted finding ruled out-of-scope", []string{"design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, matrixFinding{id: "F1", class: "severe", material: true, artifact: "metasystem/internal/elsewhere.go"})
			code, result := b.finish("| F1 | out-of-scope | it lies outside the brief's declared scope | none |")
			b.expectClosed(code, result, true)
		}},
		{"person-accepted risk", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, severe("F1"))
			b.fold(1)
			if err := dispatchcore.CritiqueRegisterAcceptRisk(b.install, matrixRoot, "F1", "person-op"); err != nil {
				t.Fatal(err)
			}
			code, result := b.finish("| F1 | accepted | a person accepted the risk | none |")
			b.expectClosed(code, result, true)
		}},
		{"undecided material finding refuses", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, severe("F1"))
			code, result := b.finish("| F1 | accepted | a fix is required | none |")
			if result.Outcome == intentConfirmed || code == 0 || b.closed() || !strings.Contains(fmt.Sprint(result.Data), "F1") {
				t.Fatalf("a chain with an open severe finding closed or did not name it: exit %d %+v", code, result)
			}
		}},
		{"open finding of a non-clean confirming round refuses", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			// Round 2 found something new: round 1's findings are not
			// confirmed fixed, and the round's own finding is undecided.
			b.round(1, severe("F1"))
			b.fold(1)
			b.round(2, unproven("F2"))
			code, result := b.finish("| F2 | accepted | a fix is required | none |")
			if result.Outcome == intentConfirmed || code == 0 || b.closed() {
				t.Fatalf("a chain whose last round found something closed: exit %d %+v", code, result)
			}
			if entry := b.entry("F1"); entry["status"] != "open" {
				t.Fatalf("round 1's finding was resolved by a round that found something: %v", entry)
			}
		}},
		{"lower-rigor re-report of a resolved entry", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, severe("F1"))
			b.fold(1)
			b.round(2, matrixFinding{id: "F1"}) // withdrawn: no longer material
			b.fold(2)
			b.round(3, bounded("F1"))
			code, result := b.finish("| F1 | refuted | withdrawn in round 2 and fixed | none |")
			b.expectClosed(code, result, true)
			if entry := b.entry("F1"); entry["status"] != "resolved" || entry["resolution"] != "withdrawn" {
				t.Fatalf("the resolved entry did not keep its resolution: %v", entry)
			}
		}},
		{"lower-rigor re-report of an accepted risk", []string{"code-critic", "design-critic"}, func(t *testing.T, b *closeMatrixBed) {
			b.round(1, severe("F1"))
			b.fold(1)
			if err := dispatchcore.CritiqueRegisterAcceptRisk(b.install, matrixRoot, "F1", "person-op"); err != nil {
				t.Fatal(err)
			}
			b.round(2, bounded("F1"))
			code, result := b.finish("| F1 | accepted | a person accepted the risk | none |")
			b.expectClosed(code, result, true)
			if entry := b.entry("F1"); entry["status"] != "accepted-risk" || entry["decisionOpid"] != "person-op" {
				t.Fatalf("a lower-rigor re-report overturned the person's accepted risk: %v", entry)
			}
		}},
	}
	for _, row := range rows {
		for _, role := range row.roles {
			t.Run(row.name+"/"+role, func(t *testing.T) {
				t.Parallel()
				row.run(t, newCloseMatrixBed(t, role))
			})
		}
	}
	t.Run("repeated accept-risk after a changed finding/design-critic", func(t *testing.T) {
		t.Parallel()
		repeatedAcceptRiskAfterChangedFinding(t)
	})
}

// invocation is an intent invocation over the bed's own owners, for the
// collect a review or design review prints.
func (b *closeMatrixBed) invocation() *intentInvocation {
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	layout, err := stateroot.NewResolver(fakeTop(b.root()), noExecutable).ResolveLayout(b.root())
	if err != nil {
		b.t.Fatal(err)
	}
	return &intentInvocation{owners: owners, layout: layout, cwd: b.root(), stateRoot: b.root(), input: intentInput{values: map[string][]string{}}}
}

// repeatedAcceptRiskAfterChangedFinding: a person accepts a severe finding's
// risk; a later fold re-reports the finding at higher rigor with changed
// evidence, which reopens it; the person accepts it again. The goal act
// already holds (the same person, the same finding), and the register must
// still carry the person's decision so the chain closes (R-142-m1e).
func repeatedAcceptRiskAfterChangedFinding(t *testing.T) {
	fixture := newObligationCommandFixture(t)
	root := fixture.root()
	dependencies := fixture.dependencies()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{
		"findingId": "S-1", "critic": "critic", "rigorClass": "unproven",
		"factsDigest": strings.Repeat("a", 64), "facts": map[string]any{"local": true},
		"artifact": "metasystem/in.go", "title": "unproven finding", "status": "open",
		"resolution": "", "decisionOpid": "", "evidence": "direct proof",
		"evidenceDigest": strings.Repeat("b", 64), "multiplicity": 1,
	}
	writeTemp(t, jobs, "critic.json", map[string]any{
		"jobId": "critic", "role": "design-critic", "round": 1, "parentJob": nil,
		"status": "completed", "goalId": "standing-validation", "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3, "demotions": []any{},
		"findingRegister": []any{entry},
	})
	args := []string{"--root", root, "--id", "standing-validation", "--finding", "S-1", "--chain", "critic",
		"--by", "Wido", "--why", "human accepts the bounded exposure", "--lineage", "m1",
		"--temporary-human-word", "Wido accepts this risk", "--review-by", "2026-09-06"}
	accept := func() (int, string, string) {
		return runOnOwnStreams(func(stdout, stderr io.Writer) int {
			return runGoalAcceptRiskWithFacts(args, fixedTemporaryGoalAuthority, fixture.commandNow, withStreams(dependencies, stdout, stderr), nil)
		})
	}
	if code, stdout, stderr := accept(); code != 0 {
		t.Fatalf("first acceptance = exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	register := func() map[string]any {
		data, err := os.ReadFile(filepath.Join(jobs, "critic.json"))
		if err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		return record["findingRegister"].([]any)[0].(map[string]any)
	}
	opid, _ := register()["decisionOpid"].(string)
	if opid == "" {
		t.Fatalf("the first acceptance did not stamp the register: %v", register())
	}
	// A higher-rigor re-report replaces the entry and reopens it.
	changed := map[string]any{}
	for key, value := range entry {
		changed[key] = value
	}
	changed["rigorClass"], changed["evidence"], changed["evidenceDigest"] = "severe", "changed proof", strings.Repeat("c", 64)
	writeTemp(t, jobs, "critic.json", map[string]any{
		"jobId": "critic", "role": "design-critic", "round": 1, "parentJob": nil,
		"status": "completed", "goalId": "standing-validation", "findingRegisterRound": 1,
		"reviewRoundLimit": 3, "criticRoundsConsumed": 3, "demotions": []any{},
		"findingRegister": []any{changed},
	})
	code, stdout, stderr := accept()
	if code != 0 {
		t.Fatalf("the repeated acceptance = exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if again := register(); again["status"] != "accepted-risk" || again["decisionOpid"] != opid {
		t.Fatalf("the repeated acceptance left the changed finding undecided: %v (stdout %q)", again, stdout)
	}
	if outcome, err := dispatchcore.CritiqueRegisterClose(root, "critic"); err != nil || outcome != "closed" {
		t.Fatalf("the person's accepted risk did not close the chain: %q %v", outcome, err)
	}
}
