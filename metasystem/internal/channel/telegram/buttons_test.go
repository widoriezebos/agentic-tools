package telegram_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram"
)

func optionPoster(t *testing.T, p channel.Provider) channel.OptionPoster {
	t.Helper()
	poster, ok := p.(channel.OptionPoster)
	if !ok {
		t.Fatalf("%T posts no buttons", p)
	}
	return poster
}

// rowsOf are the journal rows of one method, in the order they were sent.
func rowsOf(t *testing.T, dir, method string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, row := range journal(t, dir) {
		if row["method"] == method {
			out = append(out, row["form"].(map[string]any))
		}
	}
	return out
}

func TestPostOptionsPutsOneButtonPerOption(t *testing.T) {
	t.Parallel()
	ctx, p, d, dir := bed(t)
	posted, err := optionPoster(t, p).PostOptions(ctx, d, "q", []string{"Continue", "Stop here"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sent := rowsOf(t, dir, "sendMessage")
	if len(sent) != 1 || sent[0]["text"] != "q" {
		t.Fatalf("sendMessage rows = %+v", sent)
	}
	want := map[string]any{"inline_keyboard": []any{
		[]any{map[string]any{"text": "Continue", "callback_data": "Continue"}},
		[]any{map[string]any{"text": "Stop here", "callback_data": "Stop here"}},
	}}
	if !reflect.DeepEqual(sent[0]["reply_markup"], want) {
		t.Fatalf("keyboard = %#v, want %#v", sent[0]["reply_markup"], want)
	}
	if posted.ID == "" || posted.ThreadID != posted.ID {
		t.Fatalf("the returned ref is not the keyboard's message: %+v", posted)
	}
	reply, err := optionPoster(t, p).PostOptions(ctx, d, "again", []string{"Continue"}, &posted)
	if err != nil || reply.ThreadID != posted.ID {
		t.Fatal(reply, err)
	}
	sent = rowsOf(t, dir, "sendMessage")
	params, _ := sent[1]["reply_parameters"].(map[string]any)
	if len(sent) != 2 || params == nil || strconv.FormatInt(int64(params["message_id"].(float64)), 10) != posted.ID || sent[1]["reply_markup"] == nil {
		t.Fatalf("a keyboard posted in a thread does not name it: %+v", sent)
	}
}

func TestReceiveTurnsATapIntoAReply(t *testing.T) {
	t.Parallel()
	ctx, p, d, dir := bed(t)
	posted, err := optionPoster(t, p).PostOptions(ctx, d, "q", []string{"Continue", "Stop here"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	appendReply(t, dir, fmt.Sprintf(`{"face":"telegram","user":7001,"tap":"Continue","reply_to":%s}`, posted.ID))
	appendReply(t, dir, fmt.Sprintf(`{"face":"telegram","user":7001,"chat":2000,"tap":"Continue","reply_to":%s}`, posted.ID))
	appendReply(t, dir, fmt.Sprintf(`{"face":"telegram","user":7001,"text":"stop here","reply_to":%s}`, posted.ID))
	got, _, err := p.Receive(ctx, d, []channel.MessageRef{posted}, "")
	if err != nil || len(got) != 2 {
		t.Fatalf("inbounds = %+v, %v; want the tap and the word", got, err)
	}
	answered := rowsOf(t, dir, "answerCallbackQuery")
	if len(answered) != 1 {
		t.Fatalf("answerCallbackQuery rows = %+v, want the one tap in this chat", answered)
	}
	tapID, _ := answered[0]["callback_query_id"].(string)
	tap := got[0]
	if tapID == "" || tapID == posted.ID || tap.Ref != (channel.MessageRef{ID: "tap-" + tapID, ThreadID: posted.ID}) {
		t.Fatalf("the tap's ref = %+v, want its own id tap-%s in reply to %s", tap.Ref, tapID, posted.ID)
	}
	if tap.ThreadID != posted.ThreadID || tap.Text != "Continue" || tap.UserID != "7001" || !tap.SentAt.IsZero() || tap.UpdateID == 0 || tap.Ack != channel.Cursor(strconv.FormatInt(tap.UpdateID+1, 10)) {
		t.Fatalf("tap = %+v", tap)
	}
	if word := got[1]; word.Text != "stop here" || word.Ref.ThreadID != posted.ID || word.SentAt.IsZero() {
		t.Fatalf("the word reply changed: %+v", word)
	}
	updates := rowsOf(t, dir, "getUpdates")
	if len(updates) != 1 || !reflect.DeepEqual(updates[0]["allowed_updates"], []any{"message", "callback_query"}) {
		t.Fatalf("getUpdates rows = %+v", updates)
	}
}

func TestReceiveKeepsATapWhoseAnswerFails(t *testing.T) {
	t.Parallel()
	answers := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case strings.HasSuffix(request.URL.Path, "/getMe"):
			body = `{"ok":true,"result":{"id":1}}`
		case strings.HasSuffix(request.URL.Path, "/getUpdates"):
			body = `{"ok":true,"result":[{"update_id":9,"callback_query":{"id":"55","from":{"id":7001},"message":{"message_id":3,"date":0,"chat":{"id":1}},"data":"Continue"}}]}`
		case strings.HasSuffix(request.URL.Path, "/answerCallbackQuery"):
			answers++
			return nil, errors.New("query is too old")
		default:
			t.Errorf("unexpected request %s", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}
	d := channel.DestinationConfig{ChannelID: "1", Token: "token", APIBase: "https://telegram.invalid"}
	got, _, err := telegram.New(client).Receive(context.Background(), d, []channel.MessageRef{{ID: "3", ThreadID: "3"}}, "")
	if err != nil || len(got) != 1 || got[0].Text != "Continue" || got[0].Ref.ID != "tap-55" || answers != 1 {
		t.Fatalf("a failed callback answer lost the tap: %+v %v (answers %d)", got, err, answers)
	}
}

func TestClearOptionsTakesTheButtonsAway(t *testing.T) {
	t.Parallel()
	ctx, p, d, dir := bed(t)
	root, err := p.Post(ctx, d, "root", nil)
	if err != nil {
		t.Fatal(err)
	}
	keyboard, err := optionPoster(t, p).PostOptions(ctx, d, "q", []string{"Continue"}, &root)
	if err != nil || keyboard.ID == keyboard.ThreadID {
		t.Fatalf("a keyboard posted as a reply has its own id: %+v %v", keyboard, err)
	}
	if err = optionPoster(t, p).ClearOptions(ctx, d, keyboard); err != nil {
		t.Fatal(err)
	}
	edits := rowsOf(t, dir, "editMessageReplyMarkup")
	want := map[string]any{"chat_id": d.ChannelID, "message_id": float64(mustInt(t, keyboard.ID)), "reply_markup": map[string]any{"inline_keyboard": []any{}}}
	if len(edits) != 1 || !reflect.DeepEqual(edits[0], want) {
		t.Fatalf("editMessageReplyMarkup rows = %+v, want one %+v", edits, want)
	}
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
