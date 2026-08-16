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

// The step-B capture verbatim: load replays the prior answer as a
// live-looking message chunk BEFORE the watermark; only the fresh
// post-watermark answer may become the candidate.
func TestWatermarkDefeatsReplay(t *testing.T) {
	assembler := NewAssembler("marvelous-answer")
	if assembler.Consume(chunk("marvelous-answer", "agent_message_chunk", "pong")) {
		t.Fatal("replay before the watermark must not enter the candidate")
	}
	assembler.OpenWindow()
	if !assembler.Consume(chunk("marvelous-answer", "agent_message_chunk", "pong-two")) {
		t.Fatal("the live chunk must enter the window")
	}
	candidate, err := assembler.Candidate()
	if err != nil || string(candidate) != "pong-two" {
		t.Fatalf("candidate %q err %v; the stale answer must not win", candidate, err)
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
