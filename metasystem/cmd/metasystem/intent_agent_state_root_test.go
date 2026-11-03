package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The goal ledger lives at the installation's state root. In a checkout
// whose installation is a subdirectory, agent ask --goal reads the goals
// there: read at the checkout's top, no goal exists and every goal is
// refused as not open.
func TestAgentAskReadsTheGoalsAtTheInstallationBelowTheCheckout(t *testing.T) {
	t.Parallel()
	b := newAgentBed(t, "m1a")
	installation := filepath.Join(b.root, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	var read []string
	owners.agent.ledger = func(root string) (board.Ownership, error) {
		read = append(read, root)
		return b.ledger, nil
	}
	command, rest, ok := resolveIntentArgv([]string{"agent", "ask", "--goal", "goal-x", "--text", "are you there?"})
	if !ok {
		t.Fatal("no public command agent ask")
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, b.root, owners); code != 0 {
		t.Fatalf("agent ask --goal goal-x = %d %q %q; want it sent", code, stdout.String(), stderr.String())
	}
	want, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range read {
		if got, err := filepath.EvalSymlinks(root); err != nil || got != want {
			t.Fatalf("the goals were read at %s; want the installation %s", root, installation)
		}
	}
	if len(read) == 0 {
		t.Fatal("agent ask --goal read no goals")
	}
}

func leaseInboxBed(t *testing.T, nested bool) (*agentBed, intentOwners, string, string) {
	t.Helper()
	b := newAgentBed(t, "m1b")
	installation, cwd, config := filepath.Join(b.root, "metasystem"), b.root, "metasystem.template=true\n"
	if nested {
		installation = filepath.Join(b.root, "tools", "ms")
		cwd, config = installation, ""
	}
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	installation, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	owners := b.owners()
	owners.agent.root = nil
	owners.agent.getenv = func(key string) string {
		if key != "METASYSTEM_HOOK_DELEGATE_JOB" {
			t.Fatalf("unexpected environment read %q", key)
		}
		return ""
	}
	if code, _, stderr := b.as("m1a").run("agent", "ask", "m1b", "--id", "q-holder", "--text", "Can you read it?", "--deadline", "30m"); code != 0 {
		t.Fatalf("ask: %d %s", code, stderr)
	}
	return b, owners, cwd, installation
}

func readLeaseInbox(t *testing.T, owners intentOwners, cwd string, flags ...string) string {
	t.Helper()
	command, rest, ok := resolveIntentArgv(append([]string{"agent", "inbox"}, flags...))
	if !ok {
		t.Fatal("no public inbox command")
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, cwd, owners); code != 0 {
		t.Fatalf("inbox: %d %s %s", code, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestTheLeaseHolderReadMarksItsInbox(t *testing.T) {
	t.Parallel()
	for _, nested := range []bool{false, true} {
		b, owners, cwd, installation := leaseInboxBed(t, nested)
		var calls []string
		owners.agent.classify = func(root, adapters string, pid int64) (lease.ClassifyResult, error) {
			calls = append(calls, root)
			if adapters != installation || pid != int64(os.Getpid()) {
				t.Fatalf("classification adapters=%s pid=%d", adapters, pid)
			}
			if root == installation {
				return lease.ClassifyResult{Class: lease.ClassMain, MainId: "main-pane", Holder: true}, nil
			}
			return lease.ClassifyResult{Class: lease.ClassDelegate, Pid: 4242}, nil
		}
		text := readLeaseInbox(t, owners, cwd)
		if !strings.Contains(text, "Can you read it?") || strings.Contains(text, "not marked read") {
			t.Fatalf("holder read (nested=%t): %s", nested, text)
		}
		markerPath := filepath.Join(board.Dir(b.home), "m1b", "mailbox", "delivered", "q-holder", "m1b.json")
		data, err := os.ReadFile(markerPath)
		var marker board.Marker
		if err != nil || json.Unmarshal(data, &marker) != nil || marker.Event != "inbox" || marker.Machine != "m1b" || marker.Lineage != "lineage-m1b" || !marker.At.Equal(b.now) {
			t.Fatalf("delivery marker: %s %v", data, err)
		}
		if len(calls) != 1 || calls[0] != installation {
			t.Fatalf("classified at %v, want once at %s", calls, installation)
		}
		if text := readLeaseInbox(t, owners, cwd); !strings.Contains(strings.ToLower(text), "nothing pending") {
			t.Fatalf("second read: %s", text)
		}
	}
}

func TestOnlyTheLeaseHolderMarksItsInbox(t *testing.T) {
	t.Parallel()
	b, owners, cwd, installation := leaseInboxBed(t, false)
	marker := filepath.Join(board.Dir(b.home), "m1b", "mailbox", "delivered", "q-holder", "m1b.json")
	for _, reader := range []struct {
		name, job string
		result    lease.ClassifyResult
		err       error
	}{
		{name: "job", job: "job-7", result: lease.ClassifyResult{Class: lease.ClassMain, Holder: true}},
		{name: "delegate", result: lease.ClassifyResult{Class: lease.ClassDelegate}},
		{name: "person", result: lease.ClassifyResult{Class: lease.ClassHuman}},
		{name: "other main", result: lease.ClassifyResult{Class: lease.ClassMain, MainId: "main-other"}},
		{name: "unavailable", result: lease.ClassifyResult{Class: lease.ClassMain, Holder: true}, err: errors.New("classification unavailable")},
	} {
		calls := 0
		owners.agent.getenv = func(key string) string {
			if key != "METASYSTEM_HOOK_DELEGATE_JOB" {
				t.Fatalf("unexpected environment read %q", key)
			}
			return reader.job
		}
		owners.agent.classify = func(root, adapters string, pid int64) (lease.ClassifyResult, error) {
			calls++
			if root != installation || adapters != installation || pid != int64(os.Getpid()) {
				t.Fatalf("classification at %s, %s, %d", root, adapters, pid)
			}
			return reader.result, reader.err
		}
		text := readLeaseInbox(t, owners, cwd)
		if !strings.Contains(text, "Can you read it?") || !strings.Contains(text, "Shown, not marked read") {
			t.Fatalf("%s: %s", reader.name, text)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("%s marked: %v", reader.name, err)
		}
		if reader.job != "" && calls != 0 || reader.job == "" && calls != 1 {
			t.Fatalf("%s classified %d times", reader.name, calls)
		}
	}
	owners.agent.getenv = b.owners().agent.getenv
	owners.agent.classify = b.owners().agent.classify
	if text := readLeaseInbox(t, owners, cwd, "--all"); !strings.Contains(text, "Can you read it?") {
		t.Fatalf("all: %s", text)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("all marked: %v", err)
	}
	readLeaseInbox(t, owners, cwd)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("holder did not receive the message: %v", err)
	}
}
