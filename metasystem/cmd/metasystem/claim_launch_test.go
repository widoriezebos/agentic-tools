package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestClaimLaunchVerbEmitsMachineReadableOutcome(t *testing.T) {
	root := t.TempDir()
	product := filepath.Join(root, "artifacts", "agents", "worktrees", "claim-cli")
	if err := os.MkdirAll(product, 0o755); err != nil {
		t.Fatal(err)
	}
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	preparation := filepath.Join(root, "claim-occupancy.json")
	if code := runDispatchClaimOccupancyPrepare([]string{
		"--root", root,
		"--session", "codex:claim-cli",
		"--output", preparation,
	}); code != 0 {
		t.Fatalf("claim-occupancy-prepare exit=%d", code)
	}
	args := []string{
		"--root", root,
		"--opid", "claim-cli",
		"--session", "codex:claim-cli",
		"--dispatch-mode", "fresh",
		"--runtime", "codex",
		"--model", "gpt-5.6-sol",
		"--role", "implementer",
		"--launch-mode", "worktree",
		"--permission-envelope-digest", digestA,
		"--product-root", product,
		"--cap-min", "120",
		"--input-hash", digestB,
		"--main-id", "main-1",
		"--claim-epoch", "5",
		"--goal", "goal-a",
		"--goal-revision", "3",
		"--machine-id", "m-test",
		"--creator-pid", strconv.Itoa(os.Getpid()),
		"--occupancy-preparation", preparation,
	}
	out, code := captureStdout(t, func() int { return runDispatchClaimLaunch(args) })
	if code != 0 {
		t.Fatalf("claim-launch exit=%d output=%q", code, out)
	}
	var result struct {
		Outcome  string         `json:"outcome"`
		Evidence map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("claim-launch output is not JSON: %q: %v", out, err)
	}
	if result.Outcome != "WON" || result.Evidence["fingerprint"] == "" {
		t.Fatalf("claim-launch result = %+v", result)
	}
	created, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", "claim-cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(created, &record); err != nil {
		t.Fatal(err)
	}
	creator := record["creatorLiveness"].(map[string]any)
	if creator["pid"] != float64(os.Getpid()) {
		t.Fatalf("creator pid = %v, want %d", creator["pid"], os.Getpid())
	}
	if record["mainId"] != "main-1" || record["claimEpoch"] != float64(5) || record["goalId"] != "goal-a" {
		t.Fatalf("reservation provenance = mainId:%v claimEpoch:%v goalId:%v", record["mainId"], record["claimEpoch"], record["goalId"])
	}
	if record["operationId"] != "claim-cli" || record["goalRevision"] != float64(3) || record["machineId"] != "m-test" {
		t.Fatalf("setup-comparable provenance = operationId:%v goalRevision:%v machineId:%v", record["operationId"], record["goalRevision"], record["machineId"])
	}
	roots, ok := record["productRoots"].([]any)
	canonicalProduct, err := filepath.EvalSymlinks(product)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(roots) != 1 || roots[0] != canonicalProduct {
		t.Fatalf("reservation product roots = %#v, want [%s]", record["productRoots"], canonicalProduct)
	}

	mismatch := append([]string(nil), args...)
	for index, value := range mismatch {
		if value == "--input-hash" {
			mismatch[index+1] = strings.Repeat("c", 64)
			break
		}
	}
	out, code = captureStdout(t, func() int { return runDispatchClaimLaunch(mismatch) })
	if code != 1 {
		t.Fatalf("mismatch exit=%d output=%q", code, out)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Outcome != "REFUSED-OPID-MISMATCH" {
		t.Fatalf("mismatch output=%q result=%+v err=%v", out, result, err)
	}
}

func TestClaimLaunchVerbRequiresTheFingerprintTuple(t *testing.T) {
	if code := runDispatchClaimLaunch([]string{"--root", t.TempDir(), "--opid", "incomplete"}); code != 2 {
		t.Fatalf("incomplete claim-launch exit=%d, want 2", code)
	}
}
