package goal

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ChannelTree is the channel ledger state at one immutable tip.
type ChannelTree struct {
	Questions map[string]*ChannelQuestion
	Inbox     map[string]*ChannelInbound
	Listeners map[string]*ChannelListener
}

// ReadChannelTree reads every channel record at tip. Schema validation stays
// with ValidateChannelTree; this reader reports records that cannot be decoded.
func ReadChannelTree(e Endpoint, tip string) (*ChannelTree, error) {
	tree := &ChannelTree{
		Questions: make(map[string]*ChannelQuestion),
		Inbox:     make(map[string]*ChannelInbound),
		Listeners: make(map[string]*ChannelListener),
	}
	out, err := gitIn(e.Root, "ls-tree", "-r", "--name-only", tip, "--", ChannelPrefix)
	if err != nil {
		return nil, fmt.Errorf("list channel tree at %s: %w", short(tip), err)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if filePath := strings.TrimSpace(line); filePath != "" {
			paths = append(paths, filePath)
		}
	}
	if len(paths) == 0 {
		return tree, nil
	}
	files, err := readCommitGoalBlobs(e.Root, tip, paths)
	if err != nil {
		return nil, fmt.Errorf("read channel tree at %s: %w", short(tip), err)
	}
	for _, filePath := range paths {
		location, ok := classifyChannelPath(filePath)
		if !ok {
			return nil, fmt.Errorf("read channel record %s: path is not a question, inbox record, or listener record", filePath)
		}
		switch location.kind {
		case "question":
			question, _, decodeErr := decodeChannelQuestion(files[filePath])
			if decodeErr != nil {
				return nil, fmt.Errorf("read channel record %s: %w", filePath, decodeErr)
			}
			tree.Questions[location.id] = question
		case "inbound":
			record, decodeErr := decodeChannelInbound(files[filePath])
			if decodeErr != nil {
				return nil, fmt.Errorf("read channel record %s: %w", filePath, decodeErr)
			}
			tree.Inbox[location.destination+"/"+location.basename] = record
		case "listener":
			listener, decodeErr := decodeChannelListener(files[filePath])
			if decodeErr != nil {
				return nil, fmt.Errorf("read channel record %s: %w", filePath, decodeErr)
			}
			tree.Listeners[location.id] = listener
		}
	}
	return tree, nil
}

// MatchChannelInbound identifies the one question an inbound record can name.
// The callback owns the exact token predicate for an open, posted question.
func MatchChannelInbound(tree *ChannelTree, record ChannelInbound, wants func(q *ChannelQuestion) bool) (question string, bound bool) {
	ids := make([]string, 0, len(tree.Questions))
	for id := range tree.Questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if record.ReplyTo != nil {
		for _, id := range ids {
			q := tree.Questions[id]
			if q.Destination != record.Destination || !channelQuestionNamesReply(q, *record.ReplyTo) {
				continue
			}
			return id, q.State == "open" && record.Outcome == "verified"
		}
		return "unmatched", false
	}
	if record.Outcome != "verified" {
		return "unmatched", false
	}

	openCount := 0
	var matches []string
	ambiguousToken := false
	for _, id := range ids {
		q := tree.Questions[id]
		if q.Destination != record.Destination || q.State != "open" {
			continue
		}
		openCount++
		if q.Thread == nil || q.Wants == "" {
			continue
		}
		if channelTokenMatchCount(record.Text, q.Wants) > 1 {
			ambiguousToken = true
		}
		if wants(q) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 1 && !ambiguousToken {
		return matches[0], true
	}
	if len(matches) == 0 && openCount == 1 && !ambiguousToken {
		return "unbound", false
	}
	return "unmatched", false
}

func channelQuestionNamesReply(q *ChannelQuestion, replyTo string) bool {
	if q.Thread != nil && q.Thread.ID == replyTo {
		return true
	}
	for _, rejection := range q.Rejected {
		if rejection.PostRef != nil && rejection.PostRef.ID == replyTo {
			return true
		}
	}
	for _, ref := range q.OrphanPosts {
		if ref.ID == replyTo {
			return true
		}
	}
	return q.Answer != nil && q.Answer.ReceiptRef != nil && q.Answer.ReceiptRef.ID == replyTo
}

func channelTokenAppearsOnce(text, token string) bool {
	return channelTokenMatchCount(text, token) == 1
}

func channelTokenMatchCount(text, token string) int {
	fields := strings.Fields(text)
	wanted := strings.Fields(token)
	if len(wanted) == 0 {
		return 0
	}
	matches := 0
	for i := 0; i+len(wanted) <= len(fields); i++ {
		match := true
		for j := range wanted {
			field := fields[i+j]
			if j == len(wanted)-1 {
				field = strings.TrimRight(field, ".,;:!?")
			}
			if field != wanted[j] {
				match = false
				break
			}
		}
		if match {
			matches++
		}
	}
	return matches
}

// ChannelInboundRequest builds the create-once transaction for one inbound
// provider message. Its callback redoes replay and match decisions at every tip.
func ChannelInboundRequest(e Endpoint, machine, lineage, opid string, record ChannelInbound, now time.Time, decided *ChannelInbound) PublishRequest {
	recordPath := ChannelPrefix + "inbox/" + record.Destination + "/" + record.Provider + "-" + record.MessageID + ".json"
	initialOutcome := record.Outcome
	initialQuestion := record.Question
	return PublishRequest{
		Opid:    opid,
		Machine: machine,
		Lineage: lineage,
		Intent: Intent{
			Verb:    "inbox",
			Targets: []string{recordPath},
			Args: map[string]string{
				"provider":    record.Provider,
				"destination": record.Destination,
				"messageId":   record.MessageID,
				"updateId":    record.UpdateID,
				"outcome":     initialOutcome,
				"question":    initialQuestion,
			},
		},
		Message:  "channel inbox " + record.Provider + "-" + record.MessageID,
		Validate: func(commit string) error { return ValidateCommit(e.Root, commit) },
		Mutate: func(tip string) ([]Change, error) {
			candidate := record
			candidate.Opid = opid
			initialContent, err := MarshalChannel(candidate)
			if err != nil {
				return nil, err
			}
			if _, err := ChannelInboxMutate(e, tip, recordPath, initialContent); err != nil {
				return nil, err
			}

			channelTree, err := ReadChannelTree(e, tip)
			if err != nil {
				return nil, err
			}
			if candidate.Step != nil {
				for _, existing := range channelTree.Inbox {
					if existing.Step != nil && *existing.Step == *candidate.Step && existing.MessageID != candidate.MessageID {
						candidate.Outcome = "replayed"
						break
					}
				}
			}
			questionID, bound := MatchChannelInbound(channelTree, candidate, func(q *ChannelQuestion) bool {
				return channelTokenAppearsOnce(candidate.Text, q.Wants)
			})
			candidate.Question = questionID
			question := channelTree.Questions[questionID]
			if candidate.Outcome == "verified" && question != nil && question.State != "open" {
				candidate.Outcome = "late"
				bound = false
			}
			if candidate.Outcome == "replayed" {
				bound = false
			}

			var changes []Change
			if bound && candidate.Outcome == "verified" {
				if candidate.Step == nil {
					return nil, fmt.Errorf("verified channel record %s has no step", channelInboxID(&candidate))
				}
				rowName := "answer"
				if question.Kind == "budget-above-norm" && question.Budget != nil && candidate.Text == question.Wants {
					rowName = "answer budget"
				}
				writer := ""
				if question.Answer != nil {
					writer = question.Answer.Opid
				}
				apply, classifyErr := ClassifyChannelTransition(e, tip, questionID, opid, machine, writer, true, question.Tuple(), ChannelMatrix[rowName])
				if classifyErr != nil {
					return nil, classifyErr
				}
				if !apply {
					return nil, AlreadyApplied{}
				}

				phase, approvalULID, receipt, answerErr := channelAnswerDisposition(question, candidate)
				if answerErr != nil {
					return nil, answerErr
				}
				refThreadID := ""
				if candidate.ReplyTo != nil {
					refThreadID = *candidate.ReplyTo
				}
				question.State = "answered"
				question.Answer = &ChannelAnswer{
					Text: candidate.Text, UserID: candidate.UserID,
					Ref: ChannelRef{Provider: candidate.Provider, ID: candidate.MessageID, ThreadID: refThreadID},
					At:  candidate.SentAt, Step: candidate.Step, InboxID: channelInboxID(&candidate),
					Opid: opid, Phase: phase, ApprovalULID: approvalULID, Receipt: receipt, ReceiptRef: nil,
				}
				questionContent, marshalErr := MarshalChannel(question)
				if marshalErr != nil {
					return nil, marshalErr
				}
				changes = append(changes, Change{Path: ChannelPrefix + "questions/" + questionID + ".json", Content: questionContent})

				goalTree, loadErr := loadTree(e.Root, tip)
				if loadErr != nil {
					return nil, loadErr
				}
				proofRef := refThreadID + "/" + candidate.MessageID
				goalChange, goalErr := answerGoalChange(goalTree, question.Goal, questionID, candidate.Text, candidate.Text, opid, now.UTC().Format(time.RFC3339), AnswerProof{
					Provider: candidate.Provider, User: candidate.UserID, Ref: proofRef, Step: *candidate.Step,
				})
				if goalErr != nil {
					return nil, goalErr
				}
				changes = append(changes, goalChange)
			}

			content, err := MarshalChannel(candidate)
			if err != nil {
				return nil, err
			}
			changes = append([]Change{{Path: recordPath, Content: content}}, changes...)
			if decided != nil {
				*decided = candidate
			}
			return changes, nil
		},
	}
}

func channelAnswerDisposition(question *ChannelQuestion, record ChannelInbound) (phase string, approvalULID *string, receipt *string, err error) {
	if question.Kind == "budget-above-norm" && question.Budget != nil && record.Text == question.Wants {
		approval, approvalErr := channelApprovalULID(record.SentAt, question.ID)
		if approvalErr != nil {
			return "", nil, nil, approvalErr
		}
		return "recorded", &approval, nil, nil
	}
	text := "recorded"
	switch {
	case question.Kind == "stop" && channelTokenAppearsOnce(record.Text, question.Wants):
		text = "recorded: " + question.Goal + " approved for execution"
	case question.Kind == "budget-above-norm" && question.Budget != nil:
		text = "recorded: " + question.Goal + " box not raised; the reply did not carry the token"
	case question.Kind == "budget-above-norm":
		text = "recorded: " + question.Goal + " has no proposed box on this question; nothing raised"
	}
	return "approved", nil, &text, nil
}

func channelApprovalULID(answerAt, questionID string) (string, error) {
	at, err := parseChannelTime(answerAt)
	if err != nil {
		return "", fmt.Errorf("derive channel approval ULID: answer time: %w", err)
	}
	millis := at.UnixMilli()
	if millis < 0 || millis >= 1<<48 {
		return "", fmt.Errorf("derive channel approval ULID: answer time is outside the 48-bit range")
	}
	var raw [16]byte
	for i := 5; i >= 0; i-- {
		raw[i] = byte(millis)
		millis >>= 8
	}
	hash := sha256.Sum256([]byte("approve:" + questionID))
	copy(raw[6:], hash[:10])
	return encodeChannelULID(raw), nil
}

func encodeChannelULID(raw [16]byte) string {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	encoded := make([]byte, 26)
	for group := range encoded {
		value := 0
		for bit := 0; bit < 5; bit++ {
			streamBit := group*5 + bit
			value <<= 1
			if streamBit < 2 {
				continue
			}
			dataBit := streamBit - 2
			value |= int((raw[dataBit/8] >> (7 - dataBit%8)) & 1)
		}
		encoded[group] = alphabet[value]
	}
	return string(encoded)
}
