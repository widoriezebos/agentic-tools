package slack_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
)

func bed(t *testing.T) (context.Context, context.CancelFunc, channel.Provider, channel.DestinationConfig, string) {
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
	p, d, err := fake.Provider(dir)
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
	return ctx, cancel, p, d, dir
}
func TestPostRootAndThreaded(t *testing.T) {
	ctx, _, p, d, dir := bed(t)
	root, err := p.Post(ctx, d, "root", nil)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := p.Post(ctx, d, "reply", &root)
	if err != nil || reply.ThreadID != root.ID {
		t.Fatal(reply, err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "journal.jsonl"))
	if strings.Count(string(b), `"method":"chat.postMessage"`) != 2 || !strings.Contains(string(b), "thread_ts") {
		t.Fatal(string(b))
	}
}
func TestReceivePagesAndFiltersByCursor(t *testing.T) {
	ctx, _, p, d, dir := bed(t)
	root, err := p.Post(ctx, d, "root", nil)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < 205; i++ {
		fmt.Fprintf(&b, "{\"thread_ts\":%q,\"user\":\"U%d\",\"text\":\"r%d\"}\n", root.ID, i, i)
	}
	if err = os.WriteFile(filepath.Join(dir, "replies.jsonl"), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	got, cursor, err := p.Receive(ctx, d, []channel.MessageRef{root}, "")
	if err != nil || len(got) != 205 || cursor == "" || got[0].SentAt.IsZero() {
		t.Fatalf("got=%d cursor=%q err=%v", len(got), cursor, err)
	}
	again, _, err := p.Receive(ctx, d, []channel.MessageRef{root}, cursor)
	if err != nil || len(again) != 0 {
		t.Fatalf("inclusive oldest redelivered %d: %v", len(again), err)
	}
}
func TestCredential(t *testing.T) {
	ctx, _, p, d, _ := bed(t)
	id, err := p.Credential(ctx, d)
	if err != nil || id.UserID != "UFAKEBOT" {
		t.Fatal(id, err)
	}
}
func TestUnconfiguredIsTyped(t *testing.T) {
	_, _, err := fake.Provider(t.TempDir())
	if err == nil || !channel.IsKind(err, channel.Unconfigured) {
		t.Fatalf("not typed: %v", err)
	}
	if !errors.As(err, new(*channel.ProviderError)) {
		t.Fatal(err)
	}
}

func TestSecretsScrubbedFromErrors(t *testing.T) {
	ctx, _, p, d, _ := bed(t)
	d.APIBase = strings.TrimRight(d.APIBase, "/") + "/" + d.Token
	_, err := p.Post(ctx, d, "fail", nil)
	if err == nil || strings.Contains(err.Error(), d.Token) {
		t.Fatalf("adapter error exposed token: %v", err)
	}
}
