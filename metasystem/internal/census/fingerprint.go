package census

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// fingerprint port (process-census.py fingerprint): the supervision code
// staleness detector. It hashes the supervision scripts, the runtime
// signature declarations, and the relevant config into one digest;
// verify_armed forces a re-arm when it changes. This is a STRICT PORT — the
// digest must equal the python's byte-for-byte, proven by conformance
// against the real repo.

// fingerprintFiles are the fixed supervision inputs, relative to the
// metasystem root, in the python's list order (order does not affect the
// hash — the map is sorted — but it documents the contract).
var fingerprintFiles = []string{
	"scripts/agents/arm-supervision.sh",
	"scripts/agents/dispatch.sh",
	"scripts/agents/process-census.py",
	"scripts/agents/adapters/runtime-common.sh",
	"scripts/watch-background-jobs.sh",
}

// fingerprintConfig maps each relevant config key to its default, matching
// the python's dict.
var fingerprintConfig = map[string]string{
	"metasystem.runtimes":               "",
	"watch.interval-sec":                "60",
	"watch.stale-min":                   "20",
	"watch.cap-min":                     "180",
	"census.log-max-bytes":              "1048576",
	"census.max-interval-share-percent": "50",
}

// SignatureText returns an adapter's normalized signature declaration — the
// `match`/`exclude` lines joined by newlines with a trailing newline, exactly
// as process-census.py adapter_signatures' third tuple element. This is the
// value hashed per runtime.
func SignatureText(adapterPath string) (string, error) {
	out, err := exec.Command(adapterPath, "signature").Output()
	if err != nil {
		return "", fmt.Errorf("signature adapter failed: %s: %w", filepath.Base(adapterPath), err)
	}
	var normalized []string
	for _, raw := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if raw == "" || raw != strings.TrimSpace(raw) {
			return "", fmt.Errorf("malformed signature declaration from %s", filepath.Base(adapterPath))
		}
		verb, pattern, found := strings.Cut(raw, " ")
		if (verb != "match" && verb != "exclude") || !found || pattern == "" {
			return "", fmt.Errorf("malformed signature declaration from %s: %s", filepath.Base(adapterPath), raw)
		}
		normalized = append(normalized, raw)
	}
	if len(normalized) == 0 {
		return "", fmt.Errorf("signature adapter returned nothing: %s", filepath.Base(adapterPath))
	}
	return strings.Join(normalized, "\n") + "\n", nil
}

// Fingerprint computes the supervision fingerprint for a repo, hashing files
// and config from metasystemRoot. It matches process-census.py fingerprint.
func Fingerprint(metasystemRoot, repo string) (string, error) {
	repoReal, err := filepath.EvalSymlinks(repo)
	if err != nil {
		repoReal = repo
	}
	confPath := filepath.Join(metasystemRoot, "metasystem.conf")

	selected := splitRuntimes(config.ConfValue(confPath, "metasystem.runtimes", ""))
	files := append([]string(nil), fingerprintFiles...)
	for _, runtime := range selected {
		files = append(files, filepath.Join("scripts", "agents", "adapters", runtime+".sh"))
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
		text, err := SignatureText(filepath.Join(metasystemRoot, "scripts", "agents", "adapters", runtime+".sh"))
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

// canonicalJSON serializes v the way python's
// json.dumps(sort_keys=True, separators=(",", ":")) does: sorted keys,
// compact, and — crucially — WITHOUT Go's default HTML escaping of < > &,
// which python does not do. Inputs here are ASCII, matching python's
// ensure_ascii default.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder appends a newline; python's dumps does not.
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
