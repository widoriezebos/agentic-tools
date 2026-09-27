package supervisor

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// Runtime is the runtime layer the delegate-supervisor entry runs (design
// 3.5): one Go interface for a built-in, an external adapter executable, and
// a built-in overridden operation by operation. The shared lifecycle
// (preparation, custody, handshake, deadlines, adjudication, records) is the
// same for all of them; only these operations differ.
type Runtime interface {
	Name() string
	// Signature is the census signature text of the runtime's processes.
	Signature(d Deps) (string, error)
	// ConfigIdentity is the canonical configuration identity JSON.
	ConfigIdentity(d Deps) (string, error)
	// LocalConfigPaths are the checkout-relative configuration files.
	LocalConfigPaths(d Deps) ([]string, error)
	// EnforcementMap is the static envelope-enforcement map, when declared.
	EnforcementMap(d Deps) (string, bool, error)
	// Contract is the contract capability snapshot bytes.
	Contract(d Deps) ([]byte, error)
	// Probe writes a fresh capability snapshot and returns the exit status.
	Probe(d Deps, args []string) int
	// OutputStream names the round file the runtime child's stdout is.
	OutputStream(d Deps, roundDir string) (string, error)
	// Supervise owns one round from preparation to the terminal record.
	Supervise(s *Supervision, args []string) int
	// Cancel cancels a job the runtime owns and returns the exit status.
	Cancel(d Deps, job string) int
	// WaitDelivery answers whether the runtime's session holds a blocking
	// foreground wait for a complete request.
	WaitDelivery(d Deps, waitID, nonce, deadline, session string) (bool, error)
	// Selftest runs the full-contract self-test.
	Selftest(d Deps) int
	// Usage writes the runtime's usage text.
	Usage(d Deps)
}

// BuiltinOperations reports, per built-in runtime, the operations its Go
// implementation provides — the surface the external contract mirrors.
func BuiltinOperations() map[string][]string {
	out := map[string][]string{}
	for name, a := range registry {
		ops := []string{"signature", "local-config-paths", "wait-delivery", "cancel"}
		if a.configIdentity != nil {
			ops = append(ops, "identity", "config-identity")
		}
		if _, ok := runtimes.EnforcementMapJSON(name); ok {
			ops = append(ops, "enforcement-map")
		}
		if a.contract != nil {
			ops = append(ops, "contract")
		}
		if a.probe != nil {
			ops = append(ops, "probe")
		}
		if a.outputStream != nil {
			ops = append(ops, "output-stream-file")
		}
		if a.supervise != nil {
			ops = append(ops, "supervise")
		}
		if a.selftest != nil {
			ops = append(ops, "selftest")
		}
		out[name] = ops
	}
	return out
}

// builtin adapts a registered Go runtime to the interface.
type builtin struct{ a runtimeAdapter }

func (b builtin) Name() string { return b.a.name }
func (b builtin) Signature(Deps) (string, error) {
	return runtimes.SignatureText(b.a.name)
}
func (b builtin) ConfigIdentity(d Deps) (string, error) { return b.a.configIdentity(d) }
func (b builtin) LocalConfigPaths(Deps) ([]string, error) {
	paths, _ := runtimes.LocalConfigPaths(b.a.name)
	return paths, nil
}
func (b builtin) EnforcementMap(Deps) (string, bool, error) {
	enforcement, ok := runtimes.EnforcementMapJSON(b.a.name)
	return enforcement, ok, nil
}
func (b builtin) Contract(d Deps) ([]byte, error)          { return b.a.contract(d) }
func (b builtin) Probe(d Deps, args []string) int          { return b.a.probe(d, args) }
func (b builtin) Supervise(s *Supervision, a []string) int { return b.a.supervise(s, a) }
func (b builtin) Selftest(d Deps) int                      { return b.a.selftest(d) }
func (b builtin) Usage(d Deps)                             { b.a.usage(d) }
func (b builtin) OutputStream(d Deps, roundDir string) (string, error) {
	return b.a.outputStream(d, roundDir)
}
func (b builtin) Cancel(d Deps, job string) int {
	return d.Dispatch.Run(d.Stdout, d.Stderr, "__cancel-owned", "--job", job)
}
func (b builtin) WaitDelivery(_ Deps, waitID, nonce, deadline, session string) (bool, error) {
	return WaitDeliveryAccepted(waitID, nonce, deadline, session), nil
}

// ErrNotInstalled is a runtime name neither built in nor discovered.
var ErrNotInstalled = errors.New("runtime adapter is not installed")

// resolveRuntime is the registry: an external adapter discovered in the
// installation's adapters directory, an override of a built-in (per
// operation, the executable first and the built-in on exit 64), or the
// built-in.
func resolveRuntime(d Deps, name string) (Runtime, error) {
	b, isBuiltin := registry[name]
	if d.Root != "" {
		adapter, found, err := external.Lookup(d.Root, name)
		if err != nil {
			return nil, err
		}
		if found && adapter.Overrides && isBuiltin {
			return override{ext: externalRuntime{adapter}, base: builtin{b}}, nil
		}
		if found {
			return externalRuntime{adapter}, nil
		}
	}
	if isBuiltin {
		return builtin{b}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNotInstalled, name)
}

// override runs an overriding executable's operation, and the built-in's
// when the executable delegates it (exit 64).
type override struct {
	ext  externalRuntime
	base builtin
}

func delegated(err error) bool { return errors.Is(err, external.ErrDelegated) }

func (o override) Name() string { return o.base.Name() }
func (o override) Signature(d Deps) (string, error) {
	if text, err := o.ext.Signature(d); !delegated(err) {
		return text, err
	}
	return o.base.Signature(d)
}
func (o override) ConfigIdentity(d Deps) (string, error) {
	if identity, err := o.ext.ConfigIdentity(d); !delegated(err) {
		return identity, err
	}
	return o.base.ConfigIdentity(d)
}
func (o override) LocalConfigPaths(d Deps) ([]string, error) {
	if paths, err := o.ext.LocalConfigPaths(d); !delegated(err) {
		return paths, err
	}
	return o.base.LocalConfigPaths(d)
}
func (o override) EnforcementMap(d Deps) (string, bool, error) {
	if enforcement, ok, err := o.ext.EnforcementMap(d); !delegated(err) {
		return enforcement, ok, err
	}
	return o.base.EnforcementMap(d)
}
func (o override) Contract(d Deps) ([]byte, error) {
	if data, err := o.ext.Contract(d); !delegated(err) {
		return data, err
	}
	return o.base.Contract(d)
}
func (o override) Probe(d Deps, args []string) int {
	if code, err := o.ext.probe(d, args); !delegated(err) {
		return code
	}
	return o.base.Probe(d, args)
}
func (o override) OutputStream(d Deps, roundDir string) (string, error) {
	if declared, err := o.ext.declaresCommand(); err == nil && declared {
		return o.ext.OutputStream(d, roundDir)
	}
	return o.base.OutputStream(d, roundDir)
}
func (o override) Supervise(s *Supervision, args []string) int {
	declared, err := o.ext.declaresCommand()
	if err != nil {
		fmt.Fprintln(s.d.Stderr, err)
		return 1
	}
	if !declared {
		return o.base.Supervise(s, args)
	}
	return superviseExternal(s, args, o.ext, o.base.a.name)
}
func (o override) Cancel(d Deps, job string) int {
	if _, err := o.ext.cancelHook(d, job); !delegated(err) && err != nil {
		fmt.Fprintln(d.Stderr, err)
	}
	return o.base.Cancel(d, job)
}
func (o override) WaitDelivery(d Deps, waitID, nonce, deadline, session string) (bool, error) {
	if accepted, err := o.ext.WaitDelivery(d, waitID, nonce, deadline, session); !delegated(err) {
		return accepted, err
	}
	return o.base.WaitDelivery(d, waitID, nonce, deadline, session)
}
func (o override) Selftest(d Deps) int { return o.base.Selftest(d) }
func (o override) Usage(d Deps)        { o.base.Usage(d) }

// usageLines is the entry's usage for a runtime.
func usageLines(name, probe string, enforcement bool) []string {
	prefix := "metasystem " + runtimes.SupervisorEntry + " " + name
	lines := []string{
		prefix + " identity --root ROOT",
		prefix + " config-identity --root ROOT",
		prefix + " signature --root ROOT",
	}
	if enforcement {
		lines = append(lines, prefix+" enforcement-map --root ROOT")
	}
	return append(lines,
		prefix+" contract --root ROOT",
		prefix+" "+probe,
		prefix+" output-stream --root ROOT --round-dir <absolute-path>",
		prefix+" dispatch --root ROOT --job <job-id> --start-gate <file>",
		"    --instance-tag <tag> --launch-capability <opaque-capability>",
		prefix+" follow-up --root ROOT --job <job-id> --start-gate <file>",
		"    --instance-tag <tag> --launch-capability <opaque-capability>",
		prefix+" cancel --root ROOT --job <job-id>",
		prefix+" selftest --root ROOT",
		prefix+" local-config-paths --root ROOT",
		prefix+" wait-delivery --root ROOT --wait-id ID --nonce NONCE --deadline RFC3339-UTC --session SESSION-ID",
	)
}

func writeUsage(w io.Writer, lines []string) {
	fmt.Fprintln(w, "Usage:")
	for _, line := range lines {
		fmt.Fprintln(w, "  "+strings.TrimRight(line, " "))
	}
}

// LocalConfigManifest is every installed runtime's declared local
// configuration for an installation — the built-ins (or their overrides)
// and the external adapters — sorted, deduplicated.
func LocalConfigManifest(d Deps) ([]string, error) {
	names := runtimes.WithAdapter()
	adapters, _, err := external.Discover(d.Root)
	if err != nil {
		return nil, err
	}
	for _, a := range adapters {
		if !a.Overrides {
			names = append(names, a.Name)
		}
	}
	seen := map[string]bool{}
	var paths []string
	for _, name := range names {
		r, err := resolveRuntime(d, name)
		if err != nil {
			return nil, err
		}
		declared, err := r.LocalConfigPaths(d)
		if err != nil {
			return nil, err
		}
		for _, path := range declared {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}
