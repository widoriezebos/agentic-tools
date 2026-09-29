package runtimes

import (
	"embed"
	"fmt"
)

// enforcementSourceDir is where each runtime's hook settings source lives
// in the engine source; the files are compiled in below it.
const enforcementSourceDir = "internal/runtimes/enforcement"

//go:embed enforcement
var enforcementFiles embed.FS

// EnforcementSource is the installation-relative engine source of a shipped
// enforcement config named by a declaration's ShippedEnforcementConfig.
func EnforcementSource(name string) string {
	return enforcementSourceDir + "/" + name
}

// ShippedEnforcement is the hook settings source this engine installs for
// runtime: the bytes host setup merges into the runtime's settings file.
func ShippedEnforcement(runtime string) ([]byte, error) {
	declaration, ok := Lookup(runtime)
	if !ok || declaration.ShippedEnforcementConfig == "" {
		return nil, fmt.Errorf("runtime %s ships no enforcement config", runtime)
	}
	data, err := enforcementFiles.ReadFile("enforcement/" + declaration.ShippedEnforcementConfig)
	if err != nil {
		return nil, fmt.Errorf("engine has no enforcement config %s: %w", declaration.ShippedEnforcementConfig, err)
	}
	return data, nil
}
