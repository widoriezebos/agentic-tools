package goal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ChannelPrefix     = "plans/channel/"
	channelTimeLayout = "2006-01-02T15:04:05Z"
)

type ChannelRef struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

type ChannelOption struct {
	Label       string `json:"label"`
	Consequence string `json:"consequence"`
}

type ChannelPosting struct {
	Kind string `json:"kind"`
	By   string `json:"by"`
	At   string `json:"at"`
}

type ChannelRejection struct {
	Ref     ChannelRef  `json:"ref"`
	Reason  string      `json:"reason"`
	At      string      `json:"at"`
	PostRef *ChannelRef `json:"postRef"`
	By      string      `json:"by"`
}

type ChannelAnswer struct {
	Text         string      `json:"text"`
	UserID       string      `json:"userId"`
	Ref          ChannelRef  `json:"ref"`
	At           string      `json:"at"`
	Step         *int64      `json:"step"`
	InboxID      string      `json:"inboxId"`
	Opid         string      `json:"opid"`
	Phase        string      `json:"phase"`
	ApprovalULID *string     `json:"approvalUlid"`
	Receipt      *string     `json:"receipt"`
	ReceiptRef   *ChannelRef `json:"receiptRef"`
}

type ChannelQuestion struct {
	ID             string             `json:"id"`
	Goal           string             `json:"goal"`
	Kind           string             `json:"kind"`
	Machine        string             `json:"machine"`
	Lineage        string             `json:"lineage"`
	Opid           string             `json:"opid"`
	OpenedAt       string             `json:"openedAt"`
	Facts          []string           `json:"facts"`
	Options        []ChannelOption    `json:"options"`
	Recommendation string             `json:"recommendation"`
	Wants          string             `json:"wants"`
	Budget         *Budget            `json:"budget,omitempty"`
	Destination    string             `json:"destination"`
	Thread         *ChannelRef        `json:"thread"`
	OrphanPosts    []ChannelRef       `json:"orphanPosts"`
	Posting        *ChannelPosting    `json:"posting"`
	State          string             `json:"state"`
	Answer         *ChannelAnswer     `json:"answer"`
	Rejected       []ChannelRejection `json:"rejected"`
	FactsDigest    string             `json:"factsDigest"`
	ClosedAt       string             `json:"closedAt,omitempty"`
	ClosedBy       string             `json:"closedBy,omitempty"`
	ClosedBecause  string             `json:"closedBecause,omitempty"`
}

type ChannelInbound struct {
	Provider    string  `json:"provider"`
	Destination string  `json:"destination"`
	MessageID   string  `json:"messageId"`
	UpdateID    string  `json:"updateId"`
	ReplyTo     *string `json:"replyTo"`
	UserID      string  `json:"userId"`
	SentAt      string  `json:"sentAt"`
	Text        string  `json:"text"`
	Step        *int64  `json:"step"`
	Outcome     string  `json:"outcome"`
	Question    string  `json:"question"`
	Opid        string  `json:"opid"`
	ReceivedBy  string  `json:"receivedBy"`
	ReceivedAt  string  `json:"receivedAt"`
}

type ChannelListener struct {
	Machine           string  `json:"machine"`
	Engine            string  `json:"engine"`
	LastReceiveAt     string  `json:"lastReceiveAt"`
	LastConfirmAt     *string `json:"lastConfirmAt"`
	ConflictsLastHour int64   `json:"conflictsLastHour"`
	UpdatedAt         string  `json:"updatedAt"`
	Opid              string  `json:"opid"`
}

// MarshalChannel is the canonical serialization used by channel writers.
func MarshalChannel(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

type channelPath struct {
	kind        string
	id          string
	basename    string
	destination string
}

type channelQuestionAtPath struct {
	path string
	raw  map[string]json.RawMessage
	q    *ChannelQuestion
}

type channelInboundAtPath struct {
	path string
	in   *ChannelInbound
}

// ValidateChannelTree reads the committed channel ledger and applies every
// at-rest refusal. A commit without the directory has no channel problems.
func ValidateChannelTree(root, commit string) []Problem {
	out, err := gitIn(root, "ls-tree", "-r", "--name-only", commit, "--", ChannelPrefix)
	if err != nil {
		return []Problem{channelProblem("channel-json", ChannelPrefix, fmt.Sprintf("cannot list the committed channel tree: %v", err))}
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil
	}

	files, err := readCommitGoalBlobs(root, commit, paths)
	if err != nil {
		return []Problem{channelProblem("channel-json", ChannelPrefix, fmt.Sprintf("cannot read the committed channel tree: %v", err))}
	}
	goalFiles, err := ReadCommitGoals(root, commit)
	if err != nil {
		return []Problem{channelProblem("channel-json", ChannelPrefix, fmt.Sprintf("cannot read the committed goal tree: %v", err))}
	}
	goalIDs := channelGoalIDs(goalFiles)

	var problems []Problem
	questions := make(map[string]channelQuestionAtPath)
	var inbounds []channelInboundAtPath
	for _, filePath := range sortedKeys(files) {
		location, ok := classifyChannelPath(filePath)
		if !ok {
			problems = append(problems, channelProblem("channel-unknown-path", filePath, "the path is not a question, inbox record, or listener record"))
			continue
		}
		switch location.kind {
		case "question":
			q, raw, decodeErr := decodeChannelQuestion(files[filePath])
			if decodeErr != nil {
				problems = append(problems, channelProblem("channel-json", filePath, decodeErr.Error()))
				continue
			}
			questions[location.id] = channelQuestionAtPath{path: filePath, raw: raw, q: q}
			problems = append(problems, validateChannelQuestion(filePath, location.id, q, raw, goalIDs)...)
		case "inbound":
			in, decodeErr := decodeChannelInbound(files[filePath])
			if decodeErr != nil {
				problems = append(problems, channelProblem("channel-json", filePath, decodeErr.Error()))
				continue
			}
			inbounds = append(inbounds, channelInboundAtPath{path: filePath, in: in})
			problems = append(problems, validateChannelInbound(filePath, location, in)...)
		case "listener":
			listener, decodeErr := decodeChannelListener(files[filePath])
			if decodeErr != nil {
				problems = append(problems, channelProblem("channel-json", filePath, decodeErr.Error()))
				continue
			}
			problems = append(problems, validateChannelListener(filePath, location.id, listener)...)
		}
	}

	for _, record := range inbounds {
		in := record.in
		if in.Outcome != "verified" || in.Question == "unbound" || in.Question == "unmatched" {
			continue
		}
		question, exists := questions[in.Question]
		if exists && question.q.Lineage == "migrated" {
			continue
		}
		if !exists || question.q.Answer == nil || question.q.Answer.InboxID != channelInboxID(in) {
			problems = append(problems, channelProblem("channel-answer-state", record.path,
				fmt.Sprintf("verified inbox record %s is not the recorded answer of question %s", channelInboxID(in), in.Question)))
		}
	}
	return problems
}

func channelProblem(code, filePath, detail string) Problem {
	return Problem(fmt.Sprintf("%s: %s: %s", code, filePath, detail))
}

func channelGoalIDs(files map[string][]byte) map[string]bool {
	ids := make(map[string]bool)
	for filePath := range files {
		switch {
		case strings.HasPrefix(filePath, goalsPrefix):
			rel := strings.TrimPrefix(filePath, goalsPrefix)
			if strings.HasSuffix(rel, ".md") && rel != "backlog.md" {
				rel = strings.TrimPrefix(rel, "done/")
				if !strings.Contains(rel, "/") {
					ids[strings.TrimSuffix(rel, ".md")] = true
				}
			}
		case strings.HasPrefix(filePath, recordsGoalsPrefix):
			rel := strings.TrimPrefix(filePath, recordsGoalsPrefix)
			if strings.HasSuffix(rel, ".md") && !strings.Contains(rel, "/") {
				ids[strings.TrimSuffix(rel, ".md")] = true
			}
		}
	}
	return ids
}

func classifyChannelPath(filePath string) (channelPath, bool) {
	if !strings.HasPrefix(filePath, ChannelPrefix) {
		return channelPath{}, false
	}
	rel := strings.TrimPrefix(filePath, ChannelPrefix)
	parts := strings.Split(rel, "/")
	switch {
	case len(parts) == 2 && parts[0] == "questions" && strings.HasSuffix(parts[1], ".json"):
		id := strings.TrimSuffix(parts[1], ".json")
		if validChannelULID(id) {
			return channelPath{kind: "question", id: id}, true
		}
	case len(parts) == 3 && parts[0] == "inbox" && parts[1] != "" && strings.HasSuffix(parts[2], ".json"):
		basename := strings.TrimSuffix(parts[2], ".json")
		dash := strings.IndexByte(basename, '-')
		if dash > 0 && dash < len(basename)-1 {
			return channelPath{kind: "inbound", basename: basename, destination: parts[1]}, true
		}
	case len(parts) == 2 && parts[0] == "listeners" && strings.HasSuffix(parts[1], ".json"):
		id := strings.TrimSuffix(parts[1], ".json")
		if id != "" {
			return channelPath{kind: "listener", id: id}, true
		}
	}
	return channelPath{}, false
}

func validChannelULID(id string) bool {
	if len(id) != 26 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return false
		}
	}
	return true
}

func decodeChannelQuestion(data []byte) (*ChannelQuestion, map[string]json.RawMessage, error) {
	var q ChannelQuestion
	raw, err := decodeChannelObject(data, &q,
		[]string{"id", "goal", "kind", "machine", "lineage", "opid", "openedAt", "facts", "options", "recommendation", "wants", "destination", "thread", "orphanPosts", "posting", "state", "answer", "rejected", "factsDigest"},
		map[string]bool{"thread": true, "posting": true, "answer": true})
	if err != nil {
		return nil, nil, err
	}
	if q.Facts == nil || q.Options == nil || q.OrphanPosts == nil || q.Rejected == nil {
		return nil, nil, fmt.Errorf("facts, options, orphanPosts, and rejected must be arrays, using [] when empty")
	}
	if err := validateChannelOptionsRaw(raw["options"]); err != nil {
		return nil, nil, err
	}
	if err := validateChannelRefsRaw(raw["orphanPosts"]); err != nil {
		return nil, nil, err
	}
	if !rawNull(raw["thread"]) {
		if err := validateChannelRefRaw(raw["thread"]); err != nil {
			return nil, nil, fmt.Errorf("thread: %w", err)
		}
	}
	if !rawNull(raw["posting"]) {
		if _, err := validateRawObject(raw["posting"], []string{"kind", "by", "at"}, nil); err != nil {
			return nil, nil, fmt.Errorf("posting: %w", err)
		}
	}
	if !rawNull(raw["answer"]) {
		if err := validateChannelAnswerRaw(raw["answer"]); err != nil {
			return nil, nil, fmt.Errorf("answer: %w", err)
		}
	}
	if err := validateChannelRejectionsRaw(raw["rejected"]); err != nil {
		return nil, nil, err
	}
	return &q, raw, nil
}

func decodeChannelInbound(data []byte) (*ChannelInbound, error) {
	var in ChannelInbound
	_, err := decodeChannelObject(data, &in,
		[]string{"provider", "destination", "messageId", "updateId", "replyTo", "userId", "sentAt", "text", "step", "outcome", "question", "opid", "receivedBy", "receivedAt"},
		map[string]bool{"replyTo": true, "step": true})
	if err != nil {
		return nil, err
	}
	return &in, nil
}

func decodeChannelListener(data []byte) (*ChannelListener, error) {
	var listener ChannelListener
	_, err := decodeChannelObject(data, &listener,
		[]string{"machine", "engine", "lastReceiveAt", "lastConfirmAt", "conflictsLastHour", "updatedAt", "opid"},
		map[string]bool{"lastConfirmAt": true})
	if err != nil {
		return nil, err
	}
	return &listener, nil
}

func decodeChannelObject(data []byte, dst any, required []string, nullable map[string]bool) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := decodeChannelJSON(data, &raw, false); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("record must be a JSON object")
	}
	if err := decodeChannelJSON(data, dst, true); err != nil {
		return nil, err
	}
	for _, key := range required {
		value, ok := raw[key]
		if !ok {
			return nil, fmt.Errorf("required key %q is missing", key)
		}
		if rawNull(value) && !nullable[key] {
			return nil, fmt.Errorf("key %q cannot be null", key)
		}
	}
	for key, value := range raw {
		if rawNull(value) && !nullable[key] {
			return nil, fmt.Errorf("key %q cannot be null", key)
		}
	}
	return raw, nil
}

func decodeChannelJSON(data []byte, dst any, disallowUnknown bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("record contains more than one JSON value")
		}
		return err
	}
	return nil
}

func validateRawObject(data json.RawMessage, required []string, nullable map[string]bool) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("value must be a JSON object")
	}
	for _, key := range required {
		value, ok := raw[key]
		if !ok {
			return nil, fmt.Errorf("required key %q is missing", key)
		}
		if rawNull(value) && !nullable[key] {
			return nil, fmt.Errorf("key %q cannot be null", key)
		}
	}
	return raw, nil
}

func validateChannelRefRaw(data json.RawMessage) error {
	_, err := validateRawObject(data, []string{"provider", "id", "threadId"}, nil)
	return err
}

func validateChannelOptionsRaw(data json.RawMessage) error {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for i, entry := range entries {
		if _, err := validateRawObject(entry, []string{"label", "consequence"}, nil); err != nil {
			return fmt.Errorf("options[%d]: %w", i, err)
		}
	}
	return nil
}

func validateChannelRefsRaw(data json.RawMessage) error {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for i, entry := range entries {
		if err := validateChannelRefRaw(entry); err != nil {
			return fmt.Errorf("orphanPosts[%d]: %w", i, err)
		}
	}
	return nil
}

func validateChannelAnswerRaw(data json.RawMessage) error {
	raw, err := validateRawObject(data,
		[]string{"text", "userId", "ref", "at", "step", "inboxId", "opid", "phase", "approvalUlid", "receipt", "receiptRef"},
		map[string]bool{"approvalUlid": true, "receipt": true, "receiptRef": true})
	if err != nil {
		return err
	}
	if err := validateChannelRefRaw(raw["ref"]); err != nil {
		return fmt.Errorf("ref: %w", err)
	}
	if !rawNull(raw["receiptRef"]) {
		if err := validateChannelRefRaw(raw["receiptRef"]); err != nil {
			return fmt.Errorf("receiptRef: %w", err)
		}
	}
	return nil
}

func validateChannelRejectionsRaw(data json.RawMessage) error {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for i, entry := range entries {
		raw, err := validateRawObject(entry, []string{"ref", "reason", "at", "postRef", "by"}, map[string]bool{"postRef": true})
		if err != nil {
			return fmt.Errorf("rejected[%d]: %w", i, err)
		}
		if err := validateChannelRefRaw(raw["ref"]); err != nil {
			return fmt.Errorf("rejected[%d].ref: %w", i, err)
		}
		if !rawNull(raw["postRef"]) {
			if err := validateChannelRefRaw(raw["postRef"]); err != nil {
				return fmt.Errorf("rejected[%d].postRef: %w", i, err)
			}
		}
	}
	return nil
}

func rawNull(data json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func validateChannelQuestion(filePath, id string, q *ChannelQuestion, raw map[string]json.RawMessage, goalIDs map[string]bool) []Problem {
	var problems []Problem
	add := func(code, detail string) { problems = append(problems, channelProblem(code, filePath, detail)) }
	if q.ID != id {
		add("channel-id-mismatch", fmt.Sprintf("question id %q does not match basename %q", q.ID, id))
	}
	if !goalIDs[q.Goal] {
		add("channel-goal-missing", fmt.Sprintf("goal %q has no goal file on this commit", q.Goal))
	}
	if !channelOneOf(q.Kind, "budget-above-norm", "fork", "reserved-decision", "stop", "other") {
		add("channel-kind", fmt.Sprintf("question kind %q is not recognized", q.Kind))
	}
	if !channelOneOf(q.State, "open", "answered", "closed") {
		add("channel-kind", fmt.Sprintf("question state %q is not recognized", q.State))
	}
	if q.Posting != nil && !channelOneOf(q.Posting.Kind, "question", "rejection", "list", "receipt", "silence", "approval") {
		add("channel-kind", fmt.Sprintf("posting kind %q is not recognized", q.Posting.Kind))
	}
	if q.Answer != nil && !channelOneOf(q.Answer.Phase, "recorded", "approved", "receipted") {
		add("channel-kind", fmt.Sprintf("answer phase %q is not recognized", q.Answer.Phase))
	}
	if (q.Kind == "stop" || q.Kind == "budget-above-norm") && q.Wants == "" {
		add("channel-token-missing", fmt.Sprintf("question kind %s requires a wants token", q.Kind))
	}
	if q.Lineage != "migrated" {
		budgetRaw, budgetPresent := raw["budget"]
		switch {
		case q.Kind == "budget-above-norm" && q.Budget == nil:
			add("channel-budget", "budget-above-norm question has no proposed budget")
		case q.Kind == "budget-above-norm":
			complete := budgetPresent && channelBudgetComplete(budgetRaw)
			if err := q.Budget.Validate(); err != nil || !complete {
				if err != nil {
					add("channel-budget", fmt.Sprintf("proposed budget is incomplete or invalid: %v", err))
				} else {
					add("channel-budget", "proposed budget is incomplete")
				}
			}
		case q.Budget != nil:
			add("channel-budget", fmt.Sprintf("question kind %s cannot carry a proposed budget", q.Kind))
		}
	}

	closedAtPresent := raw["closedAt"] != nil
	closedByPresent := raw["closedBy"] != nil
	closedBecausePresent := raw["closedBecause"] != nil
	switch {
	case q.State == "answered" && q.Answer == nil:
		add("channel-answer-state", "answered question has no answer")
	case q.State == "closed" && q.ClosedBecause == "answered" && q.Answer == nil:
		add("channel-answer-state", "question closed as answered has no answer")
	case q.State == "closed" && q.ClosedBecause != "answered" && q.Answer != nil:
		add("channel-answer-state", "question closed for another reason still has an answer")
	case q.State == "open" && q.Answer != nil:
		add("channel-answer-state", "open question already has an answer")
	}
	if q.Answer != nil {
		switch {
		case q.Answer.Phase == "recorded" && q.Answer.Receipt != nil:
			add("channel-answer-state", "recorded answer already has a receipt")
		case q.Answer.Phase == "approved" && q.Answer.Receipt == nil:
			add("channel-answer-state", "approved answer has no receipt")
		case q.Answer.Phase == "receipted" && q.Answer.ReceiptRef == nil:
			add("channel-answer-state", "receipted answer has no receipt reference")
		}
		if q.Answer.ApprovalULID != nil && !(q.Kind == "budget-above-norm" && q.Answer.Text == q.Wants && q.Budget != nil) {
			add("channel-answer-state", "approvalUlid is set without a matching budget approval tuple")
		}
	}
	if q.State == "closed" && (!closedAtPresent || !closedByPresent || !closedBecausePresent) {
		add("channel-answer-state", "closed question is missing closedAt, closedBy, or closedBecause")
	}
	postedRejections := 0
	for _, rejection := range q.Rejected {
		if rejection.PostRef != nil {
			postedRejections++
		}
	}
	if postedRejections > 3 {
		add("channel-rejection-cap", fmt.Sprintf("question has %d posted rejections; the maximum is three", postedRejections))
	}
	if channelLastFieldIsCode(qAnswerText(q)) ||
		channelAnyFieldIsCode(q.Facts...) || channelAnyFieldIsCode(q.Recommendation) ||
		channelAnyFieldIsCode(channelRejectionReasons(q.Rejected)...) || channelAnyFieldIsCode(qAnswerReceipt(q)) {
		add("channel-secret", "a six-digit code remains in a durable text field")
	}
	if err := channelTime(q.OpenedAt); err != nil {
		add("channel-json", fmt.Sprintf("openedAt: %v", err))
	}
	if q.Posting != nil {
		if err := channelTime(q.Posting.At); err != nil {
			add("channel-json", fmt.Sprintf("posting.at: %v", err))
		}
	}
	if q.Answer != nil {
		if err := channelTime(q.Answer.At); err != nil {
			add("channel-json", fmt.Sprintf("answer.at: %v", err))
		}
	}
	for i, rejection := range q.Rejected {
		if err := channelTime(rejection.At); err != nil {
			add("channel-json", fmt.Sprintf("rejected[%d].at: %v", i, err))
		}
	}
	if closedAtPresent {
		if err := channelTime(q.ClosedAt); err != nil {
			add("channel-json", fmt.Sprintf("closedAt: %v", err))
		}
	}
	return problems
}

func validateChannelInbound(filePath string, location channelPath, in *ChannelInbound) []Problem {
	var problems []Problem
	add := func(code, detail string) { problems = append(problems, channelProblem(code, filePath, detail)) }
	if location.basename != channelInboxID(in) {
		add("channel-id-mismatch", fmt.Sprintf("inbox basename %q does not match %q", location.basename, channelInboxID(in)))
	}
	if !channelOneOf(in.Outcome, "verified", "late", "wrong-user", "no-code", "bad-code", "stale", "replayed", "unverified-migrated", "skipped") {
		add("channel-kind", fmt.Sprintf("inbox outcome %q is not recognized", in.Outcome))
	}
	stepRequired := channelOneOf(in.Outcome, "verified", "late", "replayed")
	if stepRequired != (in.Step != nil) {
		add("channel-json", fmt.Sprintf("step nullability does not match outcome %q", in.Outcome))
	}
	if channelLastFieldIsCode(in.Text) {
		add("channel-secret", "a six-digit code remains as the last field of inbox text")
	}
	if err := channelTime(in.SentAt); err != nil {
		add("channel-json", fmt.Sprintf("sentAt: %v", err))
	}
	if err := channelTime(in.ReceivedAt); err != nil {
		add("channel-json", fmt.Sprintf("receivedAt: %v", err))
	}
	return problems
}

func validateChannelListener(filePath, machine string, listener *ChannelListener) []Problem {
	var problems []Problem
	add := func(code, detail string) { problems = append(problems, channelProblem(code, filePath, detail)) }
	if listener.Machine != machine {
		add("channel-id-mismatch", fmt.Sprintf("listener machine %q does not match basename %q", listener.Machine, machine))
	}
	if err := channelTime(listener.LastReceiveAt); err != nil {
		add("channel-json", fmt.Sprintf("lastReceiveAt: %v", err))
	}
	if listener.LastConfirmAt != nil {
		if err := channelTime(*listener.LastConfirmAt); err != nil {
			add("channel-json", fmt.Sprintf("lastConfirmAt: %v", err))
		}
	}
	if err := channelTime(listener.UpdatedAt); err != nil {
		add("channel-json", fmt.Sprintf("updatedAt: %v", err))
	}
	return problems
}

func channelBudgetComplete(data json.RawMessage) bool {
	raw, err := validateRawObject(data, []string{"elapsedLimit", "attemptLimit", "reservedJobMinutesLimit", "activeJobLimit", "reviewRoundLimit"}, nil)
	return err == nil && raw != nil
}

func channelTime(value string) error {
	_, err := parseChannelTime(value)
	return err
}

func parseChannelTime(value string) (time.Time, error) {
	parsed, err := time.Parse(channelTimeLayout, value)
	if err != nil || parsed.UTC().Format(channelTimeLayout) != value {
		return time.Time{}, fmt.Errorf("must be RFC 3339 UTC at second precision")
	}
	return parsed, nil
}

func channelInboxID(in *ChannelInbound) string {
	return in.Provider + "-" + in.MessageID
}

func channelOneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func channelCodeField(field string) bool {
	field = strings.TrimRight(field, ".,;:!?")
	if len(field) != 6 {
		return false
	}
	for i := range field {
		if field[i] < '0' || field[i] > '9' {
			return false
		}
	}
	return true
}

func channelLastFieldIsCode(text string) bool {
	fields := strings.Fields(text)
	return len(fields) > 0 && channelCodeField(fields[len(fields)-1])
}

func channelAnyFieldIsCode(texts ...string) bool {
	for _, text := range texts {
		for _, field := range strings.Fields(text) {
			if channelCodeField(field) {
				return true
			}
		}
	}
	return false
}

func qAnswerText(q *ChannelQuestion) string {
	if q.Answer == nil {
		return ""
	}
	return q.Answer.Text
}

func qAnswerReceipt(q *ChannelQuestion) string {
	if q.Answer == nil || q.Answer.Receipt == nil {
		return ""
	}
	return *q.Answer.Receipt
}

func channelRejectionReasons(rejections []ChannelRejection) []string {
	reasons := make([]string, len(rejections))
	for i, rejection := range rejections {
		reasons[i] = rejection.Reason
	}
	return reasons
}

// ChannelTuple is the portion of a question that selects a legal transition.
type ChannelTuple struct {
	State          string
	Phase          string
	Posting        *ChannelPosting
	PostingStale   bool
	ThreadNull     bool
	ReceiptRefNull bool
}

func (q *ChannelQuestion) Tuple() ChannelTuple {
	tuple := ChannelTuple{
		State:          q.State,
		Posting:        q.Posting,
		ThreadNull:     q.Thread == nil,
		ReceiptRefNull: q.Answer == nil || q.Answer.ReceiptRef == nil,
	}
	if q.Answer != nil {
		tuple.Phase = q.Answer.Phase
	}
	return tuple
}

// TupleAt includes the clock-relative posting staleness used by take-over.
func (q *ChannelQuestion) TupleAt(now time.Time, staleAfter time.Duration) ChannelTuple {
	tuple := q.Tuple()
	if q.Posting == nil {
		return tuple
	}
	postedAt, err := parseChannelTime(q.Posting.At)
	if err == nil {
		tuple.PostingStale = now.Sub(postedAt) > staleAfter
	}
	return tuple
}

// ChannelTransition is one named row of the channel question state machine.
type ChannelTransition struct {
	Name            string
	From            func(t ChannelTuple, me string) bool
	To              func(t ChannelTuple, me string) bool
	RejectionReason string
}

func channelPostingIs(t ChannelTuple, kind, by string) bool {
	return t.Posting != nil && t.Posting.Kind == kind && t.Posting.By == by
}

func channelPostingKindIs(t ChannelTuple, kind string) bool {
	return t.Posting != nil && t.Posting.Kind == kind
}

// ChannelMatrix is the complete declarative transition table for questions.
var ChannelMatrix = map[string]ChannelTransition{
	"ask": {
		Name: "ask",
		From: func(ChannelTuple, string) bool { return false },
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "open" && t.Phase == "" && channelPostingKindIs(t, "question") && t.ThreadNull && t.ReceiptRefNull
		},
	},
	"migrate": {
		Name: "migrate",
		From: func(ChannelTuple, string) bool { return false },
		// The mapped legacy state is selected before this predicate; every
		// mapped result shares the posting-null invariant.
		To: func(t ChannelTuple, _ string) bool { return t.Posting == nil },
	},
	"post-ref question": {
		Name: "post-ref question",
		From: func(t ChannelTuple, me string) bool {
			return t.State == "open" && t.Phase == "" && channelPostingIs(t, "question", me) && t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "open" && t.Phase == "" && t.Posting == nil && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"answer budget": {
		Name: "answer budget",
		From: func(t ChannelTuple, _ string) bool {
			return t.State == "open" && t.Phase == "" && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "recorded" && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"answer": {
		Name: "answer",
		From: func(t ChannelTuple, _ string) bool {
			return t.State == "open" && t.Phase == "" && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "approved" && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"approve-intent": {
		Name: "approve-intent",
		From: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "recorded" && t.Posting == nil && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "recorded" && channelPostingKindIs(t, "approval") && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"approved": {
		Name: "approved",
		From: func(t ChannelTuple, me string) bool {
			return t.State == "answered" && t.Phase == "recorded" && channelPostingIs(t, "approval", me) && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "approved" && t.Posting == nil && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"receipt-intent": {
		Name: "receipt-intent",
		From: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "approved" && t.Posting == nil && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "answered" && t.Phase == "approved" && channelPostingKindIs(t, "receipt") && !t.ThreadNull && t.ReceiptRefNull
		},
	},
	"receipted": {
		Name: "receipted",
		From: func(t ChannelTuple, me string) bool {
			return t.State == "answered" && t.Phase == "approved" && channelPostingIs(t, "receipt", me) && !t.ThreadNull && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "closed" && t.Phase == "receipted" && t.Posting == nil && !t.ThreadNull && !t.ReceiptRefNull
		},
	},
	"rejection intent": channelIntentTransition("rejection intent", "rejection"),
	"list intent":      channelIntentTransition("list intent", "list"),
	"silence intent":   channelIntentTransition("silence intent", "silence"),
	"rejection ref":    channelRefTransition("rejection ref", "rejection"),
	"list ref":         channelRefTransition("list ref", "list"),
	"silence ref":      channelRefTransition("silence ref", "silence"),
	"take-over": {
		Name: "take-over",
		From: func(t ChannelTuple, me string) bool {
			return t.Posting != nil && t.Posting.By != me && t.PostingStale
		},
		To: func(t ChannelTuple, _ string) bool { return t.Posting != nil },
	},
	"orphan-post": {
		Name: "orphan-post",
		From: func(ChannelTuple, string) bool { return true },
		To:   func(ChannelTuple, string) bool { return false },
	},
	"close": {
		Name: "close",
		From: func(t ChannelTuple, _ string) bool {
			return t.State == "open" && t.Phase == "" && t.Posting == nil && t.ReceiptRefNull
		},
		To: func(t ChannelTuple, _ string) bool {
			return t.State == "closed" && t.Phase == "" && t.Posting == nil && t.ReceiptRefNull
		},
	},
}

func channelIntentTransition(name, kind string) ChannelTransition {
	return ChannelTransition{
		Name: name,
		From: func(t ChannelTuple, _ string) bool {
			if t.Posting != nil || t.ThreadNull || t.State == "closed" {
				return false
			}
			return true
		},
		To: func(t ChannelTuple, _ string) bool { return channelPostingKindIs(t, kind) },
	}
}

func channelRefTransition(name, kind string) ChannelTransition {
	return ChannelTransition{
		Name: name,
		From: func(t ChannelTuple, me string) bool { return channelPostingIs(t, kind, me) },
		To:   func(t ChannelTuple, _ string) bool { return t.Posting == nil },
	}
}

// ClassifyChannelTransition decides whether the selected matrix row may be
// applied to a freshly fetched question tuple.
func ClassifyChannelTransition(e Endpoint, tip, qid, opid, me, writer string, present bool, current ChannelTuple, row ChannelTransition) (bool, error) {
	if !present && (row.Name == "ask" || row.Name == "migrate") {
		return true, nil
	}
	fromMatches := present && row.From != nil && row.From(current, me)
	if present && row.Name == "rejection intent" && current.State == "closed" {
		fromMatches = row.RejectionReason == "late" && current.Posting == nil && !current.ThreadNull
	}
	if fromMatches {
		return true, nil
	}
	applied, err := TrailerPresent(e, tip, opid)
	if err != nil {
		return false, err
	}
	if applied {
		return false, nil
	}
	if present && row.To != nil && row.To(current, me) && writer != "" && writer != opid {
		return false, LostToCompetitor{Winner: writer}
	}
	return false, fmt.Errorf("channel-transition: %s is %s, expected %s", qid, formatChannelTuple(current), row.Name)
}

func formatChannelTuple(t ChannelTuple) string {
	phase := t.Phase
	if phase == "" {
		phase = "null"
	}
	posting := "null"
	if t.Posting != nil {
		posting = fmt.Sprintf("posting %s %s", t.Posting.Kind, t.Posting.By)
	}
	thread := "set"
	if t.ThreadNull {
		thread = "null"
	}
	receiptRef := "set"
	if t.ReceiptRefNull {
		receiptRef = "null"
	}
	return fmt.Sprintf("(%s, %s, %s, thread %s, receiptRef %s)", t.State, phase, posting, thread, receiptRef)
}

// ChannelInboxMutate applies the create-once rule for one inbox record path.
func ChannelInboxMutate(e Endpoint, tip, recordPath string, content []byte) ([]Change, error) {
	listing, err := gitIn(e.Root, "ls-tree", "--name-only", tip, "--", recordPath)
	if err != nil {
		return nil, err
	}
	if listing == "" {
		return []Change{{Path: recordPath, Content: content}}, nil
	}
	blob, err := gitIn(e.Root, "show", tip+":"+recordPath)
	if err != nil {
		return nil, err
	}
	var record struct {
		Opid string `json:"opid"`
	}
	if err := json.Unmarshal([]byte(blob), &record); err != nil {
		return nil, fmt.Errorf("read inbox record opid: %w", err)
	}
	applied, err := TrailerPresent(e, tip, record.Opid)
	if err != nil {
		return nil, err
	}
	if applied {
		return nil, LostToCompetitor{Winner: record.Opid}
	}
	return nil, errors.New("inbox record present without its transaction")
}

// ChannelOpid mints the fresh operation identity used by a channel publish.
func ChannelOpid(machine, lineage string) (ulid, opid string, err error) {
	ulid, err = NewOperationULID()
	if err != nil {
		return "", "", err
	}
	return ulid, Opid(ulid, machine, lineage), nil
}
