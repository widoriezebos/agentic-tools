package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// fakeTestRunResult is an internal test run child's envelope as a fake
// runner returns it: the outcome its exit stands for, code and data.
func fakeTestRunResult(exit int, code string, data any) verbresult.Result {
	result := verbresult.FromError("internal test run", exit, nil, data)
	result.Code = code
	if code != "" && exit == 1 {
		result.Outcome = verbresult.Refused
	}
	return result
}

// testRunEnvelopeLine is that envelope as the child prints it on stdout.
func testRunEnvelopeLine(t *testing.T, exit int, code, summary string, data any) string {
	t.Helper()
	result := fakeTestRunResult(exit, code, data)
	result.Summary = summary
	var out bytes.Buffer
	if err := verbresult.Write(&out, result); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// testRunEnvelopeScript is a fake internal test run: it says stderr words
// for a person, prints its envelope on stdout and exits.
func testRunEnvelopeScript(t *testing.T, exit int, code, summary string, data any, stderr string) string {
	t.Helper()
	return fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %s >&2\nprintf '%%s' %s\nexit %d\n",
		shellquote.Quote(stderr), shellquote.Quote(testRunEnvelopeLine(t, exit, code, summary, data)), exit)
}

// withTestRunEnvelope makes a fake test run script print the envelope its
// final "exit N" stands for, on stdout, before it exits.
func withTestRunEnvelope(t *testing.T, script string) string {
	t.Helper()
	trimmed := strings.TrimRight(script, "\n")
	at := strings.LastIndex(trimmed, "\nexit ")
	if at < 0 {
		t.Fatalf("fake test run script has no final exit: %q", script)
	}
	exit, err := strconv.Atoi(strings.TrimSpace(trimmed[at+len("\nexit "):]))
	if err != nil {
		t.Fatalf("fake test run script's final exit: %v", err)
	}
	return trimmed[:at] + "\nprintf '%s' " + shellquote.Quote(testRunEnvelopeLine(t, exit, "", "", nil)) + trimmed[at:] + "\n"
}
