package testgoal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// PublicationFixture exercises the Git candidate-to-filesystem adapter. A stub
// cannot establish that a detached commit, rather than the caller's files, was
// read. All records and refs belong to this temporary repository.
type PublicationFixture struct {
	T               *testing.T
	Root, Top, Base string
	Now             time.Time
}

func NewPublicationFixture(t *testing.T) *PublicationFixture {
	t.Helper()
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &PublicationFixture{T: t, Top: top, Root: filepath.Join(top, "metasystem"), Now: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)}
	f.Git("init", "-q", "-b", "main")
	f.Git("config", "user.name", "fixture")
	f.Git("config", "user.email", "fixture@invalid")
	f.Git("config", "core.hooksPath", "/dev/null")
	f.Git("config", "goal.sync-remote", "local")
	f.Git("config", "goal.sync-branch", "refs/heads/main")
	f.Git("config", "metasystem.goal.machine", "fixture")
	f.Write("metasystem.conf", "metasystem.template=true\nmetasystem.runtimes=fake\n")
	f.Write("scripts/agents/.keep", "")
	f.Write("plans/goals/backlog.md", string(goal.RenderRoot(&goal.RootRecord{Identity: "01J5X000000000000000000000", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})))
	f.Write("plans/goals/held.md", string(goal.RenderFile(&goal.GoalFile{
		Id: "held", State: goal.StateClaimed, Intent: "Finish the fixture", Origin: goal.OriginMain, OpenedAt: "2026-10-14T12:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: "fixture", Lineage: "session", At: "2026-10-14T12:00:00Z"},
		History: []goal.HistoryLine{{At: "2026-10-14T12:00:00Z", Opid: goal.Opid("01J5X000000000000000000001", "fixture", "session"), Verb: "claim", Actor: "fixture+session", Targets: []string{"held"}, Keep: -1}},
	})))
	f.Base = f.Commit()
	f.Git("update-ref", goal.LocalLedgerBranch, f.Base)
	f.Git("update-ref", goal.AcceptedRef, f.Base)
	if err := goal.ValidateCommit(f.Root, f.Base); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *PublicationFixture) Git(args ...string) string {
	f.T.Helper()
	command := exec.Command("git", append([]string{"-C", f.Top}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	out, err := command.CombinedOutput()
	if err != nil {
		f.T.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *PublicationFixture) Write(path, data string) {
	f.T.Helper()
	name := filepath.Join(f.Root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		f.T.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0644); err != nil {
		f.T.Fatal(err)
	}
}

func (f *PublicationFixture) Commit() string {
	f.T.Helper()
	f.Git("add", ".")
	f.Git("commit", "-qm", "fixture")
	return f.Git("rev-parse", "HEAD")
}

// Candidate leaves the caller at its original valid checkout while advancing
// the fetched ledger tip to a complete candidate with the named reference.
func (f *PublicationFixture) Candidate(reference string) string {
	f.T.Helper()
	f.Write("plans/designs/change.md", "# Change\n\n- Kind: design\n- Id: change\n- Status: accepted\n- Goals: "+reference+"\n")
	candidate := f.Commit()
	f.Git("update-ref", goal.LocalLedgerBranch, candidate)
	f.Git("checkout", "-q", "--detach", f.Base)
	return candidate
}

func (f *PublicationFixture) Request(e goal.Endpoint) goal.VerbRequest {
	return goal.VerbRequest{Endpoint: e, Actor: goal.Actor{Machine: "fixture", Lineage: "session"}, Ulid: "01J5X000000000000000000002", Now: f.Now}
}

func (f *PublicationFixture) CountPublications(e goal.Endpoint, count *int) goal.Endpoint {
	return e.WithCASPublisher(func(_ goal.Endpoint, tip, commit string) (goal.CASOutcome, error) {
		*count++
		f.Git("update-ref", goal.LocalLedgerBranch, commit, tip)
		return goal.CASLanded, nil
	})
}

func (f *PublicationFixture) AssertRefusal(err error) {
	f.T.Helper()
	want := "the design page change.md names goal missing, which is not in the ledger; the push waits — open the goal or fix the page"
	if err == nil || !strings.Contains(strings.Join(strings.Fields(err.Error()), " "), want) {
		f.T.Fatalf("refusal = %v; want %s", err, want)
	}
	if strings.Contains(err.Error(), "plans/designs/") {
		f.T.Fatalf("refusal exposes checker path: %v", err)
	}
}
