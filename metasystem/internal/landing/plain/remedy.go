package plain

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func StopHeadline(stop Stop) string {
	why := stop.Subject + " needs a person's decision"
	if stop.Cause != nil {
		why += "; cause: " + stop.Cause.Kind
		if stop.Cause.Name != "" {
			why += "; " + stop.Cause.Name
		}
		if len(stop.Cause.Tests) > 0 {
			why += "; failing tests: " + strings.Join(stop.Cause.Tests, ", ")
		}
	}
	if stop.Cause != nil && stop.Cause.Kind == "main" && stop.Cause.Name != "" {
		why = "main is red (incident " + stop.Cause.Name + "): land a fix on main, then metasystem landing prove --trunk"
		if len(stop.Cause.Tests) > 0 {
			why += "; failing tests: " + strings.Join(stop.Cause.Tests, ", ")
		}
		for _, subject := range strings.Split(stop.Subject, ", ") {
			if subject != "main" && subject != "lane" {
				why += "; or metasystem landing land-anyway " + subject + " --reason TEXT (" + lane.LandAnywayImpact(subject, stop.Cause.Name) + ")"
			}
		}
		return "the lane stopped: " + why
	}
	command := lane.Remedy(stop.Command())
	if strings.Contains(command, "landing prove --classify ") {
		why = strings.Replace(why, stop.Subject+" needs a person's decision", "classify the failed check", 1)
	}
	return "the lane stopped: " + why + "; run: " + command
}

// LaneStatusRemedy is absent when the lane is working without a person's act.
func LaneStatusRemedy(view lane.View, proving bool, headline string) string {
	if proving || strings.HasSuffix(headline, "; nothing to do") {
		return ""
	}
	if at := strings.Index(headline, "metasystem question show "); at >= 0 {
		return strings.SplitN(headline[at:], ";", 2)[0]
	}
	if at := strings.Index(headline, "metasystem landing "); at >= 0 {
		return lane.Remedy(strings.SplitN(headline[at:], ";", 2)[0])
	}
	if len(view.Owner.Fix) > 0 {
		return lane.Remedy(strings.Join(view.Owner.Fix, " "))
	}
	if view.Owner.State == lane.OwnerStopped {
		return "metasystem landing start"
	}
	if view.Owner.State == lane.OwnerHeld || view.Owner.State == lane.OwnerUnready {
		return "metasystem landing resume"
	}
	return ""
}
