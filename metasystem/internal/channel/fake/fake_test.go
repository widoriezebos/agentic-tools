package fake_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func writeControl(t *testing.T, dir string, control any) {
	t.Helper()
	data, err := json.Marshal(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "control.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func journalRows(t *testing.T, dir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
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

func TestTelegramTokenRoutes(t *testing.T) {
	_, dir, stop := serverBed(t)
	defer stop()
	baseBytes, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseBytes))
	tokenCases := []struct {
		name     string
		token    string
		listener string
	}{
		{name: "arbitrary", token: "environment-token"},
		{name: "bare fake token", token: "fake-telegram-token"},
		{name: "listener fake token", token: "fake-telegram-token-m3", listener: "m3"},
	}
	for _, tc := range tokenCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(base+"/bot"+tc.token+"/getMe", "application/json", strings.NewReader(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var body struct {
				OK bool `json:"ok"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK || !body.OK {
				t.Fatalf("status = %s, body = %+v", resp.Status, body)
			}
		})
	}
	rows := journalRows(t, dir)
	if len(rows) != len(tokenCases) {
		t.Fatalf("journal rows = %+v", rows)
	}
	for i, tc := range tokenCases {
		if rows[i]["method"] != "getMe" || rows[i]["listener"] != tc.listener {
			t.Fatalf("%s journal row = %+v", tc.name, rows[i])
		}
	}

	for _, path := range []string{"/bot/getMe", "/bottoken/", "/bottoken/getMe/again"} {
		resp, err := http.Post(base+path, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("path %q status = %s", path, resp.Status)
		}
	}
	if rows = journalRows(t, dir); len(rows) != len(tokenCases) {
		t.Fatalf("malformed paths were journaled: %+v", rows)
	}
}

func TestTelegramListenersShareStreamAndConfirmedOffset(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	first, firstDest, err := fake.TelegramProvider(dir, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, secondDest, err := fake.TelegramProvider(dir, "second")
	if err != nil {
		t.Fatal(err)
	}
	if firstDest.Token != "fake-telegram-token-first" || secondDest.Token != "fake-telegram-token-second" {
		t.Fatalf("listener tokens = %q and %q", firstDest.Token, secondDest.Token)
	}
	appendLine(t, dir, `{"face":"telegram","user":7001,"text":"one"}`)
	appendLine(t, dir, `{"face":"telegram","user":7001,"text":"two"}`)
	firstBatch, firstCursor, err := first.Receive(ctx, firstDest, nil, "")
	if err != nil || len(firstBatch) != 2 || firstCursor != firstBatch[1].Ack {
		t.Fatalf("first listener batch = %+v, cursor %q, error %v", firstBatch, firstCursor, err)
	}
	secondBatch, secondCursor, err := second.Receive(ctx, secondDest, nil, "")
	if err != nil || len(secondBatch) != 2 || secondCursor != firstCursor || secondBatch[0].UpdateID != firstBatch[0].UpdateID {
		t.Fatalf("second listener batch = %+v, cursor %q, error %v", secondBatch, secondCursor, err)
	}
	if err := first.Confirm(ctx, firstDest, firstBatch[0].Ack); err != nil {
		t.Fatal(err)
	}
	withoutOffset, _, err := second.Receive(ctx, secondDest, nil, "")
	if err != nil || len(withoutOffset) != 1 || withoutOffset[0].UpdateID != firstBatch[1].UpdateID {
		t.Fatalf("absent offset returned %+v after confirmation: %v", withoutOffset, err)
	}
	afterConfirm, _, err := second.Receive(ctx, secondDest, nil, "1")
	if err != nil || len(afterConfirm) != 1 || afterConfirm[0].UpdateID != firstBatch[1].UpdateID {
		t.Fatalf("lower requested offset replayed confirmed updates; returned %+v: %v", afterConfirm, err)
	}

	rows := journalRows(t, dir)
	var previous float64
	var sawFirstConfirm, sawLowerOffsetConfirm bool
	for _, row := range rows {
		sequence, ok := row["sequence"].(float64)
		if !ok || sequence <= previous {
			t.Fatalf("non-monotonic journal sequence after %.0f: %+v", previous, row)
		}
		previous = sequence
		if row["method"] == "confirm" {
			if row["listener"] == "first" && row["raw"] == "getUpdates" {
				sawFirstConfirm = true
			}
			form, _ := row["form"].(map[string]any)
			if row["listener"] == "second" && row["raw"] == "getUpdates" && form["offset"] == float64(1) {
				sawLowerOffsetConfirm = true
			}
		}
	}
	if !sawFirstConfirm || !sawLowerOffsetConfirm {
		t.Fatalf("journal did not identify both confirming listeners and offsets: %+v", rows)
	}
}

func TestTelegramDeliverOnlyToAndConflictCountdown(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	first, firstDest, _ := fake.TelegramProvider(dir, "first")
	second, secondDest, _ := fake.TelegramProvider(dir, "second")
	appendLine(t, dir, `{"face":"telegram","user":7001,"text":"private"}`)
	batch, _, err := first.Receive(ctx, firstDest, nil, "")
	if err != nil || len(batch) != 1 {
		t.Fatal(batch, err)
	}
	writeControl(t, dir, map[string]any{
		"deliverOnlyTo": map[string][]string{strconv.FormatInt(batch[0].UpdateID, 10): {"first"}},
		"conflict":      []map[string]any{{"listener": "first", "remaining": 2, "description": "terminated by other getUpdates request"}},
	})
	for attempt := 0; attempt < 2; attempt++ {
		if _, _, err := first.Receive(ctx, firstDest, nil, ""); err == nil || !channel.IsKind(err, channel.Busy) {
			t.Fatalf("conflict attempt %d: %v", attempt+1, err)
		}
	}
	visible, _, err := first.Receive(ctx, firstDest, nil, "")
	if err != nil || len(visible) != 1 {
		t.Fatalf("first listener after conflict countdown = %+v: %v", visible, err)
	}
	hidden, _, err := second.Receive(ctx, secondDest, nil, "")
	if err != nil || len(hidden) != 0 {
		t.Fatalf("second listener saw restricted update %+v: %v", hidden, err)
	}
}

func TestTelegramPauseBeforeHoldsOneRequest(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	p, dest, _ := fake.TelegramProvider(dir, "paused")
	until := filepath.Join(dir, "release")
	writeControl(t, dir, map[string]any{"pauseBefore": []map[string]any{{"listener": "paused", "method": "sendMessage", "until": until}}})
	done := make(chan error, 1)
	go func() {
		_, err := p.Post(ctx, dest, "held", nil)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("paused request returned before release: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	if err := os.WriteFile(until, []byte("release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("paused request did not resume")
	}
}

func TestTelegramLongPollWaitsForAnUpdate(t *testing.T) {
	_, dir, stop := serverBed(t)
	defer stop()
	base, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		count int
		err   error
	}
	done := make(chan result, 1)
	started := time.Now()
	go func() {
		resp, err := http.Post(strings.TrimSpace(string(base))+"/botfake-telegram-token-waiting/getUpdates", "application/json", strings.NewReader(`{"timeout":1,"limit":100}`))
		if err != nil {
			done <- result{err: err}
			return
		}
		defer resp.Body.Close()
		var envelope struct {
			Result []json.RawMessage `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&envelope)
		done <- result{count: len(envelope.Result), err: err}
	}()
	select {
	case got := <-done:
		t.Fatalf("long poll returned before an update arrived: %+v", got)
	case <-time.After(250 * time.Millisecond):
	}
	appendLine(t, dir, `{"face":"telegram","user":7001,"text":"arrived"}`)
	select {
	case got := <-done:
		if got.err != nil || got.count != 1 {
			t.Fatalf("long poll result = %+v", got)
		}
		if elapsed := time.Since(started); elapsed < 250*time.Millisecond || elapsed > 2*time.Second {
			t.Fatalf("long poll returned after %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("long poll did not return after an update arrived")
	}
}

func TestTelegramLongPollReloadsDeliveryControls(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	allowed, allowedDest, err := fake.TelegramProvider(dir, "allowed")
	if err != nil {
		t.Fatal(err)
	}
	posted, err := allowed.Post(ctx, allowedDest, "counter seed", nil)
	if err != nil {
		t.Fatal(err)
	}
	postedID, err := strconv.ParseInt(posted.ID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	nextUpdateID := postedID + 1

	type result struct {
		count   int
		status  int
		elapsed time.Duration
		err     error
	}
	done := make(chan result, 1)
	started := time.Now()
	go func() {
		resp, err := http.Post(allowedDest.APIBase+"/botfake-telegram-token-excluded/getUpdates", "application/json", strings.NewReader(`{"timeout":3,"limit":100}`))
		if err != nil {
			done <- result{err: err}
			return
		}
		defer resp.Body.Close()
		var envelope struct {
			Result []json.RawMessage `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&envelope)
		done <- result{count: len(envelope.Result), status: resp.StatusCode, elapsed: time.Since(started), err: err}
	}()
	select {
	case got := <-done:
		t.Fatalf("long poll returned before controls changed: %+v", got)
	case <-time.After(250 * time.Millisecond):
	}
	writeControl(t, dir, map[string]any{
		"deliverOnlyTo": map[string][]string{strconv.FormatInt(nextUpdateID, 10): {"allowed"}},
	})
	appendLine(t, dir, `{"face":"telegram","user":7001,"text":"restricted"}`)
	select {
	case got := <-done:
		if got.err != nil || got.status != http.StatusOK || got.count != 0 {
			t.Fatalf("excluded long poll result = %+v", got)
		}
		if got.elapsed < 2500*time.Millisecond {
			t.Fatalf("excluded long poll returned after %s", got.elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("excluded long poll did not reach its deadline")
	}
	visible, _, err := allowed.Receive(ctx, allowedDest, nil, "")
	if err != nil || len(visible) != 1 || visible[0].UpdateID != nextUpdateID {
		t.Fatalf("allowed listener result = %+v: %v", visible, err)
	}
}

func TestTelegramLongPollReloadsMalformedControl(t *testing.T) {
	_, dir, stop := serverBed(t)
	defer stop()
	base, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		status      int
		description string
		err         error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := http.Post(strings.TrimSpace(string(base))+"/botfake-telegram-token-waiting/getUpdates", "application/json", strings.NewReader(`{"timeout":3,"limit":100}`))
		if err != nil {
			done <- result{err: err}
			return
		}
		defer resp.Body.Close()
		var envelope struct {
			Description string `json:"description"`
		}
		err = json.NewDecoder(resp.Body).Decode(&envelope)
		done <- result{status: resp.StatusCode, description: envelope.Description, err: err}
	}()
	select {
	case got := <-done:
		t.Fatalf("long poll returned before control became malformed: %+v", got)
	case <-time.After(250 * time.Millisecond):
	}
	if err := os.WriteFile(filepath.Join(dir, "control.json"), []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil || got.status != http.StatusInternalServerError || !strings.Contains(got.description, "invalid character") {
			t.Fatalf("malformed mid-poll control result = %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("malformed mid-poll control did not end the request")
	}
}

func TestMalformedControlFailsEveryRequest(t *testing.T) {
	ctx, dir, stop := serverBed(t)
	defer stop()
	if err := os.WriteFile(filepath.Join(dir, "control.json"), []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, dest, _ := fake.TelegramProvider(dir, "broken")
	if _, err := p.Credential(ctx, dest); err == nil || !channel.IsKind(err, channel.ReceiveFailed) || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("malformed control was not surfaced through Telegram: %v", err)
	}
	slackProvider, slackDest, _ := fake.Provider(dir)
	if _, err := slackProvider.Credential(ctx, slackDest); err == nil || !channel.IsKind(err, channel.ReceiveFailed) || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("malformed control was not surfaced through Slack: %v", err)
	}
}
