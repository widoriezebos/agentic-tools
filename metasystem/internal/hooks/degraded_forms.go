package hooks

import (
	"fmt"
	"strings"
)

// The degraded Stop forms are the fixed provider payloads a Stop may return
// when no verdict can be produced. They depend on no engine state, no file and
// no environment: the forms are the last word when everything else failed.
// This table is their one source. The runtime hook renders them directly, the
// rendered settings command prints the engine-missing form when no engine is
// installed, and the shipped Claude Stop launcher carries the bootstrap form;
// tests compare each copy with this renderer and the golden contract.

type degradedCause struct {
	key, text, remedy string
	qualifiers        []string
}

var degradedCauses = []degradedCause{
	{"unreadable-output", "stop-hook-output-was-unreadable", "The steward must restore supervision.", []string{"condition-log-failed", "no-resolved-checkout"}},
	{"engine-missing", "engine missing", "Rebuild it with go run ./cmd/devgate build.", nil},
	{"engine-skew", "engine does not answer path state-root", "Rebuild it with go run ./cmd/devgate build.", nil},
	{"staging-failed", "payload staging failed", "The steward must restore supervision.", nil},
	{"deadline-expired", "stop deadline expired", "The steward must restore supervision.", []string{"record-update-failed", "condition-log-failed", "no-resolved-checkout"}},
	{"bootstrap-failed", "hook-bootstrap-failed", "The steward must restore supervision.", nil},
	{"bare", "", "", nil},
}

var degradedQualifiers = []struct{ key, text string }{
	{"record-update-failed", "record update failed"},
	{"condition-log-failed", "condition log failed"},
	{"no-resolved-checkout", "no resolved checkout"},
}

// DegradedStopForm renders one fixed Stop payload (without a trailing
// newline). outcome is "allowed" or "blocked"; only the bare cause may block.
// Qualifiers must be admitted by the cause; they render in table order and
// duplicates collapse.
func DegradedStopForm(outcome, cause string, qualifiers ...string) (string, error) {
	var outcomeWord string
	switch outcome {
	case "allowed", "blocked":
		outcomeWord = outcome
	default:
		return "", fmt.Errorf("degraded Stop form: unknown outcome %q", outcome)
	}
	index := -1
	for position, candidate := range degradedCauses {
		if candidate.key == cause {
			index = position
			break
		}
	}
	if index < 0 {
		return "", fmt.Errorf("degraded Stop form: unknown cause %q", cause)
	}
	if outcome == "blocked" && cause != "bare" {
		return "", fmt.Errorf("degraded Stop form: only the bare form blocks")
	}
	selected := degradedCauses[index]
	requested := make([]bool, len(degradedQualifiers))
	for _, qualifier := range qualifiers {
		found := -1
		for position, candidate := range degradedQualifiers {
			if candidate.key == qualifier {
				found = position
				break
			}
		}
		if found < 0 {
			return "", fmt.Errorf("degraded Stop form: unknown qualifier %q", qualifier)
		}
		admitted := false
		for _, allowed := range selected.qualifiers {
			if allowed == qualifier {
				admitted = true
				break
			}
		}
		if !admitted {
			return "", fmt.Errorf("degraded Stop form: cause %q does not admit qualifier %q", cause, qualifier)
		}
		requested[found] = true
	}
	var qualifierText strings.Builder
	for position, want := range requested {
		if want {
			qualifierText.WriteString("; " + degradedQualifiers[position].text)
		}
	}
	message := "Task unknown; Stop " + outcomeWord + "; needs supervision repair;"
	if cause == "bare" {
		message += " Status unavailable."
	} else {
		message += " " + selected.text + qualifierText.String() + ". " + selected.remedy + " Status unavailable."
	}
	if outcome == "blocked" {
		return `{"decision":"block","reason":"` + message + `"}`, nil
	}
	return `{"systemMessage":"` + message + `"}`, nil
}

// mustDegradedStopForm renders a form the hook itself names; the table
// admits every call site, so a failure is a programming error.
func mustDegradedStopForm(outcome, cause string, qualifiers ...string) string {
	form, err := DegradedStopForm(outcome, cause, qualifiers...)
	if err != nil {
		panic(err)
	}
	return form
}
