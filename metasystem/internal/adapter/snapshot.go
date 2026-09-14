package adapter

import (
	"encoding/json"
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
	if err := validateStopDeliveryCapability(capabilitiesValue); err != nil {
		return "", err
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
		"capturedAt":          timestampUTC(captured),
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

func validateStopDeliveryCapability(capabilities any) error {
	object, ok := capabilities.(map[string]any)
	if !ok {
		return fmt.Errorf("capabilities must be a JSON object")
	}
	raw, present := object["stopDelivery"]
	if !present {
		return nil
	}
	delivery, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("capabilities.stopDelivery must be an object")
	}
	required := []string{"schemaVersion", "envelope", "blockField", "blockValue", "blockTextField", "allowTextField", "humanVisibleFields", "duplicateBehavior", "reportReadRoute", "instructionHash", "trustProbe", "observationArtifact", "launchBinary", "seatCommandBinary", "continuationLimit", "level"}
	if len(delivery) != len(required) {
		return fmt.Errorf("capabilities.stopDelivery must contain exactly the version 1 fields")
	}
	for _, field := range required {
		if _, ok := delivery[field]; !ok {
			return fmt.Errorf("capabilities.stopDelivery is missing %s", field)
		}
	}
	version, ok := delivery["schemaVersion"].(json.Number)
	if !ok || version.String() != "1" {
		return fmt.Errorf("capabilities.stopDelivery.schemaVersion must be 1")
	}
	stringField := func(name string) (string, bool) { value, ok := delivery[name].(string); return value, ok }
	for _, field := range []string{"envelope", "blockField", "blockValue", "blockTextField", "allowTextField", "duplicateBehavior", "reportReadRoute", "instructionHash", "trustProbe", "observationArtifact", "launchBinary", "seatCommandBinary", "level"} {
		if _, ok := stringField(field); !ok {
			return fmt.Errorf("capabilities.stopDelivery.%s must be a string", field)
		}
	}
	envelope, _ := stringField("envelope")
	blockField, _ := stringField("blockField")
	blockValue, _ := stringField("blockValue")
	blockText, _ := stringField("blockTextField")
	allowText, _ := stringField("allowTextField")
	visibleRaw, ok := delivery["humanVisibleFields"].([]any)
	if !ok {
		return fmt.Errorf("capabilities.stopDelivery.humanVisibleFields must be a string array")
	}
	visible := make([]string, 0, len(visibleRaw))
	for _, raw := range visibleRaw {
		value, ok := raw.(string)
		if !ok {
			return fmt.Errorf("capabilities.stopDelivery.humanVisibleFields must be a string array")
		}
		visible = append(visible, value)
	}
	if envelope == "shared-reason-v1" {
		if blockField != "decision" || blockValue != "block" || blockText != "reason" || allowText != "systemMessage" {
			return fmt.Errorf("capabilities.stopDelivery shared-reason-v1 fields do not match the envelope")
		}
		if len(visible) != 2 || visible[0] != "reason" || visible[1] != "systemMessage" {
			return fmt.Errorf("capabilities.stopDelivery shared-reason-v1 visible fields must be reason and systemMessage")
		}
	} else if envelope == "unverified" {
		if blockField != "" || blockValue != "" || blockText != "" || allowText != "" || len(visible) != 0 {
			return fmt.Errorf("capabilities.stopDelivery unverified envelope must leave fields and visible fields empty")
		}
	} else {
		return fmt.Errorf("capabilities.stopDelivery.envelope must be shared-reason-v1 or unverified")
	}
	duplicate, _ := stringField("duplicateBehavior")
	if duplicate != "single" && duplicate != "duplicate" && duplicate != "unknown" {
		return fmt.Errorf("capabilities.stopDelivery.duplicateBehavior is invalid")
	}
	readRoute, _ := stringField("reportReadRoute")
	if readRoute != "standing-instruction-and-command" && readRoute != "unknown" {
		return fmt.Errorf("capabilities.stopDelivery.reportReadRoute is invalid")
	}
	instructionHash, _ := stringField("instructionHash")
	if instructionHash != "" && !keyHashRe.MatchString(instructionHash) {
		return fmt.Errorf("capabilities.stopDelivery.instructionHash must be empty or SHA-256")
	}
	for _, field := range []string{"launchBinary", "seatCommandBinary"} {
		value, _ := stringField(field)
		if value != "" && !filepath.IsAbs(value) {
			return fmt.Errorf("capabilities.stopDelivery.%s must be empty or absolute", field)
		}
	}
	if limit := delivery["continuationLimit"]; limit != nil {
		number, ok := limit.(json.Number)
		parsed, err := strconv.Atoi(number.String())
		if !ok || err != nil || parsed < 1 {
			return fmt.Errorf("capabilities.stopDelivery.continuationLimit must be null or a positive integer")
		}
	}
	level, _ := stringField("level")
	if level != "unobserved" && level != "emitted" && level != "observed" {
		return fmt.Errorf("capabilities.stopDelivery.level is invalid")
	}
	trust, _ := stringField("trustProbe")
	artifact, _ := stringField("observationArtifact")
	launch, _ := stringField("launchBinary")
	seat, _ := stringField("seatCommandBinary")
	if level == "unobserved" && (instructionHash != "" || trust != "" || artifact != "" || launch != "" || seat != "" || duplicate != "unknown" || readRoute != "unknown") {
		return fmt.Errorf("capabilities.stopDelivery unobserved level must not claim installation or observation evidence")
	}
	if level == "emitted" && (envelope == "unverified" || instructionHash == "" || artifact == "" || launch == "" || seat == "") {
		return fmt.Errorf("capabilities.stopDelivery emitted level lacks installed emission evidence")
	}
	if level == "observed" {
		if envelope == "unverified" || instructionHash == "" || trust == "" || artifact == "" || launch == "" || seat == "" || duplicate == "unknown" || readRoute == "unknown" {
			return fmt.Errorf("capabilities.stopDelivery observed level lacks observation evidence")
		}
	}
	return nil
}

func unobservedStopDeliveryCapability(envelope string) map[string]any {
	delivery := map[string]any{
		"schemaVersion": 1, "envelope": envelope,
		"blockField": "", "blockValue": "", "blockTextField": "", "allowTextField": "",
		"humanVisibleFields": []any{}, "duplicateBehavior": "unknown", "reportReadRoute": "unknown",
		"instructionHash": "", "trustProbe": "", "observationArtifact": "", "launchBinary": "", "seatCommandBinary": "",
		"continuationLimit": nil, "level": "unobserved",
	}
	if envelope == "shared-reason-v1" {
		delivery["blockField"] = "decision"
		delivery["blockValue"] = "block"
		delivery["blockTextField"] = "reason"
		delivery["allowTextField"] = "systemMessage"
		delivery["humanVisibleFields"] = []any{"reason", "systemMessage"}
	}
	return delivery
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
