package uitools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

// The wire: the Model Context Protocol over stdio, which is the one tool-server
// transport every runtime this seat admits accepts.
//
// It is JSON-RPC 2.0, one message per line, and the surface is four methods:
// initialize, the initialized notification, tools/list and tools/call. Nothing
// here subscribes, samples, or asks the client for anything: the server reads
// the checkout and answers, and a method it does not know is an error rather
// than a guess.
//
// The process this runs in is started by the Partner's own runtime, from the
// hand-off the interface server makes at session/new. It is the engine itself,
// with the checkout it was given, and it is the one exception the permission
// rule names.

// The protocol version this server speaks, and the ones it will answer a
// client with. A client that asks for a version in the list is answered with
// its own, so a newer or older adapter does not have to be taught about this
// server; anything else is answered with the one this server was written for.
const (
	protocolVersion = "2025-06-18"
)

var spokenVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Args is one tool call's arguments, as the client sent them.
type Args map[string]any

// Text is one string argument, or empty.
func (a Args) Text(name string) string {
	switch value := a[name].(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

// Number is one numeric argument, or the fallback. A client that sends it as
// a string is taken at its word: JSON Schema is a description, not a contract
// the model has to have read.
func (a Args) Number(name string, fallback int) int {
	switch value := a[name].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

// Cursor is the continuation a previous result handed back.
func (a Args) Cursor() string { return a.Text("cursor") }

// Tool is one published operation: what it is called, what it is for, and what
// it takes.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// cursorProperty is the one argument nearly every operation shares.
var cursorProperty = map[string]any{
	"type":        "string",
	"description": "Continue a previous result: pass the cursor that result carried. Omit it for the first page.",
}

// Catalogue is what tools/list publishes, in the order Operations names them.
func Catalogue() []Tool {
	return []Tool{
		{
			Name: OpBoard,
			Description: "The backlog as the board reads it: every goal at the accepted ledger tip, lane by lane, " +
				"with its intent, tier, priority, state, seat and what it is waiting on. " +
				"Bounded; a result that leaves rows out says how many and carries a cursor.",
			InputSchema: schema(map[string]any{
				"filters": map[string]any{
					"type":        "string",
					"description": "Narrow to the rows whose line contains this text: a lane, a state, a seat, an arc, a tier, an id or a word of the intent.",
				},
				"cursor": cursorProperty,
			}, nil),
		},
		{
			Name:        OpGoal,
			Description: "One goal at the accepted ledger tip, field by field, with the ids of the goals beside it in its lane.",
			InputSchema: schema(map[string]any{
				"id": map[string]any{"type": "string", "description": "The goal's ledger id."},
			}, []string{"id"}),
		},
		{
			Name: OpDocument,
			Description: "One document of the checkout, as it stands: its record head and its source text. " +
				"Bounded; a document longer than one result carries a cursor into the rest of it.",
			InputSchema: schema(map[string]any{
				"id":     map[string]any{"type": "string", "description": "The document's checkout-relative path, for example plans/designs/user-interface/g1-s29-ask-about-what-you-see.md."},
				"cursor": cursorProperty,
			}, []string{"id"}),
		},
		{
			Name:        OpRecords,
			Description: "The project's records as the checkout declares them: kind, status, id, the goals each names, and its path.",
			InputSchema: schema(map[string]any{
				"kind":   map[string]any{"type": "string", "description": "Only records of this kind: design, decision, doctrine, intent, question."},
				"cursor": cursorProperty,
			}, nil),
		},
		{
			Name:        OpQuestions,
			Description: "The open questions the checkout records, with their status and the goals they name.",
			InputSchema: schema(map[string]any{}, nil),
		},
		{
			Name:        OpOverview,
			Description: "The landing page's own numbers: what needs the human, what the fleet is doing, and the window the page compares against.",
			InputSchema: schema(map[string]any{}, nil),
		},
		{
			Name: OpFleet,
			Description: "The fleet's standings: which machines have published presence, when each was last seen, " +
				"the phase each is in, and which goals are held by a machine that has gone quiet. " +
				"Name a machine for what it is working on in detail: the goal, the job and the cap it " +
				"reserved, the goal's box, and the chain. " +
				"It flags and never acts: a silent holder keeps its claim, and the words name what a human " +
				"does at a terminal. Bounded; a result that leaves rows out carries a cursor.",
			InputSchema: schema(map[string]any{
				"machine": map[string]any{"type": "string", "description": "One machine's work, opened. Empty reads the whole fleet."},
				"cursor":  cursorProperty,
			}, nil),
		},
		{
			Name:        OpNotifications,
			Description: "The steward's journal, newest first: what it said, from where, and whether it was delivered.",
			InputSchema: schema(map[string]any{
				"limit":  map[string]any{"type": "integer", "description": "How many entries to read; the default is the journal's own page."},
				"cursor": cursorProperty,
			}, nil),
		},
		{
			Name:        OpSearch,
			Description: "Every goal, record, document and question whose line contains this text.",
			InputSchema: schema(map[string]any{
				"text":   map[string]any{"type": "string", "description": "What to look for, in any case."},
				"cursor": cursorProperty,
			}, []string{"text"}),
		},
		{
			Name: OpInterface,
			Description: "What this interface is made of, from its own sources: every section with what it is for, " +
				"what its page shows and whether this build projects it; the lanes and which are shown; the help terms; " +
				"the questions this interface suggests; the acts and the hand each needs; the ui. settings with their " +
				"defaults and what this seat resolves them to; where this checkout keeps each kind of record; and the " +
				"runtimes this build admits. Ask it before saying what a page, a term, a setting or a lane is. " +
				"Bounded; name a part and page within it.",
			InputSchema: schema(map[string]any{
				"part": map[string]any{
					"type":        "string",
					"description": "Which part to read: summary, sections, lanes, terms, questions, acts, settings, records or runtimes. Omit it for the summary.",
				},
				"cursor": cursorProperty,
			}, nil),
		},
		{
			Name: OpKit,
			Description: "What the metasystem itself means, from the kit's own owners: the glossary's definition of a " +
				"term, one of the engine's verbs and what it does, a standing human ruling, or the route that says " +
				"where a workflow is written down. Ask it for a concept, a command, a ruling id or a way of working. " +
				"It explains rules; what this workspace's goals actually are comes from the board, goal and search tools.",
			InputSchema: schema(map[string]any{
				"topic": map[string]any{
					"type":        "string",
					"description": "A term, a verb, a ruling id or a way of working. Omit it to see what this tool can answer from.",
				},
				"cursor": cursorProperty,
			}, nil),
		},
		{
			Name: OpSuggest,
			Description: "Offer the human text for one field of the editor they handed over: the editor's name as its " +
				"head says it, the field's name as its label says it, and the field's whole new value. " +
				"Call it when the human asks you to write or improve a field, once per field, beside the answer " +
				"you give in words. It writes nothing and applies nothing: the human sees a card and decides " +
				"whether to use it, and the field's value stays theirs until they do.",
			InputSchema: schema(map[string]any{
				"editor": map[string]any{
					"type":        "string",
					"description": "The editor the field is in, as its head says it, for example Edit goal.",
				},
				"field": map[string]any{
					"type":        "string",
					"description": "The field, as its own label says it, for example Intent. One field per call.",
				},
				"text": map[string]any{
					"type":        "string",
					"description": "The field's whole new value, at most 4000 characters. Not a diff and not an instruction.",
				},
			}, []string{"editor", "field", "text"}),
		},
		{
			Name: OpDeposit,
			Description: "Offer one entry for the record of the sitting the human is in: a fact with the anchor where it " +
				"can be checked, a decision with the reason the human gave, an open question with the consequence " +
				"of leaving it open, or a case at the edge for the human to settle or leave open. Call it as each " +
				"comes up in the conversation, beside the answer you give in words. It writes nothing: the human sees " +
				"a card, edits it if they like, and presses Record it, and only then does it enter the record. " +
				"The outcome is the closing deposit, and it is offered when the interface asks you to close the " +
				"sitting and not before. Weigh nothing — a fact is anchored, an option carries its " +
				"consequences, and the choice is the human's.",
			InputSchema: schema(map[string]any{
				"kind": map[string]any{
					"type":        "string",
					"enum":        DepositKinds,
					"description": "Which of the five this is: fact, decision, question, case or outcome.",
				},
				"text": map[string]any{
					"type": "string",
					"description": "The entry itself, in one sentence or a short paragraph, at most 2000 characters — " +
						"or, on an outcome, the whole closing draft, at most 8000.",
				},
				"anchor": map[string]any{
					"type":        "string",
					"description": "On a fact: where it can be checked, as a path with a line or a record's own id. At most 500 characters.",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "On a decision: the reason the human gave, in their own words as you heard them. At most 500 characters.",
				},
				"consequence": map[string]any{
					"type":        "string",
					"description": "On a question, and on a case: what follows from leaving it open. At most 500 characters.",
				},
				"clause": map[string]any{
					"type":        "string",
					"description": "On a case: the clause it would become, as you heard it, which the human's Decide sheet opens with. At most 500 characters.",
				},
			}, []string{"kind", "text"}),
		},
	}
}

func schema(properties map[string]any, required []string) map[string]any {
	built := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		built["required"] = required
	}
	return built
}

// Instructions are what the server tells the model about itself at
// initialize. They say the one thing the tool descriptions cannot: that these
// readings are the same ones the human's pages are composed from.
const Instructions = "These tools read this MetaSystem workspace exactly as its browser interface does: " +
	"the board from the accepted ledger tip, a document from the checkout as it stands, " +
	"the interface's own manifest from the build that made it and the settings this seat resolves, " +
	"and the kit's own meanings from the glossary, command catalogue, rulings register and routes that own them. " +
	"Every result names the source it read from and says how much of the whole it supplied; " +
	"when a result carries a cursor, call the same tool again with it to read the rest. " +
	"Nothing here writes. The two tools that read nothing offer the human something to decide about: " +
	"suggest offers words for a field of an editor they handed over, and deposit offers one entry for the " +
	"record of a sitting they are in. Neither applies anything, and the human decides."

/* ------------------------------------------------------------- the frames -- */

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
}

type wireError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
}

// Serve answers this server's wire until the reader ends. It returns the read
// error, or nil where the client simply closed the connection.
func Serve(in io.Reader, out io.Writer, readers Readers) error {
	server := &wire{out: out, readers: readers}
	reader := bufio.NewScanner(in)
	// One message is one line, and a document's source can be long, so the
	// line bound is the one the connection between two local processes needs
	// rather than the scanner's default.
	reader.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		var in message
		if err := json.Unmarshal([]byte(line), &in); err != nil {
			server.fail(nil, -32700, "this server could not parse that message")
			continue
		}
		server.dispatch(in)
	}
	return reader.Err()
}

type wire struct {
	out     io.Writer
	readers Readers
	writing sync.Mutex
}

func (w *wire) dispatch(in message) {
	switch in.Method {
	case "initialize":
		w.answer(in.ID, map[string]any{
			"protocolVersion": negotiated(in.Params),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": ServerName, "version": "1"},
			"instructions":    Instructions,
		})
	case "notifications/initialized", "notifications/cancelled":
		// A notification carries no id and takes no answer.
	case "ping":
		w.answer(in.ID, map[string]any{})
	case "tools/list":
		w.answer(in.ID, map[string]any{"tools": Catalogue()})
	case "tools/call":
		w.call(in)
	default:
		if in.ID == nil {
			return
		}
		w.fail(in.ID, -32601, "this server answers initialize, tools/list and tools/call, not "+in.Method)
	}
}

// negotiated answers the client's own protocol version where this server
// speaks it, and this server's otherwise.
func negotiated(params json.RawMessage) string {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(params, &asked); err != nil {
		return protocolVersion
	}
	for _, spoken := range spokenVersions {
		if spoken == asked.ProtocolVersion {
			return spoken
		}
	}
	return protocolVersion
}

// call runs one tool. A refused read is a tool result marked as an error
// rather than a JSON-RPC failure: the model is meant to read it and say so,
// and a protocol error would end the call without it ever being told why.
func (w *wire) call(in message) {
	var asked struct {
		Name      string `json:"name"`
		Arguments Args   `json:"arguments"`
	}
	if err := json.Unmarshal(in.Params, &asked); err != nil {
		w.fail(in.ID, -32602, "this server could not read that tool call")
		return
	}
	result := w.readers.Answer(asked.Name, asked.Arguments)
	w.answer(in.ID, map[string]any{
		"content": []map[string]any{{"type": "text", "text": result.Text()}},
		"isError": result.Failed(),
	})
}

func (w *wire) answer(id json.RawMessage, result any) {
	if id == nil {
		return
	}
	w.write(message{JSONRPC: "2.0", ID: id, Result: result})
}

func (w *wire) fail(id json.RawMessage, code int64, text string) {
	if id == nil {
		id = json.RawMessage("null")
	}
	w.write(message{JSONRPC: "2.0", ID: id, Error: &wireError{Code: code, Message: text}})
}

func (w *wire) write(out message) {
	body, err := json.Marshal(out)
	if err != nil {
		return
	}
	w.writing.Lock()
	defer w.writing.Unlock()
	_, _ = fmt.Fprintln(w.out, string(body))
}
