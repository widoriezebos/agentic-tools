package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestWorkerCapabilitiesCommandReportsExactProtocol(t *testing.T) {
	t.Parallel()
	var stdout strings.Builder
	err := writeTestingWorkerCapabilities(&stdout)
	var got testingWorkerCapabilities
	if decodeErr := json.Unmarshal([]byte(stdout.String()), &got); err != nil || decodeErr != nil || got != currentTestingWorkerCapabilities() {
		t.Fatalf("worker capabilities stdout=%q got=%+v write=%v decode=%v", stdout.String(), got, err, decodeErr)
	}
}

const strictBaselineWorkerHelperEnvironment = "GO_WANT_STRICT_BASELINE_WORKER_HELPER"

func TestStrictBaselineWorkerDecoderHelper(t *testing.T) {
	t.Parallel()
	if os.Getenv(strictBaselineWorkerHelperEnvironment) != "1" {
		return
	}
	separator := -1
	for index, arg := range os.Args {
		if arg == "--" {
			separator = index
		}
	}
	if separator < 0 || separator+2 >= len(os.Args) || os.Args[separator+1] != "test" {
		os.Exit(97)
	}
	args := os.Args[separator+1:]
	switch args[1] {
	case "worker-capabilities":
		fmt.Fprintln(os.Stderr, "unknown test verb worker-capabilities")
		os.Exit(2)
	case "worker":
		packet := ""
		for index := 2; index+1 < len(args); index++ {
			if args[index] == "--packet" {
				packet = args[index+1]
			}
		}
		file, err := os.Open(packet)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		var request struct{ ProjectRoot string }
		err = decoder.Decode(&request)
		if err == nil {
			err = func() error {
				if err := decoder.Decode(&struct{}{}); err != io.EOF {
					return fmt.Errorf("trailing request")
				}
				return nil
			}()
		}
		_ = file.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func strictBaselineWorkerFixture(t *testing.T) (string, string) {
	t.Helper()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	engine, calls := filepath.Join(directory, "baseline-engine"), filepath.Join(directory, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >>" + shellQuote(calls) + "\n" +
		"export " + strictBaselineWorkerHelperEnvironment + "=1\nexec " + shellQuote(testBinary) +
		" -test.run '^TestStrictBaselineWorkerDecoderHelper$' -- \"$@\"\n"
	if err := testexec.WriteFile(engine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return engine, calls
}

func TestRunPreflightsStrictRetainedBaselineBeforePreparationOrReservation(t *testing.T) {
	t.Parallel()
	engine, calls := strictBaselineWorkerFixture(t)
	packet := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(packet, []byte(`{"ProjectRoot":"/fixture","Workers":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	baseline := (proofBinaryFixture{t: t}).command(os.Environ(), engine, "test", "worker", "--packet", packet)
	if output, err := baseline.CombinedOutput(); err == nil || !strings.Contains(string(output), `unknown field "Workers"`) {
		t.Fatalf("strict baseline fixture did not reproduce the new-field refusal: err=%v output=%s", err, output)
	}
	if err := os.WriteFile(calls, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	err := requireTestingWorkerCapabilities(t.Context(), engine, []string{"PATH=/usr/bin:/bin"})
	if !errors.Is(err, errTestingWorkerPolicyUnsupported) || !strings.Contains(err.Error(), "stage, prove, and install the backend compatibility release") {
		t.Fatalf("unsupported trusted worker refusal=%v", err)
	}
	data, err := os.ReadFile(calls)
	if err != nil || string(data) != "test worker-capabilities\n" {
		t.Fatalf("trusted baseline calls=%q err=%v", data, err)
	}
}

func TestWorkerPolicyWireStaysLegacyUntilNegotiated(t *testing.T) {
	t.Parallel()
	legacy := proofrun.TestRunRequest{PreparedGroups: map[string]proofrun.PreparedGroupExecution{"group": {}}}
	data, err := json.Marshal(legacy)
	if err != nil || strings.Contains(string(data), `"Workers"`) || strings.Contains(string(data), `"AdmissionMaximum"`) ||
		strings.Contains(string(data), `"workerPolicyVersion"`) || strings.Contains(string(data), `"workers"`) {
		t.Fatalf("legacy request changed old wire: %s err=%v", data, err)
	}
	result := proofrun.NewTestResult(legacy)
	resultData, err := json.Marshal(result)
	if err != nil || result.SchemaVersion != proofrun.PreviousTestResultSchemaVersion || result.WorkerPolicyVersion != 0 || result.Workers != 0 ||
		result.AdmissionMaximum != nil || strings.Contains(string(resultData), `"workerPolicyVersion"`) || strings.Contains(string(resultData), `"workers"`) {
		t.Fatalf("legacy result changed old wire: %+v %s err=%v", result, resultData, err)
	}
	activated := legacy
	activated.Workers, activated.AdmissionMaximum = 1, 2
	activated.PreparedGroups["group"] = proofrun.PreparedGroupExecution{WorkerPolicyVersion: proofrun.TestWorkerPolicyVersion, Workers: 1}
	data, err = json.Marshal(activated)
	if err != nil || !strings.Contains(string(data), `"Workers":1`) || !strings.Contains(string(data), `"workerPolicyVersion":1`) {
		t.Fatalf("negotiated request omitted worker policy: %s err=%v", data, err)
	}
}
