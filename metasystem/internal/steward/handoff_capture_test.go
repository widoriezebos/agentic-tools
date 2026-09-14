package steward

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

var handoffCaptureNow = time.Date(2026, 9, 14, 18, 30, 0, 0, time.UTC)

func capturedGoal(state string) *goal.GoalFile {
	file := approvedStewardGoal("fix-it", "Preserve the coordinator context", "Continue from plans/handoff-capture.md.", "2026-09-14T15:00:00Z")
	file.State = goal.StateClaimed
	file.Revision++
	claimRevision := file.Revision
	file.Claimed = &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: "2026-09-14T16:00:00Z", Revision: claimRevision, AccountingRevision: claimRevision}
	file.StopCapability = &goal.StopCapability{Generation: claimRevision, Revision: claimRevision, Machine: "bed-m1", ClaimEpoch: 1}
	file.History = append(file.History, goal.HistoryLine{
		At: "2026-09-14T16:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000001",
		Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{file.Id}, Keep: -1,
	})
	if state == "landing" {
		file.Revision++
		file.History = append(file.History, goal.HistoryLine{
			At: "2026-09-14T17:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000002",
			Verb: "land-ready", Actor: "bed-m1+coordinator", Targets: []string{file.Id}, Keep: -1,
		})
		file.Landing = &goal.LandingRecord{At: "2026-09-14T17:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000002"}
	}
	return file
}

func installHandoffFixtureFiles(t *testing.T, root string) {
	t.Helper()
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metasystem.conf", "metasystem.runtimes=fake\nrole.steward-continuation.runtime=fake\nrole.steward-continuation.model.fake=fixture\n")
	write("scripts/agents/roles/steward-continuation.md", "# Role: steward-continuation\ncontract\n")
	write("scripts/agents/roles/steward-continuation.requirements.json", `{"required":[]}`)
	write("scripts/agents/schemas/steward-continuation.schema.json", `{"type":"object"}`)
	write("scripts/agents/permissions/workspace.json", `{"write":["workspace"]}`)
	write("plans/handoff-capture.md", "# Handoff capture\n\nContinue the exact goal.\n")
	write("memory/receipts.log", "1789408800|2026-09-14T17:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|goal=fix-it|note=prior landing\n")
	top, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(RepoIdentityPath(top)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MintIdentity(RepoIdentityPath(top), InstallIdentity{RepoIdentity: top, Generation: 1, InstallPath: "/bin/true", MintedAt: "2026-09-14T17:00:00Z"}); err != nil {
		t.Fatal(err)
	}
}

func handoffCaptureRepo(t *testing.T, state string) string {
	t.Helper()
	files := map[string]*goal.GoalFile{}
	if state != "none" {
		files["fix-it"] = capturedGoal(state)
	}
	root := convertedBed(t, "bed-m1", files)
	installHandoffFixtureFiles(t, root)
	return root
}

func handoffMainCaller() HandoffCaller {
	return HandoffCaller{Class: handoffClassMain, MainId: "main-1", HolderMainId: "main-1", Runtime: "fake",
		Session: "session / original", Machine: "bed-m1", Ref: identity.Ref{Pid: 4242, StartedAtSec: 100}, Tag: "main-tag"}
}

func useHandoffNonces(t *testing.T, nonces ...string) {
	t.Helper()
	previous := mintHandoffNonce
	var index atomic.Int64
	mintHandoffNonce = func() (string, error) {
		n := index.Add(1) - 1
		if n >= int64(len(nonces)) {
			return "", fmt.Errorf("fixture nonce sequence exhausted")
		}
		return nonces[n], nil
	}
	t.Cleanup(func() { mintHandoffNonce = previous })
}

func readHandoffStateFile(t *testing.T, path string) HandoffState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state HandoffState
	if err := decodeStrictHandoffJSON(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func writeHandoffJob(t *testing.T, root, id, status string, fields map[string]any) string {
	t.Helper()
	record := map[string]any{"jobId": id, "goalId": "fix-it", "role": "implementer", "status": status, "phase": "work"}
	for key, value := range fields {
		record[key] = value
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", id+".json")
	writeTestFile(t, path, append(data, '\n'))
	return path
}

func TestHandoffWritesAVerifiedStateFile(t *testing.T) {
	root := handoffCaptureRepo(t, "landing")
	useHandoffNonces(t, "6000000000000001")
	jobPath := writeHandoffJob(t, root, "running-job", "running", nil)
	scratchPath := filepath.Join(root, "artifacts", "reports", "proof.txt")
	writeTestFile(t, scratchPath, []byte("verified proof\n"))
	var receiptRows strings.Builder
	for index := 1; index <= 7; index++ {
		fmt.Fprintf(&receiptRows, "%d|2026-09-14T17:0%d:00Z|RECEIPT|type=implement|outcome=shipped|goal=fix-it|note=landing-%d\n", 1789408800+index, index, index)
	}
	writeTestFile(t, filepath.Join(root, "memory", "receipts.log"), []byte(receiptRows.String()))
	if err := QueueNotification(root, PendingNotification{Nonce: "owed-1", Message: "tell the next seat"}); err != nil {
		t.Fatal(err)
	}
	if err := BumpEnrollmentFence(root); err != nil {
		t.Fatal(err)
	}

	result, err := Handoff(root, handoffMainCaller(), []ScratchArg{
		{Purpose: "proof", Path: "artifacts/reports/proof.txt", Required: true},
		{Purpose: "optional note", Path: "artifacts/reports/absent.txt"},
	}, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Nonce != "6000000000000001" || result.StateDigest == "" || result.Intent.Handoff == nil || result.Intent.FenceAtMint != 1 {
		t.Fatalf("incomplete handoff result: %+v", result)
	}
	if digest, err := VerifyHandoffState(root, result.Nonce); err != nil || digest != result.StateDigest {
		t.Fatalf("published state did not verify: digest=%s err=%v", digest, err)
	}
	info, err := os.Lstat(result.StatePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() >= 32*1024 {
		t.Fatalf("state file is not a bounded regular file: %+v %v", info, err)
	}
	state := readHandoffStateFile(t, result.StatePath)
	if state.Seat.Session != "session / original" || state.Seat.NormalizedSession != goal.NormalizeSession("session / original") ||
		state.Seat.Identity != handoffMainCaller().Ref || state.HeldGoal.ID != "fix-it" || state.HeldGoal.State != "landing" || state.HeldGoal.AcceptedRevision != 4 ||
		state.HeldGoal.Claimant.Revision != 3 || state.NextStep.Text != capturedGoal("landing").NextStep || state.Disposable != HandoffDisposable {
		t.Fatalf("state omitted exact seat or goal facts: %+v", state)
	}
	if len(state.OpenJobs) != 1 || state.OpenJobs[0].ID != "running-job" || len(state.NextStep.References) != 1 || len(state.Scratch) != 2 ||
		state.Scratch[1].Status != "missing" || state.Scratch[1].Required {
		t.Fatalf("state did not preserve references and optional absence: %+v", state)
	}
	if len(state.LastLandings.History) != 1 || len(state.LastLandings.Receipts) != maxHandoffLandings ||
		state.LastLandings.Receipts[0].Receipt != "memory/receipts.log:7" || state.LastLandings.Receipts[4].Receipt != "memory/receipts.log:3" || len(state.MessagesOwed) != 1 {
		t.Fatalf("state omitted landing, receipt, or pending-message facts: %+v", state)
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range []HandoffReference{state.OpenJobs[0].Record, state.NextStep.References[0], state.Scratch[0]} {
		if err := validateHandoffReference(canonicalRoot, HandoffDir(root, result.Nonce), reference); err != nil {
			t.Fatalf("captured reference did not verify: %v", err)
		}
	}
	if err := os.WriteFile(jobPath, []byte(`{"jobId":"running-job","status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyHandoffState(root, result.Nonce); err != nil {
		t.Fatalf("lawful later live-record progress invalidated the immutable capture: %v", err)
	}
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 || live[0].Nonce != result.Nonce || live[0].Handoff.StateDigest != result.StateDigest {
		t.Fatalf("intent was not bound and made live: %+v %v", live, err)
	}
	receipts, err := os.ReadFile(filepath.Join(root, "memory", "receipts.log"))
	if err != nil || !strings.Contains(string(receipts), "steward revival: intent "+result.Nonce) {
		t.Fatalf("intent receipt is absent: %v %q", err, receipts)
	}
}

func TestHandoffManifestPreservesOverflow(t *testing.T) {
	landed := &goal.GoalFile{}
	for index := 0; index < 7; index++ {
		landed.History = append(landed.History, goal.HistoryLine{Verb: "release", Opid: fmt.Sprintf("landing-%d", index)})
	}
	last := newestHandoffLandings(landed)
	if len(last.History) != maxHandoffLandings || last.History[0].OpID != "landing-6" || last.History[4].OpID != "landing-2" {
		t.Fatalf("landing history was not capped to the newest five: %+v", last.History)
	}
	bounded := HandoffState{OpenJobs: make([]HandoffOpenJob, maxHandoffOpenJobs+3), Scratch: make([]HandoffReference, maxHandoffScratch+2), MessagesOwed: make([]HandoffMessage, maxHandoffMessages+4)}
	manifestBound := HandoffManifest{}
	splitHandoffStateLists(&bounded, &manifestBound)
	if len(bounded.OpenJobs) != maxHandoffOpenJobs || len(manifestBound.OpenJobs) != 3 ||
		len(bounded.Scratch) != maxHandoffScratch || len(manifestBound.Scratch) != 2 ||
		len(bounded.MessagesOwed) != maxHandoffMessages || len(manifestBound.MessagesOwed) != 4 {
		t.Fatalf("inline list bounds silently lost overflow: state=%+v manifest=%+v", bounded, manifestBound)
	}
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6000000000000002")
	for index := 0; index < maxHandoffOpenJobs+3; index++ {
		writeHandoffJob(t, root, fmt.Sprintf("running-%03d", index), "running", nil)
	}
	for index := 0; index < maxHandoffMessages+3; index++ {
		if err := QueueNotification(root, PendingNotification{Nonce: fmt.Sprintf("owed-%03d", index), Message: fmt.Sprintf("message %03d", index)}); err != nil {
			t.Fatal(err)
		}
	}
	var scratch []ScratchArg
	for index := 0; index < maxHandoffScratch+2; index++ {
		relative := fmt.Sprintf("artifacts/reports/scratch-%03d.txt", index)
		writeTestFile(t, filepath.Join(root, relative), []byte(fmt.Sprintf("scratch %d\n", index)))
		scratch = append(scratch, ScratchArg{Purpose: fmt.Sprintf("scratch %d", index), Path: relative, Required: true})
	}
	result, err := Handoff(root, handoffMainCaller(), scratch, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	state := readHandoffStateFile(t, result.StatePath)
	if state.Manifest == nil || len(state.OpenJobs) > maxHandoffOpenJobs || len(state.MessagesOwed) > maxHandoffMessages || len(state.Scratch) > maxHandoffScratch {
		t.Fatalf("inline lists did not retain their exact bounds: jobs=%d messages=%d scratch=%d manifest=%+v", len(state.OpenJobs), len(state.MessagesOwed), len(state.Scratch), state.Manifest)
	}
	manifestData, mismatch := dispatch.ReadVerifiedReference(root, *state.Manifest)
	if mismatch != nil {
		t.Fatal(mismatch.Line())
	}
	var manifest HandoffManifest
	if err := decodeStrictHandoffJSON(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(state.OpenJobs)+len(manifest.OpenJobs) != maxHandoffOpenJobs+3 ||
		len(state.MessagesOwed)+len(manifest.MessagesOwed) != maxHandoffMessages+3 ||
		len(state.Scratch)+len(manifest.Scratch) != maxHandoffScratch+2 ||
		len(manifest.OpenJobs) == 0 || len(manifest.MessagesOwed) == 0 || len(manifest.Scratch) == 0 {
		t.Fatalf("manifest lost overflow: state messages=%d scratch=%d manifest=%+v", len(state.MessagesOwed), len(state.Scratch), manifest)
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range append(append([]HandoffOpenJob(nil), state.OpenJobs...), manifest.OpenJobs...) {
		if err := validateHandoffReference(canonicalRoot, HandoffDir(root, result.Nonce), job.Record); err != nil {
			t.Fatalf("overflow job %s lost its verified record: %v", job.ID, err)
		}
	}
	if _, err := VerifyHandoffState(root, result.Nonce); err != nil {
		t.Fatalf("overflow state did not verify: %v", err)
	}
}

func handoffArtifactCount(root string) (directories, intents int) {
	if entries, err := os.ReadDir(filepath.Join(root, "artifacts", "agents", "context", "handoffs")); err == nil {
		directories = len(entries)
	}
	if entries, err := os.ReadDir(intentsDir(root)); err == nil {
		intents = len(entries)
	}
	return directories, intents
}

func TestHandoffRefusals(t *testing.T) {
	tests := []struct {
		name string
		code string
		set  func(*testing.T, string, *HandoffCaller) []ScratchArg
	}{
		{name: "unobservable runtime precedes holder checks", code: "HANDOFF_UNOBSERVABLE", set: func(_ *testing.T, _ string, caller *HandoffCaller) []ScratchArg {
			caller.Runtime, caller.MainId = "devin", "advisor"
			return nil
		}},
		{name: "advisor is not holder", code: "HANDOFF_NOT_HOLDER", set: func(_ *testing.T, _ string, caller *HandoffCaller) []ScratchArg {
			caller.MainId = "advisor"
			return nil
		}},
		{name: "waiter in flight", code: "HANDOFF_WAIT_IN_FLIGHT", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			writeTestFile(t, filepath.Join(root, "artifacts", "agents", "waiters", "job-wait.json"), []byte(`{"schemaVersion":2,"mainId":"main-1","state":"pending"}`))
			return nil
		}},
		{name: "pending launch", code: "HANDOFF_LAUNCH_IN_FLIGHT", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			writeHandoffJob(t, root, "pending-job", "pending", nil)
			return nil
		}},
		{name: "required scratch missing", code: "HANDOFF_REFERENCE", set: func(_ *testing.T, _ string, _ *HandoffCaller) []ScratchArg {
			return []ScratchArg{{Purpose: "proof", Path: "artifacts/reports/missing.txt", Required: true}}
		}},
		{name: "scratch escapes root", code: "HANDOFF_REFERENCE", set: func(_ *testing.T, _ string, _ *HandoffCaller) []ScratchArg {
			return []ScratchArg{{Purpose: "proof", Path: "../outside.txt", Required: true}}
		}},
		{name: "scratch symlink escapes root", code: "HANDOFF_REFERENCE", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			outsideDir := t.TempDir()
			outside := filepath.Join(outsideDir, "outside.txt")
			writeTestFile(t, outside, []byte("outside\n"))
			link := filepath.Join(root, "artifacts", "outside-link")
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outsideDir, link); err != nil {
				t.Fatal(err)
			}
			return []ScratchArg{{Purpose: "proof", Path: "artifacts/outside-link/outside.txt", Required: true}}
		}},
		{name: "required scratch is nonregular", code: "HANDOFF_REFERENCE", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			if err := os.MkdirAll(filepath.Join(root, "artifacts", "reports", "directory.txt"), 0o755); err != nil {
				t.Fatal(err)
			}
			return []ScratchArg{{Purpose: "proof", Path: "artifacts/reports/directory.txt", Required: true}}
		}},
		{name: "duplicate scratch argument", code: "HANDOFF_REFERENCE", set: func(_ *testing.T, _ string, _ *HandoffCaller) []ScratchArg {
			arg := ScratchArg{Purpose: "plan", Path: "plans/handoff-capture.md", Required: true}
			return []ScratchArg{arg, arg}
		}},
		{name: "invalid main identity", code: "HANDOFF_NOT_HOLDER", set: func(_ *testing.T, _ string, caller *HandoffCaller) []ScratchArg {
			caller.Ref = identity.Ref{}
			return nil
		}},
		{name: "caller machine differs from accepted claimant", code: "HANDOFF_NOT_HOLDER", set: func(_ *testing.T, _ string, caller *HandoffCaller) []ScratchArg {
			caller.Machine = "another-machine"
			return nil
		}},
		{name: "required scratch changes while captured", code: "HANDOFF_REFERENCE", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			path := filepath.Join(root, "artifacts", "reports", "changing.txt")
			writeTestFile(t, path, []byte("first\n"))
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			path, err = canonicalExistingPath(path)
			if err != nil {
				t.Fatal(err)
			}
			previous := handoffSourceAfterRead
			var changed atomic.Bool
			handoffSourceAfterRead = func(candidate string) {
				if candidate == path && changed.CompareAndSwap(false, true) {
					if err := os.WriteFile(path, []byte("other\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Cleanup(func() { handoffSourceAfterRead = previous })
			return []ScratchArg{{Purpose: "proof", Path: "artifacts/reports/changing.txt", Required: true}}
		}},
		{name: "required scratch keeps its bytes but changes identity while captured", code: "HANDOFF_REFERENCE", set: func(t *testing.T, root string, _ *HandoffCaller) []ScratchArg {
			path := filepath.Join(root, "artifacts", "reports", "touched.txt")
			writeTestFile(t, path, []byte("same\n"))
			path, err := canonicalExistingPath(path)
			if err != nil {
				t.Fatal(err)
			}
			previous := handoffSourceAfterRead
			var touched atomic.Bool
			handoffSourceAfterRead = func(candidate string) {
				if candidate == path && touched.CompareAndSwap(false, true) {
					later := handoffCaptureNow.Add(time.Hour)
					if err := os.Chtimes(path, later, later); err != nil {
						t.Fatal(err)
					}
				}
			}
			t.Cleanup(func() { handoffSourceAfterRead = previous })
			return []ScratchArg{{Purpose: "proof", Path: "artifacts/reports/touched.txt", Required: true}}
		}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := handoffCaptureRepo(t, "claimed")
			caller := handoffMainCaller()
			scratch := test.set(t, root, &caller)
			useHandoffNonces(t, fmt.Sprintf("61%014x", index+1))
			_, err := Handoff(root, caller, scratch, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
			var got *HandoffRefusal
			if !errors.As(err, &got) || got.Code != test.code {
				t.Fatalf("refusal=%v, want %s", err, test.code)
			}
			if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
				t.Fatalf("refusal published directories=%d intents=%d", directories, intents)
			}
		})
	}

	t.Run("state cannot fit", func(t *testing.T) {
		file := capturedGoal("claimed")
		file.NextStep = strings.Repeat("next ", 8000)
		root := convertedBed(t, "bed-m1", map[string]*goal.GoalFile{"fix-it": file})
		installHandoffFixtureFiles(t, root)
		previous := mintHandoffNonce
		mintCalls := 0
		mintHandoffNonce = func() (string, error) {
			mintCalls++
			return "6100000000000008", nil
		}
		t.Cleanup(func() { mintHandoffNonce = previous })
		result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		var got *HandoffRefusal
		if !errors.As(err, &got) || got.Code != "HANDOFF_STATE_TOO_LARGE" {
			t.Fatalf("oversized-state refusal=%v", err)
		}
		if result != (HandoffResult{}) {
			t.Fatalf("oversized refusal minted a result: %+v", result)
		}
		if mintCalls != 0 {
			t.Fatalf("oversized refusal minted %d nonce candidates", mintCalls)
		}
		if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
			t.Fatalf("oversized capture directories=%d intents=%d", directories, intents)
		}
	})

	t.Run("no claimed or landing goal", func(t *testing.T) {
		root := handoffCaptureRepo(t, "none")
		useHandoffNonces(t, "6200000000000001")
		_, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		var got *HandoffRefusal
		if !errors.As(err, &got) || got.Code != "HANDOFF_NO_GOAL" {
			t.Fatalf("no-goal refusal=%v", err)
		}
		if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
			t.Fatalf("no-goal refusal published directories=%d intents=%d", directories, intents)
		}
	})

	t.Run("another session already has the handoff", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6100000000000009")
		first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		caller := handoffMainCaller()
		caller.Session = "different session"
		_, err = Handoff(root, caller, nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		var got *HandoffRefusal
		if !errors.As(err, &got) || got.Code != "HANDOFF_OTHER_PENDING" || got.Detail != "nonce="+first.Nonce {
			t.Fatalf("other-session refusal=%v", err)
		}
		if directories, intents := handoffArtifactCount(root); directories != 1 || intents != 1 {
			t.Fatalf("other-session refusal published directories=%d intents=%d", directories, intents)
		}
	})

	t.Run("bad caller arguments are refused rather than clamped", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		if _, err := Handoff(root, handoffMainCaller(), nil, time.Time{}, filepath.Join(root, "memory", "receipts.log")); err == nil {
			t.Fatal("zero handoff clock was silently replaced")
		}
		badReceipt := filepath.Join(root, "artifacts", "reports", "receipts.log")
		writeTestFile(t, badReceipt, []byte("external receipt sink\n"))
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, badReceipt); err == nil {
			t.Fatal("out-of-owner receipt path was silently accepted")
		}
		if _, err := Handoff(root, handoffMainCaller(), []ScratchArg{{Path: "plans/handoff-capture.md", Required: true}}, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil {
			t.Fatal("empty scratch purpose was silently accepted")
		}
		if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
			t.Fatalf("bad argument published directories=%d intents=%d", directories, intents)
		}
	})

	t.Run("malformed matching receipt refuses", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		writeTestFile(t, filepath.Join(root, "memory", "receipts.log"), []byte("1|2026-09-14T17:00:00Z|RECEIPT|type=implement|goal=fix-it\n"))
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil || !strings.Contains(err.Error(), "last landing receipt") {
			t.Fatalf("malformed matching receipt=%v", err)
		}
		if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
			t.Fatalf("malformed receipt published directories=%d intents=%d", directories, intents)
		}
	})
}

func TestHandoffRetriesNonceCollisionWithoutOverwriting(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	const collision = "6b00000000000001"
	sentinel := filepath.Join(HandoffDir(root, collision), "keep.txt")
	writeTestFile(t, sentinel, []byte("prior evidence\n"))
	useHandoffNonces(t, collision, "6b00000000000002")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil || result.Nonce != "6b00000000000002" {
		t.Fatalf("collision retry=%+v err=%v", result, err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "prior evidence\n" {
		t.Fatalf("nonce collision overwrote evidence: %q %v", data, err)
	}

	root = handoffCaptureRepo(t, "claimed")
	const lifecycleCollision = "6b00000000000003"
	lifecycle := filepath.Join(cancelledDir(root), lifecycleCollision+".json")
	writeTestFile(t, lifecycle, []byte("prior lifecycle evidence\n"))
	useHandoffNonces(t, lifecycleCollision, "6b00000000000004")
	result, err = Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil || result.Nonce != "6b00000000000004" {
		t.Fatalf("lifecycle collision retry=%+v err=%v", result, err)
	}
	if data, err := os.ReadFile(lifecycle); err != nil || string(data) != "prior lifecycle evidence\n" {
		t.Fatalf("nonce collision crossed lifecycle evidence: %q %v", data, err)
	}
}

func TestHandoffCleansDirectoryPublicationFailure(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	const nonce = "6e00000000000001"
	useHandoffNonces(t, nonce)
	previous := syncHandoffDir
	syncHandoffDir = func(path string) error {
		if path == filepath.Dir(HandoffDir(root, nonce)) {
			if _, err := os.Stat(HandoffDir(root, nonce)); err == nil {
				return errors.New("injected handoff parent sync failure")
			}
		}
		return previous(path)
	}
	t.Cleanup(func() { syncHandoffDir = previous })
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err == nil || !strings.Contains(err.Error(), "injected handoff parent sync failure") || result != (HandoffResult{}) {
		t.Fatalf("directory partial=%+v err=%v", result, err)
	}
	if _, err := os.Stat(HandoffDir(root, nonce)); !os.IsNotExist(err) {
		t.Fatalf("failed publication left a nonce directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(intentsDir(root), nonce+".json")); !os.IsNotExist(err) {
		t.Fatalf("directory durability failure minted authority: %v", err)
	}
}

func TestHandoffPublicationIsExclusiveAndReverified(t *testing.T) {
	t.Run("existing member is not overwritten", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		const nonce = "6e00000000000002"
		useHandoffNonces(t, nonce)
		previous := syncHandoffDir
		syncHandoffDir = func(path string) error {
			dir := HandoffDir(root, nonce)
			if path == filepath.Dir(dir) {
				if _, err := os.Stat(dir); err == nil {
					writeTestFile(t, filepath.Join(dir, "references", "plan-000000.copy"), []byte("collision\n"))
				}
			}
			return previous(path)
		}
		t.Cleanup(func() { syncHandoffDir = previous })
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil || !os.IsExist(err) {
			t.Fatalf("existing immutable member was overwritten: %v", err)
		}
	})

	t.Run("published state is verified before minting", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6e00000000000003")
		previous := handoffStatePublished
		handoffStatePublished = func(path string) { writeTestFile(t, path, []byte("{}\n")) }
		t.Cleanup(func() { handoffStatePublished = previous })
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
			t.Fatalf("drifted publication minted authority: %v", err)
		}
		if directories, intents := handoffArtifactCount(root); directories != 0 || intents != 0 {
			t.Fatalf("drifted publication left directories=%d intents=%d", directories, intents)
		}
	})
}

func TestLiveHandoffUsesNoExpiryClock(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6d00000000000001")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	previous := handoffExpiryRule
	calls := 0
	handoffExpiryRule = func(recordedAt, observedAt time.Time) bool {
		calls++
		if !recordedAt.Equal(handoffCaptureNow) || !observedAt.IsZero() {
			t.Fatalf("live lookup sampled an expiry clock: recorded=%s observed=%s", recordedAt, observedAt)
		}
		return false
	}
	t.Cleanup(func() { handoffExpiryRule = previous })
	nonce, live, err := LiveHandoffForSession(root, "session / original")
	if err != nil || !live || nonce != result.Nonce || calls != 1 {
		t.Fatalf("live handoff nonce=%s live=%t expiry-calls=%d err=%v", nonce, live, calls, err)
	}
}

func TestLiveHandoffReadDoesNotWaitOrWrite(t *testing.T) {
	empty := t.TempDir()
	if nonce, live, err := LiveHandoffForSession(empty, "session"); err != nil || live || nonce != "" {
		t.Fatalf("empty read nonce=%q live=%t err=%v", nonce, live, err)
	}
	if _, err := os.Stat(filepath.Join(empty, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("read-only lookup created state: %v", err)
	}

	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6d00000000000002")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	type lookup struct {
		nonce string
		live  bool
		err   error
	}
	done := make(chan lookup, 1)
	go func() {
		nonce, live, err := LiveHandoffForSession(root, handoffMainCaller().Session)
		done <- lookup{nonce: nonce, live: live, err: err}
	}()
	select {
	case got := <-done:
		if got.err != nil || !got.live || got.nonce != result.Nonce {
			t.Fatalf("lookup behind arbitration=%+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("live handoff lookup waited for steward arbitration")
	}
}

func TestHandoffAcceptsOnlyTheActiveContinuation(t *testing.T) {
	for _, test := range []struct {
		name      string
		callerJob string
		activeJob string
		want      bool
	}{
		{name: "no active continuation", callerJob: "delegate-job"},
		{name: "different continuation", callerJob: "other", activeJob: "delegate-job"},
		{name: "exact active continuation", callerJob: "delegate-job", activeJob: "delegate-job", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := handoffCaptureRepo(t, "claimed")
			if test.activeJob != "" {
				active := testIntent("active-" + test.activeJob)
				active.JobId = test.activeJob
				consumedIntentOnDisk(t, root, active)
				writeHandoffJob(t, root, test.activeJob, "running", map[string]any{
					"mainId": "delegate-main", "runtime": "fake", "sessionId": "recorded delegate session",
					"instanceTag": "delegate-tag", "pid": 4343, "pidStartedAt": 101,
				})
			}
			caller := handoffMainCaller()
			caller.Class, caller.JobId, caller.MainId = handoffClassDelegate, test.callerJob, "supplied-main"
			caller.Runtime, caller.Session, caller.Tag, caller.Ref = "devin", "supplied-session", "supplied-tag", identity.Ref{Pid: 9999, StartedAtSec: 999}
			useHandoffNonces(t, "6200000000000002")
			result, err := Handoff(root, caller, nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
			if !test.want {
				var got *HandoffRefusal
				if !errors.As(err, &got) || got.Code != "HANDOFF_NOT_HOLDER" {
					t.Fatalf("delegate refusal=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			state := readHandoffStateFile(t, result.StatePath)
			if state.Seat.Session != "recorded delegate session" || state.Seat.MainID != "delegate-main" || state.Seat.Tag != "delegate-tag" ||
				state.Seat.Identity.Pid != 4343 || state.Seat.JobID != test.activeJob {
				t.Fatalf("delegate state trusted supplied rather than recorded custody: %+v", state.Seat)
			}
		})
	}
}

func readIntentFile(t *testing.T, path string) Intent {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var intent Intent
	if err := json.Unmarshal(data, &intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func cancelledHandoff(t *testing.T, root, nonce string) Intent {
	t.Helper()
	return readIntentFile(t, filepath.Join(cancelledDir(root), nonce+".json"))
}

func TestHandoffIgnoresConcurrentUnrelatedRecords(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6500000000000004")
	waiterRelative := filepath.ToSlash(filepath.Join("artifacts", "agents", "waiters", "other.json"))
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(waiterRelative)), []byte(`{"schemaVersion":2,"mainId":"other-main","state":"pending"}`))
	jobRelative := filepath.ToSlash(filepath.Join("artifacts", "agents", "jobs", "other.json"))
	writeHandoffJob(t, root, "other", "running", map[string]any{"goalId": "other-goal"})
	jobPath, err := canonicalExistingPath(filepath.Join(root, filepath.FromSlash(jobRelative)))
	if err != nil {
		t.Fatal(err)
	}
	previous := handoffCandidateRead
	handoffCandidateRead = func(relative string) {
		if relative == waiterRelative {
			_ = os.Remove(filepath.Join(root, filepath.FromSlash(relative)))
		}
	}
	t.Cleanup(func() { handoffCandidateRead = previous })
	// The other goal's job changes between the two reads of a stable capture. Only the
	// goal pre-filter keeps that record out of the stable read, so this fails without it.
	previousSource := handoffSourceAfterRead
	handoffSourceAfterRead = func(candidate string) {
		if candidate == jobPath {
			_ = os.WriteFile(jobPath, []byte(`{"jobId":"other","goalId":"other-goal","status":"completed"}`), 0o644)
		}
	}
	t.Cleanup(func() { handoffSourceAfterRead = previousSource })
	if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err != nil {
		t.Fatalf("unrelated waiter completion or job progress refused handoff: %v", err)
	}
}

func TestSecondHandoffSupersedesTheFirst(t *testing.T) {
	t.Run("replacement becomes the sole live authorization", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6300000000000001", "6300000000000002")
		first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		second, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow.Add(time.Minute), filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		live, err := LiveIntents(root)
		if err != nil || len(live) != 1 || live[0].Nonce != second.Nonce {
			t.Fatalf("replacement is not the sole live handoff: %+v %v", live, err)
		}
		cancelled := cancelledHandoff(t, root, first.Nonce)
		if cancelled.Outcome != "cancelled: superseded by "+second.Nonce {
			t.Fatalf("supersession outcome=%q", cancelled.Outcome)
		}
		for _, pair := range []struct {
			result HandoffResult
			intent Intent
		}{{first, cancelled}, {second, live[0]}} {
			if _, err := os.Stat(pair.result.StatePath); err != nil {
				t.Fatalf("supersession lost immutable state %s: %v", pair.result.Nonce, err)
			}
			if pair.intent.Handoff == nil || pair.intent.Handoff.StatePath != pair.result.StatePath || pair.intent.Handoff.StateDigest != pair.result.StateDigest {
				t.Fatalf("handoff %s crossed its binding: %+v", pair.result.Nonce, pair.intent.Handoff)
			}
			if digest, err := verifyBoundHandoffState(root, pair.result.Nonce, pair.intent.Goal, *pair.intent.Handoff); err != nil || digest != pair.result.StateDigest {
				t.Fatalf("handoff %s immutable state no longer verifies: %s %v", pair.result.Nonce, digest, err)
			}
		}
	})

	t.Run("failure after supersession reports the exact durable partial state", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6300000000000003", "6300000000000004")
		first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		receiptPath := filepath.Join(root, "memory", "receipts.log")
		previous := beforeHandoffPrepare
		beforeHandoffPrepare = func() {
			if err := os.Rename(receiptPath, receiptPath+".saved"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(receiptPath, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() { beforeHandoffPrepare = previous })
		second, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow.Add(time.Minute), receiptPath)
		if err == nil || !strings.Contains(err.Error(), "superseded "+first.Nonce+", but the replacement intent is not live") {
			t.Fatalf("partial supersession result=%+v err=%v", second, err)
		}
		if second.Nonce != "6300000000000004" || second.StateDigest == "" {
			t.Fatalf("partial result omitted durable replacement state: %+v", second)
		}
		if live, liveErr := LiveIntents(root); liveErr != nil || len(live) != 0 {
			t.Fatalf("failed replacement left live authority: %+v %v", live, liveErr)
		}
		for _, nonce := range []string{first.Nonce, second.Nonce} {
			if _, statErr := os.Stat(filepath.Join(cancelledDir(root), nonce+".json")); statErr != nil {
				t.Fatalf("partial transition lost cancellation evidence for %s: %v", nonce, statErr)
			}
			if _, statErr := os.Stat(filepath.Join(HandoffDir(root, nonce), "state.json")); statErr != nil {
				t.Fatalf("partial transition lost immutable state for %s: %v", nonce, statErr)
			}
		}
	})
}

func TestConcurrentHandoffsDoNotCross(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6400000000000001", "6400000000000002")
	type outcome struct {
		result HandoffResult
		err    error
	}
	results := make(chan outcome, 2)
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
			results <- outcome{result: result, err: err}
		}()
	}
	workers.Wait()
	close(results)
	var got []HandoffResult
	for outcome := range results {
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		got = append(got, outcome.result)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Nonce < got[j].Nonce })
	if len(got) != 2 || got[0].Nonce == got[1].Nonce {
		t.Fatalf("concurrent nonces=%+v", got)
	}
	live, err := LiveIntents(root)
	if err != nil || len(live) != 1 {
		t.Fatalf("concurrent live intents=%+v err=%v", live, err)
	}
	for _, result := range got {
		if result.Intent.Handoff == nil || result.Intent.Handoff.StatePath != result.StatePath || result.Intent.Handoff.StateDigest != result.StateDigest ||
			filepath.Base(filepath.Dir(result.StatePath)) != result.Nonce {
			t.Fatalf("concurrent handoff crossed state and intent: %+v", result)
		}
		stateBytes, err := os.ReadFile(result.StatePath)
		if err != nil || testableDigest(stateBytes) != result.StateDigest {
			t.Fatalf("concurrent state %s digest drifted: %v", result.Nonce, err)
		}
	}
	cancelledNonce := got[0].Nonce
	if live[0].Nonce == cancelledNonce {
		cancelledNonce = got[1].Nonce
	}
	cancelled := cancelledHandoff(t, root, cancelledNonce)
	if cancelled.Outcome != "cancelled: superseded by "+live[0].Nonce {
		t.Fatalf("concurrent supersession was not serial: %+v", cancelled)
	}
}

func TestCancelHandoffReportsItsExactPartialOutcome(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6500000000000003")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cancelledDir(root), result.Nonce+".json.tmp")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, result.Nonce); err == nil || !strings.Contains(err.Error(), "is no longer live") {
		t.Fatalf("cancellation partial was not exact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(intentsDir(root), result.Nonce+".json")); !os.IsNotExist(err) {
		t.Fatalf("reported removed authorization is still live: %v", err)
	}
}

func TestVerifyHandoffStateUsesExactLifecycleRecord(t *testing.T) {
	t.Run("consumed remains verifiable after reaping and cannot be cancelled", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6600000000000001")
		result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		intent, err := ConsumeIntent(root, result.Nonce)
		if err != nil {
			t.Fatal(err)
		}
		intent.ReapedAt = handoffCaptureNow.Add(time.Hour).Format(time.RFC3339Nano)
		data, err := json.MarshalIndent(intent, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(consumedDir(root), result.Nonce+".json"), data)
		if digest, err := VerifyHandoffState(root, result.Nonce); err != nil || digest != result.StateDigest {
			t.Fatalf("reaped consumed state digest=%s err=%v", digest, err)
		}
		if err := CancelHandoff(root, result.Nonce); !errors.As(err, new(*HandoffRefusal)) || !strings.Contains(err.Error(), "HANDOFF_NOT_LIVE") {
			t.Fatalf("consumed handoff cancellation=%v", err)
		}
	})

	t.Run("drift and absence have distinct refusals", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6600000000000002")
		result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, result.StatePath, []byte("{}\n"))
		found, err := VerifyHandoffState(root, result.Nonce)
		if err == nil || !strings.Contains(err.Error(), "HANDOFF_STATE_MISMATCH expected="+result.StateDigest+" found="+found) {
			t.Fatalf("state drift found=%s err=%v", found, err)
		}
		if _, err := VerifyHandoffState(root, "ffffffffffffffff"); err == nil || !strings.Contains(err.Error(), "HANDOFF_NOT_FOUND") {
			t.Fatalf("missing state verification=%v", err)
		}
	})
}

func TestHandoffRejectsInvalidLiveAuthority(t *testing.T) {
	t.Run("cancel requires a bound handoff", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		intent := Intent{Nonce: "6600000000000003", Reason: "seatIdle"}
		if err := MintIntent(root, intent); err != nil {
			t.Fatal(err)
		}
		var refusal *HandoffRefusal
		if err := CancelHandoff(root, intent.Nonce); !errors.As(err, &refusal) || refusal.Code != "HANDOFF_NOT_LIVE" {
			t.Fatalf("ordinary live intent cancellation=%v", err)
		}
		if _, err := os.Stat(filepath.Join(intentsDir(root), intent.Nonce+".json")); err != nil {
			t.Fatalf("ordinary intent was cancelled: %v", err)
		}
	})

	t.Run("incomplete authorization refuses live lookup", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		if err := MintIntent(root, Intent{Nonce: "6600000000000004", Reason: seatHandoffReason}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := LiveHandoffForSession(root, handoffMainCaller().Session); err == nil || !strings.Contains(err.Error(), "incomplete seatHandoff authorization") {
			t.Fatalf("incomplete live handoff=%v", err)
		}
	})

	t.Run("drifted predecessor cannot be superseded", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6600000000000005")
		first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, first.StatePath, []byte("{}\n"))
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil || !strings.Contains(err.Error(), "invalid and cannot be superseded") {
			t.Fatalf("drifted predecessor supersession=%v", err)
		}
		if directories, intents := handoffArtifactCount(root); directories != 1 || intents != 1 {
			t.Fatalf("drifted predecessor minted directories=%d intents=%d", directories, intents)
		}
	})

	t.Run("duplicate session authorization is never selected", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6600000000000006", "6600000000000007")
		first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow.Add(time.Minute), filepath.Join(root, "memory", "receipts.log")); err != nil {
			t.Fatal(err)
		}
		cancelled, err := os.ReadFile(filepath.Join(cancelledDir(root), first.Nonce+".json"))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(intentsDir(root), first.Nonce+".json"), cancelled)
		if _, _, err := LiveHandoffForSession(root, handoffMainCaller().Session); err == nil || !strings.Contains(err.Error(), "more than one live handoff") {
			t.Fatalf("duplicate live lookup=%v", err)
		}
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow.Add(2*time.Minute), filepath.Join(root, "memory", "receipts.log")); err == nil || !strings.Contains(err.Error(), "more than one live handoff already names session") {
			t.Fatalf("duplicate supersession=%v", err)
		}
	})
}

func ageHandoffState(t *testing.T, root, nonce string, at time.Time) {
	t.Helper()
	path := filepath.Join(HandoffDir(root, nonce), "state.json")
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestContextPruneKeepsLiveHandoffs(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6700000000000001", "6700000000000002", "6700000000000003")
	first, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, first.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	orphanSamples := usage.SamplesPath(root, "orphan-runtime", "old-empty")
	writeTestFile(t, orphanSamples, nil)
	if err := os.Chtimes(orphanSamples, handoffCaptureNow.AddDate(0, 0, -30), handoffCaptureNow.AddDate(0, 0, -30)); err != nil {
		t.Fatal(err)
	}
	result, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow)
	if err != nil || result.CallSessions != 1 || len(result.Handoffs) != 0 {
		t.Fatalf("live handoff pruned: %+v %v", result, err)
	}
	if _, err := os.Stat(first.StatePath); err != nil {
		t.Fatalf("live handoff state missing: %v", err)
	}
	if err := CancelHandoff(root, first.Nonce); err != nil {
		t.Fatal(err)
	}
	result, err = PruneContext(root, 14*24*time.Hour, handoffCaptureNow)
	if err != nil || len(result.Handoffs) != 1 || result.Handoffs[0] != HandoffDir(root, first.Nonce) {
		t.Fatalf("cancelled handoff was not retired: %+v %v", result, err)
	}
	for _, retained := range []string{filepath.Join(cancelledDir(root), first.Nonce+".json"), BriefPath(root, first.Nonce)} {
		if _, err := os.Stat(retained); err != nil {
			t.Fatalf("prune crossed a lifecycle ownership boundary at %s: %v", retained, err)
		}
	}

	second, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := ConsumeIntent(root, second.Nonce)
	if err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, second.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	if result, err = PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err != nil || len(result.Handoffs) != 0 {
		t.Fatalf("consumed-active handoff pruned: %+v %v", result, err)
	}
	intent.ReapedAt = handoffCaptureNow.Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(consumedDir(root), second.Nonce+".json"), data)
	if result, err = PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err != nil || len(result.Handoffs) != 1 {
		t.Fatalf("reaped handoff was not retired: %+v %v", result, err)
	}
	boundary, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, boundary.Nonce); err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, boundary.Nonce, handoffCaptureNow.Add(-14*24*time.Hour))
	if result, err = PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err != nil || len(result.Handoffs) != 0 {
		t.Fatalf("cutoff-equal handoff was not retained: %+v %v", result, err)
	}
	if _, err := os.Stat(boundary.StatePath); err != nil {
		t.Fatalf("cutoff-equal state missing: %v", err)
	}
}

func TestContextPruneSerializesWithConsumption(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6800000000000001")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, result.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	lock, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			lock.Release()
		}
	})
	entered := make(chan struct{})
	previous := beforeArbitrationWait
	beforeArbitrationWait = func() { close(entered) }
	t.Cleanup(func() { beforeArbitrationWait = previous })
	type pruneOutcome struct {
		result ContextPruneResult
		err    error
	}
	done := make(chan pruneOutcome, 1)
	go func() {
		pruned, pruneErr := PruneContext(root, 15*24*time.Hour, handoffCaptureNow)
		done <- pruneOutcome{result: pruned, err: pruneErr}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("prune did not reach steward arbitration")
	}
	if _, err := ConsumeIntent(root, result.Nonce); err != nil {
		t.Fatal(err)
	}
	lock.Release()
	released = true
	pruned := <-done
	if pruned.err != nil || len(pruned.result.Handoffs) != 0 {
		t.Fatalf("prune did not see the serialized consumed-active state: %+v %v", pruned.result, pruned.err)
	}
	if _, err := os.Stat(result.StatePath); err != nil {
		t.Fatalf("serialized consumption lost state: %v", err)
	}
}

func TestContextPruneStopsOnUsageError(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6800000000000002")
	handoff, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, handoff.Nonce); err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	previous := pruneCallSessions
	pruneCallSessions = func(string, time.Time) (int, error) { return 3, errors.New("usage store damaged") }
	t.Cleanup(func() { pruneCallSessions = previous })
	result, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow)
	if err == nil || !strings.Contains(err.Error(), "usage store damaged") || result.CallSessions != 3 || len(result.Handoffs) != 0 {
		t.Fatalf("usage partial did not stop handoff prune: %+v %v", result, err)
	}
	if _, err := os.Stat(handoff.StatePath); err != nil {
		t.Fatalf("usage error allowed handoff removal: %v", err)
	}
}

func TestContextPruneDefaultAgeComposesWithUsageFloor(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	now := time.Now().UTC().Add(time.Hour)
	if result, err := PruneContext(root, 14*24*time.Hour, now); err != nil || result.CallSessions != 0 || len(result.Handoffs) != 0 {
		t.Fatalf("documented 14-day default was blocked by usage retention: %+v %v", result, err)
	}
}

func TestContextPruneRefusesRedirectedHandoffTrees(t *testing.T) {
	t.Run("nonce directory symlink", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		outside := t.TempDir()
		sentinel := filepath.Join(outside, "keep.txt")
		writeTestFile(t, sentinel, []byte("keep\n"))
		path := HandoffDir(root, "6800000000000003")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Fatal(err)
		}
		if _, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err == nil || !strings.Contains(err.Error(), "nonsymlink directory") {
			t.Fatalf("redirected nonce directory=%v", err)
		}
		if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep\n" {
			t.Fatalf("prune followed nonce symlink: %q %v", data, err)
		}
	})

	t.Run("member symlink", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		useHandoffNonces(t, "6800000000000004")
		handoff, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
		if err != nil {
			t.Fatal(err)
		}
		if err := CancelHandoff(root, handoff.Nonce); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "keep.txt")
		writeTestFile(t, outside, []byte("keep\n"))
		if err := os.Symlink(outside, filepath.Join(HandoffDir(root, handoff.Nonce), "redirected")); err != nil {
			t.Fatal(err)
		}
		ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
		if _, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err == nil || !strings.Contains(err.Error(), "redirected or nonregular member") {
			t.Fatalf("redirected tree member=%v", err)
		}
		if data, err := os.ReadFile(outside); err != nil || string(data) != "keep\n" {
			t.Fatalf("prune followed member symlink: %q %v", data, err)
		}
	})

	t.Run("publication parent symlink", func(t *testing.T) {
		root := handoffCaptureRepo(t, "claimed")
		outside := t.TempDir()
		parent := filepath.Join(root, "artifacts", "agents", "context", "handoffs")
		if err := os.MkdirAll(filepath.Dir(parent), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, parent); err != nil {
			t.Fatal(err)
		}
		useHandoffNonces(t, "6800000000000005")
		if _, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log")); err == nil || !strings.Contains(err.Error(), "redirected or nonregular") {
			t.Fatalf("redirected publication parent=%v", err)
		}
		if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
			t.Fatalf("publication crossed redirected parent: %v %v", entries, err)
		}
	})
}

func TestContextPruneRechecksBeforeRemoval(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6800000000000006")
	handoff, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, handoff.Nonce); err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	previous := beforeHandoffPruneRemove
	beforeHandoffPruneRemove = func(string) { writeTestFile(t, handoff.StatePath, []byte("{}\n")) }
	t.Cleanup(func() { beforeHandoffPruneRemove = previous })
	if _, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow); err == nil || !strings.Contains(err.Error(), "changed before removal") {
		t.Fatalf("changed handoff removal=%v", err)
	}
	if _, err := os.Stat(handoff.StatePath); err != nil {
		t.Fatalf("recheck failure removed handoff: %v", err)
	}
}

func TestContextPruneReportsRemovalBeforeSyncFailure(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6800000000000007")
	handoff, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, handoff.Nonce); err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	previous := syncHandoffDir
	syncHandoffDir = func(path string) error {
		if path == filepath.Dir(HandoffDir(root, handoff.Nonce)) {
			return errors.New("injected post-removal sync failure")
		}
		return previous(path)
	}
	t.Cleanup(func() { syncHandoffDir = previous })
	result, err := PruneContext(root, 14*24*time.Hour, handoffCaptureNow)
	if err == nil || !strings.Contains(err.Error(), "injected post-removal sync failure") || len(result.Handoffs) != 1 || result.Handoffs[0] != HandoffDir(root, handoff.Nonce) {
		t.Fatalf("completed removal was omitted from partial result: %+v %v", result, err)
	}
	if _, err := os.Stat(handoff.StatePath); !os.IsNotExist(err) {
		t.Fatalf("reported removal still exists: %v", err)
	}
}

func TestContextPruneRetainsDamageAndRefusesBadBounds(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	for _, olderThan := range []time.Duration{0, -time.Hour} {
		result, err := PruneContext(root, olderThan, handoffCaptureNow)
		if err == nil || err.Error() != "context prune older-than must be positive" || result.CallSessions != 0 || len(result.Handoffs) != 0 {
			t.Fatalf("retention bound %s was not refused exactly: %+v %v", olderThan, result, err)
		}
	}
	if result, err := PruneContext(root, time.Hour, time.Time{}); err == nil || err.Error() != "context prune requires one nonzero clock observation" || result.CallSessions != 0 || len(result.Handoffs) != 0 {
		t.Fatalf("zero prune clock was not refused exactly: %+v %v", result, err)
	}
	parent := filepath.Join(root, "artifacts", "agents", "context", "handoffs")
	malformed := filepath.Join(parent, "not-a-nonce", "evidence.txt")
	incomplete := filepath.Join(parent, "6c00000000000001", "evidence.txt")
	writeTestFile(t, malformed, []byte("malformed identity\n"))
	writeTestFile(t, incomplete, []byte("incomplete capture\n"))
	result, err := PruneContext(root, 15*24*time.Hour, handoffCaptureNow)
	if err == nil || len(result.Handoffs) != 0 || !strings.Contains(err.Error(), "malformed nonce") || !strings.Contains(err.Error(), "keep incomplete handoff") {
		t.Fatalf("damage was not retained and reported: %+v %v", result, err)
	}
	for _, path := range []string{malformed, incomplete} {
		if data, err := os.ReadFile(path); err != nil || len(data) == 0 {
			t.Fatalf("prune removed damaged evidence %s: %q %v", path, data, err)
		}
	}
	useHandoffNonces(t, "6c00000000000002")
	handoff, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CancelHandoff(root, handoff.Nonce); err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(HandoffDir(root, handoff.Nonce), "unowned-evidence.txt")
	writeTestFile(t, extra, []byte("not declared by the snapshot\n"))
	ageHandoffState(t, root, handoff.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	if result, err = PruneContext(root, 15*24*time.Hour, handoffCaptureNow); err == nil || len(result.Handoffs) != 0 || !strings.Contains(err.Error(), "unexpected member") {
		t.Fatalf("prune did not preserve undeclared evidence: %+v %v", result, err)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("prune deleted undeclared evidence: %v", err)
	}
}

func TestHandoffAndDiagnosticsHaveSeparateLifetimes(t *testing.T) {
	root := handoffCaptureRepo(t, "claimed")
	useHandoffNonces(t, "6a00000000000001")
	result, err := Handoff(root, handoffMainCaller(), nil, handoffCaptureNow, filepath.Join(root, "memory", "receipts.log"))
	if err != nil {
		t.Fatal(err)
	}
	temporaryParent := filepath.Join(root, "operator-tmp")
	if err := os.MkdirAll(temporaryParent, 0o755); err != nil {
		t.Fatal(err)
	}
	var diagnosticRoot string
	previousMake := makeContextDiagnosticRoot
	previousRemove := removeContextDiagnosticRoot
	makeContextDiagnosticRoot = func(_ string, pattern string) (string, error) {
		var err error
		diagnosticRoot, err = os.MkdirTemp(temporaryParent, pattern)
		return diagnosticRoot, err
	}
	t.Cleanup(func() { makeContextDiagnosticRoot, removeContextDiagnosticRoot = previousMake, previousRemove })
	transcript := writeContextRuntimeTranscript(t, t.TempDir(), "claude", "diagnostic", 120000, false)
	if err := usage.RegisterSession(root, "claude", "retained-registration", 8001, 101); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
	registryBefore, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, result.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	cleanupEntered, releaseCleanup := make(chan struct{}), make(chan struct{})
	removeContextDiagnosticRoot = func(path string) error {
		close(cleanupEntered)
		<-releaseCleanup
		return os.RemoveAll(path)
	}
	diagnosticDone := make(chan error, 1)
	go func() {
		_, err := readContextTranscriptOverride("claude", "diagnostic", usage.ReadOptions{
			Capability: usage.PerCall, Transcript: transcript, Toplevel: root, Now: handoffCaptureNow,
		})
		diagnosticDone <- err
	}()
	<-cleanupEntered
	pruned, pruneErr := PruneContext(root, 15*24*time.Hour, handoffCaptureNow)
	close(releaseCleanup)
	if err := <-diagnosticDone; err != nil {
		t.Fatal(err)
	}
	if pruneErr != nil || len(pruned.Handoffs) != 0 {
		t.Fatalf("concurrent diagnostic changed live handoff retention: %+v %v", pruned, pruneErr)
	}
	removeContextDiagnosticRoot = previousRemove
	if diagnosticRoot == "" {
		t.Fatal("diagnostic root was not allocated")
	}
	if _, err := os.Stat(diagnosticRoot); !os.IsNotExist(err) {
		t.Fatalf("diagnostic evidence outlived its private root: %v", err)
	}
	if _, err := os.Stat(result.StatePath); err != nil {
		t.Fatalf("diagnostic cleanup reached live handoff state: %v", err)
	}
	if registryAfter, err := os.ReadFile(registry); err != nil || !bytes.Equal(registryBefore, registryAfter) {
		t.Fatalf("diagnostic/prune changed retained registration: before=%q after=%q err=%v", registryBefore, registryAfter, err)
	}
	userOwned := filepath.Join(root, "metasystem-context-diagnostic-user-owned", "keep.txt")
	writeTestFile(t, userOwned, []byte("keep\n"))
	if err := CancelHandoff(root, result.Nonce); err != nil {
		t.Fatal(err)
	}
	ageHandoffState(t, root, result.Nonce, handoffCaptureNow.AddDate(0, 0, -30))
	if _, err := PruneContext(root, 15*24*time.Hour, handoffCaptureNow); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(userOwned); err != nil {
		t.Fatalf("context prune swept a prefix-like directory it does not own: %v", err)
	}
}
