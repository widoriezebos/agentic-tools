package main

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func landingBatchCommand() intentCommand {
	return laneCommand(intentCommand{object: "landing", action: "batch", audience: "both", maxArgs: 1,
		summary:  "show the batch, seal its window, or reopen it with a waiting goal",
		usage:    []string{"metasystem landing batch", "metasystem landing batch seal [--by NAME]", "metasystem landing batch reopen --with G [--by NAME]"},
		flags:    []intentFlag{{name: "with", value: "G", usage: "the waiting goal to admit"}, {name: "by", value: "NAME", usage: "the enrolled person's name"}},
		examples: []string{"metasystem landing batch", "metasystem landing batch seal", "metasystem landing batch reopen --with my-goal"},
	}, runIntentLandingBatch)
}

func landingRestartCommand() intentCommand {
	return laneCommand(intentCommand{object: "landing", action: "restart", audience: "both", maxArgs: 0,
		summary:  "restart the proof and reopen the batch with every waiting goal",
		usage:    []string{"metasystem landing restart [--by NAME]"},
		details:  []string{"Records the person's decision. If clean cancellation is unavailable, the request is kept for the keeper and the running proof continues."},
		flags:    []intentFlag{{name: "by", value: "NAME", usage: "the enrolled person's name"}},
		examples: []string{"metasystem landing restart", "metasystem landing restart --by Wido"},
	}, runIntentLandingBatch)
}

func runIntentLandingBatch(inv *intentInvocation, admitted laneAdmitted) int {
	kind := ""
	if inv.command.action == "restart" {
		kind = "restart"
	} else if len(inv.input.args) > 0 {
		kind = inv.input.args[0]
	}
	if kind != "" && kind != "seal" && kind != "reopen" && kind != "restart" || inv.command.action == "batch" && kind == "restart" || kind == "reopen" && strings.TrimSpace(inv.input.text("with")) == "" || kind != "reopen" && inv.input.has("with") {
		return inv.render(intentResult{Outcome: intentFailed, code: 2, Summary: "use landing batch, landing batch seal, landing batch reopen --with G, or landing restart"})
	}
	seams := admitted.owners.plainProve
	seams.Now = admitted.owners.now
	seams.Pause = func() (lane.Pause, bool) { return lane.ReadPause(admitted.home) }
	batch, err := plain.ReadBatch(admitted.installation)
	message := ""
	if err == nil && kind != "" {
		person, problem := inv.lanePerson("decide the batch's admission window", admitted.record.Root)
		if problem != nil {
			return inv.render(*problem)
		}
		provenance := plain.ActProvenance{Person: person.Name, Root: person.Root, CheckedAt: person.At, TerminalGeneration: person.Proof.TerminalGeneration, TerminalRef: person.Proof.TerminalRef, Destination: admitted.record}
		batch, message, err = plain.ActOnBatch(admitted.installation, kind, strings.TrimSpace(inv.input.text("with")), provenance, seams)
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the batch could not be read or changed: " + oneLine(err.Error())})
	}
	entries, err := plain.Entries(admitted.installation)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the queue could not be read: " + oneLine(err.Error())})
	}
	lines, err := plain.BatchLines(batch, entries, string(admitted.layout.Checkout), seams)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the batch's merge state could not be read: " + oneLine(err.Error()), Data: batch})
	}
	if message != "" {
		lines = append([]string{message}, lines...)
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: strings.Join(lines, "\n"), Data: batch})
}
