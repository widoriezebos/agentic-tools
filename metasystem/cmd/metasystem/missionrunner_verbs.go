package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The mission-turn and mission-jobs families are the mission runner's
// decision surface (internal/missionrunner). Each verb reads the runner's
// records, judges, and prints a JSON proposal; the runner applies it — writes
// the asks, advances the hash-chained state, and drives the dispatch tooling
// — so every artifact keeps its single writer. The mission-runner family at
// the bottom of this file IS that runner: the long-lived process that drives
// a mission's cycles end to end.

// run-loop is the detached child the public mission start and resume launch
// (missionrunner.launch.go); a person answers a mission's ask with question
// answer M/Q.

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

func missionRunnerCommandEngineWith(root, mission string, repositoryTop func(string) (string, error)) (*missionrunner.Engine, error) {
	// The engine's launches are gated by the installation's stop fence. A
	// checkout whose installation cannot be found is refused rather than
	// read at root, where no fence is kept and an absent one reads as open.
	_, installation, err := processInstallationWith(root, "", repositoryTop)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", noEngineRefusal, err)
	}
	return missionRunnerCommandEngine(root, installation, mission)
}

// missionRunnerCommandEngine is the mission engine whose contract is under
// the state root root and whose run state, configuration and delegates are
// the installation's.
func missionRunnerCommandEngine(root string, installation stateroot.Installation, mission string) (*missionrunner.Engine, error) {
	commandClock, _, err := goalCommandClock(installation.Path())
	if err != nil {
		return nil, err
	}
	engine := missionrunner.NewEngineAt(root, installation.Path(), mission)
	engine.Now = commandClock
	engine.Delegate = delegateInProcess(installation.Path())
	return engine, nil
}

// runMissionRunnerRunLoop is the detached child that start/resume spawn; it
// is internal and deliberately prints no usage.
func runMissionRunnerRunLoop(args []string, stdout, stderr io.Writer) int {
	var root, installationPath, mission, mode, tag, signal, generationText string
	ignoreTerm := false
	ok := parseRunnerArgs(args, map[string]*string{
		"--root": &root, "--installation": &installationPath, "--mission": &mission, "--mode": &mode,
		"--instance-tag": &tag, "--start-signal": &signal,
		"--fence-generation": &generationText,
	}, map[string]*bool{"--ignore-term": &ignoreTerm})
	if !ok {
		for _, arg := range args {
			switch arg {
			case "--root", "--installation", "--mission", "--mode", "--instance-tag", "--start-signal", "--fence-generation", "--ignore-term":
			default:
				if strings.HasPrefix(arg, "--") {
					return refuseUnknownOption(stdout, stderr, "mission run-loop", arg, "options: --root --installation --mission --mode --instance-tag --start-signal --fence-generation --ignore-term")
				}
			}
		}
	}
	generation, generationErr := strconv.ParseInt(generationText, 10, 64)
	// The launcher names the installation that holds the mission's run
	// state, so the loop writes where the launcher and a stop look.
	installation, installationErr := stateroot.ParseInstallation(installationPath)
	if !ok || root == "" || installationErr != nil || tag == "" || signal == "" ||
		generationErr != nil || generation < 0 || !missionIDRe.MatchString(mission) ||
		(mode != "start" && mode != "resume") {
		return 2
	}
	engine, err := missionRunnerCommandEngine(root, installation, mission)
	if err != nil {
		fmt.Fprintln(stderr, "mission run-loop:", err)
		return 1
	}
	return engine.RunLoopAtGeneration(mode, tag, signal, generation, ignoreTerm)
}
