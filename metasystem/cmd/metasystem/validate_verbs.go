package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// runValidateTurnPrompt validates an assembled unattended host-turn
// prompt against its canonical turn record and the shipped orchestrator
// preamble. Exit 0 pass; 1 the first violation (printed with its check
// family); 2 usage.
func runValidateTurnPrompt(args []string) int {
	flags := flag.NewFlagSet("validate turn-prompt", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "metasystem root holding the shipped preamble")
	file := flags.String("file", "", "assembled prompt file")
	turn := flags.String("turn", "", "turn directory holding turn.json")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" || *turn == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate turn-prompt --root R --file F --turn D")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate plan-consistency --plans-dir D")
		return 2
	}
	retired, violations, err := validate.PlanConsistency(*plansDir)
	if err != nil {
		// The real error, not a guessed label: an EACCES or not-a-directory
		// mislabeled "no such plans directory" sends an investigation the
		// wrong way.
		fmt.Fprintf(os.Stderr, "plan-consistency: %s: %v\n", *plansDir, err)
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
	repo := pathFlag(flags, "repo", "", "checkout root whose register is updated")
	rootJob := flags.String("root-job", "", "critic register root job")
	if flags.Parse(args) != nil {
		return 2
	}
	if *findings == "" || *dispositions == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem work review [j2:ROOT] --check-only --findings F --dispositions F")
		return 2
	}
	if (*repo == "") != (*rootJob == "") {
		fmt.Fprintln(os.Stderr, "validate critique-closed: --repo and --root-job must be supplied together")
		return 2
	}
	var violations []string
	if *repo != "" {
		violations = validate.CritiqueClosedWithRegister(*findings, *dispositions, *repo, *rootJob)
	} else {
		violations = validate.CritiqueClosed(*findings, *dispositions)
	}
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
	root := pathFlag(flags, "root", ".", "metasystem root quote sources resolve under")
	rolesDir := flags.String("roles-dir", "", "directory holding the role preambles")
	if flags.Parse(args) != nil {
		return 2
	}
	if *rolesDir == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate preamble-quotes --root R --roles-dir D")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate wrapper-token --token F --caller-pid N")
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
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate session-isolation --source-root A --destination-root B --manifest F --harness-root H")
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
	root := pathFlag(flags, "root", "", "checkout root")
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
		violations = returnschema.ReturnCompleteJob(*root, *job)
	case *job == "" && *role != "" && *file != "":
		violations = returnschema.ReturnCompleteRole(*root, *role, *file)
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
// calling convention of metasystem internal validate design-obligations: repeated
// --file arguments, an optional --runtime-required, and --root for
// resolving a relative path unreadable from the working directory. Exit 0
// passed; 1 failed; 2 usage.
func runValidateDesignObligations(args []string) int {
	usage := func() {
		fmt.Fprint(os.Stderr, `Usage:
  metasystem internal validate design-obligations --file <plan.md> [--file <plan.md>...]
  metasystem internal validate design-obligations --runtime-required --file <plan.md>...

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
	flags := flag.NewFlagSet("validate design-obligations", flag.ContinueOnError)
	flags.Usage = usage
	root := pathFlag(flags, "root", ".", "root for resolving relative plan paths")
	files := []string{}
	flags.Func("file", "design-obligation matrix (repeatable)", func(value string) error {
		files = append(files, value)
		return nil
	})
	runtimeRequired := flags.Bool("runtime-required", false, "CRITICAL/HIGH obligations must be DONE")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		usage()
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --file is required")
		usage()
		return 2
	}
	out, errs, code := validate.DesignObligations(*root, files, *runtimeRequired)
	for _, line := range out {
		fmt.Println(line)
	}
	for _, line := range errs {
		fmt.Fprintln(os.Stderr, line)
	}
	return code
}

// runValidateConformance relays the retired conformance wrapper's
// calling convention: --stage review|recertify|merge and --job, with --root naming
// the merge-target checkout. Exit 0 conforming; 1 conformance failure; 2
// usage.
func runValidateConformance(args []string) int {
	usage := func() {
		fmt.Fprint(os.Stderr, `Usage: metasystem work review j2:<job-id> --check-only --stage review|recertify|merge [--test-command <command>] [--recertification <record>]

The review stage computes the implementer worktree's exact review object. A
temporary index contains every tracked file plus every untracked, unignored
file; ignored files are excluded. It writes diff.patch and review.json without
changing the worktree's real index. Changed paths are measured from the
branch's merge-base with the current target and checked against the cumulative
union of immutable per-round declarations. The merge stage leaves review
artifacts untouched and requires either a mechanically valid waiver or a
closed, independent code-critic chain over the branch's final committed tree.
The recertify stage preserves those review bytes while mechanically merging
disjoint text hunks onto the current target. Area-width chains require an
explicit --test-command. The merge stage accepts --recertification only for
the exact proof produced for the same implementer job.

Exit codes: 0 conforming; 1 conformance failure; 2 usage.
`)
	}
	flags := flag.NewFlagSet("validate conformance", flag.ContinueOnError)
	flags.Usage = usage
	root := pathFlag(flags, "root", ".", "merge-target checkout root")
	stage, job := "", ""
	testCommand, recertification := "", ""
	// A gate argument given twice is a caller confusion this verb refuses
	// rather than last-wins (the hand-rolled loop's strictness, kept).
	once := func(target *string, name string) func(string) error {
		return func(value string) error {
			if *target != "" {
				return fmt.Errorf("--%s given twice", name)
			}
			*target = value
			return nil
		}
	}
	flags.Func("stage", "review, recertify, or merge", once(&stage, "stage"))
	flags.Func("job", "implementer job id", once(&job, "job"))
	flags.Func("test-command", "explicit recertification test command", once(&testCommand, "test-command"))
	flags.Func("recertification", "canonical recertification record path", once(&recertification, "recertification"))
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		usage()
		return 2
	}
	if stage != "review" && stage != "merge" && stage != "recertify" {
		usage()
		return 2
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`).MatchString(job) {
		usage()
		return 2
	}
	if (stage != "recertify" && testCommand != "") || (stage != "merge" && recertification != "") ||
		(stage == "recertify" && recertification != "") || (stage == "merge" && testCommand != "") {
		usage()
		return 2
	}
	out, errs, code := validate.ConformanceWithOptions(*root, stage, job, validate.ConformanceOptions{
		Recertification: recertification, TestCommand: testCommand,
	})
	for _, line := range out {
		fmt.Println(line)
	}
	for _, line := range errs {
		fmt.Fprintln(os.Stderr, line)
	}
	return code
}

// runValidateStopLoss owns the stop-loss check's calling
// convention: --file names the investigation ledger. Exit 0 more cycles
// allowed; 1 stop-loss triggered; 2 usage error.
func runValidateStopLoss(args []string) int {
	usage := func() {
		fmt.Fprint(os.Stderr, `Usage:
  metasystem experiment check --file <investigation-ledger.md>

Reads the cycle classifications from an investigation ledger and blocks
further cycles when a machine-checkable stop-loss trigger has fired:

  - any cycle classified falsified-dead-end
  - two or more cycles classified no-progress
  - as many cycles as the declared "Cycle budget:" line (when present)
  - as many trailing cycles without a contract-improved as the declared
    "No-gain budget:" line (when present; improve mode sets 3)

unresolved (a valid measurement inside a declared noise floor) never counts
toward the no-progress trigger; only a declared no-gain budget bounds it.

The judgment triggers (repeating one mechanism family, an expensive run
that taught nothing, no novel fact) stay with the agent and the human.
This check only enforces what the ledger already states, so a ledger
that stops recording classifications also stops being protected.

Run it before contracting a new cycle.
Exit codes: 0 more cycles are allowed; 1 stop-loss triggered; 2 usage error.
`)
	}
	flags := flag.NewFlagSet("validate stop-loss", flag.ContinueOnError)
	flags.Usage = usage
	file := flags.String("file", "", "investigation ledger")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		usage()
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "missing --file ledger")
		return 2
	}
	out, errs, code := validate.StopLoss(*file)
	for _, line := range out {
		fmt.Println(line)
	}
	for _, line := range errs {
		fmt.Fprintln(os.Stderr, line)
	}
	return code
}

func runValidateMovedEffects(args []string) int {
	usage := func() {
		fmt.Fprint(os.Stderr, `Usage:
  metasystem internal validate moved-effects --file <page.md> [--root <repository-root>]
  (a person or critic uses metasystem design review PAGE --check-only)

Checks a Moved effects inventory with this table header:
| Effect | From | To | Code |

Use the exact line "No owner moves." when the page moves no owner.
`)
	}
	flags := flag.NewFlagSet("validate moved-effects", flag.ContinueOnError)
	flags.Usage = usage
	root := pathFlag(flags, "root", "..", "repository root containing the metasystem root")
	file := flags.String("file", "", "design page")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 || *file == "" {
		fmt.Fprintln(os.Stderr, "validate moved-effects: --file is required and positional arguments are not accepted")
		usage()
		return 2
	}
	page, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate moved-effects: read %s: %v\n", *file, err)
		return 2
	}
	lines, problems := movedEffectsReport(page, *root)
	for _, line := range lines {
		fmt.Println(line)
	}
	if problems > 0 {
		return 1
	}
	return 0
}

// movedEffectsReport checks a design page's moved-effect inventory against
// the files of repositoryRoot: one line per row and problem, then the
// verdict line, and the number of problems.
func movedEffectsReport(page []byte, repositoryRoot string) ([]string, int) {
	exists := func(path string) bool {
		clean := filepath.Clean(filepath.FromSlash(path))
		if filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return false
		}
		info, err := os.Stat(filepath.Join(repositoryRoot, clean))
		return err == nil && (info.Mode().IsRegular() || info.IsDir())
	}
	report := validate.CheckMovedEffects(page, exists)
	var lines []string
	for _, row := range report.Rows {
		paths := make([]string, 0, len(row.Paths))
		for _, path := range row.Paths {
			status := "absent"
			if exists(path) {
				status = "ok"
			}
			paths = append(paths, path+" "+status)
		}
		if len(paths) == 0 {
			paths = append(paths, "(none)")
		}
		lines = append(lines, fmt.Sprintf("row %d: %s | %s -> %s | code %s", row.Line, row.Effect, row.From, row.To, strings.Join(paths, " ")))
	}
	for _, problem := range report.Problems {
		lines = append(lines, fmt.Sprintf("%s: line %d: %s", problem.Code, problem.Line, problem.Detail))
	}
	if report.Inventory == "absent" {
		lines = append(lines, "moved-effects: no Moved effects section; the design critic decides whether this page moves an owner, and a page that does without this section is a material finding")
	}
	lines = append(lines, fmt.Sprintf("moved-effects: inventory=%s rows=%d problems=%d", report.Inventory, len(report.Rows), len(report.Problems)))
	return lines, len(report.Problems)
}

// runValidateRefactorBaseline relays the refactor gate:
// record a trusted baseline after the acceptance gate, or check whether a
// new refactor edit batch may start. Exit 0 safe, 1 blocked, 2 usage or
// environment error — the contract its callers script against.
func runValidateRefactorBaseline(args []string) int {
	usage := func() int {
		fmt.Fprintln(os.Stderr, `usage: metasystem internal validate refactor-baseline record --gate CMD [--file F] [--root INSTALLATION]
       metasystem internal validate refactor-baseline check [--file F] [--max-age-minutes N] [--max-commits N] [--root INSTALLATION]

record: store the current clean, committed HEAD as the trusted refactor
baseline after the project's acceptance gate passed. check: allow a new
refactor edit batch only when the worktree is clean, the baseline is an
ancestor of HEAD, and the cadence backstop is not exceeded. The cadence
resolves from flags, then environment, then the installation's
metasystem.conf, then 1440 minutes and 40 commits.
Exit codes: 0 safe; 1 blocked; 2 usage or environment error.`)
		return 2
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		usage()
		return 0
	}
	if len(args) == 0 || (args[0] != "record" && args[0] != "check") {
		return usage()
	}
	var p validate.RefactorBaselineParams
	p.Command = args[0]
	flags := flag.NewFlagSet("validate refactor-baseline", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&p.File, "file", "plans/refactor-baseline", "baseline file path")
	flags.StringVar(&p.Gate, "gate", "", "record: the acceptance gate command that passed")
	maxAge := flags.String("max-age-minutes", "", "check: maximum baseline age")
	maxCommits := flags.String("max-commits", "", "check: maximum commits since the baseline")
	root := pathFlag(flags, "root", "", "installation whose metasystem.conf supplies the cadence (default: this engine's)")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return usage()
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	installation := *root
	if installation == "" {
		if exe, err := os.Executable(); err == nil {
			installation = filepath.Dir(filepath.Dir(exe))
		}
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	for _, cadence := range []struct {
		key, value, fallback string
		target               *int
	}{
		{"refactor.max-age-minutes", *maxAge, "1440", &p.MaxAgeMinutes},
		{"refactor.max-commits", *maxCommits, "40", &p.MaxCommits},
	} {
		flagName := strings.TrimPrefix(cadence.key, "refactor.")
		value, code, err := config.Get(config.GetParams{
			Key: cadence.key, Flag: cadence.value, FlagSet: set[flagName],
			Default: cadence.fallback, DefaultSet: true, ConfPath: confPath,
		})
		if err != nil || code != 0 {
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			return 2
		}
		number, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid value %q for %s\n", value, cadence.key)
			return 2
		}
		*cadence.target = number
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	p.Cwd = cwd
	return validate.RefactorBaseline(p, os.Stdout, os.Stderr)
}

// runValidateSkills validates skills: every skill under the root's skills and
// optional-skills (validate.SkillInventory), or only the named skill
// directories: `validate skills [--root R] [DIR ...]`.
func runValidateSkills(args []string) int {
	flags := flag.NewFlagSet("validate skills", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout whose skills and optional-skills are validated")
	if flags.Parse(args) != nil {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal validate skills [--root CHECKOUT] [DIR ...]")
		return 2
	}
	if flags.NArg() > 0 {
		for _, dir := range flags.Args() {
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				fmt.Fprintf(os.Stderr, "skill directory is not a directory: %s\n", dir)
				return 2
			}
			name, err := validate.Skill(dir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Printf("%s is valid\n", name)
		}
		return 0
	}
	if info, err := os.Stat(*root); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "skill inventory root is not a directory: %s\n", *root)
		return 2
	}
	if err := validate.SkillInventory(*root, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
