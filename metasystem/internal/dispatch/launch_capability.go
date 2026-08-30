package dispatch

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ConsumeLaunchCapability spends the one launch word minted by delegate
// admission. Verification and consumption share the record lock, so replay
// cannot reach a provider fork.
func ConsumeLaunchCapability(root, job, raw, adapterVerb, instanceTag string, supervisorPID int64, reader identity.StartReader) error {
	if raw == "" || adapterVerb == "" || instanceTag == "" || supervisorPID < 1 || reader == nil {
		return fmt.Errorf("adapter launch capability requires the capability, verb, instance tag, and supervisor pid")
	}
	observed, state, err := reader.ReadStart(supervisorPID)
	if err != nil || state != identity.Alive || observed.Pid != supervisorPID || !observed.Ref().NativeExact() {
		return fmt.Errorf("adapter launch supervisor identity is not exact and live")
	}
	digest := sha256.Sum256([]byte(raw))
	encodedDigest := hex.EncodeToString(digest[:])
	return withRecordLock(root, job, func(recordPath string) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return fmt.Errorf("cannot read launch capability record: %w", readErr)
		}
		if asString(record["status"]) != "pending" {
			return fmt.Errorf("adapter launch capability requires a pending job")
		}
		capability, ok := record["launchCapability"].(map[string]any)
		if !ok || len(capability) < 7 {
			return fmt.Errorf("job has no admitted adapter launch capability")
		}
		if asString(capability["status"]) != "minted" {
			return fmt.Errorf("adapter launch capability was already consumed")
		}
		if asString(capability["jobId"]) != job || asString(capability["operationId"]) != recordOperationID(record) ||
			asString(capability["instanceTag"]) != instanceTag || asString(record["instanceTag"]) != instanceTag ||
			asString(capability["adapterVerb"]) != adapterVerb {
			return fmt.Errorf("adapter launch capability does not bind this job, operation, tag, and verb")
		}
		recordedDigest := asString(capability["digest"])
		if len(recordedDigest) != len(encodedDigest) || subtle.ConstantTimeCompare([]byte(recordedDigest), []byte(encodedDigest)) != 1 {
			return fmt.Errorf("adapter launch capability is invalid")
		}
		capability["status"] = "consumed"
		capability["consumedAt"] = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
		capability["supervisor"] = exactIdentityFields(observed.Ref())
		record["launchCapability"] = capability
		return writeRecord(recordPath, record)
	})
}
