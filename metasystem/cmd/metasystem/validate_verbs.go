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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// runValidateCritiqueClosed joins a critic return's findings array
// against the Markdown dispositions table on finding id. Exit 0 closed;
// 1 open or unjoinable; 2 usage.
func runValidateCritiqueClosed(args []string) int {
	flags := newFlagSet("validate critique-closed")
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

// runValidateSessionIsolation copies adapter-declared local
// configuration into a second-session worktree, audits the isolation,
// and prints the new checkout's harness root. Exit 0 isolated; 1 an
// unsafe manifest path or a failed audit; 2 usage.
func runValidateSessionIsolation(args []string) int {
	flags := newFlagSet("validate session-isolation")
	sourceRoot := flags.String("source-root", "", "primary checkout the configuration copies from")
	destinationRoot := flags.String("destination-root", "", "new second-session worktree")
	manifest := flags.String("manifest", "", "file listing the adapter-declared relative paths")
	harnessRoot := flags.String("harness-root", "", "harness root inside the primary checkout")
	if flags.Parse(args) != nil || !requireFlags(flags, nil, "source-root", "destination-root", "manifest", "harness-root") {
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
	flags := newFlagSet("validate conformance")
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
	flags := newFlagSet("experiment check")
	file := flags.String("file", "", "the investigation ledger")
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
		fmt.Fprintln(os.Stderr, "metasystem experiment check: needs the ledger: metasystem experiment check --file LEDGER; nothing was checked")
		return 2
	}
	if _, err := os.Stat(*file); err != nil {
		fmt.Fprintf(os.Stderr, "metasystem experiment check: no ledger at %s; nothing was checked\n", *file)
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
		fmt.Fprintln(os.Stderr, `usage: metasystem test baseline --gate CMD [--file F] [--root INSTALLATION]
       metasystem test baseline --check [--file F] [--max-age-minutes N] [--max-commits N] [--root INSTALLATION]

--gate: store the current clean, committed HEAD as the trusted refactor
baseline after the project's acceptance gate passed. --check: allow a new
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
	flags := newFlagSet("test baseline")
	flags.StringVar(&p.File, "file", "plans/refactor-baseline", "baseline file path")
	flags.StringVar(&p.Gate, "gate", "", "record: the acceptance gate command that passed")
	maxAge := flags.String("max-age-minutes", "", "check: maximum baseline age")
	maxCommits := flags.String("max-commits", "", "check: maximum commits since the baseline")
	root := pathFlag(flags, "root", "", "installation whose metasystem.conf supplies the cadence (default: this engine's)")
	if flags.Parse(args[1:]) != nil {
		return 2
	}
	if flags.NArg() != 0 {
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
	if code := resolveRefactorCadence(&p, set, *maxAge, *maxCommits, confPath, os.Stderr); code != 0 {
		return code
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	p.Cwd = cwd
	return validate.RefactorBaseline(p, os.Stdout, os.Stderr)
}

// resolveRefactorCadence resolves the refactor gate's cadence backstops: the
// flag when given, then the environment, then the installation's
// metasystem.conf, then 1440 minutes and 40 commits. It returns 2 on an
// unreadable or non-numeric value.
func resolveRefactorCadence(p *validate.RefactorBaselineParams, set map[string]bool, maxAge, maxCommits, confPath string, errOut io.Writer) int {
	for _, cadence := range []struct {
		key, value, fallback string
		target               *int
	}{
		{"refactor.max-age-minutes", maxAge, config.MustDefault("refactor.max-age-minutes"), &p.MaxAgeMinutes},
		{"refactor.max-commits", maxCommits, config.MustDefault("refactor.max-commits"), &p.MaxCommits},
	} {
		flagName := strings.TrimPrefix(cadence.key, "refactor.")
		value, code, err := config.Get(config.GetParams{
			Key: cadence.key, Flag: cadence.value, FlagSet: set[flagName],
			Default: cadence.fallback, DefaultSet: true, ConfPath: confPath,
		})
		if err != nil || code != 0 {
			if err != nil {
				fmt.Fprintln(errOut, err)
			}
			return 2
		}
		number, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintf(errOut, "invalid value %q for %s\n", value, cadence.key)
			return 2
		}
		*cadence.target = number
	}
	return 0
}
