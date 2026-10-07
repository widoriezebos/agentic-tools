package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// helmYielding names the boundaries that yield under the helm in this binary;
// each unit that adds a yield appends its clause, so help never promises a
// yield the binary does not make.
var helmYielding = []string{"the hooks allow every turn", "the tool gate is silent", "status names the person at the helm", "the pre-commit guard admits your commits in the checkout you sit in"}

// helmOwners are the facts helm take reads besides the signature, and what
// helm return's catch-up reads and runs. Zero values are the production
// readers.
type helmOwners struct {
	actor *helmActor

	reader  humanauthority.Reader
	pid     func() int64
	now     func() time.Time
	machine func(string) (string, error)
	account func() string
	zone    *time.Location

	git           func(dir string, args ...string) (string, error)
	stdin         io.Reader
	stdinTerminal func() bool
	ask           func(prompt string) (string, bool)
	holder        func(root string) (lease.CurrentHolderView, error)
	done          func(inv *intentInvocation, id, by, reason string, proof humanauthority.Proof, force bool) intentResult
	read          func(inv *intentInvocation, patch, brief string) intentResult
	recover       func(scope processScope) string
	fence         func(installation stateroot.Installation) error
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
	if o.git == nil {
		o.git = goalBranchGit
	}
	if o.stdin == nil {
		o.stdin = os.Stdin
	}
	if o.stdinTerminal == nil {
		stdin := o.stdin
		o.stdinTerminal = func() bool {
			file, ok := stdin.(*os.File)
			if !ok {
				return false
			}
			info, err := file.Stat()
			return err == nil && info.Mode()&os.ModeCharDevice != 0
		}
	}
	if o.holder == nil {
		o.holder = helmHolder
	}
	if o.done == nil {
		o.done = helmReturnDone
	}
	if o.read == nil {
		o.read = helmReturnRead
	}
	if o.recover == nil {
		o.recover = helmRecover
	}
	if o.fence == nil {
		o.fence = ledgerfence.Ensure
	}
	return o
}

func helmIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "helm", action: "take", primary: true, audience: "human", summary: "take this seat out of the machinery's hands until you return it",
		usage: []string{"metasystem helm take --reason TEXT [--name NAME] [--all]"},
		details: []string{
			"I take the helm of this seat (this checkout and its linked worktrees). From now until metasystem helm return, MetaSystem decides nothing for this seat: " +
				strings.Join(helmYielding, ", ") + ". What runs keeps running: the helm stops or contains no job. Records are still written and tests still report what they find.",
			"Run it at a terminal; it works when the ledger, the enrollment or supervision is broken. It refuses an agent anywhere in its chain; it cannot see who created the terminal (metasystem status names the session leader). Taking it again is fine.",
			"A terminal that is not enrolled is enrolled in the same act, as metasystem system enroll --name NAME would, so a person's acts there are admitted too; an enrolled one is left as it is. The fleet cutoff of a first enrollment is published by metasystem system enroll.",
		},
		flags: []intentFlag{{name: "all", usage: "take every primary checkout on this computer, including its lane and coordinator"}, {name: "reason", value: "TEXT", usage: "why you take the helm"},
			{name: "name", aliases: []string{"by"}, value: "NAME", advanced: true, usage: "your name, at the helm and on this terminal's enrollment (default: the enrolled name, else your account)"}},
		examples: []string{`metasystem helm take --reason "coordinating the U5 builders by hand tonight"`},
		run:      runIntentHelmTake,
	}, {
		object: "helm", action: "return", audience: "human", summary: "give the seat back to the machinery",
		usage: []string{"metasystem helm return [--all]"},
		details: []string{"I give the seat back to the machinery. It removes the signature first, then shows what changed while at the helm and what the machinery sees now, " +
			"then asks whether to conclude the goal and whether to request a read; without a terminal it prints the commands. " +
			"It re-enrolls the ledger hook and recovers supervision that is down, as metasystem system start --if-down does; it stops, claims or cleans nothing.",
			"Run return at the person's enrolled terminal. An agent cannot return it, including an unreadable signature. Returning a helm nobody holds is fine.",
			"--all applies on this computer, sequentially, including its landing lane and coordinator; each checkout is reported. Configuration changes made while held apply on return."},
		flags:    []intentFlag{{name: "all", usage: "return every helm on this computer"}},
		examples: []string{"metasystem helm return"},
		run:      runIntentHelmReturn,
	}, {
		object: "helm", action: "status", audience: "both", summary: "who holds this seat's helm, since when and why",
		usage:    []string{"metasystem helm status [--all]"},
		details:  []string{"Reads the helm as status's banner shows it; it changes nothing."},
		flags:    []intentFlag{{name: "all", usage: "show every helm on this computer"}},
		examples: []string{"metasystem helm status"},
		run:      runIntentHelmStatus,
	}}
}

// runIntentHelmStatus reads the helm and changes nothing: the holder, since
// when and why, as status's banner shows it, or the machinery at the helm.
func runIntentHelmStatus(inv *intentInvocation) int {
	if inv.input.switched("all") {
		return inv.runHelmAll("status")
	}
	path := inv.helmPath()
	if _, err := helm.Locate(path); err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was read",
			Decision: "run metasystem helm status inside the seat's checkout"})
	}
	reading, active := inv.readHelm(path)
	if !active {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the machinery is at the helm"})
	}
	result := intentResult{Outcome: intentUnchanged, Summary: reading.lines[0], text: reading.lines[1:],
		Data: map[string]any{"helm": reading.lines, "policies": reading.policies}}
	return inv.render(result)
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
// from the enrollment, and atomically signs its policy observations.
func runIntentHelmTake(inv *intentInvocation) int {
	reason := strings.TrimSpace(inv.input.text("reason"))
	if reason == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "helm take needs a reason; nothing was done",
			next: append(inv.typedArgv(), "--reason", "TEXT"), nextReason: "TEXT says why you take the helm"})
	}
	if inv.input.switched("all") {
		return inv.runHelmAll("take")
	}
	owners, path := inv.owners.helm.withDefaults(), inv.helmPath()
	seat, err := helm.Locate(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was done",
			Decision: "run metasystem helm take inside the seat's checkout"})
	}
	actor, problem := inv.helmActorAt("take")
	if problem != nil {
		return inv.render(*problem)
	}
	record := actor.record
	record.Checkout, record.Reason = seat.Checkout, reason
	now := actor.proof.CheckedAt
	enrollmentLine, enrolledNow := actor.enrollmentLine, actor.enrolledNow
	var standing helm.State
	var repeated bool
	signatureWritten := false
	entry := helm.Entry{At: now, Action: "take", By: record.By, Reason: reason, Leader: record.Leader}
	targets := []intentTarget{{Kind: "seat", ID: seat.Checkout}}
	err = config.WithPolicyLock(seat.Checkout, func() error {
		fresh, err := helm.Locate(path)
		if err != nil {
			return err
		}
		if fresh.CommonDir != seat.CommonDir {
			return fmt.Errorf("the helm target changed; repeat the take")
		}
		standing = helm.Active(path)
		if standing.Active && standing.Malformed == "" && (standing.By == record.By || !inv.input.has("name")) {
			// Repeats retain the original identity and policy observations,
			// but describe the checkout and terminal of this take.
			record = standing.Record
			record.Checkout, record.Reason = seat.Checkout, reason
			record.Leader, record.LeaderRef = actor.record.Leader, actor.record.LeaderRef
			record.Enrollment, record.EnrolledAs = actor.record.Enrollment, actor.record.EnrolledAs
			if standing.Reason == reason && standing.Enrollment == record.Enrollment && !enrolledNow {
				repeated = true
			}
		} else {
			if standing.Active {
				entry.Replaced = standing.By
				if standing.Malformed != "" {
					entry.Diagnostic, _ = os.ReadFile(seat.Signature)
				}
			}
			record.Policies = inv.helmPolicySnapshot(seat.Checkout, record.By, now)
		}
		_, err = helm.Write(path, record)
		if err != nil {
			return err
		}
		signatureWritten = true
		_, registered, err := inv.helmLane(seat.Checkout)
		if err != nil || registered.Install == "" {
			return err
		}
		_, _, err = plain.SetDrain(registered.Install, plain.Drain{By: record.By, At: now.UTC().Format(time.RFC3339), Reason: reason,
			Source: plain.DrainSource{Kind: "helm", Checkout: record.Checkout, By: record.By, At: record.At}})
		return err
	})
	if err != nil {
		if signatureWritten {
			// The signature stands, so the take is logged now: its retry is a
			// repeat and would not log it (nor the holder it replaced).
			details := []string{err.Error()}
			if !repeated {
				if logErr := helm.Log(path, entry); logErr != nil {
					details = append(details, "helm.log was not appended: "+logErr.Error())
				}
			}
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the helm is held, but the lane drain failed; the take is partial",
				next: inv.typedArgv(), nextReason: "retry the same take at the person's terminal after repairing the reported drain or registration failure", Details: details, Data: map[string]any{"helm": record}})
		}
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the helm signature write is incomplete; the take is not confirmed",
			next: inv.typedArgv(), nextReason: "retry at the person's enrolled terminal after correcting the write failure; any existing helm remains held", Details: []string{err.Error()}})
	}
	if repeated {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Summary: "applied: " + helmLine(record, standing.Since, owners.zone),
			text: withLine(helmTerminalLines(record), enrollmentLine), Data: map[string]any{"helm": record}, view: helmTakeView(record, standing.Since, true, false, enrollmentLine, nil)})
	}
	lines := withLine(helmTerminalLines(record), enrollmentLine)
	if enrolledNow {
		lines = []string{enrollmentLine}
	}
	var notes []string
	if err := helm.Log(path, entry); err != nil {
		lines = append(lines, "helm.log was not appended: "+err.Error())
		notes = append(notes, "the helm log was not written ("+err.Error()+")")
	}
	if entry.Replaced != "" {
		lines = append(lines, "replaces "+entry.Replaced+" at the helm; both names are in "+seat.Log)
		notes = append(notes, "it was "+entry.Replaced+"'s; both names are in the helm log")
	}
	since, _ := time.Parse(time.RFC3339, record.At)
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Summary: helmLine(record, since, owners.zone), text: lines, Data: map[string]any{"helm": record},
		view: helmTakeView(record, since, false, enrolledNow, enrollmentLine, notes)})
}

// helmTakeView is a take's page: who has the helm since when and why, the
// terminal it was taken at, and the command that gives it back. The
// session leader's identity is --verbose's.
func helmTakeView(record helm.Record, since time.Time, already, enrolledNow bool, enrollment string, notes []string) func(*textui.Page) {
	return func(page *textui.Page) {
		env := page.Env()
		headline := fmt.Sprintf("%s has the helm %s: %s", record.By, env.Since(since), record.Reason)
		if already {
			headline = fmt.Sprintf("%s already has the helm %s: %s", record.By, env.Since(since), record.Reason)
		}
		page.Done(headline)
		terminal := "a terminal whose enrollment could not be read"
		switch record.Enrollment {
		case "proven":
			terminal = "the enrolled one"
			if record.EnrolledAs != "" {
				terminal += ", as " + record.EnrolledAs
			}
			if enrolledNow {
				terminal = "enrolled now, as " + record.EnrolledAs + ", so a person's acts here are admitted"
			}
		case "other-terminal":
			terminal = "not the enrolled one, so a person's acts there are refused"
		}
		facts := []textui.KV{{Key: "terminal", Value: []textui.Span{textui.Plain(terminal)}}}
		if enrollment != "" && record.Enrollment != "proven" {
			facts = append(facts, textui.KV{Key: "enrollment", Value: []textui.Span{textui.Plain(enrollment)}})
		}
		for _, note := range notes {
			facts = append(facts, textui.KV{Key: "note", Value: []textui.Span{textui.Plain(note)}})
		}
		if page.Verbose() {
			pid, _, _ := strings.Cut(record.LeaderRef, "@")
			facts = append(facts, textui.KV{Key: "leader", Value: []textui.Span{textui.Plain("session leader " + cmpOr(record.Leader, "unknown") + ", pid " + pid)}},
				textui.KV{Key: "seat", Value: []textui.Span{textui.Plain(env.Path(record.Checkout))}})
		}
		page.Facts(facts...)
		page.Hint(textui.Hint{Argv: []string{"metasystem", "helm", "return"}, Reason: "run at the person's enrolled terminal to give the seat back to the machinery"})
	}
}

// helmLeaderName is the session leader's executable name, or unknown.
func helmLeaderName(reader humanauthority.Reader, pid int64) string {
	if leader, err := reader.Read(pid); err == nil {
		return filepath.Base(leader.Executable)
	}
	return "unknown"
}

// helmEnrollmentReading is the helm holder's terminal against the enrolled
// one: enrolled, not enrolled, or unknown with the reason; line says so and,
// when it is not the enrolled one, names the command that enrolls it.
type helmEnrollmentReading struct {
	kind    string // "enrolled", "other" or "unknown"
	leader  string
	problem string
	line    string
}

func (inv *intentInvocation) helmEnrollment(path string, record helm.Record) helmEnrollmentReading {
	enroll := shellCommand([]string{"metasystem", "system", "enroll", "--name", record.By})
	leader := fmt.Sprintf("session leader %s (%s)", record.Leader, record.LeaderRef)
	layout, err := inv.owners.resolver.ResolveLayout(path)
	var root stateroot.State
	if err == nil {
		root, err = inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
	}
	var enrollment humanauthority.Enrollment
	if err == nil {
		enrollment, err = humanauthority.ReadEnrollment(root.Path())
	}
	switch {
	case err != nil:
		return helmEnrollmentReading{kind: "unknown", leader: leader, problem: err.Error(),
			line: "the helm holder's terminal is not known to be enrolled (" + err.Error() + "); a person's acts there need it: run there " + enroll}
	case record.LeaderRef != "" && fmt.Sprintf("%d@%d", enrollment.SessionLeader.PID, enrollment.SessionLeader.PIDStartedAt) == record.LeaderRef:
		return helmEnrollmentReading{kind: "enrolled", leader: leader,
			line: "the helm holder's terminal is enrolled as " + enrollment.Human + " (" + leader + "): a person's acts there are admitted"}
	}
	return helmEnrollmentReading{kind: "other", leader: leader,
		line: "the helm holder's terminal is not enrolled (" + leader + "; the enrolled terminal is another): a person's acts there are refused until you run there " + enroll}
}

// withLine appends line to lines when there is one.
func withLine(lines []string, line string) []string {
	if line == "" {
		return lines
	}
	return append(lines, line)
}

func helmLine(record helm.Record, since time.Time, zone *time.Location) string {
	local := since.In(zone)
	return fmt.Sprintf("HUMAN AT THE HELM since %s by %s: %s — %s", local.Format("15:04 MST (2006-01-02)"), record.By, record.Reason, humanauthority.PersonActRemedy(helmReturnCommand(record.Checkout)))
}

// helmReturnCommand names the checkout whose helm it returns, so the line is
// right wherever it is printed (helm status --all prints other checkouts).
func helmReturnCommand(checkout string) string {
	if checkout == "" {
		return "metasystem helm return"
	}
	return "metasystem helm return --repo " + checkout
}

func helmTerminalLines(record helm.Record) []string {
	switch record.Enrollment {
	case "proven":
		return []string{fmt.Sprintf("taken at the enrolled terminal; session leader %s (%s)", record.Leader, record.LeaderRef)}
	case "other-terminal":
		return []string{fmt.Sprintf("taken at a terminal that is not enrolled; session leader %s (%s)", record.Leader, record.LeaderRef)}
	}
	return []string{fmt.Sprintf("taken at a terminal; the enrollment was unreadable; session leader %s (%s)", record.Leader, record.LeaderRef)}
}

// runIntentHelmReturn proves the person before removing the signature; the
// best-effort report afterwards can never undo or block the return.
func runIntentHelmReturn(inv *intentInvocation) int {
	if inv.input.switched("all") {
		return inv.runHelmAll("return")
	}
	if _, refusal := inv.helmActorAt("return"); refusal != nil {
		return inv.render(*refusal)
	}
	path, owners := inv.helmPath(), inv.owners.helm.withDefaults()
	seat, err := helm.Locate(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was done", next: inv.typedArgv(), nextReason: "run this at the person's enrolled terminal inside a checkout"})
	}
	var removal helm.Removal
	var drainLines []string
	err = config.WithPolicyLock(seat.Checkout, func() error {
		fresh, err := helm.Locate(path)
		if err != nil {
			return err
		}
		if fresh.CommonDir != seat.CommonDir {
			return fmt.Errorf("the helm target changed; repeat the return")
		}
		removal, err = helm.Remove(path)
		if err == nil {
			_, registered, readErr := inv.helmLane(seat.Checkout)
			if readErr != nil {
				drainLines = append(drainLines, "the lane drain could not be checked: "+readErr.Error()+"; a person runs metasystem landing start")
			} else if registered.Install != "" {
				record, problem := helm.Decode(removal.Raw)
				source := plain.DrainSource{}
				if removal.Present && removal.ReadErr == nil && problem == "" {
					source = plain.DrainSource{Kind: "helm", Checkout: record.Checkout, By: record.By, At: record.At}
				}
				cleared, clearErr := plain.ClearHelmDrain(registered.Install, source)
				if cleared {
					drainLines = append(drainLines, "removed the helm's drain; admission open")
				} else if drain, drainErr := plain.ReadDrain(registered.Install); drain != nil || drainErr != nil || clearErr != nil {
					line := "the lane drain remains; a person runs metasystem landing start"
					if clearErr != nil {
						line += ": " + clearErr.Error()
					}
					drainLines = append(drainLines, line)
				}
			}
		}
		return err
	})
	if err != nil && removal.Seat.CommonDir == "" {
		removal.Seat = seat
	}

	switch {
	case removal.Seat.CommonDir == "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was done",
			Decision: "run metasystem helm return inside the seat's checkout"})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the helm signature can't be removed, so the helm is still taken",
			Decision: "remove " + removal.Seat.Signature + " by hand (chmod u+w " + removal.Seat.Dir + " if needed), then run metasystem helm return again",
			Details:  []string{err.Error()}})
	case !removal.Present:
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: "the machinery is at the helm; nothing to return", text: drainLines, Details: drainLines})
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
	lines = append(lines, drainLines...)
	if err := helm.Log(path, helm.Entry{At: owners.now(), Action: "return", By: record.By, Reason: problem, Diagnostic: func() []byte {
		if problem != "" {
			return removal.Raw
		}
		return nil
	}()}); err != nil {
		lines = append(lines, "helm.log was not appended: "+err.Error())
	}
	lines = append(lines, inv.helmReport(removal.Seat, since)...)
	lines, printed := inv.helmCatchUp(removal.Seat, record, since, problem == "", lines)
	lines = append(lines, "the machinery is at the helm again")
	summary := ""
	if !printed {
		summary, lines = lines[0], lines[1:]
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: []intentTarget{{Kind: "seat", ID: removal.Seat.Checkout}},
		Summary: summary, text: lines, Details: drainLines, Data: map[string]any{"returned": record}})
}

// helmReport is the best-effort account shared by status and return: yields
// since the take and running work. Each source that fails says so in its
// place.
func (inv *intentInvocation) helmReport(seat helm.Seat, since time.Time) []string {
	lines := []string{fmt.Sprintf("acts the helm let through since the take: %s", helmYieldCount(seat, since))}
	if home, registered, err := inv.helmLane(seat.Checkout); err != nil {
		lines = append(lines, "lane admission unknown: "+err.Error())
	} else if registered.Install != "" {
		root := registered.Root
		status := plain.ReadStatus(home, registered, lane.View{Root: &root}, inv.landing().plainProve)
		lines = append(lines, "lane admission: "+status.Admission)
		lines = append(lines, status.Problems...)
	}
	layout, err := inv.owners.resolver.ResolveLayout(seat.Checkout)
	if err != nil {
		return append(lines, "running work: unavailable: "+err.Error())
	}
	running := 0
	paths, _ := filepath.Glob(layout.InstallationRoot.Path("artifacts", "agents", "jobs", "*.json"))
	for _, path := range paths {
		if object, readErr := dispatchcore.ReadRecordObject(path); readErr == nil && !dispatchcore.TerminalStatus(fmt.Sprint(object["status"])) {
			running++
		}
	}
	return append(lines, fmt.Sprintf("running dispatch jobs: %d (metasystem work status lists them; the helm stops none)", running))
}

// helmLane resolves the lane's canonical installation, including linked worktrees.
// An unreadable coordinator declaration does not hide an independently known lane.
func (inv *intentInvocation) helmLane(checkout string) (home string, record lane.Record, err error) {
	registry, registryErr := inv.policyReaders().Registry(checkout)
	var registryProblem *config.PolicyReadError
	if registry.Lane == "" && errors.As(registryErr, &registryProblem) && registryProblem.Source == "lane" {
		return "", lane.Record{}, fmt.Errorf("the lane registration could not be checked: %w", registryErr)
	}
	seat, locateErr := helm.Locate(registry.Lane)
	if registry.Lane == "" {
		return "", lane.Record{}, nil
	}
	if locateErr != nil {
		return "", lane.Record{}, locateErr
	}
	if seat.Checkout != checkout {
		return "", lane.Record{}, nil
	}
	home, err = inv.landing().home()
	if err != nil {
		return
	}
	var present bool
	record, present, err = lane.Read(home)
	if err == nil && (!present || record.Root != registry.Lane) {
		err = fmt.Errorf("the landing lane registration changed; repeat the helm act")
	}
	return
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

// helmReading is what status reads of the helm, once: the signature, the
// holder's terminal and the account since the take. Its lines are what
// --json carries; its attention is the text banner.
type helmReading struct {
	state      helm.State
	enrollment helmEnrollmentReading
	report     []string
	policies   []config.PolicyResolution
	lines      []string
}

func (inv *intentInvocation) readHelm(path string) (helmReading, bool) {
	state := helm.Active(path)
	if !state.Active {
		return helmReading{}, false
	}
	if state.Malformed != "" {
		seat, _ := helm.Locate(path)
		report := inv.helmReport(seat, time.Time{})
		lines := []string{"HUMAN AT THE HELM (the signature is unreadable: " + state.Malformed + ") — " + humanauthority.PersonActRemedy(helmReturnCommand(path))}
		return helmReading{state: state, report: report, lines: append(lines, report...)}, true
	}
	zone := inv.owners.helm.withDefaults().zone
	seat, _ := helm.Locate(path)
	reading := helmReading{state: state, enrollment: inv.helmEnrollment(path, state.Record), report: inv.helmReport(seat, state.Since)}
	reading.lines = append([]string{helmLine(state.Record, state.Since, zone), reading.enrollment.line}, reading.report...)
	reading.policies, reading.lines = inv.helmPolicyStatus(seat.Checkout, state, reading.lines)
	return reading, true
}

// helmStatusLines are the first lines of status while the seat is at the helm.
func (inv *intentInvocation) helmStatusLines(path string) []string {
	reading, _ := inv.readHelm(path)
	return reading.lines
}

// attention is the helm's banner (P12): who holds it since when and why,
// with the command that gives it back; the holder's terminal when it is not
// the enrolled one. --verbose adds the account since the take.
func (r helmReading) attention(env textui.Env) []textui.Attention {
	back := textui.Hint{Argv: []string{"metasystem", "helm", "return"}, Reason: "run at the person's enrolled terminal to give the seat back to the machinery"}
	if r.state.Malformed != "" {
		return []textui.Attention{{State: textui.Alert, Text: "the helm is taken, but its signature is unreadable: " + r.state.Malformed, Hint: back}}
	}
	by := r.state.By
	out := []textui.Attention{{State: textui.Alert, Text: fmt.Sprintf("%s has the helm %s: %s", by, env.Since(r.state.Since), r.state.Reason), Hint: back, Detail: r.report}}
	enroll := textui.Hint{Argv: []string{"metasystem", "system", "enroll", "--name", by}, Reason: "run it at that terminal"}
	switch r.enrollment.kind {
	case "other":
		pid, _, _ := strings.Cut(r.state.LeaderRef, "@")
		out = append(out, textui.Attention{State: textui.Alert, Text: by + "'s terminal is not enrolled, so a person's acts there are refused", Hint: enroll,
			Detail: []string{"that terminal: session leader " + r.state.Leader + " (pid " + pid + "); the enrolled one is another"}})
	case "unknown":
		out = append(out, textui.Attention{State: textui.Unknown, Text: by + "'s terminal is not known to be enrolled: " + r.enrollment.problem, Hint: enroll})
	}
	return out
}

// withHelm puts the helm and a live general grant first in a status
// result: as the text banner, and for --json as the Summary and data it
// always carried. It leaves the result untouched when neither holds.
func (inv *intentInvocation) withHelm(result intentResult, path string) intentResult {
	reading, active := inv.readHelm(path)
	grant, granted := inv.liveGeneralGrant(path)
	var lines []string
	if active {
		lines = reading.lines
	}
	if granted {
		attorney := attorneyLine(grant, inv.owners.helm.withDefaults().zone)
		lines = append(lines, attorney)
		if data, ok := result.Data.(map[string]any); ok {
			data["powerOfAttorney"] = attorney
		}
	}
	if lines == nil {
		return result
	}
	result.attention = func(env textui.Env) []textui.Attention {
		var items []textui.Attention
		if active {
			items = reading.attention(env)
		}
		if granted {
			items = append(items, grantAttention(grant, env))
		}
		return items
	}
	headline := result.Summary
	result.headline = &headline
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
	switch outcome, _ := humanauthority.OutcomeOf(err); outcome {
	case humanauthority.OutcomeAgent:
		return "an agent started this shell"
	case humanauthority.OutcomeTerminalMissing:
		return "this shell isn't attached to a terminal"
	case humanauthority.OutcomeNotEnrolled:
		return "this terminal is not the enrolled one"
	}
	return err.Error()
}

type helmActor struct {
	proof          humanauthority.Proof
	root           string
	record         helm.Record
	enrollmentLine string
	enrolledNow    bool
}

// helmActorAt proves the terminal independently of helm and attorney fallback.
// A host-wide act carries this observation; targets never mint new authority.
func (inv *intentInvocation) helmActorAt(act string) (*helmActor, *intentResult) {
	owners := inv.owners.helm.withDefaults()
	if owners.actor != nil {
		if owners.actor.proof.Helm == nil && owners.actor.proof.TerminalValidFor(owners.actor.root) {
			return owners.actor, nil
		}
		return nil, &intentResult{Outcome: intentRefused, code: 3, Summary: "the person at the terminal could not be proved; nothing was done", next: inv.typedArgv(), nextReason: "run this at the person's enrolled terminal"}
	}
	proofPath := inv.cwd
	if _, err := helm.Locate(proofPath); err != nil {
		proofPath = inv.helmPath()
	}
	seat, err := helm.Locate(proofPath)
	if err != nil {
		return nil, &intentResult{Outcome: intentRefused, code: 2, Summary: err.Error() + "; nothing was done", next: inv.typedArgv(), nextReason: "run this at the person's terminal inside a checkout"}
	}
	root := seat.Checkout
	if layout, err := inv.owners.resolver.ResolveLayout(proofPath); err == nil {
		root = checkoutAuthorityRoot(layout)
	} else if installation := filepath.Join(seat.Checkout, "metasystem"); config.TemplateMode(installation) {
		// The direct return remains usable when Git or the ledger is unavailable.
		root = installation
	}
	now, pid := owners.now().UTC(), owners.pid()
	proof, err := humanauthority.ProveTerminal(root, pid, owners.reader, now)
	if err == nil && !proof.TerminalValidFor(root) {
		err = fmt.Errorf("terminal human authority was not proven")
	}
	if err != nil {
		return nil, &intentResult{Outcome: intentRefused, code: 3,
			Summary: actorProofReason(err) + func() string {
				if act == "take" {
					return ", so the helm wasn't taken"
				}
				return ", so the helm wasn't returned"
			}(),
			next: inv.typedArgv(), nextReason: func() string {
				if act == "return" {
					return "at the person's enrolled terminal"
				}
				return "in a terminal you opened yourself"
			}(),
			Details: []string{"refused because: " + err.Error()}}
	}
	if act == "return" {
		return &helmActor{proof: proof, root: root}, nil
	}
	record := helm.Record{By: strings.TrimSpace(inv.input.text("name")), At: now.Format(time.RFC3339), Reason: inv.input.text("reason"), Enrollment: "unreadable"}
	enrollment, readErr := humanauthority.ReadEnrollment(root)
	if readErr == nil {
		record.Enrollment, record.EnrolledAs = "other-terminal", enrollment.Human
		// A helm proof is never the take's grade: only the real walk is.
		enrolledTerminalInChain := false
		for _, node := range proof.Nodes {
			if node.Ref == enrollment.TerminalRef {
				enrolledTerminalInChain = true
			}
			if node.Ref == proof.TerminalRef {
				break
			}
		}
		if enrolledTerminalInChain && enrollment.SessionLeader == proof.TerminalRef && enrollment.TerminalID == proof.ObservedTerminalID() {
			record.Enrollment = "proven"
			if record.By == "" {
				record.By = enrollment.Human
			}
		}
	}
	if record.By == "" {
		record.By = enrollment.Human
	}
	if record.By == "" {
		record.By = owners.account()
	}
	// One notion of a person: the terminal the helm is taken at is enrolled
	// in the same act, under the same terminal proof, so the helm holder's
	// acts there are a person's acts. An enrolled terminal is left as it is;
	// a failed enrollment leaves the helm taken and says what to run.
	enrollmentLine, enrolledNow := "", false
	if record.Enrollment != "proven" && (readErr == nil || os.IsNotExist(readErr)) {
		if enrolled, enrollErr := humanauthority.Enroll(root, pid, owners.reader, record.By, now); enrollErr != nil {
			enrollmentLine = "this terminal is not enrolled (" + humanauthority.PlainReason(enrollErr) + "): " + humanauthority.PersonActRemedy("")
		} else {
			record.Enrollment, record.EnrolledAs, enrolledNow = "proven", enrolled.Human, !enrolled.Repeat
			if enrolledNow {
				enrollmentLine = fmt.Sprintf("this terminal is now enrolled as %s (session leader %s (%d@%d)): a person's acts here are admitted",
					enrolled.Human, helmLeaderName(owners.reader, proof.TerminalRef.PID), proof.TerminalRef.PID, proof.TerminalRef.PIDStartedAt)
			}
		}
	}
	if record.Machine, err = owners.machine(root); err != nil {
		record.Machine, _ = os.Hostname()
	}
	record.LeaderRef = fmt.Sprintf("%d@%d", proof.TerminalRef.PID, proof.TerminalRef.PIDStartedAt)
	if leader := helmLeaderName(owners.reader, proof.TerminalRef.PID); leader != "unknown" {
		record.Leader = leader
	}
	return &helmActor{proof: proof, root: root, record: record, enrollmentLine: enrollmentLine, enrolledNow: enrolledNow}, nil
}
