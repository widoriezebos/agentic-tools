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
// meta) at sequences BEFORE the load response; the live chunk came
// after the prompt was sent. The sequence range — not drain timing —
// keeps the stale answer out (the slice-two linux race proved drain
// timing lies).
func TestWatermarkDefeatsReplay(t *testing.T) {
	replayUser := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"_meta":{"cognition.ai/clientMessageId":"63bca575-c374-4c72-928a-40c06ff73d46","cognition.ai/messageSubIndex":0,"cognition.ai/timestamp":"2026-08-16T11:14:17.000000+00:00"},"content":{"text":"Reply with exactly the word: pong.","type":"text"},"sessionUpdate":"user_message_chunk"}}`)
	replayAnswer := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"_meta":{"cognition.ai/timestamp":"2026-08-16T11:14:17.984606+00:00"},"content":{"text":"pong","type":"text"},"sessionUpdate":"agent_message_chunk"}}`)
	liveAnswer := json.RawMessage(`{"sessionId":"marvelous-answer","update":{"content":{"text":"pong-two","type":"text"},"sessionUpdate":"agent_message_chunk"}}`)

	assembler := NewAssembler("marvelous-answer")
	assembler.Consume(1, replayUser)   // replay, before the load response (seq 3)
	assembler.Consume(2, replayAnswer) // replay
	assembler.Consume(4, liveAnswer)   // live, inside the window (3, 5)
	candidate, err := assembler.Candidate(3, 5)
	if err != nil || string(candidate) != "pong-two" {
		t.Fatalf("candidate %q err %v; the stale answer must not win", candidate, err)
	}
}

// A chunk with a sequence AFTER the PromptResponse never enters —
// the exact linux race: the channel delivered it before the
// response was selected, but the wire order says it came later.
func TestSequenceExcludesLateChunk(t *testing.T) {
	assembler := NewAssembler("s")
	assembler.Consume(4, chunk("s", "agent_message_chunk", "in window"))
	assembler.Consume(6, chunk("s", "agent_message_chunk", "too late"))
	candidate, err := assembler.Candidate(3, 5)
	if err != nil || string(candidate) != "in window" {
		t.Fatalf("late chunk leaked: %q err %v", candidate, err)
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
	assembler.Consume(1, withID("m1", "first "))
	assembler.Consume(2, withID("m1", "message"))
	assembler.Consume(3, withID("m2", "second message"))
	candidate, err := assembler.Candidate(0, 10)
	if err != nil || string(candidate) != "second message" {
		t.Fatalf("identity boundary lost: %q err %v", candidate, err)
	}
}

func TestAssemblyRules(t *testing.T) {
	assembler := NewAssembler("s")
	assembler.Consume(1, chunk("s", "agent_thought_chunk", "thinking..."))
	assembler.Consume(2, chunk("other-session", "agent_message_chunk", "foreign"))
	assembler.Consume(3, chunk("s", "agent_message_chunk", "hello "))
	assembler.Consume(4, chunk("s", "agent_message_chunk", "world"))
	candidate, err := assembler.Candidate(0, 10)
	if err != nil || string(candidate) != "hello world" {
		t.Fatalf("arrival-order assembly broke: %q err %v", candidate, err)
	}

	assembler = NewAssembler("s")
	assembler.Consume(1, json.RawMessage(`{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"image","text":"ZZZ"}}}`))
	if candidate, _ = assembler.Candidate(0, 10); candidate != nil {
		t.Fatalf("non-text block entered the candidate: %q", candidate)
	}

	assembler = NewAssembler("s")
	assembler.Consume(1, chunk("s", "agent_message_chunk", "draft answer"))
	assembler.Consume(2, chunk("s", "user_message_chunk", "interleaved"))
	assembler.Consume(3, chunk("s", "agent_message_chunk", "final answer"))
	candidate, err = assembler.Candidate(0, 10)
	if err != nil || string(candidate) != "final answer" {
		t.Fatalf("final-message-wins broke: %q err %v", candidate, err)
	}

	assembler = NewAssembler("s")
	if candidate, err = assembler.Candidate(0, 10); err != nil || candidate != nil {
		t.Fatalf("empty window must yield no candidate: %q err %v", candidate, err)
	}
}

// Malformed assembler inputs are dropped, never fatal: the
// connection journals them and the window survives.
func TestAssemblerMalformedInputs(t *testing.T) {
	assembler := NewAssembler("s")
	if assembler.Consume(1, json.RawMessage(`not json`)) {
		t.Fatal("garbage params must not consume")
	}
	if assembler.Consume(2, json.RawMessage(`{"sessionId":"s","update":"not-an-object"}`)) {
		t.Fatal("non-object update must not consume")
	}
	if assembler.Consume(3, chunk("s", "totally_unknown_kind", "x")) {
		t.Fatal("unknown kinds never assemble")
	}
	assembler.Consume(4, chunk("s", "agent_message_chunk", "ok"))
	candidate, err := assembler.Candidate(0, 10)
	if err != nil || string(candidate) != "ok" {
		t.Fatalf("window must survive malformed inputs: %q %v", candidate, err)
	}
}

// Oversize disqualifies: at the candidate ceiling within a window,
// and at the buffer cap regardless of window.
func TestOversizeDisqualifies(t *testing.T) {
	assembler := NewAssembler("s")
	big := make([]byte, maxCandidateBytes/2+1)
	for i := range big {
		big[i] = 'x'
	}
	assembler.Consume(1, chunk("s", "agent_message_chunk", string(big)))
	assembler.Consume(2, chunk("s", "agent_message_chunk", string(big)))
	if _, err := assembler.Candidate(0, 10); err != ErrCandidateTooLarge {
		t.Fatalf("oversize must disqualify the whole window, got %v", err)
	}

	assembler = NewAssembler("s")
	for seq := uint64(1); seq <= 5; seq++ {
		assembler.Consume(seq, chunk("s", "agent_message_chunk", string(big)))
	}
	if !assembler.overflow {
		t.Fatal("the buffer cap must mark overflow")
	}
	if _, err := assembler.Candidate(0, 2); err != ErrCandidateTooLarge {
		t.Fatalf("overflow disqualifies every window: %v", err)
	}
}
