// Package applaunch owns the project's launch contract and the run records
// of the application it starts. It is the engine's hand on the application
// under construction: it interprets no language, it starts a process, probes
// it, tracks its tree, signals it by proven identity and copies its log.
package applaunch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SchemaVersion is the one schema this engine reads.
const SchemaVersion = 1

// The three facts a run has that the contract may name in argv and in its
// probe. Anything else between ${ and } is a fault the contract check names.
const (
	PlaceholderAddress = "address"
	PlaceholderHost    = "host"
	PlaceholderPort    = "port"
)

// The environment every command of the contract receives: the three
// placeholder facts, plus the run's own state root and its log path, which
// are facts a command needs but must not have to spell into its argv.
const (
	EnvAddress   = "METASYSTEM_APP_ADDRESS"
	EnvHost      = "METASYSTEM_APP_HOST"
	EnvPort      = "METASYSTEM_APP_PORT"
	EnvStateRoot = "METASYSTEM_APP_STATE_ROOT"
	EnvLog       = "METASYSTEM_APP_LOG"
)

// Readiness forms. http and tcp are probes, asked again by every status; log
// and none are startup observations scoped to one run, and for them stopping
// is proven by death alone, because a log line cannot unmatch.
const (
	ReadyHTTP = "http"
	ReadyTCP  = "tcp"
	ReadyLog  = "log"
	ReadyNone = "none"
)

// DataOwn declares that a run at another commit must have data of its own,
// which only a prepare command can make.
const (
	DataOwn    = "own"
	DataShared = "shared"
)

// DefaultReadyMS and DefaultStopMS are the waits a contract that names none
// gets: long enough for a compiled server to listen, short enough that a
// person is not left watching.
const (
	DefaultReadyMS int64 = 30_000
	DefaultStopMS  int64 = 15_000
)

// Command is one of the contract's commands: an argument vector and the
// directory it runs in, relative to the project root.
type Command struct {
	Argv []string `json:"argv"`
	CWD  string   `json:"cwd,omitempty"`
}

// Empty reports a command the contract left out.
func (c *Command) Empty() bool { return c == nil || len(c.Argv) == 0 }

// Ready is the contract's readiness form.
type Ready struct {
	Kind    string `json:"kind"`
	URL     string `json:"url,omitempty"`
	Address string `json:"address,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// Tool is a declared executable the contract's commands need, in the shape
// the testing contract declares one, so that a check names a missing JDK,
// cargo or Go before a build fails.
type Tool struct {
	ID          string   `json:"id"`
	Executable  string   `json:"executable"`
	VersionArgs []string `json:"versionArgs,omitempty"`
}

// Contract is schema 1: one application, everything but start optional.
type Contract struct {
	SchemaVersion int      `json:"schemaVersion"`
	Name          string   `json:"name,omitempty"`
	Start         *Command `json:"start"`
	Stop          *Command `json:"stop,omitempty"`
	Prepare       *Command `json:"prepare,omitempty"`
	Build         *Command `json:"build,omitempty"`
	Ready         *Ready   `json:"ready,omitempty"`
	Address       string   `json:"address,omitempty"`
	PortRange     string   `json:"portRange,omitempty"`
	Log           string   `json:"log,omitempty"`
	ReadyMS       int64    `json:"readyMs,omitempty"`
	StopMS        int64    `json:"stopMs,omitempty"`
	Check         string   `json:"check,omitempty"`
	Data          string   `json:"data,omitempty"`
	Tools         []Tool   `json:"tools,omitempty"`

	// digest is the sha256 of the exact bytes this contract was read from.
	// A run records it, so a status can say the contract changed under a
	// running application instead of pretending the record still describes it.
	digest string
}

// Digest is the sha256 of the bytes the contract was loaded from.
func (c Contract) Digest() string { return c.digest }

// ReadyKind is the readiness form, with the default for a contract that
// declares none.
func (c Contract) ReadyKind() string {
	if c.Ready == nil || c.Ready.Kind == "" {
		return ReadyNone
	}
	return c.Ready.Kind
}

// Probed reports the two readiness forms that can be asked again and can go
// dark. The other two are observations of this run's start.
func (c Contract) Probed() bool {
	kind := c.ReadyKind()
	return kind == ReadyHTTP || kind == ReadyTCP
}

// ReadyWaitMS and StopWaitMS apply the stated defaults.
func (c Contract) ReadyWaitMS() int64 {
	if c.ReadyMS > 0 {
		return c.ReadyMS
	}
	return DefaultReadyMS
}

func (c Contract) StopWaitMS() int64 {
	if c.StopMS > 0 {
		return c.StopMS
	}
	return DefaultStopMS
}

// OwnData reports whether a run at another commit must have data of its own.
// A contract with a prepare command gives every run its own data; one
// without shares the standing run's, and the record and the status say so.
func (c Contract) OwnData() bool { return !c.Prepare.Empty() }

// DataWord is what a record and a status say about a run's data.
func (c Contract) DataWord() string {
	if c.OwnData() {
		return DataOwn
	}
	return DataShared
}

// Load reads and validates a launch contract.
func Load(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}
	return Decode(data)
}

// Decode parses and validates contract bytes.
func Decode(data []byte) (Contract, error) {
	var contract Contract
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("launch contract is not readable: %w", err)
	}
	sum := sha256.Sum256(data)
	contract.digest = fmt.Sprintf("sha256:%x", sum)
	if err := contract.Validate(); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

var placeholder = regexp.MustCompile(`\$\{([^}]*)\}`)
var toolIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Validate names every fault it finds, in one line, so that a person fixes
// the contract once instead of once per run.
func (c Contract) Validate() error {
	var faults []string
	add := func(format string, args ...any) { faults = append(faults, fmt.Sprintf(format, args...)) }
	if c.SchemaVersion != SchemaVersion {
		add("schemaVersion must be %d", SchemaVersion)
	}
	if c.Start.Empty() {
		add("start is required: it is the one field a contract cannot leave out")
	}
	for _, named := range []struct {
		what    string
		command *Command
	}{{"start", c.Start}, {"stop", c.Stop}, {"prepare", c.Prepare}, {"build", c.Build}} {
		if named.command.Empty() {
			continue
		}
		if err := relativeDirectory(named.command.CWD); err != nil {
			add("%s cwd %s", named.what, err)
		}
		for _, word := range named.command.Argv {
			for _, name := range placeholders(word) {
				if !knownPlaceholder(name) {
					add("%s names unknown placeholder ${%s}; the contract knows ${address}, ${host} and ${port}", named.what, name)
				}
			}
		}
	}
	kind := c.ReadyKind()
	switch kind {
	case ReadyNone:
	case ReadyHTTP:
		if c.Ready.URL == "" {
			add("an http readiness probe needs a url")
		}
		if c.Address == "" {
			add("an http readiness probe needs the application's address; declare address")
		}
	case ReadyTCP:
		if c.Ready.Address == "" {
			add("a tcp readiness probe needs an address")
		}
		if c.Address == "" {
			add("a tcp readiness probe needs the application's address; declare address")
		}
	case ReadyLog:
		if c.Ready.Pattern == "" {
			add("a log readiness form needs a pattern")
		} else if _, err := regexp.Compile(c.Ready.Pattern); err != nil {
			add("the log readiness pattern is not a regular expression: %v", err)
		}
	default:
		add("ready kind %q is not one of http, tcp, log or none", kind)
	}
	if c.Ready != nil {
		for _, word := range []string{c.Ready.URL, c.Ready.Address, c.Ready.Pattern} {
			for _, name := range placeholders(word) {
				if !knownPlaceholder(name) {
					add("ready names unknown placeholder ${%s}; the contract knows ${address}, ${host} and ${port}", name)
				}
			}
		}
	}
	if c.Address != "" {
		if _, _, err := splitAddress(c.Address); err != nil {
			add("address %s is not host:port", c.Address)
		}
	}
	low, high, rangeErr := c.Range()
	switch {
	case c.PortRange != "" && rangeErr != nil:
		add("portRange %s", rangeErr)
	case c.PortRange != "" && c.Address != "":
		if _, port, err := splitAddress(c.Address); err == nil && port >= low && port <= high {
			add("portRange %d-%d overlaps the standing address %s; a candidate would take the standing run's port", low, high, c.Address)
		}
	}
	if c.Data != "" && c.Data != DataOwn && c.Data != DataShared {
		add("data must be own or shared")
	}
	if c.Data == DataOwn && c.Prepare.Empty() {
		add("data: own is declared with no prepare to make it")
	}
	if c.Log != "" {
		if err := relativeDirectory(c.Log); err != nil {
			add("log %s", err)
		}
	}
	if c.ReadyMS < 0 || c.StopMS < 0 {
		add("readyMs and stopMs are whole milliseconds, never negative")
	}
	seen := map[string]bool{}
	for _, tool := range c.Tools {
		if !toolIdentifier.MatchString(tool.ID) {
			add("tool id %q must be lower-case letters, digits and hyphens", tool.ID)
		}
		if seen[tool.ID] {
			add("tool %s is declared twice", tool.ID)
		}
		seen[tool.ID] = true
		if strings.TrimSpace(tool.Executable) == "" {
			add("tool %s declares no executable", tool.ID)
		}
	}
	if len(faults) == 0 {
		return nil
	}
	sort.Strings(faults)
	return fmt.Errorf("%s", strings.Join(faults, "; "))
}

// Range reads the candidate port range.
func (c Contract) Range() (int, int, error) {
	if c.PortRange == "" {
		return 0, 0, nil
	}
	lowText, highText, ok := strings.Cut(c.PortRange, "-")
	if !ok {
		return 0, 0, fmt.Errorf("must be LOW-HIGH")
	}
	low, lowErr := strconv.Atoi(strings.TrimSpace(lowText))
	high, highErr := strconv.Atoi(strings.TrimSpace(highText))
	if lowErr != nil || highErr != nil || low < 1 || high > 65535 || low > high {
		return 0, 0, fmt.Errorf("must be LOW-HIGH, both ports from 1 to 65535, low first")
	}
	return low, high, nil
}

func relativeDirectory(path string) error {
	if path == "" {
		return nil
	}
	if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") {
		return fmt.Errorf("%s must be a relative normalized path inside the project", path)
	}
	return nil
}

func placeholders(text string) []string {
	var names []string
	for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
		names = append(names, match[1])
	}
	return names
}

func knownPlaceholder(name string) bool {
	switch name {
	case PlaceholderAddress, PlaceholderHost, PlaceholderPort:
		return true
	}
	return false
}

// splitAddress reads host:port, requiring a numeric port.
func splitAddress(address string) (string, int, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("port must be a number from 1 to 65535")
	}
	return host, port, nil
}

// Facts are the run's three substituted facts.
type Facts struct {
	Address string
	Host    string
	Port    string
}

// FactsFor derives the three facts from one address. An address the run has
// none of leaves all three empty, which is right for an application with
// nothing to listen on.
func FactsFor(address string) Facts {
	if address == "" {
		return Facts{}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Facts{Address: address}
	}
	return Facts{Address: address, Host: host, Port: port}
}

// Substitute replaces the three placeholders in one word.
func (f Facts) Substitute(word string) string {
	return placeholder.ReplaceAllStringFunc(word, func(match string) string {
		switch match[2 : len(match)-1] {
		case PlaceholderAddress:
			return f.Address
		case PlaceholderHost:
			return f.Host
		case PlaceholderPort:
			return f.Port
		}
		return match
	})
}

// Argv substitutes every word of a command's argument vector.
func (f Facts) Argv(command *Command) []string {
	if command.Empty() {
		return nil
	}
	argv := make([]string, 0, len(command.Argv))
	for _, word := range command.Argv {
		argv = append(argv, f.Substitute(word))
	}
	return argv
}

// Environment overlays the run's facts onto a base environment. The three
// substituted facts plus the state root and the log reach every command of
// the contract this way, so a start script that would rather read its port
// from the environment than from its argv can.
func (f Facts) Environment(base []string, stateRoot, logPath string) []string {
	overlay := map[string]string{}
	if f.Address != "" {
		overlay[EnvAddress] = f.Address
		overlay[EnvHost] = f.Host
		overlay[EnvPort] = f.Port
	}
	if stateRoot != "" {
		overlay[EnvStateRoot] = stateRoot
	}
	if logPath != "" {
		overlay[EnvLog] = logPath
	}
	result := make([]string, 0, len(base)+len(overlay))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := overlay[name]; !replaced {
			result = append(result, entry)
		}
	}
	names := make([]string, 0, len(overlay))
	for name := range overlay {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, name+"="+overlay[name])
	}
	return result
}
