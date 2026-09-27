package main

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

// stubBatchOwnerCalls replaces the landing path's in-process owner calls for
// one test with run, which sees each call as the argv its former child
// carried, so a test can keep asserting on the words of the call.
func stubBatchOwnerCalls(t *testing.T, run func(invocation ownerInvocation, argv ...string) error) {
	t.Helper()
	original := batchOwnerCalls
	t.Cleanup(func() { batchOwnerCalls = original })
	batchOwnerCalls = batchOwnerCallSet{
		handover: func(invocation ownerInvocation, request goalHandoverRequest) error {
			argv := []string{"goal", "handover", "--root", request.Root, "--id", request.GoalID, "--lineage", invocation.lineage,
				"--target-machine", request.TargetMachine, "--target-lineage", request.TargetLineage,
				"--target-claim-epoch", strconv.FormatInt(request.TargetEpoch, 10), "--batch", request.Batch}
			if request.TargetRoot != "" {
				argv = append(argv, "--target-root", request.TargetRoot)
			}
			return run(invocation, argv...)
		},
		editNext: func(invocation ownerInvocation, root, goalID, next string) error {
			return run(invocation, "internal", "goal", "edit", "--root", root, "--id", goalID, "--next", next, "--lineage", invocation.lineage)
		},
		release: func(invocation ownerInvocation, root, goalID string) error {
			return run(invocation, "internal", "goal", "release", "--root", root, "--id", goalID, "--lineage", invocation.lineage)
		},
		held: func(root, base, commit, remote, ref string) error {
			return run(ownerInvocation{}, "landing", "held", "--root", root, "--base", base, "--commit", commit, "--remote", remote, "--ref", ref)
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
		brain: func(choice string, caller processIdentity, stdout, stderr io.Writer, root, by string) int {
			record(words("brain", choice, "--root", root, "--by", by))
			return real.brain(choice, caller, stdout, stderr, root, by)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			record(words("goal", "fetch", "--root", root))
			return real.goalFetch(stdout, stderr, root)
		},
		goalRepair: func(caller processIdentity, stdout, stderr io.Writer, root, by string) int {
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
	}
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
		brain: func(choice string, _ processIdentity, stdout, stderr io.Writer, root, by string) int {
			return run(root, stdout, stderr, "brain", choice, "--root", root, "--by", by)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			return run(root, stdout, stderr, "goal", "fetch", "--root", root)
		},
		goalRepair: func(_ processIdentity, stdout, stderr io.Writer, root, by string) int {
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
	}
}
