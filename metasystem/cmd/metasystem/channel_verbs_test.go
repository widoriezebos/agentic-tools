package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelFake "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func captureChannelOutput(t *testing.T, run func() int) (int, string, string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	code := run()
	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outW.Close()
	_ = errW.Close()
	out, _ := io.ReadAll(outR)
	problem, _ := io.ReadAll(errR)
	_ = outR.Close()
	_ = errR.Close()
	return code, string(out), string(problem)
}

func TestConfigurationIndependentChannelVerbs(t *testing.T) {
	root := t.TempDir()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: root, Goal: "g", Kind: "other", Machine: "m", Facts: []string{"fact"}})
	if err != nil {
		t.Fatal(err)
	}
	q.Answer = &channel.Answer{Text: "yes", Phase: "matched"}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "channel", "questions", q.ID+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	// Close persists the exported answer without requiring any channel configuration.
	if err := channel.Close(root, q.ID, "test", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if code, _, problem := captureChannelOutput(t, func() int { return runChannelShow([]string{"--root", root, "--question", q.ID}) }); code != 0 {
		t.Fatal(problem)
	}
	if code, out, problem := captureChannelOutput(t, func() int { return runChannelWait([]string{"--root", root, "--question", q.ID}) }); code != 0 || strings.TrimSpace(out) != "yes" {
		t.Fatal(code, out, problem)
	}
	if code, out, problem := captureChannelOutput(t, func() int { return runChannelFakeCode([]string{"--secret", "JBSWY3DPEHPK3PXP", "--at", "59"}) }); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatal(code, out, problem)
	}
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); !os.IsNotExist(err) {
		t.Fatal("configuration-independent verbs created or required metasystem.conf")
	}
}

func commandFakeBed(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- channelFake.Serve(ctx, dir) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "base-url")); err == nil {
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("fake did not stop")
				}
			})
			return dir, strings.TrimSpace(string(b))
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("fake did not start")
		}
		runtime.Gosched()
	}
}

func TestChannelStatusPostSeedsAnUnbootedBrainStatus(t *testing.T) {
	dir, _ := commandFakeBed(t)
	root := syncedClaimedGoalFixture(t)
	conf, err := os.OpenFile(filepath.Join(root, "metasystem.conf"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conf, "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=%s\n", dir); err != nil {
		_ = conf.Close()
		t.Fatal(err)
	}
	if err := conf.Close(); err != nil {
		t.Fatal(err)
	}
	record := brain.Record{
		Schema: brain.Schema, Ledger: goal.ExistingLedgerIdentity(root), Machine: "mac-cli",
		DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z",
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(brain.StatusPath(root)); !os.IsNotExist(err) {
		t.Fatalf("unbooted fixture unexpectedly had a status file: %v", err)
	}

	code, _, problem := captureChannelOutput(t, func() int { return runChannelStatus([]string{"--root", root, "--post"}) })
	if code != 0 || problem != "" {
		t.Fatalf("first status post failed: code=%d stderr=%q", code, problem)
	}
	status, err := brain.ReadStatus(root)
	if err != nil || status.Line != brain.StatusLine(record) || status.LastPostedAt == "" {
		t.Fatalf("first status post did not seed and mark brain status: status=%+v err=%v", status, err)
	}
}

func TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID(t *testing.T) {
	dir, base := commandFakeBed(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.telegram.api-base="+base+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvName("channel.destination.fleet.telegram.bot-token"), "environment-token")
	if err := os.WriteFile(filepath.Join(dir, "replies.jsonl"), []byte(`{"face":"telegram","user":7001,"chat":1000,"text":"hello from the phone"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, problem := captureChannelOutput(t, func() int { return runChannelTelegram([]string{"peek", "--root", root}) })
	if code != 0 || strings.TrimSpace(out) != "chat=1000 user=7001 text=hello from the phone" || problem != "" {
		t.Fatal(code, out, problem)
	}
}

func TestTelegramPeekTokenNeverAppearsInErrors(t *testing.T) {
	const token = "command-secret-token"
	t.Setenv(config.EnvName("channel.destination.fleet.telegram.bot-token"), token)
	cases := []struct {
		name string
		base func() (string, func())
	}{
		{"transport", func() (string, func()) { return "http://127.0.0.1:1", func() {} }},
		{"redirect", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/bot"+token+r.URL.Path, http.StatusFound)
			}))
			return s.URL, s.Close
		}},
		{"echoed 401", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"ok":false,"description":%q}`, token)
			}))
			return s.URL, s.Close
		}},
		{"malformed JSON", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, token+" not json") }))
			return s.URL, s.Close
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, cleanup := tc.base()
			defer cleanup()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("channel.destination.fleet.telegram.api-base="+base+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			code, _, problem := captureChannelOutput(t, func() int { return runChannelTelegram([]string{"peek", "--root", root}) })
			if code == 0 || strings.Contains(problem, token) || strings.Contains(problem, "/bot"+token+"/") {
				t.Fatal(code, problem)
			}
		})
	}
}
