package channel

import (
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type ReceiveRule struct {
	HumanUserID string
	TOTPSecret  string
	StaleAfter  time.Duration
	Now         time.Time
}

// Verify authenticates one inbound message in the order that determines its
// durable outcome. Its returned text has the final code removed and any other
// valid code masked regardless of the outcome.
func Verify(rule ReceiveRule, in Inbound) (outcome string, step *int64, text string) {
	verificationAt := in.SentAt
	if verificationAt.IsZero() {
		verificationAt = rule.Now
	}
	clean, code, present := StripCode(in.Text)
	text = MaskCodes(clean, rule.TOTPSecret, verificationAt)
	switch {
	case rule.HumanUserID == "" || in.UserID != rule.HumanUserID:
		return "wrong-user", nil, text
	case !present:
		return "no-code", nil, text
	case rule.Now.Sub(verificationAt) > rule.StaleAfter:
		return "stale", nil, text
	case rule.TOTPSecret == "":
		return "bad-code", nil, text
	default:
		verifiedStep, ok := VerifyTOTP(rule.TOTPSecret, code, verificationAt)
		if !ok {
			return "bad-code", nil, text
		}
		return "verified", &verifiedStep, text
	}
}

func InboundRecord(rule ReceiveRule, provider, destination, machine string, in Inbound) goal.ChannelInbound {
	outcome, step, text := Verify(rule, in)
	updateID := in.Ref.ID
	if in.UpdateID != 0 {
		updateID = strconv.FormatInt(in.UpdateID, 10)
	}
	var replyTo *string
	if in.Ref.ThreadID != "" {
		value := in.Ref.ThreadID
		replyTo = &value
	}
	return goal.ChannelInbound{
		Provider: provider, Destination: destination, MessageID: in.Ref.ID, UpdateID: updateID,
		ReplyTo: replyTo, UserID: in.UserID, SentAt: in.SentAt.UTC().Format(time.RFC3339),
		Text: text, Step: step, Outcome: outcome, Question: "", Opid: "",
		ReceivedBy: machine, ReceivedAt: rule.Now.UTC().Format(time.RFC3339),
	}
}

func PublishInbound(e goal.Endpoint, machine, lineage string, record goal.ChannelInbound, now time.Time) (goal.PublishResult, goal.ChannelInbound, error) {
	_, opid, err := goal.ChannelOpid(machine, lineage)
	if err != nil {
		return goal.PublishResult{}, goal.ChannelInbound{}, err
	}
	var decided goal.ChannelInbound
	request := goal.ChannelInboundRequest(e, machine, lineage, opid, record, now, &decided)
	result, err := goal.Publish(e, request)
	return result, decided, err
}
