package snapshot

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

const fixtureDate = "2026-08-23T09:00:00+00:00"

var fixtureNow = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// clock is an injected wall: every test decides what "now" is and when it
// moves, so no observation and no deadline depends on how long a test took.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func newClock(at time.Time) *clock { return &clock{at: at} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(by)
}

// bed is a converted single-machine checkout the tests publish ledger
// commits into.
type bed struct {
	t    *testing.T
	root string
}

func newBed(t *testing.T) *bed {
	t.Helper()
	b := &bed{t: t, root: t.TempDir()}
	b.git("init", "-q", "-b", "main")
	b.git("config", "user.name", "snapshot-fixture")
	b.git("config", "user.email", "snapshot-fixture@example.invalid")
	b.git("config", "goal.sync-remote", "local")
	return b
}

func (b *bed) git(args ...string) string {
	b.t.Helper()
	command := exec.Command("git", append([]string{"-C", b.root}, args...)...)
	command.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+fixtureDate, "GIT_COMMITTER_DATE="+fixtureDate)
	out, err := command.CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (b *bed) write(relative string, data []byte) {
	b.t.Helper()
	absolute := filepath.Join(b.root, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(absolute, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
	b.git("add", relative)
}

func (b *bed) writeRoot(syncMode string) {
	b.t.Helper()
	b.write("plans/goals/backlog.md", goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2",
		SyncMode: syncMode, Revision: 1,
	}))
}

func (b *bed) writeGoal(f *goal.GoalFile) {
	b.t.Helper()
	b.write("plans/goals/"+f.Id+".md", goal.RenderFile(f))
}

func (b *bed) commit(message string) string {
	b.t.Helper()
	b.git("commit", "-q", "-m", message)
	return b.git("rev-parse", "HEAD")
}

func (b *bed) accept(tip string) {
	b.t.Helper()
	b.git("update-ref", goal.AcceptedRef, tip)
}

func (b *bed) metasystemRefs() string {
	b.t.Helper()
	return b.git("for-each-ref", "--format=%(refname) %(objectname)", "refs/metasystem/")
}

func (b *bed) committerSeconds(tip string) string {
	b.t.Helper()
	return b.git("log", "-1", "--format=%ct", tip)
}

func bedGoal(id, state string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: state, Intent: "Do " + id, Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{
			At: "2026-08-23T00:00:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-00000000",
			Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{id}, Keep: -1,
		}},
	}
}

// relayedApprovedGoal carries a relayed human word, so whether it still
// admits work depends on the instant the observation is made.
func relayedApprovedGoal(id, reviewBy string) *goal.GoalFile {
	f := bedGoal(id, goal.StateApproved)
	f.Tier = 3
	f.Revision = 2
	f.Budget = &goal.Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1, ReviewRoundLimit: 1}
	event := goal.HistoryLine{
		At: "2026-08-23T00:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAY-human-00000002",
		Verb: "approve", Actor: "human:wido", Targets: []string{id}, Keep: -1,
		AuthorityOutcome: goal.AuthorityOutcomeTemporaryHumanWord, AuthorityReviewBy: reviewBy,
		AuthorityRuling: "01ARZ3NDEKTSV4RRFFQ69G5FB0-ruling-0000000a", TemporaryHumanWord: "go ahead",
	}
	f.History = append(f.History, event)
	f.Approved = &goal.ApprovalRecord{
		By: event.Actor, At: event.At, Revision: 2, Opid: event.Opid,
		Authority: goal.ApprovalAuthorityRelayed, ReviewBy: reviewBy,
		Digest: goal.ApprovalDigest(f.Intent, f.Tier, *f.Budget),
	}
	return f
}

// readableBed is the ordinary world: a local ledger with one goal, accepted.
func readableBed(t *testing.T) (*bed, string) {
	t.Helper()
	b := newBed(t)
	b.writeRoot(goal.SyncLocal)
	b.writeGoal(bedGoal("alpha", goal.StateQueued))
	tip := b.commit("the ledger")
	b.accept(tip)
	return b, tip
}

func TestObserveReadsTheAcceptedTip(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)

	observation := holder.Observe()

	testutil.Expect(t, "state", observation.State, StateRead)
	testutil.Expect(t, "tip", observation.Tip, tip)
	testutil.Expect(t, "the tip is a full object name", len(observation.Tip), 40)
	testutil.Expect(t, "state root", observation.StateRoot, b.root)
	testutil.Expect(t, "observed at", observation.ObservedAt, fixtureNow)
	testutil.Expect(t, "sync mode", observation.SyncMode, goal.SyncLocal)
	testutil.Expect(t, "admission", observation.Admission.Answered, true)
	testutil.Expect(t, "the committer time", observation.CommittedAt.Unix(), mustSeconds(t, b.committerSeconds(tip)))
	testutil.Expect(t, "the tree", observation.Tree.Live["alpha"].Intent, "Do alpha")
	testutil.Expect(t, "the working tree is not counted", []*int{observation.LiveFiles, observation.ArchivedFiles}, []*int{nil, nil})
	testutil.Expect(t, "problems", observation.Problems, []goal.Problem(nil))
}

func TestObserveNamesEveryLedgerState(t *testing.T) {
	t.Parallel()

	t.Run("a clone that has not fetched the ref", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.writeRoot(goal.SyncLocal)
		b.writeGoal(bedGoal("alpha", goal.StateQueued))
		b.writeGoal(bedGoal("beta", goal.StateQueued))
		b.write("records/goals/gamma.md", goal.RenderFile(concluded("gamma")))
		b.commit("the ledger")
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateAbsent)
		testutil.Expect(t, "tip", observation.Tip, "")
		testutil.Require(t, "the live count is known", observation.LiveFiles != nil, true)
		testutil.Require(t, "the archived count is known", observation.ArchivedFiles != nil, true)
		testutil.Expect(t, "the live goal files", *observation.LiveFiles, 2)
		testutil.Expect(t, "the archived goal records", *observation.ArchivedFiles, 1)
	})

	t.Run("a ref at a commit that carries no ledger", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.write("README.md", []byte("no ledger here\n"))
		before := b.commit("before the ledger")
		b.writeRoot(goal.SyncLocal)
		b.writeGoal(bedGoal("alpha", goal.StateQueued))
		b.commit("the ledger")
		b.accept(before)
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateNoLedger)
		testutil.Expect(t, "tip", observation.Tip, before)
		testutil.Require(t, "the live count is known", observation.LiveFiles != nil, true)
		testutil.Expect(t, "the live goal files", *observation.LiveFiles, 1)
	})

	t.Run("a ref file git cannot read", func(t *testing.T) {
		t.Parallel()
		b, _ := readableBed(t)
		writeGarbageRef(t, b)
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateBroken)
		testutil.Expect(t, "the message names the ref", strings.Contains(observation.Message, goal.AcceptedRef), true)
	})

	t.Run("a goal file whose integrity line does not match", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.writeRoot(goal.SyncLocal)
		torn := goal.RenderFile(bedGoal("torn", goal.StateQueued))
		b.write("plans/goals/torn.md", append([]byte("# torn\n- State: queued\n"), torn...))
		tip := b.commit("a torn ledger")
		b.accept(tip)
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateUnreadable)
		testutil.Expect(t, "tip", observation.Tip, tip)
		testutil.Require(t, "the problems are typed", len(observation.Problems) > 0, true)
		testutil.Expect(t, "a problem names the file", problemsName(observation.Problems, "plans/goals/torn.md"), true)
		testutil.Expect(t, "the message carries the refusal", strings.Contains(observation.Message, "does not parse"), true)
	})

	t.Run("a tree the at-rest rules refuse", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.writeRoot(goal.SyncLocal)
		first, second := bedGoal("alpha", goal.StateQueued), bedGoal("beta", goal.StateQueued)
		first.Priority, first.Sequence = 1, 1
		second.Priority, second.Sequence = 1, 1
		b.writeGoal(first)
		b.writeGoal(second)
		tip := b.commit("two goals at one rank")
		b.accept(tip)
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateUnreadable)
		testutil.Require(t, "the problems are typed", len(observation.Problems), 1)
		testutil.Expect(t, "the problem is the rank rule", strings.Contains(string(observation.Problems[0]), "must occupy exactly"), true)

		// A commit's refusal is as permanent as its tree, so a second look at
		// the same tip answers the same without reading it again.
		again := holder.Observe()
		testutil.Expect(t, "the second state", again.State, StateUnreadable)
		testutil.Expect(t, "the second problems", again.Problems, observation.Problems)
	})

	t.Run("a ledger whose sync mode the configuration contradicts", func(t *testing.T) {
		t.Parallel()
		b := newBed(t)
		b.writeRoot(goal.SyncRemote)
		b.writeGoal(bedGoal("alpha", goal.StateQueued))
		tip := b.commit("a remote ledger")
		b.accept(tip)
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateRefused)
		testutil.Expect(t, "the message is the engine's", strings.Contains(observation.Message, "sync-mode mismatch refused"), true)
	})

	t.Run("a sync branch that is not fully qualified", func(t *testing.T) {
		t.Parallel()
		b, _ := readableBed(t)
		b.git("config", "goal.sync-branch", "main")
		holder := New(b.root, newClock(fixtureNow).now)

		observation := holder.Observe()

		testutil.Expect(t, "state", observation.State, StateRefused)
		testutil.Expect(t, "the message is the engine's", strings.Contains(observation.Message, "must be fully qualified"), true)
	})
}

// TestObserveRereadsConfigurationUnderAStandingTip is the caching rule's
// point: git configuration changes independently of the tip, so a verdict
// kept with the tree would keep answering for a world that has moved.
func TestObserveRereadsConfigurationUnderAStandingTip(t *testing.T) {
	t.Parallel()
	b, _ := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)

	first := holder.Observe()
	testutil.Require(t, "the first state", first.State, StateRead)

	b.git("config", "goal.sync-remote", "origin")
	flipped := holder.Observe()
	testutil.Expect(t, "the flipped state", flipped.State, StateRefused)
	testutil.Expect(t, "the flipped message", strings.Contains(flipped.Message, "sync-mode mismatch refused"), true)

	b.git("config", "goal.sync-remote", "local")
	restored := holder.Observe()
	testutil.Expect(t, "the restored state", restored.State, StateRead)
	if restored.Tree != first.Tree {
		t.Fatalf("the tree was read again although the tip never moved")
	}
}

// TestObserveKeepsOneReadPerTip proves what is kept and what is not: the
// validated tree belongs to its commit, and a new tip is a new read.
func TestObserveKeepsOneReadPerTip(t *testing.T) {
	t.Parallel()
	b, _ := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)

	first := holder.Observe()
	second := holder.Observe()
	if first.Tree != second.Tree {
		t.Fatalf("the same tip was read twice")
	}

	b.writeGoal(bedGoal("beta", goal.StateQueued))
	moved := b.commit("one more goal")
	b.accept(moved)
	third := holder.Observe()

	testutil.Expect(t, "the new tip", third.Tip, moved)
	testutil.Expect(t, "the new tree", len(third.Tree.Live), 2)
	if third.Tree == first.Tree {
		t.Fatalf("a moved tip answered from the previous tip's tree")
	}
}

// TestObserveJudgesExpiryAtTheObservationsOwnInstant proves what is not
// kept: expiry moves while the tip stands still.
func TestObserveJudgesExpiryAtTheObservationsOwnInstant(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.writeRoot(goal.SyncLocal)
	b.writeGoal(relayedApprovedGoal("relayed", "2026-09-02"))
	tip := b.commit("a relayed approval")
	b.accept(tip)
	at := newClock(time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC))
	holder := New(b.root, at.now)

	within := holder.Observe()
	testutil.Require(t, "the state on the review date", within.State, StateRead)
	testutil.Expect(t, "ready on the review date", within.Admission.Ready, map[string]bool{"relayed": true})

	at.advance(48 * time.Hour)
	past := holder.Observe()
	testutil.Expect(t, "awaiting two days later", past.Admission.Awaiting, map[string]bool{"relayed": true})
	testutil.Expect(t, "ready two days later", past.Admission.Ready, map[string]bool{})
	if past.Tree != within.Tree {
		t.Fatalf("a moved clock re-read a standing tip")
	}
}

func TestObserveIsSafeFromManyReadersAtOnce(t *testing.T) {
	t.Parallel()
	b, tip := readableBed(t)
	holder := New(b.root, newClock(fixtureNow).now)

	var readers sync.WaitGroup
	states := make([]Observation, 8)
	for index := range states {
		readers.Add(1)
		go func(index int) {
			defer readers.Done()
			states[index] = holder.Observe()
		}(index)
	}
	readers.Wait()

	seen := make([]string, 0, len(states))
	wanted := make([]string, 0, len(states))
	for _, observation := range states {
		seen = append(seen, string(observation.State)+" "+observation.Tip)
		wanted = append(wanted, string(StateRead)+" "+tip)
	}
	testutil.Expect(t, "every reader sees the accepted tip", seen, wanted)
}

// TestSnapshotRunsNoGitOfItsOwn keeps every git invocation with the engine
// that owns the repository-steering scrub: -C alone does not defeat an
// inherited GIT_DIR, and a second runner here would be a second owner of
// that contract.
func TestSnapshotRunsNoGitOfItsOwn(t *testing.T) {
	t.Parallel()
	// The directory is walked and each file parsed on its own: parser.ParseDir
	// is deprecated for not honouring build tags, and the alternative it names
	// is a module this repository does not depend on. Reading every .go file
	// that is not a test is stricter than a tag-aware walk would be, because a
	// file excluded by a tag is still scanned.
	entries, err := os.ReadDir(".")
	testutil.Require(t, "read the package directory", err, nil)
	set := token.NewFileSet()
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, name, nil, parser.ImportsOnly)
		testutil.Require(t, "parse "+name, parseErr, nil)
		scanned++
		for _, imported := range file.Imports {
			if imported.Path.Value == `"os/exec"` {
				t.Fatalf("%s runs commands of its own", name)
			}
		}
	}
	// The scan proves its own reach: a rename that emptied it would otherwise
	// pass by finding nothing.
	if scanned == 0 {
		t.Fatal("the scan read no source file, so it proved nothing")
	}
}

func concluded(id string) *goal.GoalFile {
	f := bedGoal(id, goal.StateDone)
	f.Conclude = "Finished."
	return f
}

func writeGarbageRef(t *testing.T, b *bed) {
	t.Helper()
	path := filepath.Join(b.git("rev-parse", "--path-format=absolute", "--git-common-dir"), filepath.FromSlash(goal.AcceptedRef))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not an object name\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func problemsName(problems []goal.Problem, path string) bool {
	for _, problem := range problems {
		if strings.Contains(string(problem), path) {
			return true
		}
	}
	return false
}

func mustSeconds(t *testing.T, raw string) int64 {
	t.Helper()
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		t.Fatalf("the fixture's commit time is not a number: %v", err)
	}
	return seconds
}
