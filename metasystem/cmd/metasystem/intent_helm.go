package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// helmYielding names the boundaries that yield under the helm in this binary;
// each unit that adds a yield appends its clause, so help never promises a
// yield the binary does not make.
var helmYielding = []string{"status names the person at the helm"}

// helmOwners are the facts helm take reads besides the signature. Zero values
// are the production readers.
type helmOwners struct {
	reader  humanauthority.Reader
	pid     func() int64
	now     func() time.Time
	machine func(string) (string, error)
	account func() string
	zone    *time.Location
}

func (o helmOwners) withDefaults() helmOwners {
	if o.reader == nil {
		o.reader = humanauthority.KernelReader{}
	}
	if o.pid == nil {
		o.pid = func() int64 { return int64(os.Getppid()) }
	}
	if o.now == nil {
		o.now = time.Now
	}
	if o.machine == nil {
		o.machine = goal.ResolveMachine
	}
	if o.account == nil {
		o.account = func() string {
			if current, err := user.Current(); err == nil {
				return current.Username
			}
			return os.Getenv("USER")
		}
	}
	if o.zone == nil {
		o.zone = time.Local
	}
	return o
}

func helmIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "helm", action: "take", primary: true, audience: "human", summary: "take this seat out of the machinery's hands until you return it",
		usage: []string{"metasystem helm take --reason TEXT [--by NAME]"},
		details: []string{
			"I take the helm of this seat (this checkout and its linked worktrees). From now until the helm is returned, MetaSystem decides nothing for this seat: " +
				strings.Join(helmYielding, ", ") + ". What runs keeps running: the helm stops or contains no job. Records are still written and tests still report what they find.",
			"Run it at a terminal; it works when the ledger, the enrollment or supervision is broken. It refuses an agent anywhere in its chain; it cannot see who created the terminal (metasystem status names the session leader). Taking it again is fine.",
		},
		flags:    []intentFlag{{name: "reason", value: "TEXT", usage: "why you take the helm"}, {name: "by", value: "NAME", advanced: true, usage: "your name (default: the enrolled name, else your account)"}},
		examples: []string{`metasystem helm take --reason "coordinating the U5 builders by hand tonight"`},
		run:      runIntentHelmTake,
	}}
}

func (inv *intentInvocation) helmPath() string {
	if !inv.input.has("repo") {
		return inv.cwd
	}
	if path := inv.input.text("repo"); !filepath.IsAbs(path) {
		return filepath.Join(inv.cwd, path)
	}
	return inv.input.text("repo")
}

// runIntentHelmTake proves the person at the terminal, attributes the take
// from the enrollment, and writes the signature. It reads no ledger,
// announcement or supervision state.
func runIntentHelmTake(inv *intentInvocation) int {
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "helm take needs a reason: --reason TEXT; nothing was done"})
	}
	owners, path := inv.owners.helm.withDefaults(), inv.helmPath()
	seat, err := helm.Locate(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "helm take: " + err.Error()})
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	root := layout.InstallationRoot
	if err == nil {
		root, err = inv.owners.resolver.RootForInstallation(root)
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "helm take: the installation cannot be found: " + err.Error()})
	}
	now, pid := owners.now().UTC(), owners.pid()
	proof, err := humanauthority.ProveTerminal(root, pid, owners.reader, now)
	if err == nil && !proof.TerminalValidFor(root) {
		err = fmt.Errorf("terminal human authority was not proven")
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 3, Summary: "helm take refused: only a person at a terminal takes the helm: " + err.Error(),
			Decision: "run metasystem helm take yourself, at a terminal no agent started"})
	}
	record := helm.Record{By: strings.TrimSpace(inv.input.text("by")), At: now.Format(time.RFC3339), Reason: reason, Checkout: seat.Checkout, Enrollment: "unreadable"}
	if enrollment, readErr := humanauthority.ReadEnrollment(root); readErr == nil {
		record.Enrollment, record.EnrolledAs = "other-terminal", enrollment.Human
		if _, proveErr := humanauthority.Prove(root, pid, owners.reader, now); proveErr == nil {
			record.Enrollment = "proven"
			if record.By == "" {
				record.By = enrollment.Human
			}
		}
	}
	if record.By == "" {
		record.By = owners.account()
	}
	if record.Machine, err = owners.machine(root); err != nil {
		record.Machine, _ = os.Hostname()
	}
	record.LeaderRef = fmt.Sprintf("%d@%d", proof.TerminalRef.PID, proof.TerminalRef.PIDStartedAt)
	if leader, readErr := owners.reader.Read(proof.TerminalRef.PID); readErr == nil {
		record.Leader = filepath.Base(leader.Executable)
	}
	standing := helm.Active(path)
	entry := helm.Entry{At: now, Action: "take", By: record.By, Reason: reason, Leader: record.Leader}
	targets := []intentTarget{{Kind: "seat", ID: seat.Checkout}}
	switch {
	case standing.Active && standing.Malformed == "" && standing.By == record.By && standing.Reason == reason:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "applied: " + helmLine(standing.Record, standing.Since, owners.zone),
			text: helmTerminalLines(standing.Record), Data: map[string]any{"helm": standing.Record}})
	case standing.Active && standing.Malformed == "" && standing.By == record.By:
		record.At = standing.Record.At
	case standing.Active:
		entry.Replaced = standing.By
	}
	if _, err := helm.Write(path, record); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "helm take: the signature cannot be written: " + err.Error(),
			Decision: "make " + seat.Dir + " writable by you (chmod u+w), then take the helm again"})
	}
	lines := helmTerminalLines(record)
	if err := helm.Log(path, entry); err != nil {
		lines = append(lines, "helm.log was not appended: "+err.Error())
	}
	if entry.Replaced != "" {
		lines = append(lines, "replaces "+entry.Replaced+" at the helm; both names are in "+seat.Log)
	}
	since, _ := time.Parse(time.RFC3339, record.At)
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: helmLine(record, since, owners.zone), text: lines, Data: map[string]any{"helm": record}})
}

func helmLine(record helm.Record, since time.Time, zone *time.Location) string {
	local := since.In(zone)
	return fmt.Sprintf("HUMAN AT THE HELM since %s by %s: %s", local.Format("15:04 MST (2006-01-02)"), record.By, record.Reason)
}

func helmTerminalLines(record helm.Record) []string {
	switch record.Enrollment {
	case "proven":
		return []string{"taken at the enrolled terminal"}
	case "other-terminal":
		return []string{fmt.Sprintf("taken at a terminal that is not enrolled; session leader %s (%s)", record.Leader, record.LeaderRef)}
	}
	return []string{fmt.Sprintf("taken at a terminal; the enrollment was unreadable; session leader %s (%s)", record.Leader, record.LeaderRef)}
}
