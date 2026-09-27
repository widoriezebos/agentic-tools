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
	probe := "probe"
	if a.probeUsage != "" {
		probe = a.probeUsage
	}
	prefix := "metasystem " + runtimes.SupervisorEntry + " " + a.name
	lines := []string{
		prefix + " identity --root ROOT",
		prefix + " config-identity --root ROOT",
		prefix + " signature --root ROOT",
	}
	if _, ok := runtimes.EnforcementMapJSON(a.name); ok {
		lines = append(lines, prefix+" enforcement-map --root ROOT")
	}
	lines = append(lines,
		prefix+" contract --root ROOT",
		prefix+" "+probe+"",
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
	fmt.Fprintln(d.Stderr, "Usage:")
	for _, line := range lines {
		fmt.Fprintln(d.Stderr, "  "+line)
	}
}

var rfc3339UTCRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$`)

// WaitDeliveryAccepted is every runtime's wait-delivery answer: a complete
// request is held by the runtime's blocking foreground command.
func WaitDeliveryAccepted(waitID, nonce, deadline, session string) bool {
	return waitID != "" && nonce != "" && session != "" && rfc3339UTCRE.MatchString(deadline)
}

func parseWaitDelivery(args []string) bool {
	var waitID, nonce, deadline, session string
	for len(args) > 0 {
		if len(args) < 2 {
			return false
		}
		switch args[0] {
		case "--wait-id":
			waitID = args[1]
		case "--nonce":
			nonce = args[1]
		case "--deadline":
			deadline = args[1]
		case "--session":
			session = args[1]
		default:
			return false
		}
		args = args[2:]
	}
	return WaitDeliveryAccepted(waitID, nonce, deadline, session)
}

// Main runs `delegate-supervisor RUNTIME VERB --root ROOT [flags]` for a
// delegate verb and returns the exit status. newDeps builds the process
// seams for the installation root the argv names.
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
	a, ok := registry[name]
	if !ok {
		fmt.Fprintf(d.Stderr, "runtime adapter is not installed: %s\n", name)
		return 2
	}
	if verb == "-h" || verb == "--help" {
		a.usage(d)
		return 0
	}
	if root == "" || !filepath.IsAbs(root) {
		a.usage(d)
		return 2
	}
	switch verb {
	case "wait-delivery":
		if !parseWaitDelivery(rest) {
			return 2
		}
		fmt.Fprintln(d.Stdout, "blocking")
		return 0
	case "output-stream":
		if len(rest) != 2 || rest[0] != "--round-dir" || !strings.HasPrefix(rest[1], "/") {
			a.usage(d)
			return 2
		}
		stream, err := a.outputStream(d, strings.TrimSuffix(rest[1], "/"))
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprintln(d.Stdout, stream)
		return 0
	case "local-config-paths":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		paths, _ := runtimes.LocalConfigPaths(name)
		for _, path := range paths {
			fmt.Fprintln(d.Stdout, path)
		}
		return 0
	case "enforcement-map":
		enforcement, declared := runtimes.EnforcementMapJSON(name)
		if len(rest) != 0 || !declared {
			a.usage(d)
			return 2
		}
		fmt.Fprintln(d.Stdout, enforcement)
		return 0
	case "contract":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		data, err := a.contract(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		d.Stdout.Write(data)
		return 0
	case "signature":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		text, err := runtimes.SignatureText(name)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprint(d.Stdout, text)
		return 0
	case "identity":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		identity, err := a.configIdentity(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		version, hash, _, err := identityFields(identity)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprintf(d.Stdout, "%s %s\n", version, hash)
		return 0
	case "config-identity":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		identity, err := a.configIdentity(d)
		if err != nil {
			fmt.Fprintln(d.Stderr, err)
			return 1
		}
		fmt.Fprintln(d.Stdout, identity)
		return 0
	case "probe":
		return a.probe(d, rest)
	case runtimes.SupervisorDispatch, runtimes.SupervisorFollowUp:
		s := &Supervision{d: d, runtime: name, verb: verb}
		defer s.close()
		return a.supervise(s, rest)
	case "cancel":
		if len(rest) != 2 || rest[0] != "--job" {
			a.usage(d)
			return 2
		}
		return d.Dispatch.Run(d.Stdout, d.Stderr, "__cancel-owned", "--job", rest[1])
	case "selftest":
		if len(rest) != 0 {
			a.usage(d)
			return 2
		}
		return a.selftest(d)
	}
	a.usage(d)
	return 2
}

// prepareOrUsage runs the shared preparation and answers its failure the way
// the shell adapters did: the usage text and status 2.
func (s *Supervision) prepareOrUsage(args []string) bool {
	if err := s.prepare(args); err != nil {
		registry[s.runtime].usage(s.d)
		return false
	}
	return true
}
