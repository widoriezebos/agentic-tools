package batch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
	<-first
	go update(func(record *Record) error { record.TipTree += "+goal-2"; close(second); return nil })
	select {
	case <-contended:
	case <-second:
		t.Error("second join entered the batch mutation while the first held it")
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
