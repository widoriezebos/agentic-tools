package main

import (
	"io"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The passthrough actions (receipt, experiment, session status, handoff and
// isolate, test plan, list, add, remove, baseline and status) print from
// their own handlers, which have no invocation to ask for a layout: they
// build their page from the stream they write to (output-style §4, G6).

// textEnvCarrier is a stream that carries its own text layout: a golden's
// buffer, laid out the same on every run.
type textEnvCarrier interface{ TextEnv() textui.Env }

// passthroughEnv is the text layout of one stream: the one it carries, or
// its width, colour and symbols as detected, the local clock and zone, and
// repo, the checkout whose paths print repo-relative.
func passthroughEnv(stream io.Writer, repo string, verbose bool) textui.Env {
	var env textui.Env
	switch typed := stream.(type) {
	case textEnvCarrier:
		env = typed.TextEnv()
	case *os.File:
		env = textui.Detect(typed.Fd(), os.Getenv, time.Now(), time.Local)
	default:
		env = textui.DetectWith(false, 0, func(string) string { return "" }, time.Now(), time.Local)
	}
	if _, carried := stream.(textEnvCarrier); !carried {
		env.Home, _ = os.UserHomeDir()
		if repo != "" {
			env.Repo = repo
			if cwd, err := os.Getwd(); err == nil {
				env.InRepo = withinDirectory(cwd, repo)
			}
		}
	}
	env.Verbose = env.Verbose || verbose
	return env
}

// passthroughPage is a new page for one stream.
func passthroughPage(stream io.Writer, repo string, verbose bool) *textui.Page {
	return textui.New(passthroughEnv(stream, repo, verbose))
}

// printPage writes a page to its stream.
func printPage(stream io.Writer, page *textui.Page) {
	_, _ = io.WriteString(stream, page.String())
}
