package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// These verbs are the per-runtime half of the adapter family: the small
// transformations a claude, codex, or devin delegate turn asks for around its
// CLI invocation. They complement the runtime-neutral adapter verbs (root-job,
// the effective-permissions handshake, the patch and snapshot writers).

// runAdapterVersionParse prints the first dotted version token read from stdin,
// the way each runtime's --version output is turned into a config identity.
func runAdapterVersionParse(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter version-parse < cli-version-output")
		return 2
	}
	version, err := adapter.ParseCLIVersion(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(version)
	return 0
}

// runAdapterCodexEvent prints a session or turn id from Codex's JSONL event
// stream, exiting 1 when no such id has been emitted yet.
func runAdapterCodexEvent(args []string) int {
	flags := flag.NewFlagSet("adapter codex-event", flag.ContinueOnError)
	events := flags.String("events", "", "codex events JSONL file")
	field := flags.String("field", "", "session or turn")
	if flags.Parse(args) != nil {
		return 2
	}
	if *events == "" || (*field != "session" && *field != "turn") {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter codex-event --events FILE --field session|turn")
		return 2
	}
	value, ok := adapter.CodexEventField(*events, *field)
	if !ok {
		return 1
	}
	fmt.Println(value)
	return 0
}

// runAdapterCodexUsage writes the typed usage for a Codex turn from its event
// stream.
func runAdapterCodexUsage(args []string) int {
	flags := flag.NewFlagSet("adapter codex-usage", flag.ContinueOnError)
	events := flags.String("events", "", "codex events JSONL file")
	output := flags.String("output", "", "typed usage output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *events == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter codex-usage --events FILE --output FILE")
		return 2
	}
	if err := adapter.CodexUsage(*events, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterCodexCommand prints the argv for a Codex delegate turn, one token
// per NUL so a shell can read it back into an array without requoting.
func runAdapterCodexCommand(args []string) int {
	flags := flag.NewFlagSet("adapter codex-command", flag.ContinueOnError)
	verb := flags.String("verb", "", "dispatch or follow-up")
	model := flags.String("model", "", "requested model")
	workspace := flags.String("workspace", "", "workspace directory (dispatch)")
	schema := flags.String("schema", "", "output schema file")
	output := flags.String("output", "", "structured output file")
	sandbox := flags.String("sandbox", "", "sandbox mode")
	network := flags.String("network", "", "network access boolean (true|false)")
	session := flags.String("session", "", "session to resume (follow-up)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *verb == "" || *model == "" || *schema == "" || *output == "" || *sandbox == "" || *network == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter codex-command --verb V --model M --schema F --output F --sandbox M --network B [--workspace DIR] [--session SID]")
		return 2
	}
	command, err := adapter.BuildCodexCommand(*verb, *model, *workspace, *schema, *output, *sandbox, *network, *session)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, token := range command {
		fmt.Print(token)
		os.Stdout.Write([]byte{0})
	}
	return 0
}

// runAdapterClaudeSettings writes the settings file a Claude delegate turn
// launches under.
func runAdapterClaudeSettings(args []string) int {
	flags := flag.NewFlagSet("adapter claude-settings", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	output := flags.String("output", "", "settings output file")
	bin := flags.String("metasystem-bin", "", "metasystem binary the session-signal hook runs")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *output == "" || *bin == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-settings --record FILE --output FILE --metasystem-bin PATH")
		return 2
	}
	if err := adapter.BuildClaudeSettings(*record, *output, *bin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterClaudeUsage writes the typed usage for a Claude turn from its result
// document.
func runAdapterClaudeUsage(args []string) int {
	flags := flag.NewFlagSet("adapter claude-usage", flag.ContinueOnError)
	result := flags.String("result", "", "claude result document")
	output := flags.String("output", "", "typed usage output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-usage --result FILE --output FILE")
		return 2
	}
	if err := adapter.ClaudeUsage(*result, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterClaudeResultField prints a field from a Claude result document, with
// the modelUsage collapse for the model field.
func runAdapterClaudeResultField(args []string) int {
	flags := flag.NewFlagSet("adapter claude-result-field", flag.ContinueOnError)
	result := flags.String("result", "", "claude result document")
	field := flags.String("field", "", "field to read")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *field == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-result-field --result FILE --field NAME")
		return 2
	}
	value, print, err := adapter.ClaudeResultField(*result, *field)
	if err != nil {
		return 1
	}
	if print {
		fmt.Println(value)
	}
	return 0
}

// runAdapterClaudeReadRoots prints the requested read roots other than the
// workspace root, one per line.
func runAdapterClaudeReadRoots(args []string) int {
	flags := flag.NewFlagSet("adapter claude-read-roots", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-read-roots --record FILE")
		return 2
	}
	roots, err := adapter.ClaudeReadRoots(*record)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, root := range roots {
		fmt.Println(root)
	}
	return 0
}

// runAdapterClaudeAppendResult appends a Claude result document to the flight
// recorder as one compact event line.
func runAdapterClaudeAppendResult(args []string) int {
	flags := flag.NewFlagSet("adapter claude-append-result", flag.ContinueOnError)
	result := flags.String("result", "", "claude result document")
	events := flags.String("events", "", "events JSONL file to append to")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *events == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-append-result --result FILE --events FILE")
		return 2
	}
	if err := adapter.ClaudeAppendResult(*result, *events); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterClaudeSessionSignal is the SessionStart hook helper: it reads the
// hook payload from stdin, writes the session signal and a session-init event
// from the env-named paths, and echoes the session id as runtime context.
func runAdapterClaudeSessionSignal(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-session-signal < session-start-payload")
		return 2
	}
	signalPath := os.Getenv("METASYSTEM_CLAUDE_SESSION_SIGNAL")
	eventsPath := os.Getenv("METASYSTEM_CLAUDE_EVENTS")
	if signalPath == "" || eventsPath == "" {
		fmt.Fprintln(os.Stderr, "adapter claude-session-signal: METASYSTEM_CLAUDE_SESSION_SIGNAL and METASYSTEM_CLAUDE_EVENTS are required")
		return 1
	}
	sessionID, err := adapter.ClaudeSessionSignal(os.Stdin, signalPath, eventsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("Metasystem runtime session id: %s\n", sessionID)
	return 0
}

// runAdapterDevinConfig writes a Devin delegate's job config and its provenance
// from the job's requested permissions.
func runAdapterDevinConfig(args []string) int {
	flags := flag.NewFlagSet("adapter devin-config", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	output := flags.String("output", "", "config output file")
	provenance := flags.String("provenance", "", "config provenance output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *output == "" || *provenance == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-config --record FILE --output FILE --provenance FILE")
		return 2
	}
	if err := adapter.BuildDevinConfig(*record, *output, *provenance); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterDevinSession correlates this turn's Devin session against the
// pre-launch baseline. It prints the id and exits 0, exits 1 when none is found
// yet, and exits 2 (naming the candidates) when the correlation is ambiguous.
func runAdapterDevinSession(args []string) int {
	flags := flag.NewFlagSet("adapter devin-session", flag.ContinueOnError)
	before := flags.String("before", "", "pre-launch session listing")
	current := flags.String("current", "", "current session listing")
	signal := flags.String("signal", "", "hook session signal file")
	workspace := flags.String("workspace", "", "workspace to scope the correlation to")
	if flags.Parse(args) != nil {
		return 2
	}
	if *before == "" || *current == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-session --before FILE --current FILE [--signal FILE] [--workspace DIR]")
		return 2
	}
	id, candidates := adapter.DevinSessionCorrelate(*before, *current, *signal, *workspace)
	if id != "" {
		fmt.Println(id)
		return 0
	}
	if len(candidates) > 1 {
		fmt.Fprintln(os.Stderr, "ambiguous-session-correlation:"+strings.Join(candidates, ","))
		return 2
	}
	return 1
}

// runAdapterDevinUsage derives this turn's typed usage from Devin's cumulative
// session metrics, publishing the delta against its predecessor's totals.
func runAdapterDevinUsage(args []string) int {
	flags := flag.NewFlagSet("adapter devin-usage", flag.ContinueOnError)
	usage := flags.String("usage", "", "typed usage output file")
	transcript := flags.String("transcript", "", "Devin transcript with final_metrics")
	cumulative := flags.String("cumulative", "", "this turn's cumulative totals output file")
	previous := flags.String("previous", "", "predecessor's cumulative totals file (empty when none)")
	expectPrevious := flags.Bool("expect-previous", false, "the turn resumes a session and must find a predecessor")
	if flags.Parse(args) != nil {
		return 2
	}
	if *usage == "" || *transcript == "" || *cumulative == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-usage --usage FILE --transcript FILE --cumulative FILE [--previous FILE] [--expect-previous]")
		return 2
	}
	if err := adapter.DevinUsage(*usage, *transcript, *cumulative, *previous, *expectPrevious); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterUsageUnavailable writes the typed-usage record for a turn whose
// spend cannot be trusted as complete.
func runAdapterUsageUnavailable(args []string) int {
	flags := flag.NewFlagSet("adapter usage-unavailable", flag.ContinueOnError)
	output := flags.String("output", "", "typed usage output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter usage-unavailable --output FILE")
		return 2
	}
	if err := adapter.WriteUnavailableUsage(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
