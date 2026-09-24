package missionrunner

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// orphanAnchorFacts is a saved physical anchor. Only a checked pin can
// replace it; reading the live ledger never changes the recorded Git facts.
type orphanAnchorFacts struct {
	hash, sha, ledger, rel, ref, commit, blob, message string
}

type orphanContinuity struct {
	t                       *testing.T
	b                       *resolutionFileBed
	anchor                  orphanAnchorFacts
	raw                     mission.RawAnchorOperations
	reads                   map[string]int
	remaining               map[string]int
	reconcile, verify, pins int
	anchorWrites            int
}

func (c *orphanContinuity) saveAnchor() {
	c.t.Helper()
	b := c.b
	state := readTestDoc(c.t, b.state)
	cycles, ok := jsonInt(state["ledger"].(map[string]any)["cycles"])
	if !ok || state["integrity"].(map[string]any)["hash"] != b.anchorHash ||
		sha256Hex(string(b.anchored)) != b.anchorSHA {
		c.t.Fatal("saved anchor differs from checked state or saved ledger")
	}
	rel, err := filepath.Rel(b.e.Root, b.ledger)
	if err != nil {
		c.t.Fatal(err)
	}
	missionID := b.e.Mission
	sum := sha256.Sum256([]byte(b.anchorHash + ":" + b.anchorSHA))
	commit := fmt.Sprintf("%x", sum[:20])
	bytes := string(b.anchored)
	blobSum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(bytes), bytes)))
	c.anchor = orphanAnchorFacts{
		hash: b.anchorHash, sha: b.anchorSHA, ledger: bytes, rel: filepath.ToSlash(rel),
		ref:    "refs/metasystem/missions/" + missionID + "/state-anchors",
		commit: commit, blob: fmt.Sprintf("%x", blobSum),
		message: fmt.Sprintf("mission(%s): anchor cycle %d\n\nMission-Id: %s\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: %s\nMission-Cycle: %d\n\n",
			missionID, cycles, missionID, b.anchorHash, b.anchorSHA, filepath.ToSlash(rel), cycles),
	}
}

func (c *orphanContinuity) git(root string, args ...string) string {
	c.t.Helper()
	if root != c.b.e.Root {
		c.t.Fatalf("anchor root %q", root)
	}
	a := c.anchor
	var key, result string
	switch {
	case reflect.DeepEqual(args, []string{"for-each-ref", "--format=%(refname)", a.ref}):
		key, result = "ref", a.ref+"\n"
	case reflect.DeepEqual(args, []string{"log", "-1", "--format=%H%x1f%B", a.ref}):
		key, result = "log", a.commit+"\x1f"+a.message
	case reflect.DeepEqual(args, []string{"merge-base", "--is-ancestor", a.commit, a.ref}):
		key = "ancestry"
	case reflect.DeepEqual(args, []string{"show", a.commit + ":" + a.rel}):
		key, result = "blob", a.ledger
	case reflect.DeepEqual(args, []string{"ls-tree", "-r", "--full-tree", "-z", a.commit}):
		key, result = "tree", "100644 blob "+a.blob+"\t"+a.rel+"\x00"
	case reflect.DeepEqual(args, []string{"rev-list", "--parents", "-n", "1", a.commit}):
		key, result = "parents", a.commit+"\n"
	default:
		c.t.Fatalf("undeclared anchor Git call %q %q", root, args)
	}
	c.reads[key]++
	if c.remaining[key] == 0 {
		c.t.Fatalf("anchor Git call exceeds declared %s reads", key)
	}
	c.remaining[key]--
	return result
}

func (c *orphanContinuity) checkPaths(state, root, ledger string) {
	c.t.Helper()
	if state != c.b.state || root != c.b.e.Root || ledger != c.b.ledger {
		c.t.Fatalf("continuity paths %q %q %q", state, root, ledger)
	}
}

func (c *orphanContinuity) Reconcile(state, root, ledger string) (int, error) {
	c.checkPaths(state, root, ledger)
	c.reconcile++
	return mission.ReconcileWithRawAnchorOperations(c.raw, state, root, ledger)
}
func (c *orphanContinuity) VerifyStateWithAnchor(state, root, ledger string) (int64, string, error) {
	c.checkPaths(state, root, ledger)
	c.verify++
	return mission.VerifyStateWithRawAnchorOperations(c.raw, state, root, ledger)
}
func (c *orphanContinuity) LedgerPin(root, missionID string) (string, error) {
	if root != c.b.e.Root || missionID != c.b.e.Mission {
		c.t.Fatalf("ledger pin %q %q", root, missionID)
	}
	c.pins++
	return mission.AnchoredLedgerSHAWithRawAnchorOperations(c.raw, root, missionID)
}

type orphanWallReads struct {
	*resolutionFacts
	b     *resolutionFileBed
	c     *orphanContinuity
	git   []wallGitReply
	truth int
}

func (f *orphanWallReads) Git(root string, args ...string) (string, string, int) {
	f.t.Helper()
	if len(f.git) == 0 {
		f.t.Fatalf("undeclared resume Git call %q %q", root, args)
	}
	want := f.git[0]
	f.git = f.git[1:]
	if root != want.root || !reflect.DeepEqual(args, want.args) {
		f.t.Fatalf("resume Git call %q %q, want %q %q", root, args, want.root, want.args)
	}
	return want.stdout, want.stderr, want.code
}

func (f *orphanWallReads) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	f.next("LedgerTruth", root, state, path)
	if root != f.b.e.Root || path != f.b.ledger || !reflect.DeepEqual(state, f.stateDoc()) {
		f.t.Fatalf("ledger truth arguments %q %q", root, path)
	}
	f.truth++
	live, err := os.ReadFile(path)
	return f.c.anchor.ledger, string(live), err
}

func newOrphanResumeBed(t *testing.T) (*resolutionFileBed, *orphanContinuity, *orphanWallReads) {
	t.Helper()
	b := newResolutionFileBed(t)
	b.expectResolve(recoveryPost, false, true, false)
	if code := b.e.ResolveTaint(1, "adopt-disputed-tree", "", "Wido", "keeping the disputed work",
		[]string{"authorship of solo.go"}); code != 0 {
		t.Fatalf("adoption must succeed: %d", code)
	}
	if b.pins != 1 {
		t.Fatalf("adoption anchor pins = %d", b.pins)
	}
	c := &orphanContinuity{t: t, b: b, reads: make(map[string]int), remaining: map[string]int{
		"ref": 3, "log": 3, "ancestry": 2, "blob": 2, "tree": 2, "parents": 2,
	}}
	c.saveAnchor()
	c.raw = mission.RawAnchorOperations{
		GitOutput: func(root string, args ...string) (string, error) { return c.git(root, args...), nil },
		GitTry:    func(root string, args ...string) (string, int) { return c.git(root, args...), 0 },
		GitStdinOutput: func(root string, data []byte, args ...string) (string, error) {
			t.Fatalf("unexpected anchor write: %q %q (%d bytes)", root, args, len(data))
			return "", nil
		},
		GitEnvOutput: func(root string, env []string, args ...string) (string, error) {
			t.Fatalf("unexpected anchor write: %q %q %q", root, env, args)
			return "", nil
		},
	}
	b.e.continuityFacts = c
	pin, err := c.LedgerPin(b.e.Root, b.e.Mission)
	if err != nil || pin != b.anchorSHA {
		t.Fatalf("saved anchor ledger pin = %q, error %v", pin, err)
	}
	reads := &orphanWallReads{resolutionFacts: b.facts, b: b, c: c, git: []wallGitReply{
		{root: b.e.Root, args: []string{"config", "--local", "--type=bool", "--get", "core.fileMode"}, stdout: "true\n"},
		{root: b.e.Root, args: []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}, stdout: recoveryHead + "\n"},
	}}
	b.e.wallReadFacts = reads
	b.e.anchorFn = func(state, ledger, identity string) error {
		if state != b.state || ledger != b.ledger || identity != b.nextIdentity {
			t.Fatalf("unexpected orphan anchor %q %q %q", state, ledger, identity)
		}
		_, hash, err := mission.VerifyStateShape(state)
		if err != nil {
			return err
		}
		if hash == c.anchor.hash {
			t.Fatal("orphan park did not advance state")
		}
		live, err := os.ReadFile(ledger)
		if err != nil {
			return err
		}
		if sha256Hex(string(live)) != c.anchor.sha {
			t.Fatal("orphan anchor ledger moved from verified pin")
		}
		b.recordAnchor()
		c.saveAnchor()
		c.anchorWrites++
		return nil
	}
	writeJSONFile(t, b.e.birthRecordPath(), map[string]any{"missionId": b.e.Mission, "bornAt": "2026-01-01T00:00:00Z"})
	b.e.pinnedAnchorEffect = func(state, ledger, identity, hash, sha string) error {
		t.Fatalf("unexpected pinned anchor write: %q %q %q %q %q", state, ledger, identity, hash, sha)
		return nil
	}
	t.Cleanup(func() {
		if b.pins != 1 {
			t.Errorf("unexpected resolution anchor writes: %d", b.pins)
		}
		if len(reads.git) != 0 {
			t.Errorf("unconsumed resume Git declarations: %v", reads.git)
		}
		if c.reconcile != 1 || c.verify != 1 || c.pins != 1 || c.anchorWrites != 1 {
			t.Errorf("continuity calls: reconcile=%d verify=%d pin=%d anchor=%d", c.reconcile, c.verify, c.pins, c.anchorWrites)
		}
		for key, left := range c.remaining {
			if left != 0 {
				t.Errorf("%d declared %s anchor reads were not consumed", left, key)
			}
		}
	})
	return b, c, reads
}

func assertOrphanResume(t *testing.T, b *resolutionFileBed, orphan string) {
	t.Helper()
	state := readTestDoc(t, b.state)
	entries := state["workspaceTaint"].(map[string]any)["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("taint entries = %v", entries)
	}
	first := entries[0].(map[string]any)
	second := entries[1].(map[string]any)
	if first["resolution"].(map[string]any)["variant"] != "adopt-disputed-tree" || first["turnId"] != "alpha-t1-live" {
		t.Fatalf("original adoption changed: %v", first)
	}
	if second["turnId"] != orphan || second["resolution"] != nil {
		t.Fatalf("orphan taint: %v", second)
	}
	secondID, ok := jsonInt(second["taintId"])
	if !ok {
		t.Fatalf("orphan taint id: %v", second)
	}
	asks, err := filepath.Glob(filepath.Join(asksDirPath(b.e.Root, b.e.Mission), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, path := range asks {
		ask := readTestDoc(t, path)
		id, valid := jsonInt(ask["taintId"])
		if valid && id == secondID && ask["reasonClass"] == "wall-violation" && ask["answeredAt"] == nil && ask["supersededBy"] == nil {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("matching open orphan asks = %d, files = %v", matched, asks)
	}
	turns, err := os.ReadDir(filepath.Join(b.e.missionDir(), "turns"))
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 2 {
		t.Fatalf("resume opened a host turn: %v", turns)
	}
	for _, turn := range turns {
		if turn.Name() != "alpha-t1-live" && turn.Name() != orphan {
			t.Fatalf("new host turn %q", turn.Name())
		}
	}
	t.Logf("adoption resolved; orphan %s has one unresolved taint and one open ask; no host turn opened", orphan)
}
