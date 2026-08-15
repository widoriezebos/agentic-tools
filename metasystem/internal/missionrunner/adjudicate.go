package missionrunner

import (
	"fmt"
	turnvocab "github.com/widoriezebos/agentic-tools/metasystem/internal/turn"
	"os"
	"path/filepath"
	"strings"
)

// Adjudication of a host turn: first the return must prove it belongs to this
// exact turn (ValidateReturn), then each claim it makes is accepted or
// rejected against the mission state (Adjudicate). Every rejection becomes a
// host-failure ask so a human reviews the return before the mission proceeds
// on a claim the runner refused.

// Turn is the runner's record of the host turn a return is judged against.
type Turn struct {
	TurnID      string
	MissionID   string
	Cycle       any // JSON number, preserved as written
	Runtime     string
	Model       string
	HostSession any // legacy name for the announced session, retained until the fixtures migrate
	// AnnouncedSession is what the prompt's Host-Session header said (nil when
	// it said none). It derives from the previous concluded turn and can be
	// stale — a hint, never an authority.
	AnnouncedSession any
	// ObservedSession is the session the harness itself observed for this
	// turn, stamped from its own artifacts. Nil when no source named one; a
	// legacy turn record without the field reads as nil and adjudicates as
	// it did before the field existed.
	ObservedSession any
}

// TurnFromDoc extracts the identity fields from a turn record document.
func TurnFromDoc(doc map[string]any) (Turn, error) {
	turn := Turn{
		HostSession:      doc["hostSession"],
		AnnouncedSession: doc["hostSession"],
		ObservedSession:  doc["observedSession"],
		Cycle:            doc["cycle"],
	}
	if announced, present := doc["announcedSession"]; present {
		turn.AnnouncedSession = announced
	}
	for key, target := range map[string]*string{
		"turnId":    &turn.TurnID,
		"missionId": &turn.MissionID,
		"runtime":   &turn.Runtime,
		"model":     &turn.Model,
	} {
		value, ok := doc[key].(string)
		if !ok || value == "" {
			return Turn{}, fmt.Errorf("turn record %s is missing", key)
		}
		*target = value
	}
	if turn.Cycle == nil {
		return Turn{}, fmt.Errorf("turn record cycle is missing")
	}
	return turn, nil
}

// CycleString renders the turn's cycle number for ask-id prefixes.
func (t Turn) CycleString() string {
	return fmt.Sprintf("%v", t.Cycle)
}

// ReturnValidation is a validated orchestrator return: the parsed return and
// the turn-contained result paths.
type ReturnValidation struct {
	Returned   map[string]any
	RawPath    string
	ReturnPath string
}

// ValidateReturn checks a host result envelope and the orchestrator return it
// points at: the envelope carries exactly the expected fields and a completed
// outcome, both result paths stay inside the turn directory, the return
// passes the role's completeness check (checkReturn, supplied by the caller),
// and the return's identity matches this turn exactly. The session identity
// is accepted when it equals the announced session (what the prompt said) OR
// the observed session (what the harness itself saw): an honest host can
// never lose by echoing the prompt or by telling the truth. A return matching
// neither is a SessionFault, witnessed only when an observed session exists.
func ValidateReturn(turn Turn, result map[string]any, turnDir string, checkReturn func(returnPath string) error) (*ReturnValidation, error) {
	expected := map[string]bool{"sessionId": true, "outcome": true, "usage": true, "rawPath": true, "returnPath": true}
	if len(result) != len(expected) {
		return nil, fmt.Errorf("host result has missing or unexpected fields")
	}
	for key := range result {
		if !expected[key] {
			return nil, fmt.Errorf("host result has missing or unexpected fields")
		}
	}
	if outcome, _ := result["outcome"].(string); outcome != "completed" {
		return nil, fmt.Errorf("host result outcome is not completed: %v", result["outcome"])
	}
	rawPath, err := containedPath(turnDir, result["rawPath"], "rawPath")
	if err != nil {
		return nil, err
	}
	returnPath, err := containedPath(turnDir, result["returnPath"], "returnPath")
	if err != nil {
		return nil, err
	}
	returned, err := validateReturnAt(turn, returnPath, checkReturn)
	if err != nil {
		return nil, err
	}
	return &ReturnValidation{Returned: returned, RawPath: rawPath, ReturnPath: returnPath}, nil
}

// validateReturnAt runs the return-level half of ValidateReturn against an
// explicit file: completeness, turn identity, runtime/model, and the
// session rule. The delivery walk's resume (D64 phase 2) validates its
// re-collected candidate through exactly this path — one validator.
func validateReturnAt(turn Turn, returnPath string, checkReturn func(returnPath string) error) (map[string]any, error) {
	if err := checkReturn(returnPath); err != nil {
		return nil, err
	}
	returned, err := readJSONDoc(returnPath)
	if err != nil {
		return nil, fmt.Errorf("orchestrator return is unreadable: %s: %v", returnPath, err)
	}
	for _, check := range []struct {
		field    string
		expected any
	}{
		{"turnId", turn.TurnID},
		{"missionId", turn.MissionID},
		{"cycle", turn.Cycle},
	} {
		if !numericEqual(returned[check.field], check.expected) {
			return nil, fmt.Errorf("orchestrator return identity mismatch at %s", check.field)
		}
	}
	identity, ok := returned["identity"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("orchestrator return identity is missing")
	}
	if !numericEqual(identity["runtime"], turn.Runtime) || !numericEqual(identity["model"], turn.Model) {
		return nil, fmt.Errorf("orchestrator return runtime/model identity mismatch")
	}
	if err := sessionIdentityFault(identity["sessionId"], turn); err != nil {
		return nil, err
	}
	return returned, nil
}

// SessionFault is the refusal for a return whose identity.sessionId matched
// neither the announced nor the observed session. Witnessed reports whether
// the harness holds its own observation of the session: with a witness this
// is a host protocol violation that feeds the consecutive-failure breaker;
// without one, no witness convicts either side and the breaker is not fed.
// Either way the return is never applied (the one application rule).
type SessionFault struct {
	Witnessed bool
}

func (f *SessionFault) Error() string {
	if f.Witnessed {
		return "orchestrator return session identity matches neither the announced nor the observed session"
	}
	return "orchestrator return session identity matches neither the announced session nor any harness-observed session"
}

// sessionIdentityFault applies the honesty-proof session rule: echoing the
// announced session and reporting the observed session are both correct, so
// a stale announcement can never fail a truthful host.
func sessionIdentityFault(claimed any, turn Turn) error {
	if numericEqual(claimed, turn.AnnouncedSession) {
		return nil
	}
	if turn.ObservedSession != nil && numericEqual(claimed, turn.ObservedSession) {
		return nil
	}
	return &SessionFault{Witnessed: turn.ObservedSession != nil}
}

// containedPath resolves a result path and refuses one that escapes the turn
// directory, so a return cannot point the runner at files it does not own.
func containedPath(turnDir string, raw any, label string) (string, error) {
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", fmt.Errorf("host result %s is missing", label)
	}
	resolved := resolvePath(value)
	base := resolvePath(turnDir)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("host result %s escapes the turn directory", label)
	}
	return resolved, nil
}

// resolvePath makes a path absolute and resolves symlinks as far as the
// filesystem allows, so containment cannot be dodged with a link or a
// dot-dot segment through one.
func resolvePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return resolveExisting(abs)
}

func resolveExisting(abs string) string {
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return abs
	}
	return filepath.Join(resolveExisting(parent), filepath.Base(abs))
}

// Verdict is an adjudication's outcome: what was accepted and rejected, the
// stream map after accepted updates, the asks the runner must write, and the
// waiting list once it has written them. Accepted and rejected items carry
// the original return entries so the turn log records exactly what was
// claimed.
type Verdict struct {
	RawPath     string           `json:"rawPath"`
	ReturnPath  string           `json:"returnPath"`
	Accepted    []map[string]any `json:"accepted"`
	Rejected    []map[string]any `json:"rejected"`
	Streams     map[string]any   `json:"streams"`
	WaitingList []string         `json:"waitingList"`
	Asks        []map[string]any `json:"asks"`
}

// Adjudicate judges each claim in a validated orchestrator return against the
// mission state and the job records on disk:
//
//   - a dispatched job is accepted only when its record exists, is stamped
//     for this mission, and was created during this host turn;
//   - a stream update is accepted only when the stream exists, the transition
//     is legal for an orchestrator, and a parking request carries a reason;
//   - an ask candidate is accepted only when its stream exists and its reason
//     class is known.
//
// Accepted stream updates are applied to the returned stream map. Every
// accepted ask candidate and every rejection yields an ask record for the
// runner to write; rejections land on the entry's stream when it exists,
// otherwise on the first active stream (or the first stream at all) so the
// question is never lost. The caller must write exactly the returned asks for
// the waiting list to be truthful.
func Adjudicate(root, mission string, turn Turn, state map[string]any, returned map[string]any, nowISO string) (*Verdict, error) {
	streams, ok := state["streams"].(map[string]any)
	if !ok || len(streams) == 0 {
		return nil, fmt.Errorf("mission state has no streams")
	}
	dispatched, err := entryList(returned, "dispatched")
	if err != nil {
		return nil, err
	}
	streamUpdates, err := entryList(returned, "streamUpdatesRequested")
	if err != nil {
		return nil, err
	}
	askCandidates, err := entryList(returned, "askCandidates")
	if err != nil {
		return nil, err
	}

	verdict := &Verdict{Accepted: []map[string]any{}, Rejected: []map[string]any{}, Asks: []map[string]any{}}
	asksDir := asksDirPath(root, mission)
	allocated := map[string]bool{}
	newAskIDs := []string{}

	addAsk := func(prefix, streamID, reasonClass, question string) string {
		askID := nextAskID(asksDir, prefix, allocated)
		verdict.Asks = append(verdict.Asks, askRecord(askID, streamID, reasonClass, question, nowISO))
		newAskIDs = append(newAskIDs, askID)
		return askID
	}

	for _, entry := range dispatched {
		jobID, _ := entry["jobId"].(string)
		reason := ""
		record, err := readJSONDoc(filepath.Join(jobsDirPath(root), jobID+".json"))
		switch {
		case jobID == "" || err != nil:
			reason = "job record does not exist or is unreadable"
		case !numericEqual(record["mission"], mission):
			reason = "job record is not stamped for this mission"
		case !numericEqual(record["turnId"], turn.TurnID):
			reason = "job record was not created during this host turn"
		}
		if reason == "" {
			verdict.Accepted = append(verdict.Accepted, map[string]any{"kind": "dispatched", "value": entry})
		} else {
			verdict.Rejected = append(verdict.Rejected, map[string]any{"kind": "dispatched", "value": entry, "reason": reason})
		}
	}

	for _, entry := range streamUpdates {
		streamID, _ := entry["streamId"].(string)
		requested, _ := entry["requestedState"].(string)
		stream, _ := streams[streamID].(map[string]any)
		currentState, _ := stream["state"].(string)
		entryReason, _ := entry["reason"].(string)
		reason := ""
		switch {
		case stream == nil:
			reason = "stream does not exist"
		case !legalStreamTransitions[currentState][requested]:
			reason = fmt.Sprintf("illegal stream transition %v to %v", stream["state"], entry["requestedState"])
		case strings.HasPrefix(requested, "parked-") && entryReason == "":
			reason = "parked stream request has no reason"
		}
		if reason == "" {
			stream["state"] = requested
			if entryReason == "" {
				stream["reason"] = nil
			} else {
				stream["reason"] = entryReason
			}
			verdict.Accepted = append(verdict.Accepted, map[string]any{"kind": "streamUpdate", "value": entry})
		} else {
			verdict.Rejected = append(verdict.Rejected, map[string]any{"kind": "streamUpdate", "value": entry, "reason": reason})
		}
	}

	for index, entry := range askCandidates {
		streamID, _ := entry["streamId"].(string)
		reasonClass, _ := entry["reasonClass"].(string)
		question, _ := entry["question"].(string)
		reason := ""
		if _, exists := streams[streamID]; !exists {
			reason = "stream does not exist"
		} else if !turnvocab.OrchestratorMayRaise(reasonClass) {
			reason = "reason class is unknown"
		}
		if reason == "" {
			askID := addAsk(fmt.Sprintf("ask-%s-%d", turn.CycleString(), index+1), streamID, reasonClass, question)
			verdict.Accepted = append(verdict.Accepted, map[string]any{"kind": "askCandidate", "value": entry, "askId": askID})
		} else {
			verdict.Rejected = append(verdict.Rejected, map[string]any{"kind": "askCandidate", "value": entry, "reason": reason})
		}
	}

	// The fallback lands after stream updates were applied, so a stream this
	// very return parked no longer attracts the rejection asks.
	fallback := fallbackStream(streams)
	for index, item := range verdict.Rejected {
		value, _ := item["value"].(map[string]any)
		streamID, _ := value["stream"].(string)
		if streamID == "" {
			streamID, _ = value["streamId"].(string)
		}
		if _, exists := streams[streamID]; !exists {
			streamID = fallback
		}
		question := fmt.Sprintf("Runner rejected host return %v: %v. Review the return before proceeding.", item["kind"], item["reason"])
		item["askId"] = addAsk(fmt.Sprintf("rejected-%s-%d", turn.CycleString(), index+1), streamID, "host-failure", question)
	}

	verdict.Streams = streams
	verdict.WaitingList = mergedOpenAskIDs(asksDir, newAskIDs)
	return verdict, nil
}

// entryList reads a return's claim list, refusing a return whose list is not
// a list of objects (the completeness check makes this unreachable in a
// well-formed return).
func entryList(returned map[string]any, key string) ([]map[string]any, error) {
	raw, ok := returned[key].([]any)
	if !ok {
		return nil, fmt.Errorf("orchestrator return %s is not a list", key)
	}
	entries := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("orchestrator return %s entry is not an object", key)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// fallbackStream picks the stream a rejection ask lands on when the rejected
// entry names no live stream: the first active stream in id order, else the
// first stream at all.
func fallbackStream(streams map[string]any) string {
	ids := sortedKeys(streams)
	for _, id := range ids {
		if stream, ok := streams[id].(map[string]any); ok {
			if state, _ := stream["state"].(string); state == "active" {
				return id
			}
		}
	}
	return ids[0]
}

// askRecord shapes one ask exactly as the runner writes it. Newlines in the
// question flatten to spaces so the record stays one-line greppable.
func askRecord(askID, streamID, reasonClass, question, nowISO string) map[string]any {
	flat := strings.ReplaceAll(strings.ReplaceAll(question, "\r", " "), "\n", " ")
	return map[string]any{
		"askId":       askID,
		"streamId":    streamID,
		"reasonClass": reasonClass,
		"question":    flat,
		"createdAt":   nowISO,
		"answeredAt":  nil,
		"answer":      nil,
	}
}

// nextAskID allocates the first free ask id for a prefix, counting both the
// ask files already on disk and ids allocated earlier in this same pass.
func nextAskID(asksDir, prefix string, allocated map[string]bool) string {
	candidate := prefix
	for index := 1; ; index++ {
		if index > 1 {
			candidate = fmt.Sprintf("%s-%d", prefix, index)
		}
		if allocated[candidate] {
			continue
		}
		if _, err := os.Stat(filepath.Join(asksDir, candidate+".json")); err != nil {
			allocated[candidate] = true
			return candidate
		}
	}
}

// openAskIDs lists the unanswered asks on disk, sorted and deduplicated.
// Unreadable ask files are skipped: the waiting list reports what can be
// answered, and refusing to park over one corrupt ask would be worse.
func openAskIDs(asksDir string) []string {
	paths, _ := filepath.Glob(filepath.Join(asksDir, "*.json"))
	seen := map[string]bool{}
	for _, path := range paths {
		doc, err := readJSONDoc(path)
		if err != nil {
			continue
		}
		askID, ok := doc["askId"].(string)
		if ok && doc["answeredAt"] == nil {
			seen[askID] = true
		}
	}
	return sortedKeys(seen)
}

// mergedOpenAskIDs is the waiting list once the caller writes the proposed
// asks: the open asks on disk plus the new ones, sorted and deduplicated.
func mergedOpenAskIDs(asksDir string, newIDs []string) []string {
	seen := map[string]bool{}
	for _, id := range openAskIDs(asksDir) {
		seen[id] = true
	}
	for _, id := range newIDs {
		seen[id] = true
	}
	return sortedKeys(seen)
}
