package main

// system situation prints this seat's shared situation page, reading and
// saving it again when it is older than the steward's freshness: the first
// thing to do, then each part's summary, its detail with --verbose. It acts
// on nothing.

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func systemSituationCommand() intentCommand {
	return intentCommand{object: "system", action: "situation", audience: "both", summary: "one page with everything this seat should know, and the first thing to do",
		usage: []string{"metasystem system situation [--verbose] [--json]"}, flags: []intentFlag{intentInstallationFlag}, maxArgs: 0,
		examples: []string{"metasystem system situation"}, run: runIntentSystemSituation}
}

// runIntentSystemSituation reads the installation, where run state lives
// and where the steward writes its alerts, at the command's clock in the
// reader's zone.
func runIntentSystemSituation(inv *intentInvocation) int {
	scope, _, problem := inv.selectProcessScope()
	if problem != nil {
		return inv.render(*problem)
	}
	root := scope.Installation.Path()
	now, err := inv.owners.commandNow(root)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: inv.checkoutTarget(scope), Summary: "this checkout's clock could not be read, so nothing was read",
			retry: "try again", Details: []string{"clock: " + err.Error()}})
	}
	if zone := inv.textEnv(inv.stdout).Zone; zone != nil {
		now = now.In(zone)
	}
	src := inv.owners.situation
	if src.Now == nil {
		src.Now = func() time.Time { return now }
	}
	now = src.Now()
	read := func(at time.Time) steward.Situation {
		src.Now = func() time.Time { return at }
		return steward.ReadSituation(root, situationSources(inv.owners, src))
	}
	record, saveErr := steward.SituationPage(root, read, now, steward.SituationFresh)
	result := intentResult{Outcome: intentConfirmed, Targets: inv.checkoutTarget(scope), Summary: "the situation of " + record.Situation.Machine,
		Data: map[string]any{"composition": record.Composition, "situation": record.Situation}, view: situationView(record.Composition, now)}
	if saveErr != nil {
		result.Details = []string{"the shared page was not saved: " + saveErr.Error()}
	}
	return inv.render(result)
}

// situationView prints the first thing at the top and each part as a
// section; a part's detail only with --verbose, else how many lines it holds.
func situationView(page steward.Composition, now time.Time) func(*textui.Page) {
	return func(p *textui.Page) {
		env := p.Env()
		env.Now = now
		p.Headline("The situation of "+cmp.Or(page.Machine, "this seat"), "read "+env.Time(page.ReadAt)+", "+env.Ago(page.ReadAt))
		for _, part := range page.Parts {
			title := strings.ToUpper(part.Name[:1]) + part.Name[1:]
			if part.Name == "first" {
				title = "First thing"
			}
			section := p.Section(title, "")
			// A reader's error may span lines; the page prints each as one.
			for _, line := range part.Summary {
				section.Text(strings.ReplaceAll(line, "\n", " "))
			}
			if part.Unavailable != "" {
				section.Text("unavailable: " + strings.ReplaceAll(part.Unavailable, "\n", " "))
			}
			switch {
			case p.Verbose():
				for _, line := range part.Detail {
					section.Text(strings.ReplaceAll(line, "\n", " "))
				}
			case len(part.Detail) > 0:
				section.Text(fmt.Sprintf("%d more; metasystem system situation --verbose", len(part.Detail)))
			}
		}
	}
}
