package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConcludedRecordsParseFromBothSoakLocations(t *testing.T) {
	legacy := vGoal("legacy-done", StateDone)
	recorded := vGoal("recorded-done", StateDone)
	files := vTree(vRoot(), nil, []*GoalFile{recorded})
	files[legacyDonePrefix+legacy.Id+".md"] = RenderFile(legacy)

	tree, problems := ParseTreeFiles(files)
	if len(problems) != 0 {
		t.Fatalf("both concluded-goal locations parse during the soak: %v", problems)
	}
	if validation := ValidateTree(tree); len(validation) != 0 {
		t.Fatalf("both concluded-goal locations validate together: %v", validation)
	}
	if tree.DonePaths[legacy.Id] != legacyDonePrefix+legacy.Id+".md" ||
		tree.DonePaths[recorded.Id] != recordsGoalsPrefix+recorded.Id+".md" {
		t.Fatalf("the parser retains each record's source location: %+v", tree.DonePaths)
	}

	files[legacyDonePrefix+recorded.Id+".md"] = RenderFile(recorded)
	_, problems = ParseTreeFiles(files)
	if !problemsContain(problems, "also present") {
		t.Fatalf("one identifier in both archives must refuse: %v", problems)
	}
}

func TestRecordsArchiveUsesTheSameIntegrityValidation(t *testing.T) {
	files := vTree(vRoot(), nil, []*GoalFile{vGoal("recorded", StateDone)})
	p := recordsGoalsPrefix + "recorded.md"
	files[p] = []byte(strings.Replace(string(files[p]), "Shipped and verified.", "Changed after rendering.", 1))
	_, problems := ParseTreeFiles(files)
	if !problemsContain(problems, p) || !problemsContain(problems, "Integrity mismatch") {
		t.Fatalf("a tampered records-owned conclusion must name its path and integrity failure: %v", problems)
	}
}

func TestLegacyArchiveIsReadOnlyButItsRecordsCanReopen(t *testing.T) {
	repo := soloLedgerRepo(t)
	endpoint := Endpoint{Root: repo, Remote: "local", Branch: "refs/heads/main"}
	accepted, err := goalGit(repo, nil, "rev-parse", AcceptedRef)
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(accepted)
	legacy := vGoal("legacy-finished", StateDone)
	commit, err := BuildCommit(endpoint, "fixture-legacy", base, []Change{{
		Path: legacyDonePrefix + legacy.Id + ".md", Content: RenderFile(legacy),
	}}, "fixture legacy conclusion")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goalGit(repo, nil, "update-ref", LocalLedgerBranch, commit, base); err != nil {
		t.Fatal(err)
	}
	if err := AdvanceAccepted(repo, commit); err != nil {
		t.Fatal(err)
	}

	projection, err := Project(endpoint, false, time.Now().UTC())
	if err != nil || projection.Tree.Done[legacy.Id] == nil {
		t.Fatalf("the projection must read the legacy conclusion during the soak: %+v %v", projection.Tree, err)
	}
	request := VerbRequest{
		Endpoint: endpoint, Actor: Actor{Machine: "mac-solo", Lineage: "lin-1"},
		Ulid: "01J5X00000000000000000HG10", Now: time.Date(2026, 8, 29, 8, 0, 0, 0, time.UTC),
	}
	result, err := Reopen(request, legacy.Id)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reopen must move a legacy record through the ledger: %+v %v", result, err)
	}
	files, err := ReadCommitGoals(repo, result.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if _, remains := files[legacyDonePrefix+legacy.Id+".md"]; remains {
		t.Fatal("reopen leaves no legacy archive file")
	}
	reopened := files[livePath(legacy.Id)]
	parsed, problems := ParseFile(reopened)
	if len(problems) != 0 || parsed == nil || parsed.History[len(parsed.History)-1].Verb != "reopen" {
		t.Fatalf("the live record must carry the reopen History event: %+v %v", parsed, problems)
	}
}

func TestEngineRefusesNewLegacyArchiveWrites(t *testing.T) {
	_, repo := oneClone(t)
	seedLedger(t, repo)
	forbidden := vGoal("forbidden-legacy", StateDone)
	result, err := Publish(endpointFor(repo), PublishRequest{
		Opid: "op-forbidden-legacy", Machine: "mac-a", Lineage: "lin-1",
		Intent: testIntentFor("done"), Message: "try legacy write",
		Mutate: func(string) ([]Change, error) {
			return []Change{{Path: legacyDonePrefix + forbidden.Id + ".md", Content: RenderFile(forbidden)}}, nil
		},
	})
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "read-only") {
		t.Fatalf("a new legacy conclusion must be rejected by the transaction guard: %+v %v", result, err)
	}
}

func TestReconciliationCaptureIncludesBothConclusionLocations(t *testing.T) {
	repo := t.TempDir()
	legacy := vGoal("legacy", StateDone)
	recorded := vGoal("recorded", StateDone)
	for p, data := range map[string][]byte{
		legacyDonePrefix + legacy.Id + ".md":     RenderFile(legacy),
		recordsGoalsPrefix + recorded.Id + ".md": RenderFile(recorded),
	} {
		absolute := filepath.Join(repo, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := CaptureSnapshot(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 || snapshot.Files[legacyDonePrefix+legacy.Id+".md"] == nil ||
		snapshot.Files[recordsGoalsPrefix+recorded.Id+".md"] == nil {
		t.Fatalf("capture must guard both concluded-goal locations: %v", sortedKeys(snapshot.Files))
	}
}

func TestReconciliationRefusesASymlinkedRecordsGoalDirectory(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(recordsRoot)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(repo, filepath.FromSlash(recordsGoalsRoot))); err != nil {
		t.Fatal(err)
	}
	if err := ensureRealGoalDirs(repo); err == nil || !strings.Contains(err.Error(), recordsGoalsRoot) {
		t.Fatalf("the reconciliation guard must refuse a redirected records-owned archive: %v", err)
	}
}
