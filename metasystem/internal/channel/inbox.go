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
	if err := flushNotices(ctx, c, result); err != nil {
		return err
	}
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

// textWithoutCode is the message on one line with a trailing code removed
// and every other code-shaped field masked (FCG-SECRET-15), so no code is
// ever committed and no repeated code can make a record the ledger refuses.
func textWithoutCode(text string) string {
	fields := strings.Fields(text)
	if n := len(fields); n > 0 && codeShaped(fields[n-1]) {
		fields = fields[:n-1]
	}
	for i, field := range fields {
		if codeShaped(field) {
			fields[i] = "[code]"
		}
	}
	return strings.Join(fields, " ")
}

func codeShaped(field string) bool {
	field = strings.TrimRight(field, ".,;:!?")
	return len(field) == 6 && strings.Trim(field, "0123456789") == ""
}

// validateInboxCommit is the channel validation every inbox commit passes;
// tests replace it to stage a record the ledger refuses.
var validateInboxCommit = func(e goal.Endpoint, commit string) error {
	if problems := goal.ValidateChannelTreeAt(e, commit); len(problems) > 0 {
		return fmt.Errorf("%s", problems[0])
	}
	return nil
}

// commitInbound publishes one update's inbox record. The replay check runs on
// the fetched tip: a step already on another message makes this one
// replayed. It reports whether this installation won the commit. A record the
// ledger refuses by name is replaced by a minimal skipped record, so no single
// update ever blocks the queue. A rejected record's notice is queued by the
// installation that committed it, before it is posted.
func commitInbound(ctx context.Context, c PollConfig, ep goal.Endpoint, in Inbound, result *PollResult) (bool, error) {
	if in.Ref.ID == "" || strings.ContainsAny(in.Ref.ID, "/ \t\n") {
		return false, fmt.Errorf("inbox refused %s: the provider message id %q cannot name a record", updateName(in), in.Ref.ID)
	}
	draft, reason := draftInbound(c, in)
	written, existing, res, err := publishInbound(c, ep, draft)
	if err != nil {
		return false, fmt.Errorf("inbox refused %s: %w", updateName(in), err)
	}
	if res.Outcome == goal.OutcomeRejected {
		skipped := draft
		skipped.Text, skipped.Step, skipped.Outcome, skipped.Question = "", nil, "skipped", "unmatched"
		reason = "unrecordable"
		if written, existing, res, err = publishInbound(c, ep, skipped); err != nil {
			return false, fmt.Errorf("inbox refused %s: %w", updateName(in), err)
		}
	}
	notices := noticesPath(c)
	switch res.Outcome {
	case goal.OutcomeLost:
		// A record this installation committed earlier and never got to
		// notice (a kill between its commit and its notice) is noticed now.
		if existing == nil || existing.Outcome == "verified" {
			return false, nil
		}
		if _, err := goal.ReadEntry(c.RepoRoot, existing.Opid); err != nil {
			return false, nil
		}
		path := goal.ChannelInboxPath(c.Destination, existing)
		if err := queueNotice(notices, path, noticeFor(reasonFor(*existing, c.Now)), existing.ReplyTo, in.ThreadID); err != nil {
			return false, err
		}
		return false, flushNotices(ctx, c, result)
	case goal.OutcomeConfirmed:
	default:
		return false, fmt.Errorf("inbox refused %s: %s %s", updateName(in), res.Outcome, res.Detail)
	}
	if err := fail(c, "inbox-published"); err != nil {
		return false, err
	}
	if written.Outcome == "verified" {
		return true, nil
	}
	if written.Outcome == "replayed" {
		reason = "replayed code"
	}
	if err := queueNotice(notices, goal.ChannelInboxPath(c.Destination, &written), noticeFor(reason), written.ReplyTo, in.ThreadID); err != nil {
		return false, err
	}
	if err := fail(c, "inbox-notice-pending"); err != nil {
		return false, err
	}
	return true, flushNotices(ctx, c, result)
}

// publishInbound runs one inbox Publish of draft under a fresh opid. It
// returns the record as written and, on a loss, the record already there.
func publishInbound(c PollConfig, ep goal.Endpoint, draft goal.ChannelInbound) (goal.ChannelInbound, *goal.ChannelInbound, goal.PublishResult, error) {
	ulid, err := goal.NewOperationULID()
	if err != nil {
		return draft, nil, goal.PublishResult{}, err
	}
	opid := goal.Opid(ulid, c.Machine, c.Lineage)
	draft.Opid, draft.ReceivedBy, draft.ReceivedAt = opid, c.Machine, c.Now.UTC().Format(channelRecordTime)
	path := goal.ChannelInboxPath(c.Destination, &draft)
	written := draft
	var found *goal.ChannelInbound
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
					found = existing
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
		Validate: func(commit string) error { return validateInboxCommit(ep, commit) },
	})
	return written, found, res, err
}

func noticeFor(reason string) string {
	return "not recorded: " + reason + ". Reply to the question above with your answer and your code"
}

// reasonFor names a committed record's rejection for a notice sent after the
// fact.
func reasonFor(rec goal.ChannelInbound, now time.Time) string {
	switch rec.Outcome {
	case "wrong-user":
		return "wrong user"
	case "no-code":
		return "no code"
	case "bad-code":
		return "bad code"
	case "replayed":
		return "replayed code"
	case "stale":
		if sent, err := time.Parse(time.RFC3339, rec.SentAt); err == nil {
			return fmt.Sprintf("code too old: sent %ds before the poll", int64(now.Sub(sent)/time.Second))
		}
		return "code too old"
	default:
		return "unrecordable"
	}
}

// The notice ledger is local to the committing installation: a notice is
// queued before it is posted and marked posted after, so a failed post or a
// kill is retried on the next tick. A kill between the post and its mark
// may post one duplicate; nothing posts zero.
type pendingNotice struct {
	Record   string `json:"record"`
	Text     string `json:"text"`
	ReplyTo  string `json:"replyTo,omitempty"`
	ThreadID string `json:"threadID,omitempty"`
}

type noticeLedger struct {
	Pending []pendingNotice `json:"pending"`
	Posted  []string        `json:"posted"`
}

func noticesPath(c PollConfig) string {
	return filepath.Join(channelRoot(c.RepoRoot), c.Destination, "notices.json")
}

func readNotices(path string) (noticeLedger, error) {
	var ledger noticeLedger
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return ledger, err
	}
	return ledger, json.Unmarshal(b, &ledger)
}

func queueNotice(path, record, text string, replyTo *string, threadID string) error {
	ledger, err := readNotices(path)
	if err != nil {
		return err
	}
	for _, posted := range ledger.Posted {
		if posted == record {
			return nil
		}
	}
	for _, pending := range ledger.Pending {
		if pending.Record == record {
			return nil
		}
	}
	notice := pendingNotice{Record: record, Text: text, ThreadID: threadID}
	if replyTo != nil {
		notice.ReplyTo = *replyTo
	}
	ledger.Pending = append(ledger.Pending, notice)
	return writeJSON(path, ledger)
}

// flushNotices posts every queued notice in its question's thread; one whose
// post fails stays queued for the next tick.
func flushNotices(ctx context.Context, c PollConfig, result *PollResult) error {
	path := noticesPath(c)
	ledger, err := readNotices(path)
	if err != nil || len(ledger.Pending) == 0 {
		return err
	}
	var kept []pendingNotice
	for _, notice := range ledger.Pending {
		var thread *MessageRef
		if notice.ReplyTo != "" {
			root := notice.ThreadID
			if root == "" {
				root = notice.ReplyTo
			}
			thread = &MessageRef{ID: notice.ReplyTo, ThreadID: root}
		}
		if _, err := c.Provider.Post(ctx, c.DestinationConfig, notice.Text, thread); err != nil {
			result.Undelivered++
			kept = append(kept, notice)
			continue
		}
		ledger.Posted = append(ledger.Posted, notice.Record)
	}
	ledger.Pending = kept
	return writeJSON(path, ledger)
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
