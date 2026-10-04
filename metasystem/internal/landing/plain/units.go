package plain

import (
	"fmt"
	"strings"
	"unicode"
)

// UnitsOnMain counts distinct units of goal in trailers reachable from main.
func UnitsOnMain(checkout, main, goal string) (int, error) {
	return unitsOnMain(checkout, main, goal, Git)
}

func unitsOnMain(checkout, main, goal string, read func(string, ...string) (string, error)) (int, error) {
	trailers, err := read(checkout, "log", "--format=%(trailers:key=Goal-Unit,valueonly)", "--fixed-strings", "--grep=Goal-Unit: "+goal+"/", main)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(trailers, "\n") {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		id, list, valid := strings.Cut(value, "/")
		units := strings.Split(list, "+")
		local := map[string]bool{}
		for _, unit := range units {
			if unit == "" || strings.Contains(unit, "/") || strings.IndexFunc(unit, unicode.IsSpace) >= 0 || local[unit] {
				valid = false
			}
			local[unit] = true
		}
		if id != goal {
			continue
		}
		if !valid {
			return 0, fmt.Errorf("the Goal-Unit trailer for %s is damaged: %q", goal, value)
		}
		for _, unit := range units {
			seen[unit] = true
		}
	}
	return len(seen), nil
}
