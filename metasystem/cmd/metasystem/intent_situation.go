package main

// system situation reads this seat's situation once, composes it once and
// prints the page: the first thing to do, then each part's summary, its
// detail with --verbose. It acts on nothing and writes nothing.

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
	s := steward.ReadSituation(root, situationSources(inv.owners, src))
	page := steward.Compose(s)
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: inv.checkoutTarget(scope), Summary: "the situation of " + s.Machine,
		Data: map[string]any{"composition": page, "situation": s}, view: situationView(page)})
}

// situationView prints the first thing at the top and each part as a
// section; a part's detail only with --verbose, else how many lines it holds.
func situationView(page steward.Composition) func(*textui.Page) {
	return func(p *textui.Page) {
		p.Headline("The situation of "+cmp.Or(page.Machine, "this seat"), "read "+p.Env().Time(page.ReadAt))
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
