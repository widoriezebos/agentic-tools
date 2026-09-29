package landpath

// Ports of scripts/agents/pre-commit-guard-fixtures.sh: every case runs the
// guard body against a scripted staged set and fake owners. Git is the
// per-test fakeGit; the classifier, the wrapper-token proof and the
// observation log are fields each test sets.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

const guardTree = "0123456789abcdef0123456789abcdef01234567"

// guardBed is one guard invocation: a staged set (path to A or M), an
// optional unborn HEAD, and owners that record what the guard asked.
type guardBed struct {
	t      *testing.T
	root   string
	git    *fakeGit
	staged [][2]string
	unborn bool
	// syncBranch is goal.sync-branch ("" unset), syncConfigCode a config
	// read failure other than unset; commonDir is git's common directory.
	syncBranch     string
	syncConfigCode int
	commonDir      string
	// gitDir and dotGit are git's answers for --git-dir and for the work
	// tree's own .git entry ("" is the common dir); the codes fail each of
	// the three answers.
	gitDir, dotGit                        string
	dotGitCode, gitDirCode, commonDirCode int
	helmState                             helm.State
	helmReads                             int
	yields                                []helm.Yield
	owners                                GuardOwners
	observations                          []string
	tokenChecks                           int
	stdout                                bytes.Buffer
	stderr                                bytes.Buffer
}

func newGuardBed(t *testing.T) *guardBed {
	t.Helper()
	root := t.TempDir()
	g := &guardBed{t: t, root: root, git: newFakeGit(t, root)}
	g.git.tree = guardTree
	g.commonDir = filepath.Join(root, ".git")
	g.git.on("symbolic-ref --quiet HEAD", func(GitCall) GitResult {
		if g.git.branch == "" {
			return failed(1, "")
		}
		return ok("refs/heads/" + g.git.branch + "\n")
	})
	g.git.on("config --get goal.sync-branch", func(GitCall) GitResult {
		if g.syncConfigCode != 0 {
			return failed(g.syncConfigCode, "error: bad config\n")
		}
		if g.syncBranch == "" {
			return failed(1, "")
		}
		return ok(g.syncBranch + "\n")
	})
	g.git.on("rev-parse --path-format=absolute --git-common-dir", func(GitCall) GitResult {
		if g.commonDirCode != 0 {
			return failed(g.commonDirCode, "fatal: not a git repository\n")
		}
		return ok(g.commonDir + "\n")
	})
	g.git.on("rev-parse --path-format=absolute --git-dir", func(GitCall) GitResult {
		if g.gitDirCode != 0 {
			return failed(g.gitDirCode, "fatal: not a git repository\n")
		}
		return ok(orDefault(g.gitDir, g.commonDir) + "\n")
	})
	g.git.on("rev-parse --path-format=absolute --resolve-git-dir", func(call GitCall) GitResult {
		if len(call.Args) != 4 || call.Args[3] != filepath.Join(g.root, ".git") {
			t.Fatalf("resolve-git-dir asked for %q, want the work tree's .git", call.Args[3:])
		}
		if g.dotGitCode != 0 {
			return failed(g.dotGitCode, "fatal: not a gitdir\n")
		}
		return ok(orDefault(g.dotGit, orDefault(g.gitDir, g.commonDir)) + "\n")
	})
	g.git.on("diff --cached --name-only", func(GitCall) GitResult { return ok(g.names("")) })
	g.git.on("diff --cached --name-only --diff-filter=AM", func(GitCall) GitResult { return ok(g.names("AM")) })
	g.git.on("diff --cached --name-status --diff-filter=A", func(GitCall) GitResult {
		var out strings.Builder
		for _, entry := range g.staged {
			if entry[1] == "A" {
				out.WriteString("A\t" + entry[0] + "\n")
			}
		}
		return ok(out.String())
	})
	g.git.on("rev-parse --verify HEAD", func(GitCall) GitResult {
		if g.unborn {
			return failed(128, "fatal: Needed a single revision\n")
		}
		return ok(g.git.head + "\n")
	})
	g.owners = GuardOwners{
		Git:       g.git.run,
		CallerPID: 900,
		// The fixture's refusing classifier stub: strict classification
		// fails, and the guard must fail open to HUMAN.
		Classify: func(string, int64) (string, error) { return "", errors.New("registry unreadable") },
		WrapperToken: func(string, int64) bool {
			g.tokenChecks++
			return false
		},
		AppendObservation: func(_, line string) error {
			g.observations = append(g.observations, line)
			return nil
		},
	}
	return g
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (g *guardBed) stage(status string, paths ...string) {
	for _, path := range paths {
		g.staged = append(g.staged, [2]string{path, status})
	}
}

func (g *guardBed) names(filter string) string {
	var out strings.Builder
	for _, entry := range g.staged {
		if filter == "" || strings.Contains(filter, entry[1]) {
			out.WriteString(entry[0] + "\n")
		}
	}
	return out.String()
}

func (g *guardBed) run() int {
	g.t.Helper()
	return Guard(g.owners, g.root, g.root, &g.stdout, &g.stderr)
}

func (g *guardBed) expect(status, want int, texts ...string) {
	g.t.Helper()
	if status != want {
		g.t.Fatalf("guard status %d, want %d\nstdout:\n%s\nstderr:\n%s", status, want, g.stdout.String(), g.stderr.String())
	}
	for _, text := range texts {
		if !strings.Contains(g.stderr.String()+g.stdout.String(), text) {
			g.t.Fatalf("guard output lacks %q\nstdout:\n%s\nstderr:\n%s", text, g.stdout.String(), g.stderr.String())
		}
	}
}

// TestGuardClassifierFailureAdmitsHumanAndRecordsObservation ports the
// fixture's first leg: a failing classifier gates nothing (the commit is
// treated as human, no wrapper token is demanded) and leaves one durable
// would-refuse observation naming the full index tree; an empty class is the
// same unavailable decision, and a tree that is not a full object id is
// recorded as unknown.
func TestGuardClassifierFailureAdmitsHumanAndRecordsObservation(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.stage("M", "tracked.txt")
	g.expect(g.run(), 0)
	want := "schemaVersion=1 boundary=pre-commit tree=" + guardTree + " verdict=would-refuse code=classifier-unavailable"
	if len(g.observations) != 1 || g.observations[0] != want || g.tokenChecks != 0 {
		t.Fatalf("observations %q token checks %d", g.observations, g.tokenChecks)
	}

	g = newGuardBed(t)
	g.stage("M", "tracked.txt")
	g.owners.Classify = func(string, int64) (string, error) { return "", nil }
	g.git.tree = "not-a-tree"
	g.expect(g.run(), 0)
	if len(g.observations) != 1 || !strings.Contains(g.observations[0], " tree=unknown ") || g.tokenChecks != 0 {
		t.Fatalf("empty class observations %q token checks %d", g.observations, g.tokenChecks)
	}
}

// TestGuardObservationStorageFailureStaysNonRefusing ports the directory
// and write failure legs: the commit is still admitted, and the failure is
// operator-visible with the owner's cause.
func TestGuardObservationStorageFailureStaysNonRefusing(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"its observation directory could not be created", "its observation could not be written"} {
		t.Run(cause, func(t *testing.T) {
			g := newGuardBed(t)
			g.stage("M", "tracked.txt")
			g.owners.AppendObservation = func(string, string) error { return errors.New(cause) }
			g.expect(g.run(), 0, "pre-commit guard: classifier unavailable and "+cause)
		})
	}
}

// TestGuardRefusesPatchBackups: a staged .orig is refused whether it is
// added or modifies a tracked path.
func TestGuardRefusesPatchBackups(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"A", "M"} {
		g := newGuardBed(t)
		g.stage(status, "scratch.orig", "ordinary.txt")
		g.expect(g.run(), 1, "pre-commit guard: refusing scratch.orig: patch backups are never tracked")
		if strings.Contains(g.stderr.String(), "ordinary.txt") {
			t.Fatalf("an ordinary path was named: %s", g.stderr.String())
		}
	}
}

// TestGuardAdmitsAnOrdinaryStagedFile: the fixture's ordinary leg.
func TestGuardAdmitsAnOrdinaryStagedFile(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.stage("A", "ordinary.txt")
	g.expect(g.run(), 0)
	if g.stderr.Len() != 0 {
		t.Fatalf("an admitted commit wrote to stderr: %s", g.stderr.String())
	}
}

// TestGuardNewPlanNeedsAcknowledgment: the new-plan safeguard stays active
// after the classifier fails open, and the acknowledgment admits a new plan
// outside the ledger.
func TestGuardNewPlanNeedsAcknowledgment(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.stage("A", "plans/new.md")
	g.expect(g.run(), 1, "pre-commit guard: refusing to commit NEW plan file(s):\n  plans/new.md\n", "METASYSTEM_ALLOW_NEW_PLAN=1 git commit ...")

	g = newGuardBed(t)
	g.stage("M", "plans/existing.md")
	g.expect(g.run(), 0)

	g = newGuardBed(t)
	g.stage("A", "plans/new.md")
	g.owners.AllowNewPlan = true
	g.expect(g.run(), 0)
}

// TestGuardLedgerFenceOutranksBothExceptions: a staged goal or channel
// ledger path refuses under the new-plan acknowledgment and on an unborn
// HEAD (F15); the unborn exception still admits an initial payload with a
// plan in it.
func TestGuardLedgerFenceOutranksBothExceptions(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"plans/goals/smuggled.md", "plans/channel/smuggled.json"} {
		g := newGuardBed(t)
		g.stage("A", path)
		g.owners.AllowNewPlan = true
		g.expect(g.run(), 1, "pre-commit guard: goal files change only through goal verbs", "  "+path+"\n")
	}

	g := newGuardBed(t)
	g.unborn = true
	g.stage("A", "plans/goals/smuggled.md")
	g.expect(g.run(), 1, "goal files change only through goal verbs")

	g = newGuardBed(t)
	g.unborn = true
	g.stage("A", "payload.txt", "plans/initial.md")
	g.expect(g.run(), 0)
}

// TestGuardProbeAndAgentWrapperProof covers the guard's other exits: the
// enrollment probe answers its nonce with the distinct status before any git
// call, an agent class needs the live wrapper token for the one token path,
// and a human class is never asked for it.
func TestGuardProbeAndAgentWrapperProof(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.owners.Probe = "n0nce"
	g.expect(g.run(), GuardProbeStatus, "guard-probe-ack n0nce")
	if len(g.git.calls) != 0 {
		t.Fatalf("the probe ran git: %v", g.git.calls)
	}

	g = newGuardBed(t)
	g.owners.Classify = func(string, int64) (string, error) { return "DELEGATE", nil }
	var asked string
	var askedCaller int64
	g.owners.WrapperToken = func(token string, caller int64) bool {
		asked, askedCaller = token, caller
		return false
	}
	g.stage("A", "ordinary.txt")
	g.expect(g.run(), 1, "the live wrapper ancestry token is missing")
	if asked != TokenPath(g.root) || askedCaller != 900 || len(g.observations) != 0 {
		t.Fatalf("token path %q caller %d observations %q", asked, askedCaller, g.observations)
	}

	g = newGuardBed(t)
	g.owners.Classify = func(string, int64) (string, error) { return "DELEGATE", nil }
	g.owners.WrapperToken = func(string, int64) bool { return true }
	g.stage("A", "ordinary.txt")
	g.expect(g.run(), 0)

	g = newGuardBed(t)
	g.owners.Classify = func(string, int64) (string, error) { return "HUMAN", nil }
	g.stage("A", "ordinary.txt")
	g.expect(g.run(), 0)
	if g.tokenChecks != 0 || len(g.observations) != 0 {
		t.Fatalf("a human commit was asked for a token or observed: %d %q", g.tokenChecks, g.observations)
	}
}

// guardProcessTree is a fixed ancestry for the wrapper-token proof.
type guardProcessTree struct {
	parents map[int64]int64
	starts  map[int64]int64
}

func (tree guardProcessTree) ParentPid(pid int64) (int64, bool) {
	parent, found := tree.parents[pid]
	return parent, found
}

func (tree guardProcessTree) StartedAtSec(pid int64) (int64, bool) {
	start, found := tree.starts[pid]
	return start, found
}

// TestGuardAcceptsTheTokenCommitWrites: the token the commit boundary mints
// through Owners.WriteToken, written the way the production owner writes it,
// is accepted by validate.WrapperToken for a guard running under git under
// the landing process, while the commit runs; a recycled wrapper pid is not.
// It is removed once the commit concludes.
func TestGuardAcceptsTheTokenCommitWrites(t *testing.T) {
	t.Parallel()
	const landingPid, gitPid, guardPid, started = 4242, 4300, 4301, 1700000123
	for _, c := range []struct {
		name     string
		start    int64
		accepted bool
	}{{"live wrapper", started, true}, {"recycled pid", started + 1, false}} {
		t.Run(c.name, func(t *testing.T) {
			b := newBed(t)
			b.epoch = epochOf(3)
			b.owners.Getpid = func() int64 { return landingPid }
			b.owners.StartedAt = func(pid int64) (int64, error) {
				if pid != landingPid {
					t.Fatalf("start time asked for pid %d", pid)
				}
				return started, nil
			}
			b.owners.WriteToken = func(path string, token WrapperToken) error {
				encoded, err := json.MarshalIndent(token, "", "  ")
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				return os.WriteFile(path, append(encoded, '\n'), 0o644)
			}
			b.owners.RemoveFile = os.Remove
			tree := guardProcessTree{parents: map[int64]int64{guardPid: gitPid, gitPid: landingPid, landingPid: 1},
				starts: map[int64]int64{landingPid: c.start}}
			guardStatus := -1
			b.git.on("commit", func(call GitCall) GitResult {
				// The pre-commit hook runs inside git commit: run the guard
				// body there with the production token proof.
				guard := newGuardBed(t)
				guard.root = b.root
				guard.stage("A", "ordinary.txt")
				guard.owners.CallerPID = guardPid
				guard.owners.Classify = func(string, int64) (string, error) { return "DELEGATE", nil }
				guard.owners.WrapperToken = func(token string, caller int64) bool {
					return validate.WrapperToken(token, caller, tree)
				}
				guardStatus = guard.run()
				if guardStatus != 0 {
					b.stderr.WriteString(guard.stderr.String())
					return failed(1, "")
				}
				return b.git.commit(call)
			})
			status := b.commit(CommitRequest{OwnerLineage: "L"})
			if c.accepted {
				b.expect(status, 0)
			} else {
				b.expect(status, 1, "the live wrapper ancestry token is missing")
			}
			if (guardStatus == 0) != c.accepted {
				t.Fatalf("guard status %d, want accepted=%t", guardStatus, c.accepted)
			}
			if _, err := os.Stat(TokenPath(b.root)); !os.IsNotExist(err) {
				t.Fatalf("the wrapper token outlived the commit: %v", err)
			}
		})
	}
}

// agentOn is a guard bed for a DELEGATE commit on branch ("" is a detached
// HEAD) of a checkout no seat holds, whose token proof always fails.
func agentOn(t *testing.T, branch string) *guardBed {
	t.Helper()
	g := newGuardBed(t)
	g.git.branch = branch
	g.owners.Classify = func(string, int64) (string, error) { return "DELEGATE", nil }
	g.stage("A", "ordinary.txt")
	return g
}

// TestGuardWrapperFenceCoversOnlyWhatItProtects: the wrapper token (D-6 of
// "One writer, safe readers", d2d33e9fb) protects the write role a seat holds
// on its checkout and the published line the ledger and the landing path
// write to. An agent commit on a feature branch of a clone no seat holds
// (an integration or builder clone) damages neither, so it needs no token;
// the published branch, the ledger branch, a detached or unreadable HEAD, a
// seat checkout and a linked worktree of one stay fenced.
func TestGuardWrapperFenceCoversOnlyWhatItProtects(t *testing.T) {
	t.Parallel()
	g := agentOn(t, "mfix2")
	g.expect(g.run(), 0)
	if g.tokenChecks != 0 || g.stderr.Len() != 0 {
		t.Fatalf("a feature-branch commit in an unheld clone was fenced: checks %d stderr %q", g.tokenChecks, g.stderr.String())
	}

	// The configured sync branch replaces the default: main is then an
	// ordinary branch, the configured one is the published line.
	g = agentOn(t, "main")
	g.syncBranch = "refs/heads/trunk"
	g.expect(g.run(), 0)

	for _, c := range []struct {
		name   string
		branch string
		setup  func(*guardBed)
		reason string
	}{
		{"default published branch", "main", nil, "refs/heads/main is the published line"},
		{"configured published branch", "trunk", func(g *guardBed) { g.syncBranch = "refs/heads/trunk" }, "refs/heads/trunk is the published line"},
		{"ledger branch", "metasystem/goals", nil, "refs/heads/metasystem/goals is the published line"},
		{"detached HEAD", "", nil, "HEAD names no branch"},
		{"unreadable sync branch", "feature", func(g *guardBed) { g.syncConfigCode = 3 }, "goal.sync-branch cannot be read"},
		{"unqualified sync branch", "feature", func(g *guardBed) { g.syncBranch = "feature" }, "goal.sync-branch cannot be read"},
		{"seat checkout", "feature", func(g *guardBed) {
			if err := os.MkdirAll(filepath.Join(g.root, "artifacts", "agents", "mains"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "a seat holds this checkout"},
		{"linked worktree of a seat checkout", "feature", func(g *guardBed) {
			primary := t.TempDir()
			if err := os.MkdirAll(filepath.Join(primary, "artifacts", "agents", "mains"), 0o755); err != nil {
				t.Fatal(err)
			}
			g.commonDir = filepath.Join(primary, ".git")
		}, "a seat holds this checkout"},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := agentOn(t, c.branch)
			if c.setup != nil {
				c.setup(g)
			}
			g.expect(g.run(), 1, "the live wrapper ancestry token is missing", c.reason, "metasystem work land")
			if g.tokenChecks != 1 {
				t.Fatalf("token checks %d, want 1", g.tokenChecks)
			}
		})
	}
}

// TestGuardUnheldCloneKeepsTheOtherFences: relaxing the wrapper fence on a
// feature branch leaves the ledger, patch-backup and new-plan refusals whole.
func TestGuardUnheldCloneKeepsTheOtherFences(t *testing.T) {
	t.Parallel()
	g := agentOn(t, "feature")
	g.stage("A", "plans/goals/smuggled.md")
	g.expect(g.run(), 1, "goal files change only through goal verbs")
	g = agentOn(t, "feature")
	g.stage("A", "scratch.orig")
	g.expect(g.run(), 1, "patch backups are never tracked")
	g = agentOn(t, "feature")
	g.stage("A", "plans/new.md")
	g.expect(g.run(), 1, "refusing to commit NEW plan file(s)")
}

// atHelm puts the bed at the helm (By "wido") in its primary checkout: the
// work tree's .git entry, git dir and common dir are one directory, which
// exists so each answer resolves. Yields are collected in g.yields.
func (g *guardBed) atHelm() {
	g.t.Helper()
	if err := os.MkdirAll(g.commonDir, 0o755); err != nil {
		g.t.Fatal(err)
	}
	g.helmState = helm.State{Active: true, Record: helm.Record{By: "wido"}}
	g.owners.Helm = func(workTree string) helm.State {
		g.helmReads++
		if workTree != g.root {
			g.t.Fatalf("helm read for %s, want the work tree %s", workTree, g.root)
		}
		return g.helmState
	}
	g.owners.HelmYield = func(workTree string, y helm.Yield) {
		if workTree != g.root {
			g.t.Fatalf("yield recorded for %s, want the work tree %s", workTree, g.root)
		}
		g.yields = append(g.yields, y)
	}
}

// mkdir creates a directory the bed's git answers name.
func (g *guardBed) mkdir(path string) string {
	g.t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		g.t.Fatal(err)
	}
	return path
}

// expectYields asserts the gates of the recorded yields, in order.
func (g *guardBed) expectYields(gates ...string) {
	g.t.Helper()
	if len(g.yields) != len(gates) {
		g.t.Fatalf("yields %+v, want gates %q\nstderr:\n%s", g.yields, gates, g.stderr.String())
	}
	for i, gate := range gates {
		y := g.yields[i]
		if y.Gate != gate || y.Boundary != "pre-commit" || y.Would != "refuse" {
			g.t.Fatalf("yield %d is %+v, want pre-commit %s would refuse", i, y, gate)
		}
	}
}

// helmLine is the one stderr line a yield prints.
func helmLine(who, gate, commonDir string) string {
	return "pre-commit guard: HUMAN AT THE HELM (" + who + "): the " + gate + " yields; recorded in " + filepath.Join(commonDir, "metasystem", "helm-yields.log") + "\n"
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestGuardYieldsTheWrapperFenceAtTheHelmInThePrimaryCheckout: at the helm,
// an agent commit on main in the seat's primary checkout (git dir = common
// dir = the work tree's own .git) is admitted; the wrapper fence records one
// yield naming the branch, the index tree and the class, and prints one line.
func TestGuardYieldsTheWrapperFenceAtTheHelmInThePrimaryCheckout(t *testing.T) {
	t.Parallel()
	g := agentOn(t, "main")
	g.atHelm()
	g.expect(g.run(), 0)
	g.expectYields("wrapper-fence")
	subject := g.yields[0].Subject
	for _, part := range []string{"branch=refs/heads/main", "tree=" + guardTree, "class=DELEGATE"} {
		if !strings.Contains(subject, part) {
			t.Fatalf("subject %q lacks %q", subject, part)
		}
	}
	if want := helmLine("wido", "wrapper-fence", resolved(t, g.commonDir)); g.stderr.String() != want {
		t.Fatalf("stderr %q, want %q", g.stderr.String(), want)
	}
}

// TestGuardAtTheHelmReachesOnlyThePrimaryCheckout: a linked worktree (its git
// dir under the common dir's worktrees/), a caller steering GIT_DIR at the
// primary from a linked worktree (its .git entry still names worktrees/w),
// and a work tree whose .git entry names another repository all refuse as
// today and record nothing; a submodule's primary worktree (git dir = common
// dir = .git/modules/c) is admitted.
func TestGuardAtTheHelmReachesOnlyThePrimaryCheckout(t *testing.T) {
	t.Parallel()
	refused := []struct {
		name  string
		setup func(g *guardBed)
	}{
		{"linked worktree", func(g *guardBed) {
			primary := filepath.Join(t.TempDir(), ".git")
			g.commonDir = primary
			g.gitDir = g.mkdir(filepath.Join(primary, "worktrees", "w"))
		}},
		{"steered", func(g *guardBed) {
			g.dotGit = g.mkdir(filepath.Join(g.commonDir, "worktrees", "w"))
		}},
		{"foreign", func(g *guardBed) {
			g.dotGit = g.mkdir(filepath.Join(t.TempDir(), "other", ".git"))
		}},
		{"unanswered .git entry", func(g *guardBed) { g.dotGitCode = 128 }},
		{"unanswered git dir", func(g *guardBed) { g.gitDirCode = 128 }},
		{"unanswered common dir", func(g *guardBed) { g.commonDirCode = 128 }},
		{"unresolvable git dir", func(g *guardBed) { g.gitDir = filepath.Join(g.root, "absent") }},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			g := agentOn(t, "main")
			g.atHelm()
			c.setup(g)
			g.expect(g.run(), 1, "the live wrapper ancestry token is missing", "refs/heads/main is the published line")
			g.expectYields()
			if strings.Contains(g.stderr.String(), "HUMAN AT THE HELM") {
				t.Fatalf("a refused commit printed the helm line: %s", g.stderr.String())
			}
		})
	}

	t.Run("submodule", func(t *testing.T) {
		g := agentOn(t, "main")
		g.commonDir = filepath.Join(t.TempDir(), ".git", "modules", "c")
		g.atHelm()
		g.expect(g.run(), 0)
		g.expectYields("wrapper-fence")
		if want := helmLine("wido", "wrapper-fence", resolved(t, g.commonDir)); g.stderr.String() != want {
			t.Fatalf("stderr %q, want %q", g.stderr.String(), want)
		}
	})
}

// TestGuardAtTheHelmKeepsTheDamageChecks: the ledger fence and the .orig
// refusal refuse at the helm with today's messages. With the classifier
// unavailable nothing yields; with DELEGATE on main the wrapper fence yields
// first and the damage check then refuses (the guard's order is kept).
func TestGuardAtTheHelmKeepsTheDamageChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		path, message string
	}{
		{"plans/goals/smuggled.md", "goal files change only through goal verbs"},
		{"plans/channel/smuggled.json", "goal files change only through goal verbs"},
		{"scratch.orig", "patch backups are never tracked"},
	} {
		g := newGuardBed(t)
		g.atHelm()
		g.stage("A", c.path)
		g.expect(g.run(), 1, c.message)
		g.expectYields()

		g = agentOn(t, "main")
		g.atHelm()
		g.stage("A", c.path)
		g.expect(g.run(), 1, c.message, "HUMAN AT THE HELM (wido): the wrapper-fence yields")
		g.expectYields("wrapper-fence")
	}
}

// TestGuardWithoutTheHelmRefusesAsToday: no helm owner and an inactive helm
// both leave the wrapper fence and the new-plan acknowledgment refusing, and
// record nothing.
func TestGuardWithoutTheHelmRefusesAsToday(t *testing.T) {
	t.Parallel()
	g := agentOn(t, "main")
	g.expect(g.run(), 1, "the live wrapper ancestry token is missing")
	g = agentOn(t, "main")
	g.atHelm()
	g.helmState = helm.State{}
	g.expect(g.run(), 1, "the live wrapper ancestry token is missing")
	g.expectYields()

	g = newGuardBed(t)
	g.atHelm()
	g.helmState = helm.State{}
	g.stage("A", "plans/new.md")
	g.expect(g.run(), 1, "refusing to commit NEW plan file(s)")
	g.expectYields()
}

// TestGuardYieldsTheNewPlanAcknowledgmentAtTheHelm: a new plan at the helm is
// admitted with one yield; an agent's new plan on main yields both gates, the
// helm read once; the other fences of an unheld clone hold with the helm on.
func TestGuardYieldsTheNewPlanAcknowledgmentAtTheHelm(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.atHelm()
	g.stage("A", "plans/new.md")
	g.expect(g.run(), 0)
	g.expectYields("new-plan-acknowledgment")
	if subject := g.yields[0].Subject; !strings.Contains(subject, "branch=refs/heads/main") || !strings.Contains(subject, "class=unavailable") {
		t.Fatalf("subject %q", subject)
	}
	if want := helmLine("wido", "new-plan-acknowledgment", resolved(t, g.commonDir)); !strings.HasSuffix(g.stderr.String(), want) {
		t.Fatalf("stderr %q lacks %q", g.stderr.String(), want)
	}

	g = agentOn(t, "main")
	g.atHelm()
	g.stage("A", "plans/new.md")
	g.expect(g.run(), 0)
	g.expectYields("wrapper-fence", "new-plan-acknowledgment")
	if g.helmReads != 1 || strings.Count(g.stderr.String(), "HUMAN AT THE HELM") != 2 {
		t.Fatalf("helm reads %d, stderr %q", g.helmReads, g.stderr.String())
	}

	g = agentOn(t, "feature")
	g.atHelm()
	g.stage("A", "plans/goals/smuggled.md")
	g.expect(g.run(), 1, "goal files change only through goal verbs")
	g = agentOn(t, "feature")
	g.atHelm()
	g.stage("A", "scratch.orig")
	g.expect(g.run(), 1, "patch backups are never tracked")
	g.expectYields()
}

// TestGuardAtAMalformedHelmYields: a present signature that cannot be decoded
// is active with By "unknown": the guard admits, and its line says the
// signature is unreadable.
func TestGuardAtAMalformedHelmYields(t *testing.T) {
	t.Parallel()
	g := agentOn(t, "main")
	g.atHelm()
	g.helmState = helm.State{Active: true, Malformed: "helm.json: malformed", Record: helm.Record{By: "unknown"}}
	g.expect(g.run(), 0)
	g.expectYields("wrapper-fence")
	if want := helmLine("signature unreadable", "wrapper-fence", resolved(t, g.commonDir)); g.stderr.String() != want {
		t.Fatalf("stderr %q, want %q", g.stderr.String(), want)
	}
}

// TestGuardClassifierUnavailableAtTheHelmOnlyObserves: the bed's failing
// classifier at the helm admits with today's observation and no yield: the
// helm is consulted only where a gate would refuse.
func TestGuardClassifierUnavailableAtTheHelmOnlyObserves(t *testing.T) {
	t.Parallel()
	g := newGuardBed(t)
	g.atHelm()
	g.stage("M", "tracked.txt")
	g.expect(g.run(), 0)
	g.expectYields()
	if len(g.observations) != 1 || g.helmReads != 0 || g.stderr.Len() != 0 {
		t.Fatalf("observations %q helm reads %d stderr %q", g.observations, g.helmReads, g.stderr.String())
	}
}
