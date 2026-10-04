package main

// Ported from scripts/agents/brain-fixtures.sh (verb redesign U7b part 3).
// The shell bed migrated a Git ledger per scenario and drove the engine's
// brain verbs. These tests call the same owners in-process: the declaration
// owner through brainActDependencies (caller class, ledger identity, machine,
// projection, scan and registry home are per-test answers) and the boot
// composer through brainBootDependencies (the optional-input child publishes
// its sections in-process, and the deadline timer never fires). No Git, no
// wall waits.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

const brainBedLedger = "01J5X00000000000000BRAINBD"

type brainBed struct {
	t        *testing.T
	root     string
	registry string
	class    string
	machine  string
	live     map[string]*goal.GoalFile
	busy     []goal.Item
}

func newBrainBed(t *testing.T) *brainBed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &brainBed{
		t: t, root: root, registry: t.TempDir(), class: lease.ClassHuman, machine: "brain",
		live: map[string]*goal.GoalFile{"ship-widget": {Id: "ship-widget", State: goal.StateQueued, NextStep: "Let a node finish it."}},
	}
}

func (b *brainBed) deps() brainActDependencies {
	return brainActDependencies{
		classify: func(root string, _ int64) (lease.ClassifyResult, error) {
			if root != b.root {
				b.t.Errorf("caller classified at %q, want %q", root, b.root)
			}
			return lease.ClassifyResult{Class: b.class}, nil
		},
		ledgerIdentity: func(string) string { return brainBedLedger },
		machine:        func(string) (string, error) { return b.machine, nil },
		project: func(string) (goal.Projection, error) {
			return goal.Projection{Tree: &goal.TreeGoals{Live: b.live}}, nil
		},
		scan:         func(string) goal.ScanResult { return goal.ScanResult{Busy: b.busy} },
		registryHome: func() (string, error) { return b.registry, nil },
		now:          func() time.Time { return time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) },
	}
}

func (b *brainBed) declare(by string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := brainDeclareWith(ownercall.Process{Pid: 1}, &stdout, &stderr, b.root, by, false, b.deps())
	return code, stdout.String(), stderr.String()
}

func (b *brainBed) withdraw(root string, deps brainActDependencies) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := brainWithdrawWith(ownercall.Process{Pid: 1}, &stdout, &stderr, root, "Wido", false, deps)
	return code, stdout.String(), stderr.String()
}

func (b *brainBed) requireRefused(label, want string, code int, stderr string) {
	b.t.Helper()
	if code != 2 || !strings.Contains(stderr, want) {
		b.t.Fatalf("%s: expected exit 2 containing %q, got %d: %s", label, want, code, stderr)
	}
}

func TestBrainBedBootCaps(t *testing.T) {
	t.Parallel()
	b := newBrainBed(t)
	b.machine = strings.Repeat("m", 33)
	code, _, stderr := b.declare("Wido")
	b.requireRefused("33-byte machine", "32-byte cap", code, stderr)
	b.machine = "brain"
	code, _, stderr = b.declare(strings.Repeat("b", 65))
	b.requireRefused("65-byte declarer", "64-byte cap", code, stderr)
	code, _, stderr = b.declare("Wi\ndo")
	b.requireRefused("control character declarer", "control characters", code, stderr)
	if brain.Read(b.root, brainBedLedger).State != brain.Undeclared {
		t.Fatal("a refused declaration wrote a record")
	}

	b.machine = strings.Repeat("m", 32)
	if code, _, stderr := b.declare(strings.Repeat("b", 64)); code != 0 {
		t.Fatalf("the maximal declaration was refused: %d %s", code, stderr)
	}
	_, payload := brain.PhaseOne(b.root, brainBedLedger, 10000)
	header, _, _ := strings.Cut(payload, "\n")
	if len(header) > brain.HeaderBytes {
		t.Fatalf("maximal declaration produced a %d-byte header", len(header))
	}
}

func TestBrainBedDeclareQuiescenceRefusals(t *testing.T) {
	t.Parallel()
	b := newBrainBed(t)
	b.live = map[string]*goal.GoalFile{"ship-widget": {Id: "ship-widget", State: goal.StateClaimed, Claimed: &goal.ClaimRecord{Machine: "brain"}}}
	code, _, stderr := b.declare("Wido")
	b.requireRefused("claimed goal", "goal release", code, stderr)
	if !strings.Contains(stderr, "--id ship-widget") {
		t.Fatalf("the release remedy did not name the goal: %s", stderr)
	}
	b.live = map[string]*goal.GoalFile{}
	for _, test := range []struct {
		item goal.Item
		want string
	}{
		{goal.Item{Kind: "job", Id: "pending-job"}, "metasystem work stop j2:pending-job"},
		{goal.Item{Kind: "job", Id: "pending-setup-job"}, "metasystem work stop j2:pending-setup-job"},
		{goal.Item{Kind: "run", Id: "live-run"}, "work wait --run live-run"},
		{goal.Item{Kind: "mission", Id: "fixture-mission"}, "mission status"},
	} {
		b.busy = []goal.Item{test.item}
		code, _, stderr := b.declare("Wido")
		b.requireRefused(test.item.Kind+" "+test.item.Id, test.want, code, stderr)
		if !strings.Contains(stderr, test.item.Id) {
			t.Fatalf("the %s refusal did not name %s: %s", test.item.Kind, test.item.Id, stderr)
		}
	}
	b.busy = nil
	if code, stdout, stderr := b.declare("Wido"); code != 0 || !strings.Contains(stdout, `"state":"declared"`) {
		t.Fatalf("a quiescent checkout was not declared: %d %s %s", code, stdout, stderr)
	}
}

func TestBrainBedVerbsHumanOnly(t *testing.T) {
	t.Parallel()
	b := newBrainBed(t)
	b.class = lease.ClassMain
	code, _, stderr := b.declare("Wido")
	b.requireRefused("agent declare", "human act", code, stderr)
	b.class = lease.ClassHuman
	if code, _, stderr := b.declare("Wido"); code != 0 {
		t.Fatalf("human declare refused: %d %s", code, stderr)
	}
	b.class = lease.ClassDelegate
	code, _, stderr = b.withdraw(b.root, b.deps())
	b.requireRefused("agent withdraw", "human act", code, stderr)
	b.class = lease.ClassHuman

	if err := os.WriteFile(brain.Path(b.root), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := b.withdraw(b.root, b.deps()); code != 0 {
		t.Fatalf("human withdraw of a corrupt record failed: %d %s", code, stderr)
	}
	if _, err := os.Stat(brain.PointerPath(b.registry, brainBedLedger)); !os.IsNotExist(err) {
		t.Fatalf("withdraw left the host pointer: %v", err)
	}

	noLedger, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(noLedger)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(noLedger), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pointerDir := brain.PointerPath(b.registry, "")
	if err := os.MkdirAll(pointerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(pointerDir, "keep")
	if err := os.WriteFile(keep, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := b.deps()
	deps.ledgerIdentity = func(string) string { return "" }
	deps.classify = func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: lease.ClassHuman}, nil
	}
	code, stdout, stderr := b.withdraw(noLedger, deps)
	if code != 0 || !strings.Contains(stdout, `"state":"undeclared"`) {
		t.Fatalf("empty-ledger withdraw did not report the removal: %d %s %s", code, stdout, stderr)
	}
	if _, err := os.Stat(brain.Path(noLedger)); !os.IsNotExist(err) {
		t.Fatalf("empty-ledger withdraw left the record: %v", err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "untouched\n" {
		t.Fatalf("empty-ledger withdraw touched the pointer directory: %q %v", data, err)
	}
	if _, err := os.Stat(pointerDir + ".lock.d"); !os.IsNotExist(err) {
		t.Fatalf("empty-ledger withdraw took the pointer lock: %v", err)
	}
}

// packet is the role packet a declared brain boots with: the one compiled
// into the engine.
func (b *brainBed) packet(t *testing.T) []byte {
	t.Helper()
	return brain.RolePacket()
}

// declaredBootBed is a checkout declared the brain with the shipped role
// packet, the state `brain declare` leaves behind.
func declaredBootBed(t *testing.T) (*brainBed, []byte) {
	t.Helper()
	b := newBrainBed(t)
	if code, _, stderr := b.declare("Wido"); code != 0 {
		t.Fatalf("declare failed: %d %s", code, stderr)
	}
	return b, b.packet(t)
}

func (b *brainBed) writeFile(relative, body string) {
	b.t.Helper()
	path := filepath.Join(b.root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		b.t.Fatal(err)
	}
}

// boot composes the boot the way `brain boot` does. publish names the
// optional sections the child manages to publish before it stops; the
// deadline timer never fires, so the child's exit ends the wait.
func (b *brainBed) boot(bound, deadlineMS int, publish ...string) brainBootOutput {
	b.t.Helper()
	readers := brainBootInputReaders{
		machine: func(string) (string, error) { return b.machine, nil },
		resolveEndpoint: func(root string) (goal.Endpoint, error) {
			return goal.Endpoint{Root: root}, nil
		},
		project: func(goal.Endpoint) (goal.Projection, error) {
			return goal.Projection{Tree: &goal.TreeGoals{Live: b.live}}, nil
		},
		resolveLayout: brainBootTestLayoutReader(b.t, b.root),
	}
	deps := brainBootDependencies{
		ledgerIdentity: func(string) string { return brainBedLedger },
		inputsCommand: func(_ string, args ...string) *exec.Cmd {
			dir := ""
			for index := 0; index+1 < len(args); index++ {
				if args[index] == "--dir" {
					dir = args[index+1]
				}
			}
			if dir == "" {
				b.t.Fatal("boot-inputs command omitted --dir")
			}
			if len(publish) == 0 {
				if err := writeBrainBootInputs(b.root, b.root, dir, readers); err != nil {
					b.t.Fatal(err)
				}
			} else {
				scratch := b.t.TempDir()
				if err := writeBrainBootInputs(b.root, b.root, scratch, readers); err != nil {
					b.t.Fatal(err)
				}
				for _, name := range publish {
					data, err := os.ReadFile(filepath.Join(scratch, name+".json"))
					if err != nil {
						b.t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600); err != nil {
						b.t.Fatal(err)
					}
				}
			}
			return exec.Command("sh", "-c", "exit 0")
		},
		now: func() time.Time { return time.Unix(1, 0) },
		timer: func(time.Duration) brainBootTimer {
			return brainBootTimer{C: make(chan time.Time), Stop: func() bool { return true }}
		},
	}
	output, err := composeBrainBootWith(b.root, b.root, bound, deadlineMS, false, deps)
	if err != nil {
		b.t.Fatalf("brain boot failed: %v", err)
	}
	if !output.Declared {
		b.t.Fatal("brain boot did not see the declaration")
	}
	return output
}

func requireBootBound(t *testing.T, output brainBootOutput, bound int) {
	t.Helper()
	if output.Bytes > bound || output.Bytes != len(output.Payload) {
		t.Fatalf("boot reported %d bytes for a %d-byte payload against bound %d", output.Bytes, len(output.Payload), bound)
	}
}

func TestBrainBedBootBound(t *testing.T) {
	t.Parallel()
	b, packet := declaredBootBed(t)
	wants := strings.Repeat("w", 2000)
	for index := 1; index <= 60; index++ {
		b.writeFile(fmt.Sprintf("artifacts/agents/channel/questions/ask-%d.json", index),
			fmt.Sprintf(`{"id":"ask-%02d","goal":"goal-%02d","kind":"other","machine":"node","openedAt":"2026-09-07T00:%02d:00Z","wants":"%s","state":"open"}`+"\n",
				index, index, index%60, wants))
	}
	for index := 1; index <= 80; index++ {
		b.writeFile(fmt.Sprintf("artifacts/agents/jobs/job-%d.json", index),
			fmt.Sprintf(`{"jobId":"job-%02d","status":"running","role":"implementer","goalId":"ship-widget"}`+"\n", index))
	}
	b.writeFile("artifacts/agents/supervision/last-census.json",
		`{"verdict":"SUCCESS","completedAtEpoch":1788739200,"counts":{"CUSTODY":2,"ANNOUNCED":1,"UNTRACKED":0}}`+"\n")
	var digest strings.Builder
	for index := 1; index <= 400; index++ {
		fmt.Fprintf(&digest, "2026-09-07T00:00:00Z HIGHLIGHT — digest line %03d (source: fixture bound)\n", index)
	}
	b.writeFile("records/narrator-digest.log", digest.String())

	output := b.boot(10000, 5000)
	requireBootBound(t, output, 10000)
	if output.Sections["fleet"] != "cut" || output.Sections["digest"] != "cut" {
		t.Fatalf("bounded boot did not cut fleet and digest: %+v", output.Sections)
	}
	for _, line := range strings.Split(output.Payload, "\n") {
		if !strings.HasPrefix(line, "ask-") {
			continue
		}
		if len(line) > 160 || !strings.HasSuffix(line, "…") {
			t.Fatalf("long ask line is %d bytes or lacks its ellipsis: %q", len(line), line)
		}
	}
	if !strings.Contains(output.Payload, string(packet)) {
		t.Fatal("full packet bytes were not intact in the 10000-byte boot")
	}

	minimum := b.boot(2048, 5000)
	requireBootBound(t, minimum, 2048)
	if !strings.Contains(minimum.Payload, "PACKET TOO LARGE FOR THIS CHANNEL") || !strings.Contains(minimum.Payload, "## The standing instruction") {
		t.Fatalf("minimum boot omitted the packet-size notice or the standing instruction: %q", minimum.Payload)
	}
	if _, stderr, status := (hookOwners{}).BrainBoot(context.Background(), b.root, b.root, 2047, 5000); status != 2 || !strings.Contains(stderr, "below the minimum 2048") {
		t.Fatalf("a 2047-byte bound was not refused: %d %q", status, stderr)
	}
}

func TestBrainBedBootErrors(t *testing.T) {
	t.Parallel()
	b, _ := declaredBootBed(t)
	cursor := narratordigest.CursorPathWithLayoutReader(b.root, brainBootTestLayoutReader(b.t, b.root), "brain")
	if err := os.MkdirAll(filepath.Dir(cursor), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cursor, []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b.writeFile("artifacts/agents/channel/questions/broken.json", "{broken\n")
	b.writeFile("artifacts/agents/jobs/broken.json", "{broken\n")

	output := b.boot(10000, 5000)
	for _, want := range []string{"unreadable question files", "unreadable job records", "CENSUS absent or unreadable", "DIGEST unreadable"} {
		if !strings.Contains(output.Payload, want) {
			t.Fatalf("boot payload omitted %q: %q", want, output.Payload)
		}
	}
	want := map[string]string{"asks": "error", "held": "complete", "fleet": "error", "digest": "error"}
	for name, state := range want {
		if output.Sections[name] != state {
			t.Fatalf("section %s = %q, want %q", name, output.Sections[name], state)
		}
	}
	if output.DigestEmitted {
		t.Fatal("an unreadable digest was marked emitted")
	}
	if data, err := os.ReadFile(cursor); err != nil || string(data) != "{broken\n" {
		t.Fatalf("an error boot changed the brain cursor: %q %v", data, err)
	}
}

func TestBrainBedBootStalled(t *testing.T) {
	t.Parallel()
	b, _ := declaredBootBed(t)
	b.writeFile("artifacts/agents/channel/questions/kept-ask.json",
		`{"id":"kept-ask","goal":"ship-widget","kind":"other","machine":"node","openedAt":"2026-09-07T00:00:00Z","wants":"kept before the stall","state":"open"}`+"\n")
	b.writeFile("artifacts/agents/supervision/last-census.json",
		`{"verdict":"SUCCESS","completedAtEpoch":1788739200,"counts":{"CUSTODY":1,"ANNOUNCED":0,"UNTRACKED":0}}`+"\n")
	b.writeFile("records/narrator-digest.log", "2026-09-07T00:00:00Z HIGHLIGHT — never read (source: fixture stalled)\n")
	cursor := narratordigest.CursorPathWithLayoutReader(b.root, brainBootTestLayoutReader(b.t, b.root), "brain")

	// The child published asks and stalled before the rest.
	output := b.boot(10000, 1500, "asks")
	if output.Sections["asks"] != "complete" || !strings.Contains(output.Payload, "kept-ask goal ship-widget") {
		t.Fatalf("the completed asks section was not kept: %+v", output)
	}
	var missing []string
	for _, name := range []string{"asks", "held", "fleet", "digest"} {
		state := output.Sections[name]
		if state != "complete" && state != "skipped" {
			t.Fatalf("stalled boot section %s had inconsistent state %q", name, state)
		}
		if state == "skipped" {
			missing = append(missing, name)
		}
	}
	if output.Sections["digest"] != "skipped" {
		t.Fatalf("stalled digest section was not skipped: %q", output.Sections["digest"])
	}
	if want := "BOOT DEADLINE: " + strings.Join(missing, ", ") + " not read within 1500 ms"; !strings.Contains(output.Payload, want) {
		t.Fatalf("deadline payload did not match skipped sections %q: %q", want, output.Payload)
	}
	if output.DigestEmitted || output.DigestCursor != 0 || output.DigestPrefixSHA256 != "" {
		t.Fatalf("stalled digest reported delivery facts: %+v", output)
	}
	if _, err := os.Stat(cursor); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stalled boot advanced the brain cursor: %v", err)
	}
}
