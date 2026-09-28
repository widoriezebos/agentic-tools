package cachedomain_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

// processTable is a fake kernel: pid to exact identity and parent. Nothing
// reads the live process table.
type processTable map[int64]struct {
	start  time.Time
	parent int64
}

func (table processTable) seams(self int64) cachedomain.Seams {
	return cachedomain.Seams{
		Self: func() int64 { return self },
		Probe: func(pid int64) (identity.Exact, bool) {
			entry, found := table[pid]
			if !found {
				return identity.Exact{}, false
			}
			return identity.Exact{Pid: pid, StartedAt: entry.start}, true
		},
		Parent: func(pid int64) (int64, bool) {
			entry, found := table[pid]
			return entry.parent, found
		},
		Git: func(string, ...string) (string, error) { return "", errors.New("no git in this fixture") },
		DelegateCustody: func(string, string, string, int64) (bool, error) {
			return false, errors.New("no delegate custody in this fixture")
		},
		UserCacheDir: func() (string, error) { return "/machine/cache", nil },
	}
}

func ref(t *testing.T, pid int64, start time.Time) string {
	t.Helper()
	encoded, err := identity.EncodeRef(identity.Exact{Pid: pid, StartedAt: start}.Ref())
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

var (
	shellStart   = time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	engineStart  = time.Date(2026, 9, 29, 1, 1, 0, 0, time.UTC)
	childStart   = time.Date(2026, 9, 29, 1, 2, 0, 0, time.UTC)
	recycleStart = time.Date(2026, 9, 29, 1, 3, 0, 0, time.UTC)
)

// seat shell 10 -> engine 20 -> child 30 (this process)
func ancestry() processTable {
	return processTable{
		10: {start: shellStart, parent: 1},
		20: {start: engineStart, parent: 10},
		30: {start: childStart, parent: 20},
	}
}

// The issuer check walks the real ancestry by exact identity: the engine
// that issued a context is found; a recycled pid with another start time,
// a process outside the ancestry, and an unreadable reference are not.
func TestLiveAncestorComparesExactIdentity(t *testing.T) {
	t.Parallel()
	seams := ancestry().seams(30)
	for name, want := range map[string]bool{
		ref(t, 20, engineStart):  true,
		ref(t, 30, childStart):   true,
		ref(t, 20, recycleStart): false,
		ref(t, 99, engineStart):  false,
	} {
		if got, err := seams.LiveAncestor(name); err != nil || got != want {
			t.Errorf("liveAncestor(%s) = %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := seams.LiveAncestor("pid=20"); err == nil {
		t.Error("an unreadable reference was accepted")
	}
}

func writeScratchRecord(t *testing.T, control, name, body string) {
	t.Helper()
	path := filepath.Join(control, "artifacts", "agents", "proof-runs", "scratch", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Rule 3 through the real record reader: a proof child whose attempt's
// scratch record names its launcher as a live ancestor takes the record's
// paths, never GOCACHE; a forged locator naming a real ancestor (the seat's
// shell) that is no record's launcher is refused; a locator naming no
// scratch record is no evidence.
func TestProofRecordBindsTheLocatorToTheRecordsLauncher(t *testing.T) {
	t.Parallel()
	control := t.TempDir()
	writeScratchRecord(t, control, "scratch-1", `{"schema":"x","run":"scratch-1","launcher":"`+ref(t, 20, engineStart)+`","attempt":"proof-a","goCache":"/run/go-build","staticcheckCache":"/run/staticcheck"}`)
	writeScratchRecord(t, control, "scratch-2", `{"schema":"x","run":"scratch-2","launcher":"`+ref(t, 77, engineStart)+`","attempt":"proof-b","custodians":[{"ref":"`+ref(t, 88, engineStart)+`","group":1}]}`)
	seams := ancestry().seams(30)
	locator := func(attempt string) []string {
		return []string{"METASYSTEM_PROOF_CONTROL_ROOT=" + control, "METASYSTEM_PROOF_ATTEMPT=" + attempt, "GOCACHE=/forged", "STATICCHECK_CACHE=/forged"}
	}
	got, err := seams.Resolve(locator("proof-a"), "")
	if err != nil || got.Rule != "proof-record" || got.Paths != (gocache.Paths{GoCache: "/run/go-build", StaticcheckCache: "/run/staticcheck"}) {
		t.Fatalf("authenticated locator: %+v %v", got, err)
	}
	var refusal gocache.Refusal
	if _, err := seams.Resolve(locator("proof-b"), ""); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "proof-b") {
		t.Fatalf("a record whose launcher is no ancestor: %v", err)
	}
	if got, err := seams.Resolve(locator("proof-none"), ""); err != nil || got.Rule != "outermost" || got.Paths.GoCache != "/machine/cache/go-build" {
		t.Fatalf("a locator naming no record: %+v %v", got, err)
	}
}

// Rule 4 end to end: a bed-shaped child (replaced HOME, no locator) whose
// context the engine issued resolves the engine's paths; the same context
// in a process outside that ancestry is refused, named.
func TestContextIssuedByAnAncestorCarriesThePaths(t *testing.T) {
	t.Parallel()
	engine := ancestry().seams(20)
	issued, err := engine.Resolve([]string{"HOME=/Users/seat"}, "")
	if err != nil {
		t.Fatal(err)
	}
	child := issued.Apply([]string{"HOME=/bed/home", "GOCACHE=/bed/home/Library/Caches/go-build"})
	got, err := ancestry().seams(30).Resolve(child, "")
	if err != nil || got.Rule != "context" || got.Paths != issued.Paths {
		t.Fatalf("bed child: %+v %v", got, err)
	}
	stranger := processTable{40: {start: childStart, parent: 1}}.seams(40)
	var refusal gocache.Refusal
	if _, err := stranger.Resolve(child, ""); !errors.As(err, &refusal) || !strings.Contains(err.Error(), gocache.ContextEnv) {
		t.Fatalf("a foreign context: %v", err)
	}
}

// Rule 2 through a fake git: a dispatcher-made job worktree whose record
// names it is delegate whatever the environment says; a seat's own linked
// worktree without a job record is not.
func TestJobWorktreeIsDecidedByTheRecordThatNamesIt(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	worktree := filepath.Join(repo, "metasystem", "artifacts", "agents", "worktrees", "job-7")
	seat := filepath.Join(t.TempDir(), "seat")
	for _, dir := range []string{filepath.Join(worktree, "metasystem"), filepath.Join(seat, "metasystem"), filepath.Join(repo, "metasystem", "artifacts", "agents", "jobs")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "metasystem", "artifacts", "agents", "jobs", "job-7.json"), []byte(`{"jobId":"job-7","workspaceRoot":"`+worktree+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	answers := map[string]map[string]string{
		filepath.Join(worktree, "metasystem"): {"--show-toplevel": worktree, "--absolute-git-dir": filepath.Join(repo, ".git", "worktrees", "job-7"), "--git-common-dir": filepath.Join(repo, ".git")},
		filepath.Join(seat, "metasystem"):     {"--show-toplevel": seat, "--absolute-git-dir": filepath.Join(repo, ".git", "worktrees", "seat"), "--git-common-dir": filepath.Join(repo, ".git")},
	}
	seams := ancestry().seams(30)
	seams.Git = func(dir string, args ...string) (string, error) {
		value, found := answers[dir][args[len(args)-1]]
		if !found {
			return "", errors.New("not a repository")
		}
		return value, nil
	}
	got, err := seams.Resolve([]string{"GOCACHE=/machine/cache/go-build"}, filepath.Join(worktree, "metasystem"))
	if err != nil || got.Domain != gocache.DomainDelegate || got.Rule != "job-worktree" {
		t.Fatalf("job worktree: %+v %v", got, err)
	}
	if got, err := seams.Resolve(nil, filepath.Join(seat, "metasystem")); err != nil || got.Domain != gocache.DomainEngine {
		t.Fatalf("a seat's linked worktree: %+v %v", got, err)
	}
}

// Rule 1 against the real kernel: a job record whose recorded custody is
// this test process's parent authenticates the markers (delegate, the
// cooperating case); the same markers naming a job whose custody is no
// ancestor are refused, named, and never fall through with GOCACHE.
func TestDelegateMarkersAuthenticateAgainstTheRecordedCustody(t *testing.T) {
	t.Parallel()
	parent, state, err := identity.KernelProber{}.Probe(int64(os.Getppid()))
	if err != nil || state != identity.Alive {
		t.Skipf("the parent process cannot be probed: %v", err)
	}
	stateRoot := t.TempDir()
	jobs := filepath.Join(stateRoot, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := func(job string, exact identity.Exact) {
		ref := exact.Ref()
		body := `{"jobId":"` + job + `","pid":` + itoa(ref.Pid) + `,"pidStartedAt":` + itoa(ref.StartedAtSec) +
			`,"pidStartedAtExactMicro":` + itoa(ref.StartedAtUnixMicro) + `,"pidStartTicks":` + itoa(ref.StartTicks) + `,"bootId":"` + ref.BootID + `"}`
		if err := os.WriteFile(filepath.Join(jobs, job+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	record("ours", parent)
	stranger := parent
	stranger.StartedAt = stranger.StartedAt.Add(-time.Hour)
	record("theirs", stranger)
	markers := func(job string) []string {
		return []string{"METASYSTEM_HOOK_DELEGATE_STATE_ROOT=" + stateRoot, "METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT=" + stateRoot,
			"METASYSTEM_HOOK_DELEGATE_JOB=" + job, "GOCACHE=/forged"}
	}
	seams := cachedomain.Seams{UserCacheDir: func() (string, error) { return "/machine/cache", nil }}
	got, err := seams.Resolve(markers("ours"), "")
	if err != nil || got.Domain != gocache.DomainDelegate || got.Paths.GoCache != "/machine/cache/metasystem-delegate-go-build" {
		t.Fatalf("authenticated markers: %+v %v", got, err)
	}
	var refusal gocache.Refusal
	if _, err := seams.Resolve(markers("theirs"), ""); !errors.As(err, &refusal) || !strings.Contains(err.Error(), "METASYSTEM_HOOK_DELEGATE_JOB=theirs") {
		t.Fatalf("markers of a job whose custody is no ancestor: %v", err)
	}
}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }
