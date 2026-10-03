package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type partnerOwners struct {
	brain   *brainActDependencies
	running func(string) (bool, error)
	command func(conf, runtime, model, directory string) (launch.Command, error)
	replace func(launch.Command) error
}

func partnerIntentCommands() []intentCommand {
	return []intentCommand{
		{object: "partner", action: "start", audience: "human", summary: "open your project partner in this checkout",
			usage:    []string{"metasystem partner start [--for DURATION | --until TIME] [--runtime RUNTIME] [--model MODEL]"},
			details:  []string{"Run at your own enrolled terminal. Declares this checkout the partner's and grants everything for this machine, checkout and project-partner lineage, at most one week. A live grant is reused; otherwise --for or --until is required. The runtime replaces this command in the same terminal.", "Uses ui.partner.runtime and ui.partner.model. An unset runtime selects the first available runtime. The partner never claims, builds or lands. End the mode with metasystem partner end."},
			flags:    []intentFlag{{name: "for", value: "DURATION", usage: "grant for 8h, 24h, 7d or 1w"}, {name: "until", value: "TIME", usage: "grant until a local time, tomorrow or a date within one week"}, {name: "runtime", value: "RUNTIME", usage: "claude, codex or devin for this start"}, {name: "model", value: "MODEL", usage: "model for this start"}},
			examples: []string{"metasystem partner start --for 1w"}, run: runIntentPartnerStart},
		{object: "partner", action: "end", audience: "human", summary: "revoke the partner's grant and withdraw its declaration",
			usage: []string{"metasystem partner end"}, details: []string{"Run at your own enrolled terminal. A grant never stands in for this proof. Revoking a grant alone leaves the partner role and fences in place."},
			examples: []string{"metasystem partner end"}, run: runIntentPartnerEnd},
	}
}

func partnerCommand(conf, runtime, model, directory string) (launch.Command, error) {
	read := func(key, override string) (string, error) {
		value, _, err := config.Get(config.GetParams{Key: key, Flag: override, FlagSet: override != "", ConfPath: conf})
		return value, err
	}
	var err error
	if runtime, err = read(config.UIPartnerRuntimeKey, runtime); err != nil {
		return launch.Command{}, err
	}
	if model, err = read(config.UIPartnerModelKey, model); err != nil {
		return launch.Command{}, err
	}
	if runtime == "" || runtime == config.AutoRuntime {
		choice, err := config.ResolveAutoRuntime(conf, os.LookupEnv)
		if err != nil {
			return launch.Command{}, err
		}
		if !choice.Detected {
			return launch.Command{}, fmt.Errorf("no partner runtime is installed on PATH; install claude, codex or devin, then run metasystem partner start")
		}
		runtime = choice.Runtime
	}
	return launch.PartnerCommand(runtime, model, directory)
}

func partnerRunning(root string) (bool, error) {
	holder, err := lease.CurrentHolder(root)
	if errors.Is(err, lease.ErrLeaseAbsent) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, announcement := range lease.AnnouncementsForOwnerLineage(root, launch.PartnerOwnerLineage) {
		if announcement.MainId != holder.MainId {
			continue
		}
		state := identity.AliveRef(identity.KernelProber{}, identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt, StartTicks: announcement.PidStartTicks, BootID: announcement.BootID})
		if state == identity.Unknown {
			return false, fmt.Errorf("the partner session's process identity cannot be proved")
		}
		return state == identity.Alive, nil
	}
	return false, nil
}

func (inv *intentInvocation) partnerInputs() (brainActDependencies, humanauthority.Proof, string, *intentResult) {
	if problem := inv.selectRoot(); problem != nil {
		return brainActDependencies{}, humanauthority.Proof{}, "", problem
	}
	deps := defaultBrainActDependencies()
	if inv.owners.partner.brain != nil {
		deps = *inv.owners.partner.brain
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return deps, humanauthority.Proof{}, "", partnerProblem(err)
	}
	proof, err := proveGoalHumanAuthorityFor(ownercall.CurrentProcess(), "partner", &syncFlags{root: inv.stateRoot}, inv.owners.prove, inv.owners.commandNow)
	if err != nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
		return deps, proof, "", partnerProblem(fmt.Errorf("partner %s is the person's own act, at their own enrolled terminal; a grant or the helm never stands in for it; nothing was done", inv.command.action))
	}
	enrollment, err := humanauthority.ReadEnrollment(inv.stateRoot)
	if err != nil {
		return deps, proof, "", partnerProblem(err)
	}
	deps.classify = func(root string, pid int64) (lease.ClassifyResult, error) {
		return coordinatorDirectCaller(root, pid, now, func(r string, p int64, reader humanauthority.Reader, at time.Time) (humanauthority.Proof, error) {
			return inv.owners.prove(r, p, reader, "", "", at)
		})
	}
	return deps, proof, enrollment.Human, nil
}

func partnerProblem(err error) *intentResult {
	return &intentResult{Outcome: intentRefused, code: 2, Summary: err.Error()}
}

func (inv *intentInvocation) partnerGrants(machine, checkout string, now time.Time) ([]goal.PowerOfAttorneyEntry, error) {
	entries, err := inv.owners.attorney.withDefaults().entries(inv.stateRoot)
	if err != nil {
		return nil, err
	}
	var live []goal.PowerOfAttorneyEntry
	for _, entry := range entries {
		if ok, _ := entry.LiveAt(now); ok && entry.General() && entry.For == machine && entry.Checkout == checkout && entry.Lineage == launch.PartnerOwnerLineage {
			live = append(live, entry)
		}
	}
	return live, nil
}

func runIntentPartnerStart(inv *intentInvocation) int {
	deps, _, by, problem := inv.partnerInputs()
	if problem != nil {
		return inv.render(*problem)
	}
	state := brain.Read(inv.stateRoot, deps.ledgerIdentity(inv.stateRoot))
	if state.State == brain.Corrupt {
		return inv.render(*partnerProblem(errors.New(brain.RemedialRefusal(state.Reason, inv.stateRoot))))
	}
	if state.State == brain.Declared && state.Record.Role != brain.Partner {
		return inv.render(*partnerProblem(fmt.Errorf("this checkout is declared the brain; withdraw it first: metasystem settings coordinator --withdraw --by %s --repo %s", state.Record.DeclaredBy, inv.stateRoot)))
	}
	running := inv.owners.partner.running
	if running == nil {
		running = partnerRunning
	}
	active, err := running(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	if active {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "a project partner session already runs here; nothing was opened"})
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	machine, err := deps.machine(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	checkout, err := canonicalCheckout(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	grants, err := inv.partnerGrants(machine, checkout, now)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	zone := inv.owners.helm.withDefaults().zone
	var end time.Time
	if len(grants) > 0 {
		end, _ = time.Parse(time.RFC3339, grants[0].Until)
	} else {
		end, err = parseGrantEnd(now, zone, inv.input.text("for"), inv.input.text("until"))
		if err != nil {
			return inv.render(*partnerProblem(err))
		}
	}
	command := inv.owners.partner.command
	if command == nil {
		command = partnerCommand
	}
	launchCommand, err := command(filepath.Join(inv.layout.InstallationRoot.Path(), "metasystem.conf"), inv.input.text("runtime"), inv.input.text("model"), inv.layout.RepositoryRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	notice := fmt.Sprintf("From now until %s the agent opened here acts in your name on the backlog and this checkout's machinery. Every act states its impact first and is recorded as yours through the partner. It never claims, builds or lands. End it: metasystem partner end.", end.In(zone).Format("Mon 15:04 (2006-01-02)"))
	stream := inv.stdout
	if inv.input.switched("json") {
		stream = inv.stderr
	}
	fmt.Fprintln(stream, notice)
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return brainDeclareRoleWith(ownercall.CurrentProcess(), stdout, stderr, inv.stateRoot, by, brain.Partner, false, deps)
	})
	if ran.code != 0 {
		return inv.render(ownerVerbResult(ran, nil, "", nil))
	}
	id := ""
	if len(grants) > 0 {
		id = grants[0].ID
	} else {
		report := &ownerReport{}
		dependencies := inv.owners.dependencies
		dependencies.report = report
		code := runGoalGrantGeneralWithInputs([]string{"--root", inv.stateRoot, "--by", by}, goal.GeneralGrant{Machine: machine, Checkout: checkout, Lineage: launch.PartnerOwnerLineage, Until: end}, inv.owners.prove, inv.owners.commandNow, dependencies)
		if code != 0 {
			return inv.render(ownerResult(report, code, intentResult{Summary: "the checkout remains declared the partner's; its grant could not be recorded"}))
		}
		id = report.entry
	}
	inv.render(intentResult{Outcome: intentConfirmed, Summary: "opening the project partner here", Data: map[string]any{"grant": id, "role": brain.Partner, "lineage": launch.PartnerOwnerLineage, "until": end.UTC().Format(time.RFC3339)}})
	replace := inv.owners.partner.replace
	if replace == nil {
		replace = launch.ReplaceWithPartner
	}
	if err := replace(launchCommand); err != nil {
		fmt.Fprintf(inv.stderr, "the partner could not open: %v; the declaration and grant remain; retry metasystem partner start or end it with metasystem partner end\n", err)
		return 1
	}
	return 0
}

func runIntentPartnerEnd(inv *intentInvocation) int {
	deps, _, by, problem := inv.partnerInputs()
	if problem != nil {
		return inv.render(*problem)
	}
	state := brain.Read(inv.stateRoot, deps.ledgerIdentity(inv.stateRoot))
	if state.State != brain.Undeclared && (state.Record == nil || state.Record.Role != brain.Partner) {
		return inv.render(*partnerProblem(fmt.Errorf("this is not a project partner declaration; use metasystem settings coordinator --withdraw --by %s", by)))
	}
	machine, err := deps.machine(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	checkout, err := canonicalCheckout(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	grants, err := inv.partnerGrants(machine, checkout, now)
	if err != nil {
		return inv.render(*partnerProblem(err))
	}
	for _, entry := range grants {
		if _, err := inv.owners.attorney.withDefaults().exclusive(inv.stateRoot, revokeLockWait); err != nil {
			return inv.render(*partnerProblem(err))
		}
		report := &ownerReport{}
		dependencies := inv.owners.dependencies
		dependencies.report = report
		code := runGoalRevokeWithInputs([]string{"--root", inv.stateRoot, "--by", by, "--id", entry.ID}, inv.owners.prove, inv.owners.commandNow, dependencies)
		if code != 0 {
			return inv.render(ownerResult(report, code, intentResult{Summary: "the grant could not be revoked; the partner declaration remains"}))
		}
	}
	ran := ownerCall(func(stdout, stderr io.Writer) int {
		return brainWithdrawWith(ownercall.CurrentProcess(), stdout, stderr, inv.stateRoot, by, false, deps)
	})
	return inv.render(ownerVerbResult(ran, nil, "the project partner's grant is revoked and its declaration withdrawn", nil))
}
