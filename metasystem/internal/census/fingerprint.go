package census

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
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

// RuntimeSignature compiles one runtime's registry signature.
func RuntimeSignature(runtime string) (Signature, string, error) {
	text, err := SignatureText(runtime)
	if err != nil {
		return Signature{}, "", err
	}
	matches, excludes := ParseSignatureText(text)
	sig, err := CompileSignature(runtime, matches, excludes)
	if err != nil {
		return Signature{}, "", err
	}
	return sig, text, nil
}

// AllAdapterSignatures compiles the delegate signature of every runtime
// that declares an adapter, (all of them, not
// only the configured runtimes: a delegate of any installed runtime must be
// recognised as a delegate). The order is the runtime names' sort order,
// the order the adapter scripts' directory listing used to give.
func AllAdapterSignatures() ([]Signature, error) {
	names := runtimes.WithAdapter()
	sort.Strings(names)
	var sigs []Signature
	for _, runtime := range names {
		sig, _, err := RuntimeSignature(runtime)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
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
		text, err := SignatureText(runtime)
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
