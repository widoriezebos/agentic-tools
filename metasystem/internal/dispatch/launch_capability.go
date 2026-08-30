package dispatch

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

const (
	delegateClaimCapabilityVersion = 1
	DelegateClaimCapabilityTTL     = 10 * time.Minute
)

const delegateClaimCapabilityKind = "delegate-claim-launch"

// DelegateClaimCapabilityBinding is the internal claim-launch request a
// delegate capability authorizes. The public delegate knows the dispatch mode
// when it mints the bearer word; claim-launch fixes the remaining identity at
// the single consumption point.
type DelegateClaimCapabilityBinding struct {
	JobID        string
	OperationID  string
	DispatchMode DispatchMode
	AdapterVerb  string
}

// MintDelegateClaimCapability creates the short-lived bearer word that lets
// one public delegate invocation cross the sealed claim-launch boundary.
func MintDelegateClaimCapability(root string, mode DispatchMode) (string, error) {
	return mintDelegateClaimCapability(root, mode, time.Now(), rand.Reader)
}

func mintDelegateClaimCapability(root string, mode DispatchMode, now time.Time, entropy io.Reader) (string, error) {
	if root == "" || (mode != DispatchModeFresh && mode != DispatchModeFollowUp) {
		return "", fmt.Errorf("delegate claim capability requires the checkout root and dispatch mode")
	}
	rawBytes := make([]byte, 32)
	if _, err := io.ReadFull(entropy, rawBytes); err != nil {
		return "", fmt.Errorf("cannot mint delegate claim capability: %w", err)
	}
	raw := hex.EncodeToString(rawBytes)
	digest := claimCapabilityDigest(raw)
	now = now.UTC().Truncate(time.Second)
	record := map[string]any{
		"schemaVersion": delegateClaimCapabilityVersion,
		"kind":          delegateClaimCapabilityKind,
		"digest":        digest,
		"dispatchMode":  string(mode),
		"status":        "minted",
		"mintedAt":      now.Format(time.RFC3339),
		"expiresAt":     now.Add(DelegateClaimCapabilityTTL).Format(time.RFC3339),
	}
	err := withDelegateClaimCapabilityStoreLock(root, func() error {
		path := delegateClaimCapabilityPath(root, digest)
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("delegate claim capability collision")
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot inspect delegate claim capability path: %w", err)
		}
		return writeRecord(path, record)
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// ValidateDelegateClaimCapability checks the preflight half of claim-launch
// without spending the bearer word. The authoritative claim repeats the
// check and consumes it under both the capability-store lock and the job's
// record lock.
func ValidateDelegateClaimCapability(root, raw string, binding DelegateClaimCapabilityBinding) error {
	return authorizeDelegateClaimCapability(root, raw, binding, time.Now(), false)
}

// ConsumeDelegateClaimCapability spends one delegate-minted bearer word and
// records the exact job, operation, dispatch mode, and adapter verb it served.
func ConsumeDelegateClaimCapability(root, raw string, binding DelegateClaimCapabilityBinding) error {
	return authorizeDelegateClaimCapability(root, raw, binding, time.Now(), true)
}

func authorizeDelegateClaimCapability(root, raw string, binding DelegateClaimCapabilityBinding, now time.Time, consume bool) error {
	if binding.OperationID == "" {
		binding.OperationID = binding.JobID
	}
	if err := validateDelegateClaimCapabilityBinding(binding); err != nil {
		return err
	}
	if len(raw) != 64 {
		return fmt.Errorf("delegate claim capability is invalid")
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return fmt.Errorf("delegate claim capability is invalid")
	}
	digest := claimCapabilityDigest(raw)
	return withDelegateClaimCapabilityStoreLock(root, func() error {
		verify := func() (map[string]any, string, error) {
			path := delegateClaimCapabilityPath(root, digest)
			record, err := readObject(path)
			if err != nil {
				return nil, path, fmt.Errorf("delegate claim capability is unavailable")
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
				return nil, path, fmt.Errorf("delegate claim capability storage is not private")
			}
			if err := verifyDelegateClaimCapabilityRecord(record, digest, binding.DispatchMode, now); err != nil {
				return nil, path, err
			}
			return record, path, nil
		}
		if !consume {
			_, _, err := verify()
			return err
		}
		return withRecordLock(root, binding.JobID, func(string) error {
			record, path, err := verify()
			if err != nil {
				return err
			}
			record["status"] = "consumed"
			record["consumedAt"] = now.UTC().Truncate(time.Second).Format(time.RFC3339)
			record["jobId"] = binding.JobID
			record["operationId"] = binding.OperationID
			record["adapterVerb"] = binding.AdapterVerb
			return writeRecord(path, record)
		})
	})
}

func validateDelegateClaimCapabilityBinding(binding DelegateClaimCapabilityBinding) error {
	if !validJobID.MatchString(binding.JobID) {
		return fmt.Errorf("delegate claim capability job id is invalid")
	}
	if !validJobID.MatchString(binding.OperationID) {
		return fmt.Errorf("delegate claim capability operation id is invalid")
	}
	switch binding.DispatchMode {
	case DispatchModeFresh:
		if binding.AdapterVerb != "dispatch" {
			return fmt.Errorf("a fresh delegate claim capability requires the dispatch adapter verb")
		}
	case DispatchModeFollowUp:
		if binding.AdapterVerb != "dispatch" && binding.AdapterVerb != "follow-up" {
			return fmt.Errorf("a follow-up delegate claim capability requires a launch adapter verb")
		}
	default:
		return fmt.Errorf("delegate claim capability dispatch mode is invalid")
	}
	return nil
}

func verifyDelegateClaimCapabilityRecord(record map[string]any, digest string, mode DispatchMode, now time.Time) error {
	version, versionOK := numInt(record["schemaVersion"])
	recordedDigest := asString(record["digest"])
	mintedAt, mintedErr := time.Parse(time.RFC3339, asString(record["mintedAt"]))
	expiresAt, expiresErr := time.Parse(time.RFC3339, asString(record["expiresAt"]))
	if len(record) != 7 || !versionOK || version != delegateClaimCapabilityVersion ||
		asString(record["kind"]) != delegateClaimCapabilityKind || asString(record["status"]) != "minted" ||
		asString(record["dispatchMode"]) != string(mode) || len(recordedDigest) != len(digest) ||
		subtle.ConstantTimeCompare([]byte(recordedDigest), []byte(digest)) != 1 || mintedErr != nil || expiresErr != nil ||
		!expiresAt.Equal(mintedAt.Add(DelegateClaimCapabilityTTL)) || now.Before(mintedAt) || !now.Before(expiresAt) {
		return fmt.Errorf("delegate claim capability is invalid, spent, or expired")
	}
	return nil
}

// RemoveDelegateClaimCapability erases a capability after its delegate
// process returns. A hard-killed delegate can leave a record behind, but its
// expiry remains authoritative and prevents later use.
func RemoveDelegateClaimCapability(root, raw string) error {
	if len(raw) != 64 {
		return nil
	}
	digest := claimCapabilityDigest(raw)
	return withDelegateClaimCapabilityStoreLock(root, func() error {
		err := os.Remove(delegateClaimCapabilityPath(root, digest))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	})
}

func claimCapabilityDigest(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

func delegateClaimCapabilityPath(root, digest string) string {
	return filepath.Join(root, "artifacts", "agents", "capabilities", "delegate-claim", digest+".json")
}

func withDelegateClaimCapabilityStoreLock(root string, fn func() error) error {
	dir := filepath.Join(root, "artifacts", "agents", "capabilities", "delegate-claim")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create delegate claim capability store: %w", err)
	}
	lockPath := filepath.Join(dir, ".store.lock")
	handle, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("cannot open delegate claim capability store lock: %w", err)
	}
	defer handle.Close()
	deadline := time.Now().Add(recordLockWait())
	for {
		err = unix.Flock(int(handle.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("delegate claim capability store is busy after %s", recordLockWait())
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer unix.Flock(int(handle.Fd()), unix.LOCK_UN)
	return fn()
}

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
