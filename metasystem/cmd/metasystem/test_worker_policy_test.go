package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestDefaultTestingWorkersSharesCapturedCapacityAcrossAdmission(t *testing.T) {
	t.Parallel()
	for _, specimen := range []struct {
		gomax, admission, want int
	}{
		{gomax: 1, admission: 8, want: 1},
		{gomax: 18, admission: 3, want: 6},
		{gomax: 7, admission: 0, want: 7},
		{gomax: 0, admission: 0, want: 1},
	} {
		if got := defaultTestingWorkers(specimen.gomax, specimen.admission); got != specimen.want {
			t.Errorf("default workers gomax=%d admission=%d got=%d want=%d", specimen.gomax, specimen.admission, got, specimen.want)
		}
	}
}

func TestTestingWorkersAcceptsLargeExplicitValueAndReportsAdmission(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("testing.workers=4096\n"+proofrun.AdmissionCapKey+"=0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	limits, err := resolveProofRunLimitsWithEnvironment(conf, func(string) (string, bool) { return "", false })
	if err != nil || limits.workers != 4096 || limits.admissionMaximum != 0 {
		t.Fatalf("resolved limits=%+v err=%v", limits, err)
	}
	if problems, err := proofRunConfigProblems(conf); err != nil || len(problems) != 0 {
		t.Fatalf("large explicit worker value problems=%v err=%v", problems, err)
	}
	for _, value := range []string{"0", "-1", "many"} {
		if err := os.WriteFile(conf, []byte("testing.workers="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := resolveProofRunLimitsWithEnvironment(conf, func(string) (string, bool) { return "", false }); err == nil || !strings.Contains(err.Error(), "testing.workers must be a positive integer") {
			t.Fatalf("invalid workers %q error=%v", value, err)
		}
	}
}

func TestInheritedTestingWorkerCeilingCapsConfiguredAllowanceAndRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("testing.workers=4\n"+proofrun.AdmissionCapKey+"=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	limits, err := resolveProofRunLimitsWithEnvironment(conf, func(string) (string, bool) { return "1", true })
	if err != nil || limits.workers != 1 {
		t.Fatalf("nested workers=%d want=1 err=%v", limits.workers, err)
	}
	request := testingRunRequest(testingPreparation{Workers: limits.workers, AdmissionMaximum: limits.admissionMaximum}, "", "", "", "", "")
	result := proofrun.NewTestResult(request)
	if request.Workers != 1 || result.Workers != 1 || result.WorkerPolicyVersion != proofrun.TestWorkerPolicyVersion {
		t.Fatalf("nested resolved allowance did not bind request/result: request=%+v result=%+v", request, result)
	}
	for _, malformed := range []string{"0", "-1", "many"} {
		t.Run(malformed, func(t *testing.T) {
			t.Parallel()
			if _, err := resolveProofRunLimitsWithEnvironment(conf, func(string) (string, bool) { return malformed, true }); err == nil || !strings.Contains(err.Error(), "inherited "+proofrun.TestWorkersEnvironment+" must be a positive integer") {
				t.Fatalf("malformed inherited ceiling %q error=%v", malformed, err)
			}
		})
	}
}

func TestResolvedTestWorkerEnvironmentReplacesAmbientAuthority(t *testing.T) {
	t.Parallel()
	environment := resolvedTestWorkerEnvironment([]string{"PATH=/bin", proofrun.TestWorkersEnvironment + "=999", "VALUE=kept"}, 5)
	found := 0
	for _, entry := range environment {
		if strings.HasPrefix(entry, proofrun.TestWorkersEnvironment+"=") {
			found++
			if entry != proofrun.TestWorkersEnvironment+"=5" {
				t.Fatalf("worker environment=%q", entry)
			}
		}
	}
	if found != 1 || !strings.Contains(strings.Join(environment, "\n"), "VALUE=kept") {
		t.Fatalf("resolved environment=%v", environment)
	}
}

func TestTestingRunRequestCarriesResolvedWorkerPolicy(t *testing.T) {
	t.Parallel()
	request := testingRunRequest(testingPreparation{Workers: 8, AdmissionMaximum: 3}, "", "", "", "", "")
	if request.Workers != 8 || request.AdmissionMaximum != 3 {
		t.Fatalf("testing request workers=%d admission=%d", request.Workers, request.AdmissionMaximum)
	}
	result := proofrun.NewTestResult(request)
	if result.SchemaVersion != proofrun.TestResultSchemaVersion || result.WorkerPolicyVersion != proofrun.TestWorkerPolicyVersion ||
		result.Workers != 8 || result.AdmissionMaximum == nil || *result.AdmissionMaximum != 3 {
		t.Fatalf("testing result did not bind resolved worker policy: %+v", result)
	}
}

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

func TestConfiguredTopLevelWorkerOverrideRemainsUncappedWithoutInheritedCeiling(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	const want = 4096
	if err := os.WriteFile(conf, []byte("testing.workers="+strconv.Itoa(want)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	limits, err := resolveProofRunLimitsWithEnvironment(conf, func(string) (string, bool) { return "", false })
	if err != nil || limits.workers != want {
		t.Fatalf("top-level explicit workers=%d want=%d err=%v", limits.workers, want, err)
	}
}
