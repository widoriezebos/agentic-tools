package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestCurrentWaitingLinesOnAnInstallationWithoutALease(t *testing.T) {
	// A template or local-only installation has no checkout lease. It owns no
	// waiter rows, so orientation prints nothing there and reports no failure;
	// a Stop in such an installation keeps its own verdict.
	root := t.TempDir()
	lines, err := CurrentWaitingLines(root)
	if err != nil || len(lines) != 0 {
		t.Fatalf("an installation without a lease reported lines=%v err=%v", lines, err)
	}
}

func TestWaitingLinesUseDurableResumeCommand(t *testing.T) {
	root := t.TempDir()
	row := run.Waiter{SchemaVersion: 2, WaitID: "0123456789abcdef0123456789abcdef", Nonce: "fedcba9876543210fedcba9876543210", Kind: "goal", TargetID: "goal-a", OwnerLineage: "lineage-a", State: "pending", Deadline: "2026-09-14T12:00:00Z"}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(run.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.WaitersDir(root), "goal-goal-a-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	channelRow := run.Waiter{SchemaVersion: 2, WaitID: "abcdefabcdefabcdefabcdefabcdefab", Nonce: "1234567890abcdef1234567890abcdef", Kind: "goal", TargetID: "goal-b", OwnerLineage: "lineage-a", State: "pending", Deadline: "2026-09-14T13:00:00Z", Selector: run.WaitSelector{Kind: "goal", TargetID: "goal-b", Poll: "channel"}}
	channelData, err := json.Marshal(channelRow)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.WaitersDir(root), "goal-goal-b-owner.json"), channelData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.WaitersDir(root), "run-broken-owner.json"), []byte("{broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := WaitingLines(root, "lineage-a")
	if err != nil || len(lines) != 3 || lines[0] != "WAITING goal goal-a until 2026-09-14T12:00:00Z: metasystem wait --resume 0123456789abcdef0123456789abcdef" || lines[1] != "WAITING goal goal-b until 2026-09-14T13:00:00Z: metasystem channel wait --resume abcdefabcdefabcdefabcdefabcdefab" || !strings.Contains(lines[2], "run-broken-owner.json") || !strings.Contains(lines[2], "unreadable") {
		t.Fatalf("waiting lines=%q err=%v", lines, err)
	}
	if foreign, err := WaitingLines(root, "lineage-b"); err != nil || len(foreign) != 1 || !strings.Contains(foreign[0], "run-broken-owner.json") {
		t.Fatalf("foreign waiting lines=%q err=%v", foreign, err)
	}
	if rows, failures := run.PendingWaitersForLineages(root, nil); len(rows) != 0 || len(failures) != 0 {
		t.Fatalf("empty lineage allow-list exposed rows=%+v failures=%v", rows, failures)
	}
}

func TestWaitingLinesUseWaitEndForLiveLocalWait(t *testing.T) {
	root := t.TempDir()
	row := run.Waiter{
		SchemaVersion: 2,
		WaitID:        "0123456789abcdef0123456789abcdef",
		Nonce:         "fedcba9876543210fedcba9876543210",
		Kind:          "local",
		TargetID:      "0123456789abcdef0123456789abcdef",
		OwnerLineage:  "lineage-a",
		Pid:           int64(os.Getpid()),
		Label:         "compile release",
		JobID:         "job-a",
		State:         "pending",
		Deadline:      "2026-09-18T14:00:00Z",
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(run.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.WaitersDir(root), "local-live-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	lines, err := WaitingLines(root, "lineage-a")
	want := "WAITING local compile release for job job-a until 2026-09-18T14:00:00Z: metasystem wait end --wait-id 0123456789abcdef0123456789abcdef"
	if err != nil || len(lines) != 1 || lines[0] != want || strings.Contains(lines[0], "--resume") {
		t.Fatalf("waiting lines=%q err=%v", lines, err)
	}
}

func TestWaitingLinesUseWaitEndForPendingHumanWait(t *testing.T) {
	root := t.TempDir()
	row := run.Waiter{
		SchemaVersion: 2,
		WaitID:        "abcdefabcdefabcdefabcdefabcdefab",
		Nonce:         "1234567890abcdef1234567890abcdef",
		Kind:          "human",
		TargetID:      "abcdefabcdefabcdefabcdefabcdefab",
		OwnerLineage:  "lineage-a",
		Question:      "Proceed with release?",
		State:         "pending",
		Deadline:      "2026-09-18T15:00:00Z",
	}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(run.WaitersDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run.WaitersDir(root), "human-pending-owner.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	lines, err := WaitingLines(root, "lineage-a")
	want := "WAITING human answer to Proceed with release? until 2026-09-18T15:00:00Z: metasystem wait end --wait-id abcdefabcdefabcdefabcdefabcdefab"
	if err != nil || len(lines) != 1 || lines[0] != want || strings.Contains(lines[0], "--resume") {
		t.Fatalf("waiting lines=%q err=%v", lines, err)
	}
}

func TestSucceededWaitOwnerUsesMainIDChainAndAnnouncementLineage(t *testing.T) {
	root := t.TempDir()
	mains := filepath.Join(root, "artifacts", "agents", "mains")
	if err := os.MkdirAll(mains, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(mains, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("worktree-lease.json", `{"holderMainId":"main-c","ownerLineage":"lineage-c","pid":3,"claimEpoch":3,"revision":3,"takeovers":[{"fromMainId":"main-a","toMainId":"main-b","claimEpoch":2,"reason":"holder-death"},{"fromMainId":"main-b","toMainId":"main-c","claimEpoch":3,"reason":"holder-death"}]}`)
	write("a.json", `{"mainId":"main-a","ownerLineage":"lineage-a"}`)
	write("b.json", `{"mainId":"main-b"}`)
	if lineage, ok, err := SucceededWaitOwner(root, "main-c", 3, "main-a"); err != nil || !ok || lineage != "lineage-a" {
		t.Fatalf("explicit predecessor lineage=%q ok=%t err=%v", lineage, ok, err)
	}
	if lineage, ok, err := SucceededWaitOwner(root, "main-c", 3, "main-b"); err != nil || !ok || lineage != "main-b" {
		t.Fatalf("default predecessor lineage=%q ok=%t err=%v", lineage, ok, err)
	}
	write("worktree-lease.json", `{"holderMainId":"main-c","ownerLineage":"lineage-c","pid":3,"claimEpoch":3,"revision":3,"takeovers":[{"fromMainId":"main-a","toMainId":"main-b","claimEpoch":2,"reason":"holder-death"},{"fromMainId":"main-b","toMainId":"main-c","claimEpoch":2,"reason":"holder-death"}]}`)
	if lineage, ok, err := SucceededWaitOwner(root, "main-c", 3, "main-b"); err != nil || ok || lineage != "" {
		t.Fatalf("wrong takeover epoch admitted predecessor lineage=%q ok=%t err=%v", lineage, ok, err)
	}
}
