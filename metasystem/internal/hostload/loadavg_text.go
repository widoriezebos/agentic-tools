package hostload

import (
	"fmt"
	"strconv"
	"strings"
)

// parseProcLoadavg reads the first three fields of /proc/loadavg.
func parseProcLoadavg(text string) (float64, float64, float64, error) {
	fields := strings.Fields(text)
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("/proc/loadavg: %q has fewer than three fields", strings.TrimSpace(text))
	}
	var values [3]float64
	for index := range values {
		value, err := strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("/proc/loadavg: field %d: %w", index+1, err)
		}
		values[index] = value
	}
	return values[0], values[1], values[2], nil
}
