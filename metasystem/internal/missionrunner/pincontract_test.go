package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinVerifiedContract's ladder with real files: the start-mode
// bootstrap, the double-pin refusal, resume against absent fences, the
// digest mismatch, and identity checks.
func TestPinVerifiedContract(t *testing.T) {
	engine := &Engine{Root: t.TempDir(), Mission: "mr-pin"}
	anchors := &strictWallReads{t: t, git: []wallGitReply{
		{root: engine.Root, args: []string{"for-each-ref", "--format=%(refname)", "refs/metasystem/missions/mr-pin/"}},
		{root: engine.Root, args: []string{"for-each-ref", "--format=%(refname)", "refs/metasystem/missions/mr-pin/"}},
		{root: engine.Root, args: []string{"for-each-ref", "--format=%(refname)", "refs/metasystem/missions/mr-pin/"}},
	}}
	engine.wallReadFacts = anchors
	t.Cleanup(anchors.done)
	snapshot := []byte("contract bytes\n")
	sum := sha256.Sum256(snapshot)
	sha := hex.EncodeToString(sum[:])

	// Resume before any start: refused by name.
	if err := engine.pinVerifiedContract("resume", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "fence state is absent") {
		t.Fatalf("resume without fences: %v", err)
	}

	// A wrong digest never pins.
	if err := engine.pinVerifiedContract("start", snapshot, "deadbeef"); err == nil ||
		!strings.Contains(err.Error(), "does not match its verified raw-file sha256") {
		t.Fatalf("digest mismatch: %v", err)
	}

	// A clean start bootstraps the fences and lands the snapshot.
	if err := engine.pinVerifiedContract("start", snapshot, sha); err != nil {
		t.Fatalf("clean start: %v", err)
	}
	pinned, err := os.ReadFile(engine.approvedContractPath())
	if err != nil || string(pinned) != string(snapshot) {
		t.Fatalf("snapshot not pinned: %v %q", err, pinned)
	}

	// Before the mission is BORN (no state.json), a pinned-but-stateless
	// id is a stillborn remnant and a corrected start may re-pin. Once
	// the state exists, a second start refuses toward resume.
	if err := engine.pinVerifiedContract("start", snapshot, sha); err != nil {
		t.Fatalf("a stillborn pin must be re-pinnable: %v", err)
	}
	os.MkdirAll(engine.missionDir(), 0o755)
	writeText(t, filepath.Join(engine.missionDir(), "state.json"), "{}")
	if err := engine.pinVerifiedContract("start", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "already pinned; use resume") {
		t.Fatalf("double pin after birth: %v", err)
	}

	// Resume with the fences present succeeds.
	if err := engine.pinVerifiedContract("resume", snapshot, sha); err != nil {
		t.Fatalf("lawful resume: %v", err)
	}

	// A foreign missionId in the counters is an identity refusal.
	other := &Engine{Root: engine.Root, Mission: "mr-pin"}
	other.Mission = "mr-other"
	os.MkdirAll(other.missionDir(), 0o755)
	os.WriteFile(other.fencesPath(), []byte(`{"schemaVersion":1,"missionId":"mr-pin","startedAt":"x","cycles":0,"reservations":{}}`), 0o644)
	if err := other.pinVerifiedContract("resume", snapshot, sha); err == nil ||
		!strings.Contains(err.Error(), "invalid identity") {
		t.Fatalf("foreign identity: %v", err)
	}

	// An unreadable namespace cannot be treated as a successful empty one.
	t.Run("anchor probe failure", func(t *testing.T) {
		probe := &Engine{Root: t.TempDir(), Mission: "mr-pin"}
		facts := &strictWallReads{t: t, git: []wallGitReply{{
			root: probe.Root, args: []string{"for-each-ref", "--format=%(refname)", "refs/metasystem/missions/mr-pin/"},
			stderr: "probe unavailable", code: 1,
		}}}
		probe.wallReadFacts = facts
		if err := probe.pinVerifiedContract("start", snapshot, sha); err == nil ||
			!strings.Contains(err.Error(), "cannot prove the mission's anchor namespace is empty: probe unavailable") {
			t.Fatalf("failed probe must refuse instead of authorizing start: %v", err)
		}
		if pathExists(probe.approvedContractPath()) {
			t.Fatal("failed probe wrote a contract pin")
		}
		facts.done()
	})
}
