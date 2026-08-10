package dispatch

import (
	"os"
	"strings"
	"time"
)

// BriefMode extracts the working mode a brief declares. A brief must carry
// exactly one filled "Working Mode:" header — none, several, or a template
// placeholder still in place is a silent refusal, and the caller names the
// requirement.
func BriefMode(briefPath string) (string, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return "", silentRefusal(1)
	}
	var values []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Working Mode:") {
			values = append(values, strings.TrimSpace(strings.TrimPrefix(line, "Working Mode:")))
		}
	}
	if len(values) != 1 || values[0] == "" || strings.HasPrefix(values[0], "<") {
		return "", silentRefusal(1)
	}
	return values[0], nil
}

// WriteCapResolution records a non-mission cap decision: the authorized
// minutes, the absolute deadline they imply from now, and the provenance of
// the rule that chose them. Mission caps come from the mission fence instead;
// this is the non-mission authority's receipt.
func WriteCapResolution(output string, capMin int64, rule, origin string) error {
	deadline := time.Now().UTC().Truncate(time.Second).Add(time.Duration(capMin) * time.Minute)
	return writeCompactJSON(output, map[string]any{
		"capMin":      capMin,
		"capDeadline": deadline.Format("2006-01-02T15:04:05Z"),
		"source": map[string]any{
			"rule":        rule,
			"origin":      origin,
			"truncatedBy": nil,
		},
	})
}
