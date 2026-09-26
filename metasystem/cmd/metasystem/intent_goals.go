package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// The goal commands read the accepted ledger through the existing projection
// and change it only through the existing goal owners: approval, budget
// routing, park and unpark, stopped-goal resume and conclusion keep their own
// proof, locking and publication. These adapters choose the owner from the
// goal's recorded state and render the owner's typed result.

func (inv *intentInvocation) projection() (goal.Projection, time.Time, *intentResult) {
	endpoint, err := inv.owners.dependencies.endpoint(inv.stateRoot)
	if err != nil {
		return goal.Projection{}, time.Time{}, &intentResult{Outcome: intentFailed, Summary: "cannot read the goal ledger: " + err.Error(), code: 1}
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return goal.Projection{}, time.Time{}, &intentResult{Outcome: intentFailed, Summary: "cannot read the goal clock: " + err.Error(), code: 1}
	}
	// --fetch is the goal owner's explicit fetch and validation.
	projection, err := goal.Project(endpoint, inv.input.switched("fetch"), now)
	if err != nil {
		return goal.Projection{}, time.Time{}, &intentResult{Outcome: intentFailed, Summary: "cannot project the accepted goal ledger: " + err.Error(), code: 1}
	}
	return projection, now, nil
}

// singleTarget is the goal named first or with --id; naming two different
// goals is a mistake caught before anything happens.
func (inv *intentInvocation) singleTarget() (string, *intentResult) {
	named := ""
	if len(inv.input.args) > 0 {
		named = inv.input.args[0]
	}
	if flagged := inv.input.text("id"); flagged != "" {
		if named != "" && named != flagged {
			return "", &intentResult{Outcome: intentRefused, code: 2,
				Summary:  fmt.Sprintf("names two goals, %s and --id %s; nothing was done", shellCommand([]string{named}), shellCommand([]string{flagged})),
				Decision: "name the goal once"}
		}
		named = flagged
	}
	return named, nil
}

func (inv *intentInvocation) targets(ids ...string) []intentTarget {
	targets := make([]intentTarget, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, intentTarget{Kind: "goal", ID: id})
	}
	return targets
}

// publicArgv is a public command line the caller can run from where they are.
func (inv *intentInvocation) publicArgv(words ...string) []string {
	argv := append([]string{"metasystem"}, words...)
	if inv.input.has("repo") {
		repo := inv.input.text("repo")
		if !filepath.IsAbs(repo) {
			repo = filepath.Join(inv.cwd, repo)
		}
		argv = append(argv, "--repo", repo)
	}
	return argv
}

// missingTarget names the command's grammar and the goals it could take.
func (inv *intentInvocation) missingTarget(fits func(*goal.GoalFile) bool) int {
	result := intentResult{Outcome: intentRefused, code: 2,
		Summary:  fmt.Sprintf("needs a goal: %s; nothing was done", inv.command.usage[0]),
		Decision: "name the goal"}
	if projection, _, problem := inv.projection(); problem == nil {
		var candidates []string
		for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
			if fits == nil || fits(projection.Tree.Live[id]) {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) > 8 {
			candidates = append(candidates[:8], "...")
		}
		if len(candidates) > 0 {
			result.text = []string{"goals it could take: " + strings.Join(candidates, " ")}
			result.Data = map[string]any{"candidates": candidates}
		}
	}
	return inv.render(result)
}

// forward passes the named options through to an owner unchanged.
func (inv *intentInvocation) forward(names ...string) []string {
	var args []string
	for _, name := range names {
		if !inv.input.has(name) {
			continue
		}
		definition, _ := inv.command.lookupFlag(name)
		if definition.value == "" {
			if inv.input.switched(name) {
				args = append(args, "--"+name)
			}
			continue
		}
		args = append(args, "--"+name, inv.input.text(name))
	}
	return args
}

// callOwner runs one existing owner with this invocation's report and renders
// its outcome; after reads the confirmed state back from the ledger.
func (inv *intentInvocation) callOwner(targets []intentTarget, run func(syncRequestDependencies) int, after func() intentResult) int {
	return inv.render(inv.ownerCall(targets, run, after))
}

func (inv *intentInvocation) ownerCall(targets []intentTarget, run func(syncRequestDependencies) int, after func() intentResult) intentResult {
	report := &ownerReport{}
	dependencies := inv.owners.dependencies
	dependencies.report = report
	code := run(dependencies)
	var confirmed intentResult
	if code == 0 && report.result != nil && report.result.Outcome == goal.OutcomeConfirmed && report.refusal == nil {
		confirmed = after()
		if data, ok := confirmed.Data.(map[string]any); ok {
			data["owner"] = ownerPublication(*report.result)
		}
	}
	result := ownerResult(report, code, confirmed)
	result.Targets = targets
	return result
}

// actorArgs names who acts on pause and done. An agent names its session
// with --lineage or METASYSTEM_OWNER_LINEAGE, and a person may add --by. With
// none of these the act is the person's at this terminal: the enrolled name
// is used only after the owner's proof shows this shell descends from the
// enrolled terminal, and the owner proves it again when it acts.
func (inv *intentInvocation) actorArgs(id string, names ...string) ([]string, *intentResult) {
	forwarded := inv.forward(names...)
	dependencies := inv.owners.dependencies
	if inv.input.has("by") || inv.input.has("lineage") || inv.input.switched("fixture-human-authority") ||
		dependencies.ownerLineage != nil && dependencies.ownerLineage() != "" {
		return forwarded, nil
	}
	refused := func(err error) *intentResult {
		return &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary:  "cannot tell who acts: no --by, --lineage or METASYSTEM_OWNER_LINEAGE, and the enrolled-terminal proof failed: " + err.Error() + "; nothing was done",
			Decision: "a person runs this at the enrolled terminal; an agent session passes its own lineage with --lineage LINEAGE (a session is started with metasystem up)"}
	}
	if dependencies.proveHuman == nil {
		return nil, refused(fmt.Errorf("no human proof reader is available"))
	}
	now, err := inv.owners.commandNow(inv.stateRoot)
	if err != nil {
		return nil, refused(err)
	}
	proof, err := dependencies.proveHuman(inv.stateRoot, int64(os.Getppid()), nil, now)
	if err == nil && !proof.EnrolledTerminalFor(inv.stateRoot) {
		err = fmt.Errorf("this shell does not descend from the enrolled terminal (%s)", proof.Outcome)
	}
	if err != nil {
		return nil, refused(err)
	}
	flags := &syncFlags{root: inv.stateRoot}
	if err := resolveGoalHuman(flags, proof); err != nil {
		return nil, refused(err)
	}
	return append(forwarded, "--by", flags.by), nil
}

func unknownGoal(inv *intentInvocation, id string) int {
	return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
		Summary: fmt.Sprintf("no goal %s on the accepted ledger; nothing was done", shellCommand([]string{id})),
		next:    inv.publicArgv("goals", "--all"), nextReason: "list the goals by id"})
}

// goalRecord finds a goal wherever the accepted ledger keeps it.
func goalRecord(projection goal.Projection, id string) (*goal.GoalFile, string) {
	if file := projection.Tree.Live[id]; file != nil {
		return file, "live"
	}
	if file := projection.Tree.Done[id]; file != nil {
		return file, "done"
	}
	if file := projection.Tree.Abandoned[id]; file != nil {
		return file, "abandoned"
	}
	return nil, ""
}

// intentBudgetView is the budget block show and budget print: the standing
// box and the existing spending projection, never a second clock.
type intentBudgetView struct {
	Box        string                         `json:"box,omitempty"`
	Lens       string                         `json:"lens"`
	Projection *dispatchcore.BudgetProjection `json:"projection,omitempty"`
}

func budgetView(stateRoot string, file *goal.GoalFile, now time.Time) intentBudgetView {
	if file == nil || file.Budget == nil {
		return intentBudgetView{Lens: "none"}
	}
	view := intentBudgetView{Box: goalbudget.FormatBox(*file.Budget)}
	var projection dispatchcore.BudgetProjection
	if file.State == goal.StateClaimed && file.Claimed != nil {
		view.Lens = "claim"
		projection = dispatchcore.ProjectBudget(stateRoot, file, now)
	} else {
		view.Lens = "episode"
		projection = dispatchcore.BudgetProjection(dispatchcore.ProjectConsumption(stateRoot, file, now))
	}
	view.Projection = &projection
	return view
}

func (view intentBudgetView) lines() []string {
	if view.Projection == nil {
		return []string{"budget: none; approval gives the goal its tier's box"}
	}
	lines := []string{fmt.Sprintf("budget: %s (spent this %s)", view.Box, view.Lens)}
	projection := view.Projection
	if projection.Status != dispatchcore.BudgetKnown {
		reason := string(projection.Status)
		if projection.Unknown != nil {
			reason = projection.Unknown.Reason + " (" + projection.Unknown.Record + ")"
		}
		return append(lines, "spent: unknown: "+reason)
	}
	limits := projection.Limits
	lines = append(lines, fmt.Sprintf("spent: elapsed %s of %s, attempts %d of %d, job minutes %d of %d reserved, active jobs %d of %d",
		projection.Elapsed.Round(time.Minute), limits.ElapsedLimit, projection.Attempts, limits.AttemptLimit,
		projection.ReservedJobMinutes, limits.ReservedJobMinutesLimit, projection.ActiveJobs, limits.ActiveJobLimit))
	if projection.ElapsedState != "" {
		lines = append(lines, "elapsed: "+string(projection.ElapsedState))
	}
	return lines
}

type intentDesignRef struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

// linkedDesigns are the design records naming the goal on their Goals line,
// read through the project reader the interface uses.
func (inv *intentInvocation) linkedDesigns(id string) ([]intentDesignRef, string) {
	roots := project.Roots{Checkout: inv.layout.GitRoot, Installation: inv.layout.InstallationRoot, StateRoot: inv.stateRoot}
	read, err := project.Read(roots)
	if err != nil {
		return nil, err.Error()
	}
	var refs []intentDesignRef
	for _, record := range read.List(project.KindDesign, project.ListOptions{Goal: id}) {
		refs = append(refs, intentDesignRef{ID: record.ID, Path: record.Path, Status: record.Status, Title: record.Title})
	}
	return refs, ""
}

// suggestedNext is the act a goal waits for, when one person's act is the
// known next step.
func (inv *intentInvocation) suggestedNext(file *goal.GoalFile) ([]string, string) {
	switch {
	case file == nil:
		return nil, ""
	case file.State == goal.StateQueued:
		return inv.publicArgv("approve", file.Id), "the goal waits for a person's approval"
	case file.State == goal.StateParked:
		return inv.publicArgv("resume", file.Id), "the goal is parked"
	case file.IsFencedClaim():
		return inv.publicArgv("resume", file.Id), "the goal was stopped by its budget and resumes under its standing box"
	}
	return nil, ""
}

func runIntentGoals(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	labels := inv.input.values["label"]
	if err := goal.ValidateLabels(labels); err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: err.Error(), Decision: "use label tokens the ledger accepts"})
	}
	includeArchived, history := inv.input.switched("all"), inv.input.switched("history")
	grouped := map[string][]*goal.GoalFile{}
	open := []*goal.GoalFile{}
	for _, id := range goal.OrderedOpenGoalIDs(projection.Tree.Live) {
		if file := projection.Tree.Live[id]; goal.MatchesLabels(file.Labels, labels) {
			shown := goalDisplayRecord(file, history)
			open = append(open, shown)
			grouped[shown.State] = append(grouped[shown.State], shown)
		}
	}
	archived := func(records map[string]*goal.GoalFile) []*goal.GoalFile {
		list := []*goal.GoalFile{}
		for _, id := range goal.SortedGoalIds(records) {
			if file := records[id]; goal.MatchesLabels(file.Labels, labels) {
				list = append(list, goalDisplayRecord(file, history))
			}
		}
		return list
	}
	data := map[string]any{"root": inv.stateRoot, "tip": projection.Tip, "banners": projection.Banners, "open": open}
	if includeArchived {
		grouped[goal.StateDone] = archived(projection.Tree.Done)
		grouped[goal.StateAbandoned] = archived(projection.Tree.Abandoned)
		data["done"], data["abandoned"] = grouped[goal.StateDone], grouped[goal.StateAbandoned]
	}
	summary := goalListSummary(grouped, syncedListStates, projection.Tip, projection.Banners, includeArchived, projection.Horizon, projection.Tree.TrunkRed...)
	text := []string{strings.TrimRight(summary, "\n")}
	if history {
		// Every listed record, archived ones included when --all lists them.
		for _, list := range [][]*goal.GoalFile{open, grouped[goal.StateDone], grouped[goal.StateAbandoned]} {
			for _, file := range list {
				text = append(text, goalHistoryLines(file.Id+" ", file)...)
			}
		}
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Data: data, text: text,
		Summary: fmt.Sprintf("%d open goal(s) at %s", len(open), projection.Tip)})
}

// goalHistoryLines reads a goal's ledger history as one line per act: when,
// what, who, and the act's recorded reason where it has one.
func goalHistoryLines(prefix string, file *goal.GoalFile) []string {
	lines := make([]string, 0, len(file.History))
	for _, entry := range file.History {
		line := fmt.Sprintf("%shistory: %s %s by %s", prefix, entry.At, entry.Verb, entry.Actor)
		if len(entry.Targets) > 0 {
			line += " on " + strings.Join(entry.Targets, ",")
		}
		if entry.Reason != "" {
			line += ": " + entry.Reason
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, prefix+"history: none recorded")
	}
	if file.Conclude != "" {
		// A concluded goal's conclusion is on the record, not on its done line.
		lines = append(lines, prefix+"conclusion: "+file.Conclude)
	}
	return lines
}

func runIntentShow(inv *intentInvocation) int {
	if args := inv.input.args; len(args) > 0 && slices.Contains([]string{"designs", "decisions", "record", "design"}, args[0]) {
		return runIntentShowRecords(inv, args[0], args[1:])
	}
	if args := inv.input.args; len(args) == 2 && args[0] == "review" {
		return runIntentReviewRef(inv, "show", args[1])
	}
	if args := inv.input.args; len(args) > 0 && args[0] == "question" {
		return runIntentShowQuestion(inv, args[1:])
	}
	if len(inv.input.args) > 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "show takes one goal, or designs, decisions, record ID, design --goal G or question Q; nothing was done"})
	}
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.missingTarget(nil)
	}
	projection, now, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return unknownGoal(inv, id)
	}
	view := budgetView(inv.stateRoot, file, now)
	designs, designProblem := inv.linkedDesigns(id)
	data := map[string]any{"where": where, "tip": projection.Tip, "goal": goalDisplayRecord(file, inv.input.switched("history")), "budget": view, "designs": designs}
	if designProblem != "" {
		data["designsUnreadable"] = designProblem
	}
	text := []string{"intent: " + file.Intent, "next step: " + file.NextStep}
	if file.StopFence != nil {
		text = append(text, "stopped: "+string(file.StopFence.Reason)+" at "+file.StopFence.ClosedAt)
	}
	text = append(text, view.lines()...)
	for _, design := range designs {
		text = append(text, fmt.Sprintf("design: %s (%s) %s", design.Path, design.Status, design.Title))
	}
	if designProblem != "" {
		text = append(text, "design: unreadable: "+designProblem)
	}
	if inv.input.switched("history") {
		text = append(text, goalHistoryLines("", file)...)
	}
	result := intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), Data: data, text: text,
		Summary: fmt.Sprintf("%s  %s  tier %d", id, file.State, file.Tier)}
	if where == "live" {
		result.next, result.nextReason = inv.suggestedNext(file)
	}
	return inv.render(result)
}

func runIntentBudget(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	box := ""
	if len(inv.input.args) > 1 {
		box = inv.input.args[1]
	}
	if flagged := inv.input.text("budget"); flagged != "" {
		if box != "" && box != flagged {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary:  fmt.Sprintf("names two boxes, %s and --budget %s; nothing was done", shellCommand([]string{box}), shellCommand([]string{flagged})),
				Decision: "give the box once"})
		}
		box = flagged
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.missingTarget(nil)
	}
	projection, now, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return unknownGoal(inv, id)
	}
	if box == "" {
		view := budgetView(inv.stateRoot, file, now)
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: inv.targets(id), text: view.lines()[1:],
			Summary: view.lines()[0], Data: map[string]any{"where": where, "state": file.State, "budget": view}})
	}
	if where != "live" {
		// The owner refuses an archived goal with its own remedy.
		return inv.budgetOwner(id, box, file)
	}
	if under := inv.input.text("under"); under != "" {
		return inv.budgetUnderAttorney(file, box, under)
	}
	if box == "norm" && file.Tier == 0 {
		// The legacy owner would read a missing tier as tier three; a norm box
		// needs the goal classified first.
		return inv.render(*tierlessRefusal(inv, []string{id}))
	}
	return inv.budgetOwner(id, box, file)
}

func (inv *intentInvocation) budgetOwner(id, box string, before *goal.GoalFile) int {
	args := append([]string{"--root", inv.stateRoot, "--id", id}, inv.forward("by", "lineage", "approved-ref", "temporary-human-word", "review-by", "fixture-human-authority")...)
	args = append(args, box)
	result := inv.ownerCall(inv.targets(id), func(dependencies syncRequestDependencies) int {
		return runGoalBudgetWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies, inv.owners.binding)
	}, func() intentResult { return inv.afterGoalAct(id, "budget") })
	if result.Outcome == intentRefused && before.IsFencedClaim() && before.Budget != nil {
		// A goal stopped by its budget resumes under its standing box before a
		// new box is recorded; the public command for that is resume.
		if asked, problem := inv.completeBox(box, before); problem == nil && asked != *before.Budget {
			result.next = inv.publicArgv("resume", id)
			result.nextReason = "resume under the standing box first, then change it with metasystem budget " + id + " BOX"
			result.Decision = ""
		}
	}
	return inv.render(result)
}

// budgetUnderAttorney is a seat's budget act under a recorded power of
// attorney: the owner law approves a waiting goal or changes a running goal's
// box. A stopped goal is resumed only by a person.
func (inv *intentInvocation) budgetUnderAttorney(file *goal.GoalFile, box, under string) int {
	if file.IsFencedClaim() {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(file.Id),
			Summary:  "a power of attorney does not cover a goal stopped by its budget; nothing was done",
			Decision: "a person resumes it at the enrolled terminal: " + shellCommand(inv.publicArgv("resume", file.Id))})
	}
	budget, problem := inv.completeBox(box, file)
	if problem != nil {
		return inv.render(*problem)
	}
	verb := "approve"
	if file.State == goal.StateClaimed {
		verb = "set-budget"
	}
	// Every authority option goes to the owner, which refuses the ones an
	// act under a power of attorney cannot combine with.
	args := append([]string{"--root", inv.stateRoot, "--id", file.Id, "--under", under},
		inv.forward("by", "lineage", "approved-ref", "temporary-human-word", "review-by", "fixture-human-authority")...)
	args = append(args, budgetLongFlags(budget)...)
	return inv.callOwner(inv.targets(file.Id), func(dependencies syncRequestDependencies) int {
		flags, err := parseSyncFlagValues(verb, args)
		if err != nil {
			return dependencies.fail(2, err)
		}
		return runGoalUnderAttorneyWithInputs(verb, flags, inv.owners.commandNow, dependencies)
	}, func() intentResult { return inv.afterGoalAct(file.Id, "budget") })
}

// completeBox reads a compact box with the owner's own parser: norm is the
// goal's tier box, keep its standing box.
func (inv *intentInvocation) completeBox(box string, file *goal.GoalFile) (goal.Budget, *intentResult) {
	conf := filepath.Join(inv.stateRoot, "metasystem.conf")
	switch box {
	case "norm":
		tier := file.Tier
		if tier == 0 {
			return goal.Budget{}, tierlessRefusal(inv, []string{file.Id})
		}
		budget, err := config.TierBox(conf, tier)
		if err != nil {
			return goal.Budget{}, &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
		}
		return budget, nil
	case "keep":
		if file.Budget == nil {
			return goal.Budget{}, &intentResult{Outcome: intentRefused, code: 2, Summary: "keep needs a standing box and " + file.Id + " has none",
				Decision: "give the complete compact box, for example 1d/10/720m/1/3"}
		}
		return *file.Budget, nil
	}
	reviewRoundMax, err := config.ReviewRoundMax(conf)
	if err != nil {
		return goal.Budget{}, &intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()}
	}
	budget, err := goalbudget.ParseBox(box, file.Budget, reviewRoundMax)
	if err != nil {
		return goal.Budget{}, &intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(file.Id),
			Summary:  fmt.Sprintf("%s is not a box: %v; nothing was done", shellCommand([]string{box}), err),
			Decision: "give norm, keep, or the complete compact box elapsed/attempts/job-minutes/active-jobs/review-rounds, for example 1d/10/720m/1/3"}
	}
	return budget, nil
}

func budgetLongFlags(budget goal.Budget) []string {
	return []string{
		"--elapsed-limit", budget.ElapsedLimit,
		"--attempt-limit", fmt.Sprint(budget.AttemptLimit),
		"--reserved-job-minutes-limit", fmt.Sprint(budget.ReservedJobMinutesLimit),
		"--active-job-limit", fmt.Sprint(budget.ActiveJobLimit),
		"--review-round-limit", fmt.Sprint(budget.ReviewRoundLimit),
	}
}

// tierlessRefusal keeps the owner's law: a goal without a tier has no norm
// box, and classifying it takes a person's four risk answers and their basis.
// No tier or answer is guessed, so no command is offered to run.
func tierlessRefusal(inv *intentInvocation, ids []string) *intentResult {
	return &intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(ids...),
		Summary: fmt.Sprintf("%s has no tier, so there is no norm box to approve it under; nothing was approved", strings.Join(ids, ", ")),
		Decision: "classify it first: a person answers the four risk questions (severity, novelty, exposure, accumulation) and states the basis, " +
			"recorded with metasystem goal edit --id GOAL --risk severity=...,novelty=...,exposure=...,accumulation=... --basis TEXT; then approve again",
		Data: map[string]any{"missing": []string{"risk.severity", "risk.novelty", "risk.exposure", "risk.accumulation", "basis"}, "goals": ids}}
}

// afterGoalAct reads the goal back after a confirmed act.
func (inv *intentInvocation) afterGoalAct(id, act string) intentResult {
	projection, now, problem := inv.projection()
	if problem != nil {
		return intentResult{Summary: act + " confirmed for " + id + "; the ledger could not be read back: " + problem.Summary}
	}
	file, where := goalRecord(projection, id)
	if file == nil {
		return intentResult{Summary: act + " confirmed for " + id}
	}
	view := budgetView(inv.stateRoot, file, now)
	summary := fmt.Sprintf("%s: %s is %s", act, id, file.State)
	if view.Box != "" {
		summary += " under " + view.Box
	}
	return intentResult{Summary: summary, text: view.lines(),
		Data: map[string]any{"where": where, "goal": goalDisplayRecord(file, false), "budget": view}}
}

func runIntentApprove(inv *intentInvocation) int {
	ids := append([]string(nil), inv.input.args...)
	for _, id := range inv.input.values["id"] {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if len(ids) == 0 {
		return inv.missingTarget(func(file *goal.GoalFile) bool {
			return file.State == goal.StateQueued || file.State == goal.StateParked
		})
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	var files []*goal.GoalFile
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file == nil {
			return unknownGoal(inv, id)
		}
		files = append(files, file)
	}
	box, under := inv.input.text("budget"), inv.input.text("under")
	common := append([]string{"--root", inv.stateRoot}, inv.forward("by", "lineage", "approved-ref", "temporary-human-word", "review-by", "fixture-human-authority", "under")...)
	var args []string
	switch {
	case box == "" || box == "norm":
		// Each goal is approved under its own tier's box in one act.
		var tierless []string
		for _, file := range files {
			if file.Tier == 0 {
				tierless = append(tierless, file.Id)
			}
		}
		if len(tierless) > 0 {
			return inv.render(*tierlessRefusal(inv, tierless))
		}
		args = common
	case box == "keep":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(ids...),
			Summary:  "keep is a goal's own standing box, not an approval box; nothing was done",
			Decision: "approve under each tier's norm (omit --budget), or give the complete compact box"})
	case len(ids) == 1 && under == "":
		// One goal with a box is the budget owner's approval.
		return inv.budgetOwner(ids[0], box, files[0])
	default:
		budget, problem := inv.completeBox(box, files[0])
		if problem != nil {
			return inv.render(*problem)
		}
		args = append(common, budgetLongFlags(budget)...)
	}
	for _, id := range ids {
		args = append(args, "--id", id)
	}
	return inv.callOwner(inv.targets(ids...), func(dependencies syncRequestDependencies) int {
		return runGoalApproveWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies, inv.owners.binding)
	}, func() intentResult {
		result := intentResult{Data: map[string]any{}}
		var approved []string
		goals := []any{}
		for _, id := range ids {
			after := inv.afterGoalAct(id, "approve")
			goals = append(goals, after.Data)
			if data, ok := after.Data.(map[string]any); ok {
				if view, ok := data["budget"].(intentBudgetView); ok {
					approved = append(approved, id+" under "+view.Box)
					continue
				}
			}
			approved = append(approved, id)
		}
		result.Summary = "approved " + strings.Join(approved, ", ")
		result.Data = map[string]any{"goals": goals}
		return result
	})
}

func runIntentPause(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.missingTarget(func(file *goal.GoalFile) bool { return file.State != goal.StateParked })
	}
	reason := inv.input.text("reason")
	if strings.TrimSpace(reason) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "needs the reason the goal is parked; nothing was done", Decision: "say why with --reason TEXT"})
	}
	actor, problem := inv.actorArgs(id, "by", "lineage", "fixture-human-authority")
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--because", reason}, actor...)
	return inv.callOwner(inv.targets(id), func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion("park", args, inv.owners.commandNow, dependencies, inv.owners.parkBranchCheck, inv.owners.completion)
		return code
	}, func() intentResult { return inv.afterGoalAct(id, "pause") })
}

// runIntentResume chooses the owner from the goal's recorded state: a parked
// goal is unparked, and approval is not granted by it; a goal stopped by its
// budget resumes under its standing approved box. Nothing else is resumed.
func runIntentResume(inv *intentInvocation) int {
	if args := inv.input.args; len(args) == 2 && args[0] == "mission" {
		for _, other := range []string{"under", "verified", "by", "temporary-human-word", "review-by", "approved-ref", "fixture-human-authority", "id"} {
			if inv.input.has(other) {
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("resume mission M takes no --%s; nothing was done", other)})
			}
		}
		return runIntentMission(inv, "resume", args[1])
	}
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.missingTarget(func(file *goal.GoalFile) bool {
			return file.State == goal.StateParked || file.IsFencedClaim()
		})
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return inv.render(*problem)
	}
	file, where := goalRecord(projection, id)
	switch {
	case file == nil:
		return unknownGoal(inv, id)
	case where != "live":
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary:  fmt.Sprintf("%s is %s; only a parked or stopped goal resumes; nothing was done", id, where),
			Decision: "reopening an archived goal is its own act with a fresh next step",
			next:     inv.publicArgv("reopen", id, "--next", "TEXT"), nextReason: "reopens the goal under its own authority with the next step it names"})
	case file.State == goal.StateParked:
		if inv.input.has("approved-ref") {
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary: "--approved-ref resumes a goal stopped by its budget; a parked goal's unpark takes none; nothing was done",
				next:    inv.publicArgv("resume", id), nextReason: "the same resume without --approved-ref"})
		}
		if inv.input.has("temporary-human-word") || inv.input.has("review-by") {
			// A relayed word resumes only a goal stopped by its budget; it is
			// never carried into, or silently dropped from, an unpark.
			return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
				Summary:  "--temporary-human-word and --review-by resume a goal stopped by its budget; a parked goal's unpark takes neither; nothing was done",
				Decision: "a person resumes a parked goal at the enrolled terminal, or a seat unparks it under a power of attorney with --under GRANT --verified TEXT"})
		}
		var args []string
		if inv.input.has("under") || inv.input.has("verified") {
			// The seat's own act under a power of attorney: every supplied
			// option goes to the owner, which refuses what cannot combine.
			args = append([]string{"--root", inv.stateRoot, "--id", id}, inv.forward("by", "lineage", "fixture-human-authority", "under", "verified")...)
		} else {
			actor, problem := inv.actorArgs(id, "by", "lineage", "fixture-human-authority")
			if problem != nil {
				return inv.render(*problem)
			}
			args = append([]string{"--root", inv.stateRoot, "--id", id}, actor...)
		}
		return inv.callOwner(inv.targets(id), func(dependencies syncRequestDependencies) int {
			code, _ := trySyncMutationWithCompletion("unpark", args, inv.owners.commandNow, dependencies, inv.owners.parkBranchCheck, inv.owners.completion)
			return code
		}, func() intentResult { return inv.afterGoalAct(id, "resume") })
	case file.IsFencedClaim():
		if inv.input.has("under") || inv.input.has("verified") {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
				Summary:  "a power of attorney covers unparking a parked goal, not resuming a goal stopped by its budget; nothing was done",
				Decision: "a person resumes it at the enrolled terminal: " + shellCommand(inv.publicArgv("resume", id))})
		}
		if file.Budget == nil {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
				Summary:  id + " is stopped and has no standing box to resume under; nothing was done",
				Decision: "a person gives it a box: " + shellCommand(inv.publicArgv("budget", id, "BOX"))})
		}
		args := append([]string{"--root", inv.stateRoot, "--id", id}, budgetLongFlags(*file.Budget)...)
		args = append(args, inv.forward("by", "lineage", "approved-ref", "temporary-human-word", "review-by", "fixture-human-authority")...)
		return inv.callOwner(inv.targets(id), func(dependencies syncRequestDependencies) int {
			return runGoalResumeWithInputs(args, inv.owners.prove, inv.owners.commandNow, dependencies, inv.owners.binding)
		}, func() intentResult { return inv.afterGoalAct(id, "resume") })
	case file.State == goal.StateClaimed:
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: inv.targets(id),
			Summary: fmt.Sprintf("%s is running under its standing box; there is nothing to resume", id)})
	case file.State == goal.StateQueued:
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
			Summary: id + " is queued, not parked or stopped; resuming never grants approval; nothing was done",
			next:    inv.publicArgv("approve", id), nextReason: "approval is a person's own act"})
	}
	return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: inv.targets(id),
		Summary:  fmt.Sprintf("%s is %s, not parked or stopped; nothing was done", id, file.State),
		Decision: "an approved goal waits to be claimed; nothing needs resuming"})
}

func runIntentDone(inv *intentInvocation) int {
	id, problem := inv.singleTarget()
	if problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	if id == "" {
		return inv.missingTarget(func(file *goal.GoalFile) bool { return file.State == goal.StateClaimed })
	}
	reason := inv.input.text("reason")
	if strings.TrimSpace(reason) == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: inv.targets(id),
			Summary: "needs the goal's conclusion; nothing was done", Decision: "state the conclusion with --reason TEXT"})
	}
	actor, problem := inv.actorArgs(id, "by", "lineage")
	if problem != nil {
		return inv.render(*problem)
	}
	args := append([]string{"--root", inv.stateRoot, "--id", id, "--conclude", reason}, actor...)
	return inv.callOwner(inv.targets(id), func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion("done", args, inv.owners.commandNow, dependencies, inv.owners.parkBranchCheck, inv.owners.completion)
		return code
	}, func() intentResult { return inv.afterGoalAct(id, "done") })
}
