package channel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const reportAcceptedTip = "accepted-report-snapshot"
const reportIdentity = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// reportFixture holds one immutable accepted tree and uses real local files
// for the report's question, status, brain, and configuration reads.
type reportFixture struct {
	t          *testing.T
	root       string
	present    bool
	files      map[string][]byte
	acceptedAt time.Time
}

func newReportFixture(t *testing.T, acceptedAt time.Time, goals ...*goal.GoalFile) *reportFixture {
	t.Helper()
	f := &reportFixture{t: t, root: t.TempDir(), present: true, acceptedAt: acceptedAt, files: map[string][]byte{}}
	f.files["plans/goals/backlog.md"] = goal.RenderRoot(&goal.RootRecord{
		Identity: reportIdentity, FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	})
	for _, file := range goals {
		f.files["plans/goals/"+file.Id+".md"] = goal.RenderFile(file)
	}
	for path, content := range f.files {
		full := filepath.Join(f.root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func newAbsentReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	return &reportFixture{t: t, root: t.TempDir()}
}

func (f *reportFixture) reads(expectedWindow time.Time, log []byte) (reportGoalReads, *int) {
	f.t.Helper()
	calls := 0
	return reportGoalReads{
		resolveEndpoint: func(root string) (goal.Endpoint, error) {
			f.requireRoot(root)
			return goal.Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main", Repository: f}, nil
		},
		ledgerIdentity: func(root string) string {
			f.requireRoot(root)
			if f.present {
				return reportIdentity
			}
			return ""
		},
		landingLog: func(root string, window time.Time) ([]byte, error) {
			f.requireRoot(root)
			calls++
			if calls != 1 || !window.Equal(expectedWindow) {
				f.t.Fatalf("landing history request %d: window=%s, want %s", calls, window.Format(time.RFC3339), expectedWindow.Format(time.RFC3339))
			}
			return append([]byte(nil), log...), nil
		},
	}, &calls
}

func (f *reportFixture) composeStatus(c ReportConfig, expectedWindow time.Time, log []byte) (string, string, error) {
	f.t.Helper()
	reads, calls := f.reads(expectedWindow, log)
	text, goalID, err := composeStatusReportWithReads(c, reads)
	if *calls != 1 {
		f.t.Fatalf("landing history was read %d times, want one", *calls)
	}
	return text, goalID, err
}

func (f *reportFixture) mustCompose(c ReportConfig, expectedWindow time.Time, log []byte) string {
	f.t.Helper()
	text, _, err := f.composeStatus(c, expectedWindow, log)
	if err != nil {
		f.t.Fatal(err)
	}
	return text
}

func (f *reportFixture) requireRoot(root string) {
	f.t.Helper()
	if root != f.root {
		f.t.Fatalf("report read root %q, want %q", root, f.root)
	}
}

func (f *reportFixture) unexpected(method string) error {
	f.t.Helper()
	err := fmt.Errorf("unexpected report repository %s", method)
	f.t.Error(err)
	return err
}

func (f *reportFixture) Accepted() (string, bool, error) {
	if !f.present {
		return "", false, nil
	}
	return reportAcceptedTip, true, nil
}

func (f *reportFixture) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	if !f.present || commit != reportAcceptedTip || len(prefixes) != 2 || prefixes[0] != "plans/goals/" || prefixes[1] != "records/goals/" {
		return nil, f.unexpected(fmt.Sprintf("Files(%q, %q)", commit, prefixes))
	}
	files := make(map[string][]byte, len(f.files))
	for path, content := range f.files {
		if strings.HasPrefix(path, prefixes[0]) || strings.HasPrefix(path, prefixes[1]) {
			files[path] = append([]byte(nil), content...)
		}
	}
	return files, nil
}

func (f *reportFixture) CommitTime(commit string) (time.Time, error) {
	if !f.present || commit != reportAcceptedTip {
		return time.Time{}, f.unexpected("CommitTime(" + commit + ")")
	}
	return f.acceptedAt, nil
}

func (f *reportFixture) Capture(string) (string, error) { return "", f.unexpected("Capture") }
func (f *reportFixture) Build(string, string, []goal.Change, string) (string, error) {
	return "", f.unexpected("Build")
}
func (f *reportFixture) Publish(string, string) (goal.CASOutcome, error) {
	return "", f.unexpected("Publish")
}
func (f *reportFixture) AcceptedCAS(string, string) error { return f.unexpected("AcceptedCAS") }
func (f *reportFixture) IsAncestor(string, string) (bool, error) {
	return false, f.unexpected("IsAncestor")
}
func (f *reportFixture) TrailerPresent(string, string) (bool, error) {
	return false, f.unexpected("TrailerPresent")
}
func (f *reportFixture) CommitWithTrailer(string, string, string) (string, error) {
	return "", f.unexpected("CommitWithTrailer")
}
func (f *reportFixture) Release(string) error { return f.unexpected("Release") }
