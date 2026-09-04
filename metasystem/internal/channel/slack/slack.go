package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

type Adapter struct{ client *http.Client }

func New(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

type response struct {
	OK       bool                              `json:"ok"`
	Error    string                            `json:"error"`
	TS       string                            `json:"ts"`
	UserID   string                            `json:"user_id"`
	Messages []struct{ TS, User, Text string } `json:"messages"`
	Metadata struct {
		Next string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (a *Adapter) call(ctx context.Context, dest channel.DestinationConfig, method string, form url.Values, out *response, kind channel.ErrorKind) error {
	if dest.APIBase == "" || dest.ChannelID == "" || dest.Token == "" {
		return channel.ErrUnconfigured("slack destination is incomplete")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(dest.APIBase, "/")+"/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(err.Error(), dest.Secrets...)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+dest.Token)
	resp, err := a.client.Do(req)
	if err != nil {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(err.Error(), append(dest.Secrets, dest.Token)...)}
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &channel.ProviderError{Kind: kind, Problem: "invalid provider response"}
	}
	if !out.OK {
		return &channel.ProviderError{Kind: kind, Problem: channel.Scrub(out.Error, append(dest.Secrets, dest.Token)...)}
	}
	return nil
}

func (a *Adapter) Post(ctx context.Context, dest channel.DestinationConfig, text string, thread *channel.MessageRef) (channel.MessageRef, error) {
	f := url.Values{"channel": {dest.ChannelID}, "text": {text}}
	root := ""
	if thread != nil {
		root = thread.ThreadID
		if root == "" {
			root = thread.ID
		}
		f.Set("thread_ts", root)
	}
	var out response
	if err := a.call(ctx, dest, "chat.postMessage", f, &out, channel.SendFailed); err != nil {
		return channel.MessageRef{}, err
	}
	if root == "" {
		root = out.TS
	}
	return channel.MessageRef{ID: out.TS, ThreadID: root}, nil
}

func decodeCursor(c channel.Cursor) map[string]string {
	m := map[string]string{}
	if c != "" {
		_ = json.Unmarshal([]byte(c), &m)
	}
	return m
}
func encodeCursor(m map[string]string) channel.Cursor {
	b, _ := json.Marshal(m)
	return channel.Cursor(b)
}

func (a *Adapter) Receive(ctx context.Context, dest channel.DestinationConfig, threads []channel.MessageRef, after channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	last := decodeCursor(after)
	next := make(map[string]string, len(last))
	for k, v := range last {
		next[k] = v
	}
	cred, err := a.Credential(ctx, dest)
	if err != nil {
		return nil, after, err
	}
	var inbound []channel.Inbound
	seenRoots := map[string]bool{}
	for _, thread := range threads {
		root := thread.ThreadID
		if root == "" {
			root = thread.ID
		}
		if seenRoots[root] {
			continue
		}
		seenRoots[root] = true
		page := ""
		for {
			f := url.Values{"channel": {dest.ChannelID}, "ts": {root}, "limit": {"200"}}
			if last[root] != "" {
				f.Set("oldest", last[root])
			}
			if page != "" {
				f.Set("cursor", page)
			}
			var out response
			if err := a.call(ctx, dest, "conversations.replies", f, &out, channel.ReceiveFailed); err != nil {
				return nil, after, err
			}
			for _, m := range out.Messages {
				if m.TS > next[root] {
					next[root] = m.TS
				}
				if m.TS <= last[root] || m.TS == root || m.User == cred.UserID {
					continue
				}
				sentAt := time.Time{}
				if seconds, parseErr := strconv.ParseFloat(m.TS, 64); parseErr == nil {
					sentAt = time.Unix(0, int64(seconds*float64(time.Second))).UTC()
				}
				inbound = append(inbound, channel.Inbound{Ref: channel.MessageRef{ID: m.TS, ThreadID: root}, ThreadID: root, UserID: m.User, Text: m.Text, SentAt: sentAt})
			}
			page = out.Metadata.Next
			if page == "" {
				break
			}
		}
	}
	sort.SliceStable(inbound, func(i, j int) bool { return inbound[i].Ref.ID < inbound[j].Ref.ID })
	return inbound, encodeCursor(next), nil
}

func (a *Adapter) Credential(ctx context.Context, dest channel.DestinationConfig) (channel.CredentialIdentity, error) {
	var out response
	if err := a.call(ctx, dest, "auth.test", url.Values{}, &out, channel.ReceiveFailed); err != nil {
		return channel.CredentialIdentity{}, err
	}
	if out.UserID == "" {
		return channel.CredentialIdentity{}, channel.ErrReceiveFailed("auth.test returned no user identity")
	}
	return channel.CredentialIdentity{UserID: out.UserID}, nil
}

var _ channel.Provider = (*Adapter)(nil)
var _ = fmt.Sprintf
