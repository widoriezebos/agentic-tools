// Package dispatch owns the delegate-job control plane. The record
// lifecycle in this file is its spine — the single writer of a job's
// record on disk, where every create, setup, protocol-error stamp, and
// compare-and-swap runs under one exclusive per-record lock and lands
// atomically. Around it the package carries the rest of the dispatch
// decisions: attestation gates (attest.go), permission-envelope
// expansion (envelope.go), mission-lease proof (mission.go), evidence
// mirroring and chain close proof (mirror.go, close.go),
// critique-exhaustion policy (critique.go), the dispatch owner lock
// (ownerlock.go), brief and cap parsing (brief.go), and chain usage
// accounting (usage.go).
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
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
	"golang.org/x/sys/unix"
)

var validJobID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// The lawful status graph. A record may only move along these edges; a
// self-edge (target == current) is a metadata update that carries no
// transition. Terminal states have no outgoing edges.
var statusTransitions = map[string]map[string]bool{
	"pending-setup": {"failed": true, "cancelled": true},
	"pending":       {"running": true, "failed": true, "cancelled": true},
	"running":       {"completed": true, "failed": true, "cancelled": true, "timeout": true},
}

// The states a job cannot leave. Reaching one stamps endedAt.
var terminalStatuses = map[string]bool{
	"completed": true, "failed": true, "cancelled": true, "timeout": true,
}

// TerminalStatus reports whether a job status is one a record cannot leave.
// This predicate is the vocabulary's one exported home;
// consumers outside dispatch must not re-declare the set.
func TerminalStatus(status string) bool { return terminalStatuses[status] }

// Identity fields a running record fixes for its whole life; a patch that
// names any of them is refused. Mission provenance — mission, incarnation,
// turn, stream — is immutable because the host-implementer wall's
// integration authorizations bind these exact values at issue time:
// a record that could rewrite its
// provenance could launder work across missions or turns.
var immutableFields = map[string]bool{
	"jobId": true, "role": true, "runtime": true, "round": true,
	"parentJob": true, "reviews": true, "workspaceRoot": true, "baseSha": true,
	"branch": true, "startedAt": true, "claimEpoch": true, "mainId": true,
	"capMin": true, "capDeadline": true, "capResolution": true,
	"mission": true, "missionIncarnation": true, "turnId": true, "stream": true,
	"operationId": true, "goalId": true, "goalRevision": true, "machineId": true,
	"approvedRef": true,
}

// Owned metadata has a dedicated read-decide-write operation whose lock
// protects invariants that a status-only compare cannot express. Generic
// record patches must not bypass that owner.
var dedicatedMetadataFields = map[string]bool{
	"findingRegister":      true,
	"findingRegisterRound": true,
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
	// Bounded like the lease lock: production record
	// operations run through lease run-held, which holds the GLOBAL lease
	// lock across this acquire — a record-lock holder wedged mid-hold
	// (SIGSTOP, fsync stall, an inherited flock fd) would otherwise block
	// here forever and every subsequent claim, renew, and succession
	// would refuse at its own bound for as long as the wedge lasts.
	deadline := time.Now().Add(recordLockWait())
	for {
		err := unix.Flock(int(handle.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("record lock for %s is busy after %s; a wedged holder keeps it", job, recordLockWait())
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer unix.Flock(int(handle.Fd()), unix.LOCK_UN)
	return fn(recordPath)
}

// recordLockWait bounds record-lock acquisition, honoring the same env
// override the lease lock's bound honors so fixtures tune one knob.
func recordLockWait() time.Duration {
	if v := os.Getenv("METASYSTEM_LEASE_LOCK_WAIT_SEC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return time.Duration(f * float64(time.Second))
		}
	}
	return 10 * time.Second
}

// freshRoundSuffixRe captures a round-styled id suffix (-rN).
var freshRoundSuffixRe = regexp.MustCompile(`-r([0-9]+)$`)

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
		// A FRESH chain root must not be named like a later round:
		// round identity is the record's, never the id's, and a new
		// job called <name>-r2 briefs its delegate as round 2 while the
		// record says 1 — the return then dies on the identity mismatch
		// after the tokens are spent. Rounds continue by resuming the
		// chain; only resume records (parentJob set) may carry -rN ids
		// beyond r1.
		if record["parentJob"] == nil {
			if m := freshRoundSuffixRe.FindStringSubmatch(job); m != nil {
				// Fail CLOSED on an unparseable suffix: an
				// overflow literal matches the regex, errs in Atoi, and
				// would walk straight through the guard.
				if n, err := strconv.Atoi(m[1]); err != nil || n >= 2 {
					return refuse(1, "a fresh job must not claim round %s in its name (%s); continue the existing chain with a follow-up instead", m[1], job)
				}
			}
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
			!sameValue(record["mainId"], current["mainId"]) ||
			!sameValue(record["operationId"], current["operationId"]) ||
			!sameValue(record["goalId"], current["goalId"]) ||
			!sameValue(record["goalRevision"], current["goalRevision"]) ||
			!sameValue(record["machineId"], current["machineId"]) ||
			!sameValue(record["approvedRef"], current["approvedRef"]) ||
			!sameValue(record["capMin"], current["capMin"]) {
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
		// The cancellation guard has no back door: a marked record's
		// only forward path is cancelled, and the protocol-error
		// stamp would both flip it failed and erase the marker. The
		// violation evidence is already durable on disk; the stamp
		// defers exactly like a lost compare.
		if asString(record["phase"]) == "cancelling" {
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
		// A cancellation in progress voids any competing advance,
		// HERE — the one lock every writer passes through. A loss
		// verdict would conclude the TERMed group failed; a
		// handshake would erase the marker; a launch's ownership
		// write (pending→pending) would record a fresh group and
		// open the start gate for work the operator already stopped.
		// Once cancelling, the ONLY writes that proceed are the
		// conclude to cancelled and a genuine completion that beat
		// the kill; everything else — loss verdicts, the handshake,
		// same-status metadata, ownership — defers like a lost
		// compare.
		if asString(record["phase"]) == "cancelling" &&
			target != "cancelled" && target != "completed" {
			observed = "observed=cancelling"
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
			if dedicatedMetadataFields[field] {
				return refuse(1, "record patch attempts to change metadata owned by a dedicated operation")
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

// readEnvelope reads a record through the wire-document owner: the same
// frozen grammar as readJSON, carried by the package whose tests pin it.
// readObject delegates here so the grammar has ONE owner.
func readEnvelope(path string) (*wiredoc.Doc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return wiredoc.Decode(data)
}

// readObject parses a JSON object, failing when the document is not an object.
func readObject(path string) (map[string]any, error) {
	doc, err := readEnvelope(path)
	if err != nil {
		return nil, err
	}
	return doc.Raw(), nil
}

// ReadRecordObject is the exported record reader for CLI relays that pass a
// record into a package decision (verify-chain-incarnation).
func ReadRecordObject(path string) (map[string]any, error) {
	record, err := readObject(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read the job record: %v", err)
	}
	return record, nil
}

// writeRecord serializes a record and replaces its file atomically, matching
// the on-disk format every reader expects: two-space indent, sorted keys, one
// trailing newline, and no HTML escaping.
func writeRecord(recordPath string, record map[string]any) error {
	// Rendered by the wire-document owner: the corpus
	// equivalence test proves these bytes identical to the encoder this
	// replaces, and the capture test re-diffs the golden corpus on every
	// run, so a drift in either writer fails before it ships.
	rendered, err := wiredoc.FromRaw(record).Render()
	if err != nil {
		return err
	}
	_, writeErr := atomicWriteText(recordPath, rendered)
	return writeErr
}

// atomicWriteText writes bytes through the durable-write owner.
// Job-record paths — the durable STATE this
// package owns — adopt the two-outcome contract: the
// anchor is the checkout root derived from the path itself (the parent
// of artifacts/), and a publication whose directory sync failed is
// WITNESSED as doubt rather than silently trusted. Transient hand-off
// files (build staging, envelopes) keep the empty anchor.
func atomicWriteText(path string, data []byte) (durable bool, err error) {
	anchor := artifactsAnchor(path)
	durable, err = atomicfile.WriteText(path, string(data), anchor)
	if err == nil && !durable && anchor != "" {
		fmt.Fprintf(os.Stderr, "durability doubt: %s published without directory sync\n", path)
	}
	return durable, err
}

// artifactsAnchor derives the durable-chain anchor for a path under a
// checkout's artifacts/ tree: the checkout root, which pre-exists by
// construction. Paths outside an artifacts tree anchor nowhere.
func artifactsAnchor(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	marker := "/artifacts/"
	index := strings.LastIndex(clean, marker)
	if index <= 0 {
		return ""
	}
	return filepath.FromSlash(clean[:index])
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

// RepairClaim atomically claims the round's ONE paid repair:
// under the record lock it requires
// status running and returnRepairs absent-or-zero — ABSENT MEANS ZERO,
// because the record builders do not initialize the field and should
// not have to — then stamps returnRepairs to 1 in the same write. The
// claim precedes the paid provider call by contract; recording after
// the call would let a crash leave
// durable state saying no repair happened. Exit taxonomy via the
// returned observed string and error: nil error with empty observed is
// a won claim; a lost claim (already claimed, or not running) returns
// observed non-empty for exit 3 — a DELEGATE-side outcome; a mechanical
// failure (unreadable record, lock) returns an error for exit 1 — a
// HARNESS failure that must never masquerade as delegate emptiness.
func RepairClaim(root, job string) (observed string, err error) {
	err = withRecordLock(root, job, func(recordPath string) error {
		record, readErr := readObject(recordPath)
		if readErr != nil {
			return refuse(1, "cannot read job record %s: %v", job, readErr)
		}
		status := asString(record["status"])
		if status != "running" {
			observed = "observed=status=" + status
			return silentRefusal(3)
		}
		// No writer advances a marked record except the conclude: a
		// repair claim after the mark would authorize paid repair
		// work for a job the operator already stopped.
		if asString(record["phase"]) == "cancelling" {
			observed = "observed=cancelling"
			return silentRefusal(3)
		}
		repairs, has := record["returnRepairs"]
		if has && !isZeroNumber(repairs) {
			observed = "observed=returnRepairs-claimed"
			return silentRefusal(3)
		}
		record["returnRepairs"] = 1
		return writeRecord(recordPath, record)
	})
	return observed, err
}

// isZeroNumber reports whether a decoded JSON value is the number zero,
// tolerating the json.Number and float64 spellings the readers produce.
func isZeroNumber(value any) bool {
	switch typed := value.(type) {
	case json.Number:
		return typed.String() == "0"
	case float64:
		return typed == 0
	case int:
		return typed == 0
	}
	return false
}
