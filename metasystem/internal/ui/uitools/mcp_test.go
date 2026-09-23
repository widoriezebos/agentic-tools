package uitools

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The handshake a runtime makes, and the catalogue it then reads: this is the
// whole surface an adapter uses, so it is driven whole rather than asserted
// field by field.
func TestTheHandshakeAndTheCatalogue(t *testing.T) {
	t.Parallel()
	answers := drive(t, fixture(t),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"claude","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)
	testutil.Require(t, "one answer per request", len(answers), 2)

	var handshake struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
		Instructions string `json:"instructions"`
	}
	testutil.Require(t, "the handshake decodes", json.Unmarshal(answers[0], &handshake), nil)
	testutil.Expect(t, "the client's own version is answered", handshake.ProtocolVersion, "2025-06-18")
	testutil.Expect(t, "tools are offered", handshake.Capabilities.Tools != nil, true)
	testutil.Expect(t, "the server names itself", handshake.ServerInfo.Name, ServerName)
	testutil.Expect(t, "and says what its results carry",
		strings.Contains(handshake.Instructions, "names the source it read from"), true)

	var catalogue struct {
		Tools []Tool `json:"tools"`
	}
	testutil.Require(t, "the catalogue decodes", json.Unmarshal(answers[1], &catalogue), nil)
	testutil.Require(t, "eight operations", len(catalogue.Tools), len(Operations))
	for at, tool := range catalogue.Tools {
		testutil.Expect(t, "in the order they are named "+tool.Name, tool.Name, Operations[at])
		testutil.Expect(t, "each says what it is for "+tool.Name, tool.Description != "", true)
		testutil.Expect(t, "and each takes an object "+tool.Name, tool.InputSchema["type"], "object")
	}
}

// A protocol version this server does not speak is answered with the one it
// does, rather than echoed back as agreement.
func TestAVersionThisServerDoesNotSpeakIsAnsweredWithItsOwn(t *testing.T) {
	t.Parallel()
	answers := drive(t, fixture(t),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	testutil.Expect(t, "this server's own", string(answers[0]) != "", true)
	testutil.Expect(t, "named", strings.Contains(string(answers[0]), protocolVersion), true)
}

// A tool call answers with the result's text, and a refused read is a tool
// result marked as an error rather than a protocol failure — so the model
// reads why rather than losing the call.
func TestAToolCallAnswersWithItsTextAndMarksARefusedRead(t *testing.T) {
	t.Parallel()
	answers := drive(t, fixture(t),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"goal","arguments":{"id":"waiting"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"goal","arguments":{"id":"ghost"}}}`,
	)
	testutil.Require(t, "two answers", len(answers), 2)

	read := called(t, "the goal", answers[0])
	testutil.Expect(t, "the read is not an error", read.IsError, false)
	testutil.Require(t, "one block of text", len(read.Content), 1)
	testutil.Expect(t, "which names its source",
		strings.Contains(read.Content[0].Text, "Source: the accepted tip "+observedTip), true)
	testutil.Expect(t, "and carries the goal",
		strings.Contains(read.Content[0].Text, "- Goal: waiting"), true)

	refused := called(t, "the missing goal", answers[1])
	testutil.Expect(t, "the refused read is marked", refused.IsError, true)
	testutil.Expect(t, "and says what failed",
		strings.Contains(refused.Content[0].Text, "the accepted tip carries no goal ghost"), true)
}

// A method this server does not answer is an error naming what it does answer;
// a notification with no id is answered with nothing at all.
func TestAnUnknownMethodIsRefusedAndANotificationIsNotAnswered(t *testing.T) {
	t.Parallel()
	answers := drive(t, fixture(t),
		`{"jsonrpc":"2.0","method":"notifications/progress","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"resources/list","params":{}}`,
	)
	testutil.Require(t, "only the request is answered", len(answers), 1)
	var failure struct {
		Error struct {
			Code    int64  `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	testutil.Require(t, "decodes", json.Unmarshal(answers[0], &failure), nil)
	testutil.Expect(t, "as method not found", failure.Error.Code, int64(-32601))
	testutil.Expect(t, "naming what it does answer",
		strings.Contains(failure.Error.Message, "tools/list and tools/call"), true)
}

/* ---------------------------------------------------------------- driving -- */

type toolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// called decodes one tools/call answer's result.
func called(t *testing.T, named string, answer json.RawMessage) toolResult {
	t.Helper()
	var result toolResult
	testutil.Require(t, "the tool result decodes: "+named, json.Unmarshal(answer, &result), nil)
	return result
}

// drive writes the lines to the server and answers what it wrote back, one
// message per answer, with the envelope stripped down to what a caller reads.
func drive(t *testing.T, readers Readers, lines ...string) []json.RawMessage {
	t.Helper()
	var out bytes.Buffer
	testutil.Require(t, "the wire ends cleanly",
		Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, readers), nil)
	answers := []json.RawMessage{}
	for at, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var envelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
		}
		named := " " + strconv.Itoa(at)
		testutil.Require(t, "answer decodes"+named, json.Unmarshal([]byte(line), &envelope), nil)
		testutil.Expect(t, "answer is JSON-RPC 2.0"+named, envelope.JSONRPC, "2.0")
		testutil.Expect(t, "answer names the request it answers"+named, envelope.ID != nil, true)
		if envelope.Result != nil {
			answers = append(answers, envelope.Result)
			continue
		}
		answers = append(answers, json.RawMessage(line))
	}
	return answers
}
