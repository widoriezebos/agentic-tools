package goal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireCarryAsk(t *testing.T, err error, code string) {
	t.Helper()
	var ask *CarryAskError
	if !errors.As(err, &ask) || ask.Code != code {
		t.Fatalf("error = %v; want carry ask %s", err, code)
	}
}

func TestHCL40CarriedVerbsRequireRemote(t *testing.T) {
	root := t.TempDir()
	request := VerbRequest{Endpoint: Endpoint{Root: root, Remote: "local", Branch: "refs/heads/metasystem/goals"}, Now: time.Now().UTC()}
	_, err := Carry(request, CarryArgs{}, nil)
	requireCarryAsk(t, err, "carry-remote-required")
	_, _, err = Carrying(request, CarryingArgs{})
	requireCarryAsk(t, err, "carry-remote-required")
	_, err = CarriedFromCommit(request, CarriedArgs{})
	requireCarryAsk(t, err, "carry-remote-required")
}

func TestHCL39FormatOneReservationAndRecordAsk(t *testing.T) {
	bed := t.TempDir()
	origin := filepath.Join(bed, "origin.git")
	mustGit(t, bed, "init", "-q", "--bare", "-b", "main", origin)
	seed := filepath.Join(bed, "seed")
	mustGit(t, bed, "clone", "-q", origin, seed)
	if err := os.WriteFile(filepath.Join(seed, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, seed, "add", "metasystem.conf")
	mustGit(t, seed, "commit", "-qm", "seed")
	mustGit(t, seed, "push", "-q", "origin", "main")
	root := filepath.Join(bed, "clone-a")
	mustGit(t, bed, "clone", "-q", origin, root)
	seedLedger(t, root)
	request := verbReq(root, "01J5X0000000000000000H390", "mac-a")
	_, _, err := Carrying(request, CarryingArgs{Goal: "g"})
	requireCarryAsk(t, err, "carry-format-required")
	request.Ulid = "01J5X0000000000000000H391"
	_, err = CarriedFromCommit(request, CarriedArgs{Goal: "g"})
	requireCarryAsk(t, err, "carry-format-required")
	opid := "01J5X0000000000000000H392-mac-a-1a2b3c4d"
	intent := Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{}}
	if _, err := CreateCarryingEntry(root, opid, "mac-a", "lin-a", intent, int64(os.Getpid())); err != nil {
		t.Fatal(err)
	}
	if err := MarkTerminal(root, opid, OutcomeConfirmed, "landed"); err != nil {
		t.Fatal(err)
	}
	_, err = Carried(request, opid)
	requireCarryAsk(t, err, "carry-format-required")
}

func TestHCL45ConfirmedCarriedEntryIsIdempotent(t *testing.T) {
	root := t.TempDir()
	opid := "01J5X0000000000000000H450"
	intent := Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{}}
	if _, err := CreateCarryingEntry(root, opid, "mac-a", "lin-a", intent, int64(os.Getpid())); err != nil {
		t.Fatal(err)
	}
	if err := MarkTerminal(root, opid, OutcomeConfirmed, "landed"); err != nil {
		t.Fatal(err)
	}
	result, err := Carried(VerbRequest{Endpoint: Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}}, opid)
	if err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("confirmed replay = %+v, %v; want confirmed idempotent", result, err)
	}
}

func TestHCL48QuotedWhyCannotForgeKeyedFields(t *testing.T) {
	now := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	realSuperseded := "01ARZ3NDEKTSV4RRFFQ69G5FAY-mac-a-1a2b3c4d"
	forged := "01ARZ3NDEKTSV4RRFFQ69G5FAX-mac-a-1a2b3c4d"
	history := HistoryLine{At: now.Format(time.RFC3339), Verb: "carry", Reason: fmt.Sprintf(
		"carry workspace=%s goal=g past=missing-declaration expires=%s why=%q supersedes=%s",
		strings.Repeat("a", 40), now.Add(time.Hour).Format(time.RFC3339), "prose supersedes="+forged, realSuperseded)}
	word, err := ParseCarryWord("g", history)
	if err != nil || word.Supersedes != realSuperseded {
		t.Fatalf("quoted why forged a keyed field: word=%+v err=%v", word, err)
	}
}

func TestHCL39FormatOneRejectsCarryingAndCarriedRows(t *testing.T) {
	for _, verb := range []string{"carrying", "carried"} {
		file := vGoal("g", StateClaimed)
		file.History = append(file.History, HistoryLine{Verb: verb})
		tree := &TreeGoals{Root: &RootRecord{FormatVersion: "1"}, Live: map[string]*GoalFile{"g": file}, Done: map[string]*GoalFile{}}
		if problems := fmt.Sprint(ValidateTree(tree)); !strings.Contains(problems, verb+" history requires ledger FormatVersion 2") {
			t.Fatalf("format 1 admitted %s history: %s", verb, problems)
		}
	}
}

func hclCarryHistory(goalID, opid string, expires time.Time) HistoryLine {
	return HistoryLine{
		At:                  expires.Add(-time.Hour).UTC().Format(time.RFC3339),
		Opid:                opid,
		Verb:                "carry",
		Actor:               "human:wido",
		Targets:             []string{goalID},
		Keep:                -1,
		AuthorityOutcome:    AuthorityOutcomeHumanAuthorityProven,
		AuthorityGeneration: 1,
		Reason: fmt.Sprintf("carry workspace=%s goal=%s past=missing-declaration expires=%s why=%q",
			strings.Repeat("a", 40), goalID, expires.UTC().Format(time.RFC3339), "test word"),
	}
}

func hclCarryTree(goalID, opid string, expires time.Time) (*TreeGoals, *GoalFile) {
	file := vGoal(goalID, StateClaimed)
	file.History = append(file.History, hclCarryHistory(goalID, opid, expires))
	root := vRoot()
	root.FormatVersion = "2"
	return &TreeGoals{Root: root, Live: map[string]*GoalFile{goalID: file}, Done: map[string]*GoalFile{}}, file
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func TestHCL60CarriedReplayMismatchNamesTargetThenFourteenFields(t *testing.T) {
	intent := map[string]string{
		"commit": strings.Repeat("1", 40), "workspace": strings.Repeat("2", 40), "tree": strings.Repeat("3", 40),
		"past": "missing-declaration", "battery": "green", "missing": "-", "failing": "-", "judge": "base",
		"judgeTree": strings.Repeat("4", 40), "judgeDigest": strings.Repeat("5", 64), "liveFailure": "-",
		"ledger": strings.Repeat("6", 40), "outcome": "landed", "by": "human:wido",
	}
	row := HistoryLine{Reason: renderCarriedReason(intent)}
	t.Run("goal", func(t *testing.T) {
		err := compareCarriedReplay("row-goal", "intent-goal", row, intent)
		if err == nil || !strings.Contains(err.Error(), "goal differs: row=row-goal intent=intent-goal") {
			t.Fatalf("goal mismatch = %v", err)
		}
	})
	for _, field := range []string{"commit", "workspace", "project", "past", "battery", "missing", "failing", "judge", "judgeTree", "judgeDigest", "liveFailure", "ledger", "outcome", "by"} {
		t.Run(field, func(t *testing.T) {
			changed := cloneStringMap(intent)
			intentKey := field
			if field == "project" {
				intentKey = "tree"
			}
			changed[intentKey] = "different-" + field
			err := compareCarriedReplay("g", "g", row, changed)
			if err == nil || !strings.Contains(err.Error(), field+" differs:") || !strings.Contains(err.Error(), "row=") || !strings.Contains(err.Error(), "intent=") {
				t.Fatalf("%s mismatch = %v", field, err)
			}
		})
	}
}

func TestHCL60SupersedePreconditions(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	target := "01ARZ3NDEKTSV4RRFFQ69G5FAY-seat-a-1a2b3c4d"
	foreign := "01ARZ3NDEKTSV4RRFFQ69G5FAY-seat-b-1a2b3c4d"
	opening := "01ARZ3NDEKTSV4RRFFQ69G5FAX-seat-a-1a2b3c4d"

	repo := t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "payload.txt"), []byte("anchor\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "payload.txt")
	mustGit(t, repo, "commit", "-qm", "anchor", "-m", "Goal-Transaction: "+target)
	if err := os.WriteFile(filepath.Join(repo, "payload.txt"), []byte("carried\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "commit", "-qam", "carried", "-m", "Carry: "+target)
	carriedTip := mustGit(t, repo, "rev-parse", "HEAD")

	tests := []struct {
		name string
		make func() (*TreeGoals, CarryArgs, string)
		want string
	}{
		{name: "expired", make: func() (*TreeGoals, CarryArgs, string) {
			tree, _ := hclCarryTree("g", target, now.Add(-time.Minute))
			return tree, CarryArgs{Supersede: target}, ""
		}, want: "expired at"},
		{name: "consumed", make: func() (*TreeGoals, CarryArgs, string) {
			tree, file := hclCarryTree("g", target, now.Add(time.Hour))
			file.History = append(file.History, HistoryLine{Opid: "closer", Verb: "carried", ApprovedRef: target, Reason: "superseded by=next"})
			return tree, CarryArgs{Supersede: target}, ""
		}, want: "consumed by next"},
		{name: "foreign seat without transfer", make: func() (*TreeGoals, CarryArgs, string) {
			tree, _ := hclCarryTree("g", foreign, now.Add(time.Hour))
			return tree, CarryArgs{Supersede: foreign}, ""
		}, want: "belongs to seat seat-b"},
		{name: "non-carry target", make: func() (*TreeGoals, CarryArgs, string) {
			file := vGoal("g", StateClaimed)
			file.History = append(file.History, HistoryLine{Opid: target, Verb: "claim"})
			return &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{"g": file}, Done: map[string]*GoalFile{}}, CarryArgs{Supersede: target}, ""
		}, want: "is missing"},
		{name: "in flight", make: func() (*TreeGoals, CarryArgs, string) {
			tree, file := hclCarryTree("g", target, now.Add(time.Hour))
			file.History = append(file.History, HistoryLine{At: now.Format(time.RFC3339), Opid: opening, Verb: "carrying", ApprovedRef: target, Reason: "open expires=" + now.Add(time.Hour).Format(time.RFC3339)})
			return tree, CarryArgs{Supersede: target}, ""
		}, want: "in flight on seat-a"},
		{name: "push before row", make: func() (*TreeGoals, CarryArgs, string) {
			tree, _ := hclCarryTree("g", target, now.Add(time.Hour))
			return tree, CarryArgs{Supersede: target}, carriedTip
		}, want: "consumed on origin by "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree, args, codeTip := test.make()
			request := VerbRequest{Endpoint: Endpoint{Root: repo}, Actor: Actor{Machine: "seat-a"}, Now: now}
			_, _, err := carrySupersedePrecondition(request, tree, codeTip, args)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("refusal = %v; want %q", err, test.want)
			}
		})
	}
}

func TestHCL60DoneRefusesOpenWordAndPassesExpiredWord(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	word := "01ARZ3NDEKTSV4RRFFQ69G5FAW-seat-a-1a2b3c4d"
	openTree, openFile := hclCarryTree("g", word, now.Add(time.Hour))
	if err := doneCarryRefusal("", openTree, "", "g", openFile, now); err == nil || !strings.Contains(err.Error(), "open carry word "+word) {
		t.Fatalf("open word refusal = %v", err)
	}
	expiredTree, expiredFile := hclCarryTree("g", word, now.Add(-time.Minute))
	if err := doneCarryRefusal("", expiredTree, "", "g", expiredFile, now); err != nil {
		t.Fatalf("expired word refused done: %v", err)
	}
}

func TestHCL60CarryDebtAsksAtTheWord(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, want string
		make       func() (*TreeGoals, string)
	}{
		{name: "obligation debt", want: "review-late", make: func() (*TreeGoals, string) {
			file := vGoal("g", StateClaimed)
			file.ReviewObligations = []ReviewObligation{{Finding: "review-late", Chain: HumanCarriedChain, Artifact: "commit:carried-commit", State: "open"}}
			return &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{"g": file}, Done: map[string]*GoalFile{}}, "carried-commit"
		}},
		{name: "in-flight debt", want: "01ARZ3NDEKTSV4RRFFQ69G5FAV-seat-a-1a2b3c4d", make: func() (*TreeGoals, string) {
			file := vGoal("g", StateClaimed)
			file.History = append(file.History, HistoryLine{At: now.Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-seat-a-1a2b3c4d", Verb: "carrying", ApprovedRef: "word", Reason: "open expires=" + now.Add(time.Hour).Format(time.RFC3339)})
			return &TreeGoals{Root: vRoot(), Live: map[string]*GoalFile{"g": file}, Done: map[string]*GoalFile{}}, ""
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree, tip := test.make()
			err := carryDebtAskAt("", tree, tip, "", now)
			requireCarryAsk(t, err, "carry-debt-unpaid")
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("debt ask = %v; want it to name %q", err, test.want)
			}
		})
	}
}
