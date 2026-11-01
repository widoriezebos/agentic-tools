package plain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// shadowRows compares only facts captured in the same observation.
func shadowRows(status Status) []Fired {
	shadow := status.Shadow
	if len(status.Problems) > 0 {
		from := "no state record yet"
		if shadow != nil {
			from = "shadow from " + shadow.Lane.Since.At.Local().Format("15:04")
		}
		return []Fired{{Row: "observation-failed", Subject: from + "; the observation failed: " + strings.Join(status.Problems, "; "), Act: "the observation at " + status.Raw.Now.Local().Format("15:04") + " failed"}}
	}
	if shadow == nil {
		return []Fired{{Row: "drift", Subject: "no state record yet"}}
	}
	rebuilt := state.Carry(shadow, ShadowOf(status, status.Raw))
	var rows []Fired
	laneRow := -1
	for _, difference := range state.Diff(*shadow, rebuilt) {
		field, values, _ := strings.Cut(difference, ": ")
		if strings.HasPrefix(field, "lane.") {
			if laneRow < 0 {
				laneRow = len(rows)
				words := func(l state.Lane, name string) string {
					since := "since"
					if l.Since.Observed {
						since = "seen since"
					}
					text := fmt.Sprintf("%s%s %s %s", name, l.Word, since, l.Since.At.Local().Format("15:04"))
					if l.Since.Observed {
						text += " (" + l.Since.Source + " carries no transition time)"
					}
					if l.By != "" {
						text += " by " + l.By
					}
					if l.Reason != "" {
						text += "; " + l.Reason
					}
					return text
				}
				rows = append(rows, Fired{Row: "drift", Subject: "shadow: " + words(shadow.Lane, "lane ") + "; records: " + words(rebuilt.Lane, "")})
			}
			rows[laneRow].Act += field + " "
			continue
		}
		before, after, _ := strings.Cut(values, " → ")
		rows = append(rows, Fired{Row: "drift", Subject: field + "; shadow: " + localShadowValue(before) + "; records: " + localShadowValue(after)})
	}
	return rows
}

var shadowClock = regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? [+-]\d{4} [A-Za-z]+`)

func localShadowValue(value string) string {
	return shadowClock.ReplaceAllStringFunc(value, func(stamp string) string {
		if at, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", stamp); err == nil {
			return at.Local().Format("2006-01-02 15:04 MST")
		}
		return stamp
	})
}
