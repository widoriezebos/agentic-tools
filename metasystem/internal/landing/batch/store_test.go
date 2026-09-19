package batch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

const testBatchID = "01j5x00000000000000000ba01"

type scriptedProber map[int64]identity.Liveness

func (p scriptedProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, p[pid], nil
}
func must(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
func load(t *testing.T, s Store) Record      { r, e := s.Load(testBatchID); must(t, e); return r }
func contents(t *testing.T, p string) []byte { b, e := os.ReadFile(p); must(t, e); return b }
func polls(t *testing.T, owner *proofLock, wants ...lockPoll) {
	for _, want := range wants {
		if got, err := owner.poll(); err != nil || got != want {
			t.Fatalf("poll=%v error=%v, want %v", got, err, want)
		}
	}
}
func seedProofLock(t *testing.T, dir, pid string) {
	must(t, os.Mkdir(dir, 0o755))
	must(t, os.WriteFile(filepath.Join(dir, "owner"), []byte("m1e "+pid+" 2029-01-01T00:00:00Z hand\n"), 0o644))
}
func testProofLock(prober scriptedProber, lockDir, queueDir string, pid int64, now *time.Time) *proofLock {
	return newProofLock(NewStore("", prober), lockDir, queueDir, "batch:test", pid, func() time.Time { return *now })
}
func TestBatchLockIsFifoAndStaleSafe(t *testing.T) {
	root, now := t.TempDir(), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	lockDir, queueDir := filepath.Join(root, "lock"), filepath.Join(root, "queue")
	prober := scriptedProber{1: identity.Alive, 2: identity.Alive, 3: identity.Dead, 4: identity.Unknown, 9: identity.Alive, 90: identity.Alive}
	seedProofLock(t, lockDir, "90")
	first := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, first, lockQueued)
	now = now.Add(time.Second)
	second := testProofLock(prober, lockDir, queueDir, 2, &now)
	polls(t, second, lockQueued)
	must(t, os.RemoveAll(lockDir))
	polls(t, second, lockQueued)
	polls(t, first, lockAcquired)
	if owner := string(contents(t, filepath.Join(lockDir, "owner"))); !strings.HasPrefix(owner, "landing-batch-owner 1 ") {
		t.Fatalf("owner=%q", owner)
	}
	_ = first.whileHeld(func() error { return os.ErrInvalid })
	polls(t, second, lockAcquired)
	must(t, os.WriteFile(filepath.Join(lockDir, "owner"), []byte("m1e 90 2030-01-01T00:00:01Z successor\n"), 0o644))
	must(t, second.release())
	_ = contents(t, filepath.Join(lockDir, "owner"))
	must(t, os.RemoveAll(lockDir))

	now = now.Add(time.Second)
	hand := filepath.Join(queueDir, "1893456001-m1e-9")
	must(t, os.WriteFile(hand, []byte("m1e 9 2030-01-01T00:00:01Z hand proof\n"), 0o644))
	third := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, third, lockQueued)
	prober[9] = identity.Dead
	polls(t, third, lockStaleRemoved, lockAcquired)
	must(t, third.release())
	seedProofLock(t, lockDir, "3")
	fourth := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, fourth, lockStaleRemoved, lockAcquired)
	must(t, fourth.release())
	must(t, os.Mkdir(lockDir, 0o755))
	young := now.Add(-time.Minute)
	must(t, os.Chtimes(lockDir, young, young))
	early := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, early, lockQueued)
	must(t, early.release())
	must(t, os.WriteFile(filepath.Join(lockDir, "owner"), nil, 0o644))
	old := now.Add(-2*time.Minute - time.Second)
	must(t, os.Chtimes(lockDir, old, old))
	fifth := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, fifth, lockStaleRemoved, lockAcquired)
	must(t, fifth.release())
	seedProofLock(t, lockDir, "4")
	sixth := testProofLock(prober, lockDir, queueDir, 1, &now)
	polls(t, sixth, lockQueued)
	must(t, sixth.release())
	if entries, err := os.ReadDir(queueDir); err != nil || len(entries) != 0 {
		t.Fatalf("give up left queue entries: %v, %v", entries, err)
	}
}
func TestBatchStartRule(t *testing.T) {
	seat, landing := t.TempDir(), t.TempDir()
	must(t, exec.Command("git", "init", "-q", landing).Run())
	conf := filepath.Join(seat, "metasystem.conf")
	must(t, os.WriteFile(conf, []byte(config.BatchRootKey+"="+landing+"\n"+config.BatchMaxWaitKey+"=2m\n"), 0o644))
	joined, now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Time{}
	settings, err := config.ResolveBatchLanding(conf, seat, func() time.Time { return now })
	must(t, err)
	quiet := proofrun.LoadSample{Sample: hostload.Sample{Available: true, Cores: 8}, OverlapKnown: true}
	loaded := proofrun.LoadSample{Sample: hostload.Sample{Available: true, Cores: 2, Load1m: 2}, OverlapKnown: true, OverlappingHost: 2}
	check := func(name string, free bool, units, cap int, elapsed time.Duration, sample proofrun.LoadSample, start bool, window string) {
		t.Run(name, func(t *testing.T) {
			now = joined.Add(elapsed)
			startGot, windowGot, capGot := batchStartRule(free, units, joined, settings, sample, proofrun.AdmissionCap{Max: cap})
			if startGot != start || windowGot != window || capGot != (sample.OverlapKnown && sample.OverlappingHost < cap) {
				t.Fatalf("decision=%v/%s cap=%v", startGot, windowGot, capGot)
			}
		})
	}
	check("no units", true, 0, 3, 2*time.Minute, quiet, false, "")
	check("lock held", false, 2, 3, 0, quiet, false, "")
	check("one before wait", true, 1, 3, 0, quiet, false, "")
	check("at wait", true, 1, 3, 2*time.Minute, quiet, true, "expired")
	check("two quiet", true, 2, 3, 0, quiet, true, "quiet")
	check("unknown census", true, 2, 3, 0, proofrun.LoadSample{}, false, "")
	check("overlap is not quiet", true, 2, 3, 0, loaded, false, "")
	check("wait overrides load", true, 2, 3, 2*time.Minute, loaded, true, "expired")
	check("engine cap", true, 2, 2, 2*time.Minute, loaded, false, "")
}
func TestBatchJoinsSerializeUnderTheLock(t *testing.T) {
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: "base", State: StateOpen}))
	held, contended := make(chan struct{}, 1), make(chan struct{})
	store.seams.flock = func(_ int, operation int) error {
		if operation == unix.LOCK_UN {
			<-held
			return nil
		}
		select {
		case held <- struct{}{}:
		default:
			close(contended)
			held <- struct{}{}
		}
		return nil
	}
	first, release, second := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 2)
	update := func(change func(*Record) error) { done <- store.Update(testBatchID, change) }
	go update(func(record *Record) error { record.TipTree += "+goal-1"; close(first); <-release; return nil })
	select {
	case <-first:
	case err := <-done:
		t.Fatalf("first batch update returned before entering its mutation: %v", err)
	}
	go update(func(record *Record) error { record.TipTree += "+goal-2"; close(second); return nil })
	select {
	case <-contended:
	case <-second:
		t.Error("second join entered the batch mutation while the first held it")
	case err := <-done:
		t.Fatalf("second batch update returned before contending for the flock: %v", err)
	}
	close(release)
	must(t, <-done)
	must(t, <-done)
	if <-second; load(t, store).TipTree != "base+goal-1+goal-2" {
		t.Fatalf("serialized tip=%q", load(t, store).TipTree)
	}
}
func TestBatchHistoryIsAppendOnly(t *testing.T) {
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{{GoalID: "g1", Chain: "j1", Claim: Claim{Machine: "m1", Lineage: "l1", Epoch: 1, Revision: 1, AccountingRevision: 1}, State: UnitJoined}}}))
	states := []string{StateSealed, StateProving, StateDiagnosing, StateProving, StateLanding, StateLanded, StateOpen, StateDissolved}
	for index, state := range states {
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Transition(state, time.Unix(int64(index), 0), "tick", "m1l+landing-m1l", "")
			return nil
		}))
	}
	if got := len(load(t, store).History); got != len(states) {
		t.Fatalf("history has %d entries, want %d", got, len(states))
	}
	path, _ := store.recordPath(testBatchID)
	before := contents(t, path)
	for name, mutate := range map[string]func(*Record){
		"truncate history": func(r *Record) { r.History = r.History[:1] },
		"edit history":     func(r *Record) { r.History[0].Verb = "rewrite" },
		"edit revision":    func(r *Record) { r.Units[0].Claim.Revision++ },
		"remove unit":      func(r *Record) { r.Units = nil },
	} {
		if err := store.Update(testBatchID, func(r *Record) error { mutate(r); return nil }); err == nil {
			t.Fatalf("%s mutation was accepted", name)
		}
		if string(contents(t, path)) != string(before) {
			t.Fatalf("%s refusal changed the record", name)
		}
	}
}

func TestBatchRecordSchemaOneFixtureStillLoads(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "landing-batches", testBatchID+".json")
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	raw := `{
  "schema": 1,
  "batchId": "01j5x00000000000000000ba01",
  "tipTree": "chain-tree",
  "state": "open",
  "units": [
    {
      "goalId": "goal-a",
      "chain": "implementation-chain-a",
      "claim": {
        "machine": "seat-a",
        "lineage": "lineage-a",
        "epoch": 1,
        "revision": 2,
        "accountingRevision": 3
      },
      "state": "joined"
    }
  ],
  "history": []
}
`
	must(t, os.WriteFile(path, []byte(raw), 0o644))
	record, err := NewStore(root, nil).Load(testBatchID)
	must(t, err)
	if len(record.Units) != 1 || record.Units[0].Chain != "implementation-chain-a" || len(record.Units[0].CommitIDs) != 0 ||
		record.Units[0].GoalLast || record.Units[0].BranchTip != "" || len(record.Units[0].Builds) != 0 {
		t.Fatalf("legacy chain record=%+v", record)
	}
}
func TestBatchLivenessUsesInjectedProber(t *testing.T) {
	prober := scriptedProber{101: identity.Alive, 102: identity.Dead, 103: identity.Unknown}
	for pid, want := range prober {
		if got := NewStore("", prober).Liveness(identity.Ref{Pid: pid, StartedAtSec: pid}); got != want {
			t.Fatalf("pid %d liveness=%s, want %s", pid, got, want)
		}
	}
}
func writeChainDoc(t *testing.T, path string, value any) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Dir(path), 0o755))
	data, err := json.Marshal(value)
	must(t, err)
	must(t, os.WriteFile(path, data, 0o644))
}
func chainSubject(c map[string]any) map[string]any {
	return c["closure"].(map[string]any)["subject"].(map[string]any)
}
func chainFixture(t *testing.T, change func(map[string]any, map[string]any)) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	patch := []byte("diff --git a/a.go b/a.go\n")
	digest := sha256.Sum256(patch)
	implementation := map[string]any{"jobId": "implementation", "parentJob": nil, "role": "implementer", "status": "completed", "round": 1, "chainClosed": true, "goalId": "goal-a", "goalRevision": 2, "independentCritiqueJobRef": "critic"}
	critic := map[string]any{"jobId": "critic", "parentJob": nil, "role": "code-critic", "status": "completed", "round": 1, "chainClosed": true, "reviews": "implementation", "closure": map[string]any{"criticRoot": "critic", "round": 1, "mechanism": "clean", "subject": map[string]any{"kind": "live", "implementerRoot": "implementation", "reviewedMember": "implementation", "reviewedProjectTree": strings.Repeat("a", 40), "diffDigest": hex.EncodeToString(digest[:])}}}
	mirror := filepath.Join(root, "mirror")
	implementation["mirror"] = map[string]any{"path": mirror}
	critic["mirror"] = map[string]any{"path": mirror}
	if change != nil {
		change(implementation, critic)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	writeChainDoc(t, filepath.Join(jobs, "implementation.json"), implementation)
	writeChainDoc(t, filepath.Join(jobs, "critic.json"), critic)
	writeChainDoc(t, filepath.Join(jobs, "outsider.json"), map[string]any{"jobId": "outsider", "parentJob": nil, "role": "implementer", "status": "completed", "round": 1})
	round := filepath.Join(root, "artifacts", "agents", "implementation", "rounds", "1")
	must(t, os.MkdirAll(round, 0o755))
	must(t, os.WriteFile(filepath.Join(round, "diff.patch"), patch, 0o644))
	writeChainDoc(t, filepath.Join(mirror, "manifest.json"), map[string]any{"files": map[string]any{"jobs/implementation.json": map[string]any{}, "jobs/critic.json": map[string]any{}, "rounds/1/diff.patch": map[string]any{"sha256": hex.EncodeToString(digest[:])}}})
	return root, patch
}
func TestBatchJoinRefusesOpenChain(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(map[string]any, map[string]any)
	}{
		{"open", "BATCH_JOIN_CHAIN_UNCLOSED: implementation chain is open", func(i, _ map[string]any) { i["chainClosed"] = false }},
		{"implementation closure invalid", "BATCH_JOIN_CHAIN_UNREAD: implementation chain closure is invalid", func(i, _ map[string]any) { i["status"] = "running" }},
		{"wrong implementation role", "BATCH_JOIN_CHAIN_NOT_IMPLEMENTATION: chain root is not an implementer job", func(i, _ map[string]any) { i["role"] = "observer" }},
		{"wrong goal", "BATCH_JOIN_CHAIN_UNREAD: implementation chain does not bind the named goal", func(i, _ map[string]any) { i["goalId"] = "goal-b" }},
		{"revision moved", "BATCH_JOIN_CHAIN_REVISION_MOVED: chain goal revision is 1, claim revision is 2", func(i, _ map[string]any) { i["goalRevision"] = 1 }},
		{"critic missing", "BATCH_JOIN_CHAIN_UNREAD: closed code-critic root is unreadable", func(i, _ map[string]any) { delete(i, "independentCritiqueJobRef") }},
		{"wrong critic role", "BATCH_JOIN_CHAIN_UNREAD: critic chain root is not a code-critic job", func(_, c map[string]any) { c["role"] = "observer" }},
		{"critic closure invalid", "BATCH_JOIN_CHAIN_UNREAD: code-critic chain closure is invalid", func(_, c map[string]any) { c["status"] = "running" }},
		{"non-live subject", "BATCH_JOIN_CHAIN_UNREAD: code-critic closure subject is not live", func(_, c map[string]any) { chainSubject(c)["kind"] = "commit" }},
		{"other implementation", "BATCH_JOIN_CHAIN_UNREAD: code-critic closure certifies another implementation chain", func(_, c map[string]any) { chainSubject(c)["implementerRoot"] = "outsider" }},
		{"member outside chain", "BATCH_JOIN_CHAIN_UNREAD: certified implementer is outside implementation chain", func(_, c map[string]any) { chainSubject(c)["reviewedMember"] = "outsider" }},
		{"digest changed", "BATCH_JOIN_CHAIN_UNREAD: certified diff is unreadable or changed", func(_, c map[string]any) { chainSubject(c)["diffDigest"] = strings.Repeat("b", 64) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root, _ := chainFixture(t, test.change)
			if _, err := readCertifiedChain(root, "goal-a", "implementation", 2); err == nil || err.Error() != test.want {
				t.Fatalf("refusal=%v, want %s", err, test.want)
			}
		})
	}
}
func TestBatchJoinDerivesDiffFromChain(t *testing.T) {
	root, patch := chainFixture(t, nil)
	chain, err := readCertifiedChain(root, "goal-a", "implementation", 2)
	must(t, err)
	digest := sha256.Sum256(patch)
	if string(chain.Patch) != string(patch) || chain.Digest != hex.EncodeToString(digest[:]) {
		t.Fatalf("certified output=%q digest=%s", chain.Patch, chain.Digest)
	}
}
func TestBatchJoinRequiresJobDomainChain(t *testing.T) {
	root := t.TempDir()
	_, err := readCertifiedChain(root, "goal-a", "codex-rescue", 2)
	if err == nil || !strings.HasPrefix(err.Error(), "BATCH_JOIN_CHAIN_REQUIRED:") || !strings.Contains(err.Error(), "delegate --role implementer") {
		t.Fatalf("job-domain refusal=%v", err)
	}
}
func TestBatchChainTransportIsByteExact(t *testing.T) {
	root, patch := chainFixture(t, nil)
	chain, err := readCertifiedChain(root, "goal-a", "implementation", 2)
	must(t, err)
	landing := t.TempDir()
	must(t, transportChain(landing, chain))
	target := filepath.Join(landing, "artifacts", "agents", "landing-batches", "chains", chain.ID, "diff.patch")
	if string(contents(t, target)) != string(patch) {
		t.Fatal("transport changed certified bytes")
	}
	must(t, os.WriteFile(target, []byte("different"), 0o644))
	if err := transportChain(landing, chain); err == nil || !strings.HasPrefix(err.Error(), "BATCH_CHAIN_CONFLICT:") || string(contents(t, target)) != "different" {
		t.Fatalf("conflict=%v target=%q", err, contents(t, target))
	}
}
func TestBatchUnitJoinsOneBatch(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, scriptedProber{})
	member := func(goal, chain string) Unit {
		return Unit{GoalID: goal, Chain: chain, Claim: Claim{Machine: "m", Lineage: "l", Epoch: 1, Revision: 1, AccountingRevision: 1}, State: UnitJoined}
	}
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{member("goal-a", "implementation")}}))
	other := "01j5x00000000000000000ba02"
	must(t, store.Create(Record{Schema: 1, BatchID: other, State: StateOpen}))
	if err := checkMembership(store, other, "goal-b", "implementation"); err == nil || !strings.HasPrefix(err.Error(), "BATCH_UNIT_ELSEWHERE:") {
		t.Fatalf("chain refusal=%v", err)
	}
	if err := checkMembership(store, other, "goal-a", "implementation-2"); err == nil || !strings.HasPrefix(err.Error(), "BATCH_GOAL_ELSEWHERE:") {
		t.Fatalf("goal refusal=%v", err)
	}
	if err := checkMembership(store, testBatchID, "goal-a", "implementation-2"); err == nil || !strings.HasPrefix(err.Error(), "BATCH_GOAL_ELSEWHERE:") {
		t.Fatalf("same-batch goal refusal=%v", err)
	}
}
func joinBed(t *testing.T) (assemblyBed, Store) {
	bed := assemblyFixture(t)
	store := NewStore(bed.root, scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, BaseTree: bed.base, TipTree: bed.base, State: StateOpen}))
	return bed, store
}
func joiningUnit(goalID, chain string) Unit {
	return Unit{GoalID: goalID, Chain: chain, Claim: Claim{Machine: "seat", Lineage: goalID, Epoch: 1, Revision: 2, AccountingRevision: 1}, Gate: runIDs{"gate-1"}}
}

func joinPlanMode(mode testpolicy.Mode) func(string, string, string) (testpolicy.Plan, error) {
	return func(string, string, string) (testpolicy.Plan, error) { return testpolicy.Plan{RequiredMode: mode}, nil }
}

func TestBatchJoinSelectsGroupsFromEachUnitsOwnTree(t *testing.T) {
	bed, store := joinBed(t)
	plan := func(_, _ string, tree string) (testpolicy.Plan, error) {
		selected := []string{"shared"}
		if strings.Contains(bedGit(t, bed.root, "show", tree+":a.go"), "A = 1") {
			selected = append(selected, "group-a")
		}
		if strings.Contains(bedGit(t, bed.root, "show", tree+":b.go"), "B = 1") {
			selected = append(selected, "group-b")
		}
		return testpolicy.Plan{SelectedGroups: selected}, nil
	}
	must(t, PublishJoin(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), plan, func() error { return nil }))
	must(t, PublishJoin(store, testBatchID, joiningUnit("goal-b", "chain-b"), "seat+goal-b", time.Unix(2, 0), plan, func() error { return nil }))
	record := load(t, store)
	if !slices.Equal(record.Units[0].SelectedGroups, []string{"shared", "group-a"}) ||
		!slices.Equal(record.Units[1].SelectedGroups, []string{"shared", "group-b"}) {
		t.Fatalf("per-unit selections=%v / %v", record.Units[0].SelectedGroups, record.Units[1].SelectedGroups)
	}
	named := namedDiagnosticUnits(record.Units, []RedGroup{{ID: "group-a"}})
	if !named["goal-a"] || named["goal-b"] {
		t.Fatalf("group-a named units=%v", named)
	}
}

func TestBatchJoinPlanningPinsMergeBaseToBatchBase(t *testing.T) {
	bed, store := joinBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.BaseTree, record.TipTree = bed.moved, bed.moved
		return nil
	}))
	must(t, os.WriteFile(filepath.Join(bed.root, "newer-trunk"), []byte("newer\n"), 0o644))
	bedGit(t, bed.root, "add", "newer-trunk")
	bedGit(t, bed.root, "commit", "-qm", "newer trunk")
	plan := func(root, _ string, _ string) (testpolicy.Plan, error) {
		policyRef := bedGit(t, root, "config", "--worktree", "--get", "metasystem.steward.landing-ref")
		mergeBase := bedGit(t, root, "merge-base", "HEAD", policyRef)
		mergeBaseTree, err := (gittree.Workspace{Dir: root}).TreeOf(mergeBase)
		must(t, err)
		if mergeBaseTree != bed.moved {
			t.Fatalf("planning merge-base tree=%s want batch base=%s", mergeBaseTree, bed.moved)
		}
		return testpolicy.Plan{SelectedGroups: []string{"group-a"}}, nil
	}
	must(t, PublishJoin(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), plan, func() error { return nil }))
}

func TestBatchJoinPublicationHoldsTheFlock(t *testing.T) {
	for _, point := range []string{UnitJoining, "handover", UnitJoined} {
		_, store := joinBed(t)
		token, waiting, events := make(chan struct{}, 1), make(chan struct{}, 1), make(chan string, 8)
		token <- struct{}{}
		store.seams.flock = func(_ int, operation int) error {
			if operation == unix.LOCK_UN {
				events <- "release"
				token <- struct{}{}
				return nil
			}
			select {
			case <-token:
				return nil
			default:
				waiting <- struct{}{}
				<-token
				return nil
			}
		}
		reached, proceed := make(chan struct{}), make(chan struct{})
		store.seams.publish = func(got string) error {
			if got == point {
				close(reached)
				<-proceed
			}
			return nil
		}
		joinDone := make(chan error, 1)
		go func() {
			joinDone <- PublishJoin(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), joinPlanMode(testpolicy.ModeStandard), func() error { return nil })
		}()
		select {
		case <-reached:
		case err := <-joinDone:
			t.Fatalf("join returned before reaching publication %q: %v", point, err)
		}
		tickDone := make(chan error, 1)
		go func() {
			tickDone <- ReconcileJoins(store, testBatchID, "unused", "landing+owner", time.Unix(2, 0), nil)
			events <- "tick"
		}()
		select {
		case <-waiting:
		case err := <-tickDone:
			close(proceed)
			<-joinDone
			t.Fatalf("tick finished before waiting for the join flock: %v", err)
		}
		close(proceed)
		must(t, <-joinDone)
		must(t, <-tickDone)
		got := []string{<-events, <-events, <-events}
		witness(t, strings.Join(got, ",") == "release,release,tick" && load(t, store).Units[0].State == UnitJoined, "%s event order=%v", point, got)
	}
}
func TestBatchTickReconcilesAKilledJoinerOnce(t *testing.T) {
	exact := Claim{Machine: "seat", Lineage: "goal-a", Revision: 2, AccountingRevision: 1}
	bad := func(change func(*Claim)) func(string, string, string, string) (Claim, error) {
		return func(string, string, string, string) (Claim, error) { claim := exact; change(&claim); return claim, nil }
	}
	tests := []struct {
		name, point, want string
		read              func(string, string, string, string) (Claim, error)
	}{
		{"before handover", UnitJoining, UnitEjected, func(string, string, string, string) (Claim, error) { return Claim{}, os.ErrNotExist }},
		{"after handover", "handover", UnitJoined, nil},
		{"batch mismatch", "handover", UnitEjected, func(root, tree, _, goalID string) (Claim, error) { return claimAt(root, tree, "other-batch", goalID) }},
		{"source machine mismatch", "handover", UnitEjected, bad(func(c *Claim) { c.Machine = "other" })},
		{"source lineage mismatch", "handover", UnitEjected, bad(func(c *Claim) { c.Lineage = "other" })},
		{"revision mismatch", "handover", UnitEjected, bad(func(c *Claim) { c.Revision++ })},
		{"accounting revision mismatch", "handover", UnitEjected, bad(func(c *Claim) { c.AccountingRevision++ })},
	}
	for _, test := range tests {
		if test.want == UnitEjected {
			test.want = UnitReturnPending
		}
		bed, store := joinBed(t)
		store.seams.publish = func(point string) error {
			if point == test.point {
				return os.ErrProcessDone
			}
			return nil
		}
		err := PublishJoin(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), joinPlanMode(testpolicy.ModeStandard), func() error { return nil })
		witness(t, err == os.ErrProcessDone, "join error=%v", err)
		must(t, ReconcileJoins(store, testBatchID, bed.base, "landing+owner", time.Unix(2, 0), test.read))
		must(t, ReconcileJoins(store, testBatchID, bed.base, "landing+owner", time.Unix(2, 0), test.read))
		record := load(t, store)
		unit := record.Units[0]
		witness(t, unit.State == test.want && len(record.History) == 2 && record.History[1].Verb == "reconcile" && (test.want == UnitReturnPending) == (unit.Failure == "join-incomplete"), "%s unit=%+v history=%+v", test.name, unit, record.History)
	}
}
func TestBatchJoinPrechecksBeforePublication(t *testing.T) {
	_, store := joinBed(t)
	ejected := joiningUnit("old", "absent")
	ejected.State, ejected.Outcome, ejected.Failure = UnitReturnPending, UnitEjected, "old failure"
	must(t, store.Update(testBatchID, func(record *Record) error { record.Units = append(record.Units, ejected); return nil }))
	handovers := 0
	join := func(unit Unit, mode testpolicy.Mode) error {
		return PublishJoin(store, testBatchID, unit, "seat+joiner", time.Unix(1, 0), joinPlanMode(mode), func() error { handovers++; return nil })
	}
	must(t, join(joiningUnit("goal-a", "chain-a"), testpolicy.ModeStandard))
	path, _ := store.recordPath(testBatchID)
	before := string(contents(t, path))
	err := join(joiningUnit("goal-a", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_GOAL_ELSEWHERE") && string(contents(t, path)) == before && handovers == 1, "membership join=%v handovers=%d", err, handovers)
	err = join(joiningUnit("goal-conflict", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_JOIN_CONFLICT") && string(contents(t, path)) == before && handovers == 1, "conflicting join=%v handovers=%d", err, handovers)
	must(t, join(joiningUnit("goal-b", "chain-b"), testpolicy.ModeDeep))
	record := load(t, store)
	witness(t, record.ClosedReason == "deep-ceiling" && record.Units[2].State == UnitJoined && len(record.PrefixTrees) == 2 && record.TipTree == record.PrefixTrees[1] && len(record.History) == 4 && record.History[0].Verb+":"+record.History[0].Detail == "join:goal-a joining" && record.History[1].Verb+":"+record.History[1].Detail == "join:goal-a joined" && record.History[2].Verb+":"+record.History[2].Detail == "join:goal-b joining" && record.History[3].Verb+":"+record.History[3].Detail == "join:goal-b joined", "deep join record=%+v", record)
	before = string(contents(t, path))
	err = join(joiningUnit("goal-c", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_CLOSED") && string(contents(t, path)) == before && handovers == 2, "post-ceiling join=%v handovers=%d", err, handovers)
}
