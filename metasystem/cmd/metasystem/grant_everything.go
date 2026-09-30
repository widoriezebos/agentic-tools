package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// grantEndExamples are the easy forms of a general grant's end, the detail of
// every refusal of one.
const grantEndExamples = "metasystem grant add --acts everything --for 24h (or 8h, 7d, 1w), --until 18:00, --until tomorrow, or a date within the next 7 days, --until YYYY-MM-DD"

// grantEndOthers are the other easy ends, beside the --for 24h a refusal
// names.
const grantEndOthers = "or 8h, 7d, 1w; or --until 18:00, --until tomorrow, --until a date within 7 days"

var grantForPattern = regexp.MustCompile(`^([1-9][0-9]{0,3})([hdw])$`)
var grantClockPattern = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)

// parseGrantEnd reads a general grant's end from exactly one of --for (h,
// d = 24 hours, w = 168 hours of elapsed time) and --until (HH:MM today or,
// once passed, tomorrow; tomorrow; YYYY-MM-DD, the end of that day), in
// zone. A local deadline that does not exist or happens twice is refused,
// never shifted; the end is on a whole minute, after now, and at most one
// week later.
func parseGrantEnd(now time.Time, zone *time.Location, forValue, until string) (time.Time, error) {
	forValue, until = strings.TrimSpace(forValue), strings.TrimSpace(until)
	if now.IsZero() || zone == nil {
		return time.Time{}, fmt.Errorf("the clock can't be read")
	}
	if forValue != "" && until != "" {
		return time.Time{}, fmt.Errorf("a general grant takes --for or --until, not both")
	}
	var end time.Time
	switch {
	case forValue != "":
		match := grantForPattern.FindStringSubmatch(forValue)
		if match == nil {
			return time.Time{}, fmt.Errorf("--for %s isn't a duration like 8h, 24h, 7d or 1w", forValue)
		}
		n, _ := strconv.Atoi(match[1])
		unit := map[string]time.Duration{"h": time.Hour, "d": 24 * time.Hour, "w": 168 * time.Hour}[match[2]]
		end = now.Add(time.Duration(n) * unit).Truncate(time.Minute)
	case until != "":
		local := now.In(zone)
		var err error
		switch {
		case until == "tomorrow":
			end, err = localInstant(local.Year(), local.Month(), local.Day()+2, 0, 0, zone)
		case grantClockPattern.MatchString(until):
			match := grantClockPattern.FindStringSubmatch(until)
			hour, _ := strconv.Atoi(match[1])
			minute, _ := strconv.Atoi(match[2])
			day := local.Day()
			if !time.Date(local.Year(), local.Month(), day, hour, minute, 0, 0, zone).After(now) {
				day++
			}
			end, err = localInstant(local.Year(), local.Month(), day, hour, minute, zone)
		default:
			date, parseErr := time.ParseInLocation("2006-01-02", until, zone)
			if parseErr != nil {
				return time.Time{}, fmt.Errorf("--until %s isn't a time like 18:00, tomorrow or a date like 2026-10-07", until)
			}
			end, err = localInstant(date.Year(), date.Month(), date.Day()+1, 0, 0, zone)
		}
		if err != nil {
			return time.Time{}, err
		}
	default:
		return time.Time{}, fmt.Errorf("a general grant needs its end, like --for 24h")
	}
	if !end.After(now) {
		return time.Time{}, fmt.Errorf("the grant's end %s is already past", end.In(zone).Format("15:04 MST (2006-01-02)"))
	}
	if latest := now.Add(goal.GeneralAttorneyMax).Truncate(time.Minute); end.After(latest) {
		return time.Time{}, fmt.Errorf("a general grant lasts at most one week; the latest end is %s (--for 1w)", latest.In(zone).Format("15:04 MST (2006-01-02)"))
	}
	return end, nil
}

// localInstant is the one instant a local wall-clock time names, refusing a
// time the zone skips or repeats.
func localInstant(year int, month time.Month, day, hour, minute int, zone *time.Location) (time.Time, error) {
	instant := time.Date(year, month, day, hour, minute, 0, 0, zone)
	wall := time.Date(year, month, day, hour, minute, 0, 0, time.UTC).Format("2006-01-02 15:04")
	name := zone.String()
	if instant.Format("2006-01-02 15:04") != wall {
		return time.Time{}, fmt.Errorf("%s does not exist on %s in %s (the clocks skip it); name another time", wall[11:], wall[:10], name)
	}
	for shift := -3 * time.Hour; shift <= 3*time.Hour; shift += 15 * time.Minute {
		if shift != 0 && instant.Add(shift).In(zone).Format("2006-01-02 15:04") == wall {
			return time.Time{}, fmt.Errorf("%s happens twice on %s in %s (the clocks go back); name another time", wall[11:], wall[:10], name)
		}
	}
	return instant, nil
}
