package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
)

// The mission-turn and mission-jobs families are the mission runner's
// decision surface (internal/missionrunner). Each verb reads the runner's
// records, judges, and prints a JSON proposal; the runner applies it — writes
// the asks, advances the hash-chained state, and drives the dispatch tooling
// — so every artifact keeps its single writer.

// runnerNowISO is the timestamp format runner artifacts carry.
func runnerNowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// missionRunnerScope validates the two flags every runner verb needs.
func missionRunnerScope(name, root, mission string) bool {
	if root == "" || mission == "" {
		fmt.Fprintf(os.Stderr, "%s: --root and --mission are required\n", name)
		return false
	}
	if !missionIDRe.MatchString(mission) {
		fmt.Fprintf(os.Stderr, "%s: invalid mission id\n", name)
		return false
	}
	return true
}

// runnerDoc reads a JSON object input, labeling the failure for the runner's
// log.
func runnerDoc(path, label string) (map[string]any, error) {
	doc, err := missionrunner.ReadDoc(path)
	if err != nil {
		return nil, fmt.Errorf("%s is unreadable: %s: %v", label, path, err)
	}
	return doc, nil
}

// runMissionTurnAdjudicate validates a host turn's result envelope and
// orchestrator return against the turn's identity, then judges every claim in
// the return against the mission state and job records. It prints a verdict —
// accepted and rejected claims, the updated stream map, the asks to write,
// and the waiting list — for the runner to apply. The return's completeness
// is checked by the shipped role checker so return-schema authority stays in
// one place.
func runMissionTurnAdjudicate(args []string) int {
	flags := flag.NewFlagSet("mission-turn adjudicate", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	turnPath := flags.String("turn", "", "turn record path")
	resultPath := flags.String("result", "", "host result envelope path")
	turnDir := flags.String("turn-dir", "", "turn directory the result artifacts must stay inside")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission-turn adjudicate", *root, *mission) {
		return 2
	}
	if *statePath == "" || *turnPath == "" || *resultPath == "" || *turnDir == "" {
		fmt.Fprintln(os.Stderr, "mission-turn adjudicate: --state, --turn, --result, and --turn-dir are required")
		return 2
	}
	verdict, err := adjudicateTurn(*root, *mission, *statePath, *turnPath, *resultPath, *turnDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(verdict)
	return 0
}

func adjudicateTurn(root, mission, statePath, turnPath, resultPath, turnDir string) (*missionrunner.Verdict, error) {
	state, err := runnerDoc(statePath, "mission state")
	if err != nil {
		return nil, err
	}
	turnDoc, err := runnerDoc(turnPath, "turn record")
	if err != nil {
		return nil, err
	}
	turn, err := missionrunner.TurnFromDoc(turnDoc)
	if err != nil {
		return nil, err
	}
	result, err := runnerDoc(resultPath, "host result")
	if err != nil {
		return nil, err
	}
	validation, err := missionrunner.ValidateReturn(turn, result, turnDir, returnCompletenessChecker(root))
	if err != nil {
		return nil, err
	}
	verdict, err := missionrunner.Adjudicate(root, mission, turn, state, validation.Returned, runnerNowISO())
	if err != nil {
		return nil, err
	}
	verdict.RawPath = validation.RawPath
	verdict.ReturnPath = validation.ReturnPath
	return verdict, nil
}

// returnCompletenessChecker runs the shipped role checker on a return file,
// wrapping a refusal in the runner's own words.
func returnCompletenessChecker(root string) func(returnPath string) error {
	return func(returnPath string) error {
		command := exec.Command(
			filepath.Join(root, "scripts", "assert-return-complete.sh"),
			"--role", "orchestrator", "--file", returnPath,
		)
		command.Dir = root
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		if err := command.Run(); err != nil {
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = strings.TrimSpace(stdout.String())
			}
			return fmt.Errorf("orchestrator return is invalid: %s", detail)
		}
		return nil
	}
}

// runMissionTurnConclude proposes the state after a turn whose return was
// accepted: the adjudicated streams, the turn-log entry, the advanced cycle
// count, the refreshed waiting list and fences, and the continue/park/
// complete decision. The runner applies the proposal through the state's
// compare-and-write.
func runMissionTurnConclude(args []string) int {
	flags := flag.NewFlagSet("mission-turn conclude", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	turnPath := flags.String("turn", "", "turn record path")
	verdictPath := flags.String("verdict", "", "adjudication verdict path")
	returnPath := flags.String("return", "", "orchestrator return path")
	resultPath := flags.String("result", "", "host result envelope path")
	measurementPath := flags.String("measurement", "", "measurement file: {\"measurement\":..., \"gatePassed\":...}")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission-turn conclude", *root, *mission) {
		return 2
	}
	if *statePath == "" || *turnPath == "" || *verdictPath == "" || *returnPath == "" || *resultPath == "" || *measurementPath == "" {
		fmt.Fprintln(os.Stderr, "mission-turn conclude: --state, --turn, --verdict, --return, --result, and --measurement are required")
		return 2
	}
	inputs := map[string]map[string]any{}
	var err error
	for label, path := range map[string]string{
		"mission state":        *statePath,
		"turn record":          *turnPath,
		"adjudication verdict": *verdictPath,
		"orchestrator return":  *returnPath,
		"host result":          *resultPath,
		"measurement":          *measurementPath,
	} {
		if inputs[label], err = runnerDoc(path, label); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	turn, err := missionrunner.TurnFromDoc(inputs["turn record"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	state := inputs["mission state"]
	verdict := inputs["adjudication verdict"]
	streams, ok := verdict["streams"].(map[string]any)
	if !ok {
		fmt.Fprintln(os.Stderr, "adjudication verdict has no stream map")
		return 1
	}
	// The proposal builds on the adjudicated streams, not the streams as the
	// last write left them: the verdict is what this turn already decided.
	state["streams"] = streams
	gatePassed, _ := inputs["measurement"]["gatePassed"].(bool)
	proposed, err := missionrunner.ConcludeTurn(*root, *mission, state, turn, missionrunner.TurnConclusion{
		SessionID:      inputs["host result"]["sessionId"],
		Measurement:    inputs["measurement"]["measurement"],
		GatePassed:     gatePassed,
		Accepted:       verdict["accepted"],
		Rejected:       verdict["rejected"],
		Certified:      inputs["orchestrator return"]["certified"],
		FactsForLedger: inputs["orchestrator return"]["factsForLedger"],
		Gaps:           inputs["orchestrator return"]["gaps"],
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(proposed)
	return 0
}

// runMissionTurnRecordFailure proposes the state after a turn that produced
// no usable return: the failure lands in the turn log, the cycle is spent,
// and a second consecutive failure parks the mission for a human.
func runMissionTurnRecordFailure(args []string) int {
	flags := flag.NewFlagSet("mission-turn record-failure", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	turnPath := flags.String("turn", "", "turn record path")
	detail := flags.String("detail", "", "human-readable failure detail")
	outcome := flags.String("outcome", "", "turn outcome (failed or unresumable)")
	failures := flags.Int("consecutive-failures", 0, "consecutive failed turns including this one")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission-turn record-failure", *root, *mission) {
		return 2
	}
	// An empty --detail is tolerated: the failure is still worth recording
	// even when the failing step produced no message.
	if *statePath == "" || *turnPath == "" || *outcome == "" {
		fmt.Fprintln(os.Stderr, "mission-turn record-failure: --state, --turn, and --outcome are required")
		return 2
	}
	state, err := runnerDoc(*statePath, "mission state")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	turnDoc, err := runnerDoc(*turnPath, "turn record")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	turn, err := missionrunner.TurnFromDoc(turnDoc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proposed, err := missionrunner.RecordFailureProposal(*root, *mission, state, turn, *detail, *outcome, *failures)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(proposed)
	return 0
}

// runMissionTurnPark proposes parking the mission for a reason, including the
// ask a host-failure or stop-loss park must leave open so a human can answer
// it. Prints {"state":..., "asks":[...]}.
func runMissionTurnPark(args []string) int {
	flags := flag.NewFlagSet("mission-turn park", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	reason := flags.String("reason", "", "park reason")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission-turn park", *root, *mission) {
		return 2
	}
	if *statePath == "" || *reason == "" {
		fmt.Fprintln(os.Stderr, "mission-turn park: --state and --reason are required")
		return 2
	}
	state, err := runnerDoc(*statePath, "mission state")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	outcome, err := missionrunner.ParkProposal(*root, *mission, state, *reason, runnerNowISO())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(outcome)
	return 0
}

// runMissionJobsDrain prints the mission's not-yet-terminal jobs as
// {"activeJobs":[...]}. The runner reaps each and calls again until the list
// drains empty, keeping process reaping with the dispatch tooling.
func runMissionJobsDrain(args []string) int {
	return runMissionJobsList(args, "mission-jobs drain", "activeJobs", missionrunner.ActiveJobs)
}

// runMissionJobsCloseChains prints the root jobs of delegation chains that
// are fully terminal and not yet closed, as {"chains":[...]}. The runner
// reaps and closes each root through the dispatch tooling.
func runMissionJobsCloseChains(args []string) int {
	return runMissionJobsList(args, "mission-jobs close-chains", "chains", missionrunner.CloseableChains)
}

func runMissionJobsList(args []string, name, field string, list func(root, mission string) []string) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope(name, *root, *mission) {
		return 2
	}
	printJSON(map[string]any{field: list(*root, *mission)})
	return 0
}
