package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// These verbs are the per-runtime half of the adapter family: the small
// transformations a claude, codex, devin, or fake delegate turn asks for
// around its CLI invocation. They complement the runtime-neutral adapter
// verbs (root-job, the effective-permissions handshake, the patch and
// snapshot writers).

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
	sandbox := flags.String("sandbox", "", "sandbox mode (or derive via --record/--permissions)")
	network := flags.String("network", "", "network access boolean (or derive via --record/--permissions)")
	session := flags.String("session", "", "session to resume (follow-up)")
	record := flags.String("record", "", "job record: derive sandbox/network from its requested envelope")
	permissions := flags.String("permissions", "", "permission envelope file: derive sandbox/network from it")
	if flags.Parse(args) != nil {
		return 2
	}
	var extraDirs []string
	if *record != "" || *permissions != "" {
		derivedSandbox, derivedNetwork, err := adapter.CodexPermissionSettings(*permissions, *record)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		*sandbox, *network = derivedSandbox, derivedNetwork
		// Write roots outside the workspace — the worktree's git
		// metadata (issue #5) — must reach the sandbox explicitly.
		roots, err := adapter.CodexExtraWriteRoots(*permissions, *record, *workspace)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		extraDirs = roots
	}
	if *verb == "" || *model == "" || *schema == "" || *output == "" || *sandbox == "" || *network == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter codex-command --verb V --model M --schema F --output F (--record F | --permissions F | --sandbox M --network B) [--workspace DIR] [--session SID]")
		return 2
	}
	command, err := adapter.BuildCodexCommand(*verb, *model, *workspace, *schema, *output, *sandbox, *network, *session, extraDirs)
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

// runAdapterClaudeCommand relays `adapter claude-command`:
// one home for the Claude argv, the permission
// envelope's mode/tool mapping, and the native budget policy, emitted
// NUL-separated exactly like codex-command. Exit 3 is an invalid budget,
// exit 4 an invalid turn limit — the adapter maps them to its two protocol
// errors; the host keeps its one message.
func runAdapterClaudeCommand(args []string) int {
	flags := flag.NewFlagSet("adapter claude-command", flag.ContinueOnError)
	record := flags.String("record", "", "job record (empty for a host turn)")
	model := flags.String("model", "", "model to launch")
	schema := flags.String("schema", "", "return schema file (compacted into the argv)")
	settings := flags.String("settings", "", "settings file (adapter turns)")
	session := flags.String("session", "", "session to resume (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *model == "" || *schema == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter claude-command --model M --schema F [--record F] [--settings F] [--session SID]")
		return 2
	}
	budget, turns, err := adapter.ClaudeBudget(os.LookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if err.Error() == "invalid_native_turn_limit" {
			return 4
		}
		return 3
	}
	schemaBytes, err := os.ReadFile(*schema)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, schemaBytes); err != nil {
		fmt.Fprintf(os.Stderr, "schema is not valid JSON: %v\n", err)
		return 1
	}
	command, err := adapter.BuildClaudeCommand(*record, *model, compacted.String(), *settings, *session, budget, turns)
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
		// Named on stderr like every sibling adapter verb: this runs inside
		// hook plumbing where stderr is the only diagnostic channel.
		fmt.Fprintln(os.Stderr, err)
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
	// The permission mode rides on stdout beside the config: the
	// launch argv consumes the same envelope decision the config was
	// built from.
	mode, err := adapter.DevinPermissionMode(*record)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(mode)
	return 0
}

// runAdapterAdjudicateTurn relays `adapter adjudicate-turn`: the terminal-
// outcome state machine lives in adapter.AdjudicateTurn. Pure decision —
// the CAS stays in the shell wrappers riding dispatch.sh's lease-held
// re-exec.
func runAdapterAdjudicateTurn(args []string) int {
	flags := flag.NewFlagSet("adapter adjudicate-turn", flag.ContinueOnError)
	var p adapter.AdjudicateParams
	flags.StringVar(&p.Stage, "stage", "", "initial, after-repair, settle-result, or empty-reply")
	flags.StringVar(&p.Root, "root", "", "checkout root")
	flags.StringVar(&p.Job, "job", "", "job id")
	flags.StringVar(&p.RecordPath, "record", "", "job record file")
	flags.StringVar(&p.SessionID, "session", "", "correlated session id")
	flags.StringVar(&p.SchemaPath, "schema", "", "return schema file")
	flags.StringVar(&p.CandidatePath, "candidate", "", "raw reply file")
	flags.StringVar(&p.TranscriptPath, "transcript", "", "transcript file (optional)")
	flags.StringVar(&p.ReturnPath, "return", "", "round return.json output")
	flags.StringVar(&p.MarkdownPath, "markdown", "", "round return.md output")
	flags.StringVar(&p.ViolationPath, "violation", "", "violation output file")
	flags.StringVar(&p.RepairPromptPath, "repair-prompt", "", "repair prompt output file")
	flags.StringVar(&p.NamedRepairPath, "named-repair-path", "", "the repair attempt's named return file (empty-delivery)")
	flags.StringVar(&p.LogPath, "log", "", "runtime CLI stderr log (provider-overload evidence)")
	flags.Int64Var(&p.CLIStatus, "cli-status", 0, "adapter CLI exit status")
	flags.BoolVar(&p.HandshakeDone, "handshake-done", false, "the session correlated")
	flags.BoolVar(&p.RepairAvailable, "repair-available", false, "a bounded repair turn may run")
	flags.Int64Var(&p.RepairRC, "repair-rc", 0, "repair turn exit status")
	flags.StringVar(&p.RepairCandidate, "repair-candidate", "", "repair reply file")
	flags.BoolVar(&p.SettleAvailable, "settle-available", false, "a settle hook exists")
	flags.BoolVar(&p.SettleOK, "ok", false, "the settle hook agreed")
	if flags.Parse(args) != nil {
		return 2
	}
	if p.Stage == "" {
		fmt.Fprintln(os.Stderr, "adapter adjudicate-turn: --stage is required")
		return 2
	}
	verdict, err := adapter.AdjudicateTurn(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(verdict)
	return 0
}

// runAdapterDevinSettle relays `adapter devin-settle`: the
// transcript-vs-correlated-session certification and the
// effective-model fallback. Prints the derived model
// (nothing when a required transcript is absent); exit 0 certified, 1 not.
func runAdapterDevinSettle(args []string) int {
	flags := flag.NewFlagSet("adapter devin-settle", flag.ContinueOnError)
	transcript := flags.String("transcript", "", "exported ATIF transcript")
	session := flags.String("session", "", "correlated session id (empty when none)")
	roundDir := flags.String("round-dir", "", "round directory for the disagreement artifact")
	settleSnapshot := flags.String("snapshot", "", "attempt snapshot path (D64: shared bytes with usage and collection)")
	requireTranscript := flags.Bool("require-transcript", false, "an absent transcript is unconfirmable (the repair shape)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *transcript == "" || *roundDir == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-settle --transcript F --round-dir D [--snapshot F] [--session SID] [--require-transcript]")
		return 2
	}
	model, certified, err := adapter.DevinSettle(*transcript, *settleSnapshot, *session, *roundDir, *requireTranscript)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if model != "" {
		fmt.Println(model)
	}
	if certified {
		return 0
	}
	return 1
}

// runAdapterDevinCollect walks the delivery channels. Exit 0
// delivered (verdict JSON on stdout), 3 nothing qualified, 5 transcript
// over the ceiling, 1 mechanical.
func runAdapterDevinCollect(args []string) int {
	flags := flag.NewFlagSet("adapter devin-collect", flag.ContinueOnError)
	params := adapter.CollectParams{}
	flags.StringVar(&params.Root, "root", "", "checkout root")
	flags.StringVar(&params.Job, "job", "", "job id")
	flags.StringVar(&params.RoundDir, "round-dir", "", "round evidence directory")
	flags.StringVar(&params.Workspace, "workspace", "", "delegate workspace (relative write targets resolve here)")
	flags.StringVar(&params.StdoutPath, "stdout", "", "the CLI's stdout capture")
	flags.StringVar(&params.NamedPath, "named", "", "the attempt's named return file")
	flags.StringVar(&params.TranscriptPath, "transcript", "", "the exported ATIF transcript")
	flags.StringVar(&params.ACPOutcomePath, "acp-outcome", "", "the acp turn outcome (exclusive channel; no fallthrough)")
	flags.StringVar(&params.RecordPath, "record", "", "job record file")
	flags.StringVar(&params.Attempt, "attempt", "initial", "initial or repair")
	flags.StringVar(&params.Session, "session", "", "correlated session (required unless presence-only)")
	flags.BoolVar(&params.PresenceOnly, "presence-only", false, "report candidatesPresent only, no validation")
	if flags.Parse(args) != nil {
		return 2
	}
	if params.Root == "" || params.Job == "" || params.RoundDir == "" || params.RecordPath == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-collect --root D --job ID --round-dir D --record F [--workspace D --stdout F --named F --transcript F --attempt A --session SID | --presence-only]")
		return 2
	}
	verdict, err := adapter.DevinCollect(params)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, atif.ErrOversize) {
			return 5
		}
		return 1
	}
	encoded, err := json.Marshal(verdict)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(encoded))
	if verdict.Delivered || params.PresenceOnly {
		return 0
	}
	return 3
}

// runAdapterACPUsage derives the turn's typed usage from an acp
// turn outcome (the wire branch: per-turn PromptResponse.usage,
// unavailable when absent — never a transcript fallback).
func runAdapterACPUsage(args []string) int {
	flags := flag.NewFlagSet("adapter acp-usage", flag.ContinueOnError)
	usagePath := flags.String("usage", "", "typed usage output path")
	outcomePath := flags.String("outcome", "", "the acp turn outcome file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *usagePath == "" || *outcomePath == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter acp-usage --usage F --outcome F")
		return 2
	}
	if err := usagepkg.ACPUsage(*usagePath, *outcomePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterDevinSession correlates this turn's Devin session against the
// pre-launch baseline. It prints the id and exits 0, exits 1 when none is
// found yet, and exits 3 (naming the candidates) when the correlation is
// ambiguous — its own code, distinct from the package-wide 2 for usage
// errors.
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
		return 3
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
	usageSnapshot := flags.String("snapshot", "", "attempt snapshot path (D64: shared bytes with settlement and collection)")
	expectPrevious := flags.Bool("expect-previous", false, "the turn resumes a session and must find a predecessor")
	if flags.Parse(args) != nil {
		return 2
	}
	if *usage == "" || *transcript == "" || *cumulative == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter devin-usage --usage FILE --transcript FILE --cumulative FILE [--snapshot FILE] [--previous FILE] [--expect-previous]")
		return 2
	}
	if err := adapter.DevinTurnUsage(*usage, *transcript, *usageSnapshot, *cumulative, *previous, *expectPrevious); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterFakeReturn writes the fake runtime's canned, schema-valid return
// for a turn from its job record and assembled prompt.
func runAdapterFakeReturn(args []string) int {
	flags := flag.NewFlagSet("adapter fake-return", flag.ContinueOnError)
	record := flags.String("record", "", "job record file")
	prompt := flags.String("prompt", "", "assembled prompt file")
	output := flags.String("output", "", "return object file to write")
	if flags.Parse(args) != nil {
		return 2
	}
	if *record == "" || *prompt == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-return --record FILE --prompt FILE --output FILE")
		return 2
	}
	if err := adapter.WriteFakeReturn(*record, *prompt, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterFakeUsage writes the fixed typed usage the fake runtime reports
// for every turn.
func runAdapterFakeUsage(args []string) int {
	flags := flag.NewFlagSet("adapter fake-usage", flag.ContinueOnError)
	output := flags.String("output", "", "typed usage output file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-usage --output FILE")
		return 2
	}
	if err := adapter.WriteFakeUsage(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterFakeEffectiveNetwork rewrites the network field of an
// effective-permissions file, simulating a runtime whose real grant differs
// from the request.
func runAdapterFakeEffectiveNetwork(args []string) int {
	flags := flag.NewFlagSet("adapter fake-effective-network", flag.ContinueOnError)
	effective := flags.String("effective", "", "effective-permissions file")
	network := flags.String("network", "", "network grant to record (allow|ask|deny)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *effective == "" || *network == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-effective-network --effective FILE --network VALUE")
		return 2
	}
	if err := adapter.SetEffectiveNetwork(*effective, *network); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runAdapterFakeGuardedWrite attempts a write through the fake runtime's
// permission envelope: exit 0 when a writeRoots member contains the target
// and the probe line was written, 77 when the envelope refuses it.
func runAdapterFakeGuardedWrite(args []string) int {
	flags := flag.NewFlagSet("adapter fake-guarded-write", flag.ContinueOnError)
	permissions := flags.String("permissions", "", "permissions envelope file")
	target := flags.String("target", "", "file the guarded write is aimed at")
	if flags.Parse(args) != nil {
		return 2
	}
	if *permissions == "" || *target == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-guarded-write --permissions FILE --target FILE")
		return 2
	}
	allowed, err := adapter.FakeGuardedWrite(*permissions, *target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !allowed {
		return 77
	}
	return 0
}

// runAdapterFakeGuardedNetwork attempts one network call through the fake
// runtime's permission envelope: exit 0 when the envelope allows it and the
// probe request was sent, 77 when the envelope refuses it.
func runAdapterFakeGuardedNetwork(args []string) int {
	flags := flag.NewFlagSet("adapter fake-guarded-network", flag.ContinueOnError)
	permissions := flags.String("permissions", "", "permissions envelope file")
	host := flags.String("host", "", "host to probe")
	port := flags.String("port", "", "port to probe")
	if flags.Parse(args) != nil {
		return 2
	}
	if *permissions == "" || *host == "" || *port == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-guarded-network --permissions FILE --host HOST --port PORT")
		return 2
	}
	allowed, err := adapter.FakeGuardedNetwork(*permissions, *host, *port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !allowed {
		return 77
	}
	return 0
}

// runAdapterFakeCapabilitySnapshot writes the fake probe's capability
// snapshot for a profile, printing the written path.
func runAdapterFakeCapabilitySnapshot(args []string) int {
	flags := flag.NewFlagSet("adapter fake-capability-snapshot", flag.ContinueOnError)
	dir := flags.String("dir", "", "capabilities directory")
	profile := flags.String("profile", "", "current, old, or unverified-network")
	ageDays := flags.Int("age-days", 0, "days to backdate the capture")
	handshake := flags.Int("handshake-sec", 0, "session-established ceiling in seconds")
	if flags.Parse(args) != nil {
		return 2
	}
	if *dir == "" || *profile == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-capability-snapshot --dir DIR --profile P [--age-days N] --handshake-sec S")
		return 2
	}
	path, err := adapter.WriteFakeCapabilitySnapshot(*dir, *profile, *ageDays, *handshake)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(path)
	return 0
}

// runAdapterFakeSelftestRecord writes the pass record the fake adapter's
// selftest leaves behind.
func runAdapterFakeSelftestRecord(args []string) int {
	flags := flag.NewFlagSet("adapter fake-selftest-record", flag.ContinueOnError)
	output := flags.String("output", "", "selftest record file to write")
	job := flags.String("job", "", "selftest job id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *output == "" || *job == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem adapter fake-selftest-record --output FILE --job ID")
		return 2
	}
	if err := adapter.WriteFakeSelftestRecord(*output, *job); err != nil {
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
