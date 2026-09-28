package proofrun

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"os"
	"strconv"
	"strings"
	"testing"
)

func readNULFields(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(string(data), "\x00")
	return fields[:len(fields)-1]
}

func TestProofScriptEntrypointHelper(t *testing.T) {
	if os.Getenv("GO_WANT_PROOF_ENTRYPOINT_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+2 > len(os.Args) {
		os.Exit(97)
	}
	mode, arguments := os.Args[separator+1], os.Args[separator+2:]
	status, _ := strconv.Atoi(os.Getenv("PROOF_ENTRYPOINT_STATUS"))
	if mode == "go" {
		if len(arguments) > 0 && arguments[0] == "build" {
			output := ""
			for index := range arguments {
				if arguments[index] == "-o" && index+1 < len(arguments) {
					output = arguments[index+1]
				}
			}
			if output == "" {
				os.Exit(97)
			}
			writeEntrypointWrapperNow(output, "engine")
			os.Exit(0)
		}
		os.Exit(status)
	}
	joined := strings.Join(arguments, " ")
	switch {
	case strings.HasPrefix(joined, "proof-run worker-authorized "):
		status, found := os.LookupEnv("PROOF_ENTRYPOINT_WORKER_STATUS")
		if !found {
			os.Exit(3)
		}
		parsed, _ := strconv.Atoi(status)
		os.Exit(parsed)
	case strings.HasPrefix(joined, "proof-run coverage-eligible "):
		status, _ := strconv.Atoi(os.Getenv("PROOF_ENTRYPOINT_ELIGIBILITY_STATUS"))
		os.Exit(status)
	case strings.HasPrefix(joined, "proof-run coverage-begin "):
		path := os.Getenv("PROOF_ENTRYPOINT_COVERAGE_BEGIN_COUNT")
		begins := 1
		if raw, err := os.ReadFile(path); err == nil {
			begins, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			begins++
		}
		if err := os.WriteFile(path, []byte(strconv.Itoa(begins)+"\n"), 0o600); err != nil {
			os.Exit(97)
		}
		os.Exit(0)
	case strings.HasPrefix(joined, "proof-run banner "):
		fmt.Println("entrypoint fixture banner")
		os.Exit(0)
	case strings.HasPrefix(joined, "proof-run launch "):
		if path := os.Getenv("PROOF_ENTRYPOINT_GOFLAGS_OUT"); path != "" {
			if err := os.WriteFile(path, []byte(os.Getenv("GOFLAGS")), 0o600); err != nil {
				os.Exit(97)
			}
		}
		path := os.Getenv("PROOF_ENTRYPOINT_LAUNCH_COUNT")
		launches := 1
		if raw, err := os.ReadFile(path); err == nil {
			launches, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			launches++
		}
		_ = os.WriteFile(path, []byte(strconv.Itoa(launches)+"\n"), 0o600)
		os.Exit(status)
	default:
		os.Exit(97)
	}
}

func writeEntrypointWrapperNow(path, mode string) error {
	wrapper := "#!/usr/bin/env bash\nexec \"$PROOF_ENTRYPOINT_HELPER\" -test.run=^TestProofScriptEntrypointHelper$ -- " + mode + " \"$@\"\n"
	return testexec.WriteFile(path, []byte(wrapper), 0o755)
}

func filteredCoverageScriptEnvironment() []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		// The full gate arms a witness for its nested validations and then runs
		// this package's tests; a focused fast gate spawned here must not inherit
		// that handoff, which fast mode refuses by law.
		if strings.HasPrefix(key, "METASYSTEM_PROOF_") || strings.HasPrefix(key, "METASYSTEM_SUITE_PROGRESS_") || strings.HasPrefix(key, "METASYSTEM_GATE_WITNESS") {
			continue
		}
		switch key {
		case "GO_WANT_COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_HELPER", "COVERAGE_SCRIPT_REUSE_STATUS", "COVERAGE_SCRIPT_LAUNCH_COUNT", "FAST_GATE_BUILD_COUNT", "FAST_GATE_RUN_OWNER_ANSWER", "FAST_GATE_RUN_OWNER_SEEN", "METASYSTEM_BIN", "METASYSTEM_PROOFRUN_TEST_CANDIDATE_ENGINE", "METASYSTEM_RUN_OWNER", "METASYSTEM_GO_GATE_RELAUNCHED", "METASYSTEM_VALIDATE_RELAUNCHED", "METASYSTEM_ADOPT_FIXTURES_RELAUNCHED", "METASYSTEM_COVERAGE_DELTA_RELAUNCHED", "PATH":
			continue
		}
		result = append(result, entry)
	}
	return result
}
