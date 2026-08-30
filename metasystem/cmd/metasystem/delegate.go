package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const delegateClaimCapabilityEnv = "METASYSTEM_DELEGATE_CLAIM_CAPABILITY"

type delegateOutcome struct {
	Outcome  string `json:"outcome"`
	Headline string `json:"headline"`
	JobID    string `json:"jobId,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// runDelegate is the operator boundary. The shell remains the custody
// implementation while its callbacks migrate; operator callers receive only
// typed JSON and dispatch.sh itself immediately calls this verb.
func runDelegate(args []string) int {
	root, err := upMetasystemRoot(os.Getenv("METASYSTEM_DELEGATE_ROOT"))
	if err != nil {
		printJSON(delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
		return 1
	}
	if len(args) > 0 && args[0] == "--adapter-selftest" && !delegateSelftestInternalAuthorized(root) {
		printJSON(delegateOutcome{
			Outcome: "REFUSED-REQUEST", Headline: "refused",
			Detail: "delegate --adapter-selftest is reserved for the metasystem adapter self-test",
		})
		return 2
	}
	internalArgs, mode, err := normalizeDelegateArgs(args)
	if err != nil {
		printJSON(delegateOutcome{Outcome: "REFUSED-REQUEST", Headline: "refused", Detail: err.Error()})
		return 2
	}
	outcomeFile, err := os.CreateTemp("", "metasystem-delegate-outcome.*")
	if err != nil {
		printJSON(delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
		return 1
	}
	outcomePath := outcomeFile.Name()
	_ = outcomeFile.Close()
	defer os.Remove(outcomePath)
	claimCapability := ""
	if mode == "dispatch" || mode == "follow-up" {
		dispatchMode := dispatchcore.DispatchModeFresh
		if mode == "follow-up" {
			dispatchMode = dispatchcore.DispatchModeFollowUp
		}
		claimCapability, err = dispatchcore.MintDelegateClaimCapability(root, dispatchMode)
		if err != nil {
			printJSON(delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: err.Error()})
			return 1
		}
		defer dispatchcore.RemoveDelegateClaimCapability(root, claimCapability)
	}

	command := exec.Command(filepath.Join(root, "scripts", "agents", "dispatch.sh"), internalArgs...)
	command.Env = delegateCommandEnvironment(os.Environ(), outcomePath, claimCapability)
	command.Stdin = os.Stdin
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.MultiWriter(os.Stderr, &stderr)
	runErr := command.Run()
	exitCode := commandExitCode(runErr)

	if encoded, readErr := os.ReadFile(outcomePath); readErr == nil && json.Valid(bytes.TrimSpace(encoded)) && len(bytes.TrimSpace(encoded)) > 0 {
		printDelegateOutcome(bytes.TrimSpace(encoded))
		return exitCode
	}
	if encoded := bytes.TrimSpace(stdout.Bytes()); json.Valid(encoded) && len(encoded) > 0 {
		fmt.Println(string(encoded))
		return exitCode
	}
	if exitCode == 0 {
		job := strings.TrimSpace(stdout.String())
		if line, _, found := strings.Cut(job, "\n"); found {
			job = line
		}
		outcome := "WON"
		headline := "started"
		if mode == "cancel" {
			outcome, headline, job = "CANCELLED", "cancelled", delegateTarget(args)
		}
		printJSON(delegateOutcome{Outcome: outcome, Headline: headline, JobID: job})
		return 0
	}
	detail := delegateInternalRefusalDetail(stderr.String(), runErr, exitCode)
	printJSON(delegateOutcome{Outcome: "REFUSED-INTERNAL", Headline: "refused", Detail: detail})
	return exitCode
}

func delegateCommandEnvironment(base []string, outcomePath, claimCapability string) []string {
	blocked := map[string]bool{
		"METASYSTEM_DELEGATE_INTERNAL":     true,
		"METASYSTEM_DELEGATE_OUTCOME_FILE": true,
		delegateClaimCapabilityEnv:         true,
	}
	environment := make([]string, 0, len(base)+3)
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked[key] {
			environment = append(environment, entry)
		}
	}
	environment = append(environment,
		"METASYSTEM_DELEGATE_INTERNAL=1",
		"METASYSTEM_DELEGATE_OUTCOME_FILE="+outcomePath,
	)
	if claimCapability != "" {
		environment = append(environment, delegateClaimCapabilityEnv+"="+claimCapability)
	}
	return environment
}

func delegateInternalRefusalDetail(stderr string, runErr error, exitCode int) string {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return detail
	}
	if runErr != nil {
		return runErr.Error()
	}
	return fmt.Sprintf("delegate internal exited with status %d without detail", exitCode)
}

// delegateSelftestInternalAuthorized keeps the fixed self-test grammar behind
// its two actual orchestrators. The environment marker is necessary but not
// sufficient: the live parent must also be the same binary's self-test verb or
// the repository's fixed fake-adapter self-test script.
func delegateSelftestInternalAuthorized(root string) bool {
	if os.Getenv("METASYSTEM_DELEGATE_SELFTEST_INTERNAL") != "1" {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	self = resolvedDelegatePath(self)
	parent := int64(os.Getppid())
	exact, state, err := (identity.KernelProber{}).Probe(parent)
	if err != nil || state != identity.Alive || !exact.ArgvKnown {
		return false
	}
	executable, ok := identity.ExecutablePath(parent)
	if !ok {
		return false
	}
	executable = resolvedDelegatePath(executable)
	if executable == self && len(exact.Argv) >= 3 && exact.Argv[1] == "adapter" && exact.Argv[2] == "selftest-run" {
		return true
	}
	fakeAdapter := resolvedDelegatePath(filepath.Join(root, "scripts", "agents", "adapters", "fake.sh"))
	for index := 0; index+1 < len(exact.Argv) && index < 2; index++ {
		if resolvedDelegatePath(exact.Argv[index]) == fakeAdapter && exact.Argv[index+1] == "selftest" {
			return filepath.Base(executable) == "bash"
		}
	}
	return false
}

func resolvedDelegatePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func printDelegateOutcome(encoded []byte) {
	var object map[string]any
	if json.Unmarshal(encoded, &object) != nil {
		fmt.Println(string(encoded))
		return
	}
	outcome, _ := object["outcome"].(string)
	if _, present := object["headline"]; !present {
		switch {
		case outcome == "WON":
			object["headline"] = "started"
		case outcome == "IN-PROGRESS" || outcome == "BOUND" || outcome == "RECONCILING" || strings.HasPrefix(outcome, "REPLAYED-"):
			object["headline"] = "already running"
		default:
			object["headline"] = "refused"
		}
	}
	if _, present := object["jobId"]; !present {
		if evidence, ok := object["evidence"].(map[string]any); ok {
			if recordPath, ok := evidence["recordPath"].(string); ok && recordPath != "" {
				object["jobId"] = strings.TrimSuffix(filepath.Base(recordPath), ".json")
			}
		}
	}
	printJSON(object)
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return 1
}

func normalizeDelegateArgs(args []string) ([]string, string, error) {
	if len(args) == 0 {
		return nil, "", fmt.Errorf("usage: metasystem delegate --role <role> --brief <file> --goal <id|none-explicit> [--op <id>] [--reviews <job-id>]")
	}
	if args[0] == "--cancel" {
		if len(args) != 2 {
			return nil, "", fmt.Errorf("delegate --cancel requires exactly one job id")
		}
		return []string{"cancel", "--job", args[1]}, "cancel", nil
	}
	if args[0] == "--revive" {
		if len(args) != 2 || args[1] == "" {
			return nil, "", fmt.Errorf("delegate --revive requires exactly one steward intent")
		}
		return []string{"dispatch", "--steward-intent", args[1]}, "dispatch", nil
	}
	if args[0] == "--adapter-selftest" {
		if (len(args) != 8 && len(args) != 9) || args[1] == "" || args[2] != "--brief" || args[3] == "" || args[4] != "--workspace" || args[5] == "" || args[6] != "--op" || args[7] == "" || (len(args) == 9 && args[8] != "--wait") {
			return nil, "", fmt.Errorf("delegate --adapter-selftest requires <runtime> --brief <file> --workspace <dir> --op <id>")
		}
		out := []string{"dispatch", "--role", "design-critic", "--brief", args[3], "--runtime", args[1], "--workspace", args[5], "--permissions", "none", "--job-id", args[7], "--destructive-reach", "MECHANICAL"}
		if len(args) == 9 {
			out = append(out, "--wait")
		}
		return out, "dispatch", nil
	}
	if args[0] == "--follow-up" {
		if len(args) < 4 || args[1] == "" {
			return nil, "", fmt.Errorf("delegate --follow-up requires a job id and --brief <file>")
		}
		out := []string{"follow-up", "--job", args[1]}
		briefSeen := false
		opSeen := false
		for index := 2; index < len(args); index++ {
			switch args[index] {
			case "--brief":
				if index+1 >= len(args) || briefSeen {
					return nil, "", fmt.Errorf("delegate --follow-up requires exactly one --brief <file>")
				}
				briefSeen = true
				out = append(out, "--message", args[index+1])
				index++
			case "--approved-ref":
				if index+1 >= len(args) {
					return nil, "", fmt.Errorf("delegate --approved-ref requires a value")
				}
				out = append(out, args[index], args[index+1])
				index++
			case "--op":
				if index+1 >= len(args) || opSeen {
					return nil, "", fmt.Errorf("delegate --follow-up accepts at most one --op value")
				}
				opSeen = true
				out = append(out, "--operation-id", args[index+1])
				index++
			case "--wait":
				out = append(out, args[index])
			default:
				return nil, "", fmt.Errorf("delegate --follow-up does not accept %s", args[index])
			}
		}
		if !briefSeen {
			return nil, "", fmt.Errorf("delegate --follow-up requires --brief <file>")
		}
		return out, "follow-up", nil
	}

	goalSeen := false
	roleSeen := false
	role := ""
	briefSeen := false
	opSeen := false
	reviewsSeen := false
	destructiveReachSeen := false
	out := []string{"dispatch"}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--goal":
			if index+1 >= len(args) || goalSeen {
				return nil, "", fmt.Errorf("delegate requires exactly one --goal <id|none-explicit>")
			}
			goalSeen = true
			if args[index+1] != "none-explicit" {
				out = append(out, "--goal", args[index+1])
			}
			index++
		case "--role", "--brief":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			if args[index] == "--role" {
				if roleSeen {
					return nil, "", fmt.Errorf("delegate requires exactly one --role")
				}
				roleSeen = true
				role = args[index+1]
			} else {
				if briefSeen {
					return nil, "", fmt.Errorf("delegate requires exactly one --brief")
				}
				briefSeen = true
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--op":
			if index+1 >= len(args) || opSeen {
				return nil, "", fmt.Errorf("delegate accepts at most one --op value")
			}
			opSeen = true
			out = append(out, "--job-id", args[index+1])
			index++
		case "--reviews":
			if index+1 >= len(args) || reviewsSeen {
				return nil, "", fmt.Errorf("delegate accepts at most one --reviews value")
			}
			reviewsSeen = true
			out = append(out, "--reviews", args[index+1])
			index++
		case "--destructive-reach":
			if index+1 >= len(args) || destructiveReachSeen {
				return nil, "", fmt.Errorf("delegate requires exactly one --destructive-reach")
			}
			destructiveReachSeen = true
			value := args[index+1]
			if value != "MECHANICAL" && value != "DESIGN-BEARING" && value != "DESTRUCTIVE-REACH" {
				return nil, "", fmt.Errorf("delegate --destructive-reach must be MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
			}
			out = append(out, "--destructive-reach", value)
			index++
		case "--approved-ref", "--source":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("delegate %s requires a value", args[index])
			}
			out = append(out, args[index], args[index+1])
			index++
		case "--wait":
			out = append(out, args[index])
		default:
			return nil, "", fmt.Errorf("delegate does not accept %s", args[index])
		}
	}
	if !goalSeen || !roleSeen || !briefSeen || !destructiveReachSeen {
		return nil, "", fmt.Errorf("delegate requires --role, --brief, --goal <id|none-explicit>, and --destructive-reach <class>")
	}
	if reviewsSeen && role != "code-critic" && role != "warden" && role != "verifier" {
		return nil, "", fmt.Errorf("--reviews is only valid for the code-critic, warden, and verifier roles")
	}
	return out, "dispatch", nil
}

func delegateTarget(args []string) string {
	if len(args) >= 2 {
		return args[1]
	}
	return ""
}
