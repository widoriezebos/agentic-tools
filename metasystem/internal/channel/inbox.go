package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The ledger inbox (Decision 8, step 1 of the fleet channel gateway): every
// installation polls the one bot with no offset, commits each update to
// plans/channel/inbox/<destination>/<provider>-<message id>.json, and confirms
// the update only once that commit is durable. The first commit wins; a later
// commit of the same message is lost to it and still confirms. Each
// installation then matches its own open questions from the inbox.

const channelRecordTime = "2006-01-02T15:04:05Z"

// receiveConfigured says whether this installation can check a reply. An
// installation without the human's id or code secret cannot, so it receives
// nothing: it never confirms an update it could not judge.
func (c PollConfig) receiveConfigured() bool {
	return strings.TrimSpace(c.HumanUserID) != "" && strings.TrimSpace(c.TOTPSecret) != ""
}

// receiveToInbox receives every unconfirmed update, commits each one in
// order, and confirms each committed update's Ack. A provider conflict (two
// installations polling one bot) is the ordinary case: nothing is received
// this pass and the next poll retries.
func receiveToInbox(ctx context.Context, c PollConfig, threads []MessageRef, endpoint func() (goal.Endpoint, error), result *PollResult) error {
	inbound, batch, err := c.Provider.Receive(ctx, c.DestinationConfig, threads, "")
	if IsKind(err, Busy) {
		return nil
	}
	if err != nil {
		return scrubErr(err, c)
	}
	result.Received = len(inbound)
	if len(inbound) == 0 {
		return nil
	}
	ep, err := endpoint()
	if err != nil {
		return err
	}
	all := true
	for _, in := range inbound {
		if result.Dispositions >= c.MaxDispositions {
			all = false
			break
		}
		won, err := commitInbound(ctx, c, ep, in, result)
		if err != nil {
			return err
		}
		if won {
			result.Dispositions++
		}
		if err := fail(c, "inbox-committed"); err != nil {
			return err
		}
		if err := c.Provider.Confirm(ctx, c.DestinationConfig, in.Ack); err != nil {
			if IsKind(err, Busy) {
				return nil
			}
			return scrubErr(err, c)
		}
	}
	if all && batch != "" && batch != inbound[len(inbound)-1].Ack {
		if err := c.Provider.Confirm(ctx, c.DestinationConfig, batch); err != nil && !IsKind(err, Busy) {
			return scrubErr(err, c)
		}
	}
	return nil
}

// draftInbound checks sender and code in order (user id, code present, not
// stale, TOTP at the send time) and builds the record with the code removed.
// It returns the reason a rejected reply is told.
func draftInbound(c PollConfig, in Inbound) (goal.ChannelInbound, string) {
	sentAt := in.SentAt
	if sentAt.IsZero() {
		sentAt = c.Now
	}
	updateID := in.Ref.ID
	if in.UpdateID != 0 {
		updateID = strconv.FormatInt(in.UpdateID, 10)
	}
	rec := goal.ChannelInbound{
		Provider: c.ProviderName, Destination: c.Destination, MessageID: in.Ref.ID, UpdateID: updateID,
		UserID: in.UserID, SentAt: sentAt.UTC().Format(channelRecordTime), Text: textWithoutCode(in.Text), Question: "unmatched",
	}
	if in.Ref.ThreadID != "" {
		replyTo := in.Ref.ThreadID
		rec.ReplyTo = &replyTo
	}
	_, code, hasCode := SplitTOTP(in.Text)
	switch {
	case in.UserID != c.HumanUserID:
		rec.Outcome = "wrong-user"
		return rec, "wrong user"
	case !hasCode:
		rec.Outcome = "no-code"
		return rec, "no code"
	case !in.SentAt.IsZero() && c.Now.Sub(in.SentAt) > channelPollInterval+time.Duration(TOTPStep)*time.Second:
		rec.Outcome = "stale"
		return rec, fmt.Sprintf("code too old: sent %ds before the poll", int64(c.Now.Sub(in.SentAt)/time.Second))
	}
	step, ok := VerifyTOTP(c.TOTPSecret, code, sentAt)
	if !ok {
		rec.Outcome = "bad-code"
		return rec, "bad code"
	}
	rec.Outcome = "verified"
	rec.Step = &step
	return rec, ""
}

// textWithoutCode is the message on one line with a trailing code removed,
// so no code is ever committed.
func textWithoutCode(text string) string {
	fields := strings.Fields(text)
	if n := len(fields); n > 0 {
		last := strings.TrimRight(fields[n-1], ".,;:!?")
		if len(last) == 6 && strings.Trim(last, "0123456789") == "" {
			fields = fields[:n-1]
		}
	}
	return strings.Join(fields, " ")
}

// commitInbound publishes one update's inbox record. The replay check runs on
// the fetched tip: a step already on another message makes this one
// replayed. It reports whether this installation won the commit; only the
// winner tells the human a reply was not recorded, once per record.
func commitInbound(ctx context.Context, c PollConfig, ep goal.Endpoint, in Inbound, result *PollResult) (bool, error) {
	if in.Ref.ID == "" || strings.ContainsAny(in.Ref.ID, "/ \t\n") {
		return false, fmt.Errorf("inbox refused %s: the provider message id %q cannot name a record", updateName(in), in.Ref.ID)
	}
	draft, reason := draftInbound(c, in)
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return false, err
	}
	opid := goal.Opid(ulid, c.Machine, c.Lineage)
	draft.Opid, draft.ReceivedBy, draft.ReceivedAt = opid, c.Machine, c.Now.UTC().Format(channelRecordTime)
	path := goal.ChannelInboxPath(c.Destination, &draft)
	written := draft
	res, err := goal.Publish(ep, goal.PublishRequest{
		Opid: opid, Machine: c.Machine, Lineage: c.Lineage, Intent: goal.Intent{Verb: "inbox"},
		Message: "channel inbox " + c.ProviderName + "-" + draft.MessageID,
		Mutate: func(tip string) ([]goal.Change, error) {
			records, err := goal.ReadChannelInbox(ep, tip)
			if err != nil {
				return nil, err
			}
			if existing := records[path]; existing != nil {
				if existing.Opid == opid {
					return nil, goal.AlreadyApplied{}
				}
				present, err := goal.TrailerPresent(ep, tip, existing.Opid)
				if err != nil {
					return nil, err
				}
				if present {
					return nil, goal.LostToCompetitor{Winner: existing.Opid}
				}
				return nil, errors.New("inbox record present without its transaction")
			}
			written = draft
			if draft.Step != nil {
				for _, other := range records {
					if other.Step != nil && *other.Step == *draft.Step && other.MessageID != draft.MessageID {
						written.Outcome = "replayed"
						break
					}
				}
			}
			body, err := goal.RenderChannelInbound(&written)
			if err != nil {
				return nil, err
			}
			return []goal.Change{{Path: path, Content: body}}, nil
		},
		Validate: func(commit string) error {
			if problems := goal.ValidateChannelTreeAt(ep, commit); len(problems) > 0 {
				return fmt.Errorf("%s", problems[0])
			}
			return nil
		},
	})
	if err != nil {
		return false, fmt.Errorf("inbox refused %s: %w", updateName(in), err)
	}
	switch res.Outcome {
	case goal.OutcomeLost:
		return false, nil
	case goal.OutcomeConfirmed:
	default:
		return false, fmt.Errorf("inbox refused %s: %s %s", updateName(in), res.Outcome, res.Detail)
	}
	if written.Outcome == "replayed" {
		reason = "replayed code"
	}
	if written.Outcome != "verified" {
		var thread *MessageRef
		if written.ReplyTo != nil {
			root := in.ThreadID
			if root == "" {
				root = *written.ReplyTo
			}
			thread = &MessageRef{ID: *written.ReplyTo, ThreadID: root}
		}
		if _, err := c.Provider.Post(ctx, c.DestinationConfig, "not recorded: "+reason+". Reply to the question above with your answer and your code", thread); err != nil {
			result.Undelivered++
		}
	}
	return true, nil
}

func updateName(in Inbound) string {
	if in.UpdateID != 0 {
		return strconv.FormatInt(in.UpdateID, 10)
	}
	return in.Ref.ID
}

// inboxRecord is one verified record this installation may match.
type inboxRecord struct {
	path string
	in   *goal.ChannelInbound
	sent time.Time
}

// matchInbox reads the verified inbox records at the tip and matches them
// against this installation's own open questions: threaded first (the reply
// is to the question's post), then by token for an unthreaded message, never
// a stray. A record that names none of its questions is left for the other
// installation.
func matchInbox(ctx context.Context, c PollConfig, status StatusState, statusRoot string, endpoint func() (goal.Endpoint, error), resolveEndpoint pollEndpointResolver, result *PollResult) error {
	questions, err := listQuestions(c.RepoRoot)
	if err != nil {
		return err
	}
	open := 0
	for _, q := range questions {
		if q.State == "open" && q.Answer == nil {
			open++
		}
	}
	if open == 0 && (status.GoalID == "" || statusRoot == "") {
		return nil
	}
	ep, err := endpoint()
	if err != nil {
		return err
	}
	nonce, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	committed, err := goal.ReadChannelInboxAtTip(ep, "channel-match-"+nonce)
	if err != nil {
		return err
	}
	var records []inboxRecord
	for path, in := range committed {
		if in.Destination != c.Destination || in.Outcome != "verified" || in.Step == nil {
			continue
		}
		sent, err := time.Parse(time.RFC3339, in.SentAt)
		if err != nil {
			continue
		}
		records = append(records, inboxRecord{path: path, in: in, sent: sent})
	}
	sort.Slice(records, func(i, j int) bool {
		if !records[i].sent.Equal(records[j].sent) {
			return records[i].sent.Before(records[j].sent)
		}
		return records[i].path < records[j].path
	})
	handledPath := filepath.Join(channelRoot(c.RepoRoot), c.Destination, "inbox-handled.json")
	handled := readHandled(handledPath)
	for _, record := range records {
		if result.Dispositions >= c.MaxDispositions {
			return nil
		}
		in := record.in
		questions, err = listQuestions(c.RepoRoot)
		if err != nil {
			return err
		}
		var match *Question
		if in.ReplyTo != nil {
			replyTo := *in.ReplyTo
			threaded := false
			for i := range questions {
				q := &questions[i]
				if q.Thread != nil && (replyTo == q.Thread.ID || replyTo == threadRoot(*q.Thread)) {
					threaded = true
					if q.State == "open" && q.Answer == nil {
						match = q
					}
				}
			}
			if !threaded && status.GoalID != "" && statusRoot != "" && (replyTo == statusRoot || replyTo == status.Ref.ID) && !handled[record.path] {
				if err := disposeStatusReplyWithEndpoint(ctx, c, status, in, resolveEndpoint); err != nil {
					return scrubErr(err, c)
				}
				handled[record.path] = true
				if err := writeJSON(handledPath, sortedHandled(handled)); err != nil {
					return err
				}
				result.Dispositions++
				continue
			}
		} else {
			for i := range questions {
				q := &questions[i]
				if q.State != "open" || q.Answer != nil || q.Wants == "" || record.sent.Before(q.OpenedAt.Truncate(time.Second)) || !goal.ChannelTokenIn(in.Text, q.Wants) {
					continue
				}
				if match != nil {
					match = nil
					break
				}
				match = q
			}
		}
		if match == nil {
			continue
		}
		if err := recordMatch(ctx, c, match, in, resolveEndpoint); err != nil {
			return err
		}
		result.Dispositions++
	}
	return nil
}

func threadRoot(ref MessageRef) string {
	if ref.ThreadID != "" {
		return ref.ThreadID
	}
	return ref.ID
}

// recordMatch records a verified inbox record as the question's answer exactly
// as the poll always has, then advances it to the goal ledger.
func recordMatch(ctx context.Context, c PollConfig, q *Question, in *goal.ChannelInbound, resolveEndpoint pollEndpointResolver) error {
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return err
	}
	opid := goal.Opid(ulid, c.Machine, c.Lineage)
	approvalULID := ""
	if q.Kind == "budget-above-norm" && strings.TrimSpace(in.Text) == q.Wants {
		if approvalULID, err = goal.NewOperationULID(); err != nil {
			return err
		}
	}
	ref := MessageRef{ID: in.MessageID}
	if in.ReplyTo != nil {
		ref.ThreadID = *in.ReplyTo
	}
	q.Answer = &Answer{Text: in.Text, UserID: in.UserID, Ref: ref, At: c.Now, Step: *in.Step, ULID: ulid, Opid: opid, ApprovalULID: approvalULID, Phase: "matched"}
	q.State = "answered"
	if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
		return err
	}
	if err = fail(c, "matched"); err != nil {
		return err
	}
	if err = advanceAnswerWithEndpoint(ctx, c, q, resolveEndpoint); err != nil {
		return scrubErr(err, c)
	}
	return nil
}

func readHandled(path string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var paths []string
	if json.Unmarshal(b, &paths) == nil {
		for _, p := range paths {
			out[p] = true
		}
	}
	return out
}

func sortedHandled(handled map[string]bool) []string {
	out := make([]string, 0, len(handled))
	for p := range handled {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
