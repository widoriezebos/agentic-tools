package acp

import (
	"encoding/json"
	"fmt"
	"testing"
)

func chunk(session, kind, text string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(
		`{"sessionId":%q,"update":{"sessionUpdate":%q,"content":{"type":"text","text":%q}}}`,
		session, kind, text))
}

// The step-B capture, frames as recorded in plans/acp-wire-probe.md:
// the replay delivered the user chunk (with clientMessageId and
// messageSubIndex meta) and the prior answer "pong" (with timestamp
// meta) BEFORE load completed; the live post-watermark chunk
// carried no meta and the fresh answer. Only the fresh answer may
// become the candidate.
func TestWatermarkDefeatsReplay(t *testing.T) {
	replayUser := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"_meta":{"cognition.ai/clientMessageId":"63bca575-c374-4c72-928a-40c06ff73d46","cognition.ai/messageSubIndex":0,"cognition.ai/timestamp":"2026-08-16T11:14:17.000000+00:00"},"content":{"text":"Reply with exactly the word: pong.","type":"text"},"sessionUpdate":"user_message_chunk"}}`)
	replayAnswer := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"_meta":{"cognition.ai/timestamp":"2026-08-16T11:14:17.984606+00:00"},"content":{"text":"pong","type":"text"},"sessionUpdate":"agent_message_chunk"}}`)
	liveAnswer := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"content":{"text":"pong-two","type":"text"},"sessionUpdate":"agent_message_chunk"}}`)

	assembler := NewAssembler("marvelous-answer")
	if assembler.Consume(replayUser) || assembler.Consume(replayAnswer) {
		t.Fatal("replay before the watermark must not enter the candidate")
	}
	assembler.OpenWindow()
	if !assembler.Consume(liveAnswer) {
		t.Fatal("the live chunk must enter the window")
	}
	candidate, err := assembler.Candidate()
	if err != nil || string(candidate) != "pong-two" {
		t.Fatalf("candidate %q err %v; the stale answer must not win", candidate, err)
	}
}

// Message-ID grouping: a change of non-empty identity is a message
// boundary, so the final identified message wins even with no
// interleaved user chunk.
func TestMessageIDGrouping(t *testing.T) {
	withID := func(id, text string) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(
			`{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","messageId":%q,"content":{"type":"text","text":%q}}}`,
			id, text))
	}
	assembler := NewAssembler("s")
	assembler.OpenWindow()
	assembler.Consume(withID("m1", "first "))
	assembler.Consume(withID("m1", "message"))
	assembler.Consume(withID("m2", "second message"))
	candidate, err := assembler.Candidate()
	if err != nil || string(candidate) != "second message" {
		t.Fatalf("identity boundary lost: %q err %v", candidate, err)
	}
}

func TestAssemblyRules(t *testing.T) {
	assembler := NewAssembler("s")
	assembler.OpenWindow()
	assembler.Consume(chunk("s", "agent_thought_chunk", "thinking..."))
	assembler.Consume(chunk("other-session", "agent_message_chunk", "foreign"))
	assembler.Consume(chunk("s", "agent_message_chunk", "hello "))
	assembler.Consume(chunk("s", "agent_message_chunk", "world"))
	candidate, err := assembler.Candidate()
	if err != nil || string(candidate) != "hello world" {
		t.Fatalf("arrival-order assembly broke: %q err %v", candidate, err)
	}

	assembler = NewAssembler("s")
	assembler.OpenWindow()
	assembler.Consume(json.RawMessage(`{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"image","text":"ZZZ"}}}`))
	candidate, _ = assembler.Candidate()
	if candidate != nil {
		t.Fatalf("non-text block entered the candidate: %q", candidate)
	}

	assembler = NewAssembler("s")
	assembler.OpenWindow()
	assembler.Consume(chunk("s", "agent_message_chunk", "draft answer"))
	assembler.Consume(chunk("s", "user_message_chunk", "interleaved"))
	assembler.Consume(chunk("s", "agent_message_chunk", "final answer"))
	candidate, err = assembler.Candidate()
	if err != nil || string(candidate) != "final answer" {
		t.Fatalf("final-message-wins broke: %q err %v", candidate, err)
	}

	assembler = NewAssembler("s")
	assembler.OpenWindow()
	if candidate, err = assembler.Candidate(); err != nil || candidate != nil {
		t.Fatalf("empty window must yield no candidate: %q err %v", candidate, err)
	}
}

func TestOversizeDisqualifies(t *testing.T) {
	assembler := NewAssembler("s")
	assembler.OpenWindow()
	big := make([]byte, maxCandidateBytes/2+1)
	for i := range big {
		big[i] = 'x'
	}
	assembler.Consume(chunk("s", "agent_message_chunk", string(big)))
	assembler.Consume(chunk("s", "agent_message_chunk", string(big)))
	if _, err := assembler.Candidate(); err != ErrCandidateTooLarge {
		t.Fatalf("oversize must disqualify the whole window, got %v", err)
	}
}

// Malformed assembler inputs are dropped, never fatal: the
// connection journals them and the window survives.
func TestAssemblerMalformedInputs(t *testing.T) {
	assembler := NewAssembler("s")
	assembler.OpenWindow()
	if assembler.Consume(json.RawMessage(`not json`)) {
		t.Fatal("garbage params must not consume")
	}
	if assembler.Consume(json.RawMessage(`{"sessionId":"s","update":"not-an-object"}`)) {
		t.Fatal("non-object update must not consume")
	}
	if assembler.Consume(chunk("s", "totally_unknown_kind", "x")) {
		t.Fatal("unknown kinds never assemble")
	}
	assembler.Consume(chunk("s", "agent_message_chunk", "ok"))
	candidate, err := assembler.Candidate()
	if err != nil || string(candidate) != "ok" {
		t.Fatalf("window must survive malformed inputs: %q %v", candidate, err)
	}
}
