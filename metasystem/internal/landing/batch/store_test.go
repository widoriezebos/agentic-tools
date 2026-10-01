package batch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
		{"open", "BATCH_JOIN_CHAIN_UNCLOSED: the build of goal goal-a is not reviewed yet; metasystem work review goal-a reviews it", func(i, _ map[string]any) { i["chainClosed"] = false }},
		{"implementation closure invalid", "BATCH_JOIN_CHAIN_UNREAD: the build of goal goal-a did not close cleanly; metasystem work review goal-a reviews it", func(i, _ map[string]any) { i["status"] = "running" }},
		{"wrong implementation role", "BATCH_JOIN_CHAIN_NOT_IMPLEMENTATION: job implementation is not a build; metasystem work status goal-a names the goal's builds", func(i, _ map[string]any) { i["role"] = "observer" }},
		{"wrong goal", "BATCH_JOIN_CHAIN_UNREAD: job implementation is not a build of goal goal-a; metasystem work status goal-a names its builds", func(i, _ map[string]any) { i["goalId"] = "goal-b" }},
		{"revision moved", "BATCH_JOIN_CHAIN_REVISION_MOVED: goal goal-a changed after its build (revision 1, now 2); metasystem work build goal-a builds it again", func(i, _ map[string]any) { i["goalRevision"] = 1 }},
		{"critic missing", "BATCH_JOIN_CHAIN_UNREAD: the review of goal goal-a can't be read; metasystem work review goal-a reviews it again", func(i, _ map[string]any) { delete(i, "independentCritiqueJobRef") }},
		{"wrong critic role", "BATCH_JOIN_CHAIN_UNREAD: the review of goal goal-a is not a code review; metasystem work review goal-a reviews it", func(_, c map[string]any) { c["role"] = "observer" }},
		{"critic closure invalid", "BATCH_JOIN_CHAIN_UNREAD: the review of goal goal-a did not close cleanly; metasystem work review goal-a reviews it again", func(_, c map[string]any) { c["status"] = "running" }},
		{"non-live subject", "BATCH_JOIN_CHAIN_UNREAD: the review of goal goal-a read no live build; metasystem work review goal-a reviews the build", func(_, c map[string]any) { chainSubject(c)["kind"] = "commit" }},
		{"other implementation", "BATCH_JOIN_CHAIN_UNREAD: the review of goal goal-a certifies another build; metasystem work review goal-a reviews this one", func(_, c map[string]any) { chainSubject(c)["implementerRoot"] = "outsider" }},
		{"member outside chain", "BATCH_JOIN_CHAIN_UNREAD: the reviewed round of goal goal-a is not part of its build; metasystem work review goal-a reviews it again", func(_, c map[string]any) { chainSubject(c)["reviewedMember"] = "outsider" }},
		{"digest changed", "BATCH_JOIN_CHAIN_UNREAD: the reviewed change of goal goal-a is missing or changed; metasystem work review goal-a reviews it again", func(_, c map[string]any) { chainSubject(c)["diffDigest"] = strings.Repeat("b", 64) }},
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
	if err == nil || !strings.HasPrefix(err.Error(), "BATCH_JOIN_CHAIN_REQUIRED:") || !strings.Contains(err.Error(), "metasystem work build goal-a") {
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
func joiningUnit(goalID, chain string) Unit {
	return Unit{GoalID: goalID, Chain: chain, Claim: Claim{Machine: "seat", Lineage: goalID, Epoch: 1, Revision: 2, AccountingRevision: 1}, Gate: runIDs{"gate-1"}}
}

func joinPlanMode(mode testpolicy.Mode) func(string, string, string) (testpolicy.Plan, error) {
	return func(string, string, string) (testpolicy.Plan, error) { return testpolicy.Plan{RequiredMode: mode}, nil }
}

// publishJoinVerified publishes a join whose admission tree is verified,
// as a join admitted by its exact tree is.
func publishJoinVerified(store Store, batchID string, unit Unit, actor string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error), handover func() error) error {
	return PublishJoinWithAdmission(store, batchID, unit, actor, at, plan, handover, func(_ string, unit Unit) (JoinAdmission, error) {
		return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified"}, nil
	})
}

func joinBed(t *testing.T) (assemblyBed, Store) {
	bed := assemblyFixture(t)
	store := NewStore(bed.root, scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, BaseTree: bed.base, TipTree: bed.base, State: StateOpen}))
	return bed, store
}

func TestBatchJoinSelectsGroupsFromEachUnitsOwnTree(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	first := joiningUnit("goal-a", "chain-a")
	firstTree := bed.expectJoin(first, testpolicy.Plan{SelectedGroups: []string{"shared", "group-a"}})
	secondTree := ""
	plan := func(root, goal, tree string) (testpolicy.Plan, error) {
		selected := []string{"shared"}
		switch {
		case root == filepath.Join(bed.root, "planning", "goal-a") && goal == "goal-a" && tree == firstTree:
			selected = append(selected, "group-a")
		case root == filepath.Join(bed.root, "planning", "goal-b") && goal == "goal-b" && tree == secondTree:
			selected = append(selected, "group-b")
		default:
			t.Fatalf("unexpected unit planning request: root=%q goal=%q tree=%q", root, goal, tree)
		}
		return testpolicy.Plan{SelectedGroups: selected}, nil
	}
	must(t, publishJoinVerified(store, testBatchID, first, "seat+goal-a", time.Unix(1, 0), plan, func() error { return nil }))
	second := joiningUnit("goal-b", "chain-b")
	secondTree = bed.expectJoin(second, testpolicy.Plan{SelectedGroups: []string{"shared", "group-b"}})
	must(t, publishJoinVerified(store, testBatchID, second, "seat+goal-b", time.Unix(2, 0), plan, func() error { return nil }))
	record := load(t, store)
	if !slices.Equal(record.PrefixTrees, []string{testCommit(201), testCommit(202)}) || record.TipTree != testCommit(202) ||
		record.Units[0].Admission.Tree != firstTree || record.Units[1].Admission.Tree != secondTree {
		t.Fatalf("joined tree binding: prefixes=%v tip=%s admissions=%+v/%+v", record.PrefixTrees, record.TipTree, record.Units[0].Admission, record.Units[1].Admission)
	}
	if !slices.Equal(record.Units[0].SelectedGroups, []string{"shared", "group-a"}) ||
		!slices.Equal(record.Units[1].SelectedGroups, []string{"shared", "group-b"}) {
		t.Fatalf("per-unit selections=%v / %v", record.Units[0].SelectedGroups, record.Units[1].SelectedGroups)
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
	must(t, publishJoinVerified(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), plan, func() error { return nil }))
}

func TestBatchJoinPublicationHoldsTheFlock(t *testing.T) {
	t.Parallel()
	for _, point := range []string{UnitJoining, UnitJoined} {
		t.Run(point, func(t *testing.T) {
			bed := newOrdinaryJoinBed(t)
			store := bed.store
			bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
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
				joinDone <- publishJoinVerified(store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0), joinPlanMode(testpolicy.ModeStandard), func() error { return nil })
			}()
			select {
			case <-reached:
			case err := <-joinDone:
				t.Fatalf("join returned before reaching publication %q: %v", point, err)
			}
			// Any other writer of the batch record waits for the join's
			// flock: here a plain update of the record.
			tickDone := make(chan error, 1)
			go func() {
				err := store.Update(testBatchID, func(*Record) error { return nil })
				events <- "tick"
				tickDone <- err
			}()
			select {
			case <-waiting:
			case err := <-tickDone:
				close(proceed)
				<-joinDone
				t.Fatalf("the update finished before waiting for the join flock: %v", err)
			}
			close(proceed)
			must(t, <-joinDone)
			must(t, <-tickDone)
			got := make([]string, 0, 4)
			releases, ticks := 0, 0
			for range 4 {
				event := <-events
				got = append(got, event)
				if event == "release" {
					releases++
				} else if event == "tick" {
					ticks++
				}
			}
			witness(t, got[0] == "release" && releases == 3 && ticks == 1 && load(t, store).Units[0].State == UnitJoined, "%s event order=%v", point, got)
		})
	}
}

func TestBatchJoinPrechecksBeforePublication(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	store := bed.store
	ejected := joiningUnit("old", "absent")
	ejected.State, ejected.Outcome, ejected.Failure = UnitReturnPending, UnitEjected, "old failure"
	must(t, store.Update(testBatchID, func(record *Record) error { record.Units = append(record.Units, ejected); return nil }))
	handovers := 0
	join := func(unit Unit, mode testpolicy.Mode) error {
		return publishJoinVerified(store, testBatchID, unit, "seat+joiner", time.Unix(1, 0), joinPlanMode(mode), func() error { handovers++; return nil })
	}
	bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	must(t, join(joiningUnit("goal-a", "chain-a"), testpolicy.ModeStandard))
	path, _ := store.recordPath(testBatchID)
	before := string(contents(t, path))
	err := join(joiningUnit("goal-a", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_GOAL_ELSEWHERE") && string(contents(t, path)) == before && handovers == 1, "membership join=%v handovers=%d", err, handovers)
	bed.expectConflict(joiningUnit("goal-conflict", "conflict"))
	err = join(joiningUnit("goal-conflict", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_JOIN_CONFLICT") && string(contents(t, path)) == before && handovers == 1, "conflicting join=%v handovers=%d", err, handovers)
	bed.expectJoin(joiningUnit("goal-b", "chain-b"), testpolicy.Plan{RequiredMode: testpolicy.ModeDeep})
	must(t, join(joiningUnit("goal-b", "chain-b"), testpolicy.ModeDeep))
	record := load(t, store)
	witness(t, record.ClosedReason == "deep-ceiling" && record.Units[2].State == UnitJoined && len(record.PrefixTrees) == 2 && record.TipTree == record.PrefixTrees[1] && len(record.History) == 4 && record.History[0].Verb+":"+record.History[0].Detail == "join:goal-a joining" && record.History[1].Verb+":"+record.History[1].Detail == "join:goal-a joined" && record.History[2].Verb+":"+record.History[2].Detail == "join:goal-b joining" && record.History[3].Verb+":"+record.History[3].Detail == "join:goal-b joined", "deep join record=%+v", record)
	before = string(contents(t, path))
	err = join(joiningUnit("goal-c", "conflict"), testpolicy.ModeStandard)
	witness(t, err != nil && strings.Contains(err.Error(), "BATCH_CLOSED") && string(contents(t, path)) == before && handovers == 2, "post-ceiling join=%v handovers=%d", err, handovers)
}
