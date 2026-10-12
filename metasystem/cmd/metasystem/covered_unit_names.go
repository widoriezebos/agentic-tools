package main

import "strings"

func coveredUnitNames(value string) []string {
	if value == "" {
		return nil
	}
	names := strings.Split(value, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}
	return names
}
