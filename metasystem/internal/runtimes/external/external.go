// Package external is the external agent-adapter contract and the runtime
// registry (design verbs-object-action 3.5, revision 8): an agent the
// metasystem does not ship is an executable at <installation>/adapters/<name>,
// in any language, invoked as `<executable> OPERATION` with a JSON request on
// stdin and a JSON response on stdout. An executable with a built-in's name
// overrides that built-in; it may answer an operation with exit 64 to hand
// it back to the built-in. docs/agent-adapters.md is the contract.
//
// The registry (Load) is the Go built-ins plus the external adapters an
// installation names, each as one effective declaration per runtime name,
// cross-checked so no runtime can claim another's processes. Every consumer
// reads it: the process recognizers (census, lease classification, human
// authority, the janitor's shapes), configuration validation, the
// delegate-supervisor entry, probes, self-tests, dispatch and the mission
// runner. The package is a leaf over the runtime declarations, so the
// recognizers use it without importing the supervisor.
package external

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/adapterfile"
)

// SchemaVersion is the operation schema version every request carries and
// every response must echo.
const SchemaVersion = 1

// Dir is the installation-relative adapter directory.
const Dir = adapterfile.Dir

// DelegateExit is the reserved exit status by which an overriding executable
// hands one operation back to the built-in it overrides (for a new runtime:
// the operation is not implemented).
const DelegateExit = 64

// UseValue is the only value of adapters.<name>.use that lets an executable
// run.
const UseValue = adapterfile.UseValue

// Operations are the runtime layer's operations, in contract order.
var Operations = []string{
	"describe", "probe", "selftest", "prepare", "observe", "finalize", "repair", "cancel",
}

// ErrDelegated is the answer of an executable that exits 64: an override
// hands the operation to its built-in; a new runtime does not implement it.
var ErrDelegated = errors.New("the external adapter delegated this operation (exit 64)")

// UseKey is the configuration key that names an external adapter (or an
// override of a built-in): adapters.<name>.use=external.
func UseKey(name string) string { return adapterfile.UseKey(name) }

// Adapter is one discovered, named and trusted external executable.
type Adapter struct {
	Name string
	Path string
	// Overrides is true when the executable replaces the built-in of the
	// same name.
	Overrides bool
	// conf is the installation configuration the bounds read.
	conf string
}

// Refusal is an executable found in the adapter directory, or a
// declaration, that the registry does not accept, with the reason and the
// fix.
type Refusal struct {
	Name, Path, Reason string
}

func (r Refusal) Error() string {
	return fmt.Sprintf("external adapter %s (%s) is refused: %s", r.Name, r.Path, r.Reason)
}

// Discover lists the external adapters of an installation root, sorted by
// name, and the executables it refused. A missing directory is no adapters.
// Discovery never executes a file.
func Discover(root string) ([]Adapter, []Refusal, error) {
	if root == "" {
		return nil, nil, nil
	}
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("external adapter directory %s is unreadable: %w", dir, err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	var adapters []Adapter
	var refusals []Refusal
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		reason, err := adapterfile.Check(root, name)
		if errors.Is(err, adapterfile.ErrAbsent) {
			continue
		}
		if !adapterfile.ValidName(name) {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: reason})
			continue
		}
		// Trust boundary (VOA-28): an external adapter is trusted code a
		// person installed. It runs only when the configuration names it,
		// is owned by the installation's user and is not group- or
		// world-writable; discovery never executes an unnamed or unsafe
		// file, it reports it.
		if !Named(root, name) {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: adapterfile.Unnamed(name)})
			continue
		}
		if reason != "" {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: reason})
			continue
		}
		adapter := Adapter{Name: name, Path: path, conf: conf}
		if declaration, builtin := runtimes.Lookup(name); builtin && declaration.HasAdapter {
			adapter.Overrides = true
		}
		adapters = append(adapters, adapter)
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Name < adapters[j].Name })
	return adapters, refusals, nil
}

// Named reports whether the installation's configuration (metasystem.conf,
// its .local overlay) sets adapters.<name>.use=external. The environment
// never names an adapter.
func Named(root, name string) bool {
	value, _, err := config.Get(config.GetParams{
		Key: UseKey(name), Default: "", DefaultSet: true, ConfPath: filepath.Join(root, "metasystem.conf"),
		LookupEnv: func(string) (string, bool) { return "", false },
	})
	return err == nil && value == UseValue
}

// Lookup returns the discovered external adapter of a name; a refused
// executable of that name is the error.
func Lookup(root, name string) (Adapter, bool, error) {
	adapters, refusals, err := Discover(root)
	if err != nil {
		return Adapter{}, false, err
	}
	for _, refusal := range refusals {
		if refusal.Name == name {
			return Adapter{}, false, refusal
		}
	}
	for _, adapter := range adapters {
		if adapter.Name == name {
			return adapter, true, nil
		}
	}
	return Adapter{}, false, nil
}

// Call runs one operation: the request (with schemaVersion, operation and
// runtime added) as JSON on stdin, the response from stdout. Exit 64 is
// ErrDelegated; any other failure is an error naming the operation. env nil
// inherits this process's environment.
func (a Adapter) Call(operation string, request map[string]any, env []string) ([]byte, error) {
	body := map[string]any{"schemaVersion": SchemaVersion, "operation": operation, "runtime": a.Name}
	for key, value := range request {
		body[key] = value
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	command := exec.Command(a.Path, operation)
	command.Stdin = bytes.NewReader(append(encoded, '\n'))
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := boundedexec.Run(command, boundedexec.Timeout(a.conf, boundedexec.Local), "external adapter "+a.Name+" "+operation)
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == DelegateExit {
		return nil, ErrDelegated
	}
	if runErr != nil {
		return nil, fmt.Errorf("external adapter %s %s failed: %w: %s", a.Name, operation, runErr, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Decode checks a response's schema version and decodes it.
func Decode(response []byte, into any) error {
	var version struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(response, &version); err != nil {
		return fmt.Errorf("external adapter response is not JSON: %w", err)
	}
	if version.SchemaVersion != SchemaVersion {
		return fmt.Errorf("external adapter response schemaVersion %d, want %d", version.SchemaVersion, SchemaVersion)
	}
	return json.Unmarshal(response, into)
}

// Capabilities are what describe declares a runtime can do.
type Capabilities struct {
	Resume       bool   `json:"resume"`
	FollowUp     bool   `json:"followUp"`
	Repair       bool   `json:"repair"`
	WaitDelivery bool   `json:"waitDelivery"`
	Host         bool   `json:"host"`
	Usage        string `json:"usage,omitempty"`
}

// Invocation is a runtime CLI's claim-bound invocation shape: the argv
// words it carries and where the claim tag sits (VOA-29).
type Invocation struct {
	Includes    []string `json:"includes"`
	TagFlag     string   `json:"tagFlag"`
	TagPrefix   string   `json:"tagPrefix,omitempty"`
	TagPathBase bool     `json:"tagPathBase,omitempty"`
}

// Selftest is how the shared self-test treats a runtime: its turn ceiling,
// whether a denied tool ends its turn, and its custom probe, whose stages
// run through the selftest operation.
type Selftest struct {
	TurnCeilingSec int            `json:"turnCeilingSec,omitempty"`
	DenialEndsTurn bool           `json:"denialEndsTurn,omitempty"`
	Probe          *SelftestProbe `json:"probe,omitempty"`
}

// SelftestProbe names a runtime's custom self-test probe and the behavior
// labels its pass earns.
type SelftestProbe struct {
	Name           string   `json:"name"`
	BehaviorLabels []string `json:"behaviorLabels"`
}

// Description is describe's answer (schema version 1).
type Description struct {
	SchemaVersion int          `json:"schemaVersion"`
	Name          string       `json:"name"`
	CLI           string       `json:"cli,omitempty"`
	Capabilities  Capabilities `json:"capabilities"`
	// Match and Exclude are the classification signature: a process argv is
	// this runtime iff some match and no exclude pattern hits (RE2 syntax,
	// POSIX classes allowed).
	Match   []string `json:"match"`
	Exclude []string `json:"exclude"`
	// Positive is an argv the signature must classify; Lookalike one it
	// must not (the runtime's reserved exclusion).
	Positive    string       `json:"positive"`
	Lookalike   string       `json:"lookalike"`
	Invocations []Invocation `json:"invocations"`
	ConfigPaths []string     `json:"configPaths"`
	// Enforcement is the static envelope-enforcement map (writeRoots,
	// readRoots, network → mapped | notEnforced), when declared.
	Enforcement map[string]string `json:"enforcement,omitempty"`
	// OutputStream is the round-relative file the CLI's stdout is written
	// to (default events.jsonl).
	OutputStream string    `json:"outputStream,omitempty"`
	Selftest     *Selftest `json:"selftest,omitempty"`
}

// describeKey identifies one executable's bytes on disk for the describe
// memo: a changed file is described again.
type describeKey struct {
	path          string
	size, mtimeNS int64
	ino           uint64
}

type describeAnswer struct {
	description Description
	err         error
}

var (
	describeMu   sync.Mutex
	describeMemo = map[describeKey]describeAnswer{}
)

// Describe runs the describe operation, memoized per process for the
// executable's current bytes. ErrDelegated passes through for an override
// that leaves describe to its built-in.
func (a Adapter) Describe() (Description, error) {
	info, err := os.Stat(a.Path)
	if err != nil {
		return Description{}, err
	}
	key := describeKey{path: a.Path, size: info.Size(), mtimeNS: info.ModTime().UnixNano()}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		key.ino = uint64(stat.Ino)
	}
	describeMu.Lock()
	answer, found := describeMemo[key]
	describeMu.Unlock()
	if found {
		return answer.description, answer.err
	}
	answer.description, answer.err = a.describe()
	describeMu.Lock()
	describeMemo[key] = answer
	describeMu.Unlock()
	return answer.description, answer.err
}

func (a Adapter) describe() (Description, error) {
	response, err := a.Call("describe", nil, nil)
	if err != nil {
		return Description{}, err
	}
	var described Description
	if err := Decode(response, &described); err != nil {
		return Description{}, fmt.Errorf("external adapter %s describe: %w", a.Name, err)
	}
	if described.Name != a.Name {
		return Description{}, fmt.Errorf("external adapter %s describes itself as %q; the name must equal the file name", a.Name, described.Name)
	}
	if len(described.Match) == 0 {
		return Description{}, fmt.Errorf("external adapter %s declares no match pattern", a.Name)
	}
	for _, pattern := range append(append([]string(nil), described.Match...), described.Exclude...) {
		if pattern == "" || pattern != strings.TrimSpace(pattern) || strings.ContainsAny(pattern, "\n\r") {
			return Description{}, fmt.Errorf("external adapter %s declares a malformed pattern %q", a.Name, pattern)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return Description{}, fmt.Errorf("external adapter %s pattern %q: %w", a.Name, pattern, err)
		}
	}
	for field, value := range described.Enforcement {
		if value != string(runtimes.Mapped) && value != string(runtimes.NotEnforced) {
			return Description{}, fmt.Errorf("external adapter %s enforcement %s=%q is neither mapped nor notEnforced", a.Name, field, value)
		}
	}
	return described, nil
}

// SignatureText renders a match/exclude declaration in the registry's line
// form.
func SignatureText(match, exclude []string) string {
	var lines []string
	for _, pattern := range match {
		lines = append(lines, "match "+pattern)
	}
	for _, pattern := range exclude {
		lines = append(lines, "exclude "+pattern)
	}
	return strings.Join(lines, "\n") + "\n"
}

// SignatureText asks the adapter to describe itself and renders its own
// signature (without the registry's additions).
func (a Adapter) SignatureText() (string, error) {
	described, err := a.Describe()
	if err != nil {
		return "", err
	}
	return SignatureText(described.Match, described.Exclude), nil
}

// Entry is one runtime's effective declaration in the registry.
type Entry struct {
	Name string
	// Builtin is a runtime the engine ships.
	Builtin bool
	// Adapter is the executable: a new runtime's, or a built-in's override.
	// Nil for a plain built-in.
	Adapter *Adapter
	// Description is the effective describe: an external's own, an
	// override's (when it answers describe), else the built-in's.
	Description Description
	// Signature is the effective classification signature in line form:
	// for an override, its own lines plus the built-in's reserved
	// exclusions (VOA-31); for every external, the shared exclusions of the
	// supervision processes.
	Signature string
	// Lookalikes are the reserved vectors no other declaration may match.
	Lookalikes []string
	// Refused is an override the registry refused: recognition keeps the
	// built-in's declaration, and running the runtime is refused with the
	// reason until the person fixes it.
	Refused *Refusal
}

// Overridden reports whether a built-in runs through an accepted override.
func (e Entry) Overridden() bool { return e.Builtin && e.Adapter != nil && e.Refused == nil }

// External reports whether the runtime is a new external runtime.
func (e Entry) External() bool { return !e.Builtin }

// Registry is an installation's runtime universe.
type Registry struct {
	Root    string
	entries map[string]Entry
	// Refusals are every executable or declaration the registry refused.
	Refusals []Refusal
}

// Lookup returns a runtime's effective declaration.
func (r Registry) Lookup(name string) (Entry, bool) {
	entry, ok := r.entries[name]
	return entry, ok
}

// Names are every runtime with an adapter, sorted.
func (r Registry) Names() []string {
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Externals are the entries run through an executable (new runtimes and
// overrides, accepted or refused), sorted by name.
func (r Registry) Externals() []Entry {
	var out []Entry
	for _, name := range r.Names() {
		if entry := r.entries[name]; entry.Adapter != nil {
			out = append(out, entry)
		}
	}
	return out
}

// Refusal returns the refusal of a name, when the registry refused one.
func (r Registry) Refusal(name string) (Refusal, bool) {
	for _, refusal := range r.Refusals {
		if refusal.Name == name {
			return refusal, true
		}
	}
	return Refusal{}, false
}

// BuiltinDescription is a built-in's declaration in describe form, from the
// runtime declarations (the recognizer and capability facts the registry
// needs; the supervisor's built-ins describe the rest).
func BuiltinDescription(name string) (Description, bool) {
	declaration, ok := runtimes.Lookup(name)
	if !ok || !declaration.HasAdapter {
		return Description{}, false
	}
	d := Description{SchemaVersion: SchemaVersion, Name: name,
		Capabilities: Capabilities{Resume: true, FollowUp: true, WaitDelivery: true, Host: declaration.HasHostLauncher},
		Positive:     declaration.SignatureVectors.Positive, Lookalike: declaration.SignatureVectors.Lookalike,
		OutputStream: "events.jsonl",
	}
	text, err := runtimes.SignatureText(name)
	if err != nil {
		return Description{}, false
	}
	d.Match, d.Exclude = splitSignature(text)
	for _, shape := range runtimes.CLIInvocations(name) {
		d.Invocations = append(d.Invocations, Invocation{Includes: shape.Includes, TagFlag: shape.TagFlag,
			TagPrefix: shape.TagPrefix, TagPathBase: shape.TagPathBase})
	}
	d.ConfigPaths, _ = runtimes.LocalConfigPaths(name)
	if declaration.ExpectedEnvelopeEnforcement != nil {
		d.Enforcement = map[string]string{}
		for field, value := range declaration.ExpectedEnvelopeEnforcement {
			d.Enforcement[field] = string(value)
		}
	}
	return d, true
}

func splitSignature(text string) (match, exclude []string) {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		verb, pattern, _ := strings.Cut(line, " ")
		switch verb {
		case "match":
			match = append(match, pattern)
		case "exclude":
			exclude = append(exclude, pattern)
		}
	}
	return match, exclude
}

// candidate is a declaration under cross-check.
type candidate struct {
	entry    Entry
	match    []*regexp.Regexp
	exclude  []*regexp.Regexp
	positive string
}

func compile(patterns []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			out = append(out, compiled)
		}
	}
	return out
}

// claims is the census rule: some match and no exclude hits.
func claims(match, exclude []*regexp.Regexp, argv string) bool {
	if argv == "" {
		return false
	}
	for _, pattern := range exclude {
		if pattern.MatchString(argv) {
			return false
		}
	}
	for _, pattern := range match {
		if pattern.MatchString(argv) {
			return true
		}
	}
	return false
}

func candidateOf(entry Entry) candidate {
	match, exclude := splitSignature(entry.Signature)
	return candidate{entry: entry, match: compile(match), exclude: compile(exclude), positive: entry.Description.Positive}
}

// Load is the registry of an installation root: every built-in with an
// adapter, plus the external adapters the root names, each cross-checked.
// A refusal is reported (Refusals, Entry.Refused), never an error; an error
// is an unreadable adapter directory.
func Load(root string) (Registry, error) {
	r := Registry{Root: root, entries: map[string]Entry{}}
	for _, name := range runtimes.WithAdapter() {
		description, _ := BuiltinDescription(name)
		text, _ := runtimes.SignatureText(name)
		r.entries[name] = Entry{Name: name, Builtin: true, Description: description, Signature: text,
			Lookalikes: nonEmpty(description.Lookalike)}
	}
	adapters, refusals, err := Discover(root)
	if err != nil {
		return Registry{}, err
	}
	r.Refusals = append(r.Refusals, refusals...)
	var pending []candidate
	for i := range adapters {
		adapter := adapters[i]
		refuse := func(reason string) {
			refusal := Refusal{Name: adapter.Name, Path: adapter.Path, Reason: reason}
			r.Refusals = append(r.Refusals, refusal)
			if adapter.Overrides {
				builtin := r.entries[adapter.Name]
				builtin.Adapter, builtin.Refused = &adapter, &refusal
				r.entries[adapter.Name] = builtin
			}
		}
		described, err := adapter.Describe()
		if adapter.Overrides {
			builtin := r.entries[adapter.Name]
			if errors.Is(err, ErrDelegated) {
				// The override leaves describe, and so its signature, to
				// the built-in.
				builtin.Adapter = &adapter
				r.entries[adapter.Name] = builtin
				continue
			}
			if err != nil {
				refuse(fmt.Sprintf("describe failed: %v", err))
				continue
			}
			// VOA-31: the override's own signature may not claim the
			// built-in's reserved lookalikes; the effective declaration
			// keeps the built-in's exclusions for its fallback paths.
			own, ownExclude := compile(described.Match), compile(described.Exclude)
			if lost := firstClaimed(own, ownExclude, builtin.Lookalikes); lost != "" {
				refuse(fmt.Sprintf("its signature claims %q, a process the built-in %s reserves (its fallback operations depend on excluding it); keep the built-in's exclusion in describe's \"exclude\": %s",
					lost, adapter.Name, strings.Join(builtin.Description.Exclude, " | ")))
				continue
			}
			effective := described
			if effective.Positive == "" {
				effective.Positive = builtin.Description.Positive
			}
			if effective.Lookalike == "" {
				effective.Lookalike = builtin.Description.Lookalike
			}
			effective.Exclude = mergeUnique(described.Exclude, builtin.Description.Exclude)
			lookalikes := mergeUnique(builtin.Lookalikes, nonEmpty(described.Lookalike))
			if !claims(own, compile(effective.Exclude), effective.Positive) {
				refuse(fmt.Sprintf("its signature does not classify its own positive vector %q", effective.Positive))
				continue
			}
			pending = append(pending, candidateOf(Entry{Name: adapter.Name, Builtin: true, Adapter: &adapter,
				Description: effective, Signature: SignatureText(effective.Match, effective.Exclude), Lookalikes: lookalikes}))
			continue
		}
		if errors.Is(err, ErrDelegated) {
			refuse("describe is required of a new runtime (it exited 64)")
			continue
		}
		if err != nil {
			refuse(fmt.Sprintf("describe failed: %v", err))
			continue
		}
		if described.Positive == "" || described.Lookalike == "" {
			refuse("describe must declare a positive and a lookalike vector (an argv the signature claims, and one it must leave alone)")
			continue
		}
		effective := described
		effective.Exclude = mergeUnique(described.Exclude, runtimes.SharedExcludes())
		match, exclude := compile(effective.Match), compile(effective.Exclude)
		if !claims(match, exclude, effective.Positive) {
			refuse(fmt.Sprintf("its signature does not classify its own positive vector %q", effective.Positive))
			continue
		}
		if claims(match, exclude, effective.Lookalike) {
			refuse(fmt.Sprintf("its signature claims its own lookalike vector %q", effective.Lookalike))
			continue
		}
		pending = append(pending, candidateOf(Entry{Name: adapter.Name, Adapter: &adapter, Description: effective,
			Signature: SignatureText(effective.Match, effective.Exclude), Lookalikes: []string{effective.Lookalike}}))
	}
	// The cross-check (VOA-28): no declaration may claim another runtime's
	// positive or reserved vectors, and no other runtime may claim its
	// positive vector. A conflict refuses the external declaration; the
	// built-ins are fixed.
	var others []candidate
	for _, name := range r.Names() {
		if entry := r.entries[name]; entry.Refused == nil {
			others = append(others, candidateOf(entry))
		}
	}
	others = append(others, pending...)
	for _, c := range pending {
		conflict := ""
		for _, o := range others {
			if o.entry.Name == c.entry.Name {
				continue
			}
			for _, vector := range append(nonEmpty(o.positive), o.entry.Lookalikes...) {
				if claims(c.match, c.exclude, vector) {
					conflict = fmt.Sprintf("its signature claims %q, a process of runtime %s; narrow describe's match or add an exclude", vector, o.entry.Name)
					break
				}
			}
			if conflict == "" && claims(o.match, o.exclude, c.positive) {
				conflict = fmt.Sprintf("runtime %s's signature already claims its positive vector %q; choose a distinguishable process name", o.entry.Name, c.positive)
			}
			if conflict != "" {
				break
			}
		}
		adapter := *c.entry.Adapter
		if conflict != "" {
			refusal := Refusal{Name: adapter.Name, Path: adapter.Path, Reason: conflict}
			r.Refusals = append(r.Refusals, refusal)
			if adapter.Overrides {
				builtin := r.entries[adapter.Name]
				builtin.Adapter, builtin.Refused = &adapter, &refusal
				r.entries[adapter.Name] = builtin
			}
			continue
		}
		r.entries[c.entry.Name] = c.entry
	}
	sort.Slice(r.Refusals, func(i, j int) bool { return r.Refusals[i].Name < r.Refusals[j].Name })
	return r, nil
}

// firstClaimed is the first vector a signature claims.
func firstClaimed(match, exclude []*regexp.Regexp, vectors []string) string {
	for _, vector := range vectors {
		if claims(match, exclude, vector) {
			return vector
		}
	}
	return ""
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func mergeUnique(first, second []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range append(append([]string(nil), first...), second...) {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// Report writes one line per external adapter, override and refusal: what
// `settings show` and `system check` print.
func (r Registry) Report(w io.Writer) {
	for _, entry := range r.Externals() {
		switch {
		case entry.Refused != nil:
			continue
		case entry.Builtin:
			fmt.Fprintf(w, "adapter %s: built-in overridden by %s\n", entry.Name, entry.Adapter.Path)
		default:
			fmt.Fprintf(w, "adapter %s: external runtime %s\n", entry.Name, entry.Adapter.Path)
		}
	}
	for _, refusal := range r.Refusals {
		fmt.Fprintf(w, "adapter %s: refused (%s): %s\n", refusal.Name, refusal.Path, refusal.Reason)
	}
}
