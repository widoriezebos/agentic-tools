package fake_test

import (
	"context"
	"fmt"
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

func serverBed(t *testing.T) (context.Context, string, func()) {
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
			t.Fatal("fake did not start")
		}
		runtime.Gosched()
	}
	return ctx, dir, func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("fake did not stop")
		}
	}
}

func appendLine(t *testing.T, dir, line string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, "replies.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fmt.Fprintln(f, line); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramFaceSharesTheCounter(t *testing.T) {
	before := time.Now().Add(-time.Minute)
	ctx, dir, stop := serverBed(t)
	defer stop()
	p, d, err := fake.TelegramProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	posted, err := p.Post(ctx, d, "question", nil)
	if err != nil {
		t.Fatal(err)
	}
	appendLine(t, dir, fmt.Sprintf(`{"face":"telegram","reply_to":%s,"user":7001,"text":"answer"}`, posted.ID))
	got, _, err := p.Receive(ctx, d, []channel.MessageRef{posted}, "")
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	postID, _ := strconv.ParseInt(posted.ID, 10, 64)
	replyID, _ := strconv.ParseInt(got[0].Ref.ID, 10, 64)
	if replyID <= postID {
		t.Fatalf("reply %d did not sort after post %d", replyID, postID)
	}
	if got[0].SentAt.Before(before) || got[0].SentAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("reply timestamp %s is not current", got[0].SentAt)
	}
}

func TestSlackReplyUsesCurrentTimestampAndKeepsThreadRoot(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	p, d, err := fake.Provider(dir)
	if err != nil {
		t.Fatal(err)
	}
	root, err := p.Post(ctx, d, "question", nil)
	if err != nil {
		t.Fatal(err)
	}
	appendLine(t, dir, fmt.Sprintf(`{"thread_ts":%q,"user":"UWIDO","text":"answer"}`, root.ID))
	before := time.Now().Add(-time.Minute)
	got, _, err := p.Receive(ctx, d, []channel.MessageRef{root}, "")
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	if got[0].SentAt.Before(before) || got[0].SentAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("reply timestamp %s is not current", got[0].SentAt)
	}
	if got[0].ThreadID != root.ID || got[0].Ref.ThreadID != root.ID {
		t.Fatalf("reply thread = %q, reference thread = %q, want root %q", got[0].ThreadID, got[0].Ref.ThreadID, root.ID)
	}
}

func TestFixtureSuppliedTimestampsArePreserved(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()

	slackProvider, slackDest, err := fake.Provider(dir)
	if err != nil {
		t.Fatal(err)
	}
	slackRoot, err := slackProvider.Post(ctx, slackDest, "slack question", nil)
	if err != nil {
		t.Fatal(err)
	}
	const slackTS = "1893456000.123456"
	appendLine(t, dir, fmt.Sprintf(`{"thread_ts":%q,"user":"UWIDO","text":"slack answer","ts":%q}`, slackRoot.ID, slackTS))
	slackInbound, _, err := slackProvider.Receive(ctx, slackDest, []channel.MessageRef{slackRoot}, "")
	if err != nil || len(slackInbound) != 1 {
		t.Fatal(slackInbound, err)
	}
	if slackInbound[0].Ref.ID != slackTS {
		t.Fatalf("Slack timestamp = %q, want %q", slackInbound[0].Ref.ID, slackTS)
	}

	telegramProvider, telegramDest, err := fake.TelegramProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	telegramRoot, err := telegramProvider.Post(ctx, telegramDest, "telegram question", nil)
	if err != nil {
		t.Fatal(err)
	}
	const telegramDate int64 = 1234567890
	appendLine(t, dir, fmt.Sprintf(`{"face":"telegram","reply_to":%s,"user":7001,"text":"telegram answer","date":%d}`, telegramRoot.ID, telegramDate))
	telegramInbound, _, err := telegramProvider.Receive(ctx, telegramDest, []channel.MessageRef{telegramRoot}, "")
	if err != nil || len(telegramInbound) != 1 {
		t.Fatal(telegramInbound, err)
	}
	if got := telegramInbound[0].SentAt.Unix(); got != telegramDate {
		t.Fatalf("Telegram date = %d, want %d", got, telegramDate)
	}
}

func TestTelegramFaceSeparatesScriptRowsAndOtherChats(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	slackProvider, slackDest, _ := fake.Provider(dir)
	root, _ := slackProvider.Post(ctx, slackDest, "root", nil)
	appendLine(t, dir, fmt.Sprintf(`{"thread_ts":%q,"user":"USLACK","text":"slack"}`, root.ID))
	appendLine(t, dir, `{"face":"telegram","reply_to":1,"user":7001,"chat":2000,"text":"telegram"}`)
	slackInbound, _, err := slackProvider.Receive(ctx, slackDest, []channel.MessageRef{root}, "")
	if err != nil || len(slackInbound) != 1 || slackInbound[0].Text != "slack" {
		t.Fatal(slackInbound, err)
	}
	base, _ := os.ReadFile(filepath.Join(dir, "base-url"))
	d := channel.DestinationConfig{Token: "fake-telegram-token", APIBase: strings.TrimSpace(string(base)), Secrets: []string{"fake-telegram-token"}}
	updates, err := telegram.New(nil).Peek(ctx, d)
	if err != nil || len(updates) != 1 || updates[0].ChatID != 2000 || updates[0].Text != "telegram" {
		t.Fatal(updates, err)
	}
}
