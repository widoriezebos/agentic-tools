package steward

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgit"
)

type marksRead struct {
	args []string
	out  string
	err  error
}

func readMarksWithStub(t *testing.T, root string, reads ...marksRead) (func() (Marks, error), *testgit.Stub) {
	t.Helper()
	expected := make([]testgit.Expectation, 0, len(reads))
	for _, read := range reads {
		expected = append(expected, testgit.Expectation{
			Call:   testgit.Call{Dir: root, Args: read.args},
			Result: testgit.Result{Stdout: []byte(read.out), Err: read.err},
		})
	}
	stub := testgit.New(t, expected...)
	return func() (Marks, error) {
		return currentMarksWithReader(root, func(gotRoot string, args ...string) ([]byte, error) {
			result := stub.Run(testgit.Call{Dir: gotRoot, Args: args})
			return result.Stdout, result.Err
		})
	}, stub
}

func headMark(out string, err error) marksRead {
	return marksRead{args: []string{"rev-parse", "HEAD"}, out: out, err: err}
}

func acceptedMark(out string, err error) marksRead {
	return marksRead{args: []string{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"}, out: out, err: err}
}

// The ledger mark reads the accepted goals ref, not the retired
// plans/goals.md: goal movement with a still head reads as PROGRESS, and
// the retired file's churn moves nothing (steward-marks-retired-ledger).
func TestCurrentMarksFollowTheAcceptedGoalsRef(t *testing.T) {
	repo := t.TempDir()
	head := strings.Repeat("a", 40)
	tip := strings.Repeat("b", 40)
	tip2 := strings.Repeat("c", 40)
	read, stub := readMarksWithStub(t, repo,
		headMark(head+"\n", nil), acceptedMark("", errors.New("absent ref")),
		headMark(head+"\n", nil), acceptedMark("", errors.New("absent ref")),
		headMark(head+"\n", nil), acceptedMark(tip+"\n", nil),
		headMark(head+"\n", nil), acceptedMark(tip2+"\n", nil),
	)
	before, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if before.HeadOid != head || before.OpidDigest != "no-ledger" {
		t.Fatalf("absent accepted ref must keep stable HEAD and ledger sentinel: %+v", before)
	}

	// The retired ledger file no longer speaks for the mark.
	if err := os.MkdirAll(filepath.Join(repo, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plans", "goals.md"), []byte("retired churn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	retired, err := read()
	if err != nil {
		t.Fatal(err)
	}
	if retired.OpidDigest != "no-ledger" {
		t.Fatalf("the retired file moved the mark: %q", retired.OpidDigest)
	}

	// Ledger-only movement leaves HEAD still and reads as progress.
	first, err := read()
	if err != nil {
		t.Fatal(err)
	}
	firstSum := sha256.Sum256([]byte(tip))
	if first.OpidDigest != hex.EncodeToString(firstSum[:]) || first.OpidDigest == retired.OpidDigest {
		t.Fatalf("an accepted ref tip must move the mark: %q", first.OpidDigest)
	}
	if first.HeadOid != before.HeadOid {
		t.Fatalf("the ledger ref advance must not move HEAD's mark: %q vs %q", first.HeadOid, before.HeadOid)
	}

	second, err := read()
	if err != nil {
		t.Fatal(err)
	}
	secondSum := sha256.Sum256([]byte(tip2))
	if second.OpidDigest != hex.EncodeToString(secondSum[:]) || second.OpidDigest == first.OpidDigest {
		t.Fatal("a second ledger advance must read as fresh progress")
	}
	if second.HeadOid != head || len(stub.Calls()) != 8 {
		t.Fatalf("marks did not read stable HEAD and each declared ref in order: %+v calls=%+v", second, stub.Calls())
	}
	t.Run("missing repository sentinels", checkCurrentMarksUsesSentinelsWithoutRepository)
	t.Run("unreadable existing HEAD", checkCurrentMarksRejectsUnreadableHeadInExistingRepository)
	t.Run("empty accepted tip", checkCurrentMarksKeepsLedgerSentinelForEmptyAcceptedTip)
}

func checkCurrentMarksUsesSentinelsWithoutRepository(t *testing.T) {
	root := t.TempDir()
	read, _ := readMarksWithStub(t, root,
		headMark("", errors.New("no repository")), acceptedMark("", errors.New("no accepted ref")),
	)
	marks, err := read()
	if err != nil || marks.HeadOid != "no-head" || marks.OpidDigest != "no-ledger" {
		t.Fatalf("missing repository must use both sentinels: %+v %v", marks, err)
	}
}

func checkCurrentMarksRejectsUnreadableHeadInExistingRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	read, _ := readMarksWithStub(t, root, headMark("", errors.New("unreadable")))
	if _, err := read(); err == nil || !strings.Contains(err.Error(), "HEAD unreadable") {
		t.Fatalf("existing repository with unreadable HEAD must fail: %v", err)
	}
}

func checkCurrentMarksKeepsLedgerSentinelForEmptyAcceptedTip(t *testing.T) {
	root := t.TempDir()
	read, _ := readMarksWithStub(t, root, headMark("head\n", nil), acceptedMark(" \n", nil))
	marks, err := read()
	if err != nil || marks.HeadOid != "head" || marks.OpidDigest != "no-ledger" {
		t.Fatalf("empty accepted tip must keep ledger sentinel: %+v %v", marks, err)
	}
}
