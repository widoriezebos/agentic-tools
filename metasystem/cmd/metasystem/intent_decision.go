package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

func runIntentDecisionRecord(inv *intentInvocation) int {
	refuse := func(words string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: words + "; nothing was done"})
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
	if inv.input.has("lineage") || inv.owners.dependencies.ownerLineage != nil && inv.owners.dependencies.ownerLineage() != "" {
		return refuse("an agent's decision record needs the grant check")
	}
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	actor, proof, denied := inv.actingAs("decision", "", actorHuman)
	if denied != nil {
		return inv.render(*denied)
	}
	if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
		denied = inv.personRefusal("", humanauthority.Refused(humanauthority.OutcomeTerminalMissing, nil), "")
		denied.code = 2
		return inv.render(*denied)
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
	envelope := inv.delivery().ownerEnvelope
	if envelope == nil {
		envelope = runIntentOwnerEnvelope
	}
	child, readErr := envelope(intentProcess{argv: argv, dir: inv.cwd}, command.name)
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
	by := actorValue(actor, "--by")
	d := humanauthority.Decision{ID: id, At: now.UTC(), Actor: "human:" + by, By: by, Machine: machine,
		Class: inv.input.text("class"), Text: inv.input.text("text"), Rejected: inv.input.text("rejected"), Undo: inv.input.text("undo"), Context: inv.input.text("context"),
		Argv: typed, Outcome: child.Outcome, Words: child.Summary, Effect: rawEffect}
	result := intentResult{Outcome: child.Outcome, code: child.Exit, Summary: "decision " + d.ID + " recorded: " + child.Summary,
		Targets: []intentTarget{{Kind: "decision", ID: d.ID}}, Data: map[string]any{"decision": d}}
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
