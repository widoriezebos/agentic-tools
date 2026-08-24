package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/host"
)

// The host family carries the per-turn work a runtime host does around a single
// CLI invocation: writing the turn's result envelope, compacting a schema for
// the command line, extracting a return and typed usage from a runtime's
// output, and the fixtures the fake host stands up.

// runHostResultWrite writes the turn's result envelope (sessionId, outcome,
// usage, rawPath, returnPath), reading typed usage from a file and recording an
// empty session or return path as null.
func runHostResultWrite(args []string) int {
	flags := flag.NewFlagSet("host result-write", flag.ContinueOnError)
	result := flags.String("result", "", "result envelope file to write")
	session := flags.String("session", "", "confirmed session id (empty writes null)")
	outcome := flags.String("outcome", "", "turn outcome")
	usageFile := flags.String("usage-file", "", "typed usage JSON file (missing writes unavailable)")
	raw := flags.String("raw", "", "raw output path recorded in the envelope")
	returnPath := flags.String("return-path", "", "return path recorded in the envelope (empty writes null)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *outcome == "" {
		fmt.Fprintln(os.Stderr, "host result-write: --result and --outcome are required")
		return 2
	}
	if err := host.ResultWrite(*result, *session, *outcome, *usageFile, *raw, *returnPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostFinish relays `host finish`: the one turn-outcome
// adjudication. Its exit code IS the host taxonomy the mission
// runner interprets — 0 completed, 3 failed, 6 missing session — and the
// host scripts just propagate it.
func runHostFinish(args []string) int {
	flags := flag.NewFlagSet("host finish", flag.ContinueOnError)
	result := flags.String("result", "", "result envelope file to write")
	session := flags.String("session", "", "confirmed session id (empty means unresumable)")
	usageFile := flags.String("usage-file", "", "typed usage JSON file")
	raw := flags.String("raw", "", "raw output path")
	returnPath := flags.String("return-path", "", "return path for a surviving turn")
	cliStatus := flags.Int64("cli-status", 0, "host CLI exit status")
	acceptedReply := flags.String("accepted-reply", "", "the delivery walk's accepted snapshot (judges require-reply instead of raw)")
	requireReply := flags.Bool("require-reply", false, "exit 0 with an empty reply is a failure (Devin's shape)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *raw == "" {
		fmt.Fprintln(os.Stderr, "host finish: --result and --raw are required")
		return 2
	}
	code, err := host.FinishTurn(*result, *session, *usageFile, *raw, *returnPath, *acceptedReply, *cliStatus, *requireReply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return code
}

// runHostDevinCollect walks a devin HOST turn's delivery channels.
// Exit 0 delivered (facts JSON on stdout), 3 nothing
// qualified, 5 transcript over the ceiling, 1 mechanical.
func runHostDevinCollect(args []string) int {
	flags := flag.NewFlagSet("host devin-collect", flag.ContinueOnError)
	params := delegate.HostCollectInputs{}
	var rejects rejectList
	flags.StringVar(&params.Root, "root", "", "checkout root")
	flags.StringVar(&params.TurnRecordPath, "turn-record", "", "turn record file")
	flags.StringVar(&params.TurnDir, "turn-dir", "", "turn evidence directory")
	flags.StringVar(&params.Workspace, "workspace", "", "checkout root the host worked in")
	flags.StringVar(&params.StdoutPath, "stdout", "", "the CLI's stdout capture")
	flags.StringVar(&params.NamedPath, "named", "", "the turn's named return file")
	flags.StringVar(&params.TranscriptPath, "transcript", "", "the exported ATIF transcript")
	flags.Var(&rejects, "reject", "sha256 of a runner-rejected candidate (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if params.Root == "" || params.TurnRecordPath == "" || params.TurnDir == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem host devin-collect --root D --turn-record F --turn-dir D [--workspace D --stdout F --named F --transcript F --reject SHA ...]")
		return 2
	}
	params.RejectDigests = rejects
	ports, ok := delegatePorts("host devin-collect", "devin")
	if !ok || ports.HostCollect == nil {
		return 1
	}
	encoded, delivered, err := ports.HostCollect(params)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, atif.ErrOversize) {
			return 5
		}
		return 1
	}
	fmt.Println(string(encoded))
	if delivered {
		return 0
	}
	return 3
}

type rejectList []string

func (r *rejectList) String() string { return strings.Join(*r, ",") }
func (r *rejectList) Set(value string) error {
	*r = append(*r, value)
	return nil
}

// runHostJSONCompact prints the JSON in a file on a single line, preserving key
// order and number tokens.
func runHostJSONCompact(args []string) int {
	flags := flag.NewFlagSet("host json-compact", flag.ContinueOnError)
	file := flags.String("file", "", "JSON file to compact")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "host json-compact: --file is required")
		return 2
	}
	compact, err := host.Compact(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(compact)
	return 0
}

// runHostClaudeResult extracts the return object and typed usage from a Claude
// CLI result document.
func runHostClaudeResult(args []string) int {
	flags := flag.NewFlagSet("host claude-result", flag.ContinueOnError)
	provider := flags.String("provider", "", "Claude CLI result document")
	returnPath := flags.String("return", "", "return object file to write when found")
	usagePath := flags.String("usage", "", "typed usage file to write")
	if flags.Parse(args) != nil {
		return 2
	}
	if *provider == "" || *returnPath == "" || *usagePath == "" {
		fmt.Fprintln(os.Stderr, "host claude-result: --provider, --return, and --usage are required")
		return 2
	}
	ports, ok := delegatePorts("host claude-result", "claude")
	if !ok || ports.HostResult == nil {
		return 1
	}
	if err := ports.HostResult(*provider, *returnPath, *usagePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostDevinConfig writes the Devin CLI config for a turn from the user's
// config plus the workspace-scoped permission set.
func runHostDevinConfig(args []string) int {
	flags := flag.NewFlagSet("host devin-config", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root the workspace globs are scoped to")
	output := flags.String("output", "", "config file to write")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "host devin-config: --root and --output are required")
		return 2
	}
	if err := host.DevinConfig(*root, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostDevinReturn extracts the return object from Devin's raw stdout.
func runHostDevinReturn(args []string) int {
	flags := flag.NewFlagSet("host devin-return", flag.ContinueOnError)
	raw := flags.String("raw", "", "raw runtime stdout")
	output := flags.String("output", "", "return object file to write when found")
	if flags.Parse(args) != nil {
		return 2
	}
	if *raw == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "host devin-return: --raw and --output are required")
		return 2
	}
	ports, ok := delegatePorts("host devin-return", "devin")
	if !ok || ports.HostReturn == nil {
		return 1
	}
	if err := ports.HostReturn(*raw, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostDevinUsage derives this turn's typed usage from Devin's cumulative
// session metrics, publishing the delta against its predecessor's totals.
func runHostDevinUsage(args []string) int {
	flags := flag.NewFlagSet("host devin-usage", flag.ContinueOnError)
	transcript := flags.String("transcript", "", "Devin transcript with final_metrics")
	usage := flags.String("usage", "", "typed usage file to write")
	cumulative := flags.String("cumulative", "", "this turn's cumulative totals file to write")
	previous := flags.String("previous", "", "predecessor's cumulative totals file (empty when none)")
	expectPrevious := flags.Bool("expect-previous", false, "the turn resumes a session and must find a predecessor")
	if flags.Parse(args) != nil {
		return 2
	}
	if *transcript == "" || *usage == "" || *cumulative == "" {
		fmt.Fprintln(os.Stderr, "host devin-usage: --transcript, --usage, and --cumulative are required")
		return 2
	}
	ports, ok := delegatePorts("host devin-usage", "devin")
	if !ok || ports.HostTurnUsage == nil {
		return 1
	}
	if err := ports.HostTurnUsage(*usage, *transcript, *cumulative, *previous, *expectPrevious); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostFakeReturn builds the fake host's return for a behavior marker.
func runHostFakeReturn(args []string) int {
	flags := flag.NewFlagSet("host fake-return", flag.ContinueOnError)
	turn := flags.String("turn", "", "turn record")
	state := flags.String("state", "", "mission state")
	output := flags.String("output", "", "return object file to write")
	behavior := flags.String("behavior", "", "fake host behavior marker")
	root := flags.String("root", "", "checkout root for any job record the behavior writes")
	if flags.Parse(args) != nil {
		return 2
	}
	if *turn == "" || *state == "" || *output == "" || *behavior == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "host fake-return: --turn, --state, --output, --behavior, and --root are required")
		return 2
	}
	ports, ok := delegatePorts("host fake-return", "fake")
	if !ok || ports.HostFakeReturn == nil {
		return 1
	}
	if err := ports.HostFakeReturn(*turn, *state, *output, *behavior, *root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runHostFakeResult writes the fake host's result envelope with the fixed typed
// usage the test double reports.
func runHostFakeResult(args []string) int {
	flags := flag.NewFlagSet("host fake-result", flag.ContinueOnError)
	result := flags.String("result", "", "result envelope file to write")
	session := flags.String("session", "", "session id")
	raw := flags.String("raw", "", "raw output path recorded in the envelope")
	returnPath := flags.String("return-path", "", "return path recorded in the envelope (empty writes null)")
	outcome := flags.String("outcome", "", "completed or failed")
	if flags.Parse(args) != nil {
		return 2
	}
	if *result == "" || *outcome == "" {
		fmt.Fprintln(os.Stderr, "host fake-result: --result and --outcome are required")
		return 2
	}
	ports, ok := delegatePorts("host fake-result", "fake")
	if !ok || ports.HostFakeResult == nil {
		return 1
	}
	if err := ports.HostFakeResult(*result, *session, *raw, *returnPath, *outcome); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
