package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

const brainBootTestPacket = "# Fixture brain packet\n\n## The standing instruction\nKeep going.\n"

func declaredBrainBootTestRoot(t *testing.T) string {
	t.Helper()
	root := syncedClaimedGoalFixture(t)
	if err := os.MkdirAll(filepath.Join(root, "records", "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, brain.PacketRelativePath), []byte(brainBootTestPacket), 0o644); err != nil {
		t.Fatal(err)
	}
	record := brain.Record{
		Schema: brain.Schema, Ledger: goal.ExistingLedgerIdentity(root), Machine: "mac-cli",
		DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBrainBootKeepsPhaseOneWhenOptionalInputChildFails(t *testing.T) {
	root := declaredBrainBootTestRoot(t)

	original := newBrainBootInputsCommand
	newBrainBootInputsCommand = func(string, ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 23")
	}
	t.Cleanup(func() { newBrainBootInputsCommand = original })

	output, err := composeBrainBoot(root, root, minimumBrainContextBytes, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if !output.Declared || !strings.Contains(output.Payload, brainBootTestPacket) || !strings.Contains(output.Payload, "BOOT DEADLINE: asks, held, fleet, digest not read") {
		t.Fatalf("optional-input failure discarded phase one or its diagnostic: %+v", output)
	}
	for _, name := range []string{"asks", "held", "fleet", "digest"} {
		if output.Sections[name] != "skipped" {
			t.Fatalf("section %s = %q, want skipped", name, output.Sections[name])
		}
	}
}

func TestBrainBootDeadlineKeepsCompletedSections(t *testing.T) {
	root := declaredBrainBootTestRoot(t)
	original := newBrainBootInputsCommand
	newBrainBootInputsCommand = func(_ string, args ...string) *exec.Cmd {
		dir := ""
		for index := 0; index+1 < len(args); index++ {
			if args[index] == "--dir" {
				dir = args[index+1]
				break
			}
		}
		if dir == "" {
			t.Fatal("boot-inputs command omitted --dir")
		}
		// The command outlives any bound the boot could honour: a boot that
		// waited for it would take minutes, one that kept its deadline
		// returns well inside the wiring bound.
		return exec.Command("sh", "-c", `printf '%s\n' "$2" >"$1"; sleep 120`, "brain-boot-test",
			filepath.Join(dir, "asks.json"), `{"status":"complete","lines":[{"text":"kept ask"}]}`)
	}
	t.Cleanup(func() { newBrainBootInputsCommand = original })

	started := time.Now()
	output, err := composeBrainBoot(root, root, minimumBrainContextBytes, 250)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > wiringBound {
		t.Fatalf("deadline boot took %s", elapsed)
	}
	if output.Sections["asks"] != "complete" || !strings.Contains(output.Payload, "kept ask") {
		t.Fatalf("completed asks section was discarded after the deadline: %+v", output)
	}
	for _, name := range []string{"held", "fleet", "digest"} {
		if output.Sections[name] != "skipped" {
			t.Fatalf("unfinished section %s = %q, want skipped", name, output.Sections[name])
		}
	}
	if !strings.Contains(output.Payload, "BOOT DEADLINE: held, fleet, digest not read") || strings.Contains(output.Payload, "BOOT DEADLINE: asks") {
		t.Fatalf("deadline line did not name only unfinished sections: %q", output.Payload)
	}
}

func TestBrainReadOnlyBootDefersStatusUntilStartDelivered(t *testing.T) {
	root := declaredBrainBootTestRoot(t)
	statusPath := brain.StatusPath(root)

	output, err := composeBrainBootMode(root, root, minimumBrainContextBytes, 50, true)
	if err != nil {
		t.Fatal(err)
	}
	if !output.Declared || len(output.DeclarationSHA256) != 64 {
		t.Fatalf("read-only output omitted declaration authorization: %+v", output)
	}
	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Fatalf("read-only boot wrote status before publication: %v", err)
	}
	if status := runBrainStartDelivered([]string{
		"--root", root, "--repo", root, "--declaration-sha256", output.DeclarationSHA256,
	}); status != 0 {
		t.Fatalf("start-delivered status = %d", status)
	}
	if _, err := brain.ReadStatus(root); err != nil {
		t.Fatalf("delivery acknowledgment did not write status: %v", err)
	}
}

func TestBrainStartDeliveredRefusesChangedDeclaration(t *testing.T) {
	root := declaredBrainBootTestRoot(t)
	output, err := composeBrainBootMode(root, root, minimumBrainContextBytes, 50, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(brain.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	var record brain.Record
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record.DeclaredBy = "another human"
	changed, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(changed, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if status := runBrainStartDelivered([]string{
		"--root", root, "--repo", root, "--declaration-sha256", output.DeclarationSHA256,
	}); status != 1 {
		t.Fatalf("changed declaration status = %d, want 1", status)
	}
	if _, err := os.Stat(brain.StatusPath(root)); !os.IsNotExist(err) {
		t.Fatalf("changed declaration wrote status: %v", err)
	}
}

func TestBrainStartDeliveredAdvancesOnlyTheEmittedBrainDigest(t *testing.T) {
	root := declaredBrainBootTestRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "records"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "records", "narrator-digest.log"), []byte("delivered digest line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := composeBrainBootMode(root, root, minimumBrainContextBytes, 50, true)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := narratordigest.Pending(root, "brain")
	if err != nil || pending.Cursor == 0 || pending.PrefixSHA256 == "" {
		t.Fatalf("cannot prepare emitted digest coordinates: %+v %v", pending, err)
	}
	if status := runBrainStartDelivered([]string{
		"--root", root, "--repo", root,
		"--declaration-sha256", output.DeclarationSHA256,
		"--digest-cursor", fmt.Sprint(pending.Cursor),
		"--digest-prefix-sha256", pending.PrefixSHA256,
	}); status != 0 {
		t.Fatalf("start-delivered status = %d", status)
	}
	if _, err := os.Stat(narratordigest.CursorPath(root, "brain")); err != nil {
		t.Fatalf("brain digest cursor was not advanced: %v", err)
	}
	if _, err := os.Stat(narratordigest.CursorPath(root)); !os.IsNotExist(err) {
		t.Fatalf("start delivery changed the human digest cursor: %v", err)
	}
}
