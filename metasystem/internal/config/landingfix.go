package config

import "strconv"

// LandingFixLimit reads the lane's allowance; invalid limits admit no job.
func LandingFixLimit(confPath, key string) int {
	value, code, err := Get(GetParams{Key: key, ConfPath: confPath})
	limit, parseErr := strconv.Atoi(value)
	if err != nil || code != 0 || parseErr != nil || limit < 0 {
		return 0
	}
	return limit
}
