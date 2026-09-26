package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// deliveryBed is one isolated installation with its own job store, driven
// through the public delivery commands. Each owner process is a per-test
// fake that performs its owner's recorded transition.
type deliveryBed struct {
	*intentBed
	install string
	calls   [][]string
	handler func(intentProcess) intentProcessResult
	owners  *intentDeliveryOwners
	// connection is the goal-branch publication owners' per-test seam.
	connection intentConnectionOwners
}

func newDeliveryBed(t *testing.T) *deliveryBed {
	t.Helper()
	bed := &deliveryBed{intentBed: newIntentBed(t, false, nil)}
	// The project's design homes and the close owner resolve the checkout
	// through Git itself.
	if output, err := exec.Command("git", "-C", bed.root(), "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	layout, err := stateroot.NewResolver(fakeTop(bed.root()), noExecutable).ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	bed.install = layout.InstallationRoot
	bed.owners = &intentDeliveryOwners{
		// The bed's close owner runs as a person's act (its engine wrapper
		// classifies HUMAN); the record-writer authority owner judges that
		// same classification.
		recordWriter: humanRecordWriter,
		process: func(process intentProcess) intentProcessResult {
			bed.calls = append(bed.calls, process.argv)
			if bed.handler == nil {
				t.Fatalf("unexpected owner process %v", process.argv)
			}
			return bed.handler(process)
		},
		executable: func() (string, error) { return "/fake/bin/metasystem", nil },
		now:        func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) },
		batchRoot:  func(string, time.Time) (string, bool, error) { return "", false, nil },
	}
	return bed
}

func (b *deliveryBed) do(args ...string) (int, intentResult) {
	b.t.Helper()
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.connection = b.connection
	return b.runJSON(owners, args...)
}

func (b *deliveryBed) writeJob(record map[string]any) {
	b.t.Helper()
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "jobs", record["jobId"].(string)+".json"), record)
}

func (b *deliveryBed) job(id string) map[string]any {
	b.t.Helper()
	var record map[string]any
	data, err := os.ReadFile(filepath.Join(b.install, "artifacts", "agents", "jobs", id+".json"))
	if err != nil || json.Unmarshal(data, &record) != nil {
		b.t.Fatalf("job %s unreadable: %v", id, err)
	}
	return record
}

func (b *deliveryBed) writeJSON(path string, value any) {
	b.t.Helper()
	encoded, _ := json.Marshal(value)
	b.writeFile(path, string(encoded))
}

func (b *deliveryBed) writeFile(path, content string) {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

func (b *deliveryBed) writeReturn(root string, round int, job string, findings ...map[string]any) {
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", root, "rounds", string(rune('0'+round)), "return.json"),
		map[string]any{"jobId": job, "round": round, "findings": findings, "verdict": "2 findings"})
}

func flagValue(argv []string, name string) string {
	if index := slices.Index(argv, name); index >= 0 && index+1 < len(argv) {
		return argv[index+1]
	}
	return ""
}

func expectOutcome(t *testing.T, label string, code int, result intentResult, outcome string) {
	t.Helper()
	if result.Outcome != outcome {
		t.Fatalf("%s: outcome %s, want %s: %+v", label, result.Outcome, outcome, result)
	}
	if (code == 0) != (outcome == intentConfirmed || outcome == intentUnchanged) {
		t.Fatalf("%s: exit %d for outcome %s", label, code, outcome)
	}
}

// admissionFacts answers the design subject's repository reads without Git.
type admissionFacts struct{}

func (admissionFacts) CommitParent(string, string) (string, error) {
	return strings.Repeat("a", 40), nil
}
func (admissionFacts) CommitTree(string, string) (string, error)         { return strings.Repeat("b", 40), nil }
func (admissionFacts) CommitDiff(string, string, string) ([]byte, error) { return nil, nil }
func (admissionFacts) WorkspaceHead(string) (string, error)              { return strings.Repeat("c", 40), nil }
func (admissionFacts) LiveWorkspaceTree(string, string) (string, error) {
	return strings.Repeat("d", 40), nil
}

func TestIntentReviewEvidenceKinds(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	roots, err := project.ResolveRoots(b.install)
	if err != nil {
		t.Fatal(err)
	}
	var homes []string
	for _, home := range project.Homes(roots) {
		if home.Kind == project.KindDesign && home.Glob == "" {
			homes = append(homes, home.Path)
		}
	}
	design := filepath.Join(homes[len(homes)-1], "intent.md")
	workspace, err := filepath.EvalSymlinks(b.root()) // dispatch reviews the physical checkout (pwd -P)
	if err != nil {
		t.Fatal(err)
	}
	b.writeFile(design, "# Intent\n\n- Kind: design\n- Id: 01DESIGN\n- Status: accepted\n- Goals: standing-validation\n\nBody.\n")
	var briefs []string
	var operations []string
	round := func(status string) {
		b.writeJob(map[string]any{"jobId": "rev1", "role": "design-critic", "status": status, "round": 1, "goalId": "standing-validation"})
	}
	b.handler = func(process intentProcess) intentProcessResult {
		argv := process.argv
		if argv[1] != "delegate" || flagValue(argv, "--role") != "design-critic" || flagValue(argv, "--goal") != "standing-validation" ||
			flagValue(argv, "--destructive-reach") != "DESIGN-BEARING" {
			t.Fatalf("design review dispatch %v", argv)
		}
		// The dispatch's own admission of the subject and brief runs before
		// any model would launch.
		subject, _, err := dispatchcore.ComputeReadSubjectWithFacts(dispatchcore.ReadSubjectRequest{RepoRoot: b.install, Role: "design-critic",
			Workspace: workspace, Design: flagValue(argv, "--design"), DeclaredOutputs: flagValue(argv, "--outputs")}, admissionFacts{})
		if err != nil || subject.Kind != dispatchcore.SubjectDesign {
			t.Fatalf("design subject admission refused %v: %v", argv, err)
		}
		brief := flagValue(argv, "--brief")
		if mode, err := dispatchcore.BriefModeOnly(brief); err != nil || mode != "design-critique" {
			t.Fatalf("brief mode admission refused: %q %v", mode, err)
		}
		if _, err := dispatchcore.ReadBriefAdmission(brief, "", true, nil); err != nil {
			t.Fatalf("brief admission refused: %v", err)
		}
		text, _ := os.ReadFile(brief)
		for _, section := range []string{"Round budget: 2 focused rounds", "Threat model: ", "Scope: ", "## Prepared copy", "## Checklist", "Maximum reader tool calls: 30", "01DESIGN"} {
			if !strings.Contains(string(text), section) {
				t.Fatalf("brief lacks %q:\n%s", section, text)
			}
		}
		briefs = append(briefs, string(text))
		sum := sha256.Sum256(text)
		operation, err := dispatchcore.DefaultOperationID("standing-validation", 1, dispatchcore.DispatchModeFresh, "design-critic", hex.EncodeToString(sum[:]), "")
		if err != nil {
			t.Fatal(err)
		}
		operations = append(operations, operation)
		if len(briefs) == 1 {
			round("running")
			return intentProcessResult{stdout: []byte(`{"outcome":"WON","headline":"started","jobId":"rev1"}`)}
		}
		return intentProcessResult{stdout: []byte(`{"outcome":"REPLAYED-COMPLETED","headline":"already running","jobId":"rev1"}`)}
	}
	code, result := b.do("review", "design", design)
	expectOutcome(t, "no reader budget", code, result, intentRefused)
	if len(b.calls) != 0 || !strings.Contains(result.Decision, "--tool-calls") {
		t.Fatalf("a missing reader budget is named, never invented: %+v", result)
	}
	code, result = b.do("review", "design", design, "--tool-calls", "30")
	expectOutcome(t, "running design review", code, result, intentInProgress)
	round("completed")
	b.writeReturn("rev1", 1, "rev1", map[string]any{"id": "F1", "material": true}, map[string]any{"id": "F2", "material": false})
	code, result = b.do("review", "design", design, "--tool-calls", "30")
	expectOutcome(t, "collected design review", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "2 findings, 1 material") || len(briefs) != 2 || briefs[0] != briefs[1] || operations[0] != operations[1] {
		t.Fatalf("the repeat must carry the identical task and request identity: %+v", result)
	}

	outside := filepath.Join(b.root(), "notes", "intent.md")
	b.writeFile(outside, "- Kind: design\n- Id: 01OUT\n- Goals: standing-validation\n")
	calls := len(b.calls)
	code, result = b.do("review", "design", outside, "--tool-calls", "30")
	expectOutcome(t, "outside the design homes", code, result, intentRefused)
	notDesign := filepath.Join(homes[0], "notes.md")
	b.writeFile(notDesign, "# Notes\n")
	code, result = b.do("review", "design", notDesign, "--tool-calls", "30")
	expectOutcome(t, "non-design file", code, result, intentRefused)
	if len(b.calls) != calls {
		t.Fatal("refusals dispatch nothing")
	}

	b.writeJob(map[string]any{"jobId": "impl1", "role": "implementer", "status": "completed", "round": 1, "goalId": "standing-validation"})
	unknown := 0
	b.handler = func(process intentProcess) intentProcessResult {
		if flagValue(process.argv, "--role") != "code-critic" || flagValue(process.argv, "--reviews") != "impl1" {
			t.Fatalf("job review dispatch %v", process.argv)
		}
		if _, err := dispatchcore.ReadBriefAdmission(flagValue(process.argv, "--brief"), "", true, nil); err != nil {
			t.Fatalf("job review brief admission refused: %v", err)
		}
		unknown++
		if unknown == 1 {
			return intentProcessResult{stdout: []byte("lost"), code: 1}
		}
		return intentProcessResult{stdout: []byte(`{"outcome":"RECONCILING","headline":"already running"}`)}
	}
	code, result = b.do("review", "job", "impl1", "--tool-calls", "20")
	expectOutcome(t, "unknown dispatch outcome", code, result, intentInProgress)
	if result.Next == nil || !strings.Contains(result.Next.Reason, "not dispatched again") {
		t.Fatalf("unknown dispatch must be reported with recovery: %+v", result)
	}
	code, result = b.do("review", "job", "impl1", "--tool-calls", "20")
	expectOutcome(t, "reconciling dispatch", code, result, intentInProgress)

	var reads [][]string
	pending := false
	b.owners.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		reads = append(reads, args)
		if pending {
			return branch.BranchReadResult{}, 1, &branch.OpError{Code: branch.ReadDispatchPendingCode, Message: "critic dispatch outcome is unknown"}
		}
		if slices.Contains(args, "--collect") {
			return branch.BranchReadResult{State: "collected", RootJob: "crit9", GateRunID: "g1", AttestationCommit: "attest1"}, 0, nil
		}
		return branch.BranchReadResult{State: "closed", RootJob: "crit9", GateRunID: "g1"}, 0, nil
	}
	pending, reads = true, nil
	code, result = b.do("review", "commit", "abc1234", "--goal", "standing-validation")
	expectOutcome(t, "pending read dispatch", code, result, intentInProgress)
	if len(reads) != 1 || result.Decision == "" {
		t.Fatalf("a pending read dispatch is reported, not retried: %v %+v", reads, result)
	}
}

func TestIntentFoldComposesTheReviewedDecision(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "impl1", "role": "implementer", "status": "completed", "round": 1, "goalId": "standing-validation"})
	b.writeJob(map[string]any{"jobId": "crit1", "role": "code-critic", "status": "completed", "round": 1, "reviews": "impl1"})
	b.writeReturn("crit1", 1, "crit1", map[string]any{"id": "F1", "material": true})
	dispositions := filepath.Join(b.root(), "d.md")
	b.writeFile(dispositions, deliveryDispositionsHeader+"| F1 | accepted | real defect | fold it |\n")
	brief := filepath.Join(b.root(), "fix.md")
	b.writeFile(brief, "Working Mode: fix\n\n# Goal\n\nFix F1.\n")
	var messages []string
	b.handler = func(process intentProcess) intentProcessResult {
		if process.argv[2] != "--follow-up" || process.argv[3] != "impl1" {
			t.Fatalf("follow-up %v", process.argv)
		}
		text, _ := os.ReadFile(flagValue(process.argv, "--brief"))
		messages = append(messages, string(text))
		return intentProcessResult{stdout: []byte(`{"outcome":"WON","headline":"started","jobId":"impl1-r2"}`)}
	}
	for range 2 {
		code, result := b.do("fold", "review", "crit1", "--dispositions", dispositions, "--brief", brief)
		expectOutcome(t, "fold", code, result, intentInProgress)
	}
	if len(messages) != 2 || messages[0] != messages[1] || !strings.HasPrefix(messages[0], "Working Mode: fix") ||
		!strings.Contains(messages[0], `"id":"F1"`) || !strings.Contains(messages[0], "| F1 | accepted |") || !strings.Contains(messages[0], "crit1 round 1") {
		t.Fatalf("the follow-up must carry the brief, the exact return and the dispositions, stably: %q", messages)
	}
	if original, _ := os.ReadFile(brief); string(original) != "Working Mode: fix\n\n# Goal\n\nFix F1.\n" {
		t.Fatal("the caller's brief was modified")
	}
}

const deliveryDispositionsHeader = "| Finding id | Disposition | Reasoning and evidence | Amendment |\n| --- | --- | --- | --- |\n"

// realCloseOwner installs the actual scripts/agents/dispatch.sh and a built
// engine into the bed. METASYSTEM_BIN is a wrapper that answers only the
// brain fence and the lease (a human holder whose held commands run directly);
// every other engine verb is the real one.
func realCloseOwner(t *testing.T, b *deliveryBed) {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("cp", "-R", filepath.Join(source, "scripts"), b.install).CombinedOutput(); err != nil {
		t.Fatalf("install scripts: %v: %s", err, output)
	}
	engine := filepath.Join(t.TempDir(), "metasystem")
	if output, err := exec.Command("go", "build", "-o", engine, ".").CombinedOutput(); err != nil {
		t.Fatalf("build engine: %v: %s", err, output)
	}
	wrapper := filepath.Join(t.TempDir(), "metasystem-close-fixture")
	b.writeFile(wrapper, `#!/bin/sh
case "$1 $2" in
"brain fence") exit 0 ;;
"lease require-holder"|"lease classify") printf '%s\n' '{"claimEpoch":"","mainId":"","class":"HUMAN"}'; exit 0 ;;
"lease run-held") while [ "$1" != "--" ]; do shift; done; shift; exec "$@" ;;
esac
exec "`+engine+`" "$@"
`)
	if err := os.Chmod(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_BIN", wrapper)
	b.owners.process = func(process intentProcess) intentProcessResult {
		b.calls = append(b.calls, process.argv)
		return runIntentOwnerProcess(process)
	}
}

func TestIntentCloseWholeOwner(t *testing.T) {
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "crit1", "role": "code-critic", "status": "completed", "round": 1, "reviews": "impl1", "findingRegister": []any{}})
	b.writeReturn("crit1", 1, "crit1", map[string]any{"id": "F1", "material": true}, map[string]any{"id": "F2", "material": false})
	partial := filepath.Join(b.root(), "partial.md")
	b.writeFile(partial, deliveryDispositionsHeader+"| F1 | accepted | fixed in round 2 | none |\n")
	code, result := b.do("close", "crit1", "--dispositions", partial)
	expectOutcome(t, "unjoined dispositions", code, result, intentRefused)
	if len(b.calls) != 0 || !strings.Contains(fmt.Sprint(result.Data), "F2") {
		t.Fatalf("an incomplete join refuses before the close owner: %+v", result)
	}

	realCloseOwner(t, b)
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	if err := os.MkdirAll(filepath.Join(b.install, "artifacts", "agents", "record-locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	b.writeJob(map[string]any{"jobId": "inv1", "role": "investigator", "status": "completed", "round": 1,
		"destructiveReach": "MECHANICAL", "dispatchMode": "fresh", "sessionId": "inv1-session", "endedAt": "2026-09-25T10:00:00Z",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": nil,
		"configurationObligations": map[string]any{"builderEffortTier": "ordinary", "builderReasoningEffort": "medium",
			"independentCritiqueRequired": false, "independentCritiqueEffortTier": "none", "independentCritiqueReasoningEffort": "none", "liveProofRequired": false}})
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "inv1", "rounds", "1", "return.json"), `{"jobId":"inv1","round":1}`)
	conf := filepath.Join(b.install, "metasystem.conf")
	existing, _ := os.ReadFile(conf)

	// Without an evidence root the owner cannot mirror, so its close check
	// refuses and nothing is stamped.
	code, result = b.do("close", "inv1")
	expectOutcome(t, "owner refusal", code, result, intentRefused)
	if closed, _ := b.job("inv1")["chainClosed"].(bool); closed || result.Decision == "" {
		t.Fatalf("an owner refusal leaves the chain open: %+v", result)
	}
	evidence := t.TempDir()
	b.writeFile(conf, string(existing)+"\nevidence.root="+evidence+"\n")
	code, result = b.do("close", "inv1")
	if result.Outcome != intentConfirmed {
		t.Fatalf("whole close: exit %d %+v", code, result)
	}
	record := b.job("inv1")
	if closed, _ := record["chainClosed"].(bool); !closed || record["mirror"] == nil || record["runnerClosed"] != nil {
		t.Fatalf("the owner mirrors and stamps the chain closed itself: %v", record)
	}
	if _, err := os.Stat(filepath.Join(b.install, "artifacts", "agents", "locks", "inv1.d")); !os.IsNotExist(err) {
		t.Fatalf("the chain lock outlived the owner: %v", err)
	}
	calls := len(b.calls)
	code, result = b.do("close", "inv1")
	expectOutcome(t, "repeat close", code, result, intentUnchanged)
	if len(b.calls) != calls {
		t.Fatal("a closed chain is not closed again")
	}
	// A review chain: the author's dispositions record the round's
	// out-of-scope finding in the register, then the real owner closes the
	// register and the chain.
	// crit2 and crit3 are two critique chains of one design, so a review of
	// that design refuses to choose between them.
	roots, err := project.ResolveRoots(b.install)
	if err != nil {
		t.Fatal(err)
	}
	var home string
	for _, candidate := range project.Homes(roots) {
		if candidate.Kind == project.KindDesign && candidate.Glob == "" {
			home = candidate.Path
		}
	}
	design := filepath.Join(home, "two-chains.md")
	b.writeFile(design, "# Two chains\n\n- Kind: design\n- Id: 01DESIGNTWOCHAINS\n- Status: draft\n- Goals: standing-validation\n\nA design.\n")
	canonical, _ := filepath.EvalSymlinks(design)
	b.writeJob(map[string]any{"jobId": "crit2", "role": "design-critic", "status": "completed", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh", "sessionId": "crit2-session", "endedAt": "2026-09-25T11:00:00Z",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": nil, "findingRegister": []any{},
		"goalId": "standing-validation", "design": canonical, "reviewRoundLimit": 2})
	b.writeJob(map[string]any{"jobId": "crit3", "role": "design-critic", "status": "completed", "round": 1, "parentJob": nil,
		"goalId": "standing-validation", "design": canonical})
	b.writeReturn("crit2", 1, "crit2", map[string]any{"id": "C1", "material": false})
	criticDispositions := filepath.Join(b.root(), "crit2.md")
	b.writeFile(criticDispositions, deliveryDispositionsHeader+"| C1 | noted | wording only | none |\n")
	code, result = b.do("close", "crit2", "--dispositions", criticDispositions)
	expectOutcome(t, "unfolded critic round", code, result, intentRefused)
	if !strings.Contains(fmt.Sprint(result.Data), "folded through round 0 while terminal round 1 exists") {
		t.Fatalf("the real close check refuses an unfolded round: %+v", result)
	}
	// The reap owner folds each finished round into the register.
	if advanced, err := dispatchcore.CritiqueRegisterAdvance(b.install, "crit2", "crit2"); err != nil || advanced != "advanced" {
		t.Fatalf("register advance = %q, %v", advanced, err)
	}
	// The multi-chain refusal offers the public close of each chain; the
	// root and file are substituted into exactly the printed command.
	_, refused := b.do("review", "design", design, "--tool-calls", "30")
	printed := "metasystem review job ROOT --dispositions FILE"
	if refused.Outcome != intentRefused || !strings.Contains(refused.Decision, printed) || strings.Contains(refused.Decision, "metasystem close") {
		t.Fatalf("the multi-chain refusal: %+v", refused)
	}
	followed := strings.Fields(strings.NewReplacer("ROOT", "crit2", "FILE", criticDispositions).Replace(printed))
	// The reference a person copies is the one status work prints.
	_, listed := b.do("status", "work", "--all")
	reference := ""
	for _, job := range listed.Data.(map[string]any)["jobs"].([]any) {
		if view := job.(map[string]any); strings.HasSuffix(fmt.Sprint(view["reference"]), ":crit2") {
			reference = view["reference"].(string)
		}
	}
	if reference != "j2:crit2" {
		t.Fatalf("status work lists crit2 as %q: %+v", reference, listed)
	}
	code, result = b.do(followed[1:]...)
	if code, again := b.do("review", "job", reference, "--dispositions", criticDispositions); code != 0 || again.Outcome != intentUnchanged {
		t.Fatalf("the qualified reference reaches the same closed chain: code=%d %+v", code, again)
	}
	expectOutcome(t, "critic chain close", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "accepts no design") {
		t.Fatalf("a closed review chain must not read as acceptance: %+v", result)
	}
	critic := b.job("crit2")
	if closed, _ := critic["chainClosed"].(bool); !closed || critic["findingRegisterRound"] != float64(1) || critic["mirror"] == nil || critic["runnerClosed"] != nil {
		t.Fatalf("the real owner closes the folded critic chain: %v", critic)
	}
	_, result = b.do("close", "inv1", "--dispositions", partial)
	if result.Outcome != intentUnchanged {
		t.Fatalf("closed first: %+v", result)
	}
}

// landingOwners stands in for the landing owners' effects: each keeps the
// state its owner would record, so repeats read what an earlier call did.
type landingOwners struct {
	joins         []batchJoinRequest
	joinErr       error
	member        *batch.Unit
	candidates    int
	preps, pushes [][]string
	red           string
	pushErr       []error
	sweepErr      []error
	sweeps        int
	receiptExit   int
	receiptTrees  []string
	status        intentBranchState
	configured    bool
	branchDeleted bool
}

func (l *landingOwners) install(b *deliveryBed) {
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return "/landing", l.configured, nil }
	b.owners.batchUnit = func(string, batchJoinRequest, string) (batch.Record, batch.Unit, bool, error) {
		if l.member == nil {
			return batch.Record{}, batch.Unit{}, false, nil
		}
		return batch.Record{BatchID: "b-1", State: "open"}, *l.member, true, nil
	}
	b.owners.batchJoin = func(request batchJoinRequest) (batch.Record, error) {
		l.joins = append(l.joins, request)
		if l.joinErr != nil {
			return batch.Record{}, l.joinErr
		}
		l.member = &batch.Unit{GoalID: request.GoalID, Chain: request.ChainID, State: batch.UnitJoined}
		return batch.Record{BatchID: "b-1", State: "open"}, nil
	}
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		if l.branchDeleted {
			return intentBranchState{EndpointTip: l.status.EndpointTip}, nil
		}
		return l.status, nil
	}
	candidateFor := func() string { return "cand-" + l.status.EndpointTip[:4] }
	b.owners.landCandidate = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		l.candidates++
		return goalBranchLandPrepOutcome{Result: branch.LandResult{Candidate: candidateFor()}}, 0, nil
	}
	b.owners.landPrep = func(args []string) (goalBranchLandPrepOutcome, int, error) {
		l.preps = append(l.preps, args)
		var receipt landing.TestReceipt
		encoded, err := os.ReadFile(flagValue(args, "--test-receipt"))
		if err != nil || json.Unmarshal(encoded, &receipt) != nil || receipt.Tree != candidateFor() {
			return goalBranchLandPrepOutcome{}, 1, errors.New("GOAL_LAND_UNPROVEN: receipt does not prove the candidate workspace")
		}
		if err := os.MkdirAll(flagValue(args, "--out"), 0o755); err != nil {
			return goalBranchLandPrepOutcome{}, 1, err
		}
		return goalBranchLandPrepOutcome{Result: branch.LandResult{Landing: "land1", Candidate: candidateFor()}, Classification: l.red}, 0, nil
	}
	b.owners.landPush = func(args []string) (branch.PreparedLanding, string, int, error) {
		l.pushes = append(l.pushes, args)
		if len(l.pushErr) > 0 {
			err := l.pushErr[0]
			l.pushErr = l.pushErr[1:]
			if err != nil {
				return branch.PreparedLanding{}, "", 1, err
			}
		}
		pushed := branch.PreparedLanding{Landing: "land1", Branch: "goal/standing-validation"}
		if err := l.sweep(); err != nil {
			return pushed, "main", 1, err
		}
		return pushed, "main", 0, nil
	}
	b.owners.sweep = func(_, _, landingCommit string) error {
		if landingCommit != "land1" {
			return errors.New("sweep of an unknown landing")
		}
		return l.sweep()
	}
	b.handler = func(process intentProcess) intentProcessResult {
		if !slices.Equal(process.argv[1:3], []string{"landing", "test-receipt"}) || flagValue(process.argv, "--mode") != "auto" {
			panic("unexpected owner " + strings.Join(process.argv, " "))
		}
		l.receiptTrees = append(l.receiptTrees, flagValue(process.argv, "--tree"))
		if l.receiptExit != 0 {
			return intentProcessResult{code: l.receiptExit, stderr: []byte("proof refused\n")}
		}
		return intentProcessResult{stdout: []byte(`{"schemaVersion":3,"tree":"` + flagValue(process.argv, "--tree") + `","time":"2026-09-25T12:00:00Z","exitStatus":0}`)}
	}
}

// sweep is the branch owner's merged-branch deletion; a failure leaves the
// branch in place.
func (l *landingOwners) sweep() error {
	l.sweeps++
	if len(l.sweepErr) > 0 {
		err := l.sweepErr[0]
		l.sweepErr = l.sweepErr[1:]
		if err != nil {
			return err
		}
	}
	l.branchDeleted = true
	return nil
}

func readBranch(prefix int, sources ...string) intentBranchState {
	return intentBranchState{EndpointTip: strings.Repeat("e", 40), BranchTip: strings.Repeat("2", 40), Sources: sources,
		Status: branch.Status{Prefix: prefix, Units: []branch.UnitStatus{{Unit: "u1", Commit: strings.Repeat("1", 40)}, {Unit: "u2", Commit: strings.Repeat("2", 40)}}}}
}

func TestIntentLandRouteEvidence(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{configured: true, status: readBranch(2, "critic-root", "critic-root")}
	owners.install(b)

	code, result := b.do("land", "standing-validation")
	expectOutcome(t, "critic-root evidence joins the batch", code, result, intentInProgress)
	if len(owners.joins) != 1 || !owners.joins[0].Last || owners.joins[0].LandingRoot != "/landing" || owners.candidates != 0 {
		t.Fatalf("configured batch with critic-root evidence routes to the batch only: %+v", owners.joins)
	}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "repeat reads the membership", code, result, intentInProgress)
	if len(owners.joins) != 1 || result.Next == nil || result.Data.(map[string]any)["joinedNow"] != false {
		t.Fatalf("a repeated land must not join twice: %+v", result)
	}
	owners.member.State = batch.UnitLanded
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "batch landed", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "stays open") {
		t.Fatalf("landing never concludes the goal: %+v", result)
	}

	owners.member, owners.joinErr = nil, errors.New("BATCH_JOIN_UNREAD: goal is not read clean through its branch tip")
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "batch refusal", code, result, intentRefused)
	if owners.candidates != 0 || len(b.calls) != 0 {
		t.Fatalf("a batch refusal never falls back to the hand route: %+v", result)
	}

	owners.joinErr = nil
	owners.status = readBranch(2, "critic-root", "reader-record")
	joins := len(owners.joins)
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "reader-record evidence lands by hand", code, result, intentConfirmed)
	if len(owners.joins) != joins || owners.candidates != 1 || !slices.Equal(owners.receiptTrees, []string{"cand-eeee"}) || len(owners.pushes) != 1 {
		t.Fatalf("hand-reader evidence takes the hand route and proves the composed candidate: %v", owners.receiptTrees)
	}
	if result.Data.(map[string]any)["batchConfigured"] != true {
		t.Fatalf("the route is chosen with the batch configured: %+v", result)
	}

	owners.configured = false
	b.writeJob(map[string]any{"jobId": "impl1", "role": "implementer", "status": "completed", "goalId": "standing-validation"})
	code, result = b.do("land", "job", "impl1")
	expectOutcome(t, "chain without batch root", code, result, intentRefused)
	if !strings.Contains(result.Decision, "landing.batch-root") {
		t.Fatalf("a chain without batch policy names the missing input: %+v", result)
	}
}

func TestIntentLandRecovery(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	owners := &landingOwners{status: readBranch(1, "reader-record")}
	owners.install(b)
	code, result := b.do("land", "standing-validation")
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if result.Next == nil || !slices.Equal(result.Next.Argv[1:4], []string{"review", "commit", strings.Repeat("2", 40)}) || owners.candidates != 0 {
		t.Fatalf("an unread unit names its read and proves nothing: %+v", result)
	}

	owners.status = readBranch(2, "reader-record", "reader-record")
	owners.pushErr = []error{errors.New("push rejected: endpoint moved")}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "unpushed", code, result, intentPartial)
	if result.Next == nil || !slices.Equal(result.Next.Argv[:3], []string{"metasystem", "land", "standing-validation"}) || len(owners.preps) != 1 {
		t.Fatalf("a failed push keeps the prepared landing and names the retry: %+v", result)
	}
	owners.sweepErr = []error{errors.New("sweep: remote refused the branch deletion")}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "pushed, unswept", code, result, intentPartial)
	if len(owners.receiptTrees) != 1 || len(owners.preps) != 1 || len(owners.pushes) != 2 || owners.branchDeleted {
		t.Fatalf("the retry reuses receipt and preparation and reports the pending sweep: %+v", result)
	}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "resumed sweep", code, result, intentConfirmed)
	if owners.sweeps != 2 || len(owners.pushes) != 2 || !owners.branchDeleted {
		t.Fatalf("the repeat resumes the sweep, never pushes again: %+v", result)
	}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "landed and branch gone", code, result, intentUnchanged)
	if owners.sweeps != 2 || len(owners.pushes) != 2 {
		t.Fatalf("a swept landing is read before the missing branch: %+v", result)
	}

	owners.branchDeleted = false
	owners.status = readBranch(2, "reader-record", "reader-record")
	owners.status.EndpointTip = strings.Repeat("f", 40)
	owners.red = "goal-red"
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "red proof", code, result, intentRefused)
	if owners.receiptTrees[len(owners.receiptTrees)-1] != "cand-ffff" || len(owners.pushes) != 2 || result.Data.(map[string]any)["classification"] != "goal-red" {
		t.Fatalf("a moved endpoint takes a new candidate proof, and a red one pushes nothing: %+v", result)
	}
	owners.status.EndpointTip = strings.Repeat("a", 40)
	owners.receiptExit = 3
	preps := len(owners.preps)
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "proof refused", code, result, intentRefused)
	if len(owners.preps) != preps || !strings.Contains(result.Summary, "no schema-3 receipt") {
		t.Fatalf("no receipt, no preparation: %+v", result)
	}
}

// memberUnit is a goal-branch batch member as the batch join records it: its
// chain is the branch tip and its builds carry the selected unit commits.
func memberUnit(tip string, last bool, commits ...string) batch.Unit {
	unit := batch.Unit{GoalID: "standing-validation", Chain: tip, State: batch.UnitJoined, GoalLast: last, BranchTip: tip,
		Claim: batch.Claim{Machine: "m1", Lineage: "lineage-1", Epoch: 1, Revision: 1, AccountingRevision: 1}}
	for _, commit := range commits {
		unit.Builds = append(unit.Builds, batch.BranchBuild{Units: []string{"u-" + commit[:4]}, Commit: commit})
	}
	return unit
}

func TestIntentLandBatchMemberSelection(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	landingRoot := t.TempDir()
	store := batch.NewStore(landingRoot, identity.KernelProber{})
	first, second := strings.Repeat("1", 40), strings.Repeat("2", 40)
	prefix := batch.Record{Schema: 1, BatchID: "01k0000000000000000000000a", State: batch.StateOpen, Units: []batch.Unit{memberUnit(first, false, first)}}
	if err := store.Create(prefix); err != nil {
		t.Fatal(err)
	}
	markBatchLanded(t, landingRoot, prefix.BatchID)
	joins := 0
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return landingRoot, true, nil }
	b.owners.batchUnit = productionIntentBatchUnit
	b.owners.batchJoin = func(batchJoinRequest) (batch.Record, error) {
		joins++
		return batch.Record{}, errors.New("unexpected join")
	}
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{EndpointTip: strings.Repeat("e", 40)}, nil // the goal branch is gone
	}

	code, result := b.do("land", "standing-validation", "--through", first)
	expectOutcome(t, "landed prefix after branch deletion", code, result, intentConfirmed)
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "full request after a landed prefix", code, result, intentRefused)
	if !strings.Contains(result.Summary, "origin has no goal/standing-validation") || joins != 0 {
		t.Fatalf("an earlier landed prefix must not answer a whole-goal request: %+v", result)
	}
	full := batch.Record{Schema: 1, BatchID: "01k0000000000000000000000b", State: batch.StateOpen, Units: []batch.Unit{memberUnit(second, true, first, second)}}
	if err := store.Create(full); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "joined whole goal", code, result, intentInProgress)
	if joins != 0 || result.Data.(map[string]any)["batchId"] != full.BatchID {
		t.Fatalf("the whole-goal member is read, never joined again: %+v", result)
	}
	markBatchLanded(t, landingRoot, full.BatchID)
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "whole goal landed, branch deleted", code, result, intentConfirmed)
}

func TestIntentReviewCommitClosesThenPublishes(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.writeJob(map[string]any{"jobId": "crit9", "role": "code-critic", "status": "completed", "round": 1, "reviews": "commit:" + strings.Repeat("3", 40), "findingRegister": []any{}})
	b.writeReturn("crit9", 1, "crit9", map[string]any{"id": "F1", "material": true})
	var reads [][]string
	b.owners.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		reads = append(reads, args)
		if slices.Contains(args, "--collect") {
			return branch.BranchReadResult{State: "collected", RootJob: "crit9", GateRunID: "g1", AttestationCommit: "attest1"}, 0, nil
		}
		return branch.BranchReadResult{State: "closed", RootJob: "crit9", GateRunID: "g1"}, 0, nil
	}
	publishes := 0
	b.owners.publishRead = func(root, goalID, unit string) (branch.PublishReadResult, error) {
		publishes++
		if publishes == 1 {
			return branch.PublishReadResult{Attestation: "attest1", OpID: branch.PublishOperationID("g1")}, errors.New("push rejected: connection reset")
		}
		return branch.PublishReadResult{Attestation: "attest1", OpID: branch.PublishOperationID("g1"), State: "pushed", RemoteTip: "attest1"}, nil
	}
	code, result := b.do("review", "commit", "abc1234", "--goal", "standing-validation", "--model", "gpt-critic")
	expectOutcome(t, "terminal but unclosed critic", code, result, intentInProgress)
	if len(reads) != 1 || !slices.Contains(reads[0], "gpt-critic") || publishes != 0 ||
		!strings.Contains(result.Decision, "review commit abc1234 --goal standing-validation --dispositions FILE") || strings.Contains(result.Decision, "gpt-critic") || strings.Contains(result.Decision, "metasystem close") ||
		result.Data.(map[string]any)["material"] != float64(1) {
		t.Fatalf("an unclosed critic names the author's close, then the same review; nothing is collected: %v %+v", reads, result)
	}
	subject, _ := json.Marshal(readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: strings.Repeat("3", 40), Parent: strings.Repeat("4", 40), Tree: strings.Repeat("5", 40), DiffDigest: strings.Repeat("6", 64)})
	var subjectObject map[string]any
	_ = json.Unmarshal(subject, &subjectObject)
	// The author's decisions close the review through the whole close owner
	// (the bed's close process stamps the closure), then it is collected.
	b.handler = func(process intentProcess) intentProcessResult {
		record := b.job("crit9")
		record["chainClosed"] = true
		record["closure"] = map[string]any{"criticRoot": "crit9", "round": 1, "subject": subjectObject, "mechanism": "clean"}
		b.writeJob(record)
		return intentProcessResult{}
	}
	decisions := filepath.Join(b.root(), "crit9-decisions.md")
	b.writeFile(decisions, deliveryDispositionsHeader+"| F1 | refuted | the test at x_test.go:12 covers it | none |\n")
	code, result = b.do("review", "commit", "abc1234", "--goal", "standing-validation", "--model", "gpt-critic", "--dispositions", decisions)
	if len(b.calls) != 1 || filepath.Base(b.calls[0][0]) != "dispatch.sh" || b.calls[0][1] != "close" {
		t.Fatalf("the decided review did not run the whole close owner: %v %+v", b.calls, result)
	}
	expectOutcome(t, "collected, publication lost", code, result, intentPartial)
	if len(reads) != 3 || !slices.Contains(reads[2], "--collect") || result.Next == nil || !strings.Contains(result.Next.Reason, "no critic or commit is repeated") {
		t.Fatalf("a lost publication is partial with the same command: %v %+v", reads, result)
	}
	b.owners.branchRead = func(args []string) (branch.BranchReadResult, int, error) {
		reads = append(reads, args)
		return branch.BranchReadResult{State: "already-collected", RootJob: "crit9", GateRunID: "g1", AttestationCommit: "attest1"}, 0, nil
	}
	// The printed continuation is the exact subject's review; the decided
	// call's model and dispositions are not repeated.
	if want := []string{"metasystem", "review", "commit", "abc1234", "--goal", "standing-validation"}; !slices.Equal(result.Next.Argv, want) {
		t.Fatalf("the lost publication's continuation is %v, want %v", result.Next.Argv, want)
	}
	calls := len(b.calls)
	code, result = b.do(result.Next.Argv[1:]...)
	expectOutcome(t, "republished", code, result, intentConfirmed)
	if len(reads) != 4 || slices.Contains(reads[3], "--collect") || publishes != 2 || len(b.calls) != calls {
		t.Fatalf("the repeat publishes the retained attestation without collecting again: %v", reads)
	}

	calls = len(b.calls)
	for _, args := range [][]string{
		{"review", "commit", "abc1234", "--goal", "standing-validation", "--effort", "high"},
		{"review", "job", "impl1", "--model", "gpt-critic", "--tool-calls", "20"},
		{"review", "design", "plans/designs/x.md", "--model", "gpt-critic", "--tool-calls", "20"},
	} {
		code, result := b.do(args...)
		expectOutcome(t, strings.Join(args, " "), code, result, intentRefused)
		if result.Decision == "" {
			t.Fatalf("an unsupported override names the decision: %+v", result)
		}
	}
	if len(b.calls) != calls || len(reads) != 4 {
		t.Fatal("an unsupported override reached an owner")
	}
}

// markBatchLanded records a batch as landed in the store's own on-disk form;
// the real store reader then loads it. Reaching landed through the batch
// owner's proof and push transitions is the owner's own test surface.
func markBatchLanded(t *testing.T, landingRoot, id string) {
	t.Helper()
	path := filepath.Join(landingRoot, "artifacts", "agents", "landing-batches", id+".json")
	var record map[string]any
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &record) != nil {
		t.Fatalf("batch record %s: %v", path, err)
	}
	record["state"] = batch.StateLanded
	for _, unit := range record["units"].([]any) {
		unit.(map[string]any)["state"] = batch.UnitLanded
	}
	encoded, _ := json.Marshal(record)
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A landed whole-goal member answers only for the branch tip it joined at:
// new work on a later goal branch is inspected and routed, never reported
// as the old landing.
func TestIntentLandOldWholeLandingDoesNotAnswerFreshBranch(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	landingRoot := t.TempDir()
	store := batch.NewStore(landingRoot, identity.KernelProber{})
	old, fresh := strings.Repeat("1", 40), strings.Repeat("7", 40)
	record := batch.Record{Schema: 1, BatchID: "01k0000000000000000000000c", State: batch.StateOpen, Units: []batch.Unit{memberUnit(old, true, old)}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	markBatchLanded(t, landingRoot, record.BatchID)
	var joins []batchJoinRequest
	b.owners.batchRoot = func(string, time.Time) (string, bool, error) { return landingRoot, true, nil }
	b.owners.batchUnit = productionIntentBatchUnit
	b.owners.batchJoin = func(request batchJoinRequest) (batch.Record, error) {
		joins = append(joins, request)
		return batch.Record{BatchID: "01k0000000000000000000000d", State: batch.StateOpen}, nil
	}
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{EndpointTip: strings.Repeat("e", 40), BranchTip: fresh, Sources: []string{"critic-root"},
			Status: branch.Status{Tip: fresh, Prefix: 1, Units: []branch.UnitStatus{{Unit: "u9", Commit: fresh}}}}, nil
	}
	code, result := b.do("land", "standing-validation")
	expectOutcome(t, "fresh branch after an old whole landing", code, result, intentInProgress)
	if len(joins) != 1 || !joins[0].Last || result.Data.(map[string]any)["joinedNow"] != true {
		t.Fatalf("the fresh branch must join as new work: %v %+v", joins, result)
	}
	failing := errors.New("goal branch unreadable")
	b.owners.branchState = func(string, string) (intentBranchState, error) { return intentBranchState{}, failing }
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "unreadable branch", code, result, intentRefused)
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{EndpointTip: strings.Repeat("e", 40)}, nil
	}
	code, result = b.do("land", "standing-validation")
	expectOutcome(t, "retained landing once the branch is gone", code, result, intentConfirmed)
	if len(joins) != 1 {
		t.Fatal("a retained landed member is not joined again")
	}
}
