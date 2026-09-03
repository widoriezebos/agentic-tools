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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/slack"
)

func Provider(dir string) (channel.Provider, channel.DestinationConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "base-url"))
	if err != nil {
		return nil, channel.DestinationConfig{}, channel.ErrUnconfigured("fake not serving")
	}
	base := strings.TrimSpace(string(data))
	if base == "" {
		return nil, channel.DestinationConfig{}, channel.ErrUnconfigured("fake not serving")
	}
	cfg := channel.DestinationConfig{Name: "fleet", Provider: "fake", APIBase: base, Token: "xoxb-fake", ChannelID: "CFAKE", Secrets: []string{"xoxb-fake"}}
	return slack.New(nil), cfg, nil
}

type scripted struct {
	ThreadTS string `json:"thread_ts"`
	User     string `json:"user"`
	Text     string `json:"text"`
}
type assigned struct {
	scripted
	TS string
}
type server struct {
	dir      string
	mu       sync.Mutex
	counter  uint64
	assigned []assigned
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

func (s *server) nextTS() string { s.counter++; return fmt.Sprintf("%d.000000", s.counter) }
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (s *server) journal(method string, form url.Values) error {
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
		var row scripted
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			continue
		}
		if index >= len(s.assigned) {
			s.assigned = append(s.assigned, assigned{scripted: row, TS: s.nextTS()})
		}
		index++
	}
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
	for _, row := range s.assigned {
		if row.ThreadTS == root && (oldest == "" || row.TS >= oldest) {
			messages = append(messages, map[string]string{"ts": row.TS, "user": row.User, "text": row.Text})
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
