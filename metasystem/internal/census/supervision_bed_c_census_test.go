package census

// Ported from scripts/agents/supervision-fixtures.sh, scenario
// census-lifecycle (part C of the bed): every census classification and
// verdict assertion the bed made through a live watcher now runs here over a
// synthetic process table, a recorded supervision state and an injected
// clock. No process is spawned and no pass is waited for; each assertion is
// one census pass computed in process.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// supCNow is the injected census clock every pass in this file uses.
var supCNow = time.Unix(1786000000, 0)

// supCBed is one checkout under census: a fake-mode metasystem root that is
// also the repository scope (the bed's layout), with a peer directory beside
// it that must stay out of scope.
type supCBed struct {
	parent  string // canonical temp parent holding repo and peer
	repo    string // metasystem root, state root and census scope
	peer    string
	process string // the synthetic process table file
}

func supCNewBed(t *testing.T) supCBed {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := supCBed{
		parent:  parent,
		repo:    filepath.Join(parent, "repo"),
		peer:    filepath.Join(parent, "peer"),
		process: filepath.Join(parent, "process-fixture.json"),
	}
	for _, dir := range []string{
		filepath.Join(bed.repo, "artifacts", "agents", "mains"),
		filepath.Join(bed.repo, "artifacts", "agents", "jobs"),
		filepath.Join(bed.repo, "artifacts", "agents", "supervision"),
		bed.peer,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The census compiles the fake runtime's signature from the engine's
	// registry (the adapter scripts retired with the adapters' port to Go).
	supCWrite(t, filepath.Join(bed.repo, "metasystem.conf"), "metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n")
	supCWrite(t, filepath.Join(bed.repo, "artifacts", "agents", "supervision", "state.json"),
		`{"generation":3,"owner":{"pid":71001,"pidStartedAt":1,"instanceTag":"owner-t"},`+
			`"components":{"watcher":{"pid":71002,"pidStartedAt":1,"instanceTag":"watcher-t"},`+
			`"reaper":{"pid":71003,"pidStartedAt":1,"instanceTag":"reaper-t"},`+
			`"landing-owner":{"pid":71004,"pidStartedAt":1,"instanceTag":"landing-t"}}}`+"\n")
	supCWrite(t, bed.process, "[]\n")
	return bed
}

func supCWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// supCRow renders one process-table row the way the bed's
// write_process_fixture did; cwd "UNRESOLVED" renders a per-process cwd error.
func supCRow(pid, started int64, argv, cwd string, alive bool) map[string]any {
	row := map[string]any{"pid": pid, "ppid": 1, "pgid": pid, "pidStartedAt": started,
		"argv": argv, "cwd": cwd, "cwdError": false, "alive": alive}
	if cwd == "UNRESOLVED" {
		row["cwd"], row["cwdError"] = nil, true
	}
	return row
}

func (bed supCBed) table(t *testing.T, rows ...map[string]any) {
	t.Helper()
	if rows == nil {
		rows = []map[string]any{}
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	supCWrite(t, bed.process, string(data)+"\n")
}

// census runs one pass over the recorded bundle. The supervision identities
// are treated as live (the verifier seam), so a pass with nothing else wrong
// is SUCCESS, as the bed's live watcher made it.
func (bed supCBed) census(t *testing.T) Verdict {
	t.Helper()
	return bed.censusWith(t, nil)
}

func (bed supCBed) censusWith(t *testing.T, resolve func([]int64) map[int64]cwdResult) Verdict {
	t.Helper()
	verdict, err := runCensusVerifying(bed.repo, bed.repo, bed.repo, "fixture-fingerprint", 1, supCNow,
		func(root string) ([]Process, error) { return enumerateFixture(root, bed.process) }, resolve,
		func(map[string]identityRecord, identity.FixtureProbe, *[]string) {})
	if err != nil {
		t.Fatal(err)
	}
	return verdict
}

func supCHas(verdict Verdict, class string, pid int64) bool {
	for _, item := range verdict.Inventory {
		if item.Pid == pid && item.Class == class {
			return true
		}
	}
	return false
}

func supCHasPid(verdict Verdict, pid int64) bool {
	for _, item := range verdict.Inventory {
		if item.Pid == pid {
			return true
		}
	}
	return false
}

func supCHasPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

// enumerate-filter-resolve: census cost follows agent-shaped processes, not
// the host process count. A thousand unrelated rows and two agent rows leave
// exactly the two agents in the inventory, and the cwd resolver is asked only
// for the matched pids.
func TestSupCCensusFiltersBeforeResolving(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	rows := make([]map[string]any, 0, 1002)
	for index := 0; index < 1000; index++ {
		rows = append(rows, supCRow(int64(50000+index), 100, fmt.Sprintf("/usr/bin/non-agent-%d --flag value", index), bed.repo, true))
	}
	rows = append(rows,
		supCRow(61001, 100, "metasystem-fake-agent first", bed.repo, true),
		supCRow(61002, 100, "/tool/metasystem-fake-agent second", bed.repo, true))
	bed.table(t, rows...)
	var asked []int64
	verdict := bed.censusWith(t, func(pids []int64) map[int64]cwdResult {
		asked = append(asked, pids...)
		out := map[int64]cwdResult{}
		for _, pid := range pids {
			out[pid] = cwdResult{Cwd: bed.repo}
		}
		return out
	})
	var pids []int64
	for _, item := range verdict.Inventory {
		pids = append(pids, item.Pid)
	}
	if !reflect.DeepEqual(pids, []int64{61001, 61002}) {
		t.Fatalf("inventory is not exactly the two agent processes: %v", pids)
	}
	sort.Slice(asked, func(i, j int) bool { return asked[i] < asked[j] })
	if !reflect.DeepEqual(asked, []int64{61001, 61002}) {
		t.Fatalf("cwds were resolved for unmatched processes: %v", asked)
	}
	if verdict.Verdict != "SUCCESS" || verdict.DurationMs < 0 {
		t.Fatalf("verdict = %s durationMs = %d errors = %v", verdict.Verdict, verdict.DurationMs, verdict.Errors)
	}
}

// S4-5 and S4-10: the verdict is schema-versioned, names its writer, carries
// integer timing fields, a fingerprint, the arming generation, a digest that
// attests the exact state bytes, and counts over exactly the three classes.
func TestSupCCensusVerdictAttestsTheState(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	verdict := bed.census(t)
	if verdict.SchemaVersion != 2 || verdict.Writer != "watch-background-jobs.sh" {
		t.Fatalf("S4-5: envelope = %d %q", verdict.SchemaVersion, verdict.Writer)
	}
	if verdict.Verdict != "SUCCESS" {
		t.Fatalf("S4-5: verdict = %s errors = %v", verdict.Verdict, verdict.Errors)
	}
	if verdict.CompletedAtEpoch != supCNow.Unix() || verdict.IntervalSec != 1 || verdict.DurationMs < 0 {
		t.Fatalf("S4-5: timing fields = %d %d %d", verdict.CompletedAtEpoch, verdict.IntervalSec, verdict.DurationMs)
	}
	if verdict.Fingerprint == "" {
		t.Fatal("S4-5: fingerprint is empty")
	}
	if verdict.Generation == nil || *verdict.Generation != 3 {
		t.Fatalf("S4-5: census generation does not match the state: %v", verdict.Generation)
	}
	state, err := os.ReadFile(filepath.Join(bed.repo, "artifacts", "agents", "supervision", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(state)
	if verdict.StateDigest == nil || *verdict.StateDigest != hex.EncodeToString(sum[:]) {
		t.Fatalf("S4-5: stateDigest does not attest the state bytes: %v", verdict.StateDigest)
	}
	var keys []string
	for key := range verdict.Counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"ANNOUNCED", "CUSTODY", "UNTRACKED"}) {
		t.Fatalf("S4-5: counts keys = %v", keys)
	}
	// The published wire carries every field the dispatch gate requires.
	encoded, err := json.Marshal(verdict)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"schemaVersion", "writer", "verdict", "completedAtEpoch", "intervalSec",
		"durationMs", "fingerprint", "generation", "stateDigest", "counts", "inventory", "diagnostics", "errors"} {
		if _, ok := wire[field]; !ok {
			t.Fatalf("S4-5: verdict wire lacks %s: %s", field, encoded)
		}
	}
}

// The three-class inventory: an announced main is ANNOUNCED; a raw agent, a
// custody child whose recorded start is stale (S4-2), a worktree process, an
// argv naming a not-yet-created path below the repo (S4-9), a relative argv
// path resolved from a peer cwd, and an agent whose cwd cannot be resolved but
// whose argv names the repo (S4-6) are all UNTRACKED; a peer repository
// process stays out of scope; the unresolved cwd is surfaced per process.
func TestSupCCensusThreeClassInventory(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	worktree := filepath.Join(bed.repo, "artifacts", "agents", "worktrees", "w")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	now := supCNow.Unix()
	const (
		raw, announced, custody, inWorktree, peer, argvPath, unresolved, relative = 41001, 41002, 41003, 41004, 41005, 41006, 41007, 41008
	)
	bed.table(t,
		supCRow(raw, now-10, "metasystem-fake-agent raw", bed.repo, true),
		supCRow(announced, now-9, "metasystem-fake-agent announced", bed.repo, true),
		supCRow(custody, now-8, "metasystem-fake-agent child --tag metasystem-job-owned", bed.repo, true),
		supCRow(inWorktree, now-7, "metasystem-fake-agent worktree", worktree, true),
		supCRow(peer, now-6, "metasystem-fake-agent peer", bed.peer, true),
		supCRow(argvPath, now-5, "metasystem-fake-agent --workspace="+bed.repo+"/not-created/yet", bed.peer, true),
		supCRow(unresolved, now-4, "metasystem-fake-agent --repo="+bed.repo, "UNRESOLVED", true),
		supCRow(relative, now-3, "metasystem-fake-agent --workspace=../repo/not-created/relative", bed.peer, true),
	)
	supCWrite(t, filepath.Join(bed.repo, "artifacts", "agents", "mains", fmt.Sprintf("announced-%d.json", announced)),
		fmt.Sprintf(`{"sessionId":"announced","pid":%d,"pidStartedAt":%d,"pgid":%d,"runtime":"fake","instanceTag":"main-announced","announcedAt":"2026-09-27T00:00:00Z"}`+"\n",
			announced, now-9, announced))
	// The custody record names the child with a start one second off.
	supCWriteOwnedJob(t, bed, custody, now-8-1, "metasystem-job-owned")

	verdict := bed.census(t)
	if !supCHas(verdict, "ANNOUNCED", announced) {
		t.Fatalf("announced main is not ANNOUNCED: %+v", verdict.Inventory)
	}
	for _, pid := range []int64{raw, custody, inWorktree, argvPath, unresolved, relative} {
		if !supCHas(verdict, "UNTRACKED", pid) {
			t.Fatalf("pid %d is not UNTRACKED: %+v", pid, verdict.Inventory)
		}
	}
	if supCHasPid(verdict, peer) {
		t.Fatalf("peer repository process entered scope: %+v", verdict.Inventory)
	}
	want := fmt.Sprintf("UNRESOLVED-CWD pid=%d", unresolved)
	found := false
	for _, diagnostic := range verdict.Diagnostics {
		if strings.Contains(diagnostic, want) {
			found = true
		}
	}
	if !found {
		t.Fatalf("S4-6: unresolved cwd was not surfaced per process: %v", verdict.Diagnostics)
	}
	if verdict.Verdict != "SUCCESS" {
		t.Fatalf("an in-scope unresolved cwd failed the census: %v", verdict.Errors)
	}
}

func supCWriteOwnedJob(t *testing.T, bed supCBed, childPid, childStart int64, childTag string) {
	t.Helper()
	supCWrite(t, filepath.Join(bed.repo, "artifacts", "agents", "jobs", "owned.json"), fmt.Sprintf(
		`{"jobId":"owned","status":"running","runtime":"fake","workspaceRoot":%q,"pid":71010,"pidStartedAt":5,"pgid":71010,`+
			`"instanceTag":"metasystem-job-owned","startedAt":"2099-01-01T00:00:00Z","capMin":30,`+
			`"custodyProcesses":[{"pid":%d,"pidStartedAt":%d,"instanceTag":%q}]}`+"\n",
		bed.repo, childPid, childStart, childTag))
}

// S4-2: only the exact pid+start+tag triple classifies a job's child as
// CUSTODY. Correcting the start alone while the tag is wrong gains nothing.
func TestSupCCensusCustodyNeedsTheExactTriple(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	const child = 41003
	started := supCNow.Unix() - 8
	bed.table(t, supCRow(child, started, "metasystem-fake-agent child --tag metasystem-job-owned", bed.repo, true))

	supCWriteOwnedJob(t, bed, child, started, "wrong-tag")
	if verdict := bed.census(t); !supCHas(verdict, "UNTRACKED", child) {
		t.Fatalf("S4-2: wrong instanceTag gained custody: %+v", verdict.Inventory)
	}
	supCWriteOwnedJob(t, bed, child, started, "metasystem-job-owned")
	if verdict := bed.census(t); !supCHas(verdict, "CUSTODY", child) {
		t.Fatalf("S4-2: the exact triple did not join custody: %+v", verdict.Inventory)
	}
}

// S4-6: total enumeration failure is CENSUS-FAILED with the enumeration named
// in the verdict's errors, and the next readable table recovers to SUCCESS.
func TestSupCCensusEnumerationFailureAndRecovery(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	supCWrite(t, bed.process, "{broken\n")
	failed := bed.census(t)
	if failed.Verdict != "CENSUS-FAILED" || !supCHasPrefix(failed.Errors, "enumeration:") {
		t.Fatalf("S4-6: enumeration failure = %s %v", failed.Verdict, failed.Errors)
	}
	bed.table(t)
	if recovered := bed.census(t); recovered.Verdict != "SUCCESS" {
		t.Fatalf("census did not recover: %v", recovered.Errors)
	}
}

// S4-6: a process that exited between enumeration and inspection is a named
// RACED-EXIT diagnostic, never a census failure.
func TestSupCCensusRacedExitIsNotAFailure(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	const raced = 41009
	bed.table(t, supCRow(raced, supCNow.Unix(), "metasystem-fake-agent raced", bed.repo, false))
	verdict := bed.census(t)
	if !supCHasPrefix(verdict.Diagnostics, fmt.Sprintf("RACED-EXIT pid=%d", raced)) {
		t.Fatalf("S4-6: raced exit was not named: %v", verdict.Diagnostics)
	}
	if verdict.Verdict != "SUCCESS" || supCHasPid(verdict, raced) {
		t.Fatalf("S4-6: exited process race failed the census or stayed in inventory: %s %v %+v",
			verdict.Verdict, verdict.Errors, verdict.Inventory)
	}
}

// S4-6: a single in-scope process whose argv or start time cannot be read
// fails the census under its own error prefix; an empty table recovers.
func TestSupCCensusPartialReadFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		row    func(bed supCBed) map[string]any
		prefix string
	}{
		{"unreadable argv", func(bed supCBed) map[string]any {
			return supCRow(41010, supCNow.Unix(), "metasystem-fake-agent --workspace='", bed.repo, true)
		}, "argv-unreadable:41010"},
		{"unreadable start time", func(bed supCBed) map[string]any {
			return supCRow(41011, -1, "metasystem-fake-agent bad-start", bed.repo, true)
		}, "start-time-unreadable:41011"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := supCNewBed(t)
			bed.table(t, tc.row(bed))
			verdict := bed.census(t)
			if verdict.Verdict != "CENSUS-FAILED" || !supCHasPrefix(verdict.Errors, tc.prefix) {
				t.Fatalf("verdict = %s errors = %v, want CENSUS-FAILED with %s", verdict.Verdict, verdict.Errors, tc.prefix)
			}
			bed.table(t)
			if recovered := bed.census(t); recovered.Verdict != "SUCCESS" {
				t.Fatalf("partial-failure recovery = %v", recovered.Errors)
			}
		})
	}
}

// Dead announcements are pruned by the census: an announced main that has
// exited, or whose pid now belongs to a different process, loses its file,
// while a live main's announcement stays.
func TestSupCCensusPrunesDeadAnnouncements(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	now := supCNow.Unix()
	mains := filepath.Join(bed.repo, "artifacts", "agents", "mains")
	announce := func(session string, pid, started int64) string {
		path := filepath.Join(mains, fmt.Sprintf("%s-%d.json", session, pid))
		supCWrite(t, path, fmt.Sprintf(
			`{"sessionId":%q,"pid":%d,"pidStartedAt":%d,"pgid":%d,"runtime":"fake","instanceTag":"main-%s","announcedAt":"2026-09-27T00:00:00Z"}`+"\n",
			session, pid, started, pid, session))
		return path
	}
	exited := announce("dead-main", 42001, now-20)
	recycled := announce("recycled-main", 42002, now-20)
	live := announce("live-main", 42003, now-20)
	bed.table(t,
		supCRow(42001, now-20, "fixture-hold dead-main", bed.repo, false),
		supCRow(42002, now-2, "fixture-hold other", bed.repo, true),
		supCRow(42003, now-20, "fixture-hold live-main", bed.repo, true))
	bed.census(t)
	for _, path := range []string{exited, recycled} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("dead announcement was not pruned: %s (%v)", path, err)
		}
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("a live main's announcement was pruned: %v", err)
	}
}

// S4-3: the fingerprint moves with relevant configuration and never with the
// supervisor's own instance identity. That it moves with the engine carrying
// the signature registry, not with adapter scripts, is internal/census's
// TestFingerprintMovesWithEngineAndSignatureSetNotAdapterScripts.
func TestSupCFingerprintTracksCodeAndConfigNotInstances(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	for _, rel := range FingerprintFiles() {
		path := filepath.Join(bed.repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			supCWrite(t, path, "fixture "+rel+"\n")
		}
	}
	fingerprint := func() string {
		t.Helper()
		value, err := Fingerprint(bed.repo, bed.repo)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	base := fingerprint()

	conf := filepath.Join(bed.repo, "metasystem.conf")
	confBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	supCWrite(t, conf, string(confBytes)+"watch.stale-min=21\n")
	if fingerprint() == base {
		t.Fatal("S4-3: relevant configuration did not alter the fingerprint")
	}
	supCWrite(t, conf, string(confBytes))

	state := filepath.Join(bed.repo, "artifacts", "agents", "supervision", "state.json")
	stateBytes, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	supCWrite(t, state, strings.Replace(string(stateBytes), `"owner-t"`, `"owner-t-changed"`, 1))
	if fingerprint() != base {
		t.Fatal("S4-3: supervisor instance identity altered the static fingerprint")
	}
}
