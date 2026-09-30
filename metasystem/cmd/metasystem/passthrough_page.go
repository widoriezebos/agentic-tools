package main

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
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

// refusePassthrough prints a refusal's two lines, laid out, on stderr and
// returns its exit code.
func refusePassthrough(stderr io.Writer, code int, line1 string, hint textui.Hint) int {
	page := passthroughPage(stderr, "", false)
	page.Refusal(shortPaths(page.Env(), line1), hint)
	printPage(stderr, page)
	return code
}

// printOwnerLines lays out the lines an owner answered with: its first
// output line is the headline and the rest are rows under it; its first
// error line is a refusal's line 1, a "run: " line after it the hint (else
// fallback), the rest rows. Paths print as a person reads them, counts as
// counted nouns.
func printOwnerLines(stdout, stderr io.Writer, repo string, out, errs []string, code int, fallback textui.Hint) int {
	if len(out) > 0 {
		page := passthroughPage(stdout, repo, false)
		env := page.Env()
		page.Headline(diskstore.CountedNouns(shortPaths(env, out[0])))
		if len(out) > 1 {
			section := page.Section("", "")
			for _, line := range out[1:] {
				section.Text(diskstore.CountedNouns(shortPaths(env, strings.TrimSpace(line))))
			}
		}
		printPage(stdout, page)
	}
	if len(errs) > 0 {
		page := passthroughPage(stderr, repo, false)
		env := page.Env()
		var hint textui.Hint
		var rest []string
		for _, line := range errs[1:] {
			if remedy, isRemedy := strings.CutPrefix(line, "run: "); isRemedy && hint.Reason == "" {
				hint.Reason = shortPaths(env, remedy)
				continue
			}
			rest = append(rest, line)
		}
		first, remedy, twoLines := strings.Cut(errs[0], "\nrun: ")
		if twoLines && hint.Reason == "" {
			hint.Reason = shortPaths(env, remedy)
		}
		if hint.Reason == "" {
			hint = fallback
		}
		page.Refusal(diskstore.CountedNouns(shortPaths(env, first)), hint)
		if len(rest) > 0 {
			section := page.Section("", "")
			for _, line := range rest {
				section.Text(shortPaths(env, strings.TrimSpace(line)))
			}
		}
		printPage(stderr, page)
	}
	return code
}
