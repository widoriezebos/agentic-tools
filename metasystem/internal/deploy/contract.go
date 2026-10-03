// Package deploy is the project's deploy chain: the contract a project
// declares, the call of its adapter, the shared record, the pause, the lock
// and the runner that deploys origin's main. The adapter is an executable in
// any language, called as `<argv> OPERATION` with one JSON request on
// standard input and one JSON response on standard output; this package
// interprets no language.
package deploy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// ContractKey selects the deploy contract beside the settings, the way
// launch.contract selects the launch contract.
const ContractKey = "deploy.contract"

// The refusals of the deploy verbs.
const (
	CodePaused     = "DEPLOY_PAUSED"
	CodeNoPrevious = "DEPLOY_NO_PREVIOUS"
	// CodeRunning refuses a person's rollback or resume while a deploy is
	// in progress.
	CodeRunning = "DEPLOY_RUNNING"
)

// ErrNoContract is the answer of a project that declares no deploy: the
// lane does nothing more after its push, and each deploy verb says so.
var ErrNoContract = errors.New("this project declares no deploy: it has no deploy.json beside metasystem.conf")

// Contract is schema 1 of deploy.json.
type Contract struct {
	Schema  int     `json:"schema"`
	Adapter Adapter `json:"adapter"`
	Tools   []Tool  `json:"tools,omitempty"`
}

// Adapter is the executable every operation calls, with the operation
// appended, run from CWD inside the commit's clean tree.
type Adapter struct {
	Argv []string `json:"argv"`
	CWD  string   `json:"cwd,omitempty"`
}

// Tool is a tool the adapter needs, named so a missing one can be told.
type Tool struct {
	ID          string   `json:"id"`
	Executable  string   `json:"executable"`
	VersionArgs []string `json:"versionArgs,omitempty"`
}

// Declared says whether the installation of the checkout at root declares
// a deploy, and where that installation lies in every tree of the
// repository. It reads no contract: an adapter is only ever called by the
// deploy.json of the clean tree of the commit its call is about, never by a
// working tree's.
func Declared(installation, root string) (string, error) {
	if _, err := contractPath(installation); err != nil {
		return "", err
	}
	inside, err := filepath.Rel(root, installation)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("the installation %s is not inside the repository %s", installation, root)
	}
	return filepath.ToSlash(inside), nil
}

// contractPath is the deploy contract the installation's settings name. A
// project that has not written the file they select, the default or the
// one deploy.contract names, gets ErrNoContract.
func contractPath(installation string) (string, error) {
	relative, found, err := config.ConfLookup(filepath.Join(installation, "metasystem.conf"), ContractKey)
	if err != nil {
		return "", err
	}
	if !found {
		relative = config.MustDefault(ContractKey)
	}
	if strings.TrimSpace(relative) == "" {
		return "", ErrNoContract
	}
	if !normalRelative(relative) {
		return "", fmt.Errorf("%s must be a relative normalized path", ContractKey)
	}
	path := filepath.Join(installation, filepath.FromSlash(relative))
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "", ErrNoContract
	}
	return path, nil
}

// LoadContract reads the deploy contract the installation's settings name.
func LoadContract(installation string) (Contract, string, error) {
	path, err := contractPath(installation)
	if err != nil {
		return Contract{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, path, err
	}
	contract, err := DecodeContract(data)
	if err != nil {
		return Contract{}, path, fmt.Errorf("the deploy contract %s is not valid: %w", path, err)
	}
	return contract, path, nil
}

// DecodeContract parses and validates contract bytes; unknown fields are
// faults, so a misspelled key is never silently ignored.
func DecodeContract(data []byte) (Contract, error) {
	var contract Contract
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, err
	}
	var faults []string
	if contract.Schema != 1 {
		faults = append(faults, "schema must be 1")
	}
	if len(contract.Adapter.Argv) == 0 || strings.TrimSpace(contract.Adapter.Argv[0]) == "" {
		faults = append(faults, "adapter.argv must name the adapter's executable")
	}
	if contract.Adapter.CWD != "" && !normalRelative(contract.Adapter.CWD) {
		faults = append(faults, "adapter.cwd must be a relative normalized path inside the tree")
	}
	for _, tool := range contract.Tools {
		if tool.ID == "" || tool.Executable == "" {
			faults = append(faults, "each tool needs an id and an executable")
			break
		}
	}
	if len(faults) > 0 {
		return Contract{}, errors.New(strings.Join(faults, "; "))
	}
	return contract, nil
}

func normalRelative(path string) bool {
	return !filepath.IsAbs(path) && filepath.ToSlash(filepath.Clean(path)) == path && path != ".." && !strings.HasPrefix(path, "../")
}

// defaultPorts are the ports a fetch URL may leave out: one repository
// spelled with or without its scheme's own port.
var defaultPorts = map[string]string{"ssh": "22", "git": "9418", "http": "80", "https": "443"}

// ProjectKey names a repository the same way for every checkout of it on
// this computer: its origin fetch URL, normalized so the https, ssh and scp
// spellings of one repository agree, hashed to a short hexadecimal name that
// is safe as a directory name. A port other than the scheme's own names
// another server, so it stays in the name.
func ProjectKey(fetchURL string) (string, error) {
	normal := strings.TrimSpace(fetchURL)
	if normal == "" {
		return "", errors.New("the repository has no origin fetch URL")
	}
	switch {
	case strings.Contains(normal, "://"):
		parsed, err := url.Parse(normal)
		if err != nil {
			return "", fmt.Errorf("origin's fetch URL %q: %w", normal, err)
		}
		host := strings.ToLower(parsed.Hostname())
		if port := parsed.Port(); port != "" && port != defaultPorts[parsed.Scheme] {
			host = net.JoinHostPort(host, port)
		}
		normal = host + "/" + strings.TrimPrefix(parsed.Path, "/")
		if parsed.Scheme == "file" {
			normal = filepath.Clean(parsed.Path)
		}
	case scpLike(normal):
		at := strings.LastIndex(normal[:strings.Index(normal, ":")], "@")
		host, path, _ := strings.Cut(normal[at+1:], ":")
		normal = strings.ToLower(host) + "/" + strings.TrimPrefix(path, "/")
	default:
		normal = filepath.Clean(normal)
	}
	normal = strings.TrimSuffix(strings.TrimRight(normal, "/"), ".git")
	sum := sha256.Sum256([]byte(normal))
	return hex.EncodeToString(sum[:8]), nil
}

// scpLike is git's user@host:path form: a colon before any slash.
func scpLike(spelling string) bool {
	colon := strings.Index(spelling, ":")
	slash := strings.Index(spelling, "/")
	return colon > 0 && (slash < 0 || colon < slash)
}

// Dir is a project's deploy directory under the home directory every seat
// on this computer shares: one record, one lock and one pause per
// repository.
func Dir(home, project string) string { return filepath.Join(home, "deploy", project) }
