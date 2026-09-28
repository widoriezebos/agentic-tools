package census

import (
	"crypto/sha256"
	"encoding/hex"
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
	"bin/metasystem",
}

// FingerprintFiles returns the fixed supervision inputs the fingerprint
// hashes, relative to the metasystem root.
func FingerprintFiles() []string { return append([]string(nil), fingerprintFiles...) }

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

// RuntimeSignatureAt is a runtime's recognizer signature for an
// installation root, from the runtime registry (design 3.5): a built-in's,
// an overridden built-in's one effective declaration (the override's
// signature plus the built-in's reserved exclusions, VOA-31), or an external
// runtime's cross-checked declaration.
func RuntimeSignatureAt(root, runtime string) (Signature, string, error) {
	if root != "" {
		reg, err := external.Load(root)
		if err != nil {
			return Signature{}, "", err
		}
		if entry, found := reg.Lookup(runtime); found {
			return compileText(runtime, entry.Signature)
		}
		if refusal, refused := reg.Refusal(runtime); refused {
			return Signature{}, "", refusal
		}
	}
	return RuntimeSignature(runtime)
}

// absentExternal reports whether a runtime name is an external adapter the
// registry refused: absent to the recognizers, never an error for them.
func absentExternal(root, runtime string) bool {
	if _, builtin := runtimes.Lookup(runtime); builtin {
		return false
	}
	reg, err := external.Load(root)
	if err != nil {
		return false
	}
	if _, found := reg.Lookup(runtime); found {
		return false
	}
	_, refused := reg.Refusal(runtime)
	return refused
}

// ExternalAdapters reports the external adapters an installation declares
// and the executables it refused, without executing any of them.
func ExternalAdapters(root string) ([]external.Adapter, []external.Refusal, error) {
	return external.Discover(root)
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
// root: every runtime of its registry (built-ins, overrides as one
// effective declaration, and the external runtimes it names), narrowed in a
// fixture-mode root to its configured runtimes and
// FixtureSignatureRuntimesEnv. It also returns the runtime names and each
// signature text in the same order.
func InstalledAdapterSignatures(root string) ([]Signature, []string, []string, error) {
	return installedAdapterSignatures(root, os.Getenv)
}

func installedAdapterSignatures(root string, getenv func(string) string) ([]Signature, []string, []string, error) {
	reg, err := external.Load(root)
	if err != nil {
		return nil, nil, nil, err
	}
	names := reg.Names()
	if fixtureauth.FixtureModeRoot(root) {
		// A fixture-mode root (metasystem.runtimes=fake) classifies against
		// its configured runtimes only: a test or bed runs under whatever
		// agent CLI its person uses, and that ambient runtime must not turn
		// a staged human ancestry into a delegate one. The environment can
		// narrow further.
		names = splitRuntimes(config.ConfValue(filepath.Join(root, "metasystem.conf"), "metasystem.runtimes", ""))
		if narrowed := getenv(FixtureSignatureRuntimesEnv); narrowed != "" {
			names = strings.Fields(strings.ReplaceAll(narrowed, ",", " "))
		}
		names = registered(reg, names)
	}
	sort.Strings(names)
	var sigs []Signature
	var texts []string
	for _, runtime := range names {
		entry, _ := reg.Lookup(runtime)
		sig, text, err := compileText(runtime, entry.Signature)
		if err != nil {
			return nil, nil, nil, err
		}
		sigs = append(sigs, sig)
		texts = append(texts, text)
	}
	return sigs, names, texts, nil
}

// registered keeps the names the registry has (a refused external is
// absent, never an error).
func registered(reg external.Registry, names []string) []string {
	var kept []string
	for _, name := range names {
		if _, ok := reg.Lookup(name); ok {
			kept = append(kept, name)
		}
	}
	return kept
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

	reg, err := external.Load(metasystemRoot)
	if err != nil {
		return "", err
	}
	// The selected runtimes as the recognizers see them: a refused external
	// is absent (never an error), an accepted external or override hashes
	// its executable and its effective signature.
	var selected []string
	files := append([]string(nil), fingerprintFiles...)
	for _, runtime := range splitRuntimes(config.ConfValue(confPath, "metasystem.runtimes", "")) {
		entry, found := reg.Lookup(runtime)
		if !found {
			if _, refused := reg.Refusal(runtime); refused {
				continue
			}
		}
		selected = append(selected, runtime)
		if found && entry.Adapter != nil {
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
	canonical, err := config.CanonicalConfigJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), nil
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
