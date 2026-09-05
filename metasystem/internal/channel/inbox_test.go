package channel

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

const inboxTestSecret = "JBSWY3DPEHPK3PXP"

func inboundWithCode(t *testing.T, at time.Time, text string) Inbound {
	t.Helper()
	code, err := TOTPCode(inboxTestSecret, at)
	if err != nil {
		t.Fatal(err)
	}
	return Inbound{
		Ref: MessageRef{ID: "42", ThreadID: "question-post"}, UserID: "human",
		Text: strings.ReplaceAll(text, "{code}", code), SentAt: at, Ack: "43", UpdateID: 100,
	}
}

func TestVerifyOrderAndStep(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 34, 56, 0, time.UTC)
	rule := ReceiveRule{HumanUserID: "human", TOTPSecret: inboxTestSecret, StaleAfter: time.Hour, Now: now}
	tests := []struct {
		name       string
		mutateRule func(*ReceiveRule)
		in         Inbound
		want       string
		wantStep   bool
	}{
		{"wrong user precedes missing code", nil, Inbound{UserID: "other", Text: "answer", SentAt: now.Add(-2 * time.Hour)}, "wrong-user", false},
		{"empty configured user is wrong user", func(r *ReceiveRule) { r.HumanUserID = "" }, inboundWithCode(t, now, "answer {code}"), "wrong-user", false},
		{"missing code precedes stale", nil, Inbound{UserID: "human", Text: "answer", SentAt: now.Add(-2 * time.Hour)}, "no-code", false},
		{"stale precedes bad code", nil, Inbound{UserID: "human", Text: "answer 000000", SentAt: now.Add(-2 * time.Hour)}, "stale", false},
		{"bad code", nil, Inbound{UserID: "human", Text: "answer 000000", SentAt: now}, "bad-code", false},
		{"empty secret is bad code", func(r *ReceiveRule) { r.TOTPSecret = "" }, inboundWithCode(t, now, "answer {code}"), "bad-code", false},
		{"verified", nil, inboundWithCode(t, now, "answer {code}"), "verified", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRule := rule
			if test.mutateRule != nil {
				test.mutateRule(&gotRule)
			}
			outcome, step, _ := Verify(gotRule, test.in)
			if outcome != test.want || (step != nil) != test.wantStep {
				t.Fatalf("Verify outcome=%q step=%v, want %q step-present=%v", outcome, step, test.want, test.wantStep)
			}
			if step != nil && *step != now.Unix()/TOTPStep {
				t.Fatalf("verified step=%d, want %d", *step, now.Unix()/TOTPStep)
			}
		})
	}
}

func TestVerifyMasksTextForEveryOutcome(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 34, 56, 0, time.UTC)
	code, err := TOTPCode(inboxTestSecret, now)
	if err != nil {
		t.Fatal(err)
	}
	rule := ReceiveRule{HumanUserID: "human", TOTPSecret: inboxTestSecret, StaleAfter: time.Hour, Now: now}
	for _, test := range []struct {
		name string
		in   Inbound
		want string
	}{
		{"wrong user masks a middle code", Inbound{UserID: "other", Text: "before " + code + " after 000000", SentAt: now}, "before [code] after"},
		{"six digit fact remains", Inbound{UserID: "human", Text: "order 654321 now", SentAt: now}, "order 654321 now"},
		{"only code is empty", Inbound{UserID: "human", Text: code, SentAt: now}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, text := Verify(rule, test.in)
			if text != test.want {
				t.Fatalf("masked text=%q, want %q", text, test.want)
			}
		})
	}
}

func TestVerifyUsesNowForZeroSentAt(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 34, 56, 0, time.UTC)
	in := inboundWithCode(t, now, "answer {code}")
	in.SentAt = time.Time{}
	outcome, step, text := Verify(ReceiveRule{HumanUserID: "human", TOTPSecret: inboxTestSecret, StaleAfter: time.Minute, Now: now}, in)
	if outcome != "verified" || step == nil || text != "answer" {
		t.Fatalf("zero sentAt must verify at Now: outcome=%q step=%v text=%q", outcome, step, text)
	}
}

func TestInboundRecordMapsProviderFields(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 34, 56, 987000000, time.UTC)
	sent := now.Add(-time.Minute)
	rule := ReceiveRule{HumanUserID: "human", TOTPSecret: inboxTestSecret, StaleAfter: time.Hour, Now: now}
	in := inboundWithCode(t, sent, "answer {code}")
	record := InboundRecord(rule, "telegram", "team", "mac-a", in)
	if record.Provider != "telegram" || record.Destination != "team" || record.MessageID != "42" || record.UpdateID != "100" ||
		record.ReplyTo == nil || *record.ReplyTo != "question-post" || record.UserID != "human" || record.Text != "answer" ||
		record.Outcome != "verified" || record.Step == nil || record.Question != "" || record.Opid != "" || record.ReceivedBy != "mac-a" ||
		record.SentAt != "2026-09-04T12:33:56Z" || record.ReceivedAt != "2026-09-04T12:34:56Z" {
		t.Fatalf("mapped record is wrong: %+v", record)
	}

	in.Ref.ThreadID = ""
	in.UpdateID = 0
	slack := InboundRecord(rule, "slack", "team", "mac-a", in)
	if slack.ReplyTo != nil || slack.UpdateID != in.Ref.ID {
		t.Fatalf("Slack fallback mapping is wrong: %+v", slack)
	}

	wantStep := sent.Unix() / TOTPStep
	if !reflect.DeepEqual(record.Step, &wantStep) {
		t.Fatalf("step=%v, want %d", record.Step, wantStep)
	}
	if got := fmt.Sprint(record.ReplyTo); got == "<nil>" {
		t.Fatal("threaded reply unexpectedly mapped to null")
	}
}
