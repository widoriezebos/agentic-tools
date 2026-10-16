// Package buildlock supplies command-entry barriers for build-lock tests.
package buildlock

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const Root = "TEST_BUILD_LOCK_BARRIER"

// Command runs in place of go, after the caller has acquired its build lock.
// Both callers use this exact barrier, including the wait-candidate setup.
func Command() {
	root := os.Getenv(Root)
	if root == "" || len(os.Args) < 2 || os.Args[1] != "build" {
		return
	}
	directory, _ := os.Getwd()
	record, _ := json.Marshal(map[string]any{"args": os.Args[1:], "directory": directory, "home": os.Getenv("HOME"), "tmp": os.TempDir(), "pid": os.Getpid()})
	if err := os.WriteFile(filepath.Join(root, "entered.tmp"), record, 0o600); err != nil {
		panic(err)
	}
	if err := os.Rename(filepath.Join(root, "entered.tmp"), filepath.Join(root, "entered")); err != nil {
		panic(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "release")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = os.WriteFile(filepath.Join(root, "exited"), nil, 0o600)
	if _, err := os.Stat(filepath.Join(root, "fail")); err == nil {
		os.Exit(73)
	}
	os.Exit(0)
}

// TraceContext acknowledges the slot select and the first blocked flock
// select. Arm it after exec.CommandContext has inspected the context.
type TraceContext struct {
	context.Context
	Root  string
	calls atomic.Int32
}

func (ctx *TraceContext) Arm() { ctx.calls.Store(0) }
func (ctx *TraceContext) Done() <-chan struct{} {
	switch ctx.calls.Add(1) {
	case 1:
		_ = os.WriteFile(filepath.Join(ctx.Root, "slot"), nil, 0o600)
	case 2:
		_ = os.WriteFile(filepath.Join(ctx.Root, "waiting"), nil, 0o600)
	}
	return ctx.Context.Done()
}
