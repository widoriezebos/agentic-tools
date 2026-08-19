package mission

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

const oneCycleLedger = `# Mission Ledger

- Cycle budget: 5
- No-gain budget: 3

### Cycle 1
- Classification: contract-improved; candidate-sha=abcdef; observed=score=0.5
`

// anchoredMission builds a git repo on the mission branch with a one-cycle
// ledger and a state whose ledger cycle count matches, then anchors it.
func anchoredMission(t *testing.T) (repo, state, ledger string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo = t.TempDir()
	gitCmd(t, repo, "init", "-q")
	writeText(t, filepath.Join(repo, "README"), "seed\n")
	gitCmd(t, repo, "add", "README")
	gitCmd(t, repo, "commit", "-q", "-m", "init")
	gitCmd(t, repo, "checkout", "-q", "-b", "feature-x")

	ledger = filepath.Join(repo, "ledger.md")
	writeText(t, ledger, oneCycleLedger)
	contract := filepath.Join(repo, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n")
	state = filepath.Join(repo, "state.json")
	if err := InitStateWithBaseline(state, contract, ledger, "", "feature-x", strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}

	// Advance the state's recorded ledger cycles to match the ledger (1).
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["ledger"].(map[string]any)["cycles"] = 1
	src := state + ".src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, src, hash); err != nil {
		t.Fatal(err)
	}

	if err := Anchor(state, repo, ledger); err != nil {
		t.Fatalf("anchor: %v", err)
	}
	return repo, state, ledger
}

func TestAnchorAndVerifyRoundTrip(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	seq, hash, err := VerifyStateWithAnchor(state, repo, ledger)
	if err != nil {
		t.Fatalf("a freshly anchored state should verify: %v", err)
	}
	if !hashRe.MatchString(hash) || seq < 1 {
		t.Fatalf("unexpected verify result: seq=%d hash=%q", seq, hash)
	}
	// The anchor commit carries the mission trailers — on the
	// runner-owned anchor ref, never the mission branch (slice-5 r4).
	log := gitCmd(t, repo, "log", "-1", "--format=%B", "refs/metasystem/missions/demo/state-anchors")
	if !strings.Contains(log, "Mission-Id: demo") || !strings.Contains(log, "Mission-Cycle: 1") {
		t.Fatalf("anchor commit missing trailers:\n%s", log)
	}
}

// Anchor cadence (slice-6 round-3 finding 1): re-binding the SAME state
// hash to DIFFERENT ledger bytes is the mid-turn laundering shape and
// refuses; re-anchoring an identical position stays allowed (heal retry).
func TestAnchorRefusesLedgerMoveWithoutStateWrite(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	if _, err := anchorWrite(state, repo, ledger); err != nil {
		t.Fatalf("an identical re-anchor must stay allowed: %v", err)
	}
	// Tamper that keeps the cycle count parseable and identical.
	writeText(t, ledger, strings.Replace(oneCycleLedger, "observed=score=0.5", "observed=score=0.9", 1))
	if _, err := anchorWrite(state, repo, ledger); err == nil ||
		!strings.Contains(err.Error(), "ledger bytes changed without a state write") {
		t.Fatalf("a ledger move without a state write must refuse, got %v", err)
	}
}

// The pinned anchor binds exactly the verified position (slice-6
// successor finding 2): moved state or moved ledger bytes refuse.
func TestAnchorPinsRefuseMovedPosition(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	doc, _ := readStateDoc(state)
	integrity, _ := doc["integrity"].(map[string]any)
	hash, _ := integrity["hash"].(string)
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	lHash := sha256Hex(string(data))
	if err := AnchorNamed(state, repo, ledger, "Wido", hash, lHash); err != nil {
		t.Fatalf("matching pins must anchor: %v", err)
	}
	if err := AnchorNamed(state, repo, ledger, "Wido", strings.Repeat("0", 64), lHash); err == nil ||
		!strings.Contains(err.Error(), "state moved past the verified position") {
		t.Fatalf("a moved state pin must refuse, got %v", err)
	}
	if err := AnchorNamed(state, repo, ledger, "Wido", hash, strings.Repeat("0", 64)); err == nil ||
		!strings.Contains(err.Error(), "ledger moved past the verified bytes") {
		t.Fatalf("a moved ledger pin must refuse, got %v", err)
	}
}

// The two round-16/17 recovery folds, regression-certified: a deleted
// live ledger in the one-step gap refuses WITHOUT writing and heals
// after blob restoration; a healable lag whose publication fails
// (wrong checked-out branch) refuses WITHOUT writing and heals once
// the branch is restored.
func TestOneStepRecoveryShapes(t *testing.T) {
	t.Run("deleted-ledger", func(t *testing.T) {
		repo, state, ledger := anchoredMission(t)
		doc, _ := readStateDoc(state)
		doc["fences"].(map[string]any)["cycles"] = 1
		source := state + ".src"
		if err := atomicWriteJSON(source, doc); err != nil {
			t.Fatal(err)
		}
		_, hash, _ := VerifyStateShape(state)
		if err := WriteState(state, source, hash); err != nil {
			t.Fatal(err)
		}
		pristine, err := os.ReadFile(ledger)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(ledger); err != nil {
			t.Fatal(err)
		}
		_, beforeHash, _ := VerifyStateShape(state)
		code, rerr := Reconcile(state, repo, ledger)
		if code == 0 || rerr == nil || !strings.Contains(rerr.Error(), "restore the ledger bytes") {
			t.Fatalf("a deleted ledger in the one-step gap must refuse with the repair: code=%d err=%v", code, rerr)
		}
		_, afterHash, _ := VerifyStateShape(state)
		if afterHash != beforeHash {
			t.Fatal("the refusal must not write")
		}
		writeText(t, ledger, string(pristine))
		if code, rerr := Reconcile(state, repo, ledger); rerr != nil || code != 0 {
			t.Fatalf("restoration must heal: code=%d err=%v", code, rerr)
		}
	})
	t.Run("failed-lag-publication", func(t *testing.T) {
		repo, state, ledger := anchoredMission(t)
		doc, _ := readStateDoc(state)
		doc["fences"].(map[string]any)["cycles"] = 1
		source := state + ".src"
		if err := atomicWriteJSON(source, doc); err != nil {
			t.Fatal(err)
		}
		_, hash, _ := VerifyStateShape(state)
		if err := WriteState(state, source, hash); err != nil {
			t.Fatal(err)
		}
		// The healable exact-match lag exists; force publication failure
		// deterministically via the wrong checked-out branch.
		gitCmd(t, repo, "checkout", "-q", "-b", "not-the-mission-branch")
		_, beforeHash, _ := VerifyStateShape(state)
		code, rerr := Reconcile(state, repo, ledger)
		if code == 0 || rerr == nil || !strings.Contains(rerr.Error(), "heal anchor failed") {
			t.Fatalf("a failed lag publication must refuse with the cause: code=%d err=%v", code, rerr)
		}
		_, afterHash, _ := VerifyStateShape(state)
		if afterHash != beforeHash {
			t.Fatal("the refusal must not write")
		}
		gitCmd(t, repo, "checkout", "-q", "feature-x")
		if code, rerr := Reconcile(state, repo, ledger); rerr != nil || code != 0 {
			t.Fatalf("the repaired branch must heal: code=%d err=%v", code, rerr)
		}
	})
}

func TestVerifyAnchorDetectsLedgerTamper(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	// Tamper the working-tree ledger after anchoring.
	writeText(t, ledger, oneCycleLedger+"\ntampered\n")
	if _, _, err := VerifyStateWithAnchor(state, repo, ledger); err == nil ||
		!strings.Contains(err.Error(), "Mission-Ledger-SHA256") {
		t.Fatalf("a tampered ledger must fail anchor verification, got %v", err)
	}
}

func TestReconcileEqualCyclesReturnsZero(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	code, err := Reconcile(state, repo, ledger)
	if err != nil {
		t.Fatalf("reconcile errored: %v", err)
	}
	if code != 0 {
		t.Fatalf("a state that matches its anchor should reconcile to 0, got %d", code)
	}
}

// stagnationParkedMission extends anchoredMission: the mission is parked for
// stop-loss with a stagnation ask on disk, and that park is anchored — the
// exact position a crash during the reset transaction leaves behind.
func stagnationParkedMission(t *testing.T) (repo, state, ledger, asksDir string) {
	t.Helper()
	repo, state, ledger = anchoredMission(t)
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["status"] = "parked"
	doc["parkReason"] = "stop-loss"
	doc["waitingList"] = []any{"stop-loss"}
	src := state + ".src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, src, hash); err != nil {
		t.Fatal(err)
	}
	if err := Anchor(state, repo, ledger); err != nil {
		t.Fatal(err)
	}
	asksDir = filepath.Join(filepath.Dir(state), "asks")
	writeAsk(t, asksDir, "stop-loss", StopLossKindStagnation)
	return repo, state, ledger, asksDir
}

func writeAsk(t *testing.T, asksDir, askID, kind string) {
	t.Helper()
	ask := map[string]any{
		"askId": askID, "streamId": "alpha", "reasonClass": "stop-loss",
		"question": "q", "createdAt": "2026-08-11T00:00:00Z", "answeredAt": nil, "answer": nil,
	}
	if kind != "" {
		ask["stopLossKind"] = kind
	}
	if err := atomicWriteJSON(filepath.Join(asksDir, askID+".json"), ask); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileForgivesExactlyTheResetSuffix(t *testing.T) {
	repo, state, ledger, _ := stagnationParkedMission(t)
	if err := AppendReset(ledger, "stop-loss", "human funded the tail"); err != nil {
		t.Fatal(err)
	}
	code, err := Reconcile(state, repo, ledger)
	if err != nil || code != 0 {
		t.Fatalf("a trailing reset suffix must be forgiven: code=%d err=%v", code, err)
	}
	doc, _ := readStateDoc(state)
	if doc["status"] != "parked" || doc["parkReason"] != "stop-loss" {
		t.Fatalf("forgiving must not rewrite the state: %v %v", doc["status"], doc["parkReason"])
	}
	// A duplicate reset line from a re-answered crash is equally lawful.
	if err := AppendReset(ledger, "stop-loss", "answered again after a crash"); err != nil {
		t.Fatal(err)
	}
	if code, err := Reconcile(state, repo, ledger); err != nil || code != 0 {
		t.Fatalf("a double reset suffix must be forgiven: code=%d err=%v", code, err)
	}
}

func TestReconcileStillParksOnAnyOtherDivergence(t *testing.T) {
	cases := []struct {
		name    string
		distort func(t *testing.T, repo, state, ledger, asksDir string)
	}{
		{"non-reset junk in the suffix", func(t *testing.T, repo, state, ledger, asksDir string) {
			if err := AppendReset(ledger, "stop-loss", "fine"); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(ledger)
			writeText(t, ledger, string(data)+"tampered\n")
		}},
		{"reset naming an ask that is not on disk", func(t *testing.T, repo, state, ledger, asksDir string) {
			if err := AppendReset(ledger, "ghost-ask", "fine"); err != nil {
				t.Fatal(err)
			}
		}},
		{"reset naming a cycle-budget ask", func(t *testing.T, repo, state, ledger, asksDir string) {
			writeAsk(t, asksDir, "spent", StopLossKindCycleBudget)
			if err := AppendReset(ledger, "spent", "fine"); err != nil {
				t.Fatal(err)
			}
		}},
		{"reset naming an ask without a stop-loss kind", func(t *testing.T, repo, state, ledger, asksDir string) {
			writeAsk(t, asksDir, "plain", "")
			if err := AppendReset(ledger, "plain", "fine"); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, state, ledger, asksDir := stagnationParkedMission(t)
			tc.distort(t, repo, state, ledger, asksDir)
			code, err := Reconcile(state, repo, ledger)
			if err != nil {
				t.Fatalf("reconcile errored: %v", err)
			}
			if code != 3 {
				t.Fatalf("divergence outside the predicate must park (3), got %d", code)
			}
			doc, _ := readStateDoc(state)
			if r, _ := doc["parkReason"].(string); r != "state-integrity" {
				t.Fatalf("expected a state-integrity park, got %q", r)
			}
		})
	}
}

func TestReconcileResetSuffixNeedsTheStagnationPark(t *testing.T) {
	// The same reset suffix on a mission that is NOT stagnation-parked is
	// divergence, not replayable state.
	repo, state, ledger := anchoredMission(t)
	asksDir := filepath.Join(filepath.Dir(state), "asks")
	writeAsk(t, asksDir, "stop-loss", StopLossKindStagnation)
	if err := AppendReset(ledger, "stop-loss", "fine"); err != nil {
		t.Fatal(err)
	}
	code, err := Reconcile(state, repo, ledger)
	if err != nil {
		t.Fatalf("reconcile errored: %v", err)
	}
	if code != 3 {
		t.Fatalf("a running state with a reset suffix must park (3), got %d", code)
	}
}

func TestReconcileParksOnLedgerBehindState(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	// Truncate the ledger so its cycle count (0) is behind the state (1).
	writeText(t, ledger, "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n")
	code, err := Reconcile(state, repo, ledger)
	if err != nil {
		t.Fatalf("reconcile errored: %v", err)
	}
	if code != 3 {
		t.Fatalf("a ledger behind the state must park (3), got %d", code)
	}
	doc, _ := readStateDoc(state)
	if r, _ := doc["parkReason"].(string); r != "state-integrity" {
		t.Fatalf("the mission should be parked for state-integrity, got %q", r)
	}
}

// advanceStateOneStep performs one legal compare-and-write without
// anchoring — the exact heal-crash lag shape.
func advanceStateOneStep(t *testing.T, state string) {
	t.Helper()
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["fences"].(map[string]any)["jobs"] = 1
	src := state + ".lag.src"
	if err := atomicWriteJSON(src, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, src, hash); err != nil {
		t.Fatal(err)
	}
}

// The heal-crash lag (slice-5 round-6/7): an anchor exactly one state
// step behind, with identical ledger truth, re-anchors and reconciles.
func TestReconcileHealsAnchorLag(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	advanceStateOneStep(t, state)
	code, err := Reconcile(state, repo, ledger)
	if code != 0 || err != nil {
		t.Fatalf("a one-step anchor lag must heal: %d %v", code, err)
	}
	// The heal re-anchored: verification now passes outright.
	if _, _, err := VerifyStateWithAnchor(state, repo, ledger); err != nil {
		t.Fatalf("the healed state must verify: %v", err)
	}
}

// A forged child commit on the anchor ref — matching trailers, no ledger
// blob — must PARK, never launder the evidence (round-7).
func TestReconcileParksForgedAnchorTip(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	advanceStateOneStep(t, state)
	doc, _ := readStateDoc(state)
	integrity, _ := doc["integrity"].(map[string]any)
	previous, _ := integrity["previousHash"].(string)
	lHash, err := ledgerHash(ledger)
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/metasystem/missions/demo/state-anchors"
	tip := strings.TrimSpace(gitCmd(t, repo, "rev-parse", ref))
	tree := strings.TrimSpace(gitCmd(t, repo, "rev-parse", tip+"^{tree}"))
	message := "mission(demo): forged anchor\n\nMission-Id: demo\nMission-State-Hash: " + previous +
		"\nMission-Ledger-SHA256: " + lHash + "\nMission-Ledger-Path: forged.md\nMission-Cycle: 1"
	forged := strings.TrimSpace(gitCmd(t, repo, "commit-tree", tree, "-p", tip, "-m", message))
	gitCmd(t, repo, "update-ref", ref, forged)

	code, _ := Reconcile(state, repo, ledger)
	if code != 3 {
		t.Fatalf("a forged anchor tip must park, got code %d", code)
	}
	doc, _ = readStateDoc(state)
	if doc["parkReason"] != "state-integrity" {
		t.Fatalf("the forgery must park state-integrity: %v", doc["parkReason"])
	}
}

// The cycle-authentication arms of the recovery predicate (successor
// round-18): a complete, blob-carrying tip whose Mission-Cycle is WRONG
// for its own blob, and a tip lawfully ONE CYCLE BEHIND a state whose
// ledger then moved — neither is prescribed as repair; both PARK.
func TestRecoveryPredicateCycleArms(t *testing.T) {
	t.Run("wrong-cycle-trailer", func(t *testing.T) {
		repo, state, ledger := anchoredMission(t)
		// The state LAWFULLY reaches cycle 2 (ledger append + state
		// write) so the trailer-versus-state check below is SATISFIED —
		// only the blob-count gate stands between a lying trailer and
		// the repair prescription (round-19: the earlier construction
		// let the trailer-versus-state check mask this arm).
		sha := strings.Repeat("e", 40)
		if err := AppendCycle(ledger, 2, "no-progress", sha, "score=0", ""); err != nil {
			t.Fatal(err)
		}
		doc, _ := readStateDoc(state)
		doc["ledger"].(map[string]any)["cycles"] = 2
		doc["fences"].(map[string]any)["cycles"] = 2
		source := state + ".src"
		if err := atomicWriteJSON(source, doc); err != nil {
			t.Fatal(err)
		}
		_, hash, _ := VerifyStateShape(state)
		if err := WriteState(state, source, hash); err != nil {
			t.Fatal(err)
		}
		doc, _ = readStateDoc(state)
		integrity, _ := doc["integrity"].(map[string]any)
		previous, _ := integrity["previousHash"].(string)
		ref := "refs/metasystem/missions/demo/state-anchors"
		tip := strings.TrimSpace(gitCmd(t, repo, "rev-parse", ref))
		tree := strings.TrimSpace(gitCmd(t, repo, "rev-parse", tip+"^{tree}"))
		blob := gitCmd(t, repo, "show", tip+":ledger.md")
		blobSHA := sha256Hex(blob)
		// The tip's tree still carries the ONE-cycle blob; the trailer
		// claims cycle 2 and matches the state. Only blob-count refuses.
		message := "mission(demo): forged cycle\n\nMission-Id: demo\nMission-State-Hash: " + previous +
			"\nMission-Ledger-SHA256: " + blobSHA + "\nMission-Ledger-Path: ledger.md\nMission-Cycle: 2"
		forged := strings.TrimSpace(gitCmd(t, repo, "commit-tree", tree, "-p", tip, "-m", message))
		gitCmd(t, repo, "update-ref", ref, forged)
		// The live bytes moved past the trailer's sha.
		data, _ := os.ReadFile(ledger)
		writeText(t, ledger, string(data)+"\n")
		code, _ := Reconcile(state, repo, ledger)
		if code != 3 {
			t.Fatalf("a lying-cycle tip must park, got %d", code)
		}
		doc, _ = readStateDoc(state)
		if doc["parkReason"] != "state-integrity" {
			t.Fatalf("the lying-cycle tip must park, not prescribe repair: %v", doc["parkReason"])
		}
	})
	t.Run("one-cycle-behind-heals-then-rejects-movement", func(t *testing.T) {
		repo, state, ledger := anchoredMission(t)
		// POSITIVE ARM (round-19): anchor at cycle 1, state and the
		// exactly-stamped ledger at cycle 2 — the lawful one-block lag
		// must HEAL. Rejecting lawful cycle lag fails here.
		sha := strings.Repeat("e", 40)
		if err := AppendCycle(ledger, 2, "no-progress", sha, "score=0", ""); err != nil {
			t.Fatal(err)
		}
		doc, _ := readStateDoc(state)
		doc["ledger"].(map[string]any)["cycles"] = 2
		doc["fences"].(map[string]any)["cycles"] = 2
		source := state + ".src"
		if err := atomicWriteJSON(source, doc); err != nil {
			t.Fatal(err)
		}
		_, hash, _ := VerifyStateShape(state)
		if err := WriteState(state, source, hash); err != nil {
			t.Fatal(err)
		}
		if code, rerr := Reconcile(state, repo, ledger); rerr != nil || code != 0 {
			t.Fatalf("the lawful one-block lag must heal: code=%d err=%v", code, rerr)
		}
		if _, _, err := VerifyStateWithAnchor(state, repo, ledger); err != nil {
			t.Fatalf("the healed position must verify: %v", err)
		}
		// NEGATIVE ARM: bytes moved past the now-anchored truth park on
		// the next divergence beyond the one-step shape... a single
		// step with movement stays the recoverable refusal.
		data, _ := os.ReadFile(ledger)
		writeText(t, ledger, string(data)+"\n")
		advanceStateOneStep(t, state)
		code, rerr := Reconcile(state, repo, ledger)
		if code == 0 {
			t.Fatal("moved bytes past the anchored truth must not reconcile clean")
		}
		if rerr == nil || !strings.Contains(rerr.Error(), "restore the ledger bytes") {
			t.Fatalf("the one-step movement is the recoverable refusal: %v", rerr)
		}
	})
}

// Every malformed-tip shape over an OLDER VALID anchor parks (round-8):
// these pin tip-only lookup (no scan-past) and each required trailer
// independently — the parent commit below the forged tip is a complete,
// valid anchor that scan-past behavior would wrongly accept.
func TestReconcileParksMalformedAnchorTips(t *testing.T) {
	cases := []struct {
		name    string
		mission string
		drop    string
	}{
		{"wrong mission id", "other", ""},
		{"missing state hash", "demo", "Mission-State-Hash"},
		{"missing ledger sha", "demo", "Mission-Ledger-SHA256"},
		{"missing ledger path", "demo", "Mission-Ledger-Path"},
		{"missing cycle", "demo", "Mission-Cycle"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, state, ledger := anchoredMission(t)
			advanceStateOneStep(t, state)
			doc, _ := readStateDoc(state)
			integrity, _ := doc["integrity"].(map[string]any)
			previous, _ := integrity["previousHash"].(string)
			lHash, err := ledgerHash(ledger)
			if err != nil {
				t.Fatal(err)
			}
			trailers := map[string]string{
				"Mission-Id": tc.mission, "Mission-State-Hash": previous,
				"Mission-Ledger-SHA256": lHash, "Mission-Ledger-Path": "ledger.md",
				"Mission-Cycle": "1",
			}
			delete(trailers, tc.drop)
			message := "mission(demo): forged anchor\n"
			for _, key := range []string{"Mission-Id", "Mission-State-Hash", "Mission-Ledger-SHA256", "Mission-Ledger-Path", "Mission-Cycle"} {
				if value, ok := trailers[key]; ok {
					message += "\n" + key + ": " + value
				}
			}
			ref := "refs/metasystem/missions/demo/state-anchors"
			tip := strings.TrimSpace(gitCmd(t, repo, "rev-parse", ref))
			tree := strings.TrimSpace(gitCmd(t, repo, "rev-parse", tip+"^{tree}"))
			forged := strings.TrimSpace(gitCmd(t, repo, "commit-tree", tree, "-p", tip, "-m", message))
			gitCmd(t, repo, "update-ref", ref, forged)

			code, _ := Reconcile(state, repo, ledger)
			if code != 3 {
				t.Fatalf("a malformed tip must park, got code %d", code)
			}
			after, _ := readStateDoc(state)
			if after["parkReason"] != "state-integrity" {
				t.Fatalf("park reason: %v", after["parkReason"])
			}
		})
	}
}

// A tip whose trailers are COMPLETE and correct but whose tree lacks the
// declared blob parks — the committed-blob check exercised alone
// (round-8: the earlier forged-tip bed died at the path mismatch first).
func TestReconcileParksAnchorTipWithoutBlob(t *testing.T) {
	repo, state, ledger := anchoredMission(t)
	advanceStateOneStep(t, state)
	doc, _ := readStateDoc(state)
	integrity, _ := doc["integrity"].(map[string]any)
	previous, _ := integrity["previousHash"].(string)
	lHash, err := ledgerHash(ledger)
	if err != nil {
		t.Fatal(err)
	}
	message := "mission(demo): forged anchor\n\nMission-Id: demo\nMission-State-Hash: " + previous +
		"\nMission-Ledger-SHA256: " + lHash + "\nMission-Ledger-Path: ledger.md\nMission-Cycle: 1"
	ref := "refs/metasystem/missions/demo/state-anchors"
	tip := strings.TrimSpace(gitCmd(t, repo, "rev-parse", ref))
	// The well-known empty tree: correct path declared, no blob present.
	forged := strings.TrimSpace(gitCmd(t, repo, "commit-tree",
		"4b825dc642cb6eb9a060e54bf8d69288fbee4904", "-p", tip, "-m", message))
	gitCmd(t, repo, "update-ref", ref, forged)

	code, _ := Reconcile(state, repo, ledger)
	if code != 3 {
		t.Fatalf("a blob-less anchor tip must park, got code %d", code)
	}
	after, _ := readStateDoc(state)
	if after["parkReason"] != "state-integrity" {
		t.Fatalf("park reason: %v", after["parkReason"])
	}
}
