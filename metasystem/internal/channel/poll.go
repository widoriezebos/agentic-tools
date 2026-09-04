package channel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"golang.org/x/sys/unix"
)

type PollConfig struct {
	RepoRoot, Destination, ProviderName, HumanUserID, TOTPSecret, Machine, Lineage string
	Provider                                                                       Provider
	DestinationConfig                                                              DestinationConfig
	Now                                                                            time.Time
	MaxDispositions                                                                int
	FailurePoint                                                                   func(string) error
}
type PollResult struct {
	Busy                                bool
	Received, Dispositions, Undelivered int
}
type consumedRow struct {
	Step                            int64 `json:"step"`
	Destination, Provider, ThreadID string
	Ref                             MessageRef `json:"ref"`
	QID                             string     `json:"qid"`
}
type cursorRecord struct {
	Provider string `json:"provider"`
	Cursor   Cursor `json:"cursor"`
}

const channelPollInterval = 2 * time.Minute

func Poll(ctx context.Context, c PollConfig) (PollResult, error) {
	var result PollResult
	if c.MaxDispositions <= 0 {
		c.MaxDispositions = 5
	}
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	}
	if err := os.MkdirAll(channelRoot(c.RepoRoot), 0o755); err != nil {
		return result, err
	}
	lock, err := os.OpenFile(filepath.Join(channelRoot(c.RepoRoot), "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return result, err
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if err == unix.EWOULDBLOCK {
			return PollResult{Busy: true}, nil
		}
		return result, err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	questions, err := listQuestions(c.RepoRoot)
	if err != nil {
		return result, err
	}
	for i := range questions {
		if result.Dispositions >= c.MaxDispositions {
			break
		}
		q := &questions[i]
		if q.State == "open" && q.Thread == nil {
			ref, postErr := c.Provider.Post(ctx, c.DestinationConfig, renderQuestion(*q), nil)
			if postErr != nil {
				q.Undelivered++
				result.Undelivered++
			} else {
				q.Thread = &ref
			}
			if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
				return result, err
			}
			result.Dispositions++
		}
	}
	for i := range questions {
		if result.Dispositions >= c.MaxDispositions {
			break
		}
		q := &questions[i]
		if q.Answer != nil && q.Answer.Phase != "closed" {
			if err = advanceAnswer(ctx, c, q); err != nil {
				var postErr receiptPostError
				if errors.As(err, &postErr) {
					result.Undelivered++
					result.Dispositions++
					continue
				}
				return result, scrubErr(err, c)
			}
			result.Dispositions++
		}
	}
	threads := []MessageRef{}
	byThread := map[string]string{}
	matchedRefs := map[MessageRef]bool{}
	status := LoadStatusState(c.RepoRoot)
	statusRoot := status.Ref.ThreadID
	if statusRoot == "" {
		statusRoot = status.Ref.ID
	}
	if status.GoalID != "" && statusRoot != "" {
		threads = append(threads, MessageRef{ID: status.Ref.ID, ThreadID: statusRoot})
	}
	for _, q := range questions {
		if q.Answer != nil {
			matchedRefs[q.Answer.Ref] = true
		}
		if q.State == "open" && q.Thread != nil {
			root := q.Thread.ThreadID
			if root == "" {
				root = q.Thread.ID
			}
			threads = append(threads, MessageRef{ID: q.Thread.ID, ThreadID: root})
			for _, rejection := range q.Rejected {
				if rejection.PostRef != nil {
					threads = append(threads, MessageRef{ID: rejection.PostRef.ID, ThreadID: root})
				}
			}
			byThread[root] = q.ID
		}
	}
	curPath := filepath.Join(channelRoot(c.RepoRoot), c.Destination, "cursor.json")
	var old cursorRecord
	if b, e := os.ReadFile(curPath); e == nil {
		_ = json.Unmarshal(b, &old)
	}
	if old.Provider != c.ProviderName {
		old.Cursor = ""
	}
	inbound, next, err := c.Provider.Receive(ctx, c.DestinationConfig, threads, old.Cursor)
	if err != nil {
		return result, scrubErr(err, c)
	}
	result.Received = len(inbound)
	allDurable := true
	for _, in := range inbound {
		if matchedRefs[in.Ref] {
			continue
		}
		if result.Dispositions >= c.MaxDispositions {
			allDurable = false
			break
		}
		qid := byThread[in.ThreadID]
		if qid == "" && status.GoalID != "" && in.ThreadID == statusRoot {
			if err = disposeStatusReply(ctx, c, status, in); err != nil {
				return result, scrubErr(err, c)
			}
			result.Dispositions++
			continue
		}
		if qid == "" {
			if unmatchedAlready(filepath.Join(channelRoot(c.RepoRoot), c.Destination, "unmatched.jsonl"), in.Ref) {
				continue
			}
			if err = appendJSONL(filepath.Join(channelRoot(c.RepoRoot), c.Destination, "unmatched.jsonl"), in); err != nil {
				return result, err
			}
			result.Dispositions++
			continue
		}
		q, err := ReadQuestion(c.RepoRoot, qid)
		if err != nil {
			return result, err
		}
		if alreadyRejected(q, in.Ref) {
			continue
		}
		answer, code, hasCode := SplitTOTP(in.Text)
		step, reason := verifyInbound(c, in, code, hasCode)
		if reason == "" {
			row := consumedRow{Step: step, Destination: c.Destination, Provider: c.ProviderName, ThreadID: in.ThreadID, Ref: in.Ref, QID: q.ID}
			ok, e := consume(c.RepoRoot, row, c.Now)
			if e != nil {
				return result, e
			}
			if !ok {
				reason = "replayed code"
			}
		}
		if reason != "" {
			posted := len(q.Rejected) < 3
			q.Rejected = append(q.Rejected, Rejection{Ref: in.Ref, Reason: reason, At: c.Now, Posted: posted})
			if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
				return result, err
			}
			if err = fail(c, "rejection-recorded"); err != nil {
				return result, err
			}
			if posted {
				ref, e := c.Provider.Post(ctx, c.DestinationConfig, "not recorded: "+reason+"; reply with your answer and your code", q.Thread)
				if e != nil {
					q.Undelivered++
					result.Undelivered++
					if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
						return result, err
					}
				} else {
					if err = fail(c, "rejection-posted"); err != nil {
						return result, err
					}
					q.Rejected[len(q.Rejected)-1].PostRef = &ref
					if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
						return result, err
					}
				}
			}
			result.Dispositions++
			continue
		}
		ulid, e := goal.NewOperationULID()
		if e != nil {
			return result, e
		}
		opid := goal.Opid(ulid, c.Machine, c.Lineage)
		approvalULID := ""
		if q.Kind == "budget-above-norm" && strings.TrimSpace(answer) == q.Wants {
			approvalULID, e = goal.NewOperationULID()
			if e != nil {
				return result, e
			}
		}
		q.Answer = &Answer{Text: answer, UserID: in.UserID, Ref: in.Ref, At: c.Now, Step: step, ULID: ulid, Opid: opid, ApprovalULID: approvalULID, Phase: "matched"}
		q.State = "answered"
		if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
			return result, err
		}
		if err = fail(c, "matched"); err != nil {
			return result, err
		}
		if err = advanceAnswer(ctx, c, &q); err != nil {
			return result, scrubErr(err, c)
		}
		result.Dispositions++
	}
	if allDurable {
		if err = fail(c, "before-cursor"); err != nil {
			return result, err
		}
		if err = writeJSON(curPath, cursorRecord{Provider: c.ProviderName, Cursor: next}); err != nil {
			return result, err
		}
		if err = fail(c, "cursor"); err != nil {
			return result, err
		}
	}
	return result, nil
}

func verifyInbound(c PollConfig, in Inbound, code string, hasCode bool) (int64, string) {
	if strings.TrimSpace(c.HumanUserID) == "" || strings.TrimSpace(c.TOTPSecret) == "" {
		return 0, "unconfigured"
	}
	if in.UserID != c.HumanUserID {
		return 0, "wrong user"
	}
	if !hasCode {
		return 0, "no code"
	}
	if !in.SentAt.IsZero() && c.Now.Sub(in.SentAt) > channelPollInterval+time.Duration(TOTPStep)*time.Second {
		return 0, fmt.Sprintf("code too old: sent %ds before the poll", int64(c.Now.Sub(in.SentAt)/time.Second))
	}
	verificationTime := c.Now
	if !in.SentAt.IsZero() {
		verificationTime = in.SentAt
	}
	step, ok := VerifyTOTP(c.TOTPSecret, code, verificationTime)
	if !ok {
		return 0, "bad code"
	}
	return step, ""
}

func disposeStatusReply(ctx context.Context, c PollConfig, status StatusState, in Inbound) error {
	answer, code, hasCode := SplitTOTP(in.Text)
	step, reason := verifyInbound(c, in, code, hasCode)
	token := "start " + status.GoalID
	if reason == "" && strings.TrimSpace(answer) != token {
		reason = "wrong token"
	}
	if reason == "" {
		row := consumedRow{Step: step, Destination: c.Destination, Provider: c.ProviderName, ThreadID: in.ThreadID, Ref: in.Ref, QID: "status:" + in.ThreadID}
		ok, err := consume(c.RepoRoot, row, c.Now)
		if err != nil {
			return err
		}
		if !ok {
			reason = "replayed code"
		}
	}
	unmatchedPath := filepath.Join(channelRoot(c.RepoRoot), c.Destination, "unmatched.jsonl")
	if reason == "" {
		recorded := governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: c.ProviderName, UserID: in.UserID, MessageRef: in.Ref.ThreadID + "/" + in.Ref.ID, ContextID: in.ThreadID, Step: step}
		proof, err := humanauthority.VerifiedChannelAnswerProof(c.RepoRoot, recorded, c.Now)
		if err != nil {
			return err
		}
		ulid, err := goal.NewOperationULID()
		if err != nil {
			return err
		}
		ep, err := goal.ResolveEndpoint(c.RepoRoot)
		if err != nil {
			return err
		}
		published, err := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: "wido"}, Ulid: ulid, Now: c.Now}, []string{status.GoalID}, nil, &proof)
		if err != nil {
			reason = err.Error()
		} else if published.Outcome != goal.OutcomeConfirmed {
			reason = "goal approval was not confirmed: " + published.Detail
		} else {
			_, err = c.Provider.Post(ctx, c.DestinationConfig, "recorded: "+status.GoalID+" approved for execution", &status.Ref)
			return err
		}
	}
	if !unmatchedAlready(unmatchedPath, in.Ref) {
		if err := appendJSONL(unmatchedPath, in); err != nil {
			return err
		}
	}
	_, err := c.Provider.Post(ctx, c.DestinationConfig, "not recorded: "+reason+"; reply with the token and your code", &status.Ref)
	return err
}

func advanceAnswer(ctx context.Context, c PollConfig, q *Question) error {
	a := q.Answer
	if a == nil {
		return nil
	}
	if a.Phase == "matched" {
		ep, err := goal.ResolveEndpoint(c.RepoRoot)
		if err != nil {
			return err
		}
		published, err := goal.Answer(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage}, Ulid: a.ULID, Now: a.At}, q.Goal, q.ID, a.Text, q.Wants, goal.AnswerProof{Provider: c.ProviderName, User: a.UserID, Ref: a.Ref.ThreadID + "/" + a.Ref.ID, Step: a.Step})
		if err != nil {
			return err
		}
		if published.Outcome != goal.OutcomeConfirmed {
			return fmt.Errorf("goal answer was not confirmed: %s", published.Detail)
		}
		if q.Kind == "budget-above-norm" && strings.TrimSpace(a.Text) == q.Wants {
			recorded := governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: c.ProviderName, UserID: a.UserID, MessageRef: a.Ref.ThreadID + "/" + a.Ref.ID, ContextID: q.ID, Step: a.Step}
			proof, proofErr := humanauthority.VerifiedChannelAnswerProof(c.RepoRoot, recorded, a.At)
			if proofErr != nil {
				return proofErr
			}
			approved, approveErr := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: a.UserID}, Ulid: a.ApprovalULID, Now: a.At}, []string{q.Goal}, q.Budget, &proof)
			if approveErr != nil {
				a.Receipt = approveErr.Error()
			} else if approved.Outcome == goal.OutcomeConfirmed {
				a.Receipt = "recorded: " + q.Goal + " box raised to " + renderProposedBox(*q.Budget)
			} else {
				a.Receipt = approved.Detail
			}
		}
		if err = fail(c, "recorded-commit"); err != nil {
			return err
		}
		a.Phase = "recorded"
		if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
			return err
		}
		if err = fail(c, "recorded"); err != nil {
			return err
		}
	}
	if a.Phase == "recorded" {
		receipt := a.Receipt
		if receipt == "" {
			receipt = "recorded as your word on " + strings.ReplaceAll(q.Goal, "-", " ") + ", ledger operation " + a.Opid
		}
		_, err := c.Provider.Post(ctx, c.DestinationConfig, receipt, q.Thread)
		if err != nil {
			q.Undelivered++
			_ = writeJSON(questionPath(c.RepoRoot, q.ID), q)
			return receiptPostError{err}
		}
		a.Phase = "receipted"
		if err = writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
			return err
		}
		if err = fail(c, "receipted"); err != nil {
			return err
		}
	}
	if a.Phase == "receipted" {
		a.Phase = "closed"
		q.State = "closed"
		if err := writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
			return err
		}
		if err := fail(c, "closed"); err != nil {
			return err
		}
	}
	return nil
}

type receiptPostError struct{ error }

func fail(c PollConfig, phase string) error {
	if c.FailurePoint != nil {
		return c.FailurePoint(phase)
	}
	return nil
}
func scrubErr(err error, c PollConfig) error {
	return fmt.Errorf("%s", Scrub(err.Error(), append(c.DestinationConfig.Secrets, c.TOTPSecret)...))
}
func alreadyRejected(q Question, r MessageRef) bool {
	for _, x := range q.Rejected {
		if x.Ref == r {
			return true
		}
	}
	return false
}
func appendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(v); err == nil {
		err = f.Sync()
	}
	return err
}
func unmatchedAlready(path string, ref MessageRef) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var in Inbound
		if json.Unmarshal(scanner.Bytes(), &in) == nil && in.Ref == ref {
			return true
		}
	}
	return false
}
func consume(repo string, row consumedRow, now time.Time) (bool, error) {
	path := filepath.Join(channelRoot(repo), "totp-consumed.json")
	rows := []consumedRow{}
	if b, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(b, &rows); err != nil {
			return false, err
		}
	}
	min := now.Unix()/TOTPStep - int64(channelPollInterval/time.Second)/TOTPStep - 1
	kept := rows[:0]
	for _, x := range rows {
		if x.Step >= min {
			kept = append(kept, x)
		}
		if x.Step == row.Step {
			if x.Destination == row.Destination && x.Provider == row.Provider && x.ThreadID == row.ThreadID && x.Ref == row.Ref && x.QID == row.QID {
				return true, nil
			}
			return false, nil
		}
	}
	kept = append(kept, row)
	return true, writeJSON(path, kept)
}
