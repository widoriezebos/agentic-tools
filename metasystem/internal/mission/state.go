package mission

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
	"drain-stalled": true,
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
		"gatePassed", "streams", "fences", "turnLog", "waitingList", "runnerLease", "ledger", "integrity"}
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
		// meaning (plans/stop-loss-core.md); absent on legacy missions.
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
	if v, _ := intValue(state["schemaVersion"]); v != 1 {
		return stateErr("mission state schema version or mission id is invalid")
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

var authoredBlockRe = regexp.MustCompile("(?ms)^```mission[ \t]*\n(.*?)^```[ \t]*$")
var sealBlockRe = regexp.MustCompile("(?ms)^```mission-seal[ \t]*\n(.*?)^```[ \t]*$")

// authoredContractValues extracts the single authored mission block's
// key=value lines from a contract.
func authoredContractValues(contractPath string) (map[string]string, error) {
	data, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, stateErr("cannot read mission contract: %v", err)
	}
	blocks := authoredBlockRe.FindAllStringSubmatch(string(data), -1)
	if len(blocks) != 1 {
		return nil, stateErr("mission contract does not have exactly one authored block")
	}
	values := map[string]string{}
	for _, line := range strings.Split(blocks[0][1], "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, stateErr("mission contract key/value grammar is invalid")
		}
		if _, exists := values[key]; exists {
			return nil, stateErr("mission contract key/value grammar is invalid")
		}
		values[key] = value
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
		if seal := sealBlockRe.FindStringSubmatch(string(data)); seal != nil {
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
		"schemaVersion": 1,
		"missionId":     match[1],
		"branch":        branch,
		"status":        "running",
		"parkReason":    nil,
		"gatePassed":    false,
		"streams":       streams,
		"fences":        map[string]any{"startedAt": now, "cycles": 0, "jobs": 0, "activeJobs": 0, "usage": []any{}},
		"turnLog":       []any{},
		"waitingList":   []any{},
		"runnerLease":   runnerLease,
		"ledger":        map[string]any{"path": ledgerPath, "cycles": 0},
		// The ledger semantics under which this mission's stop-loss verdict
		// replays, pinned for the mission's whole life: a sealed budget's
		// meaning never changes mid-mission (plans/stop-loss-core.md).
		"ledgerSemantics": 2,
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
	if err := validateTransition(previous, proposed); err != nil {
		return err
	}
	finalized, err := finalizeNext(proposed, previous, nil)
	if err != nil {
		return err
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
