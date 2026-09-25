package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const proofCommandFixtureMarker = "GO_WANT_PROOF_COMMAND_FIXTURE_CHILD"
const proofCommandFixtureSnapshotEnv = "GO_PROOF_COMMAND_FIXTURE_SNAPSHOT"
const proofCommandFixtureTempRootEnv = "GO_PROOF_COMMAND_FIXTURE_TEMP_ROOT"
const proofCommandFixtureTestArg = "-test.run=^TestProofCommandFixtureChild$"

var proofCommandFixtureChildStatus int

type proofCommandFixtureSnapshot struct {
	Top     string            `json:"top"`
	Root    string            `json:"root"`
	Machine string            `json:"machine"`
	Files   map[string][]byte `json:"files"`
}

// writeProofCommandFixtureSnapshot fixes the accepted tree before command
// processes start. Runtime accounting may change physical files afterward.
func writeProofCommandFixtureSnapshot(t *testing.T, repository *proofAdmissionRepository, selectedMachine ...string) string {
	t.Helper()
	if len(selectedMachine) > 1 {
		t.Fatal("proof command fixture accepts one machine identity")
	}
	machine, err := repository.reads().ResolveMachine(repository.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(selectedMachine) == 1 {
		machine = selectedMachine[0]
	}
	top, err := filepath.EvalSymlinks(repository.top)
	if err != nil {
		t.Fatal(err)
	}
	repository.top, repository.root = top, filepath.Join(top, "metasystem")
	for _, directory := range []string{repository.top, repository.root} {
		if _, err := os.Lstat(filepath.Join(directory, ".git")); !os.IsNotExist(err) {
			t.Fatalf("proof command fixture contains .git at %s: %v", directory, err)
		}
	}
	repository.mu.Lock()
	files := proofAdmissionClone(repository.commits[repository.accepted].files)
	repository.mu.Unlock()
	claimedMachine := false
	for name, data := range files {
		if !strings.HasPrefix(name, "metasystem/plans/goals/") || name == "metasystem/plans/goals/backlog.md" {
			continue
		}
		file, problems := goal.ParseFile(data)
		if len(problems) == 0 && file != nil && file.Claimed != nil && file.Claimed.Machine == machine {
			claimedMachine = true
		}
	}
	if machine == "" || !claimedMachine {
		t.Fatalf("proof command fixture machine %q has no accepted claimed goal", machine)
	}
	for name, data := range files {
		path := filepath.Join(repository.top, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		physical, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(physical, data) {
			t.Fatalf("accepted file %s differs from physical bytes: %v", name, err)
		}
	}
	data, err := json.Marshal(proofCommandFixtureSnapshot{Top: repository.top, Root: repository.root, Machine: machine, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accepted-proof-tree.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// proofCommandFixtureRepository reconstructs the accepted repository without
// consulting a physical Git history.
func proofCommandFixtureRepository(t *testing.T, snapshotPath string) *proofAdmissionRepository {
	t.Helper()
	snapshot := readProofCommandFixtureSnapshot(t, snapshotPath)
	repository := &proofAdmissionRepository{top: snapshot.Top, root: snapshot.Root,
		commits: map[string]proofAdmissionCommit{}, operations: map[string]string{}}
	repository.seed(snapshot.Files)
	return repository
}

func readProofCommandFixtureSnapshot(t *testing.T, snapshotPath string) proofCommandFixtureSnapshot {
	t.Helper()
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot proofCommandFixtureSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Top == "" || snapshot.Root == "" || snapshot.Machine == "" || filepath.Join(snapshot.Top, "metasystem") != snapshot.Root || len(snapshot.Files) == 0 {
		t.Fatalf("invalid proof command fixture snapshot at %s", snapshotPath)
	}
	return snapshot
}

func proofCommandFixtureWrapper(t *testing.T) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proof-command")
	quoted := "'" + strings.ReplaceAll(binary, "'", "'\\''") + "'"
	content := "#!/bin/sh\nexec " + quoted + " " + proofCommandFixtureTestArg + " -- \"$@\"\n"
	if err := testexec.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func proofCommandFixtureArguments() ([]string, bool) {
	if os.Getenv(proofCommandFixtureMarker) != "1" || len(os.Args) < 5 ||
		os.Args[1] != proofCommandFixtureTestArg || os.Args[2] != "--" {
		return nil, false
	}
	args := os.Args[3:]
	if args[0] == "proof-run" && args[1] == "launch" || args[0] == "landing" && args[1] == "test-receipt" {
		return args, true
	}
	return nil, false
}

func TestProofCommandFixtureChild(t *testing.T) {
	args, ok := proofCommandFixtureArguments()
	if !ok {
		return
	}
	if err := os.Setenv("TMPDIR", os.Getenv(proofCommandFixtureTempRootEnv)); err != nil {
		t.Fatal(err)
	}
	snapshotPath := os.Getenv(proofCommandFixtureSnapshotEnv)
	snapshot := readProofCommandFixtureSnapshot(t, snapshotPath)
	repository := proofCommandFixtureRepository(t, snapshotPath)
	reads := repository.reads()
	reads.ResolveMachine = func(root string) (string, error) {
		if root != repository.root {
			return "", fmt.Errorf("proof machine root %q differs from %q", root, repository.root)
		}
		return snapshot.Machine, nil
	}
	admit := func(request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
		request.BeforePublish = func(reservation *proofrun.AdmissionRequest) {
			*reservation = privateProofAdmissionRequest(proofrun.WithTestHostLoadSampler(*reservation, "0"))
		}
		classify := func(root string, pid int64) (lease.ClassifyResult, error) {
			return classifyVerbCallerWith(root, pid, reads.Receipt.TopLevel)
		}
		return admitProofLaunchWithReadsAndClassifier(request, func() dispatchcore.ProofAdmissionReads { return reads }, classify)
	}
	terminal := func(completion proofrun.CompletionContext, receipt json.RawMessage, testResult *proofrun.TestResult) error {
		return commitProofTerminalWithReasonAndReads(completion, receipt, testResult, "proof launcher completed", &reads)
	}
	switch args[0] {
	case "proof-run":
		proofCommandFixtureChildStatus = runProofRunLaunchWithInputs(args[2:], admit, terminal)
	case "landing":
		fixture := newDeadlineReceiptTreeFixture(t, repository)
		proofCommandFixtureChildStatus = runLandingTestReceiptWithInputs(context.Background(), goalCommandClock,
			fixture.strictRaw, landingReceiptTestRun, args[2:],
			func(root, tree, command string) (*landing.ReceiptPreparation, error) {
				return landing.PrepareTestReceiptWithWorkspace(root, tree, command, fixture.workspace(),
					func(root, tree string) (proofrun.FrozenExport, error) {
						frozen, err := proofrun.Freeze(root)
						if err == nil {
							fixture.frozen = frozen.Root
							fixture.checkFiles(frozen.Root)
						}
						return frozen, err
					})
			}, admit,
			func(root, attemptID, tree string, now time.Time) (landing.TestReceipt, error) {
				return landing.PublishCommittedReceiptAtWithWorkspace(root, attemptID, tree, now, fixture.workspace())
			},
			func(completion proofrun.CompletionContext, receipt json.RawMessage) error {
				return terminal(completion, receipt, nil)
			})
	}
}
