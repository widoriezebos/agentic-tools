package main

// The deploy object (landing-deploys-the-engine, Decisions 2 to 4): the
// project's deploy of origin's main through the adapter its deploy.json
// names. deploy status reads it and deploy now runs it, for a person, the
// lane and a seat; rollback, pause and resume are a person's acts. A
// project without the contract file has no deploy, and each verb says so.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// deployOwners are the deploy verbs' seams; the zero value is production.
type deployOwners struct {
	// home is the ~/.metasystem directory every seat on the computer shares.
	home func() (string, error)
	// person proves the person at the enrolled terminal and names them.
	person func(root string) (string, error)
	now    func() time.Time
	// origin is a checkout's origin fetch URL, the project's name.
	origin func(root string) (string, error)
	// git is the runner's way to a checkout's repository.
	git func(root string) deploy.Git
	// path is the shell's search path, for status's line to add.
	path func() string
	// executable and launch start deploy now detached after a push.
	executable func() (string, error)
	launch     func(argv []string, dir, log string) (int64, error)
	// started hears each adapter process a run starts.
	started func(deploy.Active)
}

func (inv *intentInvocation) deploy() deployOwners {
	owners := inv.owners.deploy
	if owners.home == nil {
		owners.home = board.Home
	}
	if owners.person == nil {
		owners.person = provenPerson(humanauthority.KernelReader{}, func() int64 { return int64(os.Getppid()) }, goalCommandNow)
	}
	if owners.now == nil {
		owners.now = func() time.Time { return time.Now().UTC() }
	}
	if owners.origin == nil {
		owners.origin = deploy.OriginURL
	}
	if owners.git == nil {
		owners.git = deploy.CheckoutGit
	}
	if owners.path == nil {
		owners.path = func() string { return os.Getenv("PATH") }
	}
	if owners.executable == nil {
		owners.executable = os.Executable
	}
	if owners.launch == nil {
		owners.launch = func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log})
		}
	}
	return owners
}

// deployProject is a checkout's declared deploy: where its installation
// lies in a tree of the repository, and the shared directory of its
// repository's record, lock and pause.
type deployProject struct {
	owners       deployOwners
	root         string
	installation string
	key          string
	home         string
	dir          string
}

// resolveDeployProject reads whether the checkout at root declares a
// deploy; its deploy.json is never what an adapter is called by.
func resolveDeployProject(owners deployOwners, installation, root string) (deployProject, error) {
	inside, err := deploy.Declared(installation, root)
	if err != nil {
		return deployProject{}, err
	}
	url, err := owners.origin(root)
	if err != nil {
		return deployProject{}, fmt.Errorf("the repository's origin can't be read, so its deploy can't be named: %w", err)
	}
	key, err := deploy.ProjectKey(url)
	if err != nil {
		return deployProject{}, err
	}
	home, err := owners.home()
	if err != nil {
		return deployProject{}, fmt.Errorf("the home directory can't be found: %w", err)
	}
	return deployProject{owners: owners, root: root, installation: inside, key: key, home: home, dir: deploy.Dir(home, key)}, nil
}

func (p deployProject) runner(by string) *deploy.Runner {
	return &deploy.Runner{Dir: p.dir, Project: p.key, Installation: p.installation, Git: p.owners.git(p.root), By: by, Now: p.owners.now, Started: p.owners.started}
}

// deployCommand declares a deploy verb whose run starts once the checkout's
// deploy was read; a project without one is told so, and nothing is done.
func deployCommand(command intentCommand, run func(*intentInvocation, deployProject) int) intentCommand {
	command.run = func(inv *intentInvocation) int {
		if problem := inv.resolveLayout(); problem != nil {
			return inv.render(*problem)
		}
		project, err := resolveDeployProject(inv.deploy(), inv.layout.InstallationRoot.Path(), inv.layout.GitRoot)
		switch {
		case errors.Is(err, deploy.ErrNoContract):
			return inv.render(intentResult{Outcome: intentUnchanged, Summary: deploy.ErrNoContract.Error() + ", so there is nothing to deploy and nothing was done",
				Details: []string{"a project declares its deploy in deploy.json, or in the file the setting " + deploy.ContractKey + " names"}})
		case err != nil:
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "this project's deploy can't be read: " + oneLine(err.Error()),
				retry: "after the deploy contract or the repository's origin is fixed", Details: []string{err.Error()}})
		}
		return run(inv, project)
	}
	return command
}

// deployPerson proves the person a rollback, pause or resume needs; an
// agent is told who runs it and how.
func (inv *intentInvocation) deployPerson(project deployProject, act string) (string, *intentResult) {
	by, err := project.owners.person(inv.layout.InstallationRoot.Path())
	if err == nil {
		return by, nil
	}
	refused := inv.personRefusal("", err, "")
	refused.code = 3
	refused.Summary = "only a person may " + act + ", and " + strings.TrimSuffix(refused.Summary, ", so nothing was done") + "; nothing was changed"
	return "", refused
}

func deployIntentCommands() []intentCommand {
	return []intentCommand{
		deployCommand(intentCommand{
			object: "deploy", action: "status", audience: "both", summary: "what this project has deployed, what waits, and whether deploys are paused",
			usage: []string{"metasystem deploy status [--history N] [--verify]"},
			details: []string{"The current deploy with its version, commit, time and actor, and the one before it; main's tip when it is not yet active; the pause; a run in progress with its start and when its log last grew; the last failure.",
				"--history N adds the newest N lines of the record; --verify asks the adapter what is active.",
				"When ~/.metasystem/bin is not on PATH it prints the line to add; it never edits a shell profile."},
			flags: []intentFlag{{name: "history", value: "N", usage: "the newest N lines of the deploy record"},
				{name: "verify", usage: "ask the adapter what is active now"}},
			maxArgs:  0,
			examples: []string{"metasystem deploy status", "metasystem deploy status --history 10 --verify"},
		}, runIntentDeployStatus),
		deployCommand(intentCommand{
			object: "deploy", action: "now", audience: "both", summary: "deploy origin's main tip through the project's deploy adapter",
			usage: []string{"metasystem deploy now"},
			details: []string{"Fetches origin's main and builds, activates and verifies its tip in a clean tree; it waits for the adapter however long a build takes.",
				"When main's tip is already active nothing is built or written. While another run holds the deploy, this one exits: that run deploys main's newest tip before it ends.",
				"The landing lane starts it after every push that changed main. Refused while deploys are paused."},
			flags:    []intentFlag{{name: "by", value: "NAME", advanced: true, hidden: true, usage: "who the record names as the deploy's actor"}},
			maxArgs:  0,
			examples: []string{"metasystem deploy now"},
		}, runIntentDeployNow),
		deployCommand(intentCommand{
			object: "deploy", action: "rollback", audience: "human", summary: "return to the deploy before the newest one, and pause deploys",
			usage: []string{"metasystem deploy rollback"},
			details: []string{"A person's act at an enrolled terminal. Activates the deploy that was current before the newest forward deploy, without building, and pauses deploys so the next landing does not deploy the same commit again.",
				"Asked again, it already holds and nothing is written. metasystem deploy resume moves forward again.",
				"Refused while a deploy is in progress, and nothing is changed: metasystem deploy pause stops it, and the rollback can be repeated after it."},
			maxArgs:  0,
			examples: []string{"metasystem deploy rollback"},
		}, runIntentDeployRollback),
		deployCommand(intentCommand{
			object: "deploy", action: "pause", audience: "human", summary: "hold this project's deploys, and stop a run in progress",
			usage: []string{"metasystem deploy pause [--reason TEXT]"},
			details: []string{"A person's act at an enrolled terminal. Holds every deploy of this repository on this computer, the lane's too, until metasystem deploy resume.",
				"It ends the adapter call in progress, a rollback's included, and answers at once: the run whose call it ended appends a stopped line as it stops, and metasystem deploy status shows what is active."},
			flags:    []intentFlag{{name: "reason", value: "TEXT", usage: "why deploys are held, shown by deploy status"}},
			maxArgs:  0,
			examples: []string{"metasystem deploy pause --reason 'the build hangs on the new toolchain'"},
		}, runIntentDeployPause),
		deployCommand(intentCommand{
			object: "deploy", action: "resume", audience: "human", summary: "lift the deploy pause and deploy main's tip",
			usage: []string{"metasystem deploy resume"},
			details: []string{"A person's act at an enrolled terminal. Lifts the pause, then deploys origin's main tip as deploy now does.",
				"Deploys that are not paused change nothing. Refused while a deploy is in progress, and nothing is changed: metasystem deploy pause stops it."},
			maxArgs:  0,
			examples: []string{"metasystem deploy resume"},
		}, runIntentDeployResume),
	}
}

func runIntentDeployStatus(inv *intentInvocation, project deployProject) int {
	history := 0
	if raw := inv.input.text("history"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--history takes a count of lines, not " + strconv.Quote(raw) + "; nothing was read",
				next: inv.publicArgv("deploy", "status", "--history", "10")})
		}
		history = parsed
	}
	status, err := deploy.ReadStatus(project.dir, history)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the deploy record can't be read: " + oneLine(err.Error()), retry: "reads it again", Details: []string{err.Error()}})
	}
	data := map[string]any{"project": project.key, "record": project.dir, "status": status}
	summary := "nothing is deployed yet"
	if current := status.Current; current != nil {
		summary = "deployed " + deployed(*current) + " " + current.EndedAt.Format(time.RFC3339) + " by " + current.By
	} else if inactive := status.Inactive; inactive != nil {
		summary = "nothing is active since " + inactive.EndedAt.Format(time.RFC3339) + ": " + inactive.Detail
	}
	var lines []string
	if previous := status.Previous; previous != nil {
		lines = append(lines, "previous: "+deployed(*previous))
	}
	if tip, err := project.owners.git(project.root).FetchMain(); err != nil {
		lines = append(lines, "main's tip can't be fetched: "+oneLine(err.Error()))
	} else if status.Current == nil || status.Current.Commit != tip {
		data["waiting"] = tip
		lines = append(lines, "main's tip "+deploy.Short(tip)+" is not deployed yet")
	}
	if pause := status.Paused; pause != nil {
		line := "paused by " + pause.By + " " + pause.At.Format(time.RFC3339)
		if pause.Reason != "" {
			line += ": " + pause.Reason
		}
		lines = append(lines, line+"; metasystem deploy resume lifts it")
	}
	if adapter := status.Adapter; adapter != nil {
		line := fmt.Sprintf("running: %s of %s, process %d since %s", adapter.Operation, deploy.Short(adapter.Commit), adapter.PID, adapter.StartedAt.Format(time.RFC3339))
		if status.LogGrewAt != nil {
			line += "; its log last grew " + status.LogGrewAt.Format(time.RFC3339) + " (" + adapter.Log + ")"
		}
		lines = append(lines, line+"; metasystem deploy pause stops it")
	}
	if failure := status.LastFailure; failure != nil {
		lines = append(lines, "last failure: "+failure.Outcome+" of "+deploy.Short(failure.Commit)+" "+failure.EndedAt.Format(time.RFC3339)+": "+failure.Detail+" (log "+failure.Log+")")
	}
	if inv.input.switched("verify") {
		lines = append(lines, inv.verifyDeploy(project, status, data))
	}
	if hint := deployPathHint(project.home, project.owners.path()); hint != "" {
		data["path"] = hint
		lines = append(lines, filepath.Join(project.home, "bin")+" is not on PATH; add this line to your shell profile: "+hint)
	}
	for _, line := range status.History {
		lines = append(lines, "  "+line.EndedAt.Format(time.RFC3339)+"  "+line.Kind+" "+deploy.Short(line.Commit)+" "+line.Outcome+" by "+line.By)
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, Data: data, text: lines})
}

// verifyDeploy asks the adapter what is active and says whether the record
// agrees.
func (inv *intentInvocation) verifyDeploy(project deployProject, status deploy.Status, data map[string]any) string {
	answer, err := project.runner("").Verify()
	if err != nil {
		return "the adapter could not be asked: " + oneLine(err.Error())
	}
	data["verified"] = answer
	if answer.Outcome != deploy.OutcomeActive {
		return "the adapter reports nothing active"
	}
	line := "the adapter reports " + answer.Version + " (" + answer.Artifact + ") active"
	if status.Current == nil || status.Current.Artifact != answer.Artifact || status.Current.Digest != answer.Digest {
		line += ", which is not the record's current deploy; the next deploy now records it"
	}
	return line
}

// deployPathHint is the line that puts ~/.metasystem/bin on the search
// path, or "" when it is there.
func deployPathHint(home, path string) string {
	bin := filepath.Join(home, "bin")
	for _, entry := range filepath.SplitList(path) {
		if entry != "" && filepath.Clean(entry) == bin {
			return ""
		}
	}
	return `export PATH="` + bin + `:$PATH"`
}

func deployed(line deploy.Line) string {
	if line.Version == "" {
		return deploy.Short(line.Commit)
	}
	return line.Version + " (" + deploy.Short(line.Commit) + ")"
}

// pausedRefusal is deploy now's answer while deploys are paused.
func (inv *intentInvocation) pausedRefusal(pause deploy.Pause) intentResult {
	return intentResult{Outcome: intentRefused, code: 1, Summary: "deploys are paused by " + pause.By + ", so nothing was deployed",
		next: inv.publicArgv("deploy", "resume"), nextReason: "a person lifts the pause and deploys main's tip",
		Details: []string{"refused because: " + deploy.CodePaused + ": paused " + pause.At.Format(time.RFC3339) + " " + pause.Reason}}
}

// runIntentDeployNow runs the runner, which refuses at once while deploys
// are paused.
func runIntentDeployNow(inv *intentInvocation, project deployProject) int {
	by := strings.TrimSpace(inv.input.text("by"))
	if by == "" {
		if person, err := project.owners.person(inv.layout.InstallationRoot.Path()); err == nil {
			by = person
		} else {
			by = "the seat at " + project.root
		}
	}
	return inv.render(inv.deployRun(project, by, ""))
}

// deployRun runs the runner and says what it did; done prefixes what the
// verb did before it.
func (inv *intentInvocation) deployRun(project deployProject, by, done string) intentResult {
	report, err := project.runner(by).Run()
	return inv.deployReport(project, report, err, done)
}

// deployReport says what a run did.
func (inv *intentInvocation) deployReport(project deployProject, report deploy.Report, err error, done string) intentResult {
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Data: report, Summary: done + "the deploy run failed: " + oneLine(err.Error()), retry: "runs it again", Details: []string{err.Error()}}
	}
	result := intentResult{Outcome: intentConfirmed, Data: report}
	var last deploy.Line
	if count := len(report.Lines); count > 0 {
		last = report.Lines[count-1]
		if last.Log != "" {
			result.Details = append(result.Details, "the adapter's log: "+last.Log)
		}
	}
	switch report.Outcome {
	case deploy.RunCurrent:
		result.Outcome, result.Summary = intentUnchanged, done+"the deploy already holds: main's tip "+deploy.Short(report.Tip)+" is active, so nothing was built"
	case deploy.RunBusy:
		result.Outcome, result.Summary = intentUnchanged, done+"a deploy run is in progress; it deploys main's newest tip before it ends"
		if holder := report.Holder; holder != nil {
			result.Details = append(result.Details, fmt.Sprintf("the run in progress: %s of %s, process %d", holder.Operation, deploy.Short(holder.Commit), holder.PID))
			if report.Orphaned {
				result.Summary = fmt.Sprintf("%sthe %s of %s that an ended run left running (process %d) still holds the deploy, so nothing was started", done, holder.Operation, deploy.Short(holder.Commit), holder.PID)
				result.next, result.nextReason = inv.publicArgv("deploy", "pause"), "a person ends it; deploy resume then deploys main's tip"
			}
		}
	case deploy.RunPaused:
		pause, _, _ := deploy.ReadPause(project.dir)
		return inv.pausedRefusal(pause)
	case deploy.RunDeployed:
		result.Summary = done + "deployed " + deployed(last)
	case deploy.RunStopped:
		result.Outcome, result.code = intentPartial, 1
		result.Summary = done + "the deploy of " + deploy.Short(last.Commit) + " was stopped: " + last.Detail
		result.next, result.nextReason = inv.publicArgv("deploy", "resume"), "a person lifts the pause and deploys main's tip"
	default:
		result.Outcome, result.code = intentFailed, 1
		result.Summary = done + "the deploy of " + deploy.Short(last.Commit) + " failed (" + last.Outcome + "): " + last.Detail
		result.next, result.nextReason = inv.publicArgv("deploy", "status"), "shows what stays active"
	}
	return result
}

func runIntentDeployRollback(inv *intentInvocation, project deployProject) int {
	by, refused := inv.deployPerson(project, "roll a deploy back")
	if refused != nil {
		return inv.render(*refused)
	}
	report, err := project.runner(by).Rollback()
	var refusal *deploy.Refusal
	switch {
	case errors.As(err, &refusal) && refusal.Code == deploy.CodeRunning:
		return inv.render(inv.runningRefusal("rollback", refusal))
	case errors.As(err, &refusal):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Data: report, Summary: "there is no deploy to roll back to (" + refusal.Reason + "), so nothing was changed",
			next: inv.publicArgv("deploy", "status"), nextReason: "shows the deploy record",
			Details: []string{"refused because: " + refusal.Code + ": " + refusal.Reason}})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Data: report, Summary: "the rollback could not be made: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	resume := inv.publicArgv("deploy", "resume")
	switch report.Outcome {
	case deploy.RollbackHolds:
		return inv.render(intentResult{Outcome: intentUnchanged, Data: report, Summary: "the rollback to " + deployed(report.Target) + " already holds; deploys stay paused",
			next: resume, nextReason: "moves forward again"})
	case deploy.RollbackDone:
		return inv.render(intentResult{Outcome: intentConfirmed, Data: report, Summary: "rolled back to " + deployed(report.Target) + " and paused deploys",
			next: resume, nextReason: "moves forward again, to main's tip"})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Data: report,
		Summary: "the rollback to " + deployed(report.Target) + " failed (" + report.Line.Outcome + "): " + report.Line.Detail + "; deploys stay paused",
		next:    inv.publicArgv("deploy", "status", "--verify"), nextReason: "asks the adapter what is active",
		Details: []string{"the adapter's log: " + report.Line.Log}})
}

func runIntentDeployPause(inv *intentInvocation, project deployProject) int {
	by, refused := inv.deployPerson(project, "pause deploys")
	if refused != nil {
		return inv.render(*refused)
	}
	report, err := project.runner(by).Pause(strings.TrimSpace(inv.input.text("reason")))
	if _, paused, _ := deploy.ReadPause(project.dir); err != nil && paused {
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Data: report, Summary: "deploys are paused, but the adapter call in progress could not be ended: " + oneLine(err.Error()),
			retry: "tries again", Details: []string{err.Error()}})
	} else if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "deploys could not be paused: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	summary := "paused deploys of this project until metasystem deploy resume"
	if report.Already {
		pause, _, _ := deploy.ReadPause(project.dir)
		summary = "deploys are already paused by " + pause.By
	}
	if report.Stopped != nil {
		summary += "; " + stoppedRun(*report.Stopped) + ", and the run in progress is stopping"
	}
	result := intentResult{Outcome: intentConfirmed, Summary: summary, Data: report,
		next: inv.publicArgv("deploy", "status"), nextReason: "shows what is active"}
	if report.Already && report.Stopped == nil {
		result.Outcome = intentUnchanged
	}
	return inv.render(result)
}

// runningRefusal is a rollback's or resume's answer while a deploy is in
// progress: deploy pause stops it, and the act can be repeated after it.
func (inv *intentInvocation) runningRefusal(act string, refusal *deploy.Refusal) intentResult {
	return intentResult{Outcome: intentRefused, code: 1, Summary: "a deploy is in progress, so nothing was changed",
		next: inv.publicArgv("deploy", "pause"), nextReason: "stops it; then run metasystem deploy " + act + " again",
		Details: []string{"refused because: " + refusal.Code + ": " + refusal.Reason}}
}

// stoppedRun names the adapter call in progress that a pause ended.
func stoppedRun(stopped deploy.Active) string {
	return fmt.Sprintf("ended the %s of %s in progress (process group %d)", stopped.Operation, deploy.Short(stopped.Commit), stopped.PID)
}

func runIntentDeployResume(inv *intentInvocation, project deployProject) int {
	by, refused := inv.deployPerson(project, "resume deploys")
	if refused != nil {
		return inv.render(*refused)
	}
	was, paused, err := project.runner(by).Resume()
	var refusal *deploy.Refusal
	if errors.As(err, &refusal) {
		return inv.render(inv.runningRefusal("resume", refusal))
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the deploy pause could not be lifted: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	if !paused {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "deploys are not paused, so nothing was changed",
			next: inv.publicArgv("deploy", "now"), nextReason: "deploys main's tip"})
	}
	return inv.render(inv.deployRun(project, by, "lifted the pause by "+was.By+"; "))
}

// deployAfterPush is the lane's call after a push: when main changed and
// the project declares a deploy, it starts deploy now detached, the way
// landing prove starts its proof, and says so in one detail line. A failed
// deploy is told by one more line. A failure to start fails nothing, and a
// project without the contract file does nothing more after its push.
func (inv *intentInvocation) deployAfterPush(installation, checkout string, changed bool) []string {
	owners := inv.deploy()
	project, err := resolveDeployProject(owners, installation, checkout)
	if errors.Is(err, deploy.ErrNoContract) {
		return nil
	}
	if err != nil {
		if !changed {
			return nil
		}
		return []string{"no deploy was started: " + oneLine(err.Error())}
	}
	var told []string
	if lines, err := deploy.Lines(project.dir); err == nil && len(lines) > 0 {
		if last := lines[len(lines)-1]; last.Failed() {
			told = append(told, "the last deploy failed: "+last.Outcome+" of "+deploy.Short(last.Commit)+": "+oneLine(last.Detail)+"; metasystem deploy status shows it")
		}
	}
	if !changed {
		return told
	}
	if pause, paused, _ := deploy.ReadPause(project.dir); paused {
		return append(told, "deploys are paused by "+pause.By+", so no deploy was started; main's tip waits for metasystem deploy resume")
	}
	started, err := startDeployNow(owners, installation, project.dir)
	if err != nil {
		return append(told, "metasystem deploy now could not be started, so main's tip waits for the next push or deploy now: "+oneLine(err.Error()))
	}
	return append(told, fmt.Sprintf("started metasystem deploy now for main's tip (process %d); metasystem deploy status shows it", started))
}

func startDeployNow(owners deployOwners, installation, dir string) (int64, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	executable, err := owners.executable()
	if err != nil {
		return 0, err
	}
	return owners.launch([]string{executable, "deploy", "now", "--by", "the landing lane"}, installation, filepath.Join(dir, "lane.log"))
}
