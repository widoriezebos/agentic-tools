package dispatch

// U5f (engine-owns-disk-lifetimes Part B, 3.1's delegate row, 3.2
// "Delegate", 3.7): the real-git claim of a delegate workspace's release.
// Git's linked worktrees, its object quarantine and its alternates are the
// claim; the chain's close check and custody proof are stubbed by seam.

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var delegateNow = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func delegateGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	var stderr strings.Builder
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return out, &gitFailure{args: args, out: stderr.String(), err: err}
	}
	return out, nil
}

type gitFailure struct {
	args []string
	out  string
	err  error
}

func (e *gitFailure) Error() string {
	return strings.Join(e.args, " ") + ": " + e.err.Error() + ": " + e.out
}

// Unwrap exposes the exit status, as the production runner does.
func (e *gitFailure) Unwrap() error { return e.err }

// delegateBed is a checkout with a closed chain whose workspace is a
// registered linked worktree on agent/<chain>, a quarantine the common
// store borrows, and one commit made by the delegate into the quarantine.
type delegateBed struct {
	repo, worktree, quarantine, chain, commit string
	registry                                  diskstore.Registry
	record                                    diskstore.Record
}

func newDelegateBed(t *testing.T, closed bool) delegateBed {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	ctx := context.Background()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := delegateBed{repo: filepath.Join(base, "repo"), chain: "j-chain-1"}
	bed.worktree = filepath.Join(bed.repo, "artifacts", "agents", "worktrees", bed.chain)
	must := func(env []string, dir string, args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		command.Env = append(gittree.ScrubbedEnviron("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com"), env...)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(bed.repo, 0o755); err != nil {
		t.Fatal(err)
	}
	must(nil, bed.repo, "init", "-q", "-b", "main")
	write(filepath.Join(bed.repo, ".gitignore"), "artifacts/\n")
	write(filepath.Join(bed.repo, "README"), "base\n")
	must(nil, bed.repo, "add", ".gitignore", "README")
	must(nil, bed.repo, "commit", "-q", "-m", "base")
	bed.registry = diskstore.CheckoutRegistry(bed.repo)
	var err2 error
	bed.record, err2 = bed.registry.Register(diskstore.Registration{Path: bed.worktree, Git: true, Class: diskstore.DelegateClass,
		Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: bed.chain}, Checkout: bed.repo, Lifetime: diskstore.LifetimeOwner,
		CapKind: diskstore.CapTarget, Layout: diskstore.LayoutCopy}, delegateNow, rand.Reader)
	if err2 != nil {
		t.Fatal(err2)
	}
	must(nil, bed.repo, "worktree", "add", "-q", "-b", "agent/"+bed.chain, bed.worktree, "HEAD")
	gitdir := must(nil, bed.worktree, "rev-parse", "--absolute-git-dir")
	bed.quarantine = filepath.Join(gitdir, diskstore.QuarantineName)
	if err := os.MkdirAll(bed.quarantine, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := diskstore.AddAlternate(filepath.Join(bed.repo, ".git", "objects"), bed.quarantine); err != nil {
		t.Fatal(err)
	}
	identityOf, err := diskstore.ReadGitIdentity(bed.worktree)
	if err != nil {
		t.Fatal(err)
	}
	if bed.record, err = bed.registry.Transition(bed.record.ID, []diskstore.State{diskstore.StateReserved}, diskstore.StateAccepted,
		func(record *diskstore.Record) { record.Identity = identityOf }); err != nil {
		t.Fatal(err)
	}
	env := []string{"GIT_OBJECT_DIRECTORY=" + bed.quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(bed.repo, ".git", "objects")}
	write(filepath.Join(bed.worktree, "work.txt"), "the delegate's work\n")
	must(env, bed.worktree, "add", "work.txt")
	must(env, bed.worktree, "commit", "-q", "-m", "work")
	bed.commit = must(env, bed.worktree, "rev-parse", "HEAD")
	write(filepath.Join(bed.repo, "artifacts", "agents", "jobs", bed.chain+".json"),
		`{"jobId":"`+bed.chain+`","round":1,"role":"implementer","status":"completed","chainClosed":`+map[bool]string{true: "true", false: "false"}[closed]+`}`)
	return bed
}

func (bed delegateBed) proof(dead CustodyDeathOutcome) DelegateWorkspaceProof {
	return DelegateWorkspaceProof{Repo: bed.repo, GitRoot: bed.repo, Git: delegateGit, Now: delegateNow, Stage: "01K2Z7Q3M8XW1V0P9D4J6S5R2T",
		Closed:      func(string, string) error { return nil },
		CustodyDead: func(map[string]any) CustodyDeathResult { return CustodyDeathResult{Outcome: dead, Reason: "fixture"} }}
}

// cleanCensus reads one process whose cwd and files lie elsewhere, or, with
// holder, whose cwd lies inside path.
func cleanCensus(holder string) *diskstore.CensusReader {
	return &diskstore.CensusReader{
		UID: 501, Pids: func() ([]int64, error) { return []int64{4242}, nil },
		ProcessUID: func(int64) (uint32, bool) { return 501, true },
		Use: func(int64) (identity.ProcessUse, error) {
			if holder != "" {
				return identity.ProcessUse{Cwd: holder, Executable: "/bin/sh"}, nil
			}
			return identity.ProcessUse{Cwd: "/", Executable: "/bin/sh"}, nil
		},
		Command: func(int64) string { return "sh" },
	}
}

func (bed delegateBed) pass(t *testing.T, proof DelegateWorkspaceProof, census *diskstore.CensusReader) diskstore.Report {
	t.Helper()
	return bed.passMode(t, proof, census, diskstore.ModeApply)
}

func (bed delegateBed) passMode(t *testing.T, proof DelegateWorkspaceProof, census *diskstore.CensusReader, mode diskstore.Mode) diskstore.Report {
	t.Helper()
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: bed.repo, Registry: bed.registry,
		LockPath: filepath.Join(bed.registry.Dir, ".sweep.flock"), ReportPath: diskstore.CheckoutReportPath(bed.repo), Mode: mode, PlanDir: filepath.Join(bed.repo, "preview-plans"),
		Now: delegateNow, Clock: func() time.Time { return delegateNow }, Entropy: rand.Reader, CensusReader: census,
		Classes: []diskstore.Class{diskstore.RegisteredStores{Registry: bed.registry, Proofs: map[diskstore.OwnerKind]diskstore.OwnerProof{diskstore.OwnerDelegate: proof}}}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func (bed delegateBed) kept(t *testing.T, report diskstore.Report, reason string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(bed.worktree, "work.txt")); err != nil {
		t.Fatalf("the workspace is kept: %v", err)
	}
	lines := append(append([]diskstore.Line(nil), report.Kept...), report.Pending...)
	for _, line := range lines {
		if strings.Contains(line.Reason, reason) && line.Command != "" {
			return
		}
	}
	t.Fatalf("the report names %q with a command: %+v", reason, lines)
}

func TestAClosedCapturedCustodyDeadUnusedChainLosesItsWorkspace(t *testing.T) {
	t.Parallel()
	bed := newDelegateBed(t, true)
	bed.pass(t, bed.proof(CustodyDeathProven), cleanCensus(""))
	if _, err := os.Lstat(bed.worktree); !os.IsNotExist(err) {
		t.Fatalf("the worktree is gone: %v", err)
	}
	if _, err := os.Lstat(bed.quarantine); !os.IsNotExist(err) {
		t.Fatalf("the quarantine is gone with its worktree: %v", err)
	}
	if _, err := delegateGit(context.Background(), bed.repo, "rev-parse", "--verify", "refs/heads/agent/"+bed.chain); err == nil {
		t.Fatal("the branch is deleted")
	}
	archived, err := delegateGit(context.Background(), bed.repo, "rev-parse", "refs/archive/delegate-"+bed.chain+"/agent/"+bed.chain)
	if err != nil || strings.TrimSpace(string(archived)) != bed.commit {
		t.Fatalf("the branch tip is archived: %q %v", archived, err)
	}
	if _, err := delegateGit(context.Background(), bed.repo, "rev-list", "--objects", bed.commit); err != nil {
		t.Fatalf("every quarantine object is readable from the common store: %v", err)
	}
	if alternates, _ := os.ReadFile(filepath.Join(bed.repo, ".git", "objects", "info", "alternates")); strings.TrimSpace(string(alternates)) != "" {
		t.Fatalf("the alternates line is gone: %q", alternates)
	}
	if record, err := bed.registry.Load(bed.record.ID); err != nil || record.State != diskstore.StateReleased {
		t.Fatalf("the record is released: %+v %v", record, err)
	}
}

func TestADelegateWorkspaceIsKeptUntilEveryClauseHolds(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		closed  bool
		dead    CustodyDeathOutcome
		dirty   bool
		records bool
		holder  bool
		reason  string
	}{
		"an open chain":              {closed: false, dead: CustodyDeathProven, reason: "the chain is open"},
		"a live round":               {closed: true, dead: CustodyDeathAlive, reason: "still runs"},
		"an unproven custody":        {closed: true, dead: CustodyDeathDeferred, reason: "custody is not proven dead"},
		"edits after mirroring":      {closed: true, dead: CustodyDeathProven, dirty: true, reason: "uncommitted, untracked or ignored"},
		"a chain without its record": {closed: true, dead: CustodyDeathProven, records: true, reason: "no job records"},
		"a live process inside":      {closed: true, dead: CustodyDeathProven, holder: true, reason: "in use by pid 4242"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newDelegateBed(t, c.closed)
			if c.dirty {
				if err := os.WriteFile(filepath.Join(bed.worktree, "later.txt"), []byte("edited after the mirror\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if c.records {
				if err := os.Remove(filepath.Join(bed.repo, "artifacts", "agents", "jobs", bed.chain+".json")); err != nil {
					t.Fatal(err)
				}
			}
			holder := ""
			if c.holder {
				holder = bed.worktree
			}
			report := bed.pass(t, bed.proof(c.dead), cleanCensus(holder))
			bed.kept(t, report, c.reason)
			bed.borrowed(t, !(c.dirty || c.records))
			if c.dirty || c.records {
				bed.absorbed(t)
			}
		})
	}
}

// A person's discard releases a dirty workspace for that invocation alone,
// archiving every tip, and leaves no authority behind: the record carries
// the note, never an authorized discard.
func TestAPersonsDiscardReleasesOnlyForItsInvocation(t *testing.T) {
	t.Parallel()
	bed := newDelegateBed(t, true)
	if err := os.WriteFile(filepath.Join(bed.worktree, "later.txt"), []byte("abandoned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	census := diskstore.TakeUseCensus(context.Background(), *cleanCensus(""))
	verdict, err := bed.proof(CustodyDeathProven).ReleaseDiscarded(context.Background(), bed.registry, bed.record.ID, &census,
		diskstore.Discard{By: "Wido", At: delegateNow, Reason: "abandoned bed"})
	if err != nil || verdict.Decision != diskstore.Release {
		t.Fatalf("the discard releases it: %+v %v", verdict, err)
	}
	record, err := bed.registry.Load(bed.record.ID)
	if err != nil || record.State != diskstore.StateReleased || record.AuthorizedDiscard != nil || !strings.Contains(strings.Join(record.Notes, " "), "Wido") {
		t.Fatalf("the discard is history on the record, never authority: %+v %v", record, err)
	}
	if archived, err := delegateGit(context.Background(), bed.repo, "rev-parse", "refs/archive/delegate-"+bed.chain+"/agent/"+bed.chain); err != nil ||
		strings.TrimSpace(string(archived)) != bed.commit {
		t.Fatalf("committed work is archived even under a discard: %q %v", archived, err)
	}
	// A repeat is success and writes nothing.
	if verdict, err := bed.proof(CustodyDeathProven).ReleaseDiscarded(context.Background(), bed.registry, bed.record.ID, &census,
		diskstore.Discard{By: "Wido", At: delegateNow, Reason: "again"}); err != nil || verdict.Decision != diskstore.Release {
		t.Fatalf("a repeat succeeds: %+v %v", verdict, err)
	}
}

// Round D2 F-3: chainMembers skips a record it cannot read, and the
// skipped record may be the one that names a round of this chain; any
// unreadable job record under jobs/ therefore holds every delegate
// workspace for the pass.
func TestAnUnreadableJobRecordHoldsTheDelegateClass(t *testing.T) {
	t.Parallel()
	bed := newDelegateBed(t, true)
	if err := os.WriteFile(filepath.Join(bed.repo, "artifacts", "agents", "jobs", bed.chain+"-r2.json"), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := bed.pass(t, bed.proof(CustodyDeathProven), cleanCensus(""))
	bed.kept(t, report, "cannot be read")
}

func (bed delegateBed) borrowed(t *testing.T, want bool) {
	t.Helper()
	alternates, err := os.ReadFile(filepath.Join(bed.repo, ".git", "objects", "info", "alternates"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if got := strings.Contains(string(alternates), bed.quarantine); got != want {
		t.Fatalf("borrowed = %v, want %v: %s", got, want, alternates)
	}
}

// Compare the quarantine's actual compressed objects with their common-store
// copies; reading through git's alternates cannot satisfy this assertion.
func (bed delegateBed) absorbed(t *testing.T) {
	t.Helper()
	bed.borrowed(t, false)
	count := 0
	err := filepath.WalkDir(bed.quarantine, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(bed.quarantine, path)
		if err != nil {
			return err
		}
		if entry.IsDir() || len(filepath.Dir(rel)) != 2 || len(entry.Name()) != 38 {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target, err := os.ReadFile(filepath.Join(bed.repo, ".git", "objects", rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(source, target) {
			t.Errorf("common object differs: %s", rel)
		}
		count++
		return nil
	})
	if err != nil || count == 0 {
		t.Fatalf("verified %d objects: %v", count, err)
	}
}

func TestDelegateAbsorbPreviewAndReportAreReadOnly(t *testing.T) {
	t.Parallel()
	for _, mode := range []diskstore.Mode{diskstore.ModePreview, diskstore.ModeReport} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			bed := newDelegateBed(t, true)
			if err := os.WriteFile(filepath.Join(bed.worktree, "dirty"), []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(filepath.Join(bed.quarantine, bed.commit[:2], bed.commit[2:]))
			if err != nil {
				t.Fatal(err)
			}
			report := bed.passMode(t, bed.proof(CustodyDeathProven), cleanCensus(""), mode)
			bed.borrowed(t, true)
			if _, err := os.Stat(filepath.Join(bed.repo, ".git", "objects", bed.commit[:2], bed.commit[2:])); !os.IsNotExist(err) {
				t.Fatalf("preview published an object: %v", err)
			}
			after, err := os.ReadFile(filepath.Join(bed.quarantine, bed.commit[:2], bed.commit[2:]))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("quarantine changed: %v", err)
			}
			if len(report.Kept) != 1 || !report.Kept[0].Item.Verdict.Absorbable || !strings.Contains(strings.Join(report.VerboseLines(), "\n"), "absorbable: its chain is closed and custody dead") {
				t.Fatalf("eligibility missing: %+v", report)
			}
		})
	}
}

func TestDelegateAbsorbsBeforeDurabilityAndContentKeeps(t *testing.T) {
	t.Parallel()
	for _, durable := range []bool{false, true} {
		t.Run(map[bool]string{false: "durability pending", true: "content kept"}[durable], func(t *testing.T) {
			t.Parallel()
			bed := newDelegateBed(t, true)
			proof := bed.proof(CustodyDeathProven)
			if durable {
				if err := os.WriteFile(filepath.Join(bed.worktree, "dirty"), []byte("keep"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				proof.Closed = func(string, string) error { return errors.New("fixture capture missing") }
			}
			observed := proof.Observe(context.Background(), bed.record)
			if !observed.Absorbable {
				t.Fatalf("not absorbable: %+v", observed)
			}
			bed.borrowed(t, true)
			report := bed.pass(t, proof, cleanCensus(""))
			reason := "uncommitted"
			if !durable {
				reason = "evidence is not durable (fixture capture missing)"
				if len(report.Pending) != 1 {
					t.Fatalf("not pending: %+v", report)
				}
			}
			bed.kept(t, report, reason)
			bed.absorbed(t)
		})
	}
}

func TestDelegateAbsorbRequiresFreshCompleteWorkspaceAndQuarantineCensus(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"workspace", "late entrant", "quarantine file", "incomplete", "not taken"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newDelegateBed(t, true)
			if err := os.WriteFile(filepath.Join(bed.worktree, "dirty"), []byte("keep"), 0o644); err != nil {
				t.Fatal(err)
			}
			reader := cleanCensus("")
			reads := 0
			reader.Use = func(int64) (identity.ProcessUse, error) {
				reads++
				switch scenario {
				case "workspace":
					return identity.ProcessUse{Cwd: bed.worktree}, nil
				case "late entrant":
					if reads > 1 {
						return identity.ProcessUse{Cwd: bed.worktree}, nil
					}
				case "quarantine file":
					return identity.ProcessUse{Cwd: "/", Files: []string{filepath.Join(bed.quarantine, bed.commit[:2], bed.commit[2:])}}, nil
				case "incomplete":
					return identity.ProcessUse{}, errors.New("fixture permission denied")
				}
				return identity.ProcessUse{Cwd: "/"}, nil
			}
			if scenario == "not taken" {
				reader.Pids = func() ([]int64, error) { return nil, errors.New("fixture process table denied") }
			}
			report := bed.pass(t, bed.proof(CustodyDeathProven), reader)
			want := "in use by pid 4242"
			if scenario == "incomplete" {
				want = "fixture permission denied"
			}
			if scenario == "not taken" {
				want = "fixture process table denied"
			}
			if len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, want) || !strings.Contains(report.Pending[0].Reason, "the repository borrows its objects; remove it only through metasystem disk clean") || !report.Pending[0].Item.Verdict.Absorbable {
				t.Fatalf("pending cause %q: %+v", want, report)
			}
			bed.borrowed(t, true)
			if _, err := os.Stat(filepath.Join(bed.repo, ".git", "objects", bed.commit[:2], bed.commit[2:])); !os.IsNotExist(err) {
				t.Fatalf("published while in use: %v", err)
			}
		})
	}
}

func TestDelegateAbsorbFailureKeepsAlternateAndWorkspace(t *testing.T) {
	t.Parallel()
	bed := newDelegateBed(t, true)
	object := filepath.Join(bed.quarantine, "aa", strings.Repeat("b", 38))
	if err := os.MkdirAll(filepath.Dir(object), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := bed.pass(t, bed.proof(CustodyDeathProven), cleanCensus(""))
	if len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, "quarantine could not be absorbed") || !strings.Contains(report.Pending[0].Reason, "the repository borrows its objects; remove it only through metasystem disk clean") || !report.Pending[0].Item.Verdict.Absorbable {
		t.Fatalf("corruption not reported: %+v", report)
	}
	bed.borrowed(t, true)
	bed.kept(t, report, "could not be absorbed")
}

func TestDelegateAbsorbWaitsForEveryRoundAndBorrower(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"live follow-up", "another borrower"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newDelegateBed(t, true)
			proof := bed.proof(CustodyDeathProven)
			reason := "still runs"
			if scenario == "live follow-up" {
				path := filepath.Join(bed.repo, "artifacts", "agents", "jobs", "round-2.json")
				if err := os.WriteFile(path, []byte(`{"jobId":"round-2","parentJob":"`+bed.chain+`"}`), 0o644); err != nil {
					t.Fatal(err)
				}
				proof.CustodyDead = func(record map[string]any) CustodyDeathResult {
					if asString(record["jobId"]) == "round-2" {
						return CustodyDeathResult{Outcome: CustodyDeathAlive, Reason: "follow-up lives"}
					}
					return CustodyDeathResult{Outcome: CustodyDeathProven}
				}
			} else {
				borrower := filepath.Join(bed.repo, ".git", "worktrees", "borrower", diskstore.QuarantineName, "info", "alternates")
				if err := os.MkdirAll(filepath.Dir(borrower), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(borrower, []byte(bed.quarantine+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				reason = "still borrowed through"
			}
			report := bed.pass(t, proof, cleanCensus(""))
			bed.kept(t, report, reason)
			bed.borrowed(t, true)
			if _, err := os.Stat(filepath.Join(bed.repo, ".git", "objects", bed.commit[:2], bed.commit[2:])); !os.IsNotExist(err) {
				t.Fatalf("published before eligibility: %v", err)
			}
		})
	}
}

func TestDelegatePreviewNamesUseWithoutMutating(t *testing.T) {
	t.Parallel()
	bed := newDelegateBed(t, true)
	report := bed.passMode(t, bed.proof(CustodyDeathProven), cleanCensus(bed.worktree), diskstore.ModePreview)
	if len(report.Pending) != 1 || !report.Pending[0].Item.Verdict.Absorbable || !strings.Contains(report.Pending[0].Reason, "in use by pid 4242") || !strings.Contains(report.Pending[0].Reason, "the repository borrows its objects; remove it only through metasystem disk clean") {
		t.Fatalf("preview does not name the user and retained objects: %+v", report)
	}
	if len(report.Planned) != 0 {
		t.Fatalf("a used workspace was planned for release: %+v", report.Planned)
	}
	bed.borrowed(t, true)
	if _, err := os.Stat(filepath.Join(bed.repo, ".git", "objects", bed.commit[:2], bed.commit[2:])); !os.IsNotExist(err) {
		t.Fatalf("preview published objects: %v", err)
	}
}
