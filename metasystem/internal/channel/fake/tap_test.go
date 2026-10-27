package fake_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTelegramFaceDeliversATapAndAnswersItsMethods(t *testing.T) {
	t.Parallel()
	_, dir, stop := serverBed(t)
	defer stop()
	base, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, body string) map[string]any {
		t.Helper()
		resp, err := http.Post(strings.TrimSpace(string(base))+"/botfake-telegram-token/"+method, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	appendLine(t, dir, `{"face":"telegram","user":7001,"tap":"Continue","reply_to":5}`)
	appendLine(t, dir, `{"face":"telegram","user":7001,"tap":"Stop here","reply_to":5}`)
	got := call("getUpdates", `{"limit":1}`)
	updates, _ := got["result"].([]any)
	if len(updates) != 1 {
		t.Fatalf("getUpdates with limit 1 = %+v", got)
	}
	update := updates[0].(map[string]any)
	query, _ := update["callback_query"].(map[string]any)
	want := map[string]any{"id": "1000002", "from": map[string]any{"id": float64(7001), "is_bot": false}, "data": "Continue",
		"message": map[string]any{"message_id": float64(5), "date": float64(0), "chat": map[string]any{"id": float64(1000)}}}
	if update["update_id"] != float64(1000001) || update["message"] != nil || !reflect.DeepEqual(query, want) {
		t.Fatalf("a tap row became %+v, want callback_query %+v", update, want)
	}
	for _, method := range []string{"answerCallbackQuery", "editMessageReplyMarkup"} {
		if out := call(method, `{}`); out["ok"] != true || out["result"] != true {
			t.Fatalf("%s answered %+v", method, out)
		}
	}
	rows := journalRows(t, dir)
	if n := len(rows); n != 3 || rows[1]["method"] != "answerCallbackQuery" || rows[2]["method"] != "editMessageReplyMarkup" {
		t.Fatalf("journal = %+v", rows)
	}
}
