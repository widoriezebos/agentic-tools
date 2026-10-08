package plain

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
)

// ReturnOrigin preserves the act and the comparable failure across hand-ins.
// An absent person is an automatic return, including legacy queue lines.
type ReturnOrigin struct {
	Policy   PolicyValue    `json:"policy"`
	Warning  string         `json:"warning,omitempty"`
	Person   *ActProvenance `json:"person,omitempty"`
	Attempt  string         `json:"attempt,omitempty"`
	Class    string         `json:"class"`
	Failures []string       `json:"failures,omitempty"`
}

func returnOrigin(cause *Cause, detail *conflict.Return, person *ActProvenance) ReturnOrigin {
	origin := ReturnOrigin{Person: person, Class: "tests"}
	if cause != nil {
		origin.Failures = slices.Clone(cause.Tests)
		origin.Attempt = cause.Evidence
	}
	if detail != nil {
		origin.Class, origin.Failures = "conflict", nil
		for _, path := range detail.Paths {
			origin.Failures = append(origin.Failures, path.Path+":"+path.Class+":"+path.Resolution)
		}
	} else if len(origin.Failures) == 0 {
		origin.Class = "design"
		if cause != nil && cause.Evidence != "" {
			origin.Failures = []string{cause.Evidence}
		}
	}
	return origin
}

// checkReturnLocked covers every automatic route into the queue writer.
func checkReturnLocked(install string, latest Entry, cause *Cause, detail *conflict.Return, now time.Time, seams ProveSeams) (ReturnOrigin, error) {
	origin := returnOrigin(cause, detail, seams.Person)
	origin.Policy = PolicyValue{Value: "auto", Source: "default"}
	var readErr error
	if seams.Policy != nil {
		origin.Policy, readErr = seams.Policy("landing.on-red")
	}
	if readErr != nil {
		origin.Warning = "the red policy cannot be read: " + readErr.Error()
	}
	if origin.Person != nil {
		act := *origin.Person
		act.Kind = "return"
		act.Subject = []GoalSHA{{Goal: latest.Goal, SHA: latest.SHA}}
		origin.Person = &act
		return origin, nil
	}
	reason := ""
	if cause == nil || cause.Kind != "own" || cause.Goal != latest.Goal || cause.SHA != latest.SHA {
		reason = "no matching own-defect evidence permits an automatic return"
	}
	if readErr != nil || origin.Policy.Value == "person" {
		reason = "the return needs a person under the current red policy"
	}
	lines, skipped, err := countedLines[Line](queuePath(install))
	if err != nil {
		return origin, err
	}
	if skipped != 0 {
		reason = "the return history contains unreadable lines"
	}
	count := 0
	var previous []string
	for _, line := range lines {
		if line.Goal != latest.Goal || line.Outcome != StateReturned {
			continue
		}
		prior := line.ReturnOrigin
		if prior != nil && prior.Person != nil {
			continue
		}
		count++
		if prior == nil {
			derived := returnOrigin(line.Cause, line.Conflict, nil)
			prior = &derived
		}
		if prior.Class != origin.Class {
			continue
		}
		previous = prior.Failures
		if len(previous) == 0 || len(origin.Failures) == 0 {
			reason = "this failure class has no comparable progress evidence"
			continue
		}
		if slices.ContainsFunc(origin.Failures, func(f string) bool { return slices.Contains(previous, f) }) {
			reason = "a failing test or conflict repeats an earlier return"
		}
		if len(origin.Failures) >= len(previous) {
			reason = "the known failures have not strictly shrunk"
		}
	}
	if count >= 2 {
		reason = "this goal has already used its two automatic returns"
	}
	if reason == "" {
		return origin, nil
	}
	kind := "unclassified"
	if cause != nil && cause.Valid() {
		kind = cause.Kind
	}
	stop := Stop{Loop: "lane-return", Subject: latest.Goal, Tree: latest.SHA, Attempt: count + 1, Budget: 2, Class: reason, Decision: "stop", Handoff: "ask lane", Cause: cause, Evidence: queuePath(install), At: now.UTC().Format(time.RFC3339Nano), Required: []string{"metasystem", "landing", "return", latest.Goal, "--cause", kind, "--reason", "TEXT"}}
	stop.Measure.Name, stop.Measure.Previous, stop.Measure.Now = "return failures", previous, origin.Failures
	open, err := OpenStops(install)
	if err != nil {
		return origin, err
	}
	if !slices.ContainsFunc(open, func(s Stop) bool {
		return s.Loop == stop.Loop && s.Subject == stop.Subject && s.Tree == stop.Tree && s.Class == stop.Class
	}) {
		if err := appendLine(stopsPath(install), stop); err != nil {
			return origin, err
		}
	}
	return origin, &Refusal{Code: "LANE_RETURN_PERSON", Reason: fmt.Sprintf("goal %s stays waiting: %s", latest.Goal, reason), Next: strings.Join(stop.Required, " ")}
}
