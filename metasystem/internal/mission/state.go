package mission

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Mission state is a strictly-shaped, hash-chained JSON document: every write
// appends a new integrity entry whose hash covers the canonical bytes of the
// state with the hash and history blanked, so a tampered or forked state is
// detectable. This file owns creating it from a sealed contract, the
// compare-and-write that advances it under a lock, and validation of its
// shape, aggregation invariants, and chain.

var (
	idRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	hashRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

var streamStates = map[string]bool{
	"active": true, "parked-reserved": true, "parked-stop-loss": true, "done": true,
}

var parkReasons = map[string]bool{
	"all-streams-parked": true, "stop-loss": true, "fence": true, "gate-integrity": true,
	"state-integrity": true, "contract-changed": true, "host-failure": true,
	"drain-stalled": true, "wall-violation": true,
}

var legalStreamTransitions = map[string]map[string]bool{
	"active":           {"active": true, "parked-reserved": true, "parked-stop-loss": true, "done": true},
	"parked-reserved":  {"parked-reserved": true, "active": true},
	"parked-stop-loss": {"parked-stop-loss": true, "active": true},
	"done":             {"done": true},
}

// StateError is a mission-state validation or operation failure.
type StateError struct{ msg string }

func (e *StateError) Error() string { return e.msg }

func stateErr(format string, args ...any) error {
	return &StateError{msg: fmt.Sprintf(format, args...)}
}

// canonicalBytes renders a value as compact, key-sorted JSON with a trailing
// newline — the bytes the state hash covers.
func canonicalBytes(value any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value) // Encode appends the newline
	return buf.Bytes()
}

// deepCopyDoc deep-copies a JSON document, preserving numbers as json.Number.
func deepCopyDoc(value any) any {
	data, _ := json.Marshal(value)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out any
	_ = dec.Decode(&out)
	return out
}

// stateHash is the sha256 of the state's canonical bytes with the integrity
// hash and history blanked, so the hash certifies everything but itself.
func stateHash(state map[string]any) string {
	body, _ := deepCopyDoc(state).(map[string]any)
	integrity, _ := body["integrity"].(map[string]any)
	if integrity == nil {
		integrity = map[string]any{}
		body["integrity"] = integrity
	}
	integrity["hash"] = ""
	integrity["history"] = []any{}
	sum := sha256.Sum256(canonicalBytes(body))
	return hex.EncodeToString(sum[:])
}

func readStateDoc(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, stateErr("cannot read mission state: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, stateErr("cannot read mission state: %v", err)
	}
	doc, ok := value.(map[string]any)
	if !ok {
		return nil, stateErr("mission state must be a JSON object")
	}
	return doc, nil
}

func atomicWriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return err
	}
	return atomicWriteText(path, buf.String())
}

// --- numeric helpers over a JSON document ---

func intValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		if strings.ContainsAny(n.String(), ".eE") {
			return 0, false
		}
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return int64(n), n == math.Trunc(n)
	}
	return 0, false
}

func isNonnegInt(v any) bool {
	i, ok := intValue(v)
	return ok && i >= 0
}

func floatValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func exactKeys(m map[string]any, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// stateTopLevelKeys checks the top-level state shape: every required field
// present, nothing unknown, and two optional fields — ledgerSemantics, absent
// on missions initialized before the replay semantics existed, and
// lastDrainStall, the durable label a drain-stalled unpark writes and the
// resume heal consumes; legacy missions never carry it.
func stateTopLevelKeys(state map[string]any) bool {
	required := []string{"schemaVersion", "missionId", "branch", "status", "parkReason",
		"gatePassed", "streams", "fences", "turnLog", "waitingList", "runnerLease", "ledger", "integrity",
		"openTurn", "workspaceTaint"}
	allowed := map[string]bool{"ledgerSemantics": true, "lastDrainStall": true}
	for _, key := range required {
		if _, ok := state[key]; !ok {
			return false
		}
		allowed[key] = true
	}
	for key := range state {
		if !allowed[key] {
			return false
		}
	}
	return true
}

func nonEmptyString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok && s != ""
}

// --- validation ---

func validateShape(state map[string]any) error {
	if !stateTopLevelKeys(state) {
		return stateErr("mission state has missing or unexpected top-level fields")
	}
	if raw, present := state["ledgerSemantics"]; present {
		// Pinned at mission init by the binary that sealed the ledger's
		// meaning (docs/design/stop-loss-core.md); absent on legacy missions.
		if v, ok := intValue(raw); !ok || v < 1 {
			return stateErr("mission state ledgerSemantics must be a positive integer")
		}
	}
	if raw, present := state["lastDrainStall"]; present {
		// Written by the drain-stalled unpark, consumed by the resume heal
		// (plans/patience-mission-reap-drain.md): the cycle the stall parked
		// and the survivor ids the park ask snapshotted.
		if err := validateLastDrainStall(raw); err != nil {
			return err
		}
	}
	schemaVersion, _ := intValue(state["schemaVersion"])
	if schemaVersion != 2 && schemaVersion != 3 {
		return stateErr("mission state schema version or mission id is invalid")
	}
	// The DOWNGRADE BARRIER (issue-4 round 2): a semantics-3 mission is
	// always schema 3, because post-wall pre-semantics-3 binaries accept
	// schema 2 with any positive ledgerSemantics and would mutate the
	// state and write wrong best markers before their verdict-time
	// refusal. Those binaries refuse schema 3 at every read — the
	// barrier they already understand.
	if semantics, ok := intValue(state["ledgerSemantics"]); ok && semantics >= 3 && schemaVersion < 3 {
		return stateErr("mission state ledgerSemantics %d requires schemaVersion 3", semantics)
	}
	missionID, ok := state["missionId"].(string)
	if !ok || !idRe.MatchString(missionID) {
		return stateErr("mission state schema version or mission id is invalid")
	}
	if _, ok := nonEmptyString(state["branch"]); !ok {
		return stateErr("mission state branch is invalid")
	}
	if s, _ := state["status"].(string); s != "running" && s != "completed" && s != "parked" {
		return stateErr("mission status is invalid")
	}
	if state["parkReason"] != nil {
		if s, ok := state["parkReason"].(string); !ok || !parkReasons[s] {
			return stateErr("mission park reason is invalid")
		}
	}
	if _, ok := state["gatePassed"].(bool); !ok {
		return stateErr("mission gatePassed must be boolean")
	}
	streams, ok := state["streams"].(map[string]any)
	if !ok || len(streams) == 0 {
		return stateErr("mission streams must be a non-empty object")
	}
	for streamID, raw := range streams {
		stream, ok := raw.(map[string]any)
		if !ok || !idRe.MatchString(streamID) {
			return stateErr("mission stream identity is invalid")
		}
		if !exactKeys(stream, "goal", "state", "reason", "answeredAsk") {
			return stateErr("mission stream %s has missing or unexpected fields", streamID)
		}
		if _, ok := nonEmptyString(stream["goal"]); !ok {
			return stateErr("mission stream %s goal or state is invalid", streamID)
		}
		streamState, _ := stream["state"].(string)
		if !streamStates[streamState] {
			return stateErr("mission stream %s goal or state is invalid", streamID)
		}
		for _, field := range []string{"reason", "answeredAsk"} {
			if stream[field] != nil {
				if _, ok := stream[field].(string); !ok {
					return stateErr("mission stream %s %s is invalid", streamID, field)
				}
			}
		}
		if strings.HasPrefix(streamState, "parked-") {
			if r, _ := stream["reason"].(string); r == "" {
				return stateErr("mission stream %s is parked without a reason", streamID)
			}
		}
		if stream["answeredAsk"] != nil {
			if a, _ := stream["answeredAsk"].(string); !idRe.MatchString(a) {
				return stateErr("mission stream %s answered ask id is invalid", streamID)
			}
		}
	}
	if err := validateFences(state["fences"]); err != nil {
		return err
	}
	if err := validateLogsAndLedger(state); err != nil {
		return err
	}
	if err := validateOpenTurn(state["openTurn"]); err != nil {
		return err
	}
	if err := validateWorkspaceTaint(state["workspaceTaint"]); err != nil {
		return err
	}
	return validateIntegrityShape(state["integrity"])
}

func validateLastDrainStall(raw any) error {
	stall, ok := raw.(map[string]any)
	if !ok || !exactKeys(stall, "cycle", "survivors") {
		return stateErr("mission lastDrainStall has an invalid shape")
	}
	if cycle, ok := intValue(stall["cycle"]); !ok || cycle < 1 {
		return stateErr("mission lastDrainStall cycle is invalid")
	}
	survivors, ok := stall["survivors"].([]any)
	if !ok {
		return stateErr("mission lastDrainStall survivors must be an array of job ids")
	}
	for _, item := range survivors {
		if id, ok := item.(string); !ok || !idRe.MatchString(id) {
			return stateErr("mission lastDrainStall survivors must be an array of job ids")
		}
	}
	return nil
}

func validateFences(raw any) error {
	fences, ok := raw.(map[string]any)
	if !ok || !exactKeys(fences, "startedAt", "cycles", "jobs", "activeJobs", "usage") {
		return stateErr("mission fence counters have an invalid shape")
	}
	if s, _ := fences["startedAt"].(string); parseISO(s) != nil {
		return stateErr("mission fence start time is invalid")
	}
	for _, field := range []string{"cycles", "jobs", "activeJobs"} {
		if !isNonnegInt(fences[field]) {
			return stateErr("mission fence counter %s is invalid", field)
		}
	}
	active, _ := intValue(fences["activeJobs"])
	jobs, _ := intValue(fences["jobs"])
	if active > jobs {
		return stateErr("mission active job count exceeds total jobs")
	}
	usage, ok := fences["usage"].([]any)
	if !ok {
		return stateErr("mission usage must be an array")
	}
	seen := map[string]bool{}
	for _, item := range usage {
		entry, ok := item.(map[string]any)
		if !ok || !exactKeys(entry, "provider", "unit", "value") {
			return stateErr("mission usage entry has an invalid shape")
		}
		provider, pOK := nonEmptyString(entry["provider"])
		unit, uOK := nonEmptyString(entry["unit"])
		if !pOK || !uOK {
			return stateErr("mission usage provider/unit is invalid")
		}
		key := provider + "\x00" + unit
		if seen[key] {
			return stateErr("mission usage repeats a provider/unit tuple")
		}
		seen[key] = true
		if _, isBool := entry["value"].(bool); isBool {
			return stateErr("mission usage value is invalid")
		}
		value, ok := floatValue(entry["value"])
		if !ok || !isFinite(value) || value < 0 {
			return stateErr("mission usage value is invalid")
		}
	}
	return nil
}

func validateLogsAndLedger(state map[string]any) error {
	turnLog, ok := state["turnLog"].([]any)
	if !ok {
		return stateErr("mission turn log must be an array of objects")
	}
	for _, item := range turnLog {
		if _, ok := item.(map[string]any); !ok {
			return stateErr("mission turn log must be an array of objects")
		}
	}
	// Acceptance payloads are the consumption index's ONE recovery source
	// (wall design r4: payload-bearing entries, not hash-only history);
	// a malformed or duplicated digest refuses the whole state as corrupt.
	if _, err := ConsumedAuthorizations(state); err != nil {
		return err
	}
	waiting, ok := state["waitingList"].([]any)
	if !ok {
		return stateErr("mission waiting list must contain unique ask ids")
	}
	seen := map[string]bool{}
	for _, item := range waiting {
		id, ok := item.(string)
		if !ok || !idRe.MatchString(id) {
			return stateErr("mission waiting list has an invalid ask id")
		}
		if seen[id] {
			return stateErr("mission waiting list must contain unique ask ids")
		}
		seen[id] = true
	}
	if state["runnerLease"] != nil {
		if _, ok := state["runnerLease"].(string); !ok {
			return stateErr("mission runner lease reference is invalid")
		}
	}
	ledger, ok := state["ledger"].(map[string]any)
	if !ok || !exactKeys(ledger, "path", "cycles") {
		return stateErr("mission ledger reference is invalid")
	}
	if _, ok := nonEmptyString(ledger["path"]); !ok || !isNonnegInt(ledger["cycles"]) {
		return stateErr("mission ledger path or cycle count is invalid")
	}
	return nil
}

func validateIntegrityShape(raw any) error {
	integrity, ok := raw.(map[string]any)
	if !ok || !exactKeys(integrity, "sequence", "previousHash", "hash", "history", "recoveryOf") {
		return stateErr("mission integrity block has an invalid shape")
	}
	if !isNonnegInt(integrity["sequence"]) {
		return stateErr("mission integrity sequence is invalid")
	}
	for _, field := range []string{"previousHash", "recoveryOf"} {
		if integrity[field] != nil {
			if s, ok := integrity[field].(string); !ok || !hashRe.MatchString(s) {
				return stateErr("mission integrity %s is invalid", field)
			}
		}
	}
	if s, ok := integrity["hash"].(string); !ok || !hashRe.MatchString(s) {
		return stateErr("mission integrity hash is invalid")
	}
	if _, ok := integrity["history"].([]any); !ok {
		return stateErr("mission integrity history is invalid")
	}
	return nil
}

func validateAggregation(state map[string]any) error {
	streams, _ := state["streams"].(map[string]any)
	active := false
	for _, raw := range streams {
		if s, _ := raw.(map[string]any); s != nil {
			if st, _ := s["state"].(string); st == "active" {
				active = true
			}
		}
	}
	status, _ := state["status"].(string)
	gatePassed, _ := state["gatePassed"].(bool)
	parked := state["parkReason"] != nil
	switch status {
	case "completed":
		if !gatePassed || parked {
			return stateErr("completed mission state requires a passed gate and no park reason")
		}
	case "running":
		if gatePassed || parked || !active {
			return stateErr("running mission state requires an active stream, no park reason, and an unpassed gate")
		}
	default:
		if gatePassed || !parked {
			return stateErr("parked mission state requires an unpassed gate and a reason")
		}
		if r, _ := state["parkReason"].(string); r == "all-streams-parked" && active {
			return stateErr("all-streams-parked cannot retain an active stream")
		}
	}
	return nil
}

func validateChain(state map[string]any) error {
	integrity, _ := state["integrity"].(map[string]any)
	history, _ := integrity["history"].([]any)
	sequence, _ := intValue(integrity["sequence"])
	if int64(len(history)) != sequence+1 {
		return stateErr("mission state hash chain has a missing or forked sequence")
	}
	var previous any // nil for the genesis entry
	seen := map[string]bool{}
	for expected, raw := range history {
		entry, ok := raw.(map[string]any)
		if !ok || !exactKeys(entry, "sequence", "previousHash", "hash") {
			return stateErr("mission state hash-chain entry has an invalid shape")
		}
		entrySeq, _ := intValue(entry["sequence"])
		if entrySeq != int64(expected) || !sameHashRef(entry["previousHash"], previous) {
			return stateErr("mission state hash chain has a fork")
		}
		valueHash, ok := entry["hash"].(string)
		if !ok || !hashRe.MatchString(valueHash) || seen[valueHash] {
			return stateErr("mission state hash chain repeats or corrupts a hash")
		}
		seen[valueHash] = true
		previous = valueHash
	}
	var expectedPrev any
	if sequence > 0 {
		prevEntry, _ := history[len(history)-2].(map[string]any)
		expectedPrev = prevEntry["hash"]
	}
	if !sameHashRef(integrity["previousHash"], expectedPrev) {
		return stateErr("mission state previous hash disagrees with history")
	}
	lastEntry, _ := history[len(history)-1].(map[string]any)
	lastHash, _ := lastEntry["hash"].(string)
	currentHash, _ := integrity["hash"].(string)
	if currentHash != lastHash || currentHash != stateHash(state) {
		return stateErr("mission state hash does not match its bytes")
	}
	return nil
}

func sameHashRef(a, b any) bool {
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok != bok {
		return false
	}
	return as == bs
}

func validate(state map[string]any) error {
	// The exact legacy refusal comes BEFORE strict shape validation (wall
	// design, named contracts): a version-1 state is a pre-wall mission,
	// and the remedy is re-provisioning, not a shape diagnostic. It is a
	// SENTINEL so the reconcile path passes it through instead of
	// classifying it as corruption (slice-4 critique F-2).
	if v, _ := intValue(state["schemaVersion"]); v == 1 {
		return ErrLegacyState
	}
	if err := validateShape(state); err != nil {
		return err
	}
	if err := validateAggregation(state); err != nil {
		return err
	}
	return validateChain(state)
}

// finalizeNext builds the next state's integrity block, hashes it, appends the
// chain entry, and validates the result.
func finalizeNext(next map[string]any, previous map[string]any, recoveryOf any) (map[string]any, error) {
	value, _ := deepCopyDoc(next).(map[string]any)
	var sequence int64
	var previousHash any
	var history []any
	if previous == nil {
		sequence, previousHash, history = 0, nil, []any{}
	} else {
		prevIntegrity, _ := previous["integrity"].(map[string]any)
		prevSeq, _ := intValue(prevIntegrity["sequence"])
		sequence = prevSeq + 1
		previousHash = prevIntegrity["hash"]
		if h, _ := deepCopyDoc(prevIntegrity["history"]).([]any); h != nil {
			history = h
		} else {
			history = []any{}
		}
	}
	value["integrity"] = map[string]any{
		"sequence":     sequence,
		"previousHash": previousHash,
		"hash":         strings.Repeat("0", 64),
		"history":      history,
		"recoveryOf":   recoveryOf,
	}
	digest := stateHash(value)
	integrity := value["integrity"].(map[string]any)
	integrity["hash"] = digest
	integrity["history"] = append(history, map[string]any{
		"sequence": sequence, "previousHash": previousHash, "hash": digest,
	})
	if err := validate(value); err != nil {
		return nil, err
	}
	return value, nil
}

// BlankString reports whether a string carries NO visible content
// (slice-6 successor rounds 6-7): a rune counts as content only when it
// is GRAPHIC and not whitespace — the whole Unicode format category
// (BOM, zero-width characters, word joiners, function application, and
// every other Cf/control codepoint) is non-graphic and therefore blank
// BY CATEGORY, not by blacklist. A resolver identity or reason made of
// these is no attribution at all.
func BlankString(s string) bool {
	for _, r := range s {
		if unicode.IsGraphic(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func parseISO(s string) error {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if _, err := time.Parse(layout, s); err == nil {
			return nil
		}
	}
	return fmt.Errorf("unparsable timestamp %q", s)
}

func isFinite(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

// authoredContractValues extracts the single authored mission block's
// key=value lines from a contract.
func authoredContractValues(contractPath string) (map[string]string, error) {
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, stateErr("cannot read mission contract: %v", err)
	}
	blocks := contract.AuthoredBlocks(string(data))
	if len(blocks) != 1 {
		return nil, stateErr("mission contract does not have exactly one authored block")
	}
	values, err := contract.ParseAuthoredValues(blocks[0][1], "mission contract")
	if err != nil {
		return nil, stateErr("mission contract key/value grammar is invalid: %v", err)
	}
	return values, nil
}

var contractNameRe = regexp.MustCompile(`^mission-([a-z0-9][a-z0-9-]*)\.contract\.md$`)

// InitState creates a mission's initial state from its sealed contract.
func InitState(statePath, contractPath, ledgerPath, lease, branchArg string) error {
	values, err := authoredContractValues(contractPath)
	if err != nil {
		return err
	}
	match := contractNameRe.FindStringSubmatch(filepath.Base(contractPath))
	if match == nil {
		return stateErr("mission contract filename is invalid")
	}
	streams := map[string]any{}
	for key, value := range values {
		if strings.HasPrefix(key, "stream.") {
			streams[strings.TrimPrefix(key, "stream.")] = map[string]any{
				"goal": value, "state": "active", "reason": nil, "answeredAsk": nil,
			}
		}
	}
	branch := branchArg
	if branch == "" {
		branch = values["candidate.branch"]
	}
	if branch == "" {
		data, _ := os.ReadFile(contractPath)
		if seal := contract.SealBlock(string(data)); seal != nil {
			for _, line := range strings.Split(seal[1], "\n") {
				if strings.HasPrefix(line, "candidate.branch=") {
					branch = strings.SplitN(line, "=", 2)[1]
				}
			}
		}
	}
	if branch == "" {
		return stateErr("sealed candidate branch is absent")
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	var runnerLease any
	if lease != "" {
		runnerLease = lease
	}
	body := map[string]any{
		"schemaVersion":  3,
		"missionId":      match[1],
		"openTurn":       nil,
		"workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}},
		"branch":         branch,
		"status":         "running",
		"parkReason":     nil,
		"gatePassed":     false,
		"streams":        streams,
		"fences":         map[string]any{"startedAt": now, "cycles": 0, "jobs": 0, "activeJobs": 0, "usage": []any{}},
		"turnLog":        []any{},
		"waitingList":    []any{},
		"runnerLease":    runnerLease,
		"ledger":         map[string]any{"path": ledgerPath, "cycles": 0},
		// The ledger semantics under which this mission's stop-loss verdict
		// replays, pinned for the mission's whole life: a sealed budget's
		// meaning never changes mid-mission (docs/design/stop-loss-core.md).
		// Semantics 3 (issue #4): candidate-branch gate measurements
		// extend the stop-loss best tuple; sealed meaning never changes
		// mid-mission, so only NEW missions carry it.
		"ledgerSemantics": 3,
		"integrity":       map[string]any{},
	}
	lock, err := lockFile(statePath)
	if err != nil {
		return err
	}
	defer lock.release()
	if _, err := os.Stat(statePath); err == nil {
		return stateErr("mission state already exists")
	}
	finalized, err := finalizeNext(body, nil, nil)
	if err != nil {
		return err
	}
	return atomicWriteJSON(statePath, finalized)
}

func validateTransition(previous, next map[string]any) error {
	for _, key := range []string{"schemaVersion", "missionId", "branch", "ledgerSemantics"} {
		if !jsonEqual(previous[key], next[key]) {
			return stateErr("mission state update changes immutable identity")
		}
	}
	// Acceptance history is append-only (slice-4 critique F-1): every
	// existing turn-log entry survives byte-identical at its position, so
	// a consumed authorization can never be erased and replayed by a
	// later otherwise-valid write. Current runner paths only append.
	prevLog, _ := previous["turnLog"].([]any)
	nextLog, _ := next["turnLog"].([]any)
	if len(nextLog) < len(prevLog) {
		return stateErr("mission turn log must be append-only")
	}
	for i, entry := range prevLog {
		if !jsonEqual(entry, nextLog[i]) {
			return stateErr("mission turn log entry %d is immutable", i)
		}
	}
	// Every NEW entry carries the wall payload and its consumption list
	// (slice-5 critique F-4), and the payload's sequence point names THIS
	// write (critique F-2): the occurrence identity an authorization will
	// later bind must be the chain position the acceptance actually landed
	// at, not a number any proposal chose freely.
	prevIntegrityDoc, _ := previous["integrity"].(map[string]any)
	prevSequence, _ := intValue(prevIntegrityDoc["sequence"])
	nextTaintDoc, _ := next["workspaceTaint"].(map[string]any)
	nextSegment, _ := intValue(nextTaintDoc["segment"])
	for _, raw := range nextLog[len(prevLog):] {
		entry, _ := raw.(map[string]any)
		if entry == nil || entry["wall"] == nil || entry["consumedAuthorizations"] == nil {
			return stateErr("mission turn log entries must carry the wall payload and its consumption list")
		}
		wall, _ := entry["wall"].(map[string]any)
		point, _ := wall["sequencePoint"].(map[string]any)
		sequence, sOK := intValue(point["sequence"])
		segment, gOK := intValue(point["segment"])
		if !sOK || !gOK || sequence != prevSequence+1 || segment != nextSegment {
			return stateErr("mission acceptance sequence point must name the accepting write (%d/%d)", prevSequence+1, nextSegment)
		}
	}
	// The taint ledger is monotonic at ENTRY grain (slice-4 critique F-3):
	// existing facts are immutable except a null resolution becoming a
	// typed one; a written resolution never changes; appended entries
	// start unresolved; and the segment ordinal advances by exactly the
	// number of resolutions this write performs — a resolution starts a
	// new expected-tree segment, and nothing else does.
	prevTaint, _ := previous["workspaceTaint"].(map[string]any)
	nextTaint, _ := next["workspaceTaint"].(map[string]any)
	if prevTaint != nil && nextTaint != nil {
		prevNext, _ := intValue(prevTaint["next"])
		nowNext, _ := intValue(nextTaint["next"])
		if nowNext < prevNext {
			return stateErr("mission workspaceTaint must be monotonic")
		}
		prevEntries, _ := prevTaint["entries"].([]any)
		nowEntries, _ := nextTaint["entries"].([]any)
		if len(nowEntries) < len(prevEntries) {
			return stateErr("mission workspaceTaint must be monotonic")
		}
		resolved := int64(0)
		for i, rawPrev := range prevEntries {
			prevEntry, _ := rawPrev.(map[string]any)
			nowEntry, _ := nowEntries[i].(map[string]any)
			if prevEntry == nil || nowEntry == nil {
				return stateErr("mission workspaceTaint entry %d is invalid", i)
			}
			for _, field := range []string{"taintId", "turnId", "reason", "setAt"} {
				if !jsonEqual(prevEntry[field], nowEntry[field]) {
					return stateErr("mission workspaceTaint entry %d is immutable", i)
				}
			}
			switch {
			case jsonEqual(prevEntry["resolution"], nowEntry["resolution"]):
			case prevEntry["resolution"] == nil && nowEntry["resolution"] != nil:
				resolved++
				// The resolution's occurrence identity names THIS write,
				// exactly like an acceptance payload's (slice-6 critique
				// F3): the E-point a later authorization resolves against
				// is the chain position the resolution actually landed at.
				resolution, _ := nowEntry["resolution"].(map[string]any)
				point, _ := resolution["sequencePoint"].(map[string]any)
				pointSequence, sOK := intValue(point["sequence"])
				pointSegment, gOK := intValue(point["segment"])
				if !sOK || !gOK || pointSequence != prevSequence+1 || pointSegment != nextSegment {
					return stateErr("mission resolution sequence point must name the resolving write (%d/%d)", prevSequence+1, nextSegment)
				}
			default:
				return stateErr("mission workspaceTaint resolution %d is immutable", i)
			}
		}
		for _, rawNew := range nowEntries[len(prevEntries):] {
			newEntry, _ := rawNew.(map[string]any)
			if newEntry == nil || newEntry["resolution"] != nil {
				return stateErr("mission workspaceTaint entries are appended unresolved")
			}
		}
		// One E-EVENT per state-chain write (slice-6 round-3 finding 5):
		// every acceptance and every resolution names the occurrence
		// {prevSequence+1, segment} of the write that lands it, so a
		// write carrying two of them would put two trees on one E-point
		// and the staleness lookup stops at the first match.
		if int64(len(nextLog)-len(prevLog))+resolved > 1 {
			return stateErr("mission state write carries more than one expected-tree event")
		}
		prevSegment, _ := intValue(prevTaint["segment"])
		nowSegment, _ := intValue(nextTaint["segment"])
		if nowSegment != prevSegment+resolved {
			return stateErr("mission workspaceTaint segment must advance exactly with resolutions")
		}
	}
	// The open-turn marker never mutates in flight: a write may open a
	// turn (null to marker), conclude one (marker to null), or leave the
	// marker untouched — silently replacing the turn in flight would mask
	// a missed conclusion.
	if previous["openTurn"] != nil && next["openTurn"] != nil && !jsonEqual(previous["openTurn"], next["openTurn"]) {
		return stateErr("mission openTurn is immutable while a turn is in flight")
	}
	prevStreams, _ := previous["streams"].(map[string]any)
	nextStreams, _ := next["streams"].(map[string]any)
	if !sameKeySet(prevStreams, nextStreams) {
		return stateErr("mission state update changes the declared stream set")
	}
	for streamID, oldRaw := range prevStreams {
		old, _ := oldRaw.(map[string]any)
		nw, _ := nextStreams[streamID].(map[string]any)
		if !jsonEqual(old["goal"], nw["goal"]) {
			return stateErr("mission stream %s goal is immutable", streamID)
		}
		oldState, _ := old["state"].(string)
		newState, _ := nw["state"].(string)
		if !legalStreamTransitions[oldState][newState] {
			return stateErr("illegal mission stream transition: %s to %s", oldState, newState)
		}
		answeredChanged := !jsonEqual(nw["answeredAsk"], old["answeredAsk"])
		humanAnswer := (oldState == "parked-reserved" || oldState == "parked-stop-loss") && newState == "active" ||
			(oldState != "parked-stop-loss" && newState == "parked-stop-loss")
		if answeredChanged && !humanAnswer {
			return stateErr("mission stream answered ask changes only on a human-answer transition")
		}
		newAnswered, _ := nonEmptyString(nw["answeredAsk"])
		if oldState == "parked-reserved" && newState == "active" && (newAnswered == "" || !answeredChanged) {
			return stateErr("parked-reserved can reactivate only with an answered ask")
		}
		if oldState == "parked-stop-loss" && newState == "active" && (newAnswered == "" || !answeredChanged) {
			return stateErr("parked-stop-loss can reactivate only with a human budget answer")
		}
		if oldState != "parked-stop-loss" && newState == "parked-stop-loss" && (newAnswered == "" || !answeredChanged) {
			return stateErr("parked-stop-loss is reserved for a human answer")
		}
	}
	if s, _ := previous["status"].(string); s == "completed" {
		if ns, _ := next["status"].(string); ns != "completed" {
			return stateErr("completed mission state is terminal")
		}
	}
	prevFences, _ := previous["fences"].(map[string]any)
	nextFences, _ := next["fences"].(map[string]any)
	for _, field := range []string{"cycles", "jobs"} {
		p, _ := intValue(prevFences[field])
		n, _ := intValue(nextFences[field])
		if n < p {
			return stateErr("mission fence counter %s cannot decrease", field)
		}
	}
	prevLedger, _ := previous["ledger"].(map[string]any)
	nextLedger, _ := next["ledger"].(map[string]any)
	pc, _ := intValue(prevLedger["cycles"])
	nc, _ := intValue(nextLedger["cycles"])
	if nc < pc {
		return stateErr("mission ledger cycle count cannot decrease")
	}
	return nil
}

func sameKeySet(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// WriteState advances the state via compare-and-write: the on-disk state must
// still hash to expect, the transition must be legal, and the source's
// integrity block is recomputed rather than trusted.
func WriteState(statePath, sourcePath, expect string) error {
	return writeState(statePath, sourcePath, expect, false)
}

// WriteStateResolution is the resolve-taint writer: the ONLY entry that
// may land a typed resolution or move the taint segment. Everything the
// public writer refuses under runner custody (slice-6 critique F1) is
// legal here, because the caller has already passed the human-reserved
// classification and the verified-tree gates.
func WriteStateResolution(statePath, sourcePath, expect string) error {
	return writeState(statePath, sourcePath, expect, true)
}

func writeState(statePath, sourcePath, expect string, allowResolution bool) error {
	if !hashRe.MatchString(expect) {
		return stateErr("--expect must be a state hash")
	}
	proposed, err := readStateDoc(sourcePath)
	if err != nil {
		return err
	}
	delete(proposed, "integrity")
	lock, err := lockFile(statePath)
	if err != nil {
		return err
	}
	defer lock.release()
	previous, err := readStateDoc(statePath)
	if err != nil {
		return err
	}
	if err := validate(previous); err != nil {
		return err
	}
	prevIntegrity, _ := previous["integrity"].(map[string]any)
	if h, _ := prevIntegrity["hash"].(string); h != expect {
		return stateErr("mission state compare-and-write hash mismatch")
	}
	// Resolution custody (HIW-O6, slice-6 round-2 finding 1): the check
	// runs on the SAME parsed proposal the transition validates — one
	// read, no swap window — and only the resolution writer may pass.
	if !allowResolution {
		if err := refuseResolutionTransition(previous, proposed); err != nil {
			return &ProposalError{Err: err}
		}
	}
	if err := validateTransition(previous, proposed); err != nil {
		return &ProposalError{Err: err}
	}
	finalized, err := finalizeNext(proposed, previous, nil)
	if err != nil {
		return &ProposalError{Err: err}
	}
	return atomicWriteJSON(statePath, finalized)
}

// VerifyStateShape validates a state document and returns its sequence and hash.
func VerifyStateShape(statePath string) (sequence int64, hash string, err error) {
	state, err := readStateDoc(statePath)
	if err != nil {
		return 0, "", err
	}
	if err := validate(state); err != nil {
		return 0, "", err
	}
	integrity, _ := state["integrity"].(map[string]any)
	seq, _ := intValue(integrity["sequence"])
	h, _ := integrity["hash"].(string)
	return seq, h, nil
}

var treeIDRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// ErrLegacyState is the exact named refusal for a pre-wall mission state.
// Reconciliation must pass it through verbatim — it is a remedy message
// for the operator, not corruption.
var ErrLegacyState = stateErr("mission resume refused: state predates the host-implementer wall; re-provision the mission")

// validateOpenTurn checks the runner-owned open-turn marker: null between
// turns, or the anchored identity of the one turn in flight — the pre-tree
// and the sequence point it opened under.
func validateOpenTurn(raw any) error {
	if raw == nil {
		return nil
	}
	turn, ok := raw.(map[string]any)
	if !ok || !exactKeys(turn, "turnId", "cycle", "preTree", "sequence", "segment", "openedAt") {
		return stateErr("mission openTurn has an invalid shape")
	}
	if id, _ := turn["turnId"].(string); !idRe.MatchString(id) {
		return stateErr("mission openTurn turn id is invalid")
	}
	if c, ok := intValue(turn["cycle"]); !ok || c < 1 {
		return stateErr("mission openTurn cycle is invalid")
	}
	if tree, _ := turn["preTree"].(string); !treeIDRe.MatchString(tree) {
		return stateErr("mission openTurn preTree is invalid")
	}
	for _, field := range []string{"sequence", "segment"} {
		if v, ok := intValue(turn[field]); !ok || v < 0 {
			return stateErr("mission openTurn %s is invalid", field)
		}
	}
	if s, _ := turn["openedAt"].(string); parseISO(s) != nil {
		return stateErr("mission openTurn openedAt is invalid")
	}
	return nil
}

// validateWorkspaceTaint checks the monotonic taint ledger: the next taint
// id, the current expected-tree segment ordinal (a resolution starts a new
// segment), and the ordered taint entries with their optional typed
// resolutions.
func validateWorkspaceTaint(raw any) error {
	taint, ok := raw.(map[string]any)
	if !ok || !exactKeys(taint, "next", "segment", "entries") {
		return stateErr("mission workspaceTaint has an invalid shape")
	}
	next, ok := intValue(taint["next"])
	if !ok || next < 1 {
		return stateErr("mission workspaceTaint next id is invalid")
	}
	if v, ok := intValue(taint["segment"]); !ok || v < 0 {
		return stateErr("mission workspaceTaint segment is invalid")
	}
	entries, ok := taint["entries"].([]any)
	if !ok {
		return stateErr("mission workspaceTaint entries must be an array")
	}
	previous := int64(0)
	for _, item := range entries {
		entry, ok := item.(map[string]any)
		if !ok || !exactKeys(entry, "taintId", "turnId", "reason", "setAt", "resolution") {
			return stateErr("mission workspaceTaint entry has an invalid shape")
		}
		id, ok := intValue(entry["taintId"])
		if !ok || id <= previous {
			return stateErr("mission workspaceTaint ids must be strictly increasing")
		}
		previous = id
		if id >= next {
			return stateErr("mission workspaceTaint next id lags an existing entry")
		}
		if turn, _ := entry["turnId"].(string); !idRe.MatchString(turn) {
			return stateErr("mission workspaceTaint entry turn id is invalid")
		}
		if r, _ := entry["reason"].(string); r == "" {
			return stateErr("mission workspaceTaint entry reason is invalid")
		}
		if s, _ := entry["setAt"].(string); parseISO(s) != nil {
			return stateErr("mission workspaceTaint entry setAt is invalid")
		}
		if entry["resolution"] == nil {
			continue
		}
		resolution, ok := entry["resolution"].(map[string]any)
		if !ok {
			return stateErr("mission workspaceTaint resolution has an invalid shape")
		}
		variant, _ := resolution["variant"].(string)
		// Both variants are human-reserved acts and record who resolved
		// and why; adoption additionally records the exact attribution
		// claims being waived (slice-4 critique F-4).
		switch variant {
		case "restore":
			if !exactKeys(resolution, "variant", "treeId", "previousTree", "resolvedAt", "resolvedBy", "reason", "sequencePoint") {
				return stateErr("mission workspaceTaint resolution has an invalid shape")
			}
		case "adopt-disputed-tree":
			if !exactKeys(resolution, "variant", "treeId", "previousTree", "resolvedAt", "resolvedBy", "reason", "sequencePoint", "waivedClaims") {
				return stateErr("mission workspaceTaint resolution has an invalid shape")
			}
			claims, ok := resolution["waivedClaims"].([]any)
			if !ok || len(claims) == 0 {
				return stateErr("mission workspaceTaint waived claims must name at least one claim")
			}
			for _, claim := range claims {
				if s, ok := claim.(string); !ok || BlankString(s) {
					return stateErr("mission workspaceTaint waived claims must be non-blank strings")
				}
			}
		default:
			return stateErr("mission workspaceTaint resolution variant is invalid")
		}
		for _, field := range []string{"resolvedBy", "reason"} {
			if s, _ := resolution[field].(string); BlankString(s) {
				return stateErr("mission workspaceTaint resolution %s is invalid", field)
			}
		}
		if tree, _ := resolution["treeId"].(string); !treeIDRe.MatchString(tree) {
			return stateErr("mission workspaceTaint resolution tree id is invalid")
		}
		// The resolution is a named E-sequence point (slice-6 critique
		// F3): it records the occurrence it landed at and the expected
		// tree it replaced, so the staleness predicate can measure the
		// resolution's own delta instead of fencing whole segments.
		if tree, _ := resolution["previousTree"].(string); !treeIDRe.MatchString(tree) {
			return stateErr("mission workspaceTaint resolution previous tree id is invalid")
		}
		point, ok := resolution["sequencePoint"].(map[string]any)
		if !ok || !exactKeys(point, "sequence", "segment") {
			return stateErr("mission workspaceTaint resolution sequence point is invalid")
		}
		if s, ok := intValue(point["sequence"]); !ok || s < 1 {
			return stateErr("mission workspaceTaint resolution sequence point is invalid")
		}
		if g, ok := intValue(point["segment"]); !ok || g < 1 {
			return stateErr("mission workspaceTaint resolution sequence point is invalid")
		}
		if s, _ := resolution["resolvedAt"].(string); parseISO(s) != nil {
			return stateErr("mission workspaceTaint resolution resolvedAt is invalid")
		}
	}
	return nil
}

// ConsumedAuthorizations derives the consumption index by replaying every
// acceptance entry's payload: authorizationDigest -> consuming turn id. A
// turn-log entry carrying consumedAuthorizations must also carry the wall
// payload; a malformed digest, a duplicate consumption, or a malformed
// wall payload refuses the state as corrupt — the index is never guessed
// and never trusted from any cached copy.
func ConsumedAuthorizations(state map[string]any) (map[string]string, error) {
	turnLog, _ := state["turnLog"].([]any)
	index := map[string]string{}
	for _, item := range turnLog {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		raw, present := entry["consumedAuthorizations"]
		wallRaw, wallPresent := entry["wall"]
		if !present && !wallPresent {
			continue
		}
		turnID, _ := entry["turnId"].(string)
		if !idRe.MatchString(turnID) {
			return nil, stateErr("mission acceptance entry has an invalid turn id")
		}
		if !present || !wallPresent {
			return nil, stateErr("mission acceptance entry %s must carry both wall and consumedAuthorizations", turnID)
		}
		wall, ok := wallRaw.(map[string]any)
		if !ok || !exactKeys(wall, "verdict", "preTree", "expectedTree", "postTree", "orderedDigests", "sequencePoint") {
			return nil, stateErr("mission acceptance entry %s wall payload has an invalid shape", turnID)
		}
		point, ok := wall["sequencePoint"].(map[string]any)
		if !ok || !exactKeys(point, "sequence", "segment") {
			return nil, stateErr("mission acceptance entry %s sequence point has an invalid shape", turnID)
		}
		for _, field := range []string{"sequence", "segment"} {
			if v, ok := intValue(point[field]); !ok || v < 0 {
				return nil, stateErr("mission acceptance entry %s sequence point %s is invalid", turnID, field)
			}
		}
		if v, _ := wall["verdict"].(string); v != "passed" {
			return nil, stateErr("mission acceptance entry %s wall verdict is invalid", turnID)
		}
		for _, field := range []string{"preTree", "expectedTree", "postTree"} {
			if tree, _ := wall[field].(string); !treeIDRe.MatchString(tree) {
				return nil, stateErr("mission acceptance entry %s wall %s is invalid", turnID, field)
			}
		}
		digests, ok := raw.([]any)
		if !ok {
			return nil, stateErr("mission acceptance entry %s consumedAuthorizations must be an array", turnID)
		}
		ordered, ok := wall["orderedDigests"].([]any)
		if !ok || len(ordered) != len(digests) {
			return nil, stateErr("mission acceptance entry %s ordered digests disagree with consumption", turnID)
		}
		for position, rawDigest := range digests {
			digest, ok := rawDigest.(string)
			if !ok || !hashRe.MatchString(digest) {
				return nil, stateErr("mission acceptance entry %s has a malformed authorization digest", turnID)
			}
			if ordered[position] != rawDigest {
				return nil, stateErr("mission acceptance entry %s ordered digests disagree with consumption", turnID)
			}
			if _, taken := index[digest]; taken {
				return nil, stateErr("mission state consumes authorization %.12s twice; the state is corrupt", digest)
			}
			index[digest] = turnID
		}
	}
	return index, nil
}

// refuseResolutionTransition guards every writer except resolve-taint's
// (HIW-O6): a proposal that types a resolution onto any taint entry or
// moves the taint segment is refused — those transitions belong to the
// verified, human-reserved path alone. Runs inside writeState's lock on
// the already-parsed documents, so no source swap between check and
// write is possible (slice-6 round-2 finding 1).
func refuseResolutionTransition(current, proposed map[string]any) error {
	currentTaint, _ := current["workspaceTaint"].(map[string]any)
	proposedTaint, _ := proposed["workspaceTaint"].(map[string]any)
	currentEntries, _ := currentTaint["entries"].([]any)
	proposedEntries, _ := proposedTaint["entries"].([]any)
	for index, raw := range proposedEntries {
		entry, _ := raw.(map[string]any)
		if entry == nil || entry["resolution"] == nil {
			continue
		}
		if index >= len(currentEntries) {
			return stateErr("state-write refused: taint resolutions land only through resolve-taint")
		}
		prior, _ := currentEntries[index].(map[string]any)
		if prior == nil || prior["resolution"] == nil {
			return stateErr("state-write refused: taint resolutions land only through resolve-taint")
		}
	}
	currentSegment, _ := intValue(currentTaint["segment"])
	proposedSegment, _ := intValue(proposedTaint["segment"])
	if currentSegment != proposedSegment {
		return stateErr("state-write refused: taint segments move only through resolve-taint")
	}
	return nil
}

// CurrentSequencePoint names the occurrence identity of the CURRENT
// expected tree (host-implementer wall): the sequence point of the last
// acceptance entry carrying a wall payload, or {0, 0} for a mission whose
// expected tree is still the initial baseline. This — never the raw chain
// sequence — is what an open-turn marker records and an authorization
// binds (slice-5 critique F-2: tree ids repeat; the occurrence decides
// the intervening-change set).
func CurrentSequencePoint(state map[string]any) (sequence, segment int64) {
	// The SEGMENT is always the live taint segment (slice-5 round-3
	// finding 3): a resolution advances it immediately, before any
	// new-segment acceptance exists, so an old-segment authorization can
	// never ride a repeated tree through the k==j comparison.
	taint, _ := state["workspaceTaint"].(map[string]any)
	segment, _ = intValue(taint["segment"])
	turnLog, _ := state["turnLog"].([]any)
	for i := len(turnLog) - 1; i >= 0; i-- {
		entry, _ := turnLog[i].(map[string]any)
		if entry == nil {
			continue
		}
		wall, _ := entry["wall"].(map[string]any)
		point, _ := wall["sequencePoint"].(map[string]any)
		if point == nil {
			continue
		}
		if s, ok := intValue(point["sequence"]); ok {
			sequence = s
			break
		}
	}
	// A resolution is an E-transition too (slice-6 critique F3): when one
	// landed after the last acceptance, ITS occurrence is the current
	// expected tree's identity — an authorization issued at the
	// resolution point must bind a point that stays resolvable.
	entries, _ := taint["entries"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		point, _ := resolution["sequencePoint"].(map[string]any)
		if s, ok := intValue(point["sequence"]); ok && s > sequence {
			sequence = s
		}
	}
	return sequence, segment
}

// ExpectedTreePoints enumerates the mission's named E-sequence points from
// its acceptance entries: each accepted post-tree with the occurrence it
// was accepted at, oldest first. The current expected tree (an open turn's
// pre-tree) is NOT included — its occurrence is CurrentSequencePoint.
func ExpectedTreePoints(state map[string]any) []ExpectedTreePoint {
	turnLog, _ := state["turnLog"].([]any)
	points := []ExpectedTreePoint{}
	// E0 — the initial baseline — is a named point too (slice-5 round-2
	// finding 5): a first-turn authorization binds {0, 0}, and its base
	// must stay resolvable after later turns land. E0 is the PRE-side of
	// the earliest E-event: the first acceptance's preTree, or — when the
	// first turn violated and was resolved before anything was accepted —
	// the first resolution's previousTree (slice-6 round-2 finding 5).
	e0Sequence := int64(-1)
	e0Tree := ""
	for _, raw := range turnLog {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		wall, _ := entry["wall"].(map[string]any)
		point, _ := wall["sequencePoint"].(map[string]any)
		s, sOK := intValue(point["sequence"])
		tree, _ := wall["preTree"].(string)
		if sOK && tree != "" {
			e0Sequence, e0Tree = s, tree
			break
		}
	}
	for _, raw := range taintEntriesOf(state) {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		point, _ := resolution["sequencePoint"].(map[string]any)
		s, sOK := intValue(point["sequence"])
		tree, _ := resolution["previousTree"].(string)
		if sOK && tree != "" && (e0Sequence < 0 || s < e0Sequence) {
			e0Sequence, e0Tree = s, tree
		}
	}
	if e0Tree != "" {
		points = append(points, ExpectedTreePoint{Tree: e0Tree, Sequence: 0, Segment: 0})
	}
	for _, raw := range turnLog {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		wall, _ := entry["wall"].(map[string]any)
		point, _ := wall["sequencePoint"].(map[string]any)
		tree, _ := wall["postTree"].(string)
		if point == nil || tree == "" {
			continue
		}
		s, sOK := intValue(point["sequence"])
		g, gOK := intValue(point["segment"])
		if sOK && gOK {
			points = append(points, ExpectedTreePoint{Tree: tree, Sequence: s, Segment: g})
		}
	}
	// Resolutions are named E-points (slice-6 critique F3): E(next) after
	// a ruling is the restored or adopted tree, and fresh work issued
	// there must stay resolvable after later turns land.
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		resolution, _ := entry["resolution"].(map[string]any)
		if resolution == nil {
			continue
		}
		tree, _ := resolution["treeId"].(string)
		point, _ := resolution["sequencePoint"].(map[string]any)
		s, sOK := intValue(point["sequence"])
		g, gOK := intValue(point["segment"])
		if tree != "" && sOK && gOK {
			points = append(points, ExpectedTreePoint{Tree: tree, Sequence: s, Segment: g})
		}
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].Sequence < points[j].Sequence })
	return points
}

func taintEntriesOf(state map[string]any) []any {
	taint, _ := state["workspaceTaint"].(map[string]any)
	entries, _ := taint["entries"].([]any)
	return entries
}

// CurrentExpectedTree names the tree the workspace's filtered projection
// MUST equal between turns: the highest-occurrence E-point — the last
// accepted post-tree or, after a ruling, the resolution tree. Empty for
// a mission with no E-events yet (the first reservation defines E0).
func CurrentExpectedTree(state map[string]any) string {
	points := ExpectedTreePoints(state)
	if len(points) == 0 {
		return ""
	}
	return points[len(points)-1].Tree
}

// ExpectedTreePoint is one named E-sequence point: a tree and the
// occurrence identity it was accepted under.
type ExpectedTreePoint struct {
	Tree     string
	Sequence int64
	Segment  int64
}

// ProposalError marks a WriteState refusal caused by the PROPOSED state —
// transition rules or shape validation of the proposal itself — as opposed
// to unreadable current state, corruption, or a compare-and-write miss.
// The mission runner uses this boundary to park adjudicated host content
// instead of dying (issue #3): only the proposal's content is host-derived.
type ProposalError struct{ Err error }

func (e *ProposalError) Error() string { return e.Err.Error() }
func (e *ProposalError) Unwrap() error { return e.Err }
