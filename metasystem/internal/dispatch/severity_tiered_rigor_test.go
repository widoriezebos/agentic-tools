package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	critiqueModel "github.com/widoriezebos/agentic-tools/metasystem/internal/critique"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestSTR3Gap03OutputsGrammar(t *testing.T) {
	write := func(name, body string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for name, body := range map[string]string{
		"unsorted":  "metasystem/z.go\nmetasystem/a.go\n",
		"duplicate": "metasystem/a.go\nmetasystem/a.go\n",
		"parent":    "metasystem/../a.go\n",
	} {
		if _, err := ParseDeclaredOutputs(write(name, body)); err == nil || !strings.Contains(err.Error(), "line 2") && name != "parent" || name == "parent" && !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("%s grammar refusal = %v", name, err)
		}
	}
	path := write("canonical", "metasystem/a path.go\nmetasystem/z.go\n")
	got, err := ParseDeclaredOutputs(path)
	if err != nil || strings.Join(got, ",") != "metasystem/a path.go,metasystem/z.go" {
		t.Fatalf("canonical = %v, %v", got, err)
	}
}

func TestSTR2BSeamConstant(t *testing.T) {
	limit, err := goalReviewRoundLimit(t.TempDir(), "goal", 7)
	if err != nil || limit != 3 {
		t.Fatalf("seam = %d, %v", limit, err)
	}
}

func rigorWire(id, class, artifact string) []any {
	return []any{map[string]any{"findingId": id, "rigorClass": class, "facts": registerFacts(), "reopeningTrigger": "reopen on recurrence", "artifact": artifact}}
}

func TestMaterialArtifactMembershipAndDemotion(t *testing.T) {
	material := []any{registerFindingValue("F-1", true, "evidence")}
	for name, artifact := range map[string]string{"missing": "", "outside": "metasystem/outside.go"} {
		t.Run(name, func(t *testing.T) {
			register, demotions, err := foldCritiqueFindings(nil, "design-critic", "critic", material, rigorWire("F-1", "bounded", artifact), critiqueSubject{paths: map[string]bool{"metasystem/in.go": true}}, 1)
			if err != nil || len(register) != 0 || len(demotions) != 1 {
				t.Fatalf("fold = %+v, %+v, %v", register, demotions, err)
			}
		})
	}
	register, demotions, err := foldCritiqueFindings(nil, "design-critic", "critic", material, rigorWire("F-1", "bounded", "metasystem/in.go"), critiqueSubject{paths: map[string]bool{"metasystem/in.go": true}}, 1)
	if err != nil || len(register) != 1 || len(demotions) != 0 {
		t.Fatalf("in-set fold = %+v, %+v, %v", register, demotions, err)
	}
	register, demotions, err = foldCritiqueFindings(register, "design-critic", "critic-r2", material, rigorWire("F-1", "bounded", ""), critiqueSubject{paths: map[string]bool{"metasystem/in.go": true}}, 2)
	if err != nil || len(demotions) != 1 || register[0].Status != "open" {
		t.Fatalf("demotion withdrew open finding: %+v, %+v, %v", register, demotions, err)
	}
	withdrawn := []any{registerFindingValue("F-1", false, "fixed")}
	register, _, err = foldCritiqueFindings(register, "design-critic", "critic-r3", withdrawn, nil, critiqueSubject{paths: map[string]bool{}}, 3)
	if err != nil || register[0].Status != "resolved" || register[0].Resolution != "withdrawn" {
		t.Fatalf("explicit false did not withdraw: %+v, %v", register, err)
	}
}

func TestSTR2BRenameEitherSide(t *testing.T) {
	for _, member := range []string{"metasystem/old.go", "metasystem/new.go"} {
		register, demotions, err := foldCritiqueFindings(nil, "code-critic", "critic", []any{registerFindingValue("F-1", true, "evidence")}, rigorWire("F-1", "bounded", "metasystem/old.go=>metasystem/new.go"), critiqueSubject{paths: map[string]bool{member: true}}, 1)
		if err != nil || len(register) != 1 || len(demotions) != 0 {
			t.Fatalf("rename member %s = %+v, %+v, %v", member, register, demotions, err)
		}
	}
}

func TestCritiqueSubjectPrefixesProjectRelativeDiffPaths(t *testing.T) {
	gitRoot := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", gitRoot).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	repo := filepath.Join(gitRoot, "metasystem")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	writeJSONFile(t, jobs, "impl.json", map[string]any{
		"jobId": "impl", "role": "implementer", "round": 1, "parentJob": nil,
	})
	diffDir := filepath.Join(repo, "artifacts", "agents", "impl", "rounds", "1")
	if err := os.MkdirAll(diffDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		diff string
		want []string
	}{
		{
			name: "diff headers",
			diff: "diff --git a/cmd/metasystem/delegate.go b/cmd/metasystem/delegate.go\n",
			want: []string{"metasystem/cmd/metasystem/delegate.go"},
		},
		{
			name: "diff header with spaces",
			diff: "diff --git a/internal/file with space.go b/internal/file with space.go\n",
			want: []string{"metasystem/internal/file with space.go"},
		},
		{
			name: "rename headers",
			diff: "rename from internal/old.go\nrename to internal/new.go\n",
			want: []string{"metasystem/internal/old.go", "metasystem/internal/new.go"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(diffDir, "diff.patch"), []byte(test.diff), 0o644); err != nil {
				t.Fatal(err)
			}
			state := loadCritiqueState(repo)
			subject, err := critiqueSubjectForRound(repo, state, map[string]any{"reviews": "impl"}, "code-critic", map[string]any{"reviewedTree": "tree"})
			if err != nil {
				t.Fatal(err)
			}
			if len(subject.paths) != len(test.want) {
				t.Fatalf("subject paths = %v, want %v", subject.paths, test.want)
			}
			for _, want := range test.want {
				if !subject.paths[want] {
					t.Fatalf("subject paths = %v, missing %s", subject.paths, want)
				}
			}
		})
	}
}

func TestNEWPathAccepted(t *testing.T) {
	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	emptyTree, err := exec.Command("git", "-C", repo, "mktree").Output()
	if err != nil {
		t.Fatal(err)
	}
	subject := critiqueSubject{repoRoot: repo, tree: strings.TrimSpace(string(emptyTree)), paths: map[string]bool{"metasystem/new file.go": true}}
	register, demotions, err := foldCritiqueFindings(nil, "design-critic", "critic", []any{registerFindingValue("F-1", true, "evidence")}, rigorWire("F-1", "bounded", "NEW metasystem/new file.go"), subject, 1)
	if err != nil || len(register) != 1 || len(demotions) != 0 {
		t.Fatalf("NEW fold = %+v, %+v, %v", register, demotions, err)
	}
}

func TestChainClosesAtZeroMaterial(t *testing.T) {
	register, demotions, err := foldCritiqueFindings(nil, "design-critic", "critic", []any{}, []any{}, critiqueSubject{paths: map[string]bool{}}, 1)
	if err != nil || len(register) != 0 || len(demotions) != 0 {
		t.Fatalf("zero-material fold = %+v, %+v, %v", register, demotions, err)
	}
}

func TestRecurrenceClassesUnproven(t *testing.T) {
	facts := registerFacts()
	prior := registerFinding{FindingID: "old", Critic: "critic", RigorClass: critiqueModel.Bounded, FactsDigest: digestJSON(facts), Facts: facts, Artifact: "metasystem/in.go", Title: "old", Status: "deferred", Resolution: "deferred", DecisionOpID: "op", Evidence: "old evidence", EvidenceDigest: digestJSON("old evidence"), Multiplicity: 1}
	register, _, err := foldCritiqueFindings([]registerFinding{prior}, "design-critic", "critic-r2", []any{registerFindingValue("new", true, "new evidence")}, rigorWire("new", "bounded", "metasystem/in.go"), critiqueSubject{paths: map[string]bool{"metasystem/in.go": true}}, 2)
	if err != nil || len(register) != 2 || register[1].RigorClass != critiqueModel.Unproven {
		t.Fatalf("recurrence = %+v, %v", register, err)
	}
}

func TestCritiqueClosePrintsEveryBlockingFinding(t *testing.T) {
	repo := t.TempDir()
	root := map[string]any{
		"jobId": "critic", "role": "design-critic", "round": 1, "parentJob": nil, "status": "completed",
		findingRegisterRoundField: 1, reviewRoundLimitField: 3, criticRoundsConsumedField: 3, "demotions": []any{},
	}
	makeFinding := func(id, artifact string, class critiqueModel.RigorClass) registerFinding {
		return registerFinding{FindingID: id, Critic: "critic", RigorClass: class, FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(), Artifact: artifact, Title: id, Status: "open", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1}
	}
	root[findingRegisterField] = encodeFindingRegister([]registerFinding{
		makeFinding("S-1", "metasystem/one.go", critiqueModel.Severe),
		makeFinding("U-2", "metasystem/two.go", critiqueModel.Unproven),
	})
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", root)
	for name, err := range map[string]error{
		"register close": func() error { _, err := CritiqueRegisterClose(repo, "critic"); return err }(),
		"close check":    CloseCheck(repo, "critic"),
	} {
		if err == nil {
			t.Fatalf("%s accepted two blocking findings", name)
		}
		for _, want := range []string{"S-1 artifact=metasystem/one.go", "U-2 artifact=metasystem/two.go"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s did not print %q: %v", name, want, err)
			}
		}
		wantNext := "next: goal accept-risk --finding <id> --chain <root> --by <human> --why, or raise the goal budget and run job critique-budget-rebind"
		if !strings.HasSuffix(err.Error(), wantNext) {
			t.Fatalf("%s next step = %v", name, err)
		}
	}
}

func TestMalformedRoundAccountingNamesBudgetRebindNextStep(t *testing.T) {
	repo := t.TempDir()
	finding := registerFinding{
		FindingID: "B-1", Critic: "critic", RigorClass: critiqueModel.Bounded,
		FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(), Artifact: "metasystem/in.go",
		Title: "bounded", Status: "open", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1,
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", map[string]any{
		"jobId": "critic", "role": "design-critic", findingRegisterField: encodeFindingRegister([]registerFinding{finding}),
		findingRegisterRoundField: 1, reviewRoundLimitField: 3, criticRoundsConsumedField: "malformed",
	})
	_, err := CritiqueRegisterClose(repo, "critic")
	want := "next: job critique-budget-rebind --root-job critic"
	if err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("malformed accounting refusal = %v, want suffix %q", err, want)
	}
}

func TestRaiseByRebind(t *testing.T) {
	repo := revisionBindingBed(t, 2)
	root := map[string]any{"jobId": "critic", "role": "design-critic", "goalId": "bounded", reviewRoundLimitField: 1, criticRoundsConsumedField: 1}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic.json", root)
	if outcome, err := CritiqueBudgetRebind(repo, "critic"); err != nil || outcome != "rebound" {
		t.Fatalf("rebind = %q, %v", outcome, err)
	}
	got := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
	if limit, _ := numInt(got[reviewRoundLimitField]); limit != 3 {
		t.Fatalf("rebound limit = %v", got[reviewRoundLimitField])
	}
	binding, _ := got["critiqueBudgetBinding"].(map[string]any)
	if revision, _ := numInt(binding["goalRevision"]); revision != 2 || !strings.Contains(asString(binding["opid"]), "r2") {
		t.Fatalf("rebind provenance = %+v", binding)
	}
}

func TestCritiqueBudgetRebindBackfillsLegacyAccounting(t *testing.T) {
	repo := revisionBindingBed(t, 2)
	root := map[string]any{
		"jobId": "legacy-critic", "role": "design-critic", "round": 1, "parentJob": nil,
		"status": "completed", "goalId": "bounded", findingRegisterField: []any{}, findingRegisterRoundField: 1,
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "legacy-critic.json", root)
	if outcome, err := CritiqueBudgetRebind(repo, "legacy-critic"); err != nil || outcome != "rebound" {
		t.Fatalf("legacy rebind = %q, %v", outcome, err)
	}
	got := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "legacy-critic.json"))
	if limit, _ := numInt(got[reviewRoundLimitField]); limit != 3 {
		t.Fatalf("legacy rebound limit = %v", got[reviewRoundLimitField])
	}
	if consumed, _ := numInt(got[criticRoundsConsumedField]); consumed != 1 {
		t.Fatalf("legacy rebound consumed = %v", got[criticRoundsConsumedField])
	}
}

func TestCritiqueRegisterCloseKeepsRegisterlessCompatibility(t *testing.T) {
	repo := t.TempDir()
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "legacy-critic.json", map[string]any{
		"jobId": "legacy-critic", "role": "code-critic", "round": 1, "parentJob": nil, "status": "completed",
	})
	if outcome, err := CritiqueRegisterClose(repo, "legacy-critic"); err != nil || outcome != "closed" {
		t.Fatalf("register-less close = %q, %v", outcome, err)
	}
}

func TestSTR2BCloseOneWrite(t *testing.T) {
	repo := revisionBindingBed(t, 2)
	chain := "critic-crash"
	finding := registerFinding{FindingID: "B-1", Critic: chain, RigorClass: critiqueModel.Bounded, FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(), Artifact: "NEW metasystem/a file.go", Title: "bounded title", Status: "open", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1}
	root := map[string]any{"jobId": chain, "role": "design-critic", "goalId": "bounded", "machineId": "bed-m1", "mainId": "coordinator", "claimEpoch": 7, findingRegisterRoundField: 1, reviewRoundLimitField: 3, criticRoundsConsumedField: 3, "demotions": []any{}, findingRegisterField: encodeFindingRegister([]registerFinding{finding})}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), chain+".json", root)
	endpoint, err := goal.ResolveEndpoint(repo)
	if err != nil {
		t.Fatal(err)
	}
	req := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: "bed-m1", Lineage: "coordinator"}, Ulid: deterministicULID(chain), Now: time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC), ClaimEpoch: 7}
	obligation := goal.ReviewObligation{Finding: "B-1", Chain: chain, Artifact: finding.Artifact, Test: "prove: " + finding.Title, State: "open"}
	if _, err := goal.DeferFindings(req, "bounded", []goal.ReviewObligation{obligation}); err != nil {
		t.Fatalf("simulated ledger-first step = %v", err)
	}
	if outcome, err := CritiqueRegisterClose(repo, chain); err != nil || outcome != "deferred" {
		t.Fatalf("close recovery = %q, %v", outcome, err)
	}
	if outcome, err := CritiqueRegisterClose(repo, chain); err != nil || outcome != "closed" {
		t.Fatalf("close replay = %q, %v", outcome, err)
	}
	projection, err := goal.Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["bounded"]
	if file == nil || len(file.ReviewObligations) != 1 || file.ReviewObligations[0] != obligation {
		t.Fatalf("obligation replay duplicated or changed: %+v", file)
	}
}

func TestSTR3GapOOSWrite(t *testing.T) {
	repo := t.TempDir()
	root := map[string]any{"jobId": "critic", "role": "design-critic"}
	bounded := registerFinding{FindingID: "B-1", Critic: "critic", RigorClass: critiqueModel.Bounded, FactsDigest: digestJSON(registerFacts()), Facts: registerFacts(), Artifact: "metasystem/in.go", Title: "bounded", Status: "open", Evidence: "proof", EvidenceDigest: digestJSON("proof"), Multiplicity: 1}
	root[findingRegisterField] = encodeFindingRegister([]registerFinding{bounded})
	path := filepath.Join(repo, "artifacts", "agents", "jobs")
	writeJSONFile(t, path, "critic.json", root)
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"B-1"}); err != nil {
		t.Fatal(err)
	}
	first := readJSONFile(t, filepath.Join(path, "critic.json"))
	if err := CritiqueRegisterResolveOutOfScope(repo, "critic", []string{"B-1"}); err != nil {
		t.Fatal(err)
	}
	second := readJSONFile(t, filepath.Join(path, "critic.json"))
	if string(canonicalJSON(first)) != string(canonicalJSON(second)) {
		t.Fatal("idempotent out-of-scope rerun wrote again")
	}
	severe := bounded
	severe.FindingID, severe.RigorClass, severe.Status, severe.Resolution = "S-1", critiqueModel.Severe, "open", ""
	root[findingRegisterField] = encodeFindingRegister([]registerFinding{severe})
	writeJSONFile(t, path, "severe.json", root)
	before := readJSONFile(t, filepath.Join(path, "severe.json"))
	if err := CritiqueRegisterResolveOutOfScope(repo, "severe", []string{"S-1"}); err == nil || !strings.Contains(err.Error(), "S-1") {
		t.Fatalf("severe out-of-scope = %v", err)
	}
	after := readJSONFile(t, filepath.Join(path, "severe.json"))
	if string(canonicalJSON(before)) != string(canonicalJSON(after)) {
		t.Fatal("severe refusal changed register")
	}
}
