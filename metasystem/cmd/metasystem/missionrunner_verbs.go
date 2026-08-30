package main

import (
	"flag"
	"fmt"
	missionpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"strconv"
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
	flags := flag.NewFlagSet("mission turn-adjudicate", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	turnPath := flags.String("turn", "", "turn record path")
	resultPath := flags.String("result", "", "host result envelope path")
	turnDir := flags.String("turn-dir", "", "turn directory the result artifacts must stay inside")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission turn-adjudicate", *root, *mission) {
		return 2
	}
	if *statePath == "" || *turnPath == "" || *resultPath == "" || *turnDir == "" {
		fmt.Fprintln(os.Stderr, "mission turn-adjudicate: --state, --turn, --result, and --turn-dir are required")
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
	flags := flag.NewFlagSet("mission turn-conclude", flag.ContinueOnError)
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
	if !missionRunnerScope("mission turn-conclude", *root, *mission) {
		return 2
	}
	if *statePath == "" || *turnPath == "" || *verdictPath == "" || *returnPath == "" || *resultPath == "" || *measurementPath == "" {
		fmt.Fprintln(os.Stderr, "mission turn-conclude: --state, --turn, --verdict, --return, --result, and --measurement are required")
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
	flags := flag.NewFlagSet("mission turn-record-failure", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	turnPath := flags.String("turn", "", "turn record path")
	detail := flags.String("detail", "", "human-readable failure detail")
	outcome := flags.String("outcome", "", "turn outcome (failed or unresumable)")
	failures := flags.Int("consecutive-failures", 0, "consecutive failed turns including this one")
	feedsBreaker := flags.Bool("feeds-breaker", true, "whether this failure counts toward the host-failure breaker (false for provider overload)")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission turn-record-failure", *root, *mission) {
		return 2
	}
	// An empty --detail is tolerated: the failure is still worth recording
	// even when the failing step produced no message.
	if *statePath == "" || *turnPath == "" || *outcome == "" {
		fmt.Fprintln(os.Stderr, "mission turn-record-failure: --state, --turn, and --outcome are required")
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
	proposed, err := missionrunner.RecordFailureProposal(*root, *mission, state, turn, *detail, *outcome, *failures, *feedsBreaker)
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
	flags := flag.NewFlagSet("mission turn-park", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mission := flags.String("mission", "", "mission id")
	statePath := flags.String("state", "", "mission state path")
	reason := flags.String("reason", "", "park reason")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionRunnerScope("mission turn-park", *root, *mission) {
		return 2
	}
	if *statePath == "" || *reason == "" {
		fmt.Fprintln(os.Stderr, "mission turn-park: --state and --reason are required")
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
			"  metasystem mission start --mission <id> [--foreground]\n"+
			"  metasystem mission resume --mission <id> [--foreground]\n"+
			"  metasystem mission status --mission <id>\n"+
			"  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>\n"+
			"  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>\n"+
			"      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])\n")
}

// parseRunnerArgs reads --key value pairs and bare switches with the
// runner's strict grammar: only the given keys, every valued key valued, and
// nothing else. It reports whether the arguments parsed cleanly.
//
// Kept hand-rolled deliberately: the five runner verbs share ONE
// grammar whose flag sets are data (the valued/switches maps), and the
// grammar refuses a stray positional anywhere in the argument list —
// flag.FlagSet stops parsing at the first positional instead of refusing
// it, and cannot be table-driven this tersely. Nothing here re-implements
// flag semantics loosely: unknown keys and unvalued keys refuse.
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

func runMissionRunnerResolveTaint(args []string) int {
	// ONE strict left-to-right scan over the RAW tokens: every token
	// in flag position must be a known
	// flag; --adopt is a bare switch; every valued flag consumes exactly
	// the next token, which must not itself look like a flag; duplicates
	// refuse (only --waives repeats). No token is lifted, filtered, or
	// repaired before this scan, so no malformed spelling can collapse
	// into a lawful one.
	values := map[string]string{}
	var waived []string
	adopt := false
	for index := 0; index < len(args); {
		flag := args[index]
		switch flag {
		case "--adopt":
			if adopt {
				missionRunnerUsage()
				return 2
			}
			adopt = true
			index++
		case "--root", "--mission", "--taint", "--restore", "--by", "--reason", "--waives":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				missionRunnerUsage()
				return 2
			}
			value := args[index+1]
			if flag == "--waives" {
				waived = append(waived, value)
			} else {
				if _, duplicate := values[flag]; duplicate {
					missionRunnerUsage()
					return 2
				}
				values[flag] = value
			}
			index += 2
		default:
			missionRunnerUsage()
			return 2
		}
	}
	root, mission := values["--root"], values["--mission"]
	restoreTree, by, reason := values["--restore"], values["--by"], values["--reason"]
	taintID, err := strconv.ParseInt(values["--taint"], 10, 64)
	blankWaiver := false
	for _, claim := range waived {
		if missionpkg.BlankString(claim) {
			blankWaiver = true
		}
	}
	if root == "" || !missionIDRe.MatchString(mission) || err != nil || taintID < 1 ||
		adopt == (restoreTree != "") ||
		(restoreTree != "" && !treeIDRe.MatchString(restoreTree)) ||
		missionpkg.BlankString(by) || missionpkg.BlankString(reason) ||
		(adopt && (len(waived) == 0 || blankWaiver)) ||
		(!adopt && len(waived) > 0) {
		missionRunnerUsage()
		return 2
	}
	variant, tree := "restore", restoreTree
	if adopt {
		variant, tree = "adopt-disputed-tree", ""
	}
	return missionrunner.NewEngine(root, mission).ResolveTaint(taintID, variant, tree, by, reason, waived)
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
