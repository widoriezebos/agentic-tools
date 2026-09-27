package supervisor

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// builtinOps carries the operations a Go built-in shares with every other
// built-in: describe from the runtime registry, and the probe, contract,
// identity and self-test functions each built-in names. A built-in embeds it
// and adds prepare, observe, finalize (and repair when it declares one).
type builtinOps struct {
	name           string
	cli            string
	usage          string // native, unavailable, or metered
	host, repair   bool
	configIdentity func(Deps) (string, error)
	probe          func(Deps, []string) int
	contract       func(Deps) ([]byte, error)
	selftest       func(Deps) int
	invocations    []InvocationShape
}

func (b builtinOps) Describe(Deps) (Description, error) {
	d := Description{Name: b.name, CLI: b.cli, SchemaVersion: OperationsSchemaVersion,
		Capabilities: Capabilities{Resume: true, FollowUp: true, Repair: b.repair, WaitDelivery: true, Host: b.host, Usage: b.usage},
		Invocations:  b.invocations}
	text, err := runtimes.SignatureText(b.name)
	if err != nil {
		return Description{}, err
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		verb, pattern, _ := strings.Cut(line, " ")
		if verb == "match" {
			d.Match = append(d.Match, pattern)
		} else {
			d.Exclude = append(d.Exclude, pattern)
		}
	}
	if declaration, ok := runtimes.Lookup(b.name); ok {
		d.Positive, d.Lookalike = declaration.SignatureVectors.Positive, declaration.SignatureVectors.Lookalike
	}
	d.ConfigPaths, _ = runtimes.LocalConfigPaths(b.name)
	d.Enforcement, _ = runtimes.EnforcementMapJSON(b.name)
	return d, nil
}

func (b builtinOps) ConfigIdentity(d Deps) (string, error) { return b.configIdentity(d) }
func (b builtinOps) Probe(d Deps, args []string) int       { return b.probe(d, args) }
func (b builtinOps) Contract(d Deps) ([]byte, error)       { return b.contract(d) }
func (b builtinOps) Selftest(d Deps) int                   { return b.selftest(d) }

// Repair is absent unless a built-in declares and implements it.
func (b builtinOps) Repair(*Turn, RepairInput) RepairResult { return RepairResult{Status: 1} }

// Cancel has no runtime-specific step: the shared cancellation.
func (b builtinOps) Cancel(d Deps, job string) int {
	return d.Dispatch.Run(d.Stdout, d.Stderr, "__cancel-owned", "--job", job)
}

// OperationsFor is the operation interface of a runtime for an installation
// (the built-ins that run on the shared rounds). ok is false for a runtime
// that does not.
func OperationsFor(d Deps, name string) (Operations, bool) {
	a, found := registry[name]
	if !found || a.ops == nil {
		return nil, false
	}
	return a.ops, true
}
