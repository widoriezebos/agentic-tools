// Package dispatch owns the job-record lifecycle: the single writer of a
// job's control-plane record on disk. Every create, setup, protocol-error
// stamp, and compare-and-swap runs under one exclusive per-record lock so two
// dispatchers can never double-create or double-reap the same job, and every
// write lands atomically so a record is never observed half-written.
package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

var validJobID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// The lawful status graph. A record may only move along these edges; a
// self-edge (target == current) is a metadata update that carries no
// transition. Terminal states have no outgoing edges.
var statusTransitions = map[string]map[string]bool{
	"pending-setup": {"failed": true},
	"pending":       {"running": true, "failed": true, "cancelled": true},
	"running":       {"completed": true, "failed": true, "cancelled": true, "timeout": true},
}

// The states a job cannot leave. Reaching one stamps endedAt.
var terminalStatuses = map[string]bool{
	"completed": true, "failed": true, "cancelled": true, "timeout": true,
}

// Identity fields a running record fixes for its whole life; a patch that
// names any of them is refused.
var immutableFields = map[string]bool{
	"jobId": true, "role": true, "runtime": true, "round": true,
	"parentJob": true, "reviews": true, "workspaceRoot": true, "baseSha": true,
	"branch": true, "startedAt": true, "claimEpoch": true, "mainId": true,
	"capMin": true, "capDeadline": true, "capResolution": true,
}

// The only fields a terminal record still accepts: its evidence mirror, the
// chain-closure flags, the aggregated chain usage, and recorded critique
// exhaustions. Everything else about a finished job is final.
var terminalMetadataFields = map[string]bool{
	"mirror": true, "chainClosed": true, "chainUsage": true,
	"runnerClosed": true, "critiqueExhaustions": true,
}

// OpError carries the process exit code a lifecycle refusal must surface. An
// empty Message is a silent refusal (the caller reads the outcome from the
// exit code alone); a non-empty Message is printed to stderr.
type OpError struct {
	Code    int
	Message string
}

func (e *OpError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("dispatch record operation refused with code %d", e.Code)
}

func refuse(code int, format string, args ...any) *OpError {
	return &OpError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// silentRefusal reports a refusal whose only signal is the exit code.
func silentRefusal(code int) *OpError { return &OpError{Code: code} }

// paths resolves the jobs directory, the record file, and the per-record lock
// file for a job under a checkout root.
func paths(root, job string) (jobsDir, recordPath, lockPath string) {
	agents := filepath.Join(root, "artifacts", "agents")
	jobsDir = filepath.Join(agents, "jobs")
	recordPath = filepath.Join(jobsDir, job+".json")
	lockPath = filepath.Join(agents, "record-locks", job+".lock")
	return
}

// withRecordLock runs fn while holding the exclusive lock for one job's
// record. The lock is a single flock held for the whole read-decide-write
// cycle, so a concurrent dispatcher blocks rather than racing the same record.
func withRecordLock(root, job string, fn func(recordPath string) error) error {
	if !validJobID.MatchString(job) {
		return silentRefusal(2)
	}
	jobsDir, recordPath, lockPath := paths(root, job)
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	handle, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open record lock for %s: %w", job, err)
	}
	defer handle.Close()
	if err := unix.Flock(int(handle.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("cannot lock record for %s: %w", job, err)
	}
	defer unix.Flock(int(handle.Fd()), unix.LOCK_UN)
	return fn(recordPath)
}

// RecordCreate reserves a job by writing its initial pending-setup record.
// It refuses if a record already exists, so two dispatchers racing one id
// cannot both win. The source must already carry the job's own id and the
// pending-setup status.
func RecordCreate(root, job, sourcePath string) error {
	return withRecordLock(root, job, func(recordPath string) error {
		if _, err := os.Stat(recordPath); err == nil {
			return refuse(1, "job id collision: %s", job)
		} else if !os.IsNotExist(err) {
			return refuse(1, "cannot reserve job record %s: %v", job, err)
		}
		record, err := readObject(sourcePath)
		if err != nil {
			return refuse(1, "invalid initial record for %s: %v", job, err)
		}
		if asString(record["jobId"]) != job || asString(record["status"]) != "pending-setup" {
			return refuse(1, "invalid initial record identity or status for %s", job)
		}
		return writeRecord(recordPath, record)
	})
}

// RecordSetup completes a reservation: it swaps the pending-setup husk for the
// full pending record, refusing unless the current record is still
// pending-setup and the new record keeps the same job id, claim epoch, and
// main id. This is the create/setup handshake that makes reservation atomic.
func RecordSetup(root, job, sourcePath string) error {
	return withRecordLock(root, job, func(recordPath string) error {
		current, err := readObject(recordPath)
		if err != nil {
			return refuse(1, "cannot complete setup for job record %s: %v", job, err)
		}
		record, err := readObject(sourcePath)
		if err != nil {
			return refuse(1, "cannot complete setup for job record %s: %v", job, err)
		}
		if asString(current["status"]) != "pending-setup" ||
			asString(record["jobId"]) != job ||
			asString(record["status"]) != "pending" ||
			!sameValue(record["claimEpoch"], current["claimEpoch"]) ||
			!sameValue(record["mainId"], current["mainId"]) {
			return refuse(1, "invalid setup transition for %s", job)
		}
		return writeRecord(recordPath, record)
	})
}

// RecordProtocolError stamps a job failed with a protocol violation. It is
// idempotent: a record already failed with this exact violation is left
// untouched. The job must be pending or running to accept the stamp.
func RecordProtocolError(root, job, expect, violation, violationFile string) error {
	return withRecordLock(root, job, func(recordPath string) error {
		record, err := readObject(recordPath)
		if err != nil {
			return refuse(1, "cannot record protocol error for %s: %v", job, err)
		}
		if violation == "" && violationFile != "" {
			data, readErr := os.ReadFile(violationFile)
			if readErr != nil {
				return refuse(1, "cannot read protocol violation for %s: %v", job, readErr)
			}
			violation = strings.TrimSpace(string(data))
		}
		if violation == "" {
			return refuse(1, "protocol violation text is empty for %s", job)
		}
		key := violationKey(job, record["round"], violation)
		if asString(record["status"]) == "failed" && asString(record["error"]) == "protocol_error" {
			if existing, ok := record["protocolError"].(map[string]any); ok && asString(existing["key"]) == key {
				return nil
			}
		}
		if asString(record["status"]) != expect || (expect != "pending" && expect != "running") {
			return silentRefusal(3)
		}
		now := nowISO()
		record["status"] = "failed"
		record["error"] = "protocol_error"
		record["phase"] = "validation"
		record["protocolError"] = map[string]any{
			"key": key, "violation": violation, "detectedAt": now,
		}
		if isFalsy(record["endedAt"]) {
			record["endedAt"] = now
		}
		return writeRecord(recordPath, record)
	})
}

// RecordCAS is the compare-and-swap at the heart of the lifecycle: it applies
// patch and moves the record to target only if the record is still at expect.
// A lost compare returns the observed status on stdout and exit 3, so the
// caller can witness exactly what this atomic read saw. A self-target
// (target == expect) is a metadata update that changes fields without a
// transition. Reaching a terminal status stamps endedAt.
//
// The returned observed string is non-empty only on a lost compare.
func RecordCAS(root, job, expect, target, patchPath string) (observed string, err error) {
	err = withRecordLock(root, job, func(recordPath string) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return refuse(1, "cannot update job record %s: %v", job, readErr)
		}
		patchValue, patchErr := readJSON(patchPath)
		if patchErr != nil {
			return refuse(1, "cannot update job record %s: %v", job, patchErr)
		}
		current := asString(record["status"])
		if current != expect {
			// The atomic compare's own observation, so a later re-read cannot
			// disagree about what THIS compare saw.
			observed = "observed=" + current
			return silentRefusal(3)
		}
		metadataUpdate := current == target
		if !metadataUpdate && !statusTransitions[current][target] {
			return refuse(1, "illegal job transition: %s to %s", current, target)
		}
		patch, ok := patchValue.(map[string]any)
		if !ok {
			return refuse(1, "record patch must be an object and cannot contain status")
		}
		if _, has := patch["status"]; has {
			return refuse(1, "record patch must be an object and cannot contain status")
		}
		for field := range patch {
			if immutableFields[field] {
				return refuse(1, "record patch attempts to change immutable identity")
			}
		}
		if terminalStatuses[current] && metadataUpdate {
			for field := range patch {
				if !terminalMetadataFields[field] {
					return refuse(1, "terminal record metadata is final except mirror, closure, aggregate usage, and critique exhaustion")
				}
			}
		}
		for field, value := range patch {
			record[field] = value
		}
		record["status"] = target
		if terminalStatuses[target] && isFalsy(record["endedAt"]) {
			record["endedAt"] = nowISO()
		}
		return writeRecord(recordPath, record)
	})
	return observed, err
}

// violationKey derives the stable idempotency key that ties a protocol-error
// stamp to its job, round, and violation text.
func violationKey(job string, round any, violation string) string {
	sum := sha256.Sum256([]byte(job + roundToken(round) + violation))
	return hex.EncodeToString(sum[:])[:16]
}

// roundToken renders a record's round for the idempotency key: an absent round
// reads as the literal "None", a present round as its integer text.
func roundToken(round any) string {
	if round == nil {
		return "None"
	}
	if number, ok := round.(json.Number); ok {
		return number.String()
	}
	return fmt.Sprint(round)
}

// --- JSON read/write helpers ---

// readJSON parses a JSON document from a file, keeping numbers in their exact
// on-disk form so a re-serialized record is byte-stable.
func readJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// readObject parses a JSON object, failing when the document is not an object.
func readObject(path string) (map[string]any, error) {
	value, err := readJSON(path)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not a JSON object")
	}
	return object, nil
}

// writeRecord serializes a record and replaces its file atomically, matching
// the on-disk format every reader expects: two-space indent, sorted keys, one
// trailing newline, and no HTML escaping.
func writeRecord(recordPath string, record map[string]any) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(record); err != nil {
		return err
	}
	return atomicWriteText(recordPath, buf.Bytes())
}

// atomicWriteText writes bytes to a temp file in the target directory, fsyncs
// it, renames it into place, and fsyncs the directory so the record survives a
// crash exactly as written or not at all.
func atomicWriteText(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	if dir, err := os.Open(directory); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// --- value helpers ---

// asString returns the string value of v, or "" for any non-string.
func asString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// sameValue reports whether two decoded JSON values are equal for the identity
// fields the setup handshake pins (numbers, strings, and null).
func sameValue(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	an, aok := a.(json.Number)
	bn, bok := b.(json.Number)
	if aok && bok {
		return an.String() == bn.String()
	}
	return a == b
}

// isFalsy reports whether a value should be treated as unset for endedAt
// stamping: absent, null, or the empty string.
func isFalsy(v any) bool {
	switch typed := v.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case bool:
		return !typed
	default:
		return false
	}
}

// nowISO renders the current instant in the record's UTC second-precision
// timestamp format.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
