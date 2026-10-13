package plain

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func stopDescription(stop Stop) *state.Stop {
	r := &state.Stop{Subject: stop.Subject, Command: stop.Command()}
	if stop.Cause != nil {
		r.Kind, r.Name, r.Tests = stop.Cause.Kind, stop.Cause.Name, stop.Cause.Tests
	}
	return r
}
func StopHeadline(stop Stop) string {
	return state.Describe(state.Record{Stop: stopDescription(stop)}, state.RenderInputs{}).Headline
}

// LaneStatusRemedy is absent when the lane is working without a person's act.
func LaneStatusRemedy(view lane.View, proving bool, headline string) string {
	if proving || strings.HasSuffix(headline, "; nothing to do") {
		return ""
	}
	if len(view.Owner.Fix) > 0 {
		return lane.Remedy(strings.Join(view.Owner.Fix, " "))
	}
	if at := strings.Index(headline, "metasystem question show "); at >= 0 {
		return strings.SplitN(headline[at:], ";", 2)[0]
	}
	if at := strings.Index(headline, "metasystem landing "); at >= 0 {
		return lane.Remedy(strings.SplitN(headline[at:], ";", 2)[0])
	}
	d := state.Describe(view.DescriptionRecord(false), state.RenderInputs{})
	if len(d.Remedies) > 0 {
		return d.Remedies[0]
	}
	return ""
}
