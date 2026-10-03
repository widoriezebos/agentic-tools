package main

// The roster object: which agent, model and effort does each kind of work, from the file every checkout shares.

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// rosterOwners are the roster verbs' seams; the zero value is production.
type rosterOwners struct {
	home      func() (string, error)      // the registry home whose host folder holds the rosters file
	lookupEnv func(string) (string, bool) // nil reads the process's environment
}

func rosterIntentCommands() []intentCommand {
	return []intentCommand{
		{
			object: "roster", action: "list", audience: "both", summary: "every roster on this computer, with the agent, model and effort of each row",
			usage:    []string{"metasystem roster list"},
			maxArgs:  0,
			examples: []string{"metasystem roster list"},
			run:      func(inv *intentInvocation) int { return runIntentRosters(inv, "") },
		},
		{
			object: "roster", action: "show", audience: "both", summary: "one roster: the agent, model and effort of each of its rows",
			usage:    []string{"metasystem roster show ROSTER"},
			maxArgs:  1,
			examples: []string{"metasystem roster show tier-1"},
			run: func(inv *intentInvocation) int {
				ids, id, subject := config.RosterIDs(), strings.Join(inv.input.args, ""), "roster show names one roster"
				if slices.Contains(ids, id) {
					return runIntentRosters(inv, id)
				} else if id != "" {
					subject = id + " is not a roster"
				}
				return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: subject + "; the rosters are " + strings.Join(ids[:len(ids)-1], ", ") + " and " + ids[len(ids)-1] + "; nothing was read",
					next: inv.publicArgv("roster", "list"), nextReason: "lists every roster and its rows"})
			},
		},
	}
}

// runIntentRosters prints every roster, or only id, row by row; an unset row reads "not set", never a value.
func runIntentRosters(inv *intentInvocation, id string) int {
	owners := inv.owners.rosters
	if owners.home == nil {
		owners.home = board.Home
	}
	path := inv.helmPath()
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err != nil {
		return inv.render(*inv.notARepository(path, err))
	}
	listed, _, err := config.Get(config.GetParams{Key: "metasystem.runtimes", ConfPath: filepath.Join(layout.InstallationRoot, "metasystem.conf"), LookupEnv: owners.lookupEnv})
	home, homeErr := owners.home()
	if err == nil {
		err = homeErr
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error() + "; nothing was read"})
	}
	views, err := config.Rosters(home)
	var refused *config.RosterRefusal
	if errors.As(err, &refused) {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "The rosters file " + config.RostersPath(home) + " can't be used: " + refused.Problem + ". Nothing was read.",
			Decision: "a person repairs or removes the file; setting a row refuses to write over it", Details: []string{refused.Code}})
	} else if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
	}
	summary := "the rosters on this computer"
	if id != "" {
		views = slices.DeleteFunc(views, func(view config.RosterView) bool { return view.ID != id })
		summary = "roster " + id + " on this computer"
	}
	runtimes, lines := strings.FieldsFunc(listed, func(r rune) bool { return r == ',' || r == ' ' }), []string(nil)
	for _, view := range views {
		lines = append(lines, fmt.Sprintf("%s (type %s)", view.ID, view.Type))
		for _, row := range view.Rows {
			lines = append(lines, "  "+rosterLine(row, runtimes))
		}
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, text: lines,
		Data: map[string]any{"rosters": views}, Details: []string{"read from " + config.RostersPath(home)}})
}

// rosterLine is one row as a person reads it: "build  claude  claude-opus-5-5  xhigh".
func rosterLine(row config.RosterLine, runtimes []string) string {
	if !row.Set {
		return row.Row + "  not set"
	}
	fields := slices.DeleteFunc([]string{row.Row, row.Runtime, row.Model, row.Effort}, func(field string) bool { return field == "" })
	if row.Runtime != "main" && !slices.Contains(runtimes, row.Runtime) {
		fields = append(fields, "(this installation does not run "+row.Runtime+")")
	}
	return strings.Join(fields, "  ")
}
