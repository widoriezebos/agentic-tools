package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// These verbs are the per-runtime half of the adapter family: the small
// transformations a claude, codex, devin, or fake delegate turn asks for
// around its CLI invocation. They complement the runtime-neutral adapter
// verbs (root-job, the effective-permissions handshake, the patch and
// snapshot writers).

var (
	toolGateProcessBirth = identity.ProcessBirth
	toolGateClock        = time.Now
)

// runAdapterClaudeToolGate decides one Claude PreToolUse call. Once flags are
// valid the hook fails open: every diagnostic path exits successfully.
func runAdapterClaudeToolGate(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("adapter claude-tool-gate", stdout, stderr)
	var root string
	pathFlagVar(flags, &root, "root", "", "installation or containing template root")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "root") {
		return 2
	}
	if root == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal adapter claude-tool-gate --root ROOT")
		return 2
	}
	stateRoot, err := goal.ResolveStateRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, "metasystem internal adapter claude-tool-gate:", err)
		return 0
	}
	mode, err := config.ToolGateMode(stateRoot)
	if err != nil {
		fmt.Fprintln(stderr, "metasystem internal adapter claude-tool-gate:", err)
		return 0
	}
	memoryDir, _ := usagepkg.MemoryDirectory(usagepkg.ReadOptions{Installation: stateRoot})
	startedAt, _ := toolGateProcessBirth(int64(os.Getpid()))
	err = adapter.RunToolGate(adapter.ToolGateOptions{
		ShellStartedAt: startedAt, Clock: toolGateClock, MemoryDir: memoryDir, Mode: mode,
		StateRoot: stateRoot, Installation: stateRoot,
		Stdin: os.Stdin, Stdout: stdout, Stderr: stderr,
	})
	if err != nil {
		fmt.Fprintln(stderr, "metasystem internal adapter claude-tool-gate:", err)
	}
	return 0
}

// runAdapterClaudeSessionSignal is the SessionStart hook helper: it reads the
// hook payload from stdin, writes the session signal and a session-init event
// from the env-named paths, and echoes the session id as runtime context.
func runAdapterClaudeSessionSignal(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		return refuseUnknownOption(stdout, stderr, "adapter claude-session-signal", args[0], "it reads the session-start payload on standard input and takes no options")
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: metasystem internal adapter claude-session-signal < session-start-payload")
		return 2
	}
	signalPath := os.Getenv("METASYSTEM_CLAUDE_SESSION_SIGNAL")
	eventsPath := os.Getenv("METASYSTEM_CLAUDE_EVENTS")
	if signalPath == "" || eventsPath == "" {
		fmt.Fprintln(stderr, "adapter claude-session-signal: METASYSTEM_CLAUDE_SESSION_SIGNAL and METASYSTEM_CLAUDE_EVENTS are required")
		return 1
	}
	sessionID, err := adapter.ClaudeSessionSignal(os.Stdin, signalPath, eventsPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Metasystem runtime session id: %s\n", sessionID)
	return 0
}
