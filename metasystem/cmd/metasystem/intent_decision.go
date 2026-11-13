package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

type decisionOwners struct {
	home  func() (string, error)
	grant func(root string, pid int64, now time.Time, argv []string) (attorneyDecision, bool)
}

func runIntentDecisionRecord(inv *intentInvocation) int {
	refuse := func(words string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: words + "; nothing was done"})
	}
	agent := inv.input.has("lineage") || inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != ""
	if inv.input.has("overrules") {
		if agent {
			return refuse("an overrule is a person's decision; an agent records its own decision instead")
		}
		if strings.TrimSpace(inv.input.text("overrules")) == "" {
			return refuse("an overrule needs --overrules ID")
		}
		if !inv.input.has("rejected") {
			inv.input.values["rejected"] = []string{"decision " + inv.input.text("overrules") + " as recorded"}
		}
	}
	disposition, dispositionProblem := decisionDisposition(inv)
	if dispositionProblem != "" {
		return refuse(dispositionProblem)
	}
	if disposition != nil && disposition.Kind != humanauthority.DisposeAct || disposition == nil && inv.input.has("overrules") && len(inv.input.args) == 0 {
		return runIntentDecisionDisposition(inv, disposition)
	}
	separator := slices.Index(inv.raw, "--")
	if separator < 0 || separator+1 == len(inv.raw) {
		return refuse("decision record needs the act after --")
	}
	typed := inv.raw[separator+1:]
	if len(inv.input.args) != len(typed) {
		return refuse("decision record takes no positional words before --")
	}
	if len(typed) < 2 {
		return refuse("the act must name a public command with two verb words")
	}
	command, ok := findIntentAction(typed[0], typed[1])
	if !ok || command.hidden || command.name == "decision record" {
		return refuse("the act must name a public command other than decision record")
	}
	for _, key := range []string{"text", "rejected", "undo"} {
		if strings.TrimSpace(inv.input.text(key)) == "" {
			return refuse("decision record needs --" + key + " TEXT")
		}
	}
	input, problem := parseIntentArgs(command, typed[2:])
	if problem != nil {
		return refuse(problem.summary)
	}
	if input.help {
		return refuse("the act asks for --help; choose the act to run")
	}
	if input.has("json") && !input.switched("json") {
		return refuse("decision record reads the act's JSON result; drop --json=false")
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	by := ""
	if agent {
		if !slices.Contains(goal.AttorneyClasses, inv.input.text("class")) {
			return refuse("an agent's --class must be one of " + strings.Join(goal.AttorneyClasses, ", "))
		}
		if inv.input.has("repo") || input.has("repo") {
			return refuse("an agent records from its own checkout")
		}
	} else {
		actor, proof, denied := inv.actingAs("decision", "", actorHuman)
		if denied != nil {
			return inv.render(*denied)
		}
		if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
			denied = inv.personRefusal("", humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil), "")
			denied.code = 2
			return inv.render(*denied)
		}
		by = actorValue(actor, "--by")
	}
	extra := []string{}
	if !input.has("json") {
		extra = append(extra, "--json")
	}
	if input.has("repo") {
		layout, err := inv.owners.resolver.ResolveLayout(inv.inputPath(input.text("repo")))
		if err != nil || layout.GitRoot != inv.layout.GitRoot {
			return refuse("decision record cannot name two repositories")
		}
	} else if inv.input.has("repo") {
		extra = append(extra, "--repo", inv.layout.GitRoot)
	}
	now, err := inv.owners.commandNow(inv.layout.InstallationRoot.Path())
	if err != nil {
		return refuse("the clock could not be read: " + err.Error())
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		return refuse("the machine could not be read: " + err.Error())
	}
	home := inv.owners.decisions.home
	if home == nil {
		home = board.Home
	}
	dir, err := home()
	if err != nil {
		return refuse("the decision log's home could not be read: " + err.Error())
	}
	binary, err := inv.delivery().executable()
	if err != nil {
		return refuse("the running engine's path could not be read: " + err.Error())
	}
	id, err := project.NewID()
	if err != nil {
		return refuse("the decision id could not be made: " + err.Error())
	}
	argv := append(append(append([]string{binary}, typed[:2]...), extra...), typed[2:]...)
	process := intentProcess{argv: argv, dir: inv.cwd}
	var admitted attorneyDecision
	if agent {
		grant := inv.owners.decisions.grant
		if grant == nil {
			grant = (&attorneyAdmitter{}).grantFor
		}
		var ok bool
		admitted, ok = grant(inv.stateRoot, int64(os.Getppid()), now, argv[1:])
		wanted := inv.input.text("class")
		if !ok {
			return refuse("no live grant admits " + command.name + " as " + wanted + "; run it as yourself; a decision record is an act in a person's name")
		}
		if admitted.act == "" {
			return refuse(command.name + " is in no class a grant can carry; wanted " + wanted)
		}
		if admitted.act != wanted {
			return refuse(command.name + " is " + admitted.act + ", not " + wanted)
		}
		if !admitted.entry.Carries(wanted) {
			return refuse(command.name + " is " + admitted.act + "; the grant does not carry " + wanted)
		}
		process.env = append(process.env, "METASYSTEM_DECISION="+id)
		if inv.input.has("lineage") {
			process.env = append(process.env, "METASYSTEM_OWNER_LINEAGE="+inv.input.text("lineage"))
		}
		by = strings.TrimPrefix(admitted.entry.By, "human:")
	}
	impact, denied := decisionOverrule(inv, dir)
	if denied != nil {
		return inv.render(*denied)
	}
	if command.name == "question withdraw" {
		process.env = append(process.env, "METASYSTEM_DECISION_ID="+id)
	}
	envelope := inv.delivery().ownerEnvelope
	if envelope == nil {
		envelope = runIntentOwnerEnvelope
	}
	child, readErr := envelope(process, command.name)
	if readErr != nil || child.Outcome == verbresult.Unknown {
		child.Outcome, child.Exit, child.Summary = verbresult.Failed, 1, "the act returned no readable result"
		if readErr != nil {
			child.Summary = readErr.Error()
		}
	}
	targets := child.Targets
	if targets == nil {
		targets = []verbresult.Target{}
	}
	effect := map[string]any{"targets": targets}
	var data struct {
		Owner map[string]json.RawMessage `json:"owner"`
	}
	if json.Unmarshal(child.Data, &data) == nil && data.Owner != nil {
		effect = map[string]any{"owner": data.Owner}
	}
	rawEffect, _ := json.Marshal(effect)
	d := humanauthority.Decision{ID: id, At: now.UTC(), Actor: "human:" + by, By: by, Machine: machine,
		Class: inv.input.text("class"), Text: inv.input.text("text"), Rejected: inv.input.text("rejected"), Undo: inv.input.text("undo"), Context: inv.input.text("context"),
		Argv: typed, Outcome: child.Outcome, Words: child.Summary, Effect: rawEffect, Overrules: inv.input.text("overrules"), Impact: impact}
	if agent {
		d.Grant, d.Actor, d.Lineage = admitted.entry.ID, admitted.caller.MainId, admitted.entry.Lineage
	}
	if disposition != nil {
		disposition.Decision, d.Disposition = id, disposition
	}
	result := intentResult{Outcome: child.Outcome, code: child.Exit, Summary: "decision " + d.ID + " recorded: " + child.Summary,
		Targets: []intentTarget{{Kind: "decision", ID: d.ID}}, Data: map[string]any{"decision": d}}
	if impact != "" {
		result.Data.(map[string]any)["impact"] = impact
	}
	for _, target := range child.Targets {
		result.Targets = append(result.Targets, intentTarget{Kind: target.Kind, ID: target.ID})
	}
	if err := humanauthority.AppendDecision(dir, d); err != nil {
		raw, _ := json.Marshal(d)
		fmt.Fprintf(inv.stderr, "the act ran (%s) but its decision could not be saved: %v\nunsaved decision: %s\n", child.Outcome, err, raw)
		result.Summary = "the act ran (" + child.Outcome + ") but its decision could not be saved: " + err.Error()
		result.Details = []string{string(raw)}
		if child.Outcome == verbresult.Confirmed {
			result.Outcome, result.code = intentPartial, 1
		}
	}
	return inv.render(result)
}

func decisionDisposition(inv *intentInvocation) (*humanauthority.Disposition, string) {
	keys := []string{"row", "subject", "occurrence", "dispose"}
	if !inv.input.has("dispose") && (inv.input.has("trigger") || inv.input.has("until")) {
		return nil, "--trigger and --until require --dispose wait"
	}
	if !slices.ContainsFunc(keys, inv.input.has) {
		return nil, ""
	}
	for _, key := range keys {
		if strings.TrimSpace(inv.input.text(key)) == "" {
			return nil, "a disposition needs --" + key
		}
	}
	d := &humanauthority.Disposition{Row: inv.input.text("row"), Subject: inv.input.text("subject"), Occurrence: inv.input.text("occurrence"), Kind: inv.input.text("dispose"), Trigger: inv.input.text("trigger")}
	if d.Kind != humanauthority.DisposeAct && d.Kind != humanauthority.DisposeWait && d.Kind != humanauthority.DisposeAsk {
		return nil, "--dispose takes act, wait or ask"
	}
	if d.Kind == humanauthority.DisposeWait {
		for _, key := range []string{"trigger", "until"} {
			if strings.TrimSpace(inv.input.text(key)) == "" {
				return nil, "a wait needs --" + key
			}
		}
	} else if inv.input.has("trigger") || inv.input.has("until") {
		return nil, "--trigger and --until require --dispose wait"
	}
	if d.Kind != humanauthority.DisposeAct && len(inv.input.args) > 0 {
		return nil, "a wait or ask takes no act or positional words"
	}
	return d, ""
}

func runIntentDecisionDisposition(inv *intentInvocation, disposition *humanauthority.Disposition) int {
	refuse := func(words string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: words + "; nothing was done"})
	}
	for _, key := range []string{"text", "rejected"} {
		if strings.TrimSpace(inv.input.text(key)) == "" {
			return refuse("decision record needs --" + key + " TEXT")
		}
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	lineage := inv.input.text("lineage")
	if lineage == "" && inv.owners.dependencies.ownerLineage != nil {
		lineage = inv.owners.dependencies.ownerLineage()
	}
	actor, by := lineage, ""
	if !inv.input.has("lineage") && lineage == "" {
		args, proof, denied := inv.actingAs("decision", "", actorHuman)
		if denied != nil {
			return inv.render(*denied)
		}
		if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
			return refuse("record this decision from your own enrolled terminal")
		}
		by = actorValue(args, "--by")
		actor = "human:" + by
	}
	now, err := inv.owners.commandNow(inv.layout.InstallationRoot.Path())
	if err != nil {
		return refuse("the clock could not be read: " + err.Error())
	}
	if disposition != nil && disposition.Kind == humanauthority.DisposeWait {
		zone := inv.owners.helm.withDefaults().zone
		end, err := parseGrantEnd(now, zone, "", inv.input.text("until"))
		bounds := fmt.Sprintf("choose an end after %s and within 24 hours, by %s", now.In(zone).Format("15:04 MST (2006-01-02)"), now.Add(humanauthority.DispositionMaxWait).In(zone).Format("15:04 MST (2006-01-02)"))
		if err != nil {
			return refuse("--until needs a time like 19:30, tomorrow or a date; given " + inv.input.text("until") + "; " + bounds)
		}
		if !end.After(now) || end.After(now.Add(humanauthority.DispositionMaxWait)) {
			return refuse("--until " + end.In(zone).Format("15:04 MST (2006-01-02)") + ": " + bounds)
		}
		disposition.WaitUntil = end.UTC()
	}
	machine, err := inv.owners.dependencies.machine(inv.stateRoot)
	if err != nil {
		return refuse("the machine could not be read: " + err.Error())
	}
	home := inv.owners.decisions.home
	if home == nil {
		home = board.Home
	}
	dir, err := home()
	if err != nil {
		return refuse("the decision log's home could not be read: " + err.Error())
	}
	id, err := project.NewID()
	if err != nil {
		return refuse("the decision id could not be made: " + err.Error())
	}
	if disposition != nil {
		disposition.Decision = id
	}
	impact, denied := decisionOverrule(inv, dir)
	if denied != nil {
		return inv.render(*denied)
	}
	d := humanauthority.Decision{ID: id, At: now.UTC(), Actor: actor, Lineage: lineage, By: by, Machine: machine, Class: inv.input.text("class"), Text: inv.input.text("text"), Rejected: inv.input.text("rejected"), Context: inv.input.text("context"), Outcome: intentConfirmed, Words: inv.input.text("text"), Effect: json.RawMessage(`{"targets":[]}`), Disposition: disposition, Overrules: inv.input.text("overrules"), Impact: impact}
	if err := humanauthority.AppendDecision(dir, d); err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the disposition could not be saved: " + err.Error()})
	}
	data := map[string]any{"decision": d}
	if impact != "" {
		data["impact"] = impact
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: "decision " + id + " recorded: " + d.Words, Targets: []intentTarget{{Kind: "decision", ID: id}}, Data: data})
}

func decisionOverrule(inv *intentInvocation, home string) (string, *intentResult) {
	id := inv.input.text("overrules")
	if id == "" {
		return "", nil
	}
	lines, _, err := humanauthority.ReadDecisions(home)
	if err != nil {
		return "", &intentResult{Outcome: intentRefused, code: 2, Summary: "the decision log could not be read: " + err.Error() + "; nothing was done"}
	}
	for _, d := range lines {
		if d.ID != id {
			continue
		}
		who := "by " + d.Actor
		if d.By != "" {
			who = "in " + d.By + "'s name"
		}
		if d.Grant != "" {
			who += " under grant " + d.Grant
		}
		if d.Class != "" {
			who += ", class " + d.Class
		}
		impact := fmt.Sprintf("Decision %s (%s, %s)", d.ID, d.At.In(inv.owners.helm.withDefaults().zone).Format("15:04 MST (2006-01-02)"), who)
		if len(d.Argv) == 0 {
			action := "recorded " + d.Text
			if d.Disposition != nil {
				action = fmt.Sprintf("disposed of %s for %s: %s", d.Disposition.Row, d.Disposition.Subject, d.Disposition.Kind)
			}
			impact += " " + action + ". Overruling records your decision; it changes nothing that ran."
		} else {
			var effect struct {
				Owner   *struct{ Tip, Detail string }
				Targets []intentTarget
			}
			_ = json.Unmarshal(d.Effect, &effect)
			shown := "none"
			if effect.Owner != nil {
				shown = effect.Owner.Tip + ": " + effect.Owner.Detail
			} else if len(effect.Targets) > 0 {
				var targets []string
				for _, target := range effect.Targets {
					targets = append(targets, target.Kind+" "+target.ID)
				}
				shown = strings.Join(targets, ", ")
			}
			impact += fmt.Sprintf(" ran %s: %s; %s. Overruling records your decision; it does not undo that act. To undo it: %s.", shellCommand(d.Argv), d.Outcome, shown, d.Undo)
		}
		out := inv.stdout
		if inv.input.switched("json") {
			out = inv.stderr
		}
		if _, err := fmt.Fprintln(out, impact); err != nil {
			return "", &intentResult{Outcome: intentFailed, code: 1, Summary: "the impact could not be shown: " + err.Error() + "; nothing was done"}
		}
		return impact, nil
	}
	return "", &intentResult{Outcome: intentRefused, code: 2, Summary: "the decision log has no decision " + id + "; nothing was done", next: inv.publicArgv("decision", "list"), nextReason: "lists the decisions with their ids"}
}
