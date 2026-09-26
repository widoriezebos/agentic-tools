package main

import (
	"crypto/sha256"
	"time"

	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// Design authoring asks the design-author lane for a draft of one project
// design document, from and into the invoking checkout. It needs an approved
// goal with a review budget; it takes no build claim and makes no goal
// worktree. The launch owner retains each attempt; the project owner
// publishes a finished proposal only over the exact bytes it was made from.

func designCommand() intentCommand {
	return intentCommand{
		name: "design", group: "work", audience: "agent", summary: "have a design author write a goal's design as a draft",
		usage: []string{"metasystem design G --brief FILE [--out FILE] [--after N]"},
		details: []string{
			"The design author works in this checkout and writes a staged draft; the goal's design document is updated only",
			"when it still holds the bytes the request was made against. A document someone edited meanwhile is left as it is",
			"and the proposal is kept: show design --goal G --attempt N shows it.",
			"Without --out: the goal's one draft design, or a new <goal>.md in the project's design home. --out names a file",
			"inside a design home. An accepted design is never rewritten; ask for a new draft file instead.",
			"The same request again reports the same attempt; --after N asks for one new attempt after attempt N.",
			"Needs an approved goal with a review allowance. It does not claim the goal for building and does not touch your checkout's work.",
			"The finished draft is reviewed with: metasystem review design FILE",
		},
		flags: []intentFlag{
			intentBriefFlag,
			{name: "out", value: "FILE", usage: "the design document (default: the goal's draft design, or a new one)"},
			{name: "after", value: "N", usage: "ask for one new attempt after attempt N"},
		},
		maxArgs:  1,
		examples: []string{"metasystem design verbs-match-intent --brief design-request.md", "metasystem design verbs-match-intent --brief more.md --after 1"},
		run:      runIntentDesign,
	}
}

func (inv *intentInvocation) designManager() *launch.Manager {
	return inv.work().units(inv.layout).Manager
}

// designDestination resolves the document a design request, show or stop
// names: --out, the goal's one draft design, or a new <goal>.md in the first
// design home of the invoking checkout.
func (inv *intentInvocation) designDestination(id string, creating bool) (string, string, *intentResult) {
	path, recordID, problem := inv.designDestinationPath(id, creating)
	if problem != nil {
		return path, recordID, problem
	}
	return canonicalDocument(path), recordID, nil
}

// canonicalDocument is a document's path with its directory's symbolic
// links resolved, so one file has one lock and one retained entry.
func canonicalDocument(path string) string {
	directory := filepath.Dir(path)
	for probe := directory; ; probe = filepath.Dir(probe) {
		if resolved, err := filepath.EvalSymlinks(probe); err == nil {
			rest, _ := filepath.Rel(probe, directory)
			return filepath.Join(resolved, rest, filepath.Base(path))
		}
		if probe == filepath.Dir(probe) {
			return path
		}
	}
}

func (inv *intentInvocation) designDestinationPath(id string, creating bool) (string, string, *intentResult) {
	canonical := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return resolved
		}
		return path
	}
	checkout := canonical(inv.layout.GitRoot)
	roots := project.Roots{Checkout: checkout, Installation: canonical(inv.layout.InstallationRoot), StateRoot: canonical(inv.stateRoot)}
	var home string
	for _, candidate := range project.Homes(roots) {
		if candidate.Kind == project.KindDesign && candidate.Glob == "" && designWithin(candidate.Path, checkout) {
			home = candidate.Path
			break
		}
	}
	if home == "" {
		for _, candidate := range project.Homes(roots) {
			if candidate.Kind == project.KindDesign && candidate.Glob == "" {
				home = candidate.Path
				break
			}
		}
	}
	if out := inv.input.text("out"); out != "" {
		path := canonicalDocument(inv.callerPath(out))
		inside := false
		for _, candidate := range project.Homes(roots) {
			if candidate.Kind == project.KindDesign && candidate.Glob == "" && designWithin(path, candidate.Path) && path != candidate.Path {
				inside = true
			}
		}
		if !inside || filepath.Ext(path) != ".md" {
			return "", "", &intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--out %s is not a Markdown file inside a design home (%s); nothing was done", shellCommand([]string{out}), home)}
		}
		return path, "", nil
	}
	read, err := project.Read(roots)
	if err != nil {
		return "", "", &intentResult{Outcome: intentFailed, code: 1, Summary: "the project's design records cannot be read: " + err.Error()}
	}
	var drafts, others []project.Record
	seen := map[string]bool{}
	for _, record := range read.List(project.KindDesign, project.ListOptions{Goal: id}) {
		// Two homes that are spellings of one directory list a record twice.
		if seen[record.ID] {
			continue
		}
		seen[record.ID] = true
		if record.Status == "draft" {
			drafts = append(drafts, record)
		} else {
			others = append(others, record)
		}
	}
	switch {
	case len(drafts) == 1:
		return filepath.Join(checkout, drafts[0].Path), drafts[0].ID, nil
	case len(drafts) > 1:
		lines := []string{}
		for _, record := range drafts {
			lines = append(lines, "  "+shellCommand(append(inv.sameCommand(), "--out", record.Path)))
		}
		return "", "", &intentResult{Outcome: intentRefused, code: 2, text: lines,
			Summary: fmt.Sprintf("goal %s has %d draft designs; name one with --out; nothing was done", id, len(drafts))}
	}
	if !creating && len(others) == 0 {
		// A first attempt still writing has no record yet: its retained
		// request at the new document's path names it.
		if manager := inv.designManager(); manager != nil {
			pending := canonicalDocument(filepath.Join(home, id+".md"))
			if _, attempts, err := manager.DesignDocument(pending); err == nil && len(attempts) > 0 {
				return pending, "", nil
			}
		}
		return "", "", &intentResult{Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("goal %s has no design; nothing was done", id),
			next: inv.publicArgv("design", id, "--brief", "FILE"), nextReason: "ask a design author for one"}
	}
	path := filepath.Join(home, id+".md")
	if _, err := os.Stat(path); err == nil {
		if data, readErr := os.ReadFile(path); readErr == nil {
			if record, _, ok := project.ParseRecord(filepath.Base(path), string(data)); ok && record.Kind == project.KindDesign && record.Status == "draft" && slicesContains(record.Goals, id) {
				return path, record.ID, nil
			}
		}
		return "", "", &intentResult{Outcome: intentRefused, code: 2,
			Summary:  fmt.Sprintf("%s already exists and is not this goal's draft design; nothing was done", path),
			Decision: "name a new draft file with --out FILE inside " + home}
	}
	return path, "", nil
}

func slicesContains(values []string, value string) bool {
	for _, one := range values {
		if one == value {
			return true
		}
	}
	return false
}

func runIntentDesign(inv *intentInvocation) int {
	if len(inv.input.args) != 1 || !inv.input.has("brief") {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "design needs the goal and --brief FILE; nothing was done", Decision: "metasystem design G --brief FILE"})
	}
	id := inv.input.args[0]
	after := 0
	if inv.input.has("after") {
		value, err := strconv.Atoi(inv.input.text("after"))
		if err != nil || value < 1 {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--after is an attempt number such as 1; nothing was done"})
		}
		after = value
	}
	brief, err := os.ReadFile(inv.callerPath(inv.input.text("brief")))
	if err != nil || len(strings.TrimSpace(string(brief))) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "the design brief is missing or empty; nothing was done"})
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	targets := inv.targets(id)
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	switch {
	case file == nil:
		return unknownGoal(inv, id)
	case where != "live":
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: fmt.Sprintf("goal %s is %s; nothing was done", id, where)})
	case file.State == goal.StateQueued || file.State == goal.StateParked || file.Budget == nil || file.Budget.ReviewRoundLimit <= 0:
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary:  fmt.Sprintf("goal %s is not approved with a review allowance; nothing was done", id),
			Decision: "a person approves the goal with its box: metasystem approve " + id})
	}
	destination, recordID, problem := inv.designDestination(id, true)
	if problem != nil {
		problem.Targets = targets
		return inv.render(*problem)
	}
	manager := inv.designManager()
	if manager == nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the launch owner is unavailable"})
	}
	if retainedID, _, err := manager.DesignDocument(destination); err == nil && retainedID != "" {
		recordID = retainedID
	}
	if recordID == "" {
		if data, readErr := os.ReadFile(destination); readErr == nil {
			if record, _, ok := project.ParseRecord(filepath.Base(destination), string(data)); ok {
				recordID = record.ID
			}
		}
	}
	if recordID == "" {
		if recordID, err = project.NewID(); err != nil {
			return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
		}
	}
	if data, readErr := os.ReadFile(destination); readErr == nil {
		if record, _, ok := project.ParseRecord(filepath.Base(destination), string(data)); ok && record.Status != "draft" {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
				Summary:  fmt.Sprintf("%s is %s; an accepted or done design is not rewritten; nothing was done", destination, record.Status),
				Decision: "ask for a new draft file: metasystem design " + id + " --brief FILE --out NEW-FILE"})
		}
	}
	header := fmt.Sprintf("# Design for %s\n\n- Kind: design\n- Id: %s\n- Status: draft\n- Goals: %s\n", id, recordID, id)
	contract := fmt.Sprintf(`# Design author contract

You write one design record as a draft, into the one output file named under "Your output" below (never
into the project's own document or any other project record). Its head is exactly:

- Kind: design
- Id: %s
- Status: draft
- Goals: %s

Where a decision belongs to a person, write it as an open question in the draft and stop; never decide it.
You never approve or accept your own design.

`, recordID, id)
	result, err := manager.RequestDesign(launch.DesignRequest{Goal: id, RecordID: recordID, Destination: destination,
		WorkingDirectory: inv.layout.GitRoot, Brief: brief, Contract: []byte(contract), Header: []byte(header), After: after})
	return inv.render(inv.designOutcome(id, destination, result, err))
}

// designOutcome publishes a finished attempt's proposal through the project
// owner, once, and reports the attempt in public terms.
func (inv *intentInvocation) designOutcome(id, destination string, result launch.DesignResult, err error) intentResult {
	targets := []intentTarget{{Kind: "goal", ID: id}, {Kind: "design", ID: destination}}
	rel := relativeOrSame(inv.layout.GitRoot, destination)
	data := map[string]any{"goal": id, "document": rel, "attempt": result.Attempt.Attempt, "current": result.Current, "rejoined": result.Rejoined, "draft": result.Attempt.Draft}
	switch {
	case errors.Is(err, launch.ErrDesignStale):
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, Summary: err.Error() + "; nothing was launched",
			next: inv.publicArgv("design", id, "--brief", "FILE", "--after", strconv.Itoa(result.Current)), nextReason: "a new attempt follows the newest one"}
	case errors.Is(err, launch.ErrDesignWriterRunning):
		return intentResult{Outcome: intentInProgress, Targets: targets, Data: data, Summary: err.Error() + "; nothing was launched",
			next: inv.publicArgv("wait", "goal", id), nextReason: "the current author is still writing"}
	case err != nil && result.Attempt.Attempt == 0:
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data, Summary: err.Error() + "; nothing was launched"}
	}
	record := result.Record
	if err != nil && record.ID == "" {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
			Summary: fmt.Sprintf("design attempt %d for goal %s did not start: %v", result.Attempt.Attempt, id, err),
			next:    inv.sameCommand(), nextReason: "the same request starts the same reserved attempt once the cause is fixed"}
	}
	data["state"] = record.State
	attempt := result.Attempt
	switch {
	case record.State == launch.Completed && attempt.Outcome == "":
		manager := inv.designManager()
		attempt, err = manager.RecordDesignOutcome(destination, attempt.Attempt, func(retained launch.DesignAttempt) (string, string, string, error) {
			draft, readErr := os.ReadFile(retained.Draft)
			if readErr != nil {
				return "invalid", "the author left no draft: " + readErr.Error(), "", nil
			}
			var expected []byte
			if retained.Prior != "" {
				expected, _ = os.ReadFile(retained.Prior)
			}
			_, publishErr := project.PublishDesign(project.DesignPublication{Destination: destination, Root: inv.layout.GitRoot,
				Expected: expected, ExpectedPresent: retained.ExpectedPresent, Draft: draft, RecordID: designRecordID(destination, retained, inv), Goal: id})
			switch {
			case errors.Is(publishErr, project.ErrDesignChanged):
				return "conflict", publishErr.Error(), "", nil
			case errors.Is(publishErr, project.ErrDesignInvalid):
				return "invalid", publishErr.Error(), "", nil
			case publishErr != nil:
				return "", "", "", publishErr
			}
			return "published", "", digestText(draft), nil
		})
		if err != nil {
			return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data, Summary: "the draft could not be judged against the document: " + err.Error(), next: inv.sameCommand(), nextReason: "the same request retries the publication"}
		}
	case record.State == launch.Completed:
	case record.State.Terminal():
		data["reason"] = record.Reason
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data,
			Summary: fmt.Sprintf("design attempt %d for goal %s ended %s (%s); the document is unchanged", attempt.Attempt, id, record.State, record.Reason),
			next:    inv.publicArgv("design", id, "--brief", "FILE", "--after", strconv.Itoa(attempt.Attempt)), nextReason: "ask for one new attempt after the failed one"}
	default:
		return intentResult{Outcome: intentInProgress, Targets: targets, Data: data,
			Summary: fmt.Sprintf("design attempt %d for goal %s is %s", attempt.Attempt, id, record.State),
			next:    inv.publicArgv("show", "design", "--goal", id, "--attempt", strconv.Itoa(attempt.Attempt)), nextReason: "the attempt and its draft"}
	}
	data["outcome"], data["detail"] = attempt.Outcome, attempt.Detail
	switch attempt.Outcome {
	case "published":
		return intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
			Summary: fmt.Sprintf("design attempt %d for goal %s is written to %s as a draft", attempt.Attempt, id, rel),
			next:    inv.publicArgv("review", "design", rel, "--goal", id), nextReason: "an independent critique examines the draft"}
	case "conflict", "invalid", "superseded":
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data,
			Summary:  fmt.Sprintf("design attempt %d was not written to %s (%s): %s; the document is unchanged and the proposal is kept", attempt.Attempt, rel, attempt.Outcome, attempt.Detail),
			Decision: fmt.Sprintf("merge the proposal (%s) into the document yourself, or ask for a new attempt against the current version: metasystem design %s --brief FILE --after %d", attempt.Draft, id, attempt.Attempt)}
	}
	return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: data, Summary: "the attempt has no recorded outcome"}
}

func designRecordID(destination string, attempt launch.DesignAttempt, inv *intentInvocation) string {
	if id, _, err := inv.designManager().DesignDocument(destination); err == nil {
		return id
	}
	return ""
}

func relativeOrSame(root, path string) string {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// runIntentShowDesignAttempts shows a document's retained author attempts,
// or one attempt's proposal.
func (inv *intentInvocation) showDesignAttempts(id string) intentResult {
	destination, _, problem := inv.designDestination(id, false)
	if problem != nil {
		return *problem
	}
	manager := inv.designManager()
	_, attempts, err := manager.DesignDocument(destination)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
	}
	rel := relativeOrSame(inv.layout.GitRoot, destination)
	views, lines := []map[string]any{}, []string{}
	for _, attempt := range attempts {
		state := ""
		if record, readErr := manager.Store.Read(attempt.LaunchID); readErr == nil {
			state = string(record.State)
		}
		views = append(views, map[string]any{"attempt": attempt.Attempt, "state": state, "outcome": attempt.Outcome, "draft": attempt.Draft, "detail": attempt.Detail})
		lines = append(lines, fmt.Sprintf("  attempt %d: %s %s  draft %s", attempt.Attempt, state, attempt.Outcome, attempt.Draft))
	}
	if n := inv.input.text("attempt"); n != "" {
		number, convErr := strconv.Atoi(n)
		if convErr != nil || number < 1 || number > len(attempts) {
			return intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("%s has no attempt %s; nothing was read", rel, n)}
		}
		attempt := attempts[number-1]
		draft, _ := os.ReadFile(attempt.Draft)
		return intentResult{Outcome: intentConfirmed, Data: views[number-1], text: strings.Split(strings.TrimRight(string(draft), "\n"), "\n"),
			Summary: fmt.Sprintf("design attempt %d for %s (%s): its proposal is %s", number, rel, attempt.Outcome, attempt.Draft)}
	}
	return intentResult{Outcome: intentConfirmed, Data: map[string]any{"document": rel, "attempts": views}, text: lines,
		Summary: fmt.Sprintf("%s has %d design attempt(s)", rel, len(attempts))}
}

// runIntentStopDesign stops a design author through the launch owner's
// cancellation: the named or newest attempt of one document only.
func runIntentStopDesign(inv *intentInvocation, id string) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	destination, _, problem := inv.designDestination(id, false)
	if problem != nil {
		return inv.render(*problem)
	}
	manager := inv.designManager()
	_, attempts, err := manager.DesignDocument(destination)
	if err != nil || len(attempts) == 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("%s has no design attempt to stop; nothing was done", destination)})
	}
	attempt := attempts[len(attempts)-1]
	if n := inv.input.text("attempt"); n != "" {
		number, convErr := strconv.Atoi(n)
		if convErr != nil || number < 1 || number > len(attempts) {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--attempt names a recorded attempt; nothing was done"})
		}
		attempt = attempts[number-1]
	}
	record, err := manager.Cancel(attempt.LaunchID)
	targets := []intentTarget{{Kind: "design", ID: destination}}
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: "the design author was not stopped: " + err.Error()})
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: map[string]any{"attempt": attempt.Attempt, "state": record.State},
		Summary: fmt.Sprintf("design attempt %d is %s", attempt.Attempt, record.State)})
}

func digestText(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// designWithin reports whether path is root or beneath it.
func designWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// designAttemptView is the newest author attempt of one of the goal's
// design documents, read through the launch owner.
type designAttemptView struct {
	Document, Destination string
	Attempt               launch.DesignAttempt
	Record                launch.Record
	// SupervisorLost is a claimed supervisor the prober reports dead.
	SupervisorLost bool
}

// goalDesignAttempts reads the newest author attempt of every design
// document of the goal: its records and its default new document.
func (inv *intentInvocation) goalDesignAttempts(id string) []designAttemptView {
	manager := inv.designManager()
	if manager == nil {
		return nil
	}
	roots := project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: inv.stateRoot}
	candidates := []string{}
	if read, err := project.Read(roots); err == nil {
		for _, record := range read.List(project.KindDesign, project.ListOptions{Goal: id}) {
			candidates = append(candidates, canonicalDocument(filepath.Join(inv.layout.GitRoot, record.Path)))
		}
	}
	for _, home := range project.Homes(roots) {
		if home.Kind == project.KindDesign && home.Glob == "" {
			candidates = append(candidates, canonicalDocument(filepath.Join(home.Path, id+".md")))
		}
	}
	seen := map[string]bool{}
	var views []designAttemptView
	for _, destination := range candidates {
		if seen[destination] {
			continue
		}
		seen[destination] = true
		_, attempts, err := manager.DesignDocument(destination)
		if err != nil || len(attempts) == 0 {
			continue
		}
		view := designAttemptView{Document: relativeOrSame(inv.layout.GitRoot, destination), Destination: destination, Attempt: attempts[len(attempts)-1]}
		if record, readErr := manager.Store.Read(view.Attempt.LaunchID); readErr == nil {
			view.Record = record
			if !record.State.Terminal() && record.Supervisor != nil && manager.Prober != nil && identity.AliveRef(manager.Prober, *record.Supervisor) == identity.Dead {
				view.SupervisorLost = true
			}
		}
		views = append(views, view)
	}
	return views
}

func designStage(view designAttemptView) string {
	switch {
	case view.SupervisorLost:
		return "author lost before finishing"
	case view.Attempt.Outcome != "":
		return view.Attempt.Outcome
	case view.Record.State == "":
		return "reserved"
	}
	return string(view.Record.State)
}

// waitDesign waits for the one running design author of the goal through
// the launch owner, then judges its proposal as design G would.
func (inv *intentInvocation) waitDesign(id string, timeout time.Duration) (intentResult, bool) {
	var running []designAttemptView
	for _, view := range inv.goalDesignAttempts(id) {
		if view.Record.State != "" && !view.Record.State.Terminal() && !view.SupervisorLost {
			running = append(running, view)
		}
	}
	if len(running) != 1 {
		return intentResult{}, false
	}
	view := running[0]
	if timeout <= 0 {
		timeout = time.Minute
	}
	record, _, err := inv.designManager().Wait(view.Attempt.LaunchID, timeout)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Summary: "the design author cannot be observed: " + err.Error()}, true
	}
	return inv.designOutcome(id, view.Destination, launch.DesignResult{Attempt: view.Attempt, Record: record, Current: view.Attempt.Attempt}, nil), true
}
