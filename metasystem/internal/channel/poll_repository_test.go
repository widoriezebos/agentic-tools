package channel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type pollCommit struct {
	parent, trailer string
	files           map[string][]byte
	at              time.Time
	published       bool
}

// pollRepository models the channel tests' goal transaction storage, including
// separate canonical and accepted pointers. The goal verbs still own policy.
type pollRepository struct {
	t         *testing.T
	root      string
	endpoint  goal.Endpoint
	commits   map[string]pollCommit
	canonical string
	accepted  string
	serial    int
	captured  map[string]bool
	built     map[string]bool
}

var _ goal.Repository = (*pollRepository)(nil)

func copyPollFiles(files map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for path, b := range files {
		out[path] = append([]byte(nil), b...)
	}
	return out
}

func newPollRepository(t *testing.T, rootBytes, goalBytes []byte) *pollRepository {
	t.Helper()
	f := &pollRepository{t: t, root: t.TempDir(), commits: make(map[string]pollCommit), captured: make(map[string]bool), built: make(map[string]bool)}
	f.endpoint = goal.Endpoint{Root: f.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: f}
	seed := f.nextID()
	f.commits[seed] = pollCommit{files: copyPollFiles(map[string][]byte{
		"plans/goals/backlog.md": rootBytes, "plans/goals/g.md": goalBytes,
	}), at: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), published: true}
	f.canonical, f.accepted = seed, seed
	for path, b := range f.commits[seed].files {
		full := filepath.Join(f.root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte("metasystem.budget.tier-1=1h/3/360m/1/0\nmetasystem.budget.review-round-max=3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for opid, pending := range f.captured {
			if pending {
				t.Errorf("goal capture %s was not released", opid)
			}
		}
		for id, pending := range f.built {
			if pending {
				t.Errorf("goal commit %s was not published", id)
			}
		}
	})
	return f
}

func (f *pollRepository) nextID() string {
	f.serial++
	return fmt.Sprintf("%040x", f.serial)
}

func (f *pollRepository) resolve(root string) (goal.Endpoint, error) {
	if root != f.root {
		return goal.Endpoint{}, fmt.Errorf("unexpected poll root %q", root)
	}
	return f.endpoint, nil
}

func (f *pollRepository) poll(ctx context.Context, c PollConfig) (PollResult, error) {
	return pollWithEndpoint(ctx, c, f.resolve)
}

func (f *pollRepository) Capture(opid string) (string, error) {
	if opid == "" {
		return "", fmt.Errorf("empty capture operation")
	}
	f.captured[opid] = true
	return f.canonical, nil
}

func (f *pollRepository) Accepted() (string, bool, error) {
	if f.accepted == "" {
		return "", false, nil
	}
	if _, ok := f.commits[f.accepted]; !ok {
		return "", false, fmt.Errorf("unknown accepted commit %q", f.accepted)
	}
	return f.accepted, true, nil
}

func (f *pollRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	c, ok := f.commits[commit]
	if !ok {
		return nil, fmt.Errorf("unknown goal commit %q", commit)
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("goal file read without a path")
	}
	for _, prefix := range prefixes {
		switch prefix {
		case "plans/goals/", "plans/goals/backlog.md", "plans/goals/done/", "records/goals/", goal.ChannelPrefix:
		default:
			return nil, fmt.Errorf("undeclared goal file prefix %q", prefix)
		}
	}
	out := make(map[string][]byte)
	for path, b := range c.files {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				out[path] = append([]byte(nil), b...)
				break
			}
		}
	}
	return out, nil
}

func (f *pollRepository) Build(opid, parent string, changes []goal.Change, message string) (string, error) {
	p, ok := f.commits[parent]
	if !ok || !p.published {
		return "", fmt.Errorf("unknown build parent %q", parent)
	}
	if opid == "" || message == "" || len(changes) == 0 {
		return "", fmt.Errorf("incomplete goal build")
	}
	if opid != "declared-consumed-history" && !f.captured[opid] {
		return "", fmt.Errorf("goal build without capture for %q", opid)
	}
	files := copyPollFiles(p.files)
	seen := make(map[string]bool)
	for _, change := range changes {
		if change.Path != "plans/goals/g.md" && change.Path != "plans/goals/backlog.md" && change.Path != "records/goals/g.md" {
			return "", fmt.Errorf("undeclared goal change path %q", change.Path)
		}
		if seen[change.Path] {
			return "", fmt.Errorf("duplicate goal change path %q", change.Path)
		}
		seen[change.Path] = true
		if change.Delete {
			delete(files, change.Path)
		} else {
			files[change.Path] = append([]byte(nil), change.Content...)
		}
	}
	id := f.nextID()
	f.commits[id] = pollCommit{parent: parent, trailer: opid, files: files, at: p.at.Add(time.Minute)}
	f.built[id] = true
	return id, nil
}

func (f *pollRepository) Publish(parent, commit string) (goal.CASOutcome, error) {
	c, ok := f.commits[commit]
	if !ok || c.parent != parent || !f.built[commit] {
		return goal.CASUnknown, fmt.Errorf("undeclared goal publication %q", commit)
	}
	if f.canonical != parent {
		return goal.CASRefused, fmt.Errorf("stale canonical goal tip")
	}
	c.published = true
	f.commits[commit] = c
	f.canonical = commit
	delete(f.built, commit)
	return goal.CASLanded, nil
}

func (f *pollRepository) AcceptedCAS(old, next string) error {
	c, ok := f.commits[next]
	if !ok || !c.published {
		return fmt.Errorf("unknown accepted target %q", next)
	}
	if old != f.accepted {
		return fmt.Errorf("stale accepted goal tip %q", old)
	}
	f.accepted = next
	return nil
}

func (f *pollRepository) IsAncestor(ancestor, descendant string) (bool, error) {
	if _, ok := f.commits[ancestor]; !ok {
		return false, fmt.Errorf("unknown ancestor %q", ancestor)
	}
	if _, ok := f.commits[descendant]; !ok {
		return false, fmt.Errorf("unknown descendant %q", descendant)
	}
	for descendant != "" {
		if descendant == ancestor {
			return true, nil
		}
		descendant = f.commits[descendant].parent
	}
	return false, nil
}

func (f *pollRepository) TrailerPresent(tip, opid string) (bool, error) {
	if opid == "" {
		return false, fmt.Errorf("empty trailer operation")
	}
	if _, ok := f.commits[tip]; !ok {
		return false, fmt.Errorf("unknown trailer tip %q", tip)
	}
	for tip != "" {
		c := f.commits[tip]
		if c.trailer == opid {
			return true, nil
		}
		tip = c.parent
	}
	return false, nil
}

func (f *pollRepository) CommitWithTrailer(revision, key, value string) (string, error) {
	err := fmt.Errorf("undeclared goal trailer lookup %q %q %q", revision, key, value)
	f.t.Error(err)
	return "", err
}
func (f *pollRepository) CommitTime(commit string) (time.Time, error) {
	c, ok := f.commits[commit]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown goal commit time %q", commit)
	}
	return c.at, nil
}
func (f *pollRepository) Release(opid string) error {
	if !f.captured[opid] {
		err := fmt.Errorf("undeclared goal ref release %q", opid)
		f.t.Error(err)
		return err
	}
	delete(f.captured, opid)
	return nil
}

func (f *pollRepository) canonicalGoal(t *testing.T, id string) *goal.GoalFile {
	t.Helper()
	if id != "g" {
		t.Fatalf("undeclared goal %q", id)
	}
	b := f.commits[f.canonical].files["plans/goals/g.md"]
	file, problems := goal.ParseFile(append([]byte(nil), b...))
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return file
}

func (f *pollRepository) syncAccepted(t *testing.T) {
	t.Helper()
	if err := f.AcceptedCAS(f.accepted, f.canonical); err != nil {
		t.Fatal(err)
	}
}

func (f *pollRepository) recordConsumedHistory(t *testing.T, file *goal.GoalFile) {
	t.Helper()
	b := goal.RenderFile(file)
	if err := os.WriteFile(filepath.Join(f.root, "plans", "goals", "g.md"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	commit, err := f.Build("declared-consumed-history", f.canonical, []goal.Change{{Path: "plans/goals/g.md", Content: b}}, "record approval use")
	if err != nil {
		t.Fatal(err)
	}
	if outcome, err := f.Publish(f.canonical, commit); err != nil || outcome != goal.CASLanded {
		t.Fatalf("consumed history publication: %s %v", outcome, err)
	}
	f.syncAccepted(t)
}
