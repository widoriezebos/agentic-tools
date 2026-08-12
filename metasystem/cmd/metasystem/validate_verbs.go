package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// runConfigTailor rewrites a metasystem.conf in place for the selected
// runtime set: the runtime list becomes durable state, unselected
// runtimes lose their role and mode bindings, per-runtime model keys,
// and model-tier members, and the default runtime is set. Exit 2 marks
// bad flags; exit 1 a failed rewrite.
func runConfigTailor(args []string) int {
	flags := flag.NewFlagSet("config tailor", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to the metasystem.conf to rewrite")
	runtimes := flags.String("runtimes", "", "comma-separated selected runtimes, or none")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" || *runtimes == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem config tailor --conf F --runtimes claude,devin,codex|none")
		return 2
	}
	selected := strings.Split(*runtimes, ",")
	seen := map[string]bool{}
	for _, runtime := range selected {
		switch runtime {
		case "claude", "devin", "codex", "none":
		default:
			fmt.Fprintf(os.Stderr, "unknown runtime: %s (claude, devin, codex, or none)\n", runtime)
			return 2
		}
		if seen[runtime] {
			fmt.Fprintln(os.Stderr, "--runtimes contains a duplicate runtime")
			return 2
		}
		seen[runtime] = true
	}
	if seen["none"] && len(selected) > 1 {
		fmt.Fprintln(os.Stderr, "--runtimes none cannot be combined with other runtimes")
		return 2
	}
	if err := validate.TailorConf(*conf, selected); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runValidateTurnPrompt validates an assembled unattended host-turn
// prompt against its canonical turn record and the shipped orchestrator
// preamble. Exit 0 pass; 1 the first violation (printed with its check
// family); 2 usage.
func runValidateTurnPrompt(args []string) int {
	flags := flag.NewFlagSet("validate turn-prompt", flag.ContinueOnError)
	root := flags.String("root", ".", "metasystem root holding the shipped preamble")
	file := flags.String("file", "", "assembled prompt file")
	turn := flags.String("turn", "", "turn directory holding turn.json")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" || *turn == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate turn-prompt --root R --file F --turn D")
		return 2
	}
	if violation := validate.TurnPrompt(*root, *file, *turn); violation != nil {
		fmt.Fprintf(os.Stderr, "turn prompt violation [%s]: %s\n", violation.Check, violation.Message)
		return 1
	}
	return 0
}

// runValidatePlanConsistency fails when a plan prescribes a term
// another plan has retired via a RETIRED: marker. Exit 0 consistent;
// 1 a retired term is still prescribed; 2 usage or a missing directory.
func runValidatePlanConsistency(args []string) int {
	flags := flag.NewFlagSet("validate plan-consistency", flag.ContinueOnError)
	plansDir := flags.String("plans-dir", "", "directory holding the plans")
	if flags.Parse(args) != nil {
		return 2
	}
	if *plansDir == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate plan-consistency --plans-dir D")
		return 2
	}
	retired, violations, err := validate.PlanConsistency(*plansDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no such plans directory: %s\n", *plansDir)
		return 2
	}
	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "plan consistency: a retired term is still prescribed")
		for _, item := range violations {
			fmt.Fprintf(os.Stderr, "  %s\n", item)
		}
		fmt.Fprintln(os.Stderr, "  Either state the change on that line (say it was replaced, or mark it SUPERSEDED) or bring the line up to date.")
		return 1
	}
	fmt.Printf("plan consistency: %d retired term(s), none prescribed\n", retired)
	return 0
}

// runValidateCritiqueClosed joins a critic return's findings array
// against the Markdown dispositions table on finding id. Exit 0 closed;
// 1 open or unjoinable; 2 usage.
func runValidateCritiqueClosed(args []string) int {
	flags := flag.NewFlagSet("validate critique-closed", flag.ContinueOnError)
	findings := flags.String("findings", "", "critic return JSON")
	dispositions := flags.String("dispositions", "", "Markdown file holding the dispositions table")
	if flags.Parse(args) != nil {
		return 2
	}
	if *findings == "" || *dispositions == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate critique-closed --findings F --dispositions F")
		return 2
	}
	violations := validate.CritiqueClosed(*findings, *dispositions)
	for _, item := range violations {
		fmt.Fprintf(os.Stderr, "violation: %s\n", item)
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

// runValidatePreambleQuotes verifies every role preamble's quote block
// is a byte-exact, contiguous substring of its named source under the
// metasystem root. Exit 0 pass; 1 drift or malformed quote; 2 usage.
func runValidatePreambleQuotes(args []string) int {
	flags := flag.NewFlagSet("validate preamble-quotes", flag.ContinueOnError)
	root := flags.String("root", ".", "metasystem root quote sources resolve under")
	rolesDir := flags.String("roles-dir", "", "directory holding the role preambles")
	if flags.Parse(args) != nil {
		return 2
	}
	if *rolesDir == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate preamble-quotes --root R --roles-dir D")
		return 2
	}
	violations := validate.PreambleQuotes(*root, *rolesDir)
	for _, item := range violations {
		fmt.Fprintf(os.Stderr, "quote violation: %s\n", item)
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

// runValidateCodeCritiqueClaim verifies a receipt's code-critique
// claim: the delegate triples (runtime:model:job-id, passed as
// arguments after the flags) must include a top-level code-critic chain
// whose reviews field names one of the implementer delegate jobs.
// Exit 0 verified; 1 refused.
func runValidateCodeCritiqueClaim(args []string) int {
	flags := flag.NewFlagSet("validate code-critique-claim", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root holding artifacts/agents/jobs")
	if flags.Parse(args) != nil {
		return 2
	}
	if validate.CodeCritiqueClaim(*root, flags.Args()) {
		return 0
	}
	fmt.Fprintln(os.Stderr, "receipt refused: skills=code-critique requires delegate entries naming a "+
		"code-critic chain id and the implementer job id in that chain's reviews field")
	return 1
}

// runValidateWaiverFacts resolves an implementer delegate's
// critique-waiver facts from the delegate triples passed after the
// flags: it prints the waiver class and the mission stream on two
// lines, or none/none when no delegate carries a waiver. Always exit 0.
func runValidateWaiverFacts(args []string) int {
	flags := flag.NewFlagSet("validate waiver-facts", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root holding artifacts/agents")
	if flags.Parse(args) != nil {
		return 2
	}
	class, stream := validate.WaiverFacts(*root, flags.Args())
	fmt.Println(class)
	fmt.Println(stream)
	return 0
}

// runValidateWrapperToken proves the caller runs under the live commit
// wrapper the token names: valid token fields, the wrapper pid in the
// caller's native process ancestry, and the wrapper's kernel start time
// matching the token. Exit 0 proven; 1 not proven; 2 usage.
func runValidateWrapperToken(args []string) int {
	flags := flag.NewFlagSet("validate wrapper-token", flag.ContinueOnError)
	token := flags.String("token", "", "wrapper commit-token JSON file")
	callerPid := flags.Int64("caller-pid", 0, "pid whose ancestry must contain the live wrapper")
	if flags.Parse(args) != nil {
		return 2
	}
	if *token == "" || *callerPid <= 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate wrapper-token --token F --caller-pid N")
		return 2
	}
	if validate.WrapperToken(*token, *callerPid, validate.KernelProcessTree{}) {
		return 0
	}
	return 1
}

// runValidateSessionIsolation copies adapter-declared local
// configuration into a second-session worktree, audits the isolation,
// and prints the new checkout's harness root. Exit 0 isolated; 1 an
// unsafe manifest path or a failed audit; 2 usage.
func runValidateSessionIsolation(args []string) int {
	flags := flag.NewFlagSet("validate session-isolation", flag.ContinueOnError)
	sourceRoot := flags.String("source-root", "", "primary checkout the configuration copies from")
	destinationRoot := flags.String("destination-root", "", "new second-session worktree")
	manifest := flags.String("manifest", "", "file listing the adapter-declared relative paths")
	harnessRoot := flags.String("harness-root", "", "harness root inside the primary checkout")
	if flags.Parse(args) != nil {
		return 2
	}
	if *sourceRoot == "" || *destinationRoot == "" || *manifest == "" || *harnessRoot == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem validate session-isolation --source-root A --destination-root B --manifest F --harness-root H")
		return 2
	}
	newHarness, err := validate.SessionIsolation(*sourceRoot, *destinationRoot, *manifest, *harnessRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(newHarness)
	return 0
}

// runValidateReturnComplete validates a canonical agent return against the
// shipped role schema — by role and file, or by job (walking the chain and
// checking identity). Violations print to stderr, one per line.
func runValidateReturnComplete(args []string) int {
	flags := flag.NewFlagSet("validate return-complete", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	role := flags.String("role", "", "role name (with --file)")
	file := flags.String("file", "", "return file (with --role)")
	job := flags.String("job", "", "job id (instead of --role/--file)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "validate return-complete: --root is required")
		return 2
	}
	var violations []string
	switch {
	case *job != "" && *role == "" && *file == "":
		violations = validate.ReturnCompleteJob(*root, *job)
	case *job == "" && *role != "" && *file != "":
		violations = validate.ReturnCompleteRole(*root, *role, *file)
	default:
		fmt.Fprintln(os.Stderr, "validate return-complete: --job, or --role with --file")
		return 2
	}
	for _, violation := range violations {
		fmt.Fprintf(os.Stderr, "violation: %s\n", violation)
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

// runValidateDesignObligations checks design-obligation matrices with the
// calling convention of scripts/assert-design-obligation-gate.sh: repeated
// --file arguments, an optional --runtime-required, and --root for
// resolving a relative path unreadable from the working directory. Exit 0
// passed; 1 failed; 2 usage.
func runValidateDesignObligations(args []string) int {
	usage := func() {
		fmt.Fprint(os.Stderr, `Usage:
  scripts/assert-design-obligation-gate.sh --file <plan.md> [--file <plan.md>...]
  scripts/assert-design-obligation-gate.sh --runtime-required --file <plan.md>...

Checks the structure and declared state of design-obligation matrices.

Required table header:
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |

By default, CRITICAL/HIGH obligations must be DONE or READY_FOR_RUNTIME.
With --runtime-required, CRITICAL/HIGH obligations must be DONE.

Proof cells on CRITICAL/HIGH rows must be concrete: a backticked token, a
path-shaped token (a slash, or a filename with a letter-bearing stem and an
extension of two or more characters, plus .c/.h/.m/.r), or "Not applicable"
followed by a reason. Bare "Not applicable" fails, and so does keyword-only
prose ("needs testing"): a status is only as trustworthy as the proof behind
it. Owner cells on CRITICAL/HIGH rows need a backticked, dotted, slashed,
double-colon, or CamelCase code token; plain prose fails.

Matrix rows inside fenced code blocks are ignored, so documentation that shows
the template does not satisfy the gate. Table cells must not contain literal
pipe characters; the column parser cannot see an escaped pipe as content.
`)
	}
	root := "."
	files := []string{}
	runtimeRequired := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --file")
				return 2
			}
			files = append(files, args[i+1])
			i++
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "missing value for --root")
				return 2
			}
			root = args[i+1]
			i++
		case "--runtime-required":
			runtimeRequired = true
		case "-h", "--help":
			usage()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
			usage()
			return 2
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --file is required")
		usage()
		return 2
	}
	out, errs, code := validate.DesignObligations(root, files, runtimeRequired)
	for _, line := range out {
		fmt.Println(line)
	}
	for _, line := range errs {
		fmt.Fprintln(os.Stderr, line)
	}
	return code
}
