package main

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

// stubBatchOwnerCalls replaces the landing path's in-process owner calls for
// one test with run, which sees each call as the argv its former child
// carried, so a test can keep asserting on the words of the call.
func stubBatchOwnerCalls(t *testing.T, run func(invocation ownercall.Invocation, argv ...string) error) {
	t.Helper()
	original := batchowner.BatchOwnerCalls
	t.Cleanup(func() { batchowner.BatchOwnerCalls = original })
	batchowner.BatchOwnerCalls = batchowner.BatchOwnerCallSet{
		Handover: func(invocation ownercall.Invocation, request ownercall.HandoverRequest) error {
			argv := []string{"goal", "handover", "--root", request.Root, "--id", request.GoalID, "--lineage", invocation.Lineage,
				"--target-machine", request.TargetMachine, "--target-lineage", request.TargetLineage,
				"--target-claim-epoch", strconv.FormatInt(request.TargetEpoch, 10), "--batch", request.Batch}
			if request.TargetRoot != "" {
				argv = append(argv, "--target-root", request.TargetRoot)
			}
			return run(invocation, argv...)
		},
		EditNext: func(invocation ownercall.Invocation, root, goalID, next string) error {
			return run(invocation, "internal", "goal", "edit", "--root", root, "--id", goalID, "--next", next, "--lineage", invocation.Lineage)
		},
		Release: func(invocation ownercall.Invocation, root, goalID string) error {
			return run(invocation, "internal", "goal", "release", "--root", root, "--id", goalID, "--lineage", invocation.Lineage)
		},
		Held: func(root, base, commit, remote, ref string) error {
			return run(ownercall.Invocation{}, "landing", "held", "--root", root, "--base", base, "--commit", commit, "--remote", remote, "--ref", ref)
		},
	}
}

// scriptDelegator adapts a fake delegate executable to the in-process
// delegate boundary: it runs the script with the argv and environment the
// former `internal delegate` child received, so a fake written for the child
// observes the same words and variables.
func scriptDelegator(script string) delegateCaller {
	return func(request delegateRequest, stdout, stderr io.Writer) int {
		command := exec.Command(script, append([]string{"internal", "delegate"}, request.args...)...)
		command.Env = append(append(os.Environ(), request.environment...), "METASYSTEM_DELEGATE_ROOT="+request.rootOverride)
		command.Dir, command.Stdin, command.Stdout, command.Stderr = request.dir, request.stdin, stdout, stderr
		return commandExitCode(command.Run())
	}
}

// recordingOwnerCalls are the production in-process owner calls, each first
// recorded as the argv its former child carried (prefix, then the owner's
// words), so tests that name the route keep naming it.
func recordingOwnerCalls(prefix []string, record func([]string)) *intentOwnerCalls {
	real := defaultIntentOwnerCalls()
	words := func(rest ...string) []string { return append(append([]string(nil), prefix...), rest...) }
	return &intentOwnerCalls{
		brain: func(choice string, caller ownercall.Process, stdout, stderr io.Writer, root, by string) int {
			record(words("brain", choice, "--root", root, "--by", by))
			return real.brain(choice, caller, stdout, stderr, root, by)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			record(words("goal", "fetch", "--root", root))
			return real.goalFetch(stdout, stderr, root)
		},
		goalRepair: func(caller ownercall.Process, stdout, stderr io.Writer, root, by string) int {
			record(words("goal", "repair", "--accept-remote", "--by", by, "--root", root))
			return real.goalRepair(caller, stdout, stderr, root, by)
		},
		configKeys: func(stdout io.Writer, conf, matching string) int {
			argv := words("config", "keys", "--conf", conf)
			if matching != "" {
				argv = append(argv, "--matching", matching)
			}
			record(argv)
			return real.configKeys(stdout, conf, matching)
		},
		configValidate: func(stdout, stderr io.Writer, conf, repo string) int {
			record(words("config", "validate", "--conf", conf, "--repo", repo))
			return real.configValidate(stdout, stderr, conf, repo)
		},
		delegate: func(request delegateRequest, stdout, stderr io.Writer) int {
			record(words(append([]string{"delegate"}, request.args...)...))
			return real.delegate(request, stdout, stderr)
		},
		goalReconcile: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			record(words(append([]string{"goal", "reconcile"}, args...)...))
			return real.goalReconcile(dependencies, stdout, stderr, dir, args)
		},
		goalMigrate: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			record(words(append([]string{"goal", "migrate"}, args...)...))
			return real.goalMigrate(dependencies, stdout, stderr, dir, args)
		},
		goalCarry: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			record(words(append([]string{"goal", "carry"}, args...)...))
			return real.goalCarry(dependencies, stdout, stderr, dir, args)
		},
		landingTestReceipt: func(caller ownercall.Process, stdout, stderr io.Writer, dir string, args []string) int {
			record(words(append([]string{"landing", "test-receipt"}, args...)...))
			return real.landingTestReceipt(caller, stdout, stderr, dir, args)
		},
		channelWait: func(caller ownercall.Process, lineage string, stdout, stderr io.Writer, args []string) int {
			record(words(append([]string{"channel", "wait"}, args...)...))
			return real.channelWait(caller, lineage, stdout, stderr, args)
		},
		missionStatus: func(stdout, stderr io.Writer, root, mission string) int {
			record(words("mission", "status", "--root", root, "--mission", mission))
			return real.missionStatus(stdout, stderr, root, mission)
		},
		missionLaunch: func(caller ownercall.Process, stdout, stderr io.Writer, root, mission, mode string, wait bool) int {
			record(words("mission", mode, "--root", root, "--mission", mission))
			return real.missionLaunch(caller, stdout, stderr, root, mission, mode, wait)
		},
		missionResolveTaint: func(caller ownercall.Process, stdout, stderr io.Writer, request missionResolveRequest) int {
			record(words(request.words()...))
			return real.missionResolveTaint(caller, stdout, stderr, request)
		},
	}
}

// processBackedDelivery gives a delivery stand-in whose fake engine answers
// by argv the owner calls too: each in-process owner call reaches the fake as
// the argv its former child carried.
func processBackedDelivery(delivery *intentDeliveryOwners) *intentDeliveryOwners {
	delivery.calls = processBackedOwnerCalls(delivery.executable, delivery.process)
	return delivery
}

// processBackedOwnerCalls route each owner call to a test's fake owner
// process as the argv its former child carried, for beds whose fakes answer
// by argv.
func processBackedOwnerCalls(executable func() (string, error), process func(intentProcess) intentProcessResult) *intentOwnerCalls {
	run := func(dir string, stdout, stderr io.Writer, words ...string) int {
		binary, err := executable()
		if err != nil {
			io.WriteString(stderr, err.Error())
			return 1
		}
		ran := process(intentProcess{argv: append([]string{binary, "internal"}, words...), dir: dir})
		stdout.Write(ran.stdout)
		stderr.Write(ran.stderr)
		if ran.err != nil && ran.code == 0 {
			return 1
		}
		return ran.code
	}
	return &intentOwnerCalls{
		brain: func(choice string, _ ownercall.Process, stdout, stderr io.Writer, root, by string) int {
			return run(root, stdout, stderr, "brain", choice, "--root", root, "--by", by)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			return run(root, stdout, stderr, "goal", "fetch", "--root", root)
		},
		goalRepair: func(_ ownercall.Process, stdout, stderr io.Writer, root, by string) int {
			return run(root, stdout, stderr, "goal", "repair", "--accept-remote", "--by", by, "--root", root)
		},
		configKeys: func(stdout io.Writer, conf, matching string) int {
			words := []string{"config", "keys", "--conf", conf}
			if matching != "" {
				words = append(words, "--matching", matching)
			}
			return run("", stdout, io.Discard, words...)
		},
		configValidate: func(stdout, stderr io.Writer, conf, repo string) int {
			return run(repo, stdout, stderr, "config", "validate", "--conf", conf, "--repo", repo)
		},
		delegate: func(request delegateRequest, stdout, stderr io.Writer) int {
			return run(request.dir, stdout, stderr, append([]string{"delegate"}, request.args...)...)
		},
		goalReconcile: func(_ syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			return run(dir, stdout, stderr, append([]string{"goal", "reconcile"}, args...)...)
		},
		goalMigrate: func(_ syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			return run(dir, stdout, stderr, append([]string{"goal", "migrate"}, args...)...)
		},
		goalCarry: func(_ syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int {
			return run(dir, stdout, stderr, append([]string{"goal", "carry"}, args...)...)
		},
		landingTestReceipt: func(_ ownercall.Process, stdout, stderr io.Writer, dir string, args []string) int {
			return receiptProcess(executable, process, stdout, stderr, dir, args)
		},
		channelWait: func(_ ownercall.Process, _ string, stdout, stderr io.Writer, args []string) int {
			return run(flagValue(args, "--root"), stdout, stderr, append([]string{"channel", "wait"}, args...)...)
		},
		missionStatus: func(stdout, stderr io.Writer, root, mission string) int {
			return run(root, stdout, stderr, "mission", "status", "--root", root, "--mission", mission)
		},
		missionLaunch: func(_ ownercall.Process, stdout, stderr io.Writer, root, mission, mode string, _ bool) int {
			return run(root, stdout, stderr, "mission", mode, "--root", root, "--mission", mission)
		},
		missionResolveTaint: func(_ ownercall.Process, stdout, stderr io.Writer, request missionResolveRequest) int {
			return run(request.root, stdout, stderr, request.words()...)
		},
	}
}

// receiptProcess reaches a test's fake engine with the argv the former
// landing test-receipt child carried (the engine's top-level landing form).
func receiptProcess(executable func() (string, error), process func(intentProcess) intentProcessResult, stdout, stderr io.Writer, dir string, args []string) int {
	binary, err := executable()
	if err != nil {
		io.WriteString(stderr, err.Error())
		return 1
	}
	ran := process(intentProcess{argv: append([]string{binary, "landing", "test-receipt"}, args...), dir: dir})
	stdout.Write(ran.stdout)
	stderr.Write(ran.stderr)
	if ran.err != nil && ran.code == 0 {
		return 1
	}
	return ran.code
}

// processBackedReceipt keeps a delivery's production owner calls and routes
// only the landing proof to its fake engine, as the argv the former child
// carried.
func processBackedReceipt(delivery *intentDeliveryOwners) *intentDeliveryOwners {
	if delivery.calls == nil {
		delivery.calls = defaultIntentOwnerCalls()
	}
	delivery.calls.landingTestReceipt = func(_ ownercall.Process, stdout, stderr io.Writer, dir string, args []string) int {
		return receiptProcess(delivery.executable, delivery.process, stdout, stderr, dir, args)
	}
	return delivery
}
