package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestReadItemsFileSkipsBlanksAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "items.txt")
	if err := os.WriteFile(path, []byte("# read return\n\n  First item.  \r\n   # ignored\r\nSecond item.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := readItemsFile(path)
	if err != nil || !reflect.DeepEqual(items, []string{"First item.", "Second item."}) {
		t.Fatalf("items file = %q, %v", items, err)
	}
}

func readItemCommandFixture(t *testing.T) string {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	amendSyncedGoalFixture(t, root, "read item fixture", func(file *goal.GoalFile) {
		file.ReadItems = []goal.ReadItem{
			{ID: "critic-1", Read: "critic", Text: "Name the boundary.", State: goal.ReadItemOpen, AddedAt: "2026-09-17T10:00:00Z"},
			{ID: "critic-2", Read: "critic", Text: "Already repaired.", State: goal.ReadItemFixed, AddedAt: "2026-09-17T10:00:00Z", ChangedAt: "2026-09-17T11:00:00Z", ClosingReference: strings.Repeat("a", 40)},
		}
	})
	return root
}

func TestGoalShowAndNextPrintOpenReadItemFixUnit(t *testing.T) {
	root := readItemCommandFixture(t)
	show, showCode := captureGoalOutput(t, func() int {
		return runGoalShow([]string{"--root", root, "--id", "standing-validation"})
	})
	if showCode != 0 || !strings.Contains(show, `"heading":"Open read items (fix unit critic): 1"`) || !strings.Contains(show, `"id":"critic-1"`) {
		t.Fatalf("goal show omitted read fix unit: code=%d output=%q", showCode, show)
	}
	next, nextCode := captureGoalOutput(t, func() int {
		return runGoalNext([]string{"--root", root, "--machine", "mac-cli"})
	})
	if nextCode != 0 || !strings.Contains(next, "continue your claimed goal: standing-validation\nOpen read items (fix unit critic): 1\n- critic-1: Name the boundary.\n") {
		t.Fatalf("goal next omitted block after selection: code=%d output=%q", nextCode, next)
	}
}

func TestGoalReadItemsListJSONShape(t *testing.T) {
	root := readItemCommandFixture(t)
	output, code := captureGoalOutput(t, func() int {
		return runGoalReadItems([]string{"list", "--root", root, "--open", "--json"})
	})
	var envelope struct {
		Tip   string              `json:"tip"`
		Goals []readItemsGoalJSON `json:"goals"`
	}
	if code != 0 || json.Unmarshal([]byte(output), &envelope) != nil || envelope.Tip == "" || len(envelope.Goals) != 1 || envelope.Goals[0].Goal != "standing-validation" || envelope.Goals[0].State != goal.StateClaimed || len(envelope.Goals[0].Items) != 1 || envelope.Goals[0].Items[0].ID != "critic-1" {
		t.Fatalf("read-items list JSON shape: code=%d output=%q decoded=%+v", code, output, envelope)
	}
}

func TestDoneReadItemRefusalRemedyExecutes(t *testing.T) {
	root := readItemCommandFixture(t)
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return runGoalDone([]string{"--root", root, "--id", "standing-validation", "--conclude", "Finished.", "--lineage", "m1"})
	})
	var refusal struct {
		Detail string `json:"detail"`
	}
	if code != 1 || stderr != "" || json.Unmarshal([]byte(stdout), &refusal) != nil {
		t.Fatalf("done refusal: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	template := "metasystem goal read-items close --id standing-validation --item critic-1 --fixed <commit>"
	if !strings.Contains(refusal.Detail, template) {
		t.Fatalf("done refusal lacks executable fixed remedy %q: %s", template, refusal.Detail)
	}
	commit := goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	command := strings.Replace(template, "<commit>", commit, 1)
	fields := strings.Fields(command)
	closeArgs := append(append([]string{}, fields[3:]...), "--root", root, "--lineage", "m1")
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runGoalReadItems(closeArgs)
	})
	if code != 0 || stderr != "" {
		t.Fatalf("printed fixed remedy failed: command=%q code=%d stdout=%q stderr=%q", command, code, stdout, stderr)
	}
	projection, err := goal.Project(goal.Endpoint{Root: root, Remote: goal.SyncLocal, Branch: "refs/heads/main"}, false, time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC))
	if err != nil || projection.Tree.Live["standing-validation"].ReadItems[0].State != goal.ReadItemFixed {
		t.Fatalf("printed fixed remedy did not close item: projection=%+v err=%v", projection, err)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return runGoalDone([]string{"--root", root, "--id", "standing-validation", "--conclude", "Finished.", "--lineage", "m1"})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("done after remedy: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
