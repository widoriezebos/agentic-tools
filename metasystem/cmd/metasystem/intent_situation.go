package main

// system situation prints this seat's shared situation page, reading and
// saving it again when it is older than the steward's freshness: the first
// thing to do, then each part's summary, its detail with --verbose. It acts
// on nothing but the main session's read receipt; --wait waits for a change.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func systemSituationCommand() intentCommand {
	return intentCommand{object: "system", action: "situation", audience: "both", summary: "one page with everything this seat should know, and the first thing to do",
		usage: []string{"metasystem system situation [--verbose] [--json] [--wait DURATION]"}, maxArgs: 0, flags: []intentFlag{intentInstallationFlag,
			{name: "wait", value: "DURATION", usage: "return when something that matters changes, at most DURATION (at most 60m)"}},
		examples: []string{"metasystem system situation"}, run: runIntentSystemSituation}
}

// runIntentSystemSituation reads the installation, where run state lives
// and where the steward writes its alerts, at the command's clock in the
// reader's zone.
func runIntentSystemSituation(inv *intentInvocation) int {
	bound, err := time.ParseDuration(inv.input.text("wait"))
	if inv.input.has("wait") && (err != nil || bound <= 0 || bound > time.Hour) {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "--wait takes a duration up to 60m, such as 10m; nothing was done",
			next: append(inv.typedArgvLess("wait"), "--wait", "60m")})
	}
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
	var details []string
	data, woke := map[string]any{}, ""
	if inv.input.has("wait") {
		// work wait's clock; the page's read never fails, a failed save is a detail.
		options := metarun.WaitOptions{}
		if clock := inv.owners.work.waitClock; clock != nil {
			clock(&options)
		}
		options = metarun.NormalizeWaitOptions(options)
		page := func(at time.Time, fresh time.Duration) (steward.Composition, error) {
			record, saveErr = steward.SituationPage(root, read, at.In(now.Location()), fresh)
			return record.Composition, nil
		}
		if _, data["woke"], err = steward.WaitSituation(context.Background(), page, record.Composition, options.Now, options.Sleep, bound, steward.SituationPoll); err != nil {
			details = append(details, "the wait ended early: "+err.Error())
		}
		if woke, now = data["woke"].(string), options.Now().In(now.Location()); !strings.HasPrefix(woke, "nothing") {
			woke = "woke: " + woke
		}
	}
	holder := inv.owners.dependencies.claimHolder
	if caller, err := holder.Classify(root, waitCallerPID()); err != nil || caller.Holder && caller.Class == lease.ClassMain {
		view, readErr := holder.ReadHolder(root)
		if err = errors.Join(err, readErr); err == nil {
			err = steward.RecordSituationRead(root, view.SessionId, now, record.Composition.First)
		}
		if err != nil {
			details = append(details, "the read receipt was not written: "+err.Error())
		}
	}
	if saveErr != nil {
		details = append(details, "the shared page was not saved: "+saveErr.Error())
	}
	data["composition"], data["situation"] = record.Composition, record.Situation
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: inv.checkoutTarget(scope), Summary: "the situation of " + record.Situation.Machine,
		Data: data, Details: details, view: situationView(record.Composition, now, woke)})
}

// situationView prints the first thing at the top and each part as a
// section; a part's detail only with --verbose, else how many lines it holds.
func situationView(page steward.Composition, now time.Time, woke string) func(*textui.Page) {
	return func(p *textui.Page) {
		env := p.Env()
		env.Now = now
		if woke != "" {
			p.Headline(woke)
		}
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
