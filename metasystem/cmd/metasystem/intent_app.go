package main

import (
	"bytes"
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// One word for the application this project is building: start it, stop it,
// restart it, read where it is, read its log, reset its data and check it.
// The project's launch contract names the commands; these verbs own the
// process, the record and the proof of stopping.

var (
	intentAppAtFlag   = intentFlag{name: "at", value: "REF", usage: "run the application at this commit, beside the standing run"}
	intentAppGoalFlag = intentFlag{name: "goal", value: "G", usage: "run the application at this goal's branch tip (sugar for --at goal/G)"}
	intentAppWaitFlag = intentFlag{name: "wait-seconds", value: "N", usage: "seconds to wait for the application to stop (default: the contract's stopMs)"}
)

func appIntentCommands() []intentCommand {
	run := func(verb string) func(*intentInvocation) int {
		return func(inv *intentInvocation) int { return inv.appVerb(verb) }
	}
	return []intentCommand{
		{
			object: "app", action: "start", primary: true, audience: "both", summary: "start this project's application",
			usage: []string{"metasystem app start [--at REF | --goal G]"},
			details: []string{
				"Starts the application the project's launch contract names, waits until it is ready, and records where it runs.",
				"--at REF and --goal G run a candidate from its own worktree, on its own port, beside the standing run.",
				"A start for a run that is already live rejoins it and still waits for readiness.",
			},
			flags:    []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem app start", "metasystem app start --goal browser-interface"},
			run:      run("start"),
		},
		{
			object: "app", action: "stop", audience: "both", summary: "stop this project's application and prove it stopped",
			usage:   []string{"metasystem app stop [--at REF | --goal G] [--wait-seconds N] [--clean]"},
			details: []string{"Nothing is reported as stopped that is not: every recorded process must be dead, the run's process group empty, and a probed readiness dark.", "--clean also reclaims a candidate run's worktree."},
			flags: []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentAppWaitFlag, intentInstallationFlag,
				{name: "clean", usage: "also reclaim the run's worktree"}},
			maxArgs:  0,
			examples: []string{"metasystem app stop"},
			run:      run("stop"),
		},
		{
			object: "app", action: "restart", audience: "both", summary: "stop and start this project's application",
			usage:    []string{"metasystem app restart [--at REF | --goal G] [--wait-seconds N]"},
			details:  []string{"Starts again only after the stop was proven; if either half fails, it says where it got to."},
			flags:    []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentAppWaitFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem app restart"},
			run:      run("restart"),
		},
		{
			object: "app", action: "status", audience: "both", summary: "whether this project's application runs, where, and since when",
			usage:    []string{"metasystem app status [--at REF | --goal G]"},
			details:  []string{"Liveness and readiness are said separately: an application can be alive and not answering, and neither is reported as the other."},
			flags:    []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem app status", "metasystem app status --goal browser-interface"},
			run:      run("status"),
		},
		{
			object: "app", action: "log", audience: "both", summary: "print this project's application log",
			usage:   []string{"metasystem app log [--at REF | --goal G] [--lines N] [--follow]"},
			details: []string{"Reads the file the engine captured, or the file the contract named. An ended run's log is still there to read."},
			flags: []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentInstallationFlag,
				{name: "lines", value: "N", usage: "how many lines of the tail to print (default: 40)"},
				{name: "follow", usage: "keep printing the log as it grows, until interrupted"}},
			maxArgs:  0,
			examples: []string{"metasystem app log", "metasystem app log --follow"},
			run:      run("log"),
		},
		{
			object: "app", action: "reset", audience: "both", summary: "stop, remake this run's data, and start again",
			usage:    []string{"metasystem app reset [--at REF | --goal G] [--wait-seconds N]"},
			details:  []string{"Runs the contract's prepare command again, which is what a learning sitting needs to repeat an experiment from a known state.", "A contract with no prepare says so: reset is then a restart."},
			flags:    []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentAppWaitFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem app reset"},
			run:      run("reset"),
		},
		{
			object: "app", action: "check", audience: "both", summary: "run the contract's testing group against the running application",
			usage:    []string{"metasystem app check [--at REF | --goal G]"},
			details:  []string{"Runs the named group through the testing contract's own runner, freshly, with the run's address in its environment, and records the verdict on the run.", "A run that is not answering is refused: a check against nothing proves nothing."},
			flags:    []intentFlag{intentAppAtFlag, intentAppGoalFlag, intentInstallationFlag},
			maxArgs:  0,
			examples: []string{"metasystem app check"},
			run:      run("check"),
		},
	}
}

// appRef reads the run a verb names: the standing run, a commit, or a goal's
// branch tip. --at and --goal are two spellings of one thing, so naming both
// is refused rather than guessed, and --at goal/G names goal G as --goal G
// does.
func (inv *intentInvocation) appRef() (string, string, *intentResult) {
	at, goal := strings.TrimSpace(inv.input.text("at")), strings.TrimSpace(inv.input.text("goal"))
	if inv.input.has("at") && inv.input.has("goal") {
		return "", "", &intentResult{Outcome: intentRefused, code: 2,
			Summary: "--at and --goal name the same thing; use one of them. Nothing was done"}
	}
	if goal != "" {
		return "goal/" + goal, goal, nil
	}
	if named, ok := strings.CutPrefix(at, "goal/"); ok && named != "" {
		return at, named, nil
	}
	return at, "", nil
}

func (inv *intentInvocation) appWait() (time.Duration, *intentResult) {
	if !inv.input.has("wait-seconds") {
		return 0, nil
	}
	seconds, err := strconv.ParseInt(inv.input.text("wait-seconds"), 10, 64)
	if err != nil || seconds < 0 || seconds > int64((1<<63-1)/time.Second) {
		return 0, &intentResult{Outcome: intentRefused, code: 2,
			Summary: "--wait-seconds needs a nonnegative whole number of seconds that fits a duration; nothing was done"}
	}
	return time.Duration(seconds) * time.Second, nil
}

// appVerb resolves the run every app verb works on and then runs the verb.
func (inv *intentInvocation) appVerb(verb string) int {
	layout, installation, _, problem := inv.selectInstallation()
	if problem != nil {
		return inv.render(*problem)
	}
	targets := []intentTarget{{Kind: "app", ID: layout.GitRoot}}
	roots, err := lifecycle.ResolveRootsWith(inv.owners.processes.process.repositoryTop, inv.owners.resolver.RootForInstallation, layout.GitRoot, installation)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: err.Error() + "; nothing was done"})
	}
	_, contract, contractPath, err := loadPhysicalLaunchContract(roots.Installation)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: err.Error() + "; nothing was done"})
	}
	ref, goal, problem := inv.appRef()
	if problem != nil {
		return inv.render(*problem)
	}
	run := resolveAppRun(roots, contract, contractPath, ref, goal)
	targets = []intentTarget{{Kind: "app", ID: run.key}}
	switch verb {
	case "status":
		return inv.render(inv.appStatus(run, targets))
	case "log":
		return inv.render(inv.appLog(run, targets))
	case "check":
		return inv.render(inv.appCheck(run, targets))
	case "stop":
		wait, problem := inv.appWait()
		if problem != nil {
			return inv.render(*problem)
		}
		return inv.render(inv.appStop(run, targets, wait, inv.input.has("clean")))
	case "start":
		return inv.render(inv.appStart(run, targets, false))
	case "restart", "reset":
		wait, problem := inv.appWait()
		if problem != nil {
			return inv.render(*problem)
		}
		stopped := inv.appStop(run, targets, wait, false)
		if stopped.Outcome != intentConfirmed {
			stopped.Summary = "the application was not stopped, so it was not started again: " + stopped.Summary
			stopped.next, stopped.nextReason = inv.publicArgv("app", "status"), "what the application is doing now"
			return inv.render(stopped)
		}
		started := inv.appStart(run, targets, verb == "reset")
		started.text = append(stopped.text, started.text...)
		return inv.render(started)
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "unknown app action " + verb})
}

func appData(run appRun, status applaunch.Status) map[string]any {
	data := map[string]any{"run": run.key, "state": string(status.State), "readiness": string(status.Readiness)}
	if run.ref != "" {
		data["ref"] = run.ref
	}
	if run.goal != "" {
		data["goal"] = run.goal
	}
	if status.Record != nil {
		data["address"] = status.Record.Address
		data["commit"] = status.Record.Commit
		if status.Record.ResolvedFrom != "" {
			data["resolvedFrom"] = status.Record.ResolvedFrom
		}
		data["data"] = status.Record.Data
		data["log"] = status.Record.Log
		if status.Record.Check != nil {
			data["check"] = status.Record.Check
		}
	}
	return data
}

func (inv *intentInvocation) appStatus(run appRun, targets []intentTarget) intentResult {
	status, err := run.status()
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run record could not be read: " + err.Error()}
	}
	summary := "the application is " + string(status.State)
	switch status.State {
	case applaunch.Running:
		summary = "the application is running and " + map[applaunch.Readiness]string{
			applaunch.Answering: "answering", applaunch.NotAnswering: "not answering",
			applaunch.ObservedOnce: "was ready at startup", applaunch.NoProbe: "has no probe"}[status.Readiness]
	case applaunch.Stopped:
		summary = "no application run is recorded for " + run.key
	}
	outcome := intentConfirmed
	if status.State == applaunch.Unreadable {
		outcome = intentFailed
	}
	return intentResult{Outcome: outcome, Targets: targets, Summary: summary, text: status.Lines(), Data: appData(run, status)}
}

func (inv *intentInvocation) appLog(run appRun, targets []intentTarget) intentResult {
	status, _ := run.status()
	path := run.logPath
	if status.Record != nil && status.Record.Log != "" {
		path = status.Record.Log
	}
	lines := 40
	if inv.input.has("lines") {
		parsed, err := strconv.Atoi(inv.input.text("lines"))
		if err != nil || parsed < 1 {
			return intentResult{Outcome: intentRefused, code: 2, Targets: targets, Summary: "--lines needs a positive whole number; nothing was done"}
		}
		lines = parsed
	}
	tail, err := applaunch.Tail(path, lines)
	if err != nil {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "there is no log to read at " + path + ": " + err.Error()}
	}
	if !inv.input.has("follow") {
		return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "the last " + strconv.Itoa(len(tail)) + " line(s) of " + path,
			text: tail, Data: map[string]any{"run": run.key, "log": path, "lines": nonNilLines(tail)}}
	}
	if inv.input.has("json") {
		return intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "--follow prints a stream, which is not one JSON result; use it without --json. Nothing was done"}
	}
	for _, line := range tail {
		fmt.Fprintln(inv.stdout, line)
	}
	ctx, stop := appFollowContext()
	defer stop()
	if err := applaunch.Follow(ctx, path, inv.stdout, 0); err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the log could not be followed: " + err.Error()}
	}
	return intentResult{Outcome: intentConfirmed, Targets: targets, Summary: "stopped following " + path,
		Data: map[string]any{"run": run.key, "log": path}}
}

// appFollowContext is how long `log --follow` follows: until the person
// interrupts it. It is a variable only so that a test can end a follow.
var appFollowContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
}

// appStart starts one run: it rejoins a live one, ends a finished one into
// its evidence, takes the run's tree and data where the ref asks for them,
// and only then launches the supervisor.
func (inv *intentInvocation) appStart(run appRun, targets []intentTarget, reset bool) intentResult {
	status, err := run.status()
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run record could not be read: " + err.Error()}
	}
	var lines []string
	if moved, commit := run.tipMoved(status); moved {
		// One run per ref at a time, and a moved ref makes the next start
		// replace the run: a review is never shown the commit before the
		// one it asked for.
		lines = append(lines, "ref "+run.ref+" moved to "+shortCommit(commit)+"; the run at "+shortCommit(status.Record.Commit)+" is replaced")
		stopped := inv.appStop(run, targets, 0, false)
		lines = append(lines, stopped.text...)
		if stopped.Outcome != intentConfirmed {
			stopped.text = lines
			stopped.Summary = "the run at the old commit was not stopped, so the new one was not started: " + stopped.Summary
			return stopped
		}
		if status, err = run.status(); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines, Summary: "the run record could not be read: " + err.Error()}
		}
	}
	switch status.State {
	case applaunch.Running, applaunch.Starting:
		rejoined, err := applaunch.Rejoin(context.Background(), run.roots.StateRoot, run.key, run.contract, run.readOptions(), 0)
		if err != nil {
			return intentResult{Outcome: intentPartial, code: 1, Targets: targets, text: rejoined.Lines(), Data: appData(run, rejoined),
				Summary: "the application is already running but did not become ready: " + err.Error()}
		}
		return intentResult{Outcome: intentConfirmed, Targets: targets, text: rejoined.Lines(), Data: appData(run, rejoined),
			Summary: "the application is already running at " + rejoined.Record.Address}
	case applaunch.Orphaned, applaunch.Stale, applaunch.Uninspectable, applaunch.ChildEnded:
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: status.Lines(), Data: appData(run, status),
			Summary: "run " + run.key + " is " + string(status.State) + " and was not started again",
			next:    inv.publicArgv("app", "stop"), nextReason: "end the recorded run first; nothing is started over something that may still be alive"}
	case applaunch.Finished:
		if err := run.endRun(status.Record, false, logWriter(&lines)); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines,
				Summary: "the ended run's evidence could not be preserved: " + err.Error()}
		}
	}
	if err := run.allocateAddress(); err != nil {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: lines, Summary: err.Error() + "; nothing was started"}
	}
	if run.ref != "" {
		commit, from, err := run.resolveCommitFor(run.ref)
		if err != nil {
			return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: lines, Summary: err.Error() + "; nothing was started"}
		}
		run.commit, run.resolvedFrom = commit, from
		if err := run.takeWorktree(logWriter(&lines)); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines, Summary: "the run's tree could not be taken: " + err.Error()}
		}
		if err := run.preflightTools(); err != nil {
			return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: lines, Summary: err.Error() + "; nothing was built or started"}
		}
		if err := run.runContractCommand("build", run.contract.Build, 30*time.Minute, logWriter(&lines)); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines, Summary: err.Error()}
		}
	}
	if run.ref == "" {
		if err := run.preflightTools(); err != nil {
			return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: lines, Summary: err.Error() + "; nothing was prepared or started"}
		}
	}
	switch {
	case run.contract.Prepare.Empty() && reset:
		lines = append(lines, "no prepare declared: reset is a restart")
	case run.contract.Prepare.Empty():
		lines = append(lines, "data: shared with the standing run (the contract declares no prepare)")
	default:
		if err := run.prepareData(logWriter(&lines), reset); err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines, Summary: err.Error()}
		}
	}
	address, err := run.launchSupervisor()
	if err != nil {
		after, _ := run.status()
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: append(lines, after.Lines()...), Data: appData(run, after),
			Summary: "the application did not start: " + err.Error(),
			next:    inv.publicArgv("app", "log"), nextReason: "what the application wrote before it stopped"}
	}
	after, _ := run.status()
	where := address
	if where == "" {
		where = "with no address to listen on"
	} else {
		where = "at " + where
	}
	return intentResult{Outcome: intentConfirmed, Targets: targets, text: append(lines, after.Lines()...), Data: appData(run, after),
		Summary: "the application is started " + where}
}

// tipMoved reports a live run at a ref that now names another commit.
func (r appRun) tipMoved(status applaunch.Status) (bool, string) {
	if r.ref == "" || status.Record == nil || (status.State != applaunch.Running && status.State != applaunch.Starting) {
		return false, ""
	}
	commit, _, err := r.resolveCommitFor(r.ref)
	if err != nil || commit == status.Record.Commit {
		return false, ""
	}
	return true, commit
}

// appStop ends one run and proves it, then copies a goal run's evidence and
// only afterwards takes the record away.
func (inv *intentInvocation) appStop(run appRun, targets []intentTarget, wait time.Duration, clean bool) intentResult {
	before, err := run.status()
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run record could not be read: " + err.Error()}
	}
	if before.State == applaunch.Stopped {
		if clean {
			var lines []string
			if err := run.reclaimWorktree(logWriter(&lines)); err != nil {
				return intentResult{Outcome: intentFailed, code: 1, Targets: targets, text: lines, Summary: err.Error()}
			}
			return intentResult{Outcome: intentConfirmed, Targets: targets, text: lines, Data: appData(run, before),
				Summary: "no application run is recorded for " + run.key}
		}
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: appData(run, before),
			Summary: "no application run is recorded for " + run.key}
	}
	result, err := applaunch.Stop(run.roots.StateRoot, run.key, run.contract, applaunch.StopOptions{
		Probe: applaunch.ProbeOnce, Wait: wait, ProjectRoot: run.tree, Environment: run.environment()})
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the application could not be stopped: " + err.Error()}
	}
	lines := append([]string(nil), result.Lines...)
	if !result.Proven {
		after, _ := run.status()
		return intentResult{Outcome: intentPartial, code: 1, Targets: targets, text: append(lines, after.Lines()...), Data: appData(run, after),
			Summary: "the application was asked to stop but is not proven stopped (" + string(result.Outcome) + ")",
			next:    inv.publicArgv("app", "status"), nextReason: "what is still alive"}
	}
	if err := run.endRun(result.Record, clean, logWriter(&lines)); err != nil {
		return intentResult{Outcome: intentPartial, code: 1, Targets: targets, text: lines,
			Summary: "the application stopped, but its run could not be closed: " + err.Error()}
	}
	after, _ := run.status()
	return intentResult{Outcome: intentConfirmed, Targets: targets, text: lines, Data: appData(run, after),
		Summary: "the application is stopped and every recorded process is proven dead"}
}

// appCheck runs the contract's named testing group through the testing
// contract's own runner and nothing else, against a run that is answering.
func (inv *intentInvocation) appCheck(run appRun, targets []intentTarget) intentResult {
	if run.contract.Check == "" {
		return intentResult{Outcome: intentConfirmed, Targets: targets,
			Summary: "no check is declared in the launch contract", Data: map[string]any{"run": run.key, "check": nil}}
	}
	status, err := run.status()
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the run record could not be read: " + err.Error()}
	}
	if status.State != applaunch.Running || (run.contract.Probed() && status.Readiness != applaunch.Answering) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, text: status.Lines(), Data: appData(run, status),
			Summary: "run " + run.key + " is not live and answering, so its check was not run",
			next:    inv.publicArgv("app", "start"), nextReason: "start the application before checking it"}
	}
	address := status.Record.Address
	if address == "" {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: appData(run, status),
			Summary: "this run has no address, so there is nothing for group " + run.contract.Check + " to be run against"}
	}
	// The check runs the testing runner in this process (design 6.2).
	var stderr bytes.Buffer
	output, code, err := inv.work().testRun(run.roots.Installation, append([]string{"internal"}, appCheckArgv(run.roots.Installation, run.contract.Check, address)...), &stderr)
	ran := intentProcessResult{stdout: output, stderr: stderr.Bytes(), code: code, err: err}
	verdict := "pass"
	if ran.code != 0 || ran.err != nil {
		verdict = "fail"
	}
	at := time.Now().UTC().Format(time.RFC3339)
	_ = applaunch.UpdateRecord(run.roots.StateRoot, run.key, func(record *applaunch.Record) {
		record.Check = &applaunch.Check{Group: run.contract.Check, Verdict: verdict, At: at, Address: address}
	})
	result := ownerVerbResult(ran, targets, "check "+run.contract.Check+" passed against "+address,
		map[string]any{"run": run.key, "group": run.contract.Check, "address": address, "verdict": verdict, "at": at})
	if result.Outcome != intentConfirmed {
		result.Summary = "check " + run.contract.Check + " did not pass against " + address + ": " + result.Summary
	}
	return result
}

// appCheckArgv is the bridge to the testing contract's own runner: the one
// form that runs a group by name (the diagnostic canary), executed freshly,
// with the run's address as the runner's one declared input. Nothing else
// runs a check, and nothing here runs a test itself.
func appCheckArgv(installation, group, address string) []string {
	return []string{"test", "run", "--root", installation, "--mode", "canary",
		"--groups", group, "--no-reuse", "--app-address", address, "--json"}
}

// logWriter collects a verb's own narration into the result's text.
func logWriter(lines *[]string) *lineCollector { return &lineCollector{lines: lines} }

type lineCollector struct {
	lines   *[]string
	partial string
}

func (c *lineCollector) Write(data []byte) (int, error) {
	c.partial += string(data)
	for {
		line, rest, found := strings.Cut(c.partial, "\n")
		if !found {
			break
		}
		if strings.TrimSpace(line) != "" {
			*c.lines = append(*c.lines, line)
		}
		c.partial = rest
	}
	return len(data), nil
}
