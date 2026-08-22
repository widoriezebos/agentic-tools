package adapter

import (
	"fmt"
	"sort"
)

// SelftestProbe is one runtime's typed self-test probe, a registered
// capability. A probe prepares scratch state,
// contributes prompt text, verifies returned evidence, and names the
// exact behavior labels the pass record earns. Shared self-test code
// consumes the probe and never the runtime name.
type SelftestProbe struct {
	Name string
	// PrepareScratch adds the probe's fixtures to the scratch repo.
	PrepareScratch func(scratch, nonce string) error
	// PromptText is the extra goal instruction for the main turn.
	PromptText func(nonce string) string
	// VerifyEvidence checks the returned evidence proved the probe.
	VerifyEvidence func(returnPath, nonce string) error
	// BehaviorLabels are appended to provenBehaviorally on success.
	BehaviorLabels []string
}

// selftestProbes is the adapter seam's typed capability table, keyed
// runtime then probe name. Registration happens from per-runtime seam
// files at init; core code only looks up.
var selftestProbes = map[string]map[string]SelftestProbe{}

// RegisterSelftestProbe registers a runtime's probe. A duplicate
// (runtime, name) key is a declaration bug and panics at init.
func RegisterSelftestProbe(runtime string, probe SelftestProbe) {
	if probe.Name == "" || probe.PrepareScratch == nil || probe.PromptText == nil || probe.VerifyEvidence == nil {
		panic(fmt.Sprintf("selftest probe for %s is incomplete", runtime))
	}
	if _, dup := selftestProbes[runtime][probe.Name]; dup {
		panic(fmt.Sprintf("selftest probe %s/%s registered twice", runtime, probe.Name))
	}
	if selftestProbes[runtime] == nil {
		selftestProbes[runtime] = map[string]SelftestProbe{}
	}
	selftestProbes[runtime][probe.Name] = probe
}

// SelftestProbeFor resolves a runtime's declared probe by name; a
// cross-runtime or undeclared name is a refusal, never a fallback.
func SelftestProbeFor(runtime, name string) (SelftestProbe, error) {
	probe, ok := selftestProbes[runtime][name]
	if !ok {
		return SelftestProbe{}, fmt.Errorf("no selftest probe %q declared for runtime %s", name, runtime)
	}
	return probe, nil
}

// SelftestProbeList is the read-only conformance view: "runtime/name"
// keys, sorted.
func SelftestProbeList() []string {
	var keys []string
	for runtime, probes := range selftestProbes {
		for name := range probes {
			keys = append(keys, runtime+"/"+name)
		}
	}
	sort.Strings(keys)
	return keys
}
