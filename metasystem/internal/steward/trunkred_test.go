package steward

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestTrunkRedRoleVerdicts(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	t.Run("none is alive", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", nil)
		role := checkTrunkRed(root, now)
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("no entries: %+v", role)
		}
	})
	t.Run("owned is alive with age", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", nil)
		writeHealthTrunkRed(t, root, []goal.TrunkRedEntry{healthTrunkRedEntry("owned", "bed-m1", now.Add(-2*time.Hour))})
		role := checkTrunkRed(root, now)
		if role.Status != HealthAlive || role.Reason != "1 open, all owned; oldest 2h0m0s" {
			t.Fatalf("owned entry: %+v", role)
		}
	})
	t.Run("empty owner is dead", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", nil)
		writeHealthTrunkRed(t, root, []goal.TrunkRedEntry{healthTrunkRedEntry("unowned", "", now.Add(-time.Hour))})
		role := checkTrunkRed(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "unowned") || !strings.Contains(role.Remedy, "goal trunk-red own --id unowned") {
			t.Fatalf("unowned entry: %+v", role)
		}
	})
	t.Run("stale empty batch hold is dead but a fresh hold is alive", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", nil)
		landing := healthBatchRoot(t)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+landing+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeHealthBatch(t, landing, now.Add(-2*time.Minute), "held-opid")
		role := checkTrunkRed(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "batch batch-held opid") || !strings.Contains(role.Remedy, "landing batch tick") {
			t.Fatalf("stale hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-2*time.Minute), "second-opid")
		role = checkTrunkRed(root, now)
		if role.Status != HealthDead || !strings.Contains(role.Reason, "batch batch-held opid second-opid") {
			t.Fatalf("stale rotated hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-30*time.Second), "held-opid")
		role = checkTrunkRed(root, now)
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("fresh hold: %+v", role)
		}
		writeHealthBatch(t, landing, now.Add(-30*time.Second), "second-opid", now.Add(-2*time.Minute))
		role = checkTrunkRed(root, now)
		if role.Status != HealthAlive || role.Reason != "no open trunk red" {
			t.Fatalf("fresh re-hold after stale hold: %+v", role)
		}
	})
	t.Run("unreadable configured batch root is unknown", func(t *testing.T) {
		root := convertedBed(t, "bed-m1", nil)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("landing.batch-root="+filepath.Join(t.TempDir(), "missing")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		role := checkTrunkRed(root, now)
		if role.Status != HealthUnknown || !strings.Contains(role.Reason, "batch root") {
			t.Fatalf("unreadable root: %+v", role)
		}
	})
	if !KnownHealthRole(RoleTrunkRed) {
		t.Fatal("trunk-red is absent from the published health role order")
	}
}

func healthTrunkRedEntry(id, machine string, opened time.Time) goal.TrunkRedEntry {
	stamp := opened.UTC().Format(time.RFC3339)
	owner := goal.TrunkRedOwner{}
	if machine != "" {
		owner = goal.TrunkRedOwner{Machine: machine, Since: stamp, How: "joiner"}
	}
	return goal.TrunkRedEntry{ID: id, Identity: id, Group: "fast", Status: "failed", Failures: []goal.TrunkRedFailure{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "attempt-1", Batch: "batch-1", BaseCommit: "base-1", SeenAt: stamp,
			Opid: goal.Opid("01J5X0000000000000000000W1", "bed-m1", "lineage")}}, Owner: owner, Holds: []string{"batch-1"}, Opened: stamp}
}

func writeHealthTrunkRed(t *testing.T, root string, entries []goal.TrunkRedEntry) {
	t.Helper()
	path := filepath.Join(root, "plans", "goals", "trunk-red.json")
	if err := os.WriteFile(path, goal.RenderTrunkRed(entries), 0o644); err != nil {
		t.Fatal(err)
	}
	healthGit(t, root, "add", "plans/goals/trunk-red.json")
	healthGit(t, root, "commit", "-q", "-m", "trunk-red health fixture")
	healthGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
}

func healthBatchRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	healthGit(t, root, "init", "-q", "-b", "main")
	return root
}

func writeHealthBatch(t *testing.T, root string, heldAt time.Time, currentOpid string, previousHoldAt ...time.Time) {
	t.Helper()
	directory := filepath.Join(root, "artifacts", "agents", "landing-batches")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	history := []map[string]any{}
	opids := []string{"held-opid"}
	if len(previousHoldAt) != 0 {
		history = append(history,
			map[string]any{"at": previousHoldAt[0].UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=a groups=fast opid=held-opid"},
			map[string]any{"at": previousHoldAt[0].Add(time.Second).UTC().Format(time.RFC3339Nano), "verb": "trunk-red-recorded", "detail": "entries=entry-1"})
		if currentOpid != "held-opid" {
			opids = append(opids, currentOpid)
		}
		history = append(history, map[string]any{"at": heldAt.UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=b groups=fast opid=" + currentOpid})
	} else {
		history = append(history, map[string]any{"at": heldAt.UTC().Format(time.RFC3339Nano), "verb": "trunk-red-hold", "detail": "attempt=a groups=fast opid=held-opid"})
	}
	if currentOpid != "held-opid" && len(previousHoldAt) == 0 {
		history = append(history, map[string]any{"at": heldAt.Add(time.Second).UTC().Format(time.RFC3339Nano), "verb": "trunk-red-record-failed", "detail": "opid=held-opid outcome=aborted new-opid=" + currentOpid})
		opids = append(opids, currentOpid)
	}
	record := map[string]any{
		"schema": 1, "batchId": "batch-held", "state": "held-trunk-red",
		"trunkRed": map[string]any{"opid": currentOpid, "opids": opids, "entries": []any{}},
		"history":  history,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "batch-held.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func healthGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
