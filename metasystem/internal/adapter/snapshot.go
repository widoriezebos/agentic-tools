package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	keyHashRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	sequenceRe  = regexp.MustCompile(`^(\d{3})\.json$`)
	enforcement = []string{"writeRoots", "readRoots", "network"}
)

// WriteCapabilitySnapshot validates a runtime's probe result and writes it as
// the newest capability snapshot for that runtime, configuration, and day. It
// refuses a snapshot whose envelope-enforcement declaration is not exactly the
// three fields mapped to "mapped" or "notEnforced", and one whose configuration
// key hashes are not dotted paths to SHA-256 digests, because both are what a
// later dispatch trusts to decide whether the runtime can hold a restrictive
// permission. The written path is returned.
//
// The dated file name carries a per-day sequence; the snapshot is created with
// an exclusive open so two probes racing the same sequence cannot clobber one
// another's capture. transports, capabilities, permissions, envelope, and
// keyHashes are the JSON blobs the adapter assembled.
func WriteCapabilitySnapshot(dir, runtime, version, configHash, transports, capabilities, permissions, envelope, keyHashes string) (string, error) {
	transportsValue, err := parseBlob(transports, "transports")
	if err != nil {
		return "", err
	}
	capabilitiesValue, err := parseBlob(capabilities, "capabilities")
	if err != nil {
		return "", err
	}
	permissionsValue, err := parseBlob(permissions, "permissions")
	if err != nil {
		return "", err
	}
	envelopeValue, err := parseBlob(envelope, "envelope enforcement")
	if err != nil {
		return "", err
	}
	keyHashesValue, err := parseBlob(keyHashes, "configuration key hashes")
	if err != nil {
		return "", err
	}
	if !validEnvelopeEnforcement(envelopeValue) {
		return "", fmt.Errorf("envelope enforcement declaration must map writeRoots, readRoots, and network to mapped or notEnforced")
	}
	if !validKeyHashes(keyHashesValue) {
		return "", fmt.Errorf("configuration key hashes must map dotted paths to SHA-256 hashes")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	captured := now().UTC()
	date := captured.Format("20060102")
	prefix := fmt.Sprintf("%s-%s-%s-%s-", runtime, version, configHash, date)
	sequence := nextSequence(dir, prefix)

	name := fmt.Sprintf("%s%03d.json", prefix, sequence)
	path := filepath.Join(dir, name)
	value := map[string]any{
		"runtime":             runtime,
		"cliVersion":          version,
		"configHash":          configHash,
		"configKeyHashes":     keyHashesValue,
		"capturedAt":          captured.Format("2006-01-02T15:04:05") + "Z",
		"sequence":            sequence,
		"transports":          transportsValue,
		"capabilities":        capabilitiesValue,
		"permissions":         permissionsValue,
		"envelopeEnforcement": envelopeValue,
	}
	if err := exclusiveWriteJSON(path, value); err != nil {
		return "", err
	}
	return path, nil
}

// nextSequence is one past the highest three-digit sequence already written for
// this prefix, or 1 when none exists.
func nextSequence(dir, prefix string) int {
	highest := 0
	matches, _ := filepath.Glob(filepath.Join(dir, prefix+"*.json"))
	for _, match := range matches {
		rest := filepath.Base(match)[len(prefix):]
		if m := sequenceRe.FindStringSubmatch(rest); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > highest {
				highest = n
			}
		}
	}
	return highest + 1
}

func validEnvelopeEnforcement(value any) bool {
	object, ok := value.(map[string]any)
	if !ok || len(object) != len(enforcement) {
		return false
	}
	for _, field := range enforcement {
		v, ok := object[field].(string)
		if !ok || (v != "mapped" && v != "notEnforced") {
			return false
		}
	}
	return true
}

func validKeyHashes(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, raw := range object {
		digest, ok := raw.(string)
		if !ok || !keyHashRe.MatchString(digest) {
			return false
		}
	}
	return true
}

func parseBlob(raw, name string) (any, error) {
	value, err := decodeJSONBytes([]byte(raw))
	if err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", name, err)
	}
	return value, nil
}

// exclusiveWriteJSON creates path with an exclusive open so a colliding
// sequence fails instead of overwriting, then fsyncs the file and its directory
// so the snapshot survives a crash exactly as written or not at all.
func exclusiveWriteJSON(path string, value any) error {
	data, err := encodeJSON(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	syncDir(filepath.Dir(path))
	return nil
}
