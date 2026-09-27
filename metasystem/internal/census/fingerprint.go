package census

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// The fingerprint is the supervision code staleness detector. It hashes the
// supervision scripts, the runtime signature declarations, and the relevant
// config into one digest; a re-arm is forced when the digest changes.

// fingerprintFiles are the fixed supervision inputs, relative to the
// metasystem root (order does not affect the hash — the map is sorted — but
// it documents the contract).
var fingerprintFiles = []string{
	"scripts/agents/arm-supervision.sh",
	"scripts/agents/dispatch.sh",
	"bin/metasystem",
	"scripts/watch-background-jobs.sh",
}

// fingerprintConfig maps each relevant config key to its default.
var fingerprintConfig = map[string]string{
	"metasystem.runtimes":               "",
	"watch.interval-sec":                "60",
	"watch.stale-min":                   "20",
	"watch.cap-min":                     "180",
	"census.log-max-bytes":              "1048576",
	"census.max-interval-share-percent": "50",
}

// SignatureText returns a runtime's normalized signature declaration — the
// `match`/`exclude` lines joined by newlines with a trailing newline, from
// the runtime registry's one process definition. This is the value hashed
// per runtime.
func SignatureText(runtime string) (string, error) {
	return runtimes.SignatureText(runtime)
}

// RuntimeSignature compiles one built-in runtime's registry signature.
func RuntimeSignature(runtime string) (Signature, string, error) {
	text, err := SignatureText(runtime)
	if err != nil {
		return Signature{}, "", err
	}
	return compileText(runtime, text)
}

func compileText(runtime, text string) (Signature, string, error) {
	matches, excludes := ParseSignatureText(text)
	sig, err := CompileSignature(runtime, matches, excludes)
	if err != nil {
		return Signature{}, "", err
	}
	return sig, text, nil
}

// RuntimeSignatureAt compiles one runtime's signature for an installation:
// an external adapter's (design 3.5) from its signature operation, an
// overriding executable's unless it delegates the operation (exit 64), and
// otherwise the built-in's.
func RuntimeSignatureAt(root, runtime string) (Signature, string, error) {
	if root != "" {
		adapter, found, err := external.Lookup(root, runtime)
		if err != nil {
			return Signature{}, "", err
		}
		if found {
			text, err := adapter.SignatureText()
			switch {
			case err == nil:
				return compileText(runtime, text)
			case !errors.Is(err, external.ErrDelegated) || !adapter.Overrides:
				return Signature{}, "", err
			}
		}
	}
	return RuntimeSignature(runtime)
}

// FixtureSignatureRuntimesEnv narrows the adapter signature universe in a
// fixture-mode root (metasystem.runtimes=fake) to the named runtimes, so a
// process fixture's ancestry is agent-free although the test runner itself
// may run under a real runtime. It stands in for what fixture beds did when
// the signatures were scripts: delete the unrelated adapter scripts from the
// scratch installation. Outside a fixture-mode root it is ignored.
const FixtureSignatureRuntimesEnv = "METASYSTEM_FIXTURE_SIGNATURE_RUNTIMES"

// AllAdapterSignatures compiles the delegate signature of every runtime
// that declares an adapter (all of them, not only the configured runtimes: a
// delegate of any installed runtime must be recognised as a delegate). The
// order is the runtime names' sort order, the order the adapter scripts'
// directory listing used to give.
func AllAdapterSignatures() ([]Signature, error) {
	sigs, _, err := adapterSignatures(runtimes.WithAdapter())
	return sigs, err
}

// InstalledAdapterSignatures is AllAdapterSignatures for an installation
// root: the built-ins (an override's signature where one answers) plus every
// external adapter discovered in the root's adapters directory, honoring
// FixtureSignatureRuntimesEnv in a fixture-mode root. It also returns the
// runtime names and each signature text in the same order.
func InstalledAdapterSignatures(root string) ([]Signature, []string, []string, error) {
	return installedAdapterSignatures(root, os.Getenv)
}

func installedAdapterSignatures(root string, getenv func(string) string) ([]Signature, []string, []string, error) {
	names := runtimes.WithAdapter()
	adapters, _, err := external.Discover(root)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, adapter := range adapters {
		if !adapter.Overrides {
			names = append(names, adapter.Name)
		}
	}
	if narrowed := getenv(FixtureSignatureRuntimesEnv); narrowed != "" && fixtureauth.FixtureModeRoot(root) {
		names = strings.Fields(strings.ReplaceAll(narrowed, ",", " "))
	}
	sort.Strings(names)
	var sigs []Signature
	var texts []string
	for _, runtime := range names {
		sig, text, err := RuntimeSignatureAt(root, runtime)
		if err != nil {
			return nil, nil, nil, err
		}
		sigs = append(sigs, sig)
		texts = append(texts, text)
	}
	return sigs, names, texts, nil
}

func adapterSignatures(names []string) ([]Signature, []string, error) {
	names = append([]string(nil), names...)
	sort.Strings(names)
	var sigs []Signature
	var texts []string
	for _, runtime := range names {
		sig, text, err := RuntimeSignature(runtime)
		if err != nil {
			return nil, nil, err
		}
		sigs = append(sigs, sig)
		texts = append(texts, text)
	}
	return sigs, texts, nil
}

// Fingerprint computes the supervision fingerprint for a repo, hashing files
// and config from metasystemRoot.
func Fingerprint(metasystemRoot, repo string) (string, error) {
	repoReal, err := filepath.EvalSymlinks(repo)
	if err != nil {
		repoReal = repo
	}
	confPath := filepath.Join(metasystemRoot, "metasystem.conf")

	selected := splitRuntimes(config.ConfValue(confPath, "metasystem.runtimes", ""))
	files := append([]string(nil), fingerprintFiles...)
	for _, runtime := range selected {
		if _, found, _ := external.Lookup(metasystemRoot, runtime); found {
			files = append(files, filepath.Join(external.Dir, runtime))
		}
	}

	fileHashes := map[string]string{}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(metasystemRoot, rel))
		if err != nil {
			return "", fmt.Errorf("fingerprint input is unavailable: %s: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		fileHashes[rel] = hex.EncodeToString(sum[:])
	}

	signatures := map[string]string{}
	for _, runtime := range selected {
		_, text, err := RuntimeSignatureAt(metasystemRoot, runtime)
		if err != nil {
			return "", err
		}
		signatures[runtime] = text
	}

	relevantConfig := map[string]string{}
	for key, def := range fingerprintConfig {
		relevantConfig[key] = config.ConfValue(confPath, key, def)
	}

	payload := map[string]any{
		"repositoryScope": repoReal,
		"files":           fileHashes,
		"signatures":      signatures,
		"config":          relevantConfig,
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalJSON serializes v canonically: sorted keys, compact (no spaces
// after separators), and — crucially — WITHOUT Go's default HTML escaping of
// < > &. Inputs here are ASCII, so no non-ASCII escaping arises.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder appends a newline; the canonical form has none.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func splitRuntimes(csv string) []string {
	var out []string
	for _, item := range strings.Split(csv, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out) // determinism; the hash sorts the map regardless
	return out
}
