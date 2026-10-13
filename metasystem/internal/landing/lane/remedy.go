package lane

import "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"

func Remedy(command string) string {
	d := state.Describe(state.Record{RemedyCommand: command}, state.RenderInputs{})
	if len(d.Remedies) > 0 {
		return d.Remedies[0]
	}
	return command
}
func LandAnywayImpact(goal, incident string) string { return state.LandAnywayImpact(goal, incident) }
