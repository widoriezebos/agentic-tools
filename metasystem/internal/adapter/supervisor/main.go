package supervisor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// runtimeAdapter is one runtime's half of the supervisor: the verbs whose
// behavior differs per runtime. A nil verb is not offered by the runtime.
type runtimeAdapter struct {
	name string
	// configIdentity prints the canonical configuration identity JSON.
	configIdentity func(d Deps) (string, error)
	// probe writes a fresh capability snapshot.
	probe func(d Deps, args []string) int
	// contract prints the contract snapshot bytes.
	contract func(d Deps) ([]byte, error)
	// outputStream names the round file the CLI child's stdout stream is.
	outputStream func(d Deps, roundDir string) (string, error)
	// supervise owns one round from prepare to the terminal record.
	supervise func(s *Supervision, args []string) int
	// selftest runs the full-contract self-test.
	selftest func(d Deps) int
	// usage is the verbs the runtime's usage text lists after the shared
	// ones.
	probeUsage string
}

var registry = map[string]runtimeAdapter{}

func register(adapter runtimeAdapter) { registry[adapter.name] = adapter }

// Runtimes lists the runtimes the supervisor serves, sorted.
func Runtimes() []string {
	var names []string
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a runtimeAdapter) usage(d Deps) {
	probe := "probe --root ROOT"
	if a.probeUsage != "" {
		probe = a.probeUsage
	}
	_, enforcement := runtimes.EnforcementMapJSON(a.name)
	writeUsage(d.Stderr, usageLines(a.name, probe, enforcement))
}

var rfc3339UTCRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)

// WaitDeliveryAccepted is every runtime's wait-delivery answer: a complete
// request is held by the runtime's blocking foreground command.
func WaitDeliveryAccepted(waitID, nonce, deadline, session string) bool {
	return waitID != "" && nonce != "" && session != "" && rfc3339UTCRE.MatchString(deadline)
}

// Main runs `delegate-supervisor RUNTIME VERB --root ROOT [flags]` for a
// delegate verb and returns the exit status. newDeps builds the process
// seams for the installation root the argv names. The runtime is resolved
// through the registry: a built-in, an external adapter executable, or a
// built-in overridden by one (design 3.5).
func Main(args []string, newDeps func(root string) Deps) int {
	if len(args) < 2 {
		d := newDeps("")
		fmt.Fprintf(d.Stderr, "usage: metasystem %s RUNTIME VERB --root ROOT [flags]\n", runtimes.SupervisorEntry)
		return 2
	}
	name, verb, rest := args[0], args[1], args[2:]
	root := ""
	if len(rest) >= 2 && rest[0] == "--root" {
		root, rest = rest[1], rest[2:]
	}
	d := newDeps(root)
	if root != "" && !filepath.IsAbs(root) {
		d.Root = ""
	}
	r, err := resolveRuntime(d, name)
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 2
	}
	if verb == "-h" || verb == "--help" {
		r.Usage(d)
		return 0
	}
	if root == "" || !filepath.IsAbs(root) {
		r.Usage(d)
		return 2
	}
	switch verb {
	case "wait-delivery":
		var waitID, nonce, deadline, session string
		for len(rest) > 0 {
			if len(rest) < 2 {
				return 2
			}
			switch rest[0] {
			case "--wait-id":
				waitID = rest[1]
			case "--nonce":
				nonce = rest[1]
			case "--deadline":
				deadline = rest[1]
			case "--session":
				session = rest[1]
			default:
				return 2
			}
			rest = rest[2:]
		}
		accepted, err := r.WaitDelivery(d, waitID, nonce, deadline, session)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		if !accepted {
			return 2
		}
		fmt.Fprintln(d.Stdout, "blocking")
		return 0
	case "output-stream":
		if len(rest) != 2 || rest[0] != "--round-dir" || !strings.HasPrefix(rest[1], "/") {
			r.Usage(d)
			return 2
		}
		stream, err := r.OutputStream(d, strings.TrimSuffix(rest[1], "/"))
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprintln(d.Stdout, stream)
		return 0
	case "local-config-paths":
		if len(rest) != 0 {
			r.Usage(d)
			return 2
		}
		paths, err := r.LocalConfigPaths(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		for _, path := range paths {
			fmt.Fprintln(d.Stdout, path)
		}
		return 0
	case "enforcement-map":
		enforcement, declared, err := r.EnforcementMap(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		if len(rest) != 0 || !declared {
			r.Usage(d)
			return 2
		}
		fmt.Fprintln(d.Stdout, enforcement)
		return 0
	case "contract":
		if len(rest) != 0 {
			r.Usage(d)
			return 2
		}
		data, err := r.Contract(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		d.Stdout.Write(data)
		return 0
	case "signature":
		if len(rest) != 0 {
			r.Usage(d)
			return 2
		}
		text, err := r.Signature(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprint(d.Stdout, text)
		return 0
	case "identity", "config-identity":
		if len(rest) != 0 {
			r.Usage(d)
			return 2
		}
		identity, err := r.ConfigIdentity(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		if verb == "config-identity" {
			fmt.Fprintln(d.Stdout, identity)
			return 0
		}
		version, hash, _, err := identityFields(identity)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprintf(d.Stdout, "%s %s\n", version, hash)
		return 0
	case "probe":
		return r.Probe(d, rest)
	case runtimes.SupervisorDispatch, runtimes.SupervisorFollowUp:
		s := &Supervision{d: d, runtime: name, verb: verb, usage: r.Usage}
		defer s.close()
		return r.Supervise(s, rest)
	case "cancel":
		if len(rest) != 2 || rest[0] != "--job" {
			r.Usage(d)
			return 2
		}
		return r.Cancel(d, rest[1])
	case "selftest":
		if len(rest) != 0 {
			r.Usage(d)
			return 2
		}
		return r.Selftest(d)
	}
	r.Usage(d)
	return 2
}

// prepareOrUsage runs the shared preparation and answers its failure the way
// the shell adapters did: the usage text and status 2.
func (s *Supervision) prepareOrUsage(args []string) bool {
	if err := s.prepare(args); err != nil {
		if s.usage != nil {
			s.usage(s.d)
		} else if a, ok := registry[s.runtime]; ok {
			a.usage(s.d)
		}
		return false
	}
	return true
}
