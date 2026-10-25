package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

type PollConfig struct {
	RepoRoot, Destination, ProviderName, HumanUserID, TOTPSecret, Machine, Lineage string
	Provider                                                                       Provider
	DestinationConfig                                                              DestinationConfig
	Now                                                                            time.Time
	MaxDispositions                                                                int
	FailurePoint                                                                   func(string) error
	// RunAct runs a chosen option's act in the repository; nil runs the engine.
	RunAct func(ctx context.Context, dir string, argv []string) ([]byte, error)
	// answerCodeOff is channel.human.answer-code=off, read once per poll.
	answerCodeOff bool
	// validateInbox replaces the inbox commit validation in tests.
	validateInbox func(goal.Endpoint, string) error
}

type ReplyPublicationError struct {
	Question string
	Err      error
}

func (e *ReplyPublicationError) Error() string { return fmt.Sprintf("%s: %v", e.Question, e.Err) }
func (e *ReplyPublicationError) Unwrap() error { return e.Err }

type PollResult struct {
	Busy                                bool
	Received, Dispositions, Undelivered int
}

const channelPollInterval = 2 * time.Minute

type pollEndpointResolver func(string) (goal.Endpoint, error)

func Poll(ctx context.Context, c PollConfig) (PollResult, error) {
	return pollWithEndpoint(ctx, c, project.PublicationEndpoint)
}

// SettleReplies matches committed replies and finishes answers without receiving
// messages, delivering questions or applying status commands. No session is needed.
func SettleReplies(ctx context.Context, c PollConfig, resolveEndpoint func(string) (goal.Endpoint, error)) (PollResult, error) {
	if resolveEndpoint == nil {
		resolveEndpoint = project.PublicationEndpoint
	}
	return runPoll(ctx, c, resolveEndpoint, false)
}

func pollWithEndpoint(ctx context.Context, c PollConfig, resolveEndpoint pollEndpointResolver) (PollResult, error) {
	return runPoll(ctx, c, resolveEndpoint, true)
}

func runPoll(ctx context.Context, c PollConfig, resolveEndpoint pollEndpointResolver, receive bool) (PollResult, error) {
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
	held, err := lock.File(filepath.Join(channelRoot(c.RepoRoot), "lock"), 0o644, lock.TryExclusive)
	if err != nil {
		if lock.Busy(err) {
			return PollResult{Busy: true}, nil
		}
		return result, err
	}
	defer held.Release()
	if receive {
		if c.answerCodeOff, err = AnswerCodeOff(c.RepoRoot); err != nil {
			return result, err
		}
	}
	questions, err := listQuestions(c.RepoRoot)
	if err != nil {
		return result, err
	}
	for i := range questions {
		if result.Dispositions >= c.MaxDispositions {
			break
		}
		q := &questions[i]
		if receive && q.State == "open" && q.Thread == nil {
			if err = ensureText(c.RepoRoot, q); err != nil {
				return result, err
			}
			ref, postErr := c.Provider.Post(ctx, c.DestinationConfig, q.Text, nil)
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
	var publicationErrors error
	for i := range questions {
		if result.Dispositions >= c.MaxDispositions {
			break
		}
		q := &questions[i]
		if unfinishedAnswer(*q) {
			if err = advanceAnswerWithEndpoint(ctx, c, q, resolveEndpoint); err != nil {
				publicationErrors = errors.Join(publicationErrors, &ReplyPublicationError{Question: strings.Join(q.Facts, " "), Err: scrubErr(err, c)})
				continue
			}
			result.Dispositions++
		}
	}
	status := StatusState{}
	if receive {
		status = LoadStatusState(c.RepoRoot)
	}
	statusRoot := status.Ref.ThreadID
	if statusRoot == "" {
		statusRoot = status.Ref.ID
	}
	threads := []MessageRef{}
	if status.GoalID != "" && statusRoot != "" {
		threads = append(threads, MessageRef{ID: status.Ref.ID, ThreadID: statusRoot})
	}
	for _, q := range questions {
		if (q.State == "open" || recentlyActed(q, c.Now)) && q.Thread != nil {
			threads = append(threads, MessageRef{ID: q.Thread.ID, ThreadID: threadRoot(*q.Thread)})
		}
	}
	var ep *goal.Endpoint
	endpoint := func() (goal.Endpoint, error) {
		if ep == nil {
			resolved, err := resolveEndpoint(c.RepoRoot)
			if err != nil {
				return goal.Endpoint{}, err
			}
			ep = &resolved
		}
		return *ep, nil
	}
	var receiveErr error
	if receive && c.receiveConfigured() {
		receiveErr = receiveToInbox(ctx, c, threads, endpoint, &result)
	}
	err = matchInbox(ctx, c, status, statusRoot, endpoint, resolveEndpoint, &result)
	return result, errors.Join(publicationErrors, receiveErr, err)
}

// disposeStatusReplyWithEndpoint answers a verified inbox record that replies
// to the status post: its sender, code and replay were checked when it was
// committed, so only the start token is read here.
func disposeStatusReplyWithEndpoint(ctx context.Context, c PollConfig, status StatusState, in *goal.ChannelInbound, resolveEndpoint pollEndpointResolver) error {
	reason := ""
	if strings.TrimSpace(in.Text) != "start "+status.GoalID {
		reason = "wrong token"
	}
	if reason == "" {
		replyTo := ""
		if in.ReplyTo != nil {
			replyTo = *in.ReplyTo
		}
		recorded := governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: c.ProviderName, UserID: in.UserID, MessageRef: replyTo + "/" + in.MessageID, ContextID: replyTo, Step: *in.Step}
		proof, err := humanauthority.VerifiedChannelAnswerProof(c.RepoRoot, recorded, c.Now)
		if err != nil {
			return err
		}
		ulid, err := goal.NewOperationULID()
		if err != nil {
			return err
		}
		ep, err := resolveEndpoint(c.RepoRoot)
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
	ask := "; reply with the token and your code"
	if c.answerCodeOff {
		ask = "; reply with the token"
	}
	_, err := c.Provider.Post(ctx, c.DestinationConfig, "not recorded: "+reason+ask, &status.Ref)
	return err
}

func advanceAnswerWithEndpoint(ctx context.Context, c PollConfig, q *Question, resolveEndpoint pollEndpointResolver) error {
	if q.ProcessAct != "" || q.Recovery != nil {
		return nil
	}
	a := q.Answer
	if a == nil {
		return nil
	}
	if a.Machine != "" {
		c.Machine, c.Lineage = a.Machine, a.Lineage
	}
	if a.Provider != "" {
		c.ProviderName = a.Provider
	}
	if a.Phase == "matched" && q.Goal == "" {
		// A question without a goal has no goal file: its answer is recorded
		// on the question record alone.
		if err := fail(c, "recorded-commit"); err != nil {
			return err
		}
		a.Phase = "recorded"
		if err := writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
			return err
		}
		if err := fail(c, "recorded"); err != nil {
			return err
		}
	}
	if a.Phase == "matched" {
		ep, err := resolveEndpoint(c.RepoRoot)
		if err != nil {
			return err
		}
		wants := q.Wants
		if q.Kind == "carry" {
			wants = ""
		}
		published, err := goal.Answer(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage}, Ulid: a.ULID, Now: a.At}, q.Goal, q.ID, a.Text, wants, goal.AnswerProof{Provider: c.ProviderName, User: a.UserID, Ref: a.Ref.ThreadID + "/" + a.Ref.ID, Step: a.Step, OperationID: a.Opid})
		if err != nil {
			return err
		}
		if published.Outcome != goal.OutcomeConfirmed {
			return goal.Coded(published.Code, fmt.Errorf("goal answer was not confirmed: %s", published.Detail))
		}
		if q.Kind == "budget-above-norm" && strings.TrimSpace(a.Text) == q.Wants {
			if q.Budget == nil {
				a.Receipt = "recorded: " + q.Goal + " has no proposed box on this question; nothing raised"
			} else {
				recorded := governance.RecordedChannelAuthority{Outcome: governance.AuthorityOutcomeVerifiedChannelAnswer, Provider: c.ProviderName, UserID: a.UserID, MessageRef: a.Ref.ThreadID + "/" + a.Ref.ID, ContextID: q.ID, Step: a.Step}
				proof, proofErr := humanauthority.VerifiedChannelAnswerProof(c.RepoRoot, recorded, a.At)
				if proofErr != nil {
					return proofErr
				}
				approved, approveErr := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: a.UserID}, Ulid: a.ApprovalULID, Now: a.At}, []string{q.Goal}, q.Budget, &proof)
				if approveErr != nil {
					a.Receipt = approveErr.Error()
				} else if approved.Outcome == goal.OutcomeConfirmed {
					a.Receipt = "recorded: " + q.Goal + " box raised to " + ProposedBox(*q.Budget)
				} else {
					a.Receipt = approved.Detail
				}
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
	// Decision 7: the answer receipt is no longer posted; a recorded answer,
	// or a receipted one from an older engine, closes locally.
	if a.Phase == "recorded" || a.Phase == "receipted" {
		if q.Contract {
			if c.Provider == nil || a.Phase != "recorded" {
				return nil
			}
			return settleChoice(ctx, c, q)
		}
		if q.UnitStop != nil || LaneStopCommand(*q) != "" {
			return nil
		}
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

func fail(c PollConfig, phase string) error {
	if c.FailurePoint != nil {
		return c.FailurePoint(phase)
	}
	return nil
}
func scrubErr(err error, c PollConfig) error {
	if capture, ok := err.(goal.PublicationCaptureFailure); ok {
		return goal.PublicationCaptureFailure{Cause: fmt.Errorf("%s", Scrub(capture.Cause.Error(), append(c.DestinationConfig.Secrets, c.TOTPSecret)...)), Local: capture.Local}
	}
	return goal.Coded(goal.RefusalCode(err), fmt.Errorf("%s", Scrub(err.Error(), append(c.DestinationConfig.Secrets, c.TOTPSecret)...)))
}

// settleChoice performs the option the person chose and tells him, in the
// question's thread, what came of it; only the act's own read of its record
// says whether it started.
func settleChoice(ctx context.Context, c PollConfig, q *Question) error {
	answer := *q.Answer
	option := chosenOption(*q, answer.Text)
	if option == nil {
		option = &Option{}
	}
	label := option.Label
	attempt := Attempt{At: c.Now, Label: label, Outcome: "chosen", Message: goal.ChannelInboxPath(c.Destination, &goal.ChannelInbound{Provider: answer.Provider, MessageID: answer.Ref.ID})}
	line := label + " it is. Nothing runs until you choose again; this question stays open."
	if len(option.Act) > 0 {
		run := c.RunAct
		if run == nil {
			run = engineActRunner(exec.CommandContext)
		}
		out, _ := run(ctx, c.RepoRoot, append(slices.Clone(option.Act), "--answer", q.ID, "--json"))
		var envelope struct {
			Summary string `json:"summary"`
			Data    struct {
				Answer struct{ State, Hold string } `json:"answer"`
			} `json:"data"`
		}
		_ = json.Unmarshal(out, &envelope)
		if fresh, err := ReadQuestion(c.RepoRoot, q.ID); err == nil && fresh.Answer != nil {
			*q = fresh
		}
		_, undo, _ := strings.Cut(option.Consequence, "Undo: ")
		attempt.Outcome = envelope.Data.Answer.State
		switch attempt.Outcome {
		case "started":
			attempt.Outcome = "acted"
			line = fmt.Sprintf("%s it is. The builder's next round of \"%s\" started at %s. If it stops again, I ask here. Undo: %s.", label, QuestionSubject(*q), c.Now.Local().Format("15:04"), strings.TrimSuffix(undo, "."))
		case "accepted":
			line = fmt.Sprintf("%s it is; it waits: %s. When that is fixed, reply %s again.", label, plainReason(envelope.Data.Answer.Hold, *q), label)
		default:
			attempt.Outcome = "refused"
			line = fmt.Sprintf("I could not %s: %s. The question above is still open; reply %s again once that is fixed.", strings.ToLower(label), plainReason(envelope.Summary, *q), label)
		}
	}
	q.Answer.Phase = attempt.Outcome
	if attempt.Outcome == "chosen" || attempt.Outcome == "refused" {
		q.Answer = nil
	}
	q.Attempts = append(q.Attempts, attempt)
	if q.Thread != nil {
		if err := queueNotice(noticesPath(c), "answer:"+q.ID+":"+answer.ULID, line, &q.Thread.ID, threadRoot(*q.Thread)); err != nil {
			return err
		}
	}
	if err := writeJSON(questionPath(c.RepoRoot, q.ID), q); err != nil {
		return err
	}
	return fail(c, "settled")
}

// plainReason is a refusal or hold as a person reads it; words he cannot
// read send him to the goal's status instead.
func plainReason(reason string, q Question) string {
	reason = strings.TrimSuffix(strings.TrimSpace(reason), ".")
	if reason == "" || unplainWords.MatchString(reason) {
		return "the command refused it; metasystem status " + q.Goal + " at your terminal says why"
	}
	return reason
}

// engineActRunner runs an act's argv as this engine in the repository,
// outside the channel duty's deadline, and returns what it printed.
func engineActRunner(command func(context.Context, string, ...string) *exec.Cmd) func(context.Context, string, []string) ([]byte, error) {
	return func(ctx context.Context, dir string, argv []string) ([]byte, error) {
		engine, err := os.Executable()
		if err != nil {
			return nil, err
		}
		act := command(context.WithoutCancel(ctx), engine, argv[1:]...)
		act.Dir = dir
		return act.Output()
	}
}

// Recorded replies to action requests are complete even while the act is pending.
func unfinishedAnswer(q Question) bool {
	if q.Contract && q.Answer != nil {
		return q.Answer.Phase == "matched" || q.Answer.Phase == "recorded"
	}
	return q.Answer != nil && q.Answer.Phase != "closed" &&
		!((q.Answer.Phase == "recorded" || q.Answer.Phase == "receipted") && (q.UnitStop != nil || LaneStopCommand(q) != "")) &&
		q.ProcessAct == "" && q.Recovery == nil
}
