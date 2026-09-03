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
		reason := ""
		step := int64(0)
		if strings.TrimSpace(c.HumanUserID) == "" || strings.TrimSpace(c.TOTPSecret) == "" {
			reason = "unconfigured"
		} else if in.UserID != c.HumanUserID {
			reason = "wrong user"
		} else if !hasCode {
			reason = "no code"
		} else if step, hasCode = VerifyTOTP(c.TOTPSecret, code, c.Now); !hasCode {
			reason = "bad code"
		}
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
		q.Answer = &Answer{Text: answer, UserID: in.UserID, Ref: in.Ref, At: c.Now, Step: step, ULID: ulid, Opid: opid, Phase: "matched"}
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
		_, err := c.Provider.Post(ctx, c.DestinationConfig, "recorded as your word on "+strings.ReplaceAll(q.Goal, "-", " ")+", ledger operation "+a.Opid, q.Thread)
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
	min := now.Unix()/TOTPStep - 1
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
