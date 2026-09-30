package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func runReportStopStatus(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("session status", stdout, stderr)
	id := flags.String("id", "", "exact short Stop report alias or legacy full id")
	root := pathFlag(flags, "root", "", "explicit metasystem installation")
	verbose := flags.Bool("verbose", false, "also the plan lines older than a week, every health role and the report's file")
	if flags.Parse(args) != nil {
		return 2
	}
	refuse := func(code int, line1 string, hint textui.Hint) int {
		page := passthroughPage(stderr, "", *verbose)
		page.Refusal(line1, hint)
		printPage(stderr, page)
		return code
	}
	retry := textui.Hint{Reason: "run metasystem session status --id with the id the Stop line printed"}
	if *id == "" || len(flags.Args()) != 0 {
		return refuse(2, "session status needs the id from a Stop line, so nothing was read", retry)
	}
	if err := report.ValidateStopStatusID(*id); err != nil {
		return refuse(2, fmt.Sprintf("%s is not a Stop report id (those are lowercase hexadecimal); nothing was read", *id), retry)
	}
	resolvedRoot, err := report.StopStatusRoot(*root)
	var data []byte
	var resolution stopreport.Resolution
	if err == nil {
		data, _, resolution, err = stopreport.Read(resolvedRoot, *id)
	}
	if err != nil {
		return refuse(1, fmt.Sprintf("Stop report %s could not be read: %v", *id, err),
			textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
	}
	summary, err := readStopSummary(data)
	if err != nil {
		return refuse(1, fmt.Sprintf("Stop report %s could not be read: %v", *id, err),
			textui.Hint{Argv: []string{"metasystem", "system", "check"}, Reason: "names what is wrong here"})
	}
	page := passthroughPage(stdout, resolvedRoot, *verbose)
	// Its first line names the open behaviour alerts, live, beside the
	// report's own record.
	if attention, open := patternAlertAttention(openPatternAlerts(resolvedRoot, board.Home)); open {
		page.Banner(attention)
	}
	summary.lay(page, resolvedRoot, resolution.Path)
	printPage(stdout, page)
	return 0
}

// stopSummary is a published Stop report's record as session status reads
// it: the JSON sections the presenter wrote beside its prose, never the
// prose itself.
type stopSummary struct {
	control     report.StopControl
	completion  report.StopCompletion
	judgment    *goal.TurnVerdictFacts
	health      *steward.HookHealthPreview
	arming      *report.StopArmingResult
	notices     []report.StopNotice
	unavailable []report.StopUnavailable
}

// readStopSummary decodes the report's JSON sections by their headings.
func readStopSummary(data []byte) (stopSummary, error) {
	var summary stopSummary
	sections := map[string]any{
		"Retained Stop control": &summary.control, "Completion": &summary.completion,
		"Original turn verdict and frozen judgment": &summary.judgment, "Health": &summary.health,
		"Supervision arming": &summary.arming, "Notices": &summary.notices, "Unavailable observations": &summary.unavailable,
	}
	for _, part := range strings.Split(string(data), "\n## ")[1:] {
		heading, body, _ := strings.Cut(part, "\n")
		target, known := sections[heading]
		if !known {
			continue
		}
		_, encoded, found := strings.Cut(body, "```json\n")
		encoded, _, closed := strings.Cut(encoded, "\n```")
		if !found || !closed {
			return stopSummary{}, fmt.Errorf("its %s section holds no record", strings.ToLower(heading))
		}
		if err := json.Unmarshal([]byte(encoded), target); err != nil {
			return stopSummary{}, fmt.Errorf("its %s section: %v", strings.ToLower(heading), err)
		}
	}
	return summary, nil
}

// stopPlanAge is how long a plan may go untouched before session status
// leaves its line to --verbose.
const stopPlanAge = 7 * 24 * time.Hour

// lay prints the summary: first what a person must decide and what
// supervision needs repaired, each with its remedy; then the headline;
// then the seat's actions with their commands, the plans' open lines
// touched within a week, and the counts.
func (s stopSummary) lay(page *textui.Page, installation, reportPath string) {
	env := page.Env()
	for _, need := range s.interventions() {
		attention := textui.Attention{State: textui.Alert, Text: shortPaths(env, need.who+need.detail)}
		if need.remedy != "" {
			attention.Hint = textui.Hint{Reason: shortPaths(env, need.remedy)}
		}
		page.Banner(attention)
	}
	headline := "Stop was allowed"
	if s.control.ShouldBlock {
		headline = "Stop was blocked"
	}
	task := "no task in flight"
	if s.judgment != nil {
		switch {
		case s.judgment.Ownership.GoalId != "":
			task = "task " + s.judgment.Ownership.GoalId
		case s.judgment.Work.Selected != nil:
			task = "next " + s.judgment.Work.Selected.Id
		}
	}
	observed := ""
	if at, err := time.Parse(time.RFC3339Nano, s.completion.ObservedAt); err == nil {
		observed = "observed " + env.Time(at)
	}
	page.Headline(headline, task, observed)
	if s.judgment != nil && len(s.seatActions()) > 0 {
		section := page.Section("Seat actions", "")
		for _, action := range s.seatActions() {
			item := section.Item(textui.Alert, action.Instruction)
			if action.Command != "" {
				item.KV("run", textui.Plain(shortPaths(env, action.Command)))
			}
			if action.Restriction != "" {
				item.KV("only", textui.Plain(action.Restriction))
			}
		}
	}
	s.layPlans(page, installation)
	facts := []textui.KV{{Key: "finished", Value: []textui.Span{textui.Plain(s.completionWords(env))}}}
	if s.judgment != nil {
		work := s.judgment.Work
		facts = append(facts, textui.KV{Key: "goals", Value: []textui.Span{textui.Plain(fmt.Sprintf("%d claimable · %d queued · %s",
			len(work.Claimable), work.Queued, textui.Count(len(work.NonTerminalJobs), "job running", "jobs running")))}})
	}
	if s.health != nil {
		facts = append(facts, textui.KV{Key: "health", Value: []textui.Span{textui.Plain(s.healthWords())}})
	}
	section := page.Section("Summary", "")
	for _, fact := range facts {
		section.KV(fact.Key, fact.Value...)
	}
	if page.Verbose() {
		if s.health != nil {
			roles := page.Section("Health", "")
			for _, role := range s.health.Verdict.Roles {
				words := string(role.Status) + ": " + role.Reason
				if role.Remedy != "" {
					words += "; " + role.Remedy
				}
				roles.KV(string(role.Role), textui.Plain(shortPaths(env, words)))
			}
		}
		// The whole report is the file whose name ends in its attempt; the
		// session's digest that leads the name is no reading matter.
		section := page.Section("Whole report", "")
		section.KV("in", textui.Plain(env.Path(filepath.Dir(reportPath))))
		if _, attempt, found := strings.Cut(strings.TrimSuffix(filepath.Base(reportPath), ".md"), "-"); found {
			section.KV("named", textui.Plain("…-"+attempt+".md"))
		}
	}
}

// stopNeed is one thing only a person, or a repair of supervision,
// resolves.
type stopNeed struct{ who, detail, remedy string }

// interventions are the report's human decisions and supervision repairs,
// in the order the report lists them.
func (s stopSummary) interventions() []stopNeed {
	var needs []stopNeed
	add := func(detail, remedy string, human, repair bool) {
		switch {
		case human:
			needs = append(needs, stopNeed{"needs your decision: ", detail, remedy})
		case repair:
			needs = append(needs, stopNeed{"supervision needs repair: ", detail, remedy})
		}
	}
	if s.judgment != nil {
		for _, action := range s.judgment.Actions {
			add(action.Instruction, action.Command, action.HumanRequired, action.SupervisionRepair)
		}
		refusal := s.judgment.Refusal
		add(refusal.Detail, refusal.Remedy, refusal.HumanRequired, refusal.SupervisionRepair)
		add(refusal.Escalation.Detail, refusal.Escalation.AlarmDetail, refusal.Escalation.HumanRequired, refusal.Escalation.SupervisionRepair)
	}
	if s.health != nil {
		for index, intervention := range s.health.Interventions {
			if index < len(s.health.Verdict.Roles) {
				role := s.health.Verdict.Roles[index]
				add(role.Reason, role.Remedy, intervention.HumanRequired, intervention.SupervisionRepair)
			}
		}
	}
	if s.arming != nil {
		for _, component := range s.arming.Components {
			add(component.Detail, component.Remedy, component.HumanRequired, component.SupervisionRepair)
		}
	}
	for _, notice := range s.notices {
		add(notice.Detail, notice.Remedy, notice.HumanRequired, notice.SupervisionRepair)
	}
	for _, unavailable := range s.unavailable {
		add(unavailable.Cause, unavailable.Remedy, unavailable.HumanRequired, unavailable.SupervisionRepair)
	}
	return needs
}

// seatActions are the judgment's actions the seat itself takes.
func (s stopSummary) seatActions() []goal.TurnAction {
	var actions []goal.TurnAction
	for _, action := range s.judgment.Actions {
		if !action.HumanRequired && !action.SupervisionRepair {
			actions = append(actions, action)
		}
	}
	return actions
}

// stopPlanLine is one plan's open line: its kind, the plan and what it
// asks.
var stopPlanLine = regexp.MustCompile(`^[A-Z-]+ (\S+): (.*)$`)

// layPlans lists the plans' open lines with how long each plan has gone
// untouched; a plan untouched for more than a week is left to --verbose.
func (s stopSummary) layPlans(page *textui.Page, installation string) {
	if s.judgment == nil {
		return
	}
	env := page.Env()
	scan := s.judgment.Scan
	var rows []string
	older := 0
	for _, group := range []struct {
		label string
		items []goal.Item
	}{{"waits on a person", scan.WaitingOnHuman}, {"open work", scan.Open}, {"template unfilled", scan.TemplateUnfilled}, {"stale", scan.StalePlans}} {
		for _, item := range group.items {
			plan, asks := item.SourcePath, item.RequestedAction
			if match := stopPlanLine.FindStringSubmatch(item.FullDetail); match != nil {
				if plan == "" {
					plan = match[1]
				}
				if asks == "" {
					asks = match[2]
				}
			}
			if plan == "" {
				plan, asks = item.Id, item.Detail
			}
			age := ""
			if info, err := os.Stat(filepath.Join(installation, filepath.FromSlash(plan))); err == nil {
				if untouched := env.Now.Sub(info.ModTime()); untouched > stopPlanAge {
					older++
					if !page.Verbose() {
						continue
					}
					age = ", untouched " + textui.Duration(untouched)
				} else {
					age = ", touched " + env.Ago(info.ModTime())
				}
			}
			rows = append(rows, fmt.Sprintf("%s (%s%s): %s", plan, group.label, age, asks))
		}
	}
	if len(rows) == 0 && older == 0 {
		return
	}
	aside := ""
	if older > 0 && !page.Verbose() {
		aside = textui.Count(older, "plan line", "plan lines") + " older than 7 days: --verbose shows them"
	}
	section := page.Section("Plans", aside)
	for _, row := range rows {
		section.Text(shortPaths(env, row))
	}
	if len(rows) == 0 {
		section.Text("no plan line touched in the last 7 days")
	}
}

// completionWords says whether the seat finished work since its last Stop.
func (s stopSummary) completionWords(env textui.Env) string {
	since := ""
	if at, err := time.Parse(time.RFC3339Nano, s.completion.IntervalStart); err == nil {
		since = " since " + env.Time(at)
	}
	switch s.completion.State {
	case "observed":
		return "new finished work" + since
	case "none":
		return "nothing new" + since
	}
	return "unknown for this turn"
}

// healthWords counts the roles by state.
func (s stopSummary) healthWords() string {
	alive, dead, unknown := 0, 0, 0
	for _, role := range s.health.Verdict.Roles {
		switch role.Status {
		case steward.HealthAlive:
			alive++
		case steward.HealthDead:
			dead++
		default:
			unknown++
		}
	}
	if dead == 0 && unknown == 0 {
		return fmt.Sprintf("all %s alive", textui.Count(alive, "role", "roles"))
	}
	return fmt.Sprintf("%d alive · %d dead · %d unknown", alive, dead, unknown)
}

// renderStopBlock renders one Stop refusal: an unrecorded block (bounded idle
// or the open-work form) with an optional system message, or, when the
// request names a record, the recorded refusal that owns the occurrence
// count. An open-work root first marks its open-work lines durably seen.
func renderStopBlock(request hooks.StopBlockRequest, now time.Time) (map[string]any, int, error) {
	class := request.Class
	if class == "" {
		class = string(report.StopClassSeatActionable)
	}
	if class != string(report.StopClassInfrastructure) && class != string(report.StopClassSeatActionable) {
		return nil, 2, fmt.Errorf("report stop-block: the class must be infrastructure or seat-actionable")
	}
	systemMessage := request.SystemMessage
	if request.OpenWorkRoot != "" {
		if warning := report.OpenWorkSeenWarning(request.OpenWorkRoot); warning != "" {
			if systemMessage != "" {
				systemMessage += "\n"
			}
			systemMessage += warning
		}
	}
	if request.ArmingResult != "" {
		if systemMessage != "" {
			systemMessage += "\n"
		}
		systemMessage += request.ArmingResult
	}
	systemMessage = report.BoundSystemMessage(systemMessage)
	if request.RefusalRecord != "" || request.Session != "" || request.Cause != "" || request.Remedy != "" {
		if request.RefusalRecord == "" || request.Session == "" || request.Cause == "" || request.Remedy == "" {
			return nil, 2, fmt.Errorf("report stop-block: the refusal record, session, cause, and remedy must be provided together")
		}
		block, err := report.StopRefusal(request.RefusalRecord, request.Session, request.Cause, request.Remedy, request.Detail, systemMessage, report.StopClass(class), now)
		if err != nil {
			return nil, 1, fmt.Errorf("report stop-block: %v", err)
		}
		return block, 0, nil
	}
	var block map[string]any
	if request.BoundedIdle {
		block = report.BoundedIdleStopBlock(request.Detail)
	} else {
		block = report.StopBlock(request.Detail)
	}
	if systemMessage != "" {
		block["systemMessage"] = systemMessage
	}
	return block, 0, nil
}
