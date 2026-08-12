package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
)

// The mission-turn and mission-jobs families are the mission runner's
// decision surface (internal/missionrunner). Each verb reads the runner's
// records, judges, and prints a JSON proposal; the runner applies it — writes
// the asks, advances the hash-chained state, and drives the dispatch tooling
// — so every artifact keeps its single writer. The mission-runner family at
// the bottom of this file IS that runner: the long-lived process that drives
// a mission's cycles end to end.

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
	verdict, err := missionrunner.AdjudicateFiles(*root, *mission, *statePath, *turnPath, *resultPath, *turnDir, runnerNowISO())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(verdict)
	return 0
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
	proposed, err := missionrunner.ConcludeFiles(*root, *mission, *statePath, *turnPath, *verdictPath, *returnPath, *resultPath, *measurementPath)
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

// The mission-runner family: the runner process itself. start and resume
// launch the detached run loop and hold the caller until the first host turn
// verifiably starts; run-loop is that detached child; status and answer are
// the human/driver surface over a mission's artifacts.

// missionRunnerUsage prints the runner's public usage, which names the shell
// entry point callers actually invoke.
func missionRunnerUsage() {
	fmt.Fprint(os.Stderr,
		"Usage:\n"+
			"  scripts/agents/mission-runner.sh start --mission <id> [--foreground]\n"+
			"  scripts/agents/mission-runner.sh resume --mission <id> [--foreground]\n"+
			"  scripts/agents/mission-runner.sh status --mission <id>\n"+
			"  scripts/agents/mission-runner.sh answer --mission <id> --ask <ask-id> --answer <text>\n")
}

// parseRunnerArgs reads --key value pairs and bare switches with the
// runner's strict grammar: only the given keys, every valued key valued, and
// nothing else. It reports whether the arguments parsed cleanly.
func parseRunnerArgs(args []string, valued map[string]*string, switches map[string]*bool) bool {
	for index := 0; index < len(args); {
		if target, known := valued[args[index]]; known && index+1 < len(args) {
			*target = args[index+1]
			index += 2
			continue
		}
		if target, known := switches[args[index]]; known {
			*target = true
			index++
			continue
		}
		return false
	}
	return true
}

func runMissionRunnerStart(args []string) int {
	return runMissionRunnerLaunch("start", args)
}

func runMissionRunnerResume(args []string) int {
	return runMissionRunnerLaunch("resume", args)
}

func runMissionRunnerLaunch(mode string, args []string) int {
	var root, mission string
	foreground := false
	ok := parseRunnerArgs(args,
		map[string]*string{"--root": &root, "--mission": &mission},
		map[string]*bool{"--foreground": &foreground})
	if !ok || root == "" || !missionIDRe.MatchString(mission) {
		missionRunnerUsage()
		return 2
	}
	return missionrunner.NewEngine(root, mission).Launch(mode, foreground)
}

func runMissionRunnerStatus(args []string) int {
	var root, mission string
	ok := parseRunnerArgs(args, map[string]*string{"--root": &root, "--mission": &mission}, nil)
	if !ok || root == "" || !missionIDRe.MatchString(mission) {
		missionRunnerUsage()
		return 2
	}
	return missionrunner.NewEngine(root, mission).Status()
}

func runMissionRunnerAnswer(args []string) int {
	var root, mission, askID, answer string
	ok := parseRunnerArgs(args, map[string]*string{
		"--root": &root, "--mission": &mission, "--ask": &askID, "--answer": &answer,
	}, nil)
	if !ok || root == "" || !missionIDRe.MatchString(mission) || !missionIDRe.MatchString(askID) ||
		answer == "" || strings.ContainsRune(answer, 0) {
		missionRunnerUsage()
		return 2
	}
	return missionrunner.NewEngine(root, mission).Answer(askID, answer)
}

// runMissionRunnerRunLoop is the detached child that start/resume spawn; it
// is internal and deliberately prints no usage.
func runMissionRunnerRunLoop(args []string) int {
	var root, mission, mode, tag, signal string
	ok := parseRunnerArgs(args, map[string]*string{
		"--root": &root, "--mission": &mission, "--mode": &mode,
		"--instance-tag": &tag, "--start-signal": &signal,
	}, nil)
	if !ok || root == "" || tag == "" || signal == "" ||
		!missionIDRe.MatchString(mission) || (mode != "start" && mode != "resume") {
		return 2
	}
	return missionrunner.NewEngine(root, mission).RunLoop(mode, tag, signal)
}
