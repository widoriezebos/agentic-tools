package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type alertClearPersonReader struct {
	sessionStopCommandAuthorityReader
	agent bool
}

func (r alertClearPersonReader) Read(pid int64) (humanauthority.Snapshot, error) {
	snapshot, err := r.sessionStopCommandAuthorityReader.Read(pid)
	if r.agent && pid == r.pid {
		snapshot.Exact.Argv = []string{"codex", "exec"}
		snapshot.Executable = "/fixture/codex"
	}
	return snapshot, err
}

func alertClearOwners(t *testing.T, b *processBed, agent, enrolled bool) intentOwners {
	t.Helper()
	owners := b.owners()
	now := time.Date(2026, 9, 20, 10, 0, 0, 500_000_000, time.UTC)
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture process: %v %v", state, err)
	}
	owners.helm.reader = alertClearPersonReader{sessionStopCommandAuthorityReader{pid: exact.Pid, exact: exact}, agent}
	owners.helm.pid = func() int64 { return exact.Pid }
	owners.helm.account = func() string { return "fixture-login" }
	owners.alerts = alertOwners{home: func() (string, error) { return b.home, nil }, now: func() time.Time { return now }, invoker: func() (steward.AlertInvoker, error) {
		return steward.AlertInvoker{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix()}, nil
	}}
	adapters := filepath.Join(b.root(), "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(adapters, "codex.sh"), []byte("#!/bin/sh\n[ \"$1\" = signature ] && printf '%s\\n' 'match codex'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if !enrolled {
		if err := os.Remove(filepath.Join(b.root(), "artifacts/agents/authority/human-terminal.json")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if enrolled {
		reader := owners.helm.reader.(alertClearPersonReader)
		reader.agent = false
		if _, err := humanauthority.Enroll(b.root(), exact.Pid, reader, "Wido", now); err != nil {
			t.Fatal(err)
		}
	}
	owners.processes.healthNow = func(string) (time.Time, error) { return now, nil }
	owners.processes.health = func(root, _ string, _ time.Time) steward.HealthVerdict {
		output := filepath.Join(t.TempDir(), "preview.json")
		alertClearProduce(t, root, "preview", 1, output)
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		var verdict steward.HealthVerdict
		if err := json.Unmarshal(data, &verdict); err != nil {
			t.Fatal(err)
		}
		return verdict
	}
	return owners
}

func alertClearProducer(root, action string, count int, output string) *exec.Cmd {
	return exec.Command(*remedyClearProducer, "-test.run=^TestAlertClearHealthBed$", "-test.timeout=30m", "-alert-clear-bed-root="+root, "-alert-clear-bed-action="+action, fmt.Sprintf("-alert-clear-bed-count=%d", count), "-alert-clear-bed-output="+output)
}
func alertClearProduce(t *testing.T, root, action string, count int, output string) {
	t.Helper()
	command := alertClearProducer(root, action, count, output)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("health bed %s: %v\n%s", action, err, out)
	}
}
func alertClearEpisode(t *testing.T, root string) string {
	t.Helper()
	episodes, err := steward.AlertEpisodes(root)
	if err != nil {
		t.Fatal(err)
	}
	digest := alertClearRecord(t, root).Verdict.FindingDigest
	for i := len(episodes) - 1; i >= 0; i-- {
		if episodes[i].Owner == "" && episodes[i].Digest == digest {
			return "here/" + episodes[i].EpisodeID
		}
	}
	t.Fatal("no health episode")
	return ""
}
func alertClearRecord(t *testing.T, root string) struct {
	State   steward.HealthObservationState
	Verdict steward.HealthVerdict
} {
	t.Helper()
	data, err := os.ReadFile(steward.HealthRecordPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		State   steward.HealthObservationState
		Verdict steward.HealthVerdict
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}
func alertClearAttention(t *testing.T, root string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts/agents/steward/ledger-attention.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestAlertClearRearmsAndAcknowledgesLedger(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	for _, corrupt := range []string{"health", "attention", "alive", "standing"} {
		t.Run(corrupt, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			alertClearProduce(t, b.root(), "tick", 5, "")
			owners := alertClearOwners(t, b, false, false)
			id := alertClearEpisode(t, b.root())
			path := steward.HealthRecordPath(b.root())
			if corrupt == "attention" {
				path = filepath.Join(b.root(), "artifacts/agents/steward/ledger-attention.json")
			}
			if corrupt == "standing" {
				record := alertClearRecord(t, b.root())
				record.State.FailureCounts = map[steward.HealthRole]int{steward.RoleProofAttempts: 5, steward.RoleTrunkRed: 5}
				record.State.FailureCauses = map[steward.HealthRole]string{steward.RoleProofAttempts: "fixture standing failure", steward.RoleTrunkRed: "fixture standing failure"}
				for i, role := range record.Verdict.Roles {
					role.Status, role.Standing, role.FailureEscalation, role.ConsecutiveFailures = steward.HealthAlive, false, "", 0
					if role.Role == steward.RoleProofAttempts || role.Role == steward.RoleTrunkRed {
						role.Status, role.Standing, role.FailureEscalation, role.ConsecutiveFailures = steward.HealthDead, true, steward.NoLawfulRemedy, 5
						role.Reason = "fixture standing failure"
					}
					record.Verdict.Roles[i] = role
				}
				record.Verdict.State = record.State
				record.Verdict.Aggregate, record.Verdict.ShouldAlert = "healthy", false
				record.Verdict.FindingDigest = fmt.Sprintf("%x", sha256.Sum256(nil))
				data, _ := json.Marshal(record)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				episode, _, err := steward.OpenAlert(b.root(), steward.AlertOpening{Owner: steward.PatternOwner("health-standing-red"), Work: string(steward.RoleProofAttempts), Since: owners.alerts.now(), Now: owners.alerts.now(), Message: "The proof attempts have a standing defect.", Deliver: func(string, string) error { return nil }})
				if err != nil {
					t.Fatal(err)
				}
				code, result := b.runJSON(owners, "alert", "clear", "here/"+episode.EpisodeID)
				after := alertClearRecord(t, b.root())
				if code != 0 || !strings.HasSuffix(result.Summary, "re-armed: proof-attempts") || after.State.FailureCounts[steward.RoleProofAttempts] != 0 || after.State.FailureCounts[steward.RoleTrunkRed] != 5 || after.Verdict.Aggregate != "unhealthy" || !after.Verdict.ShouldAlert || after.Verdict.FindingDigest == record.Verdict.FindingDigest {
					t.Fatalf("standing clear must re-arm only its named defect and update the verdict: %d %+v %+v", code, result, after)
				}
				return
			}
			if corrupt == "alive" {
				record := alertClearRecord(t, b.root())
				for i := range record.Verdict.Roles {
					record.Verdict.Roles[i].Status = steward.HealthAlive
					record.Verdict.Roles[i].FailureEscalation = ""
				}
				data, _ := json.Marshal(record)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				code, first := b.runJSON(owners, "alert", "clear", id)
				if code != 0 || strings.Contains(first.Summary, "re-armed") || strings.Contains(first.Summary, "acknowledged") {
					t.Fatalf("alive clear changed roles: %d %+v", code, first)
				}
				before, _ := os.ReadFile(path)
				code, repeat := b.runJSON(owners, "alert", "clear", id)
				after, _ := os.ReadFile(path)
				if code != 0 || repeat.Outcome != intentUnchanged || !bytes.Equal(before, after) {
					t.Fatalf("alive repeat changed state: %d %+v", code, repeat)
				}
				return
			}
			if err := os.WriteFile(path, []byte("{torn"), 0600); err != nil {
				t.Fatal(err)
			}
			code, result := b.runJSON(owners, "alert", "clear", id)
			if code != 1 || !strings.Contains(result.Summary, path) {
				t.Fatalf("unreadable clear: %d %+v", code, result)
			}
			episodes, _ := steward.AlertEpisodes(b.root())
			for _, episode := range episodes {
				if "here/"+episode.EpisodeID == id && episode.Cleared {
					t.Fatalf("unreadable state cleared episode: %+v", episode)
				}
			}
		})
	}
	for _, test := range []struct {
		name                  string
		agent, enrolled, helm bool
	}{{"enrolled person", false, true, false}, {"unenrolled person", false, false, false}, {"agent shell", true, false, false}, {"helm caller", true, true, true}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			alertClearProduce(t, b.root(), "tick", 5, "")
			owners := alertClearOwners(t, b, test.agent, test.enrolled)
			if test.helm {
				if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				takeHelmAt(t, b.root())
				fake := newFakeHelm(b.root(), filepath.Join(b.root(), ".git"), "DELEGATE")
				useHelmAdmitter(b.root(), fake.admitter)
				t.Cleanup(func() { helmAdmitters.Delete(resolvedHelmRoot(b.root())) })
			}
			id := alertClearEpisode(t, b.root())
			code, check := b.runJSON(owners, "system", "check")
			data, _ := json.Marshal(check)
			if code != 1 || !strings.Contains(string(data), "metasystem alert clear "+id) {
				t.Fatalf("exhausted system check: %d %s", code, data)
			}
			before := alertClearRecord(t, b.root())
			for _, role := range []steward.HealthRole{steward.RoleLedgerAttention, steward.RoleCapabilitySnapshots} {
				if before.State.FailureCounts[role] != 5 {
					t.Fatalf("%s did not exhaust: %+v", role, before.State)
				}
			}
			code, result := b.runJSON(owners, "alert", "clear", id)
			if code != 0 || result.Outcome != intentConfirmed || !strings.Contains(result.Summary, "re-armed:") {
				t.Fatalf("clear: %d %+v", code, result)
			}
			cleared := alertClearRecord(t, b.root())
			for _, role := range []steward.HealthRole{steward.RoleLedgerAttention, steward.RoleCapabilitySnapshots} {
				if cleared.State.FailureCounts[role] != 0 || cleared.State.FailureCauses[role] != "" || len(cleared.State.FailureEpisodes[role]) != len(before.State.FailureEpisodes[role]) {
					t.Fatalf("clear lost history or failed to reset %s: %+v", role, cleared.State)
				}
			}
			episodes, _ := steward.AlertEpisodes(b.root())
			for _, episode := range episodes {
				if "here/"+episode.EpisodeID == id && (!episode.Cleared || episode.ClearedBy == nil) {
					t.Fatalf("clear not recorded: %+v", episode)
				}
			}
			if !test.enrolled {
				if _, err := humanauthority.ReadEnrollment(b.root()); !os.IsNotExist(err) {
					t.Fatalf("unenrolled fixture has an enrollment: %v", err)
				}
			}
			attention := alertClearAttention(t, b.root())
			digest, _ := os.ReadFile(filepath.Join(b.root(), "records/narrator-digest.log"))
			if test.agent && !test.helm {
				if attention["examinedTip"] != "examined-tip" || bytes.Contains(digest, []byte("A person (")) || result.Next == nil || !strings.Contains(result.Next.Reason, "in a terminal you opened yourself") || strings.Join(result.Next.Argv, " ") != "metasystem alert clear "+id {
					t.Fatalf("agent clear acknowledged or lost the person's act: %+v %s %+v", attention, digest, result)
				}
			} else {
				who := "Wido"
				if test.helm {
					who = "wido"
				}
				if !test.enrolled {
					who = "fixture-login (not enrolled)"
				}
				if attention["examinedTip"] != "remote-tip" || attention["movedAt"] != nil || !bytes.Contains(digest, []byte("A person ("+who+") acknowledged")) {
					t.Fatalf("person not acknowledged: %+v %s", attention, digest)
				}
			}
			action := "restart"
			if test.agent && !test.helm {
				action = "tick"
			}
			alertClearProduce(t, b.root(), action, 1, "")
			after := alertClearRecord(t, b.root())
			for _, role := range after.Verdict.Roles {
				if role.Role == steward.RoleLedgerAttention && ((role.Status == steward.HealthAlive) == (test.agent && !test.helm)) {
					t.Fatalf("ledger role after clear: %+v", role)
				}
			}
			probePath := steward.ComponentEvidencePath(b.root(), "capability-probe-claude")
			var probe struct{ AttemptSeq int64 }
			raw, _ := os.ReadFile(probePath)
			json.Unmarshal(raw, &probe)
			if probe.AttemptSeq != 5 {
				t.Fatalf("first clear did not retry the probe: %s", raw)
			}
			// Exhaust again without a healthy snapshot, then clear the same episode.
			alertClearProduce(t, b.root(), "tick", 4, "")
			_, check = b.runJSON(owners, "system", "check")
			data, _ = json.Marshal(check)
			currentID := alertClearEpisode(t, b.root())
			if !strings.Contains(string(data), "metasystem alert clear "+currentID) {
				t.Fatalf("second exhaustion has no usable act: %s", data)
			}
			// A person's ledger acknowledgment changes the finding once. From here
			// the capability-only episode stands across both subsequent clears.
			code, result = b.runJSON(owners, "alert", "clear", currentID)
			if code != 0 || (!strings.Contains(result.Summary, "re-armed:") || !strings.Contains(result.Summary, "capability-snapshots")) {
				t.Fatalf("second clear: %d %+v", code, result)
			}
			alertClearProduce(t, b.root(), "tick", 5, "")
			_, check = b.runJSON(owners, "system", "check")
			data, _ = json.Marshal(check)
			if !strings.Contains(string(data), "metasystem alert clear "+currentID) {
				t.Fatalf("cleared episode hid the current retry: %s", data)
			}
			code, result = b.runJSON(owners, "alert", "clear", currentID)
			if code != 0 || (!strings.Contains(result.Summary, "was already cleared; re-armed:") || !strings.Contains(result.Summary, "capability-snapshots")) {
				t.Fatalf("repeated clear did not re-arm: %d %+v", code, result)
			}
			alertClearProduce(t, b.root(), "tick", 1, "")
			raw, _ = os.ReadFile(probePath)
			json.Unmarshal(raw, &probe)
			if probe.AttemptSeq != 13 {
				t.Fatalf("repeated clear did not retry probe: %s", raw)
			}
		})
	}
}

func TestAlertClearAndTickFinishTogether(t *testing.T) {
	t.Parallel()
	if runHealthRemedyCommandChild(t) {
		return
	}
	b := newProcessBed(t)
	alertClearProduce(t, b.root(), "tick", 5, "")
	owners := alertClearOwners(t, b, true, false)
	id := alertClearEpisode(t, b.root())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	args := alertClearProducer(b.root(), "concurrent", 1, "").Args
	tick := exec.CommandContext(ctx, args[0], args[1:]...)
	tick.WaitDelay = time.Second
	stdout, err := tick.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	tick.Stderr = &stderr
	if err := tick.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "tick holds arbitration" {
		t.Fatalf("tick did not start under arbitration: %s %s", scanner.Text(), stderr.String())
	}
	deadline := time.AfterFunc(8*time.Second, cancel)
	defer deadline.Stop()
	done := make(chan int, 1)
	go func() { code, _ := b.runJSON(owners, "alert", "clear", id); done <- code }()
	tickErr := tick.Wait()
	select {
	case code := <-done:
		if code != 0 || tickErr != nil {
			t.Fatalf("concurrent tick/clear: clear=%d tick=%v %s", code, tickErr, stderr.String())
		}
	case <-ctx.Done():
		t.Fatal("clear and tick did not both finish")
	}
}
