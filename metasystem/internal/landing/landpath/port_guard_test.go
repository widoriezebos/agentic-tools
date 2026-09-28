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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

const guardTree = "0123456789abcdef0123456789abcdef01234567"

// guardBed is one guard invocation: a staged set (path to A or M), an
// optional unborn HEAD, and owners that record what the guard asked.
type guardBed struct {
	t            *testing.T
	root         string
	git          *fakeGit
	staged       [][2]string
	unborn       bool
	owners       GuardOwners
	observations []string
	tokenChecks  int
	stdout       bytes.Buffer
	stderr       bytes.Buffer
}

func newGuardBed(t *testing.T) *guardBed {
	t.Helper()
	root := t.TempDir()
	g := &guardBed{t: t, root: root, git: newFakeGit(t, root)}
	g.git.tree = guardTree
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
