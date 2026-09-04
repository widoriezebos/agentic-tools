package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

const (
	chunkLimit         = 4000
	defaultHTTPTimeout = 30 * time.Second
)

type Adapter struct{ client *http.Client }

func New(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

type chat struct {
	ID int64 `json:"id"`
}

type user struct {
	ID int64 `json:"id"`
}

type message struct {
	MessageID int64    `json:"message_id"`
	Date      int64    `json:"date"`
	Text      string   `json:"text"`
	Chat      chat     `json:"chat"`
	From      user     `json:"from"`
	ReplyTo   *message `json:"reply_to_message"`
}

type wireUpdate struct {
	UpdateID int64   `json:"update_id"`
	Message  message `json:"message"`
}

type Update struct {
	ChatID int64
	UserID int64
	Text   string
}

func (a *Adapter) request(ctx context.Context, dest channel.DestinationConfig, method string, body any, result any, kind channel.ErrorKind) error {
	if dest.APIBase == "" || dest.Token == "" {
		return channel.ErrUnconfigured("telegram bot token and API base are required")
	}
	timeout := dest.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	payload, err := json.Marshal(body)
	if err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(err.Error(), dest.Secrets...)}
	}
	url := strings.TrimRight(dest.APIBase, "/") + "/bot" + dest.Token + "/" + method
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(err.Error(), dest.Secrets...)}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(err.Error(), dest.Secrets...)}
	}
	defer resp.Body.Close()
	var envelope apiResponse
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub("HTTP "+resp.Status+": invalid provider response: "+err.Error(), dest.Secrets...)}
	}
	if resp.StatusCode == http.StatusConflict && method == "getUpdates" {
		switch {
		case strings.Contains(envelope.Description, "terminated by other getUpdates request"):
			return channel.ErrBusy("terminated by other getUpdates request")
		case strings.Contains(envelope.Description, "webhook is active"):
			return channel.ErrReceiveFailed("a webhook is set on this bot; delete it")
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.OK {
		problem := fmt.Sprintf("HTTP %s: %s", resp.Status, envelope.Description)
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(problem, dest.Secrets...)}
	}
	if result != nil && len(envelope.Result) != 0 {
		if err = json.Unmarshal(envelope.Result, result); err != nil {
			return &channel.ProviderError{Kind: kind, Problem: channel.Scrub("invalid provider result: "+err.Error(), dest.Secrets...)}
		}
	}
	return nil
}

func (a *Adapter) Post(ctx context.Context, dest channel.DestinationConfig, text string, thread *channel.MessageRef) (channel.MessageRef, error) {
	if dest.ChannelID == "" || dest.Token == "" || dest.APIBase == "" {
		return channel.MessageRef{}, channel.ErrUnconfigured("telegram destination is incomplete")
	}
	chunks := splitText(text)
	var first channel.MessageRef
	parent := thread
	for i, chunk := range chunks {
		body := map[string]any{"chat_id": dest.ChannelID, "text": chunk}
		if parent != nil {
			id, err := strconv.ParseInt(parent.ID, 10, 64)
			if err != nil {
				return channel.MessageRef{}, channel.ErrSendFailed("invalid Telegram reply message id")
			}
			body["reply_parameters"] = map[string]any{"message_id": id}
		}
		var out message
		if err := a.request(ctx, dest, "sendMessage", body, &out, channel.SendFailed); err != nil {
			if i > 0 {
				return channel.MessageRef{}, channel.ErrSendFailed(fmt.Sprintf("chunk %d: %s", i+1, channel.Scrub(err.Error(), dest.Secrets...)))
			}
			return channel.MessageRef{}, err
		}
		root := strconv.FormatInt(out.MessageID, 10)
		if thread != nil {
			root = thread.ThreadID
			if root == "" {
				root = thread.ID
			}
		}
		ref := channel.MessageRef{ID: strconv.FormatInt(out.MessageID, 10), ThreadID: root}
		if i == 0 {
			first = ref
		}
		parent = &ref
	}
	return first, nil
}

func splitText(text string) []string {
	if text == "" {
		return []string{""}
	}
	var chunks []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		for utf8.RuneCountInString(line) > chunkLimit {
			flush()
			runes := []rune(line)
			chunks = append(chunks, string(runes[:chunkLimit]))
			line = string(runes[chunkLimit:])
		}
		if utf8.RuneCountInString(current.String())+utf8.RuneCountInString(line) > chunkLimit {
			flush()
		}
		current.WriteString(line)
	}
	flush()
	return chunks
}

func (a *Adapter) Receive(ctx context.Context, dest channel.DestinationConfig, refs []channel.MessageRef, after channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	if dest.ChannelID == "" || dest.Token == "" || dest.APIBase == "" {
		return nil, after, channel.ErrUnconfigured("telegram destination is incomplete")
	}
	identity, err := a.Credential(ctx, dest)
	if err != nil {
		return nil, after, err
	}
	updates, next, err := a.updates(ctx, dest, after, true, 0)
	if err != nil {
		return nil, after, err
	}
	roots := map[string]string{}
	for _, ref := range refs {
		root := ref.ThreadID
		if root == "" {
			root = ref.ID
		}
		roots[ref.ID] = root
	}
	var inbound []channel.Inbound
	for _, update := range updates {
		m := update.Message
		if strconv.FormatInt(m.Chat.ID, 10) != dest.ChannelID || strconv.FormatInt(m.From.ID, 10) == identity.UserID {
			continue
		}
		parent := ""
		root := ""
		if m.ReplyTo != nil {
			parent = strconv.FormatInt(m.ReplyTo.MessageID, 10)
			root = roots[parent]
		}
		ack := channel.Cursor(strconv.FormatInt(update.UpdateID+1, 10))
		inbound = append(inbound, channel.Inbound{Ref: channel.MessageRef{ID: strconv.FormatInt(m.MessageID, 10), ThreadID: parent}, ThreadID: root, UserID: strconv.FormatInt(m.From.ID, 10), Text: m.Text, SentAt: time.Unix(m.Date, 0).UTC(), Ack: ack, UpdateID: update.UpdateID})
	}
	return inbound, next, nil
}

func (a *Adapter) updates(ctx context.Context, dest channel.DestinationConfig, after channel.Cursor, includePollingFields bool, longPollSeconds int) ([]wireUpdate, channel.Cursor, error) {
	body := map[string]any{}
	if after != "" {
		offset, err := strconv.ParseInt(string(after), 10, 64)
		if err != nil {
			return nil, after, channel.ErrReceiveFailed("invalid Telegram cursor")
		}
		body["offset"] = offset
	}
	if includePollingFields {
		body["limit"] = 100
		body["timeout"] = longPollSeconds
		body["allowed_updates"] = []string{"message"}
	}
	if longPollSeconds > 0 {
		dest.HTTPTimeout = time.Duration(longPollSeconds+15) * time.Second
	}
	var updates []wireUpdate
	if err := a.request(ctx, dest, "getUpdates", body, &updates, channel.ReceiveFailed); err != nil {
		return nil, after, err
	}
	next := after
	for _, update := range updates {
		candidate := channel.Cursor(strconv.FormatInt(update.UpdateID+1, 10))
		if next == "" || cursorNumber(candidate) > cursorNumber(next) {
			next = candidate
		}
	}
	return updates, next, nil
}

func (a *Adapter) Confirm(ctx context.Context, dest channel.DestinationConfig, cursor channel.Cursor) error {
	if cursor == "" {
		return nil
	}
	offset, err := strconv.ParseInt(string(cursor), 10, 64)
	if err != nil {
		return channel.ErrReceiveFailed("invalid Telegram cursor")
	}
	body := map[string]any{"offset": offset, "timeout": 0, "limit": 1}
	var updates []wireUpdate
	return a.request(ctx, dest, "getUpdates", body, &updates, channel.ReceiveFailed)
}

func cursorNumber(cursor channel.Cursor) int64 {
	n, _ := strconv.ParseInt(string(cursor), 10, 64)
	return n
}

func (a *Adapter) Credential(ctx context.Context, dest channel.DestinationConfig) (channel.CredentialIdentity, error) {
	var out user
	if err := a.request(ctx, dest, "getMe", map[string]any{}, &out, channel.ReceiveFailed); err != nil {
		return channel.CredentialIdentity{}, err
	}
	if out.ID == 0 {
		return channel.CredentialIdentity{}, channel.ErrReceiveFailed("getMe returned no bot identity")
	}
	return channel.CredentialIdentity{UserID: strconv.FormatInt(out.ID, 10)}, nil
}

func (a *Adapter) Peek(ctx context.Context, dest channel.DestinationConfig) ([]Update, error) {
	updates, _, err := a.updates(ctx, dest, "", false, 0)
	if err != nil {
		return nil, err
	}
	out := make([]Update, 0, len(updates))
	for _, update := range updates {
		out = append(out, Update{ChatID: update.Message.Chat.ID, UserID: update.Message.From.ID, Text: update.Message.Text})
	}
	return out, nil
}

var _ channel.Provider = (*Adapter)(nil)
