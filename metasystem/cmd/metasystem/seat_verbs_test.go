package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

var seatVerbClock = time.Date(2026, 9, 24, 9, 40, 0, 0, time.UTC)

// seatVerbCheckout is a single-machine checkout the verb can read without
// reaching any remote.
func seatVerbCheckout(t *testing.T, machine string) string {
	t.Helper()
	root := t.TempDir()
	seatVerbGit(t, root, "init", "-q", "-b", "main")
	seatVerbGit(t, root, "config", "user.name", "fixture")
	seatVerbGit(t, root, "config", "user.email", "fixture@example.invalid")
	seatVerbGit(t, root, "config", "goal.sync-remote", "local")
	if machine != "" {
		seatVerbGit(t, root, "config", "metasystem.goal.machine", machine)
	}
	return root
}

func seatVerbGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestSeatFleetSaysSoWhenTheCheckoutHasNoNickname(t *testing.T) {
	t.Parallel()
	root := seatVerbCheckout(t, "")
	report, err := seatFleetReport(root, false, seatVerbClock)
	if err != nil {
		t.Fatal(err)
	}
	if !report.NoNickname {
		t.Fatalf("report = %+v", report)
	}
	text := report.Text()
	if !strings.Contains(text, "this checkout has no machine nickname and publishes no presence") {
		t.Fatalf("text = %q", text)
	}
}

func TestSeatFleetReadsTheLocalRefsAndPrintsThisMachineFirst(t *testing.T) {
	t.Parallel()
	root := seatVerbCheckout(t, "m1e")
	publishSeatFixture(t, root, "m1e", seatVerbClock.Add(-2*time.Minute))
	publishSeatFixture(t, root, "m1c", seatVerbClock.Add(-6*time.Hour))

	report, err := seatFleetReport(root, false, seatVerbClock)
	if err != nil {
		t.Fatal(err)
	}
	if report.NoNickname || report.This != "m1e" {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Machines) != 2 || report.Machines[0].Machine != "m1e" {
		t.Fatalf("machines = %+v", report.Machines)
	}
	if report.Machines[0].Standing != seat.Reachable || report.Machines[1].Standing != seat.Unreachable {
		t.Fatalf("standings = %+v", report.Machines)
	}
	text := report.Text()
	for _, want := range []string{"m1e  reachable  2 min ago", "m1c  unreachable  6 h ago"} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q:\n%s", want, text)
		}
	}
	encoded, err := report.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Machines []struct {
			Machine  string `json:"machine"`
			Standing string `json:"standing"`
		} `json:"machines"`
		Now string `json:"now"`
	}
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}
	if back.Now != "2026-09-24T09:40:00Z" || len(back.Machines) != 2 {
		t.Fatalf("json = %s", encoded)
	}
}

func TestSeatFleetVerbExitsZeroWithoutANickname(t *testing.T) {
	t.Parallel()
	root := seatVerbCheckout(t, "")
	if code := runSeatFleet([]string{"--root", root}); code != 0 {
		t.Fatalf("seat fleet = %d; want 0", code)
	}
	// No --root is not a refusal: the checkout the caller is standing in is
	// the answer, and this test process stands in the engine's own checkout.
	if code := runSeatFleet(nil); code != 0 {
		t.Fatalf("seat fleet with no root = %d; want it to read the current checkout", code)
	}
	if code := runSeatFleet([]string{"--nonsense"}); code != 2 {
		t.Fatalf("seat fleet with an unknown flag = %d; want the usage refusal", code)
	}
}

func TestSeatFamilyIsRegistered(t *testing.T) {
	t.Parallel()
	var found bool
	for _, family := range families() {
		if family.name != "seat" {
			continue
		}
		found = true
		if len(family.verbs) != 1 || family.verbs[0].name != "fleet" {
			t.Fatalf("seat family = %+v", family.verbs)
		}
	}
	if !found {
		t.Fatal("no seat family is registered")
	}
}

// publishSeatFixture writes one machine's presence into the checkout's own
// refs, which is what LocalMode publishing does.
func publishSeatFixture(t *testing.T, root, machine string, at time.Time) {
	t.Helper()
	record := seat.Record{
		PresenceSchema: seat.RecordSchema, Machine: machine, RepoIdentity: "repo-" + machine,
		Generation: 4, Engine: "3f9c1e2", ArmedLineage: seat.NoLease, TickSeconds: 600,
		TickAt: seat.FormatTime(at),
	}
	file, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), machine+".json")
	if err := os.WriteFile(path, file, 0o644); err != nil {
		t.Fatal(err)
	}
	blob := seatVerbGit(t, root, "hash-object", "-w", path)
	tree := seatVerbMktree(t, root, blob)
	commit := seatVerbGit(t, root, "commit-tree", tree, "-m", seat.CommitMessage(record))
	seatVerbGit(t, root, "update-ref", "refs/metasystem/presence/"+machine, commit)
}

func seatVerbMktree(t *testing.T, root, blob string) string {
	t.Helper()
	command := exec.Command("git", "-C", root, "mktree")
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	command.Stdin = strings.NewReader("100644 blob " + blob + "\tpresence.json\n")
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git mktree: %v", err)
	}
	return strings.TrimSpace(string(out))
}
