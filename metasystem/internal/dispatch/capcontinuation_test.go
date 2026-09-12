package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func capContinuationWorktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "kept.txt")
	run("commit", "-q", "-m", "base")
	// The capped round changed a tracked file and left a new one.
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("changed by the capped round\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new-file.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func cappedParentRecord(t *testing.T, fields map[string]any) string {
	t.Helper()
	record := map[string]any{
		"jobId": "chain-a", "role": "implementer", "round": 1, "status": "timeout", "error": "budget-cap",
		"capMin": 120, "groupDeathProvenAt": "2026-09-12T10:00:00Z",
	}
	for key, value := range fields {
		record[key] = value
	}
	path := filepath.Join(t.TempDir(), "chain-a.json")
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCapContinuationTextTellsTheFactAndWhatTheWorktreeHolds(t *testing.T) {
	worktree := capContinuationWorktree(t)
	text, err := CapContinuationText(cappedParentRecord(t, nil), worktree)
	if err != nil {
		t.Fatal(err)
	}
	head, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Round 1 of this chain was cut off at its 120-minute cap at 2026-09-12T10:00:00Z and wrote no return.",
		"2 path(s) changed against the worktree head " + head,
		"- kept.txt\n", "- new-file.go\n",
		"your diffBoundary must list every changed path of the chain, the predecessor's included.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("paragraph lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "run the") || strings.Contains(text, "do not start over") {
		t.Fatalf("paragraph carries instruction beyond the fact and the boundary rule:\n%s", text)
	}
}

func TestCapContinuationTextCapsTheListAndNamesTheTotal(t *testing.T) {
	worktree := capContinuationWorktree(t)
	for index := 0; index < capContinuationListLimit+5; index++ {
		if err := os.WriteFile(filepath.Join(worktree, "many-"+strings.Repeat("0", 3-len(itoa(index)))+itoa(index)+".txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text, err := CapContinuationText(cappedParentRecord(t, nil), worktree)
	if err != nil {
		t.Fatal(err)
	}
	total := capContinuationListLimit + 5 + 2
	if !strings.Contains(text, itoa(total)+" path(s) changed") || !strings.Contains(text, "- and 7 more (the total is "+itoa(total)+")") {
		t.Fatalf("the list was not capped with the total named:\n%s", text[len(text)-400:])
	}
}

func TestCapContinuationTextRefusesWhatIsNotACappedImplementerRound(t *testing.T) {
	worktree := capContinuationWorktree(t)
	for _, test := range []struct {
		name   string
		fields map[string]any
		want   string
	}{
		{"completed", map[string]any{"status": "completed", "error": nil}, "needs a parent in timeout with budget-cap"},
		{"running", map[string]any{"status": "running", "error": nil}, "needs a parent in timeout with budget-cap"},
		{"cancelled", map[string]any{"status": "cancelled", "error": nil}, "needs a parent in timeout with budget-cap"},
		{"process-lost", map[string]any{"status": "failed", "error": "process-lost"}, "needs a parent in timeout with budget-cap"},
		{"protocol error", map[string]any{"status": "failed", "error": "protocol_error"}, "needs a parent in timeout with budget-cap"},
		{"critic", map[string]any{"role": "design-critic"}, "implementer chain's"},
		{"verifier", map[string]any{"role": "verifier"}, "implementer chain's"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CapContinuationText(cappedParentRecord(t, test.fields), worktree); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("refusal = %v, want %q", err, test.want)
			}
		})
	}
	if _, err := CapContinuationText(cappedParentRecord(t, nil), filepath.Join(t.TempDir(), "gone")); err == nil || !strings.Contains(err.Error(), "use a fresh dispatch") {
		t.Fatalf("a missing worktree did not refuse naming the fresh dispatch: %v", err)
	}
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func TestBuildFollowRecordCarriesTheContinuation(t *testing.T) {
	dir := t.TempDir()
	parent := writeJSONFile(t, dir, "chain.json", map[string]any{
		"jobId": "chain", "role": "implementer", "mission": nil, "runtime": "fake", "reviews": nil, "round": 1, "status": "timeout", "error": "budget-cap",
		"workspaceRoot": dir, "baseSha": "0000000000000000000000000000000000000000", "branch": "agent/chain", "machineId": nil,
		"permissions":    map[string]any{"requested": map[string]any{}},
		"requestedModel": "fake-model", "destructiveReach": "MECHANICAL", "sessionId": "fake-session-chain",
	})
	capResolution := writeJSONFile(t, dir, "cap.json", map[string]any{
		"capMin": 30, "capDeadline": nil,
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	build := func(continuation string) (map[string]any, error) {
		output := filepath.Join(dir, "chain-r2-"+continuation+".json")
		err := BuildFollowRecord(BuildFollowRecordParams{
			Output: output, Parent: parent, Job: "chain-r2", OperationID: "chain-r2", Round: 2,
			ParentJob: "chain", Fallbacks: "[]", ResumeMode: "fresh-context", Continuation: continuation,
			CapResolution: capResolution, Model: "fake-model",
			DestructiveReach: HazardMechanical, LaunchMode: LaunchModeWorktree,
			OutputStream: filepath.Join(dir, "chain-r2.jsonl"),
		})
		if err != nil {
			return nil, err
		}
		return readJSONFile(t, output), nil
	}
	// The lawful after-cap record is built in
	// TestAfterCapContinuationRecordHoldsItsInvariant with its packet; here
	// the field is absent, not null, on an ordinary follow-up.
	plain, err := build("")
	if _, present := plain["continuation"]; err != nil || present {
		t.Fatalf("an ordinary follow-up recorded a continuation: %v %v", plain["continuation"], err)
	}
	if _, err := build("after-lunch"); err == nil || !strings.Contains(err.Error(), "after-cap") {
		t.Fatalf("an unknown continuation was accepted: %v", err)
	}
}

func TestAfterCapContinuationRecordHoldsItsInvariant(t *testing.T) {
	root := compositionRepoRoot(t)
	dir := t.TempDir()
	brief := filepath.Join(dir, "brief.md")
	fact := filepath.Join(dir, "prior-worktree.md")
	if err := os.WriteFile(brief, []byte("Continue.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fact, []byte("Round 1 of this chain was cut off at its 1-minute cap.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compose := func(name string, continuations ...CompositionContinuation) (string, int64, string) {
		t.Helper()
		params := ComposeRolePacketParams{
			Root: root, Role: "implementer", Brief: brief, JobID: "chain-r2", Runtime: "fake",
			Model: "fake-model", ToolPolicy: "read-write", Round: 2, DestructiveReach: HazardMechanical,
			Output: filepath.Join(dir, name+".md"), CompositionOutput: filepath.Join(dir, name+".json"),
			Continuations: continuations,
		}
		record, err := ComposeRolePacket(params)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := os.ReadFile(params.Output)
		if err != nil {
			t.Fatal(err)
		}
		return params.CompositionOutput, int64(len(packet)), record.PacketDigest
	}
	withSlot, withSlotBytes, withSlotHash := compose("slot", CompositionContinuation{Slot: "prior-worktree", Path: fact})
	withReturn, withReturnBytes, withReturnHash := compose("ret", CompositionContinuation{Slot: "prior-worktree", Path: fact}, CompositionContinuation{Slot: "prior-return", Path: fact})
	capResolution := writeJSONFile(t, dir, "cap.json", map[string]any{
		"capMin": 30, "capDeadline": nil,
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	parentFields := func(over map[string]any) string {
		fields := map[string]any{
			"jobId": "chain", "role": "implementer", "mission": nil, "runtime": "fake", "reviews": nil, "round": 1, "status": "timeout", "error": "budget-cap",
			"workspaceRoot": dir, "baseSha": "0000000000000000000000000000000000000000", "branch": "agent/chain", "machineId": nil,
			"permissions":    map[string]any{"requested": map[string]any{}},
			"requestedModel": "fake-model", "destructiveReach": "MECHANICAL", "sessionId": "fake-session-chain",
		}
		for key, value := range over {
			fields[key] = value
		}
		return writeJSONFile(t, dir, "parent-"+strconv.Itoa(len(over))+".json", fields)
	}
	build := func(parent string, resumeMode string, launch LaunchMode, composition string, bytes int64, hash string) error {
		return BuildFollowRecord(BuildFollowRecordParams{
			Output: filepath.Join(dir, "out.json"), Parent: parent, Job: "chain-r2", OperationID: "chain-r2", Round: 2,
			ParentJob: "chain", Fallbacks: "[]", ResumeMode: resumeMode, Continuation: ContinuationAfterCap,
			CapResolution: capResolution, Model: "fake-model", DestructiveReach: HazardMechanical, LaunchMode: launch,
			OutputStream: filepath.Join(dir, "chain-r2.jsonl"), Composition: composition, InputBytes: bytes, InputHash: hash,
		})
	}
	if err := build(parentFields(nil), "fresh-context", LaunchModeWorktree, withSlot, withSlotBytes, withSlotHash); err != nil {
		t.Fatalf("a lawful continuation was refused: %v", err)
	}
	if record := readJSONFile(t, filepath.Join(dir, "out.json")); record["continuation"] != "after-cap" || record["resumeMode"] != "fresh-context" || record["parentJob"] != "chain" {
		t.Fatalf("the continuation was not recorded: %v", record)
	}
	for _, test := range []struct {
		name   string
		parent map[string]any
		mode   string
		launch LaunchMode
		comp   string
		bytes  int64
		hash   string
		want   string
	}{
		{"resumed", nil, "resumed", LaunchModeWorktree, withSlot, withSlotBytes, withSlotHash, "fresh context"},
		{"shared checkout", nil, "fresh-context", LaunchModeSharedCheckout, withSlot, withSlotBytes, withSlotHash, "job worktree"},
		{"completed parent", map[string]any{"status": "completed", "error": nil}, "fresh-context", LaunchModeWorktree, withSlot, withSlotBytes, withSlotHash, "timeout with budget-cap"},
		{"critic parent", map[string]any{"role": "design-critic"}, "fresh-context", LaunchModeWorktree, withSlot, withSlotBytes, withSlotHash, "implementer round"},
		{"prior return in the packet", nil, "fresh-context", LaunchModeWorktree, withReturn, withReturnBytes, withReturnHash, "no prior return"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := build(parentFields(test.parent), test.mode, test.launch, test.comp, test.bytes, test.hash)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("refusal = %v, want %q", err, test.want)
			}
		})
	}
	if !immutableFields["continuation"] {
		t.Fatal("continuation is not an immutable record field")
	}
}

func TestCapContinuationTextQuotesAndBoundsThePaths(t *testing.T) {
	worktree := capContinuationWorktree(t)
	odd := "odd name.txt"
	if err := os.WriteFile(filepath.Join(worktree, odd), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := CapContinuationText(cappedParentRecord(t, nil), worktree)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "- \"odd name.txt\"\n") || strings.Contains(text, "git status") {
		t.Fatalf("a path with whitespace was not quoted, or the paragraph carries an extra instruction:\n%s", text)
	}
	long := strings.Repeat("a", 200)
	for index := 0; index < 60; index++ {
		if err := os.WriteFile(filepath.Join(worktree, long+"-"+strconv.Itoa(index)+".txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text, err = CapContinuationText(cappedParentRecord(t, nil), worktree)
	if err != nil {
		t.Fatal(err)
	}
	if len(text) > capContinuationListBytes+1024 || !strings.Contains(text, " more (the total is 63)") {
		t.Fatalf("the listing is not bounded in bytes with the total named: %d bytes\n%s", len(text), text[len(text)-300:])
	}
	if quotePath("plain/path.go") != "plain/path.go" || quotePath("new\nline") != "\"new\\nline\"" {
		t.Fatalf("quotePath: %q %q", quotePath("plain/path.go"), quotePath("new\nline"))
	}
}
