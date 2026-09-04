package fake

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/slack"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/telegram"
)

func Provider(dir string) (channel.Provider, channel.DestinationConfig, error) {
	return provider(dir, "slack")
}

func TelegramProvider(dir string) (channel.Provider, channel.DestinationConfig, error) {
	return provider(dir, "telegram")
}

func provider(dir, face string) (channel.Provider, channel.DestinationConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return nil, channel.DestinationConfig{}, channel.ErrUnconfigured("fake not serving")
	}
	base := strings.TrimSpace(string(data))
	if face == "telegram" {
		const token = "fake-telegram-token"
		cfg := channel.DestinationConfig{Name: "fleet", Provider: "fake", APIBase: base, Token: token, ChannelID: "1000", Secrets: []string{token}}
		return telegram.New(nil), cfg, nil
	}
	cfg := channel.DestinationConfig{Name: "fleet", Provider: "fake", APIBase: base, Token: "xoxb-fake", ChannelID: "CFAKE", Secrets: []string{"xoxb-fake"}}
	return slack.New(nil), cfg, nil
}

type scripted struct {
	Face     string  `json:"face"`
	ThreadTS string  `json:"thread_ts"`
	TS       *string `json:"ts"`
	User     any     `json:"user"`
	Text     string  `json:"text"`
	ReplyTo  int64   `json:"reply_to"`
	Chat     int64   `json:"chat"`
	Date     *int64  `json:"date"`
}

type slackAssigned struct {
	scripted
	Timestamp string
}

type telegramAssigned struct {
	Scripted  scripted
	UpdateID  int64
	MessageID int64
	Date      int64
}

type server struct {
	dir              string
	mu               sync.Mutex
	counter          uint64
	lastTSMicros     int64
	loadedLines      int
	slackAssigned    []slackAssigned
	telegramAssigned []telegramAssigned
}

func Serve(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s := &server{dir: dir, counter: 1000000}
	httpServer := &http.Server{Handler: s}
	base := "http://" + ln.Addr().String()
	if err := writeRename(filepath.Join(dir, "base-url"), []byte(base+"\n")); err != nil {
		ln.Close()
		return err
	}
	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(ln) }()
	select {
	case <-ctx.Done():
		_ = httpServer.Shutdown(context.Background())
		<-done
		return nil
	case err := <-done:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func writeRename(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".channel-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (s *server) nextID() int64 { s.counter++; return int64(s.counter) }
func (s *server) nextTS() string {
	micros := time.Now().UnixMicro()
	if micros <= s.lastTSMicros {
		micros = s.lastTSMicros + 1
	}
	s.lastTSMicros = micros
	return fmt.Sprintf("%d.%06d", micros/1_000_000, micros%1_000_000)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/bot") {
		s.serveTelegram(w, r)
		return
	}
	_ = r.ParseForm()
	method := strings.TrimPrefix(r.URL.Path, "/")
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.journal(method, r.Form)
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "chat.postMessage":
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "ts": s.nextTS()})
	case "auth.test":
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "user_id": "UFAKEBOT"})
	case "conversations.replies":
		s.replies(w, r.Form)
	default:
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unknown_method " + r.URL.Path})
	}
}

func (s *server) serveTelegram(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.journal(method, body)
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "sendMessage":
		id := s.nextID()
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": id, "chat": map[string]any{"id": intValue(body["chat_id"])}, "date": time.Now().Unix(), "text": stringValue(body["text"])}})
	case "getUpdates":
		s.telegramUpdates(w, body)
	case "getMe":
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 424242, "is_bot": true, "username": "fakebot"}})
	default:
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "unknown method " + method})
	}
}

func intValue(v any) int64 {
	switch value := v.(type) {
	case float64:
		return int64(value)
	case string:
		n, _ := strconv.ParseInt(value, 10, 64)
		return n
	default:
		return 0
	}
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func (s *server) journal(method string, form any) error {
	f, err := os.OpenFile(filepath.Join(s.dir, "journal.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(map[string]any{"method": method, "form": form}); err == nil {
		err = f.Sync()
	}
	return err
}

func (s *server) loadNew() {
	f, err := os.Open(filepath.Join(s.dir, "replies.jsonl"))
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	index := 0
	for scanner.Scan() {
		if index < s.loadedLines {
			index++
			continue
		}
		var row scripted
		if json.Unmarshal(scanner.Bytes(), &row) == nil {
			switch row.Face {
			case "":
				timestamp := s.nextTS()
				if row.TS != nil {
					timestamp = *row.TS
				}
				s.slackAssigned = append(s.slackAssigned, slackAssigned{scripted: row, Timestamp: timestamp})
			case "telegram":
				if row.Chat == 0 {
					row.Chat = 1000
				}
				date := time.Now().Unix()
				if row.Date != nil {
					date = *row.Date
				}
				s.telegramAssigned = append(s.telegramAssigned, telegramAssigned{Scripted: row, UpdateID: s.nextID(), MessageID: s.nextID(), Date: date})
			}
		}
		index++
	}
	s.loadedLines = index
}

func (s *server) replies(w http.ResponseWriter, form url.Values) {
	s.loadNew()
	root := form.Get("ts")
	oldest := form.Get("oldest")
	limit, _ := strconv.Atoi(form.Get("limit"))
	if limit <= 0 {
		limit = 200
	}
	offset, _ := strconv.Atoi(form.Get("cursor"))
	messages := []map[string]string{{"ts": root, "user": "UFAKEBOT", "text": "root"}}
	for _, row := range s.slackAssigned {
		if row.ThreadTS == root && (oldest == "" || row.Timestamp >= oldest) {
			messages = append(messages, map[string]string{"ts": row.Timestamp, "user": stringValue(row.User), "text": row.Text})
		}
	}
	if offset > len(messages) {
		offset = len(messages)
	}
	end := offset + limit
	if end > len(messages) {
		end = len(messages)
	}
	next := ""
	if end < len(messages) {
		next = strconv.Itoa(end)
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "messages": messages[offset:end], "response_metadata": map[string]string{"next_cursor": next}})
}

func (s *server) telegramUpdates(w http.ResponseWriter, body map[string]any) {
	s.loadNew()
	offset := intValue(body["offset"])
	limit := int(intValue(body["limit"]))
	if limit <= 0 {
		limit = 100
	}
	updates := make([]map[string]any, 0, limit)
	for _, row := range s.telegramAssigned {
		if row.UpdateID < offset {
			continue
		}
		message := map[string]any{"message_id": row.MessageID, "date": row.Date, "text": row.Scripted.Text, "chat": map[string]any{"id": row.Scripted.Chat}, "from": map[string]any{"id": intValue(row.Scripted.User), "is_bot": false}}
		if row.Scripted.ReplyTo != 0 {
			message["reply_to_message"] = map[string]any{"message_id": row.Scripted.ReplyTo}
		}
		updates = append(updates, map[string]any{"update_id": row.UpdateID, "message": message})
		if len(updates) == limit {
			break
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
}
