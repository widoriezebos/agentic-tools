package telegram_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram"
)

func bed(t *testing.T) (context.Context, channel.Provider, channel.DestinationConfig, string) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- fake.Serve(ctx, dir) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "base-url")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("fake did not publish base-url")
		}
		runtime.Gosched()
	}
	p, d, err := fake.TelegramProvider(dir)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
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
	return ctx, p, d, dir
}

func journal(t *testing.T, dir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []map[string]any
	s := bufio.NewScanner(f)
	for s.Scan() {
		var row map[string]any
		if err := json.Unmarshal(s.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestPostRootAndReplyChain(t *testing.T) {
	ctx, p, d, dir := bed(t)
	root, err := p.Post(ctx, d, "root", nil)
	if err != nil || root.ID == "" || root.ThreadID != root.ID {
		t.Fatal(root, err)
	}
	reply, err := p.Post(ctx, d, "reply", &root)
	if err != nil || reply.ThreadID != root.ID {
		t.Fatal(reply, err)
	}
	rows := journal(t, dir)
	form := rows[len(rows)-1]["form"].(map[string]any)
	params := form["reply_parameters"].(map[string]any)
	if strconv.FormatInt(int64(params["message_id"].(float64)), 10) != root.ID {
		t.Fatal(form)
	}
}

func TestPostSplitsLongTextOnLines(t *testing.T) {
	ctx, p, d, dir := bed(t)
	text := strings.Repeat("a", 3000) + "\n" + strings.Repeat("b", 3000) + "\n" + strings.Repeat("c", 3000)
	first, err := p.Post(ctx, d, text, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows := journal(t, dir)
	if len(rows) != 3 || first.ID == "" {
		t.Fatalf("rows=%d first=%+v", len(rows), first)
	}
	for i, row := range rows {
		form := row["form"].(map[string]any)
		if i > 0 && form["reply_parameters"] == nil {
			t.Fatalf("chunk %d is not chained: %+v", i+1, form)
		}
		if strings.Contains(form["text"].(string), "a\nb") {
			t.Fatal("line boundary was cut")
		}
	}
}

func TestPostHandlesSingleLineOverChunkLimit(t *testing.T) {
	ctx, p, d, dir := bed(t)
	if _, err := p.Post(ctx, d, strings.Repeat("界", 8001), nil); err != nil {
		t.Fatal(err)
	}
	rows := journal(t, dir)
	if len(rows) != 3 {
		t.Fatalf("got %d chunks", len(rows))
	}
	for _, row := range rows {
		if len([]rune(row["form"].(map[string]any)["text"].(string))) > 4000 {
			t.Fatal("oversized chunk")
		}
	}
}

func appendReply(t *testing.T, dir string, row string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "replies.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fmt.Fprintln(f, row); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReceiveResolvesRootsFromEveryPostedRef(t *testing.T) {
	ctx, p, d, dir := bed(t)
	root, _ := p.Post(ctx, d, "root", nil)
	receipt, _ := p.Post(ctx, d, "receipt", &root)
	appendReply(t, dir, fmt.Sprintf(`{"face":"telegram","reply_to":%s,"user":7001,"text":"answer"}`, receipt.ID))
	appendReply(t, dir, `{"face":"telegram","reply_to":0,"user":7001,"text":"stray"}`)
	got, _, err := p.Receive(ctx, d, []channel.MessageRef{root, receipt}, "")
	if err != nil || len(got) != 2 || got[0].ThreadID != root.ID || got[1].ThreadID != "" {
		t.Fatal(got, err)
	}
	without, _, err := p.Receive(ctx, d, nil, "")
	if err != nil || without[0].Ref != got[0].Ref || without[0].ThreadID != "" {
		t.Fatal(without, err)
	}
}

func TestReceiveFiltersOtherChatsAndOwnBot(t *testing.T) {
	ctx, p, d, dir := bed(t)
	appendReply(t, dir, `{"face":"telegram","reply_to":1,"user":424242,"text":"own"}`)
	appendReply(t, dir, `{"face":"telegram","reply_to":1,"user":7001,"chat":2000,"text":"other"}`)
	got, cursor, err := p.Receive(ctx, d, []channel.MessageRef{{ID: "1", ThreadID: "1"}}, "")
	if err != nil || len(got) != 0 || cursor == "" {
		t.Fatal(got, cursor, err)
	}
}

func TestReceiveAdvancesOffsetOnlyOnUpdates(t *testing.T) {
	ctx, p, d, _ := bed(t)
	got, cursor, err := p.Receive(ctx, d, nil, "77")
	if err != nil || len(got) != 0 || cursor != "77" {
		t.Fatal(got, cursor, err)
	}
}

func TestReceiveHonoursLimitAndOffset(t *testing.T) {
	ctx, p, d, dir := bed(t)
	for i := 0; i < 101; i++ {
		appendReply(t, dir, fmt.Sprintf(`{"face":"telegram","reply_to":1,"user":7001,"text":"%d"}`, i))
	}
	got, cursor, err := p.Receive(ctx, d, []channel.MessageRef{{ID: "1", ThreadID: "1"}}, "")
	if err != nil || len(got) != 100 || cursor == "" {
		t.Fatal(len(got), cursor, err)
	}
	again, _, err := p.Receive(ctx, d, []channel.MessageRef{{ID: "1", ThreadID: "1"}}, cursor)
	if err != nil || len(again) != 1 {
		t.Fatal(len(again), err)
	}
}

func TestCredentialIsBotIdentity(t *testing.T) {
	ctx, p, d, _ := bed(t)
	id, err := p.Credential(ctx, d)
	if err != nil || id.UserID != "424242" {
		t.Fatal(id, err)
	}
}

func TestUnconfiguredIsTyped(t *testing.T) {
	p := telegram.New(nil)
	_, err := p.Post(context.Background(), channel.DestinationConfig{}, "x", nil)
	if err == nil || !channel.IsKind(err, channel.Unconfigured) || !errors.As(err, new(*channel.ProviderError)) {
		t.Fatal(err)
	}
}

func TestWebhookConflictIsTyped(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			fmt.Fprint(w, `{"ok":true,"result":{"id":1}}`)
			return
		}
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"ok":false,"description":"Conflict"}`)
	}))
	defer s.Close()
	d := channel.DestinationConfig{ChannelID: "1", Token: "token", APIBase: s.URL, Secrets: []string{"token"}}
	_, _, err := telegram.New(nil).Receive(context.Background(), d, nil, "")
	if err == nil || !channel.IsKind(err, channel.ReceiveFailed) || !strings.Contains(err.Error(), "a webhook is set on this bot; delete it") {
		t.Fatal(err)
	}
}

func TestTokenNeverAppearsInErrors(t *testing.T) {
	const token = "super-secret-token"
	cases := []struct {
		name string
		base func() (string, func())
	}{
		{"closed-port", func() (string, func()) { return "http://127.0.0.1:1", func() {} }},
		{"redirect", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/bot"+token+r.URL.Path, http.StatusFound)
			}))
			return s.URL, s.Close
		}},
		{"echoed-401", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"ok":false,"description":%q}`, token)
			}))
			return s.URL, s.Close
		}},
		{"malformed", func() (string, func()) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, token+" not json") }))
			return s.URL, s.Close
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, cleanup := tc.base()
			defer cleanup()
			d := channel.DestinationConfig{Token: token, APIBase: base, Secrets: []string{token}}
			_, err := telegram.New(nil).Peek(context.Background(), d)
			if err == nil || strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "/bot"+token+"/") {
				t.Fatalf("unscrubbed or absent error: %v", err)
			}
		})
	}
}
