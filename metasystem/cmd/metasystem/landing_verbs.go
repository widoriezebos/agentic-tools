package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type landingRepeatedStrings []string

func (values *landingRepeatedStrings) String() string { return fmt.Sprint([]string(*values)) }
func (values *landingRepeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runLandingObserve(args []string) int {
	flags := flag.NewFlagSet("landing observe", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "prospective project tree")
	chain := flags.String("chain", "", "closed implementation chain root")
	directFix := flags.String("direct-fix", "", "typed direct-fix class; register-carriage may accompany --chain")
	revertOf := flags.String("revert-of", "", "commit inverted by exact-revert")
	goal := flags.String("goal", "", "goal item carried by the landing")
	actor := flags.String("actor", "", "wrapper actor as machine+lineage")
	rootJob := flags.String("root-job", "", "tier-1 root implementer job")
	testReceipt := flags.String("test-receipt", "", "candidate test receipt path")
	recertification := flags.String("recertification", "", "canonical recertification record path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	observation := landing.Observe(landing.ObserveParams{
		RepoRoot: *root, CandidateTree: *tree, Chain: *chain,
		DirectFix: *directFix, RevertOf: *revertOf, Goal: *goal, Actor: *actor,
		RootJob: *rootJob, TestReceipt: *testReceipt, Recertification: *recertification,
	})
	printJSON(observation)
	return 0
}

func runLandingTestReceipt(args []string) (status int) {
	flags := flag.NewFlagSet("landing test-receipt", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "candidate project tree")
	command := flags.String("command", "", "test command to run from the isolated candidate workspace")
	mode := flags.String("mode", "", "shared testing mode: auto, standard, or deep")
	goalID := flags.String("goal", "", "accepted goal owning the receipt proof")
	capMin := flags.String("cap-min", "", "reserved proof minutes")
	retryDecision := flags.String("retry-decision", "", "accountable retry decision")
	resultPath := flags.String("result", "", "atomic structured launch result path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if (*mode == "") == (*command == "") {
		fmt.Fprintln(os.Stderr, "landing test-receipt requires exactly one of --mode or legacy --command")
		return 2
	}
	controlRoot, err := canonicalProofRoot(*root)
	if err != nil {
		return recordExit(err)
	}
	if *mode != "" {
		if *mode != "auto" && *mode != "standard" && *mode != "deep" {
			fmt.Fprintln(os.Stderr, "landing test-receipt --mode must be auto, standard, or deep")
			return 2
		}
		resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "delivery", "testing-result-"+*tree+".json")
		testArgs := []string{"--root", controlRoot, "--tree", *tree, "--mode", *mode, "--purpose", "delivery", "--result", resultPath}
		if *goalID != "" {
			testArgs = append(testArgs, "--goal", *goalID)
		}
		if *capMin != "" {
			testArgs = append(testArgs, "--cap-min", *capMin)
		}
		if *retryDecision != "" {
			testArgs = append(testArgs, "--retry-decision", *retryDecision)
		}
		status := runTestRun(testArgs)
		if status != 0 && status != proofrun.ExitReusableSuccess {
			return status
		}
		var result proofrun.TestResult
		if err := readStrictJSON(resultPath, &result); err != nil {
			return recordExit(fmt.Errorf("read shared testing result: %w", err))
		}
		var receipt landing.TestReceipt
		if result.AttemptID != "" {
			receipt, err = landing.PublishCommittedReceipt(controlRoot, result.AttemptID)
		} else {
			// A verifier may lawfully compose unchanged successful groups from
			// several older outer attempts. No new execution or synthetic
			// attempt is created for that schema-2 projection.
			receipt, err = landing.CreateTestingReceipt(controlRoot, *tree, result)
		}
		if err != nil {
			return recordExit(err)
		}
		printJSON(receipt)
		return 0
	}
	preparation, err := landing.PrepareTestReceipt(controlRoot, *tree, *command)
	if err != nil {
		return recordExit(err)
	}
	defer func() {
		if closeErr := preparation.Close(); closeErr != nil {
			fmt.Fprintln(os.Stderr, "landing test-receipt: preserve detached suite-failure evidence:", closeErr)
			if status == 0 {
				status = 1
			}
		}
	}()
	confPath := filepath.Join(preparation.ExecutionRoot(), "metasystem.conf")
	executionEnvironment := []string(nil)
	var expected []string
	if *command == landing.CanonicalValidatorCommand {
		if proofRunAlternateGoInputs() {
			return recordExit(fmt.Errorf("canonical validator refuses GOFLAGS containing -modfile or -overlay"))
		}
		executionEnvironment = canonicalValidatorEnvironment()
		expected, _, err = selectedSections(filepath.Join(preparation.ExecutionRoot(), "scripts", "agents", "validate-section-selector.sh"), "", false)
		if err != nil {
			return recordExit(err)
		}
	}
	limits, err := resolveProofRunLimits(confPath)
	if err != nil {
		return recordExit(err)
	}
	proofDir := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "delivery")
	if err := os.MkdirAll(proofDir, 0o700); err != nil {
		return recordExit(err)
	}
	attempt, decision, joined, err := admitProofLaunch(proofLaunchAdmission{ControlRoot: controlRoot,
		ExecutionRoot: preparation.ExecutionRoot(), ConfPath: confPath, GoalID: *goalID, CapMin: *capMin,
		RetryDecision: *retryDecision, ScopeClass: "full", CommandClass: "landing-test-receipt", Sections: expected,
		Environment: executionEnvironment})
	if err != nil {
		decision = proofrun.LaunchResult{SchemaVersion: 1, Disposition: proofrun.DispositionAdmissionRefused, ExitStatus: proofrun.ExitAdmissionRefused}
		_ = proofrun.EncodeResult(os.Stderr, *resultPath, decision)
		fmt.Fprintln(os.Stderr, "landing test-receipt:", err)
		return proofrun.ExitAdmissionRefused
	}
	if decision.Disposition != proofrun.DispositionExecuted {
		if decision.Disposition == proofrun.DispositionReusableSuccess {
			if _, err := landing.PublishCommittedReceipt(controlRoot, decision.AttemptID); err != nil {
				return recordExit(err)
			}
		}
		if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
			return recordExit(err)
		}
		return decision.ExitStatus
	}
	deadline, _ := time.Parse(time.RFC3339Nano, attempt.Deadline)
	status = proofrun.LaunchSuite(proofrun.LaunchOptions{Suite: "landing-receipt", Root: preparation.ExecutionRoot(),
		ControlRoot: controlRoot, AttemptID: attempt.AttemptID, JoinedAttempt: joined, Deadline: deadline, ConfPath: confPath,
		ProgressPath: filepath.Join(proofDir, attempt.AttemptID+".progress.jsonl"), LogPath: filepath.Join(proofDir, attempt.AttemptID+".log"),
		Banner:  "EXPENSIVE SUITE landing-receipt: one admitted execution; retained result controls repeats",
		Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax, Poll: time.Second,
		TermGrace: 5 * time.Second, KillGrace: time.Second, Command: []string{"bash", "-c", *command}, Output: os.Stdout, ErrorOutput: os.Stderr,
		Environment: executionEnvironment,
		PrepareSuccess: func(completion proofrun.CompletionContext) (json.RawMessage, error) {
			current, err := proofrun.ReadAttempt(controlRoot, attempt.AttemptID)
			if err != nil {
				return nil, err
			}
			receipt, err := preparation.Complete(current, completion.CompletedAt)
			if err != nil {
				return nil, err
			}
			if *command == landing.CanonicalValidatorCommand && receipt.Coverage == nil {
				return nil, fmt.Errorf("canonical validator produced no authenticated coverage component")
			}
			return json.Marshal(receipt)
		}, CommitTerminal: commitProofTerminal})
	status = retainIncompleteProofAttempt(controlRoot, attempt.AttemptID, joined, status)
	decision.ExitStatus = status
	if status != 0 {
		decision.Disposition = proofrun.DispositionFailed
	} else {
		receipt, publishErr := landing.PublishCommittedReceipt(controlRoot, attempt.AttemptID)
		if publishErr != nil {
			fmt.Fprintln(os.Stderr, publishErr)
			status = 1
			decision.ExitStatus = status
			decision.Disposition = proofrun.DispositionFailed
		} else {
			printJSON(receipt)
		}
	}
	if err := proofrun.EncodeResult(os.Stderr, *resultPath, decision); err != nil {
		return recordExit(err)
	}
	return status
}

func canonicalValidatorEnvironment() []string {
	owned := map[string]bool{"GOFLAGS": true, "METASYSTEM_GATE_FROZEN_TOOLCHAIN": true}
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !owned[name] {
			environment = append(environment, entry)
		}
	}
	return append(environment, "GOFLAGS=-mod=readonly", "METASYSTEM_GATE_FROZEN_TOOLCHAIN=1")
}

func runLandingPark(args []string) int {
	flags := flag.NewFlagSet("landing park", flag.ContinueOnError)
	root := flags.String("root", "", "integration project root")
	chain := flags.String("chain", "", "root implementation chain")
	target := flags.String("target", "", "frozen target commit")
	reason := flags.String("reason", "", "original landing refusal code")
	detail := flags.String("detail", "", "specific refusal explanation")
	recertification := flags.String("recertification", "", "diagnostic recertification record path")
	candidate := flags.String("candidate-commit", "", "retained local landing commit")
	var refs landingRepeatedStrings
	flags.Var(&refs, "recovery-ref", "retained source/result anchor ref (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := landing.Park(landing.ParkParams{
		Root: *root, Chain: *chain, TargetCommit: *target, Reason: *reason, Detail: *detail,
		Recertification: *recertification, CandidateCommit: *candidate, RecoveryRefs: refs,
		CallerPID: int64(os.Getppid()),
	})
	if err != nil {
		var failure *landing.ParkFailure
		if errors.As(err, &failure) {
			fmt.Fprintf(os.Stderr, "reason=chain-recertification-park-failed cause=%s error=%v\n", failure.Cause, failure.Err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	fmt.Printf("state=%s\nreason=%s\nparkRecord=%s\n", result.State, result.Reason, result.ParkRecord)
	return 0
}

func runLandingAdoptionRulings(args []string) int {
	flags := flag.NewFlagSet("landing adoption-rulings", flag.ContinueOnError)
	source := flags.String("source", "", "staged template installation")
	target := flags.String("target", "", "application installation")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *source == "" || *target == "" {
		return 2
	}
	data, err := landing.AdoptionRulings(*source, *target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
