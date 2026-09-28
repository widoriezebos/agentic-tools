package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// TestIntentTestRunRunsTheTestingRunnerInThisProcess is the U9a witness that
// the public test run reaches the testing runner in this process (design
// 6.2): with the production seam, a checkout whose committed configuration
// names no testing contract is refused by the runner's own admission check,
// on this command's standard error, with the runner's exit; no engine child
// (which, in a test, would be this test binary) runs.
func TestIntentTestRunRunsTheTestingRunnerInThisProcess(t *testing.T) {
	t.Parallel()
	b := newIntentBed(t, false, nil)
	conf := filepath.Join(b.root(), "metasystem.conf")
	data, err := os.ReadFile(conf)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "testing.contract=") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(conf, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	command, rest, _ := resolveIntentArgv([]string{"test", "run"})
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append(rest, "--json"), &stdout, &stderr, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("no JSON result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if code != 1 || !strings.Contains(stderr.String(), "metasystem test run: testing.contract is required in committed metasystem.conf") {
		t.Fatalf("test run without a contract = %d %+v; stderr=%q", code, result, stderr.String())
	}
}

// TestProofAdmissionClassifiesTheSuppliedCaller: proof admission classifies
// the caller its request supplies (the command that runs the testing runner
// in its own process supplies itself), and an entry, which supplies none,
// classifies its own parent as before.
func TestProofAdmissionClassifiesTheSuppliedCaller(t *testing.T) {
	t.Parallel()
	var classified []int64
	classify := func(_ string, pid int64) (lease.ClassifyResult, error) {
		classified = append(classified, pid)
		return lease.ClassifyResult{}, errors.New("stop after classification")
	}
	request := proofLaunchAdmission{ControlRoot: t.TempDir(), CallerPID: int64(os.Getpid())}
	if _, _, _, err := admitProofLaunchWithReadsAndClassifier(request, nil, classify); err == nil {
		t.Fatal("the stub classification error was not returned")
	}
	request.CallerPID = 0
	admitProofLaunchWithReadsAndClassifier(request, nil, classify)
	if want := []int64{int64(os.Getpid()), int64(os.Getppid())}; !slices.Equal(classified, want) {
		t.Fatalf("classified %v, want %v", classified, want)
	}
	if got := (testingSelectionRequest{CallerPID: 42}).callerPID(); got != 42 {
		t.Fatalf("the selection request's supplied caller = %d", got)
	}
}
