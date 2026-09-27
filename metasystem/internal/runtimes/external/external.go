// Package external is the external agent-adapter contract (design
// verbs-object-action 3.5, revision 8): an agent the metasystem does not ship
// is an executable at <installation>/adapters/<name>, in any language,
// invoked as `<executable> OPERATION` with a JSON request on stdin and a JSON
// response on stdout. The runtime registry is the Go built-ins plus the
// executables discovered here; an executable with a built-in's name overrides
// that built-in only when `adapters.<name>.override=external` is set, and may
// answer an operation with exit 64 to hand it back to the built-in.
//
// The package is a leaf over the runtime declarations so the process
// recognizers can discover external runtimes without importing the
// supervisor. Until unit U6c adds the registry cross-check they only
// discover and report them: describe is never executed for a recognizer,
// and a refused executable is absent, never an error.
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
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// SchemaVersion is the operation schema version every request carries and
// every response must echo.
const SchemaVersion = 1

// Dir is the installation-relative adapter directory.
const Dir = "adapters"

// DelegateExit is the reserved exit status by which an overriding executable
// hands one operation back to the built-in it overrides.
const DelegateExit = 64

// UseValue is the only value of adapters.<name>.use that lets an executable
// run: a new runtime, or an override of a built-in of the same name.
const UseValue = "external"

// Operations are the runtime layer's operations, in contract order.
var Operations = []string{
	"describe", "probe", "selftest", "prepare", "observe", "finalize", "repair", "cancel",
}

// ErrDelegated is the answer of an overriding executable that exits 64: the
// built-in performs this operation.
var ErrDelegated = errors.New("the external adapter delegated this operation to the built-in")

// UseKey is the configuration key that names an external adapter (or an
// override of a built-in): adapters.<name>.use=external.
func UseKey(name string) string { return "adapters." + name + ".use" }

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Adapter is one discovered external executable.
type Adapter struct {
	Name string
	Path string
	// Overrides is true when the executable replaces the built-in of the
	// same name (its override setting is present).
	Overrides bool
	// conf is the installation configuration the bounds read.
	conf string
}

// Refusal is an executable found in the adapter directory that the registry
// does not accept, with the reason.
type Refusal struct {
	Name, Path, Reason string
}

func (r Refusal) Error() string { return fmt.Sprintf("adapter %s (%s): %s", r.Name, r.Path, r.Reason) }

// Discover lists the external adapters of an installation root, sorted by
// name, and the executables it refused. A missing directory is no adapters.
func Discover(root string) ([]Adapter, []Refusal, error) {
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
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			continue
		}
		if !nameRE.MatchString(name) {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: "the name violates the runtime-name grammar"})
			continue
		}
		adapter := Adapter{Name: name, Path: path, conf: conf}
		// Trust boundary (VOA-28): an external adapter is trusted code a
		// person installed. It runs only when the configuration names it,
		// is owned by the installation's user and is not group- or
		// world-writable; discovery never executes an unnamed or unsafe
		// file, it reports it.
		value, _, getErr := config.Get(config.GetParams{
			Key: UseKey(name), Default: "", DefaultSet: true, ConfPath: conf,
			LookupEnv: func(string) (string, bool) { return "", false },
		})
		if getErr != nil || value != UseValue {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: fmt.Sprintf(
				"an external adapter runs only when %s=%s is set", UseKey(name), UseValue)})
			continue
		}
		if reason := unsafeFile(info); reason != "" {
			refusals = append(refusals, Refusal{Name: name, Path: path, Reason: reason})
			continue
		}
		if declaration, builtin := runtimes.Lookup(name); builtin && declaration.HasAdapter {
			adapter.Overrides = true
		}
		adapters = append(adapters, adapter)
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Name < adapters[j].Name })
	return adapters, refusals, nil
}

// Lookup returns the discovered external adapter of a name.
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

// Call runs one operation: the request (with schemaVersion added) as JSON on
// stdin, extra appended after it when the operation streams input
// (output-stream), the response from stdout. Exit 64 is ErrDelegated; any
// other failure is an error naming the operation.
func (a Adapter) Call(operation string, request map[string]any, extra io.Reader) ([]byte, error) {
	body := map[string]any{"schemaVersion": SchemaVersion, "operation": operation, "runtime": a.Name}
	for key, value := range request {
		body[key] = value
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var stdin io.Reader = bytes.NewReader(append(encoded, '\n'))
	if extra != nil {
		stdin = io.MultiReader(stdin, extra)
	}
	command := exec.Command(a.Path, operation)
	command.Stdin = stdin
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

// DescribeResponse is the part of the describe operation's response the
// recognizers read: the classification signatures with their exclusions.
type DescribeResponse struct {
	SchemaVersion int      `json:"schemaVersion"`
	Match         []string `json:"match"`
	Exclude       []string `json:"exclude"`
}

// SignatureText asks the adapter to describe itself and renders its process
// signature in the registry's line form. ErrDelegated passes through for an
// override that leaves describe to its built-in.
func (a Adapter) SignatureText() (string, error) {
	response, err := a.Call("describe", nil, nil)
	if err != nil {
		return "", err
	}
	var described DescribeResponse
	if err := Decode(response, &described); err != nil {
		return "", err
	}
	if len(described.Match) == 0 {
		return "", fmt.Errorf("external adapter %s declares no match pattern", a.Name)
	}
	var lines []string
	for _, pattern := range described.Match {
		lines = append(lines, "match "+pattern)
	}
	for _, pattern := range described.Exclude {
		lines = append(lines, "exclude "+pattern)
	}
	for _, line := range lines {
		if line != strings.TrimSpace(line) || strings.ContainsAny(line, "\n\r") {
			return "", fmt.Errorf("external adapter %s declares a malformed pattern", a.Name)
		}
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// unsafeFile names why an adapter file is not trusted: not owned by this
// user, or writable by its group or by everyone.
func unsafeFile(info os.FileInfo) string {
	if info.Mode()&0o022 != 0 {
		return "the adapter file is group- or world-writable"
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return "the adapter file is not owned by the installation's user"
	}
	return ""
}
