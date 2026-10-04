package config

import "fmt"

const DesignGateModeKey = "design.gate.mode"

func ParseDesignGateMode(value string) (string, error) {
	if value != "warn" && value != "refuse" {
		return "warn", fmt.Errorf("%s must be warn or refuse, got %q", DesignGateModeKey, value)
	}
	return value, nil
}
