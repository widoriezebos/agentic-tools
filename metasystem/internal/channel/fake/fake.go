package fake

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	return provider(dir, "slack", "")
}

func TelegramProvider(dir string, listener ...string) (channel.Provider, channel.DestinationConfig, error) {
	name := ""
	if len(listener) > 0 {
		name = listener[0]
	}
	return provider(dir, "telegram", name)
}

func provider(dir, face, listener string) (channel.Provider, channel.DestinationConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return nil, channel.DestinationConfig{}, channel.ErrUnconfigured("fake not serving")
	}
	base := strings.TrimSpace(string(data))
	if face == "telegram" {
		token := "fake-telegram-token"
		if listener != "" {
			token += "-" + listener
		}
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

type conflictControl struct {
	Listener    string `json:"listener"`
	Remaining   int    `json:"remaining"`
	Description string `json:"description"`
}

type pauseControl struct {
	Listener string `json:"listener"`
	Method   string `json:"method"`
	Until    string `json:"until"`
}

type controls struct {
	Conflict      []conflictControl   `json:"conflict"`
	DeliverOnlyTo map[string][]string `json:"deliverOnlyTo"`
	PauseBefore   []pauseControl      `json:"pauseBefore"`
}

type server struct {
	dir              string
	mu               sync.Mutex
	counter          uint64
	lastTSMicros     int64
	loadedLines      int
	slackAssigned    []slackAssigned
	telegramAssigned []telegramAssigned
	confirmedOffset  int64
	journalSequence  uint64
	controlBytes     string
	controls         controls
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
	w.Header().Set("Content-Type", "application/json")
	if strings.HasPrefix(r.URL.Path, "/bot") {
		s.serveTelegram(w, r)
		return
	}
	_ = r.ParseForm()
	method := strings.TrimPrefix(r.URL.Path, "/")
	pause, conflict, controlledConflict, err := s.requestControls("", method, method)
	if err != nil {
		s.controlError(w, err)
		return
	}
	if !waitForPause(r.Context(), pause) {
		return
	}
	s.record(method, method, "", r.Form)
	if controlledConflict {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": http.StatusConflict, "description": conflict})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
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
	listener, method, ok := telegramRoute(r.URL.Path)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "invalid fake Telegram token or method"})
		return
	}
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	effectiveMethod := method
	if method == "getUpdates" {
		if _, confirming := body["offset"]; confirming {
			effectiveMethod = "confirm"
		}
	}
	pause, conflict, controlledConflict, err := s.requestControls(listener, method, effectiveMethod)
	if err != nil {
		s.controlError(w, err)
		return
	}
	if !waitForPause(r.Context(), pause) {
		return
	}
	s.record(effectiveMethod, method, listener, body)
	if controlledConflict {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": http.StatusConflict, "description": conflict})
		return
	}
	switch method {
	case "sendMessage":
		s.mu.Lock()
		id := s.nextID()
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": id, "chat": map[string]any{"id": intValue(body["chat_id"])}, "date": time.Now().Unix(), "text": stringValue(body["text"])}})
	case "getUpdates":
		s.telegramUpdates(r.Context(), w, body, listener)
	case "getMe":
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 424242, "is_bot": true, "username": "fakebot"}})
	default:
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "description": "unknown method " + method})
	}
}

func telegramRoute(path string) (listener, method string, ok bool) {
	rest := strings.TrimPrefix(path, "/bot")
	slash := strings.IndexByte(rest, '/')
	if slash < 1 || slash == len(rest)-1 || strings.Contains(rest[slash+1:], "/") {
		return "", "", false
	}
	token := rest[:slash]
	method = rest[slash+1:]
	const base = "fake-telegram-token"
	switch {
	case token == base:
		return "", method, true
	case strings.HasPrefix(token, base+"-") && len(token) > len(base)+1:
		return strings.TrimPrefix(token, base+"-"), method, true
	default:
		return "", method, true
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

func (s *server) record(method, raw, listener string, form any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.journal(method, raw, listener, form)
}

func (s *server) journal(method, raw, listener string, form any) error {
	f, err := os.OpenFile(filepath.Join(s.dir, "journal.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	s.journalSequence++
	row := map[string]any{"method": method, "form": form, "listener": listener, "sequence": s.journalSequence}
	if method == "confirm" {
		row["raw"] = raw
	}
	if err = json.NewEncoder(f).Encode(row); err == nil {
		err = f.Sync()
	}
	return err
}

func (s *server) requestControls(listener, rawMethod, effectiveMethod string) (pause, conflict string, controlledConflict bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.reloadControls(); err != nil {
		return "", "", false, err
	}
	for i, item := range s.controls.PauseBefore {
		if item.Listener == listener && item.Method == effectiveMethod {
			pause = item.Until
			s.controls.PauseBefore = append(s.controls.PauseBefore[:i], s.controls.PauseBefore[i+1:]...)
			break
		}
	}
	if rawMethod == "getUpdates" {
		for i := range s.controls.Conflict {
			item := &s.controls.Conflict[i]
			if item.Listener == listener && item.Remaining > 0 {
				item.Remaining--
				conflict = item.Description
				controlledConflict = true
				break
			}
		}
	}
	return pause, conflict, controlledConflict, nil
}

func (s *server) reloadControls() error {
	data, err := os.ReadFile(filepath.Join(s.dir, "control.json"))
	if errors.Is(err, os.ErrNotExist) {
		s.controlBytes = ""
		s.controls = controls{}
		return nil
	}
	if err != nil {
		return err
	}
	if string(data) == s.controlBytes {
		return nil
	}
	var next controls
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	s.controlBytes = string(data)
	s.controls = next
	return nil
}

func (s *server) controlError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error(), "description": err.Error()})
}

func waitForPause(ctx context.Context, until string) bool {
	if until == "" {
		return true
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(until); err == nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
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

func (s *server) telegramUpdates(ctx context.Context, w http.ResponseWriter, body map[string]any, listener string) {
	timeout := time.Duration(intValue(body["timeout"])) * time.Second
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		if err := s.reloadControls(); err != nil {
			s.mu.Unlock()
			s.controlError(w, err)
			return
		}
		updates := s.telegramUpdatesLocked(body, listener)
		s.mu.Unlock()
		if len(updates) > 0 || timeout <= 0 || !time.Now().Before(deadline) {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": updates})
			return
		}
		remaining := time.Until(deadline)
		if remaining > 100*time.Millisecond {
			remaining = 100 * time.Millisecond
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *server) telegramUpdatesLocked(body map[string]any, listener string) []map[string]any {
	s.loadNew()
	offset := s.confirmedOffset
	if rawOffset, present := body["offset"]; present {
		requestedOffset := intValue(rawOffset)
		if requestedOffset > s.confirmedOffset {
			s.confirmedOffset = requestedOffset
		}
		offset = s.confirmedOffset
	}
	limit := int(intValue(body["limit"]))
	if limit <= 0 {
		limit = 100
	}
	updates := make([]map[string]any, 0, limit)
	for _, row := range s.telegramAssigned {
		if row.UpdateID < offset {
			continue
		}
		if listeners, restricted := s.controls.DeliverOnlyTo[strconv.FormatInt(row.UpdateID, 10)]; restricted && !contains(listeners, listener) {
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
	return updates
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
