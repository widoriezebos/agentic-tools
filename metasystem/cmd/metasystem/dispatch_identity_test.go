package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOwnershipPatchUsesAuthorizedFixtureExactIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pid := int64(os.Getpid())
	entry := map[string]any{"pidStartedAt": int64(100)}
	if runtime.GOOS == "linux" {
		entry["pidStartTicks"] = int64(7001)
		entry["bootId"] = "fixture-boot"
	} else {
		entry["pidStartedAtExactMicro"] = int64(100_000_001)
	}
	table, err := json.Marshal(map[string]any{strconv.FormatInt(pid, 10): entry})
	if err != nil {
		t.Fatal(err)
	}
	tablePath := filepath.Join(root, "identities.json")
	if err := os.WriteFile(tablePath, table, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)

	output := filepath.Join(root, "ownership.json")
	code := runDispatchOwnershipPatch([]string{
		"--root", root, "--output", output,
		"--pid", strconv.FormatInt(pid, 10), "--pgid", strconv.FormatInt(pid, 10),
		"--instance-tag", "fixture-tag", "--proven-at", "2026-08-27T20:00:00Z",
	})
	if code != 0 {
		t.Fatalf("ownership patch exit = %d", code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var patch map[string]any
	if err := json.Unmarshal(data, &patch); err != nil {
		t.Fatal(err)
	}
	if got := int64(patch["pidStartedAt"].(float64)); got != 100 {
		t.Fatalf("pidStartedAt = %d, want fixture value 100: %s", got, fmt.Sprint(patch))
	}
	if runtime.GOOS == "linux" {
		if got := int64(patch["pidStartTicks"].(float64)); got != 7001 || patch["bootId"] != "fixture-boot" {
			t.Fatalf("Linux fixture identity missing: %+v", patch)
		}
	} else if got := int64(patch["pidStartedAtExactMicro"].(float64)); got != 100_000_001 {
		t.Fatalf("Darwin fixture identity = %d, want 100000001", got)
	}
}

func TestPreforkMarkVerbWritesSupervisorIdentityBeforeFork(t *testing.T) {
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	root := t.TempDir()
	job := "prefork-command"
	tag := "metasystem-job-prefork-command-nonce"
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := json.Marshal(map[string]any{
		"jobId": job, "status": "pending", "instanceTag": tag, "custodyProcesses": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, job+".json"), record, 0o644); err != nil {
		t.Fatal(err)
	}
	pid := os.Getpid()
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		t.Fatal(err)
	}
	if code := runDispatchPreforkMark([]string{
		"--root", root, "--job", job, "--tag", tag,
		"--supervisor-pid", strconv.Itoa(pid), "--intended-pgid", strconv.Itoa(pgid),
	}); code != 0 {
		t.Fatalf("prefork-mark exit=%d", code)
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "prefork", tag))
	if err != nil {
		t.Fatal(err)
	}
	var marker map[string]any
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatal(err)
	}
	supervisor, _ := marker["supervisor"].(map[string]any)
	if int64(supervisor["pid"].(float64)) != int64(pid) || int64(marker["intendedPgid"].(float64)) != int64(pgid) {
		t.Fatalf("prefork marker = %+v", marker)
	}
}

func TestCustodyGroupsVerbListsEveryWindDownTarget(t *testing.T) {
	record := filepath.Join(t.TempDir(), "job.json")
	data, err := json.Marshal(map[string]any{
		"pgid": 88,
		"custodyProcesses": []any{
			map[string]any{"pid": 90, "pgid": 99},
			map[string]any{"pid": 91, "pgid": 88},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int { return runDispatchCustodyGroups([]string{"--record", record}) })
	if code != 0 || strings.TrimSpace(out) != "88\n99" {
		t.Fatalf("custody-groups exit=%d output=%q", code, out)
	}
}
