package hostload

import (
	"fmt"
	"os"
)

func readLoad() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("/proc/loadavg: %w", err)
	}
	return parseProcLoadavg(string(data))
}
