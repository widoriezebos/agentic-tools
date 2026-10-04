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
	write     config.RosterWriter
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
		{
			object: "roster", action: "set", audience: "human", summary: "set the agent, model and effort of one roster row on this computer",
			usage:    []string{"metasystem roster set ROSTER ROW RUNTIME:MODEL:EFFORT"},
			maxArgs:  3,
			examples: []string{"metasystem roster set tier-1 build codex:gpt-6-sol:xhigh"},
			run:      runIntentRosterSet,
		},
	}
}

func runIntentRosterSet(inv *intentInvocation) int {
	if len(inv.input.args) != 3 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "roster set needs a roster, a row and its value; nothing was changed",
			next: inv.publicArgv("roster", "set", "ROSTER", "ROW", "RUNTIME:MODEL:EFFORT")})
	}
	if problem := inv.resolveLayout(); problem != nil {
		return inv.render(*problem)
	}
	if problem := inv.directPersonProof("roster set"); problem != nil {
		return inv.render(*problem)
	}
	owners := inv.owners.rosters
	if owners.home == nil {
		owners.home = board.Home
	}
	confPath := filepath.Join(inv.layout.InstallationRoot, "metasystem.conf")
	listed, _, err := config.Get(config.GetParams{Key: "metasystem.runtimes", ConfPath: confPath, LookupEnv: owners.lookupEnv})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error() + "; nothing was changed"})
	}
	home, err := owners.home()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error() + "; nothing was changed"})
	}
	roster, row, value := inv.input.args[0], inv.input.args[1], inv.input.args[2]
	runtimes := config.RuntimeNames(listed)
	changed, err := owners.write.SetRow(home, roster, row, value, runtimes, func(runtime, model string) (string, error) {
		canonical, _, err := config.ResolveModelAlias(confPath, runtime, model)
		return canonical, err
	})
	if err != nil && !errors.Is(err, config.ErrRosterNotDurable) {
		code := 1
		// An input refusal wants another roster, row or value; file, home and
		// resolver failures need repair instead.
		if errors.Is(err, config.ErrRosterInput) {
			code = 2
		}
		return inv.render(intentResult{Outcome: intentRefused, code: code, Summary: err.Error()})
	}
	fields := append(strings.Split(value, ":"), "", "")
	data := map[string]any{"roster": roster, "row": row, "changed": changed, "runtime": fields[0], "model": fields[1], "effort": fields[2]}
	shown := strings.ReplaceAll(value, ":", " ")
	if !changed {
		return inv.render(intentResult{Outcome: intentUnchanged, Summary: roster + " " + row + " already is " + shown + "; nothing was changed.", Data: data})
	}
	use := "the next " + row + " of a " + roster + " goal uses it"
	switch roster {
	case "tier-3":
		use = "the next " + row + " of a tier-3 goal, or of work with no goal, uses it"
	case "seat":
		use = "the next seat agent uses it"
		if row == "steward" {
			use = "the next steward continuation uses it"
		}
	case "landing":
		use = "the next landing agent uses it"
	case "partner":
		use = "the Project Partner uses it from its next turn"
	}
	if value == "main" {
		use = "the calling session does that work itself"
	}
	summary := roster + " " + row + " is now " + shown + " on this computer; " + use + "."
	if errors.Is(err, config.ErrRosterNotDurable) {
		summary += "\nit is written, but this computer could not confirm it is on disk; metasystem roster show " + roster + " checks it after a restart"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, Data: data})
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
			Decision: "a person repairs or removes the file; metasystem roster set refuses to write over it", Details: []string{refused.Code}})
	} else if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
	}
	summary := "the rosters on this computer"
	if id != "" {
		views = slices.DeleteFunc(views, func(view config.RosterView) bool { return view.ID != id })
		summary = "roster " + id + " on this computer"
	}
	runtimes, lines := config.RuntimeNames(listed), []string(nil)
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
