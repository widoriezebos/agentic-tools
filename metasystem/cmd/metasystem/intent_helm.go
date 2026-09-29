package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// helmYielding names the boundaries that yield under the helm in this binary;
// each unit that adds a yield appends its clause, so help never promises a
// yield the binary does not make.
var helmYielding = []string{"the hooks allow every turn", "the tool gate is silent", "status names the person at the helm"}

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
			"I take the helm of this seat (this checkout and its linked worktrees). From now until metasystem helm return, MetaSystem decides nothing for this seat: " +
				strings.Join(helmYielding, ", ") + ". What runs keeps running: the helm stops or contains no job. Records are still written and tests still report what they find.",
			"Run it at a terminal; it works when the ledger, the enrollment or supervision is broken. It refuses an agent anywhere in its chain; it cannot see who created the terminal (metasystem status names the session leader). Taking it again is fine.",
		},
		flags:    []intentFlag{{name: "reason", value: "TEXT", usage: "why you take the helm"}, {name: "by", value: "NAME", advanced: true, usage: "your name (default: the enrolled name, else your account)"}},
		examples: []string{`metasystem helm take --reason "coordinating the U5 builders by hand tonight"`},
		run:      runIntentHelmTake,
	}, {
		object: "helm", action: "return", audience: "both", summary: "give the seat back to the machinery",
		usage: []string{"metasystem helm return"},
		details: []string{"I give the seat back to the machinery. It removes the signature first, then shows what changed while at the helm and what the machinery sees now; it starts, stops, claims or cleans nothing.",
			"Anyone in the seat may run it; returning a helm nobody holds is fine."},
		examples: []string{"metasystem helm return"},
		run:      runIntentHelmReturn,
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
		return inv.render(intentResult{Outcome: intentRefused, code: 3,
			Summary:  "only a person at a terminal no agent started can take the helm, and this shell is not one (" + actorProofReason(err) + "); nothing was done",
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
	return fmt.Sprintf("HUMAN AT THE HELM since %s by %s: %s — metasystem helm return ends it", local.Format("15:04 MST (2006-01-02)"), record.By, record.Reason)
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

// runIntentHelmReturn removes the signature first; everything after it is a
// best-effort report that can never undo or block the return.
func runIntentHelmReturn(inv *intentInvocation) int {
	path, owners := inv.helmPath(), inv.owners.helm.withDefaults()
	removal, err := helm.Remove(path)
	switch {
	case removal.Seat.CommonDir == "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "helm return: " + err.Error()})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "helm return: " + err.Error(),
			Decision: "make " + removal.Seat.Dir + " writable by you (chmod u+w) and remove what is at the signature path, then return again"})
	case !removal.Present:
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the machinery is at the helm; nothing to return"})
	}
	record, problem := helm.Decode(removal.Raw)
	if removal.ReadErr != nil {
		problem = removal.ReadErr.Error()
	}
	since, _ := time.Parse(time.RFC3339, record.At)
	if problem != "" {
		record = helm.Record{By: "unknown", Reason: problem}
	}
	lines := []string{"returned the helm " + record.By + " held: " + record.Reason}
	if err := helm.Log(path, helm.Entry{At: owners.now(), Action: "return", By: record.By, Reason: problem}); err != nil {
		lines = append(lines, "helm.log was not appended: "+err.Error())
	}
	lines = append(lines, inv.helmReport(removal.Seat, since)...)
	lines = append(lines, "supervision re-arms at the next turn end (the Stop hook arms it) or now with metasystem system start")
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "seat", ID: removal.Seat.Checkout}},
		Summary: "the machinery is at the helm again", text: lines, Data: map[string]any{"returned": record}})
}

// helmReport is the best-effort account shared by status and return: yields
// since the take, running work, and the landing batches holding the seat's
// work. Each source that fails says so in its place.
func (inv *intentInvocation) helmReport(seat helm.Seat, since time.Time) []string {
	lines := []string{fmt.Sprintf("yields since the take: %s", helmYieldCount(seat, since))}
	layout, err := inv.owners.resolver.ResolveLayout(seat.Checkout)
	if err != nil {
		return append(lines, "running work: unavailable: "+err.Error(), "landing batches: unavailable: "+err.Error())
	}
	running := 0
	paths, _ := filepath.Glob(filepath.Join(layout.InstallationRoot, "artifacts", "agents", "jobs", "*.json"))
	for _, path := range paths {
		if object, readErr := dispatchcore.ReadRecordObject(path); readErr == nil && !dispatchcore.TerminalStatus(fmt.Sprint(object["status"])) {
			running++
		}
	}
	lines = append(lines, fmt.Sprintf("running dispatch jobs: %d (metasystem work status lists them; the helm stops none)", running))
	held, err := helmHeldBatches(layout.InstallationRoot, seat)
	if err != nil {
		return append(lines, "landing batches: unavailable: "+err.Error())
	}
	if len(held) == 0 {
		held = []string{"none"}
	}
	return append(lines, "landing batches carrying this seat's work: "+strings.Join(held, ", "))
}

func helmYieldCount(seat helm.Seat, since time.Time) string {
	file, err := os.Open(seat.Yields)
	if os.IsNotExist(err) {
		return "0"
	}
	if err != nil {
		return "unavailable: " + err.Error()
	}
	defer file.Close()
	count := 0
	for scanner := bufio.NewScanner(file); scanner.Scan(); {
		var yield helm.Yield
		if json.Unmarshal(scanner.Bytes(), &yield) == nil && !yield.At.Before(since) {
			count++
		}
	}
	return fmt.Sprint(count)
}

// helmHeldBatches reads the configured landing checkout's batch records
// directly and names those with a unit this seat joined.
func helmHeldBatches(installation string, seat helm.Seat) ([]string, error) {
	root, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(installation, "metasystem.conf"), Default: "", DefaultSet: true})
	if err != nil || strings.TrimSpace(root) == "" {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "landing-batches", "*.json"))
	var held []string
	for _, path := range paths {
		var record struct {
			BatchID, State string
			Units          []struct{ SeatRoot string }
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			readErr = json.Unmarshal(data, &record)
		}
		if readErr != nil {
			return held, readErr
		}
		for _, unit := range record.Units {
			if other, locateErr := helm.Locate(unit.SeatRoot); unit.SeatRoot != "" && locateErr == nil && other.CommonDir == seat.CommonDir {
				held = append(held, record.BatchID+" ("+record.State+")")
				break
			}
		}
	}
	return held, err
}

// helmStatusLines are the first lines of status while the seat is at the helm.
func (inv *intentInvocation) helmStatusLines(path string) []string {
	state := helm.Active(path)
	if !state.Active {
		return nil
	}
	zone := inv.owners.helm.withDefaults().zone
	if state.Malformed != "" {
		return []string{"HUMAN AT THE HELM (the signature is unreadable: " + state.Malformed + ") — metasystem helm return ends it"}
	}
	seat, _ := helm.Locate(path)
	return append(append([]string{helmLine(state.Record, state.Since, zone)}, helmTerminalLines(state.Record)...), inv.helmReport(seat, state.Since)...)
}

// withHelm puts the helm lines first in a status result while the seat is at
// the helm, and leaves it untouched otherwise.
func (inv *intentInvocation) withHelm(result intentResult, path string) intentResult {
	lines := inv.helmStatusLines(path)
	if lines == nil {
		return result
	}
	result.text = append(append(append([]string{}, lines[1:]...), result.Summary), result.text...)
	result.Summary = lines[0]
	if data, ok := result.Data.(map[string]any); ok {
		data["helm"] = lines
	}
	return result
}

// actorProofReason is why this shell was not proven to be a person at a
// terminal, in the reader's terms rather than the proof's outcome code; an
// error the proof does not classify is kept as it is.
func actorProofReason(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, humanauthority.OutcomeAgent):
		return "an agent started this shell"
	case strings.Contains(text, humanauthority.OutcomeTerminalMissing):
		return "no terminal was found above this shell"
	case strings.Contains(text, humanauthority.OutcomeNotEnrolled):
		return "this terminal is not the enrolled one"
	}
	return text
}
