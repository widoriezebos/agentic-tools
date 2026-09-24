package missionrunner

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type driftAnchorContinuity struct {
	*resolutionFileBed
	raw            mission.RawAnchorOperations
	verifyErr      error
	reconcileCode  int
	reconcileErr   error
	reconcileCalls int
}

func (c *driftAnchorContinuity) VerifyStateWithAnchor(state, root, ledger string) (int64, string, error) {
	c.checkPaths(state, root, ledger)
	sequence, hash, err := mission.VerifyStateWithRawAnchorOperations(c.raw, state, root, ledger)
	c.verifyErr = err
	return sequence, hash, err
}

func (c *driftAnchorContinuity) Reconcile(state, root, ledger string) (int, error) {
	c.checkPaths(state, root, ledger)
	c.reconcileCalls++
	c.reconcileCode, c.reconcileErr = mission.ReconcileWithRawAnchorOperations(c.raw, state, root, ledger)
	return c.reconcileCode, c.reconcileErr
}

func (c *driftAnchorContinuity) checkPaths(state, root, ledger string) {
	if state != c.state || root != c.e.Root || ledger != c.ledger {
		c.t.Fatalf("continuity paths = %q %q %q, want %q %q %q", state, root, ledger, c.state, c.e.Root, c.ledger)
	}
}

func newDriftAnchorContinuity(t *testing.T, b *resolutionFileBed) *driftAnchorContinuity {
	t.Helper()
	state := readTestDoc(t, b.state)
	missionID := state["missionId"].(string)
	cycles, ok := jsonInt(state["ledger"].(map[string]any)["cycles"])
	if !ok {
		t.Fatal("saved state has no ledger cycle count")
	}
	if hash := state["integrity"].(map[string]any)["hash"]; hash != b.anchorHash {
		t.Fatalf("saved anchor hash %q differs from state %q", b.anchorHash, hash)
	}
	anchored := string(b.anchored)
	if b.anchorSHA != sha256Hex(anchored) {
		t.Fatal("saved anchor ledger digest differs from saved bytes")
	}
	ledgerRel, err := filepath.Rel(b.e.Root, b.ledger)
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/metasystem/missions/" + missionID + "/state-anchors"
	commitSum := sha256.Sum256([]byte(b.anchorHash + ":" + b.anchorSHA))
	commit := fmt.Sprintf("%x", commitSum[:20])
	blobSum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(anchored), anchored)))
	blob := fmt.Sprintf("%x", blobSum)
	message := fmt.Sprintf("mission(%s): anchor cycle %d\n\nMission-Id: %s\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: %s\nMission-Cycle: %d\n\n",
		missionID, cycles, missionID, b.anchorHash, b.anchorSHA, ledgerRel, cycles)

	var calls []testgit.Expectation
	output := func(stdout string, args ...string) {
		calls = append(calls, testgit.Expectation{Call: testgit.Call{Dir: b.e.Root, Args: args}, Result: testgit.Result{Stdout: []byte(stdout)}})
	}
	tip := func() {
		output(ref+"\n", "for-each-ref", "--format=%(refname)", ref)
		output(commit+"\x1f"+message, "log", "-1", "--format=%H%x1f%B", ref)
	}
	// The saved anchor verifies before the live ledger changes. This exercises
	// its path, ancestry, tree shape, and blob through the mission owner.
	tip()
	output("", "merge-base", "--is-ancestor", commit, ref)
	output(anchored, "show", commit+":"+ledgerRel)
	output("100644 blob "+blob+"\t"+ledgerRel+"\x00", "ls-tree", "-r", "--full-tree", "-z", commit)
	output(commit+"\n", "rev-list", "--parents", "-n", "1", commit)
	// Resolve verifies once, then Reconcile verifies and probes for a safe
	// recovery. Each read still sees the saved anchor, never the live append.
	tip()
	tip()
	tip()
	output(".git\n", "rev-parse", "--git-dir")
	tip()
	stub := testgit.New(t, calls...)
	raw := mission.RawAnchorOperations{
		GitOutput: func(root string, args ...string) (string, error) {
			result := stub.Run(testgit.Call{Dir: root, Args: args})
			return string(result.Stdout), result.Err
		},
		GitTry: func(root string, args ...string) (string, int) {
			result := stub.Run(testgit.Call{Dir: root, Args: args})
			if result.Err != nil {
				return string(result.Stdout), -1
			}
			return string(result.Stdout), 0
		},
		GitStdinOutput: func(root string, data []byte, args ...string) (string, error) {
			t.Fatalf("unexpected anchor write: root=%q args=%q bytes=%d", root, args, len(data))
			return "", testgit.ErrUnexpectedCall
		},
		GitEnvOutput: func(root string, env []string, args ...string) (string, error) {
			t.Fatalf("unexpected anchor write: root=%q env=%q args=%q", root, env, args)
			return "", testgit.ErrUnexpectedCall
		},
	}
	if _, _, err := mission.VerifyStateWithRawAnchorOperations(raw, b.state, b.e.Root, b.ledger); err != nil {
		t.Fatalf("saved anchor must verify before the ledger append: %v", err)
	}
	c := &driftAnchorContinuity{resolutionFileBed: b, raw: raw}
	b.e.continuityFacts = c
	return c
}

func assertDriftRefused(t *testing.T, b *resolutionFileBed, c *driftAnchorContinuity, savedHash, savedSHA, savedLedger string, pins int) {
	t.Helper()
	if c.verifyErr == nil || !strings.Contains(c.verifyErr.Error(), "Mission-Ledger-SHA256") {
		t.Fatalf("VerifyStateWithAnchor must identify ledger drift: %v", c.verifyErr)
	}
	if c.reconcileCalls != 1 || c.reconcileCode != 3 || c.reconcileErr != nil {
		t.Fatalf("Reconcile result: calls=%d code=%d error=%v", c.reconcileCalls, c.reconcileCode, c.reconcileErr)
	}
	state := readTestDoc(t, b.state)
	if state["status"] != "parked" || state["parkReason"] != "state-integrity" || unresolvedTaint(state) == "" {
		t.Fatalf("reconciliation must preserve the parked taint: status=%v reason=%v taint=%v", state["status"], state["parkReason"], state["workspaceTaint"])
	}
	entry := state["workspaceTaint"].(map[string]any)["entries"].([]any)[0].(map[string]any)
	if entry["resolution"] != nil {
		t.Fatalf("refused resolution was saved: %v", entry["resolution"])
	}
	asks, err := filepath.Glob(filepath.Join(asksDirPath(b.e.Root, b.e.Mission), "wall-violation*.json"))
	if err != nil || len(asks) != 1 {
		t.Fatalf("wall ask files: %v, %v", asks, err)
	}
	ask := readTestDoc(t, asks[0])
	if ask["answeredAt"] != nil || ask["answer"] != nil {
		t.Fatalf("refused resolution answered the ask: %v", ask)
	}
	if b.pins != pins || b.anchorHash != savedHash || b.anchorSHA != savedSHA || string(b.anchored) != savedLedger {
		t.Fatal("refused resolution advanced the saved anchor")
	}
	t.Logf("raw mission refusal: VerifyStateWithAnchor=%v; Reconcile=code %d, error %v, park reason %v", c.verifyErr, c.reconcileCode, c.reconcileErr, state["parkReason"])
}
