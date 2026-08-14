package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"time"
)

// Dispatch refuses to launch under dead or stale supervision. These are the
// two attestation gates: a fresh census verdict that attests the current
// supervision state, and a live watcher heartbeat that attests the cap
// ceiling it enforces.

var hexDigest64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// CensusFresh verifies the last census verdict is trustworthy for a dispatch:
// well-formed, successful, generation-matched to the current arming record,
// attesting that record's exact bytes, young enough, and — when
// expectedFingerprint is non-empty — carrying the fingerprint of the armed
// code, signatures, and configuration (script-orchestration-12: the whole
// gate renders ONE verdict; the fingerprint half used to be shell string
// equality). armHint and repoHint only name the re-arm command in refusals.
func CensusFresh(verdictPath, statePath, armHint, repoHint, expectedFingerprint string, now time.Time) error {
	verdictBytes, err := os.ReadFile(verdictPath)
	if err != nil {
		return fmt.Errorf("dispatch refused: census verdict is unreadable: %v", err)
	}
	decoded, err := decodeJSONValue(verdictBytes)
	if err != nil {
		return fmt.Errorf("dispatch refused: census verdict is unreadable: %v", err)
	}
	value, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("dispatch refused: census verdict schema or writer is invalid")
	}
	for _, required := range []string{
		"schemaVersion", "writer", "verdict", "completedAtEpoch", "intervalSec",
		"fingerprint", "counts", "inventory", "diagnostics", "errors",
	} {
		if _, present := value[required]; !present {
			return fmt.Errorf("dispatch refused: census verdict schema or writer is invalid")
		}
	}
	if schema, ok := numFloat(value["schemaVersion"]); !ok || schema != 2 || asString(value["writer"]) != "watch-background-jobs.sh" {
		return fmt.Errorf("dispatch refused: census verdict schema or writer is invalid")
	}
	switch asString(value["verdict"]) {
	case "CENSUS-FAILED":
		return fmt.Errorf("dispatch refused: last census verdict is CENSUS-FAILED")
	case "SUCCESS":
	default:
		return fmt.Errorf("dispatch refused: census verdict is not successful")
	}
	completed, completedOK := numInt(value["completedAtEpoch"])
	interval, intervalOK := numInt(value["intervalSec"])
	if !completedOK || !intervalOK || interval < 1 {
		return fmt.Errorf("dispatch refused: census freshness fields are invalid")
	}
	age := now.Unix() - completed
	window := 2 * interval
	if window > 180 {
		window = 180
	}
	generation, generationOK := numInt(value["generation"])
	digest := asString(value["stateDigest"])
	if !generationOK || generation < 1 || !hexDigest64.MatchString(digest) {
		return fmt.Errorf("dispatch refused: census generation fields are invalid")
	}
	armedBytes, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("dispatch refused: arming record is unreadable: %v", err)
	}
	armedDecoded, err := decodeJSONValue(armedBytes)
	if err != nil {
		return fmt.Errorf("dispatch refused: arming record is unreadable: %v", err)
	}
	armed, _ := armedDecoded.(map[string]any)
	armedGeneration, armedOK := numInt(armed["generation"])
	if !armedOK || armedGeneration < 1 {
		return fmt.Errorf("dispatch refused: arming record generation is invalid")
	}
	if generation != armedGeneration {
		// The arming-window transient (script-validate-3/D34): the one
		// refusal that is safe to retry before a job record exists gets a
		// TYPE, so callers branch on a contract instead of grepping the
		// diagnostic's wording. The message bytes are unchanged.
		return ArmingWindowError{msg: fmt.Sprintf(
			"dispatch refused: census verdict is stale (age=%ds window=%ds censusGeneration=%d armedGeneration=%d); retry in a moment; re-arm with %s --repo %s if supervision is dead",
			age, window, generation, armedGeneration, armHint, repoHint)}
	}
	armedDigest := sha256.Sum256(armedBytes)
	if hex.EncodeToString(armedDigest[:]) != digest {
		return fmt.Errorf("dispatch refused: census verdict does not attest the current supervision state")
	}
	if age >= window {
		return fmt.Errorf(
			"dispatch refused: census verdict is stale (age=%ds window=%ds); retry in a moment; re-arm with %s --repo %s if supervision is dead",
			age, window, armHint, repoHint)
	}
	if expectedFingerprint != "" && asString(value["fingerprint"]) != expectedFingerprint {
		return fmt.Errorf("dispatch refused: census fingerprint does not match the armed code, signatures, and configuration")
	}
	return nil
}

// WatcherCeiling returns the cap ceiling the armed watcher attests to
// enforcing: the loadedCapMin from a heartbeat that matches the armed
// watcher's exact identity and is fresh. A dispatch cap must stay below this
// ceiling, so a dead or stale watcher refuses every dispatch.
func WatcherCeiling(statePath string, now time.Time) (int64, error) {
	state, err := readObject(statePath)
	if err != nil {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is unreadable: %v", err)
	}
	components, _ := state["components"].(map[string]any)
	watcher, ok := components["watcher"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is unreadable: no armed watcher component")
	}
	heartbeatPath, ok := watcher["heartbeat"].(string)
	if !ok {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is unreadable: no watcher heartbeat path")
	}
	heartbeat, err := readObject(heartbeatPath)
	if err != nil {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is unreadable: %v", err)
	}
	for _, key := range []string{"pid", "pidStartedAt", "instanceTag"} {
		if !looseEqual(heartbeat[key], watcher[key]) {
			return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation identity does not match the armed watcher")
		}
	}
	ceiling, ok := numInt(heartbeat["loadedCapMin"])
	if !ok || ceiling < 1 {
		return 0, fmt.Errorf("dispatch refused: watcher heartbeat has no valid loadedCapMin attestation")
	}
	observed, observedOK := numInt(heartbeat["observedAtEpoch"])
	interval, intervalOK := numInt(state["intervalSec"])
	if !observedOK || !intervalOK {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is stale")
	}
	age := now.Unix() - observed
	if age < -5 || age > interval*2+2 {
		return 0, fmt.Errorf("dispatch refused: watcher ceiling attestation is stale")
	}
	return ceiling, nil
}

// ArmingWindowError marks the between-arming-publication-and-census
// transient: the only census refusal that is safe to retry before a job
// record exists. The census-fresh verb surfaces it as exit code 9.
type ArmingWindowError struct{ msg string }

func (e ArmingWindowError) Error() string { return e.msg }
