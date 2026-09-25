// Package fakeacp is an ACP server that answers from a script, over a pipe.
//
// It exists so that nothing has to run a real agent to prove the seam: the
// package's own tests, the interface server's tests and the walkthrough all
// drive the host through this, and a real runtime is only ever run by hand.
// It speaks the same wire the adapters speak — initialize, session/new, the
// model config option, session/prompt with streamed updates, one permission
// request, session/cancel — and nothing else, which is exactly the surface
// the host uses.
package fakeacp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// Script is what this server will do.
type Script struct {
	// Protocol is the version initialize answers with; zero means 1.
	Protocol int64
	// InitError, when set, refuses initialize with this message — a runtime
	// that is not signed in, as the wire carries it.
	InitError string
	// Models are the model ids the session offers. Nil offers no model config
	// option at all, which is how a runtime that cannot select one behaves.
	Models []string
	// ModelError, when set, refuses session/set_config_option with it.
	ModelError string
	// Chunks are the answer, in the pieces it streams in.
	Chunks []string
	// Activity, when set, is a tool call the Partner makes on the way, which
	// the drawer shows as a muted line.
	Activity string
	// Permission, when set, is one permission request the client will refuse,
	// with this tool title and kind.
	Permission     string
	PermissionKind string
	// Reads are the tool calls this server makes and completes, in order. They
	// are what a Partner with read tools does, on the wire: a tool_call, then a
	// tool_call_update carrying the result and the status it ended in.
	Reads []Read
	// AskFor, when set, is one permission request for a named tool — the shape
	// an application tool has, which carries no path and no read kind — so the
	// permission point's named exception can be driven end to end.
	AskFor string
	// Pause is how long the server waits between chunks, so a walkthrough can
	// stop a turn and reload a page while one is still streaming.
	Pause time.Duration
	// StopReason is what the prompt settles with; empty is end_turn.
	StopReason string
	// Words is what this server "said" on its error stream.
	Words string
}

// Read is one tool call this server makes: what it is called, what came back,
// and whether it ended as a completion or a failure.
type Read struct {
	// Name is the tool's own name on the wire, where the script sets one:
	// "mcp__metasystem__suggest". A runtime carries it beside the title, and
	// the host reads which operation a call is out of it.
	Name   string
	Title  string
	Result string
	// Failed makes the completion a failure rather than a completion.
	Failed bool
	// When narrows this call to the prompts that carry this text, where the
	// script sets one. A fake that prepares a suggestion for one sheet's field
	// should prepare it when that sheet was handed over rather than on every
	// question a human asks from anywhere.
	When string
}

// Servers is what the client actually sent this server: the tool servers it
// handed over at session/new, and the prompts it sent afterwards.
//
// It is what makes a test of composition possible. A scripted answer proves
// nothing about what the Partner was told — the fake would stream the same
// words with no index, no skill and no tools — so the assertions are made
// against this record of the wire instead, where a missing input is a missing
// substring rather than an answer that still passes.
type Servers struct {
	mu      sync.Mutex
	named   []string
	prompts []string
}

// Named is every tool server the client handed over, by name.
func (s *Servers) Named() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.named...)
}

// Prompts is every prompt the client sent, whole, in order.
func (s *Servers) Prompts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.prompts...)
}

// First is the first prompt of the session, and whether there was one. A test
// that asserted against an empty string would pass for a session that was
// never prompted at all.
func (s *Servers) First() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.prompts) == 0 {
		return "", false
	}
	return s.prompts[0], true
}

func (s *Servers) add(name string) {
	s.mu.Lock()
	s.named = append(s.named, name)
	s.mu.Unlock()
}

func (s *Servers) asked(prompt string) {
	s.mu.Lock()
	s.prompts = append(s.prompts, prompt)
	s.mu.Unlock()
}

// Open answers an endpoint opener the host can be built on.
func Open(script Script) func(context.Context) (partner.Endpoint, error) {
	opener, _ := OpenWatched(script)
	return opener
}

// OpenWatched is the same opener with the tool servers the client handed over,
// which is the one thing about session/new a test cannot see from the outside.
func OpenWatched(script Script) (func(context.Context) (partner.Endpoint, error), *Servers) {
	handed := &Servers{}
	return func(context.Context) (partner.Endpoint, error) {
		clientReads, serverWrites := io.Pipe()
		serverReads, clientWrites := io.Pipe()
		server := &server{
			script: script, out: serverWrites, handed: handed,
			permission: make(chan json.RawMessage, 1),
			wake:       make(chan struct{}, 1),
		}
		go func() {
			server.serve(serverReads)
			_ = serverWrites.Close()
		}()
		return partner.Endpoint{
			Reader: clientReads,
			Writer: clientWrites,
			Close: func() {
				server.stopped.Store(true)
				server.tripped()
				_ = clientWrites.Close()
				_ = serverReads.Close()
				_ = serverWrites.Close()
				_ = clientReads.Close()
			},
			Words: func() string { return script.Words },
		}, nil
	}, handed
}

// SessionID is the session every fake server opens, so a test can name it.
const SessionID = "fake-session"

type server struct {
	script  Script
	out     io.Writer
	handed  *Servers
	writing sync.Mutex

	cancelled atomic.Bool
	stopped   atomic.Bool
	// wake carries at most one signal and is tripped by a cancellation or a
	// teardown, so a pause between chunks ends the moment either happens.
	wake chan struct{}
	// answered counts permission requests the client has answered, so the
	// prompt waits for the answer before it goes on — which is what a real
	// server does and what makes the refusal visible in order.
	permission chan json.RawMessage
}

type frame struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *wireError      `json:"error,omitempty"`
}

type wireError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

func (s *server) serve(reads io.Reader) {
	scanner := bufio.NewScanner(reads)
	scanner.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var in frame
		if err := json.Unmarshal(line, &in); err != nil {
			continue
		}
		if in.Method == "" && in.ID != nil {
			// A response to our own permission request.
			select {
			case s.permission <- in.Result:
			default:
			}
			continue
		}
		s.dispatch(in)
		if s.stopped.Load() {
			return
		}
	}
}

func (s *server) dispatch(in frame) {
	switch in.Method {
	case "initialize":
		if s.script.InitError != "" {
			s.fail(in.ID, s.script.InitError)
			return
		}
		version := s.script.Protocol
		if version == 0 {
			version = 1
		}
		s.answer(in.ID, map[string]any{
			"protocolVersion":   version,
			"agentCapabilities": map[string]any{"loadSession": false},
			"authMethods":       []any{},
		})
	case "session/new":
		s.remember(in.Params)
		result := map[string]any{"sessionId": SessionID}
		if s.script.Models != nil {
			options := make([]map[string]any, 0, len(s.script.Models))
			for _, model := range s.script.Models {
				options = append(options, map[string]any{"value": model, "name": model})
			}
			result["configOptions"] = []map[string]any{{
				"id": "model", "name": "Model", "type": "select",
				"currentValue": s.script.Models[0], "options": options,
			}}
		}
		s.answer(in.ID, result)
	case "session/set_config_option":
		if s.script.ModelError != "" {
			s.fail(in.ID, s.script.ModelError)
			return
		}
		s.answer(in.ID, map[string]any{})
	case "session/cancel":
		s.cancelled.Store(true)
		s.tripped()
	case "session/prompt":
		s.heard(in.Params)
		go s.prompt(in.ID, promptText(in.Params))
	default:
		s.fail(in.ID, "the fake server does not answer "+in.Method)
	}
}

// prompt runs the script: the activity line, the refused permission request,
// then the answer in its chunks, then the settled response.
func (s *server) prompt(id json.RawMessage, asked string) {
	s.cancelled.Store(false)
	if s.script.Activity != "" {
		s.notify("session/update", map[string]any{
			"sessionId": SessionID,
			"update": map[string]any{
				"sessionUpdate": "tool_call", "toolCallId": "call-1",
				"title": s.script.Activity, "kind": "read", "status": "pending",
			},
		})
	}
	for at, read := range s.script.Reads {
		if read.When != "" && !strings.Contains(asked, read.When) {
			continue
		}
		id := "read-" + strconv.Itoa(at)
		started := map[string]any{
			"sessionUpdate": "tool_call", "toolCallId": id,
			"title": read.Title, "kind": "other", "status": "pending",
		}
		if read.Name != "" {
			started["name"] = read.Name
			started["_meta"] = map[string]any{"claudeCode": map[string]any{"toolName": read.Name}}
		}
		s.notify("session/update", map[string]any{"sessionId": SessionID, "update": started})
		status := "completed"
		if read.Failed {
			status = "failed"
		}
		s.notify("session/update", map[string]any{
			"sessionId": SessionID,
			"update": map[string]any{
				"sessionUpdate": "tool_call_update", "toolCallId": id, "status": status,
				"content": []map[string]any{{
					"type":    "content",
					"content": map[string]any{"type": "text", "text": read.Result},
				}},
			},
		})
		if s.script.Pause > 0 {
			s.sleep(s.script.Pause)
		}
	}
	if s.script.AskFor != "" {
		s.request(map[string]any{
			"sessionId": SessionID,
			"toolCall": map[string]any{
				"toolCallId": "call-tool", "title": s.script.AskFor, "kind": "other",
				"name":  s.script.AskFor,
				"_meta": map[string]any{"claudeCode": map[string]any{"toolName": s.script.AskFor}},
			},
			"options": []map[string]any{
				{"optionId": "yes", "kind": "allow_once", "name": "Allow"},
				{"optionId": "no", "kind": "reject_once", "name": "Refuse"},
			},
		})
		select {
		case <-s.permission:
		case <-time.After(10 * time.Second):
		}
	}
	if s.script.Permission != "" {
		kind := s.script.PermissionKind
		if kind == "" {
			kind = "edit"
		}
		s.request(map[string]any{
			"sessionId": SessionID,
			"toolCall": map[string]any{
				"toolCallId": "call-2", "title": s.script.Permission, "kind": kind,
			},
			"options": []map[string]any{
				{"optionId": "yes", "kind": "allow_once", "name": "Allow"},
				{"optionId": "no", "kind": "reject_once", "name": "Refuse"},
			},
		})
		select {
		case <-s.permission:
		case <-time.After(10 * time.Second):
		}
	}
	for _, chunk := range s.script.Chunks {
		if s.cancelled.Load() || s.stopped.Load() {
			s.answer(id, map[string]any{"stopReason": "cancelled"})
			return
		}
		s.notify("session/update", map[string]any{
			"sessionId": SessionID,
			"update": map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": chunk},
			},
		})
		if s.script.Pause > 0 {
			s.sleep(s.script.Pause)
		}
	}
	if s.cancelled.Load() {
		s.answer(id, map[string]any{"stopReason": "cancelled"})
		return
	}
	reason := s.script.StopReason
	if reason == "" {
		reason = "end_turn"
	}
	s.answer(id, map[string]any{"stopReason": reason})
}

// sleep waits, but wakes as soon as the turn is cancelled, so a Stop lands
// within a frame rather than within a pause.
func (s *server) sleep(pause time.Duration) {
	timer := time.NewTimer(pause)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-s.wake:
	}
}

// tripped wakes a pause without ever blocking the caller.
func (s *server) tripped() {
	if s.wake == nil {
		return
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// promptText is the prompt the client sent, whole, joining the text blocks the
// protocol carries it in. It is read for every prompt, because a script may
// narrow a call to the prompts that carry something.
func promptText(params json.RawMessage) string {
	var sent struct {
		Prompt []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(params, &sent); err != nil {
		return ""
	}
	parts := make([]string, 0, len(sent.Prompt))
	for _, block := range sent.Prompt {
		parts = append(parts, block.Text)
	}
	return strings.Join(parts, "")
}

// heard records the prompt the client sent, whole, joining the text blocks the
// protocol carries it in.
func (s *server) heard(params json.RawMessage) {
	if s.handed == nil {
		return
	}
	var sent struct {
		Prompt []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(params, &sent); err != nil {
		return
	}
	parts := make([]string, 0, len(sent.Prompt))
	for _, block := range sent.Prompt {
		parts = append(parts, block.Text)
	}
	s.handed.asked(strings.Join(parts, ""))
}

// remember records the tool servers session/new handed over.
func (s *server) remember(params json.RawMessage) {
	if s.handed == nil {
		return
	}
	var opened struct {
		MCPServers []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(params, &opened); err != nil {
		return
	}
	for _, server := range opened.MCPServers {
		s.handed.add(server.Name)
	}
}

func (s *server) answer(id json.RawMessage, result any) {
	s.write(frame{ID: id, Result: mustJSON(result)})
}

func (s *server) fail(id json.RawMessage, message string) {
	s.write(frame{ID: id, Error: &wireError{Code: -32000, Message: message}})
}

func (s *server) notify(method string, params any) {
	s.write(frame{Method: method, Params: mustJSON(params)})
}

// requestID is the id this server's own requests carry; one is enough,
// because the script makes one request per turn.
const requestID = `"fake-request"`

func (s *server) request(params any) {
	s.write(frame{ID: json.RawMessage(requestID), Method: "session/request_permission", Params: mustJSON(params)})
}

func (s *server) write(out frame) {
	body, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		frame
	}{JSONRPC: "2.0", frame: out})
	if err != nil {
		return
	}
	s.writing.Lock()
	defer s.writing.Unlock()
	_, _ = s.out.Write(append(body, '\n'))
}

func mustJSON(value any) json.RawMessage {
	body, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return body
}
