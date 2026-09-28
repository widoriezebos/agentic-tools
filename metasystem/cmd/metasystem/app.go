package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// The application under the engine's hand. `app` starts, stops, restarts,
// reads, resets and checks the project's own application through the
// contract the project wrote, and `app serve` is the supervisor that owns
// one run for its life. The contract names the commands; nothing here
// interprets a language.

// appContractKey selects the launch contract beside the settings, the way
// testing.contract selects the testing contract.
const appContractKey = "launch.contract"

// errNoLaunchContract is the refusal a project without a contract gets. It
// names the file to write, because a refusal that does not say what to do
// next is a dead end.
var errNoLaunchContract = errors.New("this project has no launch contract: write launch.json beside metasystem.conf (launch.contract=launch.json is the default; set launch.contract to name another path)")

// loadPhysicalLaunchContract reads the committed launch contract the
// settings name, exactly as the testing contract is read.
func loadPhysicalLaunchContract(root string) (string, applaunch.Contract, string, error) {
	installation, err := canonicalProofRoot(root)
	if err != nil {
		return "", applaunch.Contract{}, "", err
	}
	confPath := filepath.Join(installation, "metasystem.conf")
	relative, found, err := config.ConfLookup(confPath, appContractKey)
	if err != nil {
		return "", applaunch.Contract{}, "", err
	}
	if !found {
		// The compiled default names launch.json; a project that has not
		// written it has no launch contract, not a broken one.
		relative = config.MustDefault(appContractKey)
		if _, statErr := os.Lstat(filepath.Join(installation, filepath.FromSlash(relative))); os.IsNotExist(statErr) {
			return "", applaunch.Contract{}, "", errNoLaunchContract
		}
	}
	if strings.TrimSpace(relative) == "" {
		return "", applaunch.Contract{}, "", errNoLaunchContract
	}
	if filepath.IsAbs(relative) || filepath.ToSlash(filepath.Clean(relative)) != relative || strings.HasPrefix(relative, "../") {
		return "", applaunch.Contract{}, "", fmt.Errorf("%s must be a relative normalized path", appContractKey)
	}
	path := filepath.Join(installation, filepath.FromSlash(relative))
	contract, err := applaunch.Load(path)
	if err != nil {
		return installation, applaunch.Contract{}, path, fmt.Errorf("the launch contract %s is not valid: %w", path, err)
	}
	return installation, contract, path, nil
}

// launchContractReady validates the launch contract and its declared tools
// without starting anything, for `settings check`.
func launchContractReady(root string) (string, applaunch.Contract, []applaunch.ToolLine, error) {
	installation, contract, path, err := loadPhysicalLaunchContract(root)
	if err != nil {
		return "", applaunch.Contract{}, nil, err
	}
	projectRoot, err := (gittree.Workspace{Dir: installation}).TopLevel()
	if err != nil {
		return "", applaunch.Contract{}, nil, err
	}
	tools, err := launchToolLines(projectRoot, contract)
	if err != nil {
		return "", applaunch.Contract{}, nil, err
	}
	return path, contract, tools, nil
}

// launchToolLines is the declared tools' preflight: each executable resolved
// in the environment the contract's commands will actually run in, so a
// missing JDK, cargo or Go is named before a build fails, and the first line
// its version arguments print. It is an availability check; a version
// constraint is not this contract's to state.
func launchToolLines(projectRoot string, contract applaunch.Contract) ([]applaunch.ToolLine, error) {
	cwd := projectRoot
	if !contract.Start.Empty() && contract.Start.CWD != "" {
		cwd = filepath.Join(projectRoot, filepath.FromSlash(contract.Start.CWD))
	}
	var lines []applaunch.ToolLine
	var unavailable []string
	for _, tool := range contract.Tools {
		program, err := resolveLaunchTool(cwd, tool.Executable)
		if err != nil {
			unavailable = append(unavailable, tool.ID)
			continue
		}
		lines = append(lines, applaunch.ToolLine{ID: tool.ID, Executable: program, Version: toolVersionLine(cwd, program, tool.VersionArgs)})
	}
	if len(unavailable) > 0 {
		sort.Strings(unavailable)
		return nil, fmt.Errorf("declared launch tools unavailable: %s", strings.Join(unavailable, ","))
	}
	return lines, nil
}

// resolveLaunchTool finds a declared tool the way the testing contract's
// runner finds one, in the directory the contract's commands run in. What it
// returns is the project's tool, never the engine.
func resolveLaunchTool(cwd, name string) (string, error) {
	program, err := proofrun.ResolveTestingExecutable(context.Background(), cwd, os.Environ(), []string{name})
	if err != nil {
		return "", err
	}
	return program, nil
}

// toolVersionLine runs a tool's version arguments, bounded, and keeps the
// first line it printed. A tool that prints nothing, or declares no version
// arguments, has no line; its availability is still recorded.
func toolVersionLine(cwd, program string, args []string) string {
	if len(args) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = cwd
	out, _ := command.CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// appRun is one resolved run: which tree its commands run in, which data it
// uses, where its log is and what it is called.
type appRun struct {
	roots        lifecycle.Roots
	contract     applaunch.Contract
	contractPath string
	key          string
	ref          string
	goal         string
	commit       string
	resolvedFrom string
	address      string
	tree         string
	dataRoot     string
	logPath      string
	runDir       string
	// lookupEnv answers the environment the evidence root resolves under;
	// nil is os.LookupEnv.
	lookupEnv func(string) (string, bool)
}

// resolveAppRun derives every path of one run from the roots, the contract
// and the ref. It is the one place they are derived, so that a verb and the
// supervisor it launches cannot disagree about where the run lives.
func resolveAppRun(roots lifecycle.Roots, contract applaunch.Contract, contractPath, ref, goal string) appRun {
	run := appRun{roots: roots, contract: contract, contractPath: contractPath, ref: ref, goal: goal,
		key: applaunch.KeyFor(ref)}
	run.runDir = applaunch.RunDir(roots.StateRoot, run.key)
	// Every run's data has a directory of its own, the standing run's too:
	// the state root a prepare is given is one it may clear, and the
	// engine's own installation is never that.
	run.dataRoot = filepath.Join(run.runDir, "data")
	if ref == "" {
		run.tree = roots.Checkout
	} else {
		run.tree = filepath.Join(run.runDir, "tree")
	}
	if contract.Log != "" {
		run.logPath = filepath.Join(run.tree, filepath.FromSlash(contract.Log))
	} else {
		run.logPath = applaunch.DefaultLogPath(roots.StateRoot, run.key)
	}
	return run
}

func (r appRun) preparedMarker() string { return filepath.Join(r.runDir, "prepared") }

func (r appRun) readOptions() applaunch.ReadOptions {
	return applaunch.ReadOptions{Probe: applaunch.ProbeOnce}
}

func (r appRun) status() (applaunch.Status, error) {
	return applaunch.Read(r.roots.StateRoot, r.key, r.contract, r.readOptions())
}

// seedRecord is what the supervisor writes before it spawns anything.
func (r appRun) seedRecord() applaunch.Record {
	return applaunch.Record{Key: r.key, Name: r.contract.Name, Ref: r.ref, Goal: r.goal, Commit: r.commit,
		ResolvedFrom: r.resolvedFrom, Address: r.address, Log: r.logPath, StateRoot: r.dataRoot, Tree: r.tree}
}

// environment is the base every command of the contract inherits.
func (r appRun) environment() []string { return os.Environ() }

// resolveCommitFor reads the commit a ref names and says which ref named it.
// A goal's candidate is origin's tip, so goal/G tries origin/goal/G first and
// falls back to the local branch only when origin has none; any other ref is
// read locally first and at origin when the local ref is not there.
func (r appRun) resolveCommitFor(ref string) (string, string, error) {
	workspace := gittree.Workspace{Dir: r.roots.Checkout}
	candidates := []string{ref, "origin/" + ref}
	if strings.HasPrefix(ref, "goal/") {
		candidates = []string{"origin/" + ref, ref}
	}
	for _, candidate := range candidates {
		if commit, err := workspace.ResolveCommit(candidate); err == nil {
			return commit, candidate, nil
		}
	}
	return "", "", fmt.Errorf("no commit is named by %s", ref)
}

// gitIn runs one git command in a directory and returns its combined output.
func gitIn(dir string, args ...string) (string, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// takeWorktree gives a run at a commit a tree of its own under artifacts,
// as a critique already gets one. A tree that holds another commit is
// reclaimed first: one run per ref at a time, and a moved ref replaces it.
func (r *appRun) takeWorktree(out io.Writer) error {
	if r.ref == "" {
		return nil
	}
	if existing, err := gitIn(r.tree, "rev-parse", "HEAD"); err == nil {
		if strings.TrimSpace(existing) == r.commit {
			fmt.Fprintln(out, "tree: "+r.tree+" is already at "+shortCommit(r.commit))
			return nil
		}
		if err := r.reclaimWorktree(out); err != nil {
			return err
		}
	} else if _, statErr := os.Stat(r.tree); statErr == nil {
		if err := os.RemoveAll(r.tree); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(r.tree), 0o755); err != nil {
		return err
	}
	if _, err := gitIn(r.roots.Checkout, "worktree", "add", "--detach", "--quiet", r.tree, r.commit); err != nil {
		return err
	}
	fmt.Fprintln(out, "tree: "+r.tree+" at "+shortCommit(r.commit))
	return nil
}

// reclaimWorktree takes a run's tree away. It is called only by the next
// start of that ref or by stop --clean, and never while a record still names
// the tree.
func (r *appRun) reclaimWorktree(out io.Writer) error {
	if r.ref == "" {
		return nil
	}
	if _, err := os.Stat(r.tree); err != nil {
		return nil
	}
	if _, err := gitIn(r.roots.Checkout, "worktree", "remove", "--force", r.tree); err != nil {
		if removeErr := os.RemoveAll(r.tree); removeErr != nil {
			return removeErr
		}
		_, _ = gitIn(r.roots.Checkout, "worktree", "prune")
	}
	fmt.Fprintln(out, "tree: reclaimed "+r.tree)
	return nil
}

// runContractCommand runs one of the contract's commands to completion in
// the run's tree, with the run's facts in its environment and its output in
// the run's log. Every wait has a deadline and an owner; this one's owner is
// the verb that called it.
func (r appRun) runContractCommand(what string, command *applaunch.Command, timeout time.Duration, out io.Writer) error {
	if command.Empty() {
		return nil
	}
	facts := applaunch.FactsFor(r.address)
	argv := facts.Argv(command)
	process := exec.Command(argv[0], argv[1:]...)
	process.Dir = filepath.Join(r.tree, filepath.FromSlash(command.CWD))
	process.Env = facts.Environment(r.environment(), r.dataRoot, r.logPath)
	if err := os.MkdirAll(filepath.Dir(r.logPath), 0o755); err != nil {
		return err
	}
	log, err := os.OpenFile(r.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	fmt.Fprintf(log, "\n--- %s: %s\n", what, strings.Join(argv, " "))
	process.Stdout, process.Stderr = log, log
	fmt.Fprintln(out, what+": "+strings.Join(argv, " "))
	if err := process.Start(); err != nil {
		return fmt.Errorf("%s could not be run: %w", what, err)
	}
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s failed: %w; see %s", what, err, r.logPath)
		}
		return nil
	case <-time.After(timeout):
		_ = process.Process.Kill()
		return fmt.Errorf("%s did not finish within %s; see %s", what, timeout, r.logPath)
	}
}

// prepareData runs the contract's prepare command, which is what gives a run
// data of its own. Without one, a run at another commit shares the standing
// run's data and the record, the status and the room say so.
func (r appRun) prepareData(out io.Writer, force bool) error {
	if r.contract.Prepare.Empty() {
		return nil
	}
	if !force {
		if _, err := os.Stat(r.preparedMarker()); err == nil {
			fmt.Fprintln(out, "prepare: already run for this run")
			return nil
		}
	}
	if err := os.MkdirAll(r.dataRoot, 0o755); err != nil {
		return err
	}
	if err := r.runContractCommand("prepare", r.contract.Prepare, 10*time.Minute, out); err != nil {
		return err
	}
	if err := os.MkdirAll(r.runDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.preparedMarker(), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
}

// allocateAddress gives the standing run the contract's address and every
// other run one of its own from the range. A port a contract cannot give, or
// one another process holds, is named, not guessed.
func (r *appRun) allocateAddress() error {
	if r.contract.Address == "" {
		r.address = ""
		return nil
	}
	if r.ref == "" {
		// The standing address is the contract's own and is not chosen, so it
		// is checked: another service answering there would be taken for
		// this application.
		if !available(r.contract.Address) {
			return fmt.Errorf("the standing address %s is taken by another process; free it or change address in the launch contract", r.contract.Address)
		}
		r.address = r.contract.Address
		return nil
	}
	if record, err := applaunch.ReadRecord(r.roots.StateRoot, r.key); err == nil && record.Address != "" {
		if available(record.Address) {
			r.address = record.Address
			return nil
		}
	}
	low, high, err := r.contract.Range()
	if err != nil || low == 0 {
		return errors.New("a run at another commit needs an address of its own, but the contract declares no portRange")
	}
	host, _, splitErr := net.SplitHostPort(r.contract.Address)
	if splitErr != nil {
		return splitErr
	}
	taken := r.takenAddresses()
	for port := low; port <= high; port++ {
		candidate := net.JoinHostPort(host, strconv.Itoa(port))
		if taken[candidate] || !available(candidate) {
			continue
		}
		r.address = candidate
		return nil
	}
	return fmt.Errorf("every port from %d to %d is taken; free one or widen portRange", low, high)
}

// takenAddresses are the addresses this seat's other runs already hold.
func (r appRun) takenAddresses() map[string]bool {
	taken := map[string]bool{r.contract.Address: true}
	keys, err := applaunch.Keys(r.roots.StateRoot)
	if err != nil {
		return taken
	}
	for _, key := range keys {
		if key == r.key {
			continue
		}
		if record, err := applaunch.ReadRecord(r.roots.StateRoot, key); err == nil && record.Address != "" {
			taken[record.Address] = true
		}
	}
	return taken
}

func available(address string) bool {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	return listener.Close() == nil
}

// preserveRunEvidence copies a goal's run evidence — the log tail and the
// last check — to the durable evidence root before the record is removed and
// before the tree is reclaimed, as the proof runner preserves evidence
// before it disposes of a candidate. The record's own goal decides: every
// spelling of a goal's run is the same run, and the invocation's goal is a
// convenience only. A goal run whose evidence cannot be copied is refused
// closure, so the record and the tree stay for the next stop or start.
func (r appRun) preserveRunEvidence(record *applaunch.Record, out io.Writer) error {
	if record == nil {
		return nil
	}
	goal := record.Goal
	if goal == "" {
		goal = r.goal
	}
	if goal == "" {
		return nil
	}
	resolved, err := config.ResolveEvidenceRoot(config.EvidenceRootParams{
		ConfPath: filepath.Join(r.roots.Installation, "metasystem.conf"), LookupEnv: r.lookupEnv})
	if err != nil {
		return fmt.Errorf("%v, so goal %s's run evidence was not copied and its record is kept; fix the evidence root, then stop or start again to close the run", err, goal)
	}
	root := resolved.Path
	staging, err := os.MkdirTemp("", "app-evidence.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	summary := strings.Join(append([]string{"run " + r.key}, applaunch.Status{Key: r.key, Record: record}.Lines()...), "\n") + "\n"
	summaryPath := filepath.Join(staging, "run.txt")
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return err
	}
	sources := []string{summaryPath}
	if lines, err := applaunch.Tail(record.Log, 500); err == nil && len(lines) > 0 {
		tailPath := filepath.Join(staging, "log-tail.txt")
		if err := os.WriteFile(tailPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
		sources = append(sources, tailPath)
	}
	destination := filepath.Join(root, "goals", goal, "app", r.key, time.Now().UTC().Format("20060102T150405.000000000Z"))
	result, err := proofrun.PreserveEvidence(destination, sources, 32*1024*1024)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "evidence: %d byte(s) copied to %s\n", result.CopiedBytes, destination)
	return nil
}

// endRun is the bookkeeping after a run is over: the evidence copy first,
// then the record, and only then may the tree be reclaimed.
func (r *appRun) endRun(record *applaunch.Record, clean bool, out io.Writer) error {
	if err := r.preserveRunEvidence(record, out); err != nil {
		return err
	}
	if err := applaunch.RemoveRecord(r.roots.StateRoot, r.key); err != nil {
		return err
	}
	if clean {
		return r.reclaimWorktree(out)
	}
	return nil
}

// appEngine names the engine that supervises a run. It is this executable,
// as `ui start` launches this executable; it is a variable only so that a
// test can point at an engine it built for the purpose.
var appEngine = os.Executable

// launchSupervisor starts this run's supervisor detached and waits for its
// one readiness answer.
func (r appRun) launchSupervisor() (string, error) {
	executable, err := appEngine()
	if err != nil {
		return "", fmt.Errorf("the engine executable is unavailable: %w", err)
	}
	spec := applaunch.LaunchSpec{
		Executable: executable,
		Args:       applaunch.ServeArgs(r.roots.Checkout, r.roots.Installation, r.key, r.ref, r.goal, r.address),
		Dir:        r.roots.Checkout,
		LogPath:    filepath.Join(applaunch.Dir(r.roots.StateRoot), r.key+".launch.log"),
	}
	address, _, err := applaunch.LaunchSupervisor(spec, applaunch.ExecSpawn,
		time.Duration(r.contract.ReadyWaitMS())*time.Millisecond+10*time.Second,
		time.Duration(r.contract.StopWaitMS())*time.Millisecond+5*time.Second)
	return address, err
}

// runAppServe is the supervisor: one process that owns one run of the
// application for its life. It is internal because a person never types it;
// `app start` launches it the way `ui start` launches the interface.
func runAppServe(args []string) int {
	flags := flag.NewFlagSet("app serve", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout path (default: the checkout that contains the installation)")
	root := flags.String("metasystem-root", "", "metasystem installation")
	key := flags.String("key", applaunch.StandingKey, "the run's key")
	at := flags.String("at", "", "the ref this run is at")
	goal := flags.String("goal", "", "the goal this run serves")
	address := flags.String("address", "", "the address this run listens on")
	readyFD := flags.Int("ready-fd", -1, "readiness pipe descriptor")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *readyFD < -1 {
		fmt.Fprintln(os.Stderr, "invalid arguments for app serve")
		return 2
	}
	var ready *os.File
	if *readyFD >= 0 {
		ready = applaunch.ReadinessPipe(*readyFD)
		defer ready.Close()
	}
	report := func(line string) {
		if ready != nil {
			fmt.Fprintln(ready, line)
			_ = ready.Close()
			ready = nil
		}
	}
	refuse := func(line string) int {
		report("failed " + line)
		fmt.Fprintln(os.Stderr, line)
		return 1
	}
	installation := *root
	if installation == "" {
		installation = *repo
	}
	roots, err := lifecycle.ResolveRoots(*repo, installation)
	if err != nil {
		return refuse(err.Error())
	}
	_, contract, contractPath, err := loadPhysicalLaunchContract(roots.Installation)
	if err != nil {
		return refuse(err.Error())
	}
	run := resolveAppRun(roots, contract, contractPath, *at, *goal)
	if run.key != *key {
		return refuse(fmt.Sprintf("the run key %q does not name the ref %q", *key, *at))
	}
	run.address = *address
	if *at != "" {
		if commit, from, err := run.resolveCommitFor(*at); err == nil {
			run.commit, run.resolvedFrom = commit, from
		}
	}
	tools, err := launchToolLines(run.tree, contract)
	if err != nil {
		return refuse(err.Error())
	}
	seed := run.seedRecord()
	seed.Tools = tools
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	err = applaunch.Supervise(applaunch.SuperviseOptions{
		Context:     ctx,
		StateRoot:   roots.StateRoot,
		Seed:        seed,
		Contract:    contract,
		ProjectRoot: run.tree,
		Environment: run.environment(),
		Ready:       func(address string) { report("ready " + address) },
		Failed:      func(message string) { report("failed " + message) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// preflightTools is the start's check of the declared tools in the run's own
// tree, before anything is prepared, built or started.
func (r appRun) preflightTools() error {
	_, err := launchToolLines(r.tree, r.contract)
	return err
}
