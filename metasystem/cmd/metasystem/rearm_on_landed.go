package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginecause"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// A landed engine is trusted by its landing. When another seat lands engine
// code, this checkout's enrolled engine falls behind the landing ref's ENGINE
// projection and every engine-driven run refuses until a person fetches,
// resets, rebuilds and re-arms. The run does those steps itself when, and
// only when, the skew is landed commits alone: the landing ref is fetched,
// the checkout is fast-forwarded to its tip, the engine is rebuilt there and
// the enrollment is re-armed through the same path `metasystem up --repo`
// takes. A checkout with local commits the remote lacks, or with changes in
// an ENGINE-projection path, keeps the manual path and its typed refusal.

// landedRearmFacts is what the decision is made from.
type landedRearmFacts struct {
	LandingRef       string
	Remote           string
	Branch           string
	Tip              string
	Head             string
	Source           string
	SourceOwnsTip    bool
	HeadIsAncestor   bool
	DirtyEnginePaths []string
	BlockingPaths    []landedRearmBlocker
	// FetchErr is set when the landing ref could not be fetched (no such
	// remote, or the remote unreachable): the local ref is judged as today,
	// and a re-arm from a tip that could not be fetched is refused.
	FetchErr error
	// LiveAttempts names this installation's proof attempts without a
	// terminal: a rebuild under them is what the seats' box forbids.
	LiveAttempts []string
	// NamedDeliveryTree says the caller named the exact index it proves (a
	// delivery run with --tree, the landing receipt): a fast-forward would
	// move that index under the receipt, so such a run keeps the manual path.
	NamedDeliveryTree bool
}

type landedRearmBlocker struct {
	Path string
	Kind string
}

// landedRearmDecision is the decision: nothing to do, re-arm, or a refusal
// in plain words.
type landedRearmDecision struct {
	Rearm   bool
	Refusal string
}

func landedRearmCommand(checkout string) string {
	return "scripts/agents/go-build.sh && bin/metasystem up --repo " + shellQuote(checkout)
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

// decideLandedRearm applies DONE's two clauses: a run whose enrolled engine
// is behind the tip by landed commits only re-arms itself; a run whose
// checkout has commits the remote lacks, or is dirty in an engine input,
// refuses with the typed reason naming the two commits and the one command.
func decideLandedRearm(facts landedRearmFacts, checkout string) landedRearmDecision {
	if facts.SourceOwnsTip {
		return landedRearmDecision{}
	}
	observed := []enginecause.Fact{
		enginecause.Value("source", facts.Source), enginecause.Value("landing-ref", facts.LandingRef), enginecause.Value("tip", facts.Tip),
		enginecause.Value("remote", facts.Remote), enginecause.Path("checkout", checkout),
	}
	if facts.FetchErr != nil {
		outcomeFacts := append(observed, enginecause.Value("fact", "fetch-failed"))
		return landedRearmDecision{Refusal: engineRefusal("engine-behind-tip", outcomeFacts, fmt.Sprintf("the landing ref could not be fetched (%v), so the run cannot bring the checkout to a landed tip", facts.FetchErr)).Error()}
	}
	if !facts.HeadIsAncestor {
		observed = append(observed, enginecause.Value("fact", "head-diverged"), enginecause.Value("head", facts.Head))
		return landedRearmDecision{Refusal: engineRefusal("engine-behind-tip", observed, "the checkout's HEAD is not an ancestor of that tip, so the run cannot bring the checkout to it").Error()}
	}
	if len(facts.BlockingPaths) > 0 {
		blockerFacts := append([]enginecause.Fact(nil), observed...)
		for _, blocker := range facts.BlockingPaths {
			blockerFacts = append(blockerFacts, enginecause.Path(blocker.Kind+"-path", blocker.Path))
		}
		return landedRearmDecision{Refusal: engineRefusal("fast-forward-blocked", blockerFacts,
			"dirty or untracked checkout paths collide with paths changed by the landed tip, so the run refuses before changing the checkout").Error()}
	}
	if len(facts.DirtyEnginePaths) > 0 {
		observed = append(observed, enginecause.Value("fact", "dirty-engine-paths"))
		for _, path := range facts.DirtyEnginePaths {
			observed = append(observed, enginecause.Path("path", path))
		}
		return landedRearmDecision{Refusal: engineRefusal("engine-behind-tip", observed, "the checkout is dirty in engine inputs ("+strings.Join(facts.DirtyEnginePaths, ", ")+"), so the run does not rebuild under them").Error()}
	}
	if facts.NamedDeliveryTree {
		observed = append(observed, enginecause.Value("fact", "named-delivery-tree"))
		return landedRearmDecision{Refusal: engineRefusal("engine-behind-tip", observed, "this delivery run names the exact index it proves, and bringing the checkout to the tip would move that index under it").Error()}
	}
	if len(facts.LiveAttempts) > 0 {
		observed = append(observed, enginecause.Value("fact", "live-attempt"), enginecause.Value("attempt", facts.LiveAttempts[0]))
		return landedRearmDecision{Refusal: engineRefusal("engine-behind-tip", observed, "a proof attempt of this installation is live ("+strings.Join(facts.LiveAttempts, ", ")+") and the engine is never rebuilt under a live attempt").Error()}
	}
	return landedRearmDecision{Rearm: true}
}

// landingRefParts reads the landing ref the way the trusted policy base
// does and splits it into the remote and branch the fetch needs.
func landingRefParts(ctx context.Context, clock steward.RearmClock, seconds int, projectRoot string) (ref, remote, branch string, err error) {
	ref, err = landedRearmGitStep(ctx, clock, seconds, "read-landing-ref", projectRoot, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	if err != nil {
		if errors.Is(err, steward.ErrJudgmentStalled) {
			return "", "", "", err
		}
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			return "", "", "", err
		}
	}
	tail := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if tail == ref || !qualified || remote == "" || branch == "" {
		return "", "", "", fmt.Errorf("trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>")
	}
	return ref, remote, branch, nil
}

// landedRearmGitPins keep git on one behavior whatever the checkout's
// configuration says: no replacement objects (a replaced commit could make
// a divergent HEAD look ancestral), no hooks on the fast-forward, no
// background maintenance under a rebuild.
var landedRearmGitPins = []string{"-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}

type landedRearmGitCommandFactory func(context.Context, ...string) *exec.Cmd
type landedRearmGitCommandContextKey struct{}

func landedRearmGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	if factory, ok := ctx.Value(landedRearmGitCommandContextKey{}).(landedRearmGitCommandFactory); ok {
		return factory(ctx, args...)
	}
	return exec.CommandContext(ctx, "git", args...)
}

func landedRearmGitStep(ctx context.Context, clock steward.RearmClock, seconds int, step, dir string, args ...string) (string, error) {
	output, err := landedRearmGitOutputStep(ctx, clock, seconds, step, dir, args...)
	return strings.TrimSpace(output), err
}

func landedRearmGitOutputStep(ctx context.Context, clock steward.RearmClock, seconds int, step, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := steward.RunRearmStep(ctx, clock, time.Duration(seconds)*time.Second, step, func(stepContext context.Context, progress func()) error {
		command := landedRearmGitCommand(stepContext, append(append([]string{"-C", dir}, landedRearmGitPins...), args...)...)
		if command.Env == nil {
			command.Env = gittree.ScrubbedEnviron()
		}
		command.Stdout = steward.RearmProgressWriter(&stdout, progress)
		command.Stderr = steward.RearmProgressWriter(&stderr, progress)
		return command.Run()
	})
	if err != nil {
		if errors.Is(err, steward.ErrJudgmentStalled) {
			return "", err
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// fetchLandingRef brings the landing ref to the remote's tip, bounded like
// the re-arm resolver. A fixture installation may carry the landing ref
// without a remote to fetch it from: then, as when the remote is
// unreachable, the local ref is judged as before and only a re-arm is
// refused (a re-arm from a tip that could not be fetched is a stale base).
func fetchLandingRef(ctx context.Context, clock steward.RearmClock, seconds int, projectRoot, remote, branch string) error {
	_, err := landedRearmGitStep(ctx, clock, seconds, "fetch-landing-ref", projectRoot, "fetch", "--progress", remote, branch)
	return err
}

// dirtyEnginePaths lists the paths changed against HEAD (index or working
// tree) or untracked that the behavior-surface policy classifies as ENGINE,
// top-relative under the installation prefix: the same classification
// go-build.sh applies before it stamps a build dirty.
type checkoutDirtyPath struct {
	path      string
	untracked bool
}

func dirtyCheckoutPaths(ctx context.Context, clock steward.RearmClock, seconds int, installation string) ([]checkoutDirtyPath, error) {
	// --no-relative: an inherited diff.relative would strip the installation
	// prefix and let every engine path slip past the classification.
	changed, err := landedRearmGitOutputStep(ctx, clock, seconds, "list-dirty-paths", installation, "diff", "--name-only", "--no-renames", "--no-relative", "-z", "HEAD", "--")
	if err != nil {
		return nil, err
	}
	untracked, err := landedRearmGitOutputStep(ctx, clock, seconds, "list-untracked-paths", installation, "ls-files", "--others", "--exclude-standard", "--full-name", "-z")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var dirty []checkoutDirtyPath
	for index, listing := range []string{changed, untracked} {
		for _, path := range strings.Split(listing, "\x00") {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			dirty = append(dirty, checkoutDirtyPath{path: path, untracked: index == 1})
		}
	}
	sort.Slice(dirty, func(left, right int) bool { return dirty[left].path < dirty[right].path })
	return dirty, nil
}

func dirtyEnginePaths(ctx context.Context, clock steward.RearmClock, seconds int, installation, prefix string) ([]string, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return nil, err
	}
	dirty, err := dirtyCheckoutPaths(ctx, clock, seconds, installation)
	if err != nil {
		return nil, err
	}
	var enginePaths []string
	for _, item := range dirty {
		engine, classifyErr := policy.Includes(behaviorsurface.Engine, item.path, prefix)
		if classifyErr != nil {
			return nil, classifyErr
		}
		if engine {
			enginePaths = append(enginePaths, item.path)
		}
	}
	return enginePaths, nil
}

func dirtyBlockingPaths(ctx context.Context, clock steward.RearmClock, seconds int, installation, prefix, head, tip string) ([]landedRearmBlocker, error) {
	dirty, err := dirtyCheckoutPaths(ctx, clock, seconds, installation)
	if err != nil {
		return nil, err
	}
	changed, err := landedRearmGitOutputStep(ctx, clock, seconds, "list-landed-changed-paths", installation,
		"diff", "--name-only", "--no-renames", "--no-relative", "-z", head, tip, "--")
	if err != nil {
		return nil, err
	}
	changedPaths := strings.Split(changed, "\x00")
	var blockers []landedRearmBlocker
	for _, item := range dirty {
		for _, incoming := range changedPaths {
			if incoming == "" || !gittree.PathsCollide(item.path, incoming) {
				continue
			}
			kind := "tracked"
			if item.untracked {
				kind = "untracked"
			}
			installationPath := strings.TrimPrefix(item.path, strings.TrimSuffix(prefix, "/")+"/")
			if enginecause.IsAppendOnlyLedger(installationPath) {
				kind = "ledger"
			}
			blockers = append(blockers, landedRearmBlocker{Path: item.path, Kind: kind})
			break
		}
	}
	return blockers, nil
}

var (
	landedRearmClock    steward.RearmClock = steward.SystemRearmClock()
	landedRearmFetch                       = fetchLandingRef
	landedRearmAncestry                    = func(ctx context.Context, clock steward.RearmClock, seconds int, projectRoot, head, tip string) (bool, error) {
		_, err := landedRearmGitStep(ctx, clock, seconds, "compare-checkout-ancestry", projectRoot, "merge-base", "--is-ancestor", head, tip)
		if err == nil {
			return true, nil
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	landedRearmDirty    = dirtyEnginePaths
	landedRearmBlockers = dirtyBlockingPaths
	landedRearmAttempts = proofrun.ReadAttempts
)

// readLandedRearmFacts fetches the landing ref and reads the three facts the
// decision needs: source names the enrolled engine's commit, ownsTip says
// whether that engine still owns the ENGINE projection at a commit.
func readLandedRearmFacts(ctx context.Context, clock steward.RearmClock, seconds int, installation, projectRoot, prefix, source string, ownsTip func(tip string) (bool, error)) (landedRearmFacts, error) {
	ref, remote, branch, err := landingRefParts(ctx, clock, seconds, projectRoot)
	if err != nil {
		return landedRearmFacts{}, err
	}
	facts := landedRearmFacts{LandingRef: ref, Remote: remote, Branch: branch, Source: source}
	facts.FetchErr = landedRearmFetch(ctx, clock, seconds, projectRoot, remote, branch)
	if errors.Is(facts.FetchErr, steward.ErrJudgmentStalled) {
		return facts, facts.FetchErr
	}
	facts.Tip, err = landedRearmGitStep(ctx, clock, seconds, "resolve-landing-tip", projectRoot, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return facts, fmt.Errorf("resolve trusted testing policy base %s: %w", ref, err)
	}
	head, err := landedRearmGitStep(ctx, clock, seconds, "resolve-checkout-head", projectRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		if errors.Is(err, steward.ErrJudgmentStalled) {
			return facts, err
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return facts, fmt.Errorf("testing requires a committed project HEAD")
		}
		return facts, fmt.Errorf("resolve-checkout-head: %w", err)
	}
	facts.Head = head
	facts.SourceOwnsTip, err = ownsTip(facts.Tip)
	if err != nil {
		return facts, err
	}
	if facts.SourceOwnsTip {
		return facts, nil
	}
	facts.HeadIsAncestor, err = landedRearmAncestry(ctx, clock, seconds, projectRoot, head, facts.Tip)
	if err != nil {
		return facts, err
	}
	facts.DirtyEnginePaths, err = landedRearmDirty(ctx, clock, seconds, installation, prefix)
	if err != nil {
		return facts, err
	}
	facts.BlockingPaths, err = landedRearmBlockers(ctx, clock, seconds, installation, prefix, head, facts.Tip)
	if err != nil {
		return facts, err
	}
	var attempts []proofrun.Attempt
	err = steward.RunRearmStep(ctx, clock, time.Duration(seconds)*time.Second, "read-proof-attempts", func(context.Context, func()) error {
		var readErr error
		attempts, readErr = landedRearmAttempts(installation)
		return readErr
	})
	if err != nil {
		return facts, fmt.Errorf("read this installation's proof attempts: %w", err)
	}
	for _, attempt := range attempts {
		if attempt.Terminal == nil {
			facts.LiveAttempts = append(facts.LiveAttempts, attempt.AttemptID)
		}
	}
	return facts, nil
}

// The three acts of a re-arm, behind seams a fixture can watch.
var (
	landedRearmFastForward = func(ctx context.Context, installation, tip string) error {
		return landing.FastForwardPreservingRegisters(ctx, installation, tip)
	}
	landedRearmRebuild = func(ctx context.Context, installation string) error {
		command := exec.CommandContext(ctx, "bash", "scripts/agents/go-build.sh")
		command.Dir = installation
		command.Env = os.Environ()
		out, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("scripts/agents/go-build.sh: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	// landedRearmUp runs the REBUILT binary's own `up --repo <checkout>`:
	// the enrollment rules that mint the generation are the landed ones,
	// not this process's older copy, and the options are exactly what a
	// person's `metasystem up --repo` would build from this shell. The
	// result line is returned as up printed it.
	landedRearmUp = func(ctx context.Context, installation, projectRoot string) (upOutcome, error) {
		command := exec.CommandContext(ctx, filepath.Join(installation, "bin", "metasystem"), "up", "--repo", projectRoot)
		command.Dir = installation
		command.Env = os.Environ()
		out, err := command.CombinedOutput()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		last := strings.TrimSpace(lines[len(lines)-1])
		outcome := upOutcome{Line: last, Failed: err != nil}
		if match := upOutcomeField.FindStringSubmatch(last); match != nil {
			outcome.Outcome = match[1]
		}
		if err != nil {
			return outcome, fmt.Errorf("bin/metasystem up --repo %s: %w: %s", projectRoot, err, last)
		}
		return outcome, nil
	}
	// landedRearmReexec restarts the run on the re-armed engine with the
	// record in its environment, so the whole run, its watchdog and its
	// authenticated helpers are one generation. It returns only on failure.
	landedRearmReexec = func(record *proofrun.EngineRearm) error {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			return err
		}
		return syscall.Exec(binary, os.Args, append(os.Environ(), engineRearmEnv+"="+string(encoded)))
	}
)

// engineRearmEnv carries the record of a re-arm across the re-exec, and is
// the loop guard: a run that finds it never re-arms again.
const engineRearmEnv = "METASYSTEM_ENGINE_REARM"

// upOutcome is what the rebuilt engine's up said, as it said it.
type upOutcome struct {
	Line    string
	Outcome string
	Failed  bool
}

var upOutcomeField = regexp.MustCompile(`\boutcome=([A-Za-z_-]+)`)

// performLandedRearm brings the checkout to the tip, rebuilds the engine
// there and re-arms the enrollment; the record names what changed.
func performLandedRearm(ctx context.Context, installation, projectRoot string, facts landedRearmFacts, previousGeneration int) (*proofrun.EngineRearm, error) {
	if err := landedRearmFastForward(ctx, installation, facts.Tip); err != nil {
		facts := append(engineCheckoutFacts(projectRoot), enginecause.Value("tip", facts.Tip))
		return nil, engineRefusal("fast-forward-blocked", facts, fmt.Sprintf("fast-forward the checkout to the landed tip: %v", err))
	}
	if err := landedRearmRebuild(ctx, installation); err != nil {
		facts := append(engineCheckoutFacts(projectRoot), enginecause.Value("tip", facts.Tip))
		return nil, engineRefusal("rebuild-failed", facts, fmt.Sprintf("rebuild the engine at the landed tip: %v", err))
	}
	// up mints the generation first and proves the session and the
	// components after; a run launched detached (no runtime ancestor) or
	// beside a closed fence gets a failed up whose enrollment is already
	// re-armed. The enrollment is what the run needs: it continues on the
	// re-opened generation and records up's outcome; only an enrollment
	// that did not advance is a refusal, with up's own line.
	result, upErr := landedRearmUp(ctx, installation, projectRoot)
	record := &proofrun.EngineRearm{SourceCommit: facts.Source, LandedTip: facts.Tip, PreviousGeneration: previousGeneration,
		ReArmed: result.Line, UpOutcome: result.Outcome, At: time.Now().UTC().Format(time.RFC3339Nano)}
	pinned, openErr := landedRearmOpenEnrollment(installation)
	if openErr != nil || pinned.Generation <= previousGeneration {
		if upErr != nil {
			return nil, engineRefusal("rearm-failed", engineCheckoutFacts(projectRoot), fmt.Sprintf("re-arm the enrollment on the landed tip %s: %v", facts.Tip, upErr))
		}
		return nil, engineRefusal("rearm-failed", engineCheckoutFacts(projectRoot), fmt.Sprintf("re-arm the enrollment on the landed tip %s: the enrollment did not advance past generation %d (%v; up said: %s)", facts.Tip, previousGeneration, openErr, result.Line))
	}
	record.Generation = pinned.Generation
	return record, nil
}

// landedRearmAct is the guarded act: the decision, and only on "re-arm" the
// three acts under the installation's proof mutation lock, so no attempt is
// admitted while the engine is rebuilt (the seats' box rule, held by the
// same lock admission takes).
func landedRearmAct(ctx context.Context, installation, projectRoot string, facts landedRearmFacts, previousGeneration int) (*proofrun.EngineRearm, error) {
	decision := decideLandedRearm(facts, projectRoot)
	if decision.Refusal != "" {
		return nil, errors.New(decision.Refusal)
	}
	if !decision.Rearm {
		return nil, nil
	}
	lock, err := landedRearmMutationLock(installation)
	if err != nil {
		return nil, engineRefusal("mutation-lock", engineCheckoutFacts(projectRoot), "take the proof mutation lock before rebuilding the engine: "+err.Error())
	}
	defer lock()
	// The live check is repeated under the lock: an attempt admitted
	// between the first read and the lock would otherwise be rebuilt under.
	attempts, err := proofrun.ReadAttempts(installation)
	if err != nil {
		return nil, fmt.Errorf("read this installation's proof attempts: %w", err)
	}
	for _, attempt := range attempts {
		if attempt.Terminal == nil {
			return nil, errors.New(decideLandedRearm(withLiveAttempt(facts, attempt.AttemptID), projectRoot).Refusal)
		}
	}
	return performLandedRearm(ctx, installation, projectRoot, facts, previousGeneration)
}

func withLiveAttempt(facts landedRearmFacts, id string) landedRearmFacts {
	facts.LiveAttempts = append([]string{id}, facts.LiveAttempts...)
	return facts
}

// landedRearmMutationLock is admission's own lock; a fixture without an
// installation answers with a no-op release.
var landedRearmMutationLock = func(installation string) (func(), error) {
	lock, err := proofrun.AcquireMutation(installation)
	if err != nil {
		return nil, err
	}
	return func() { _ = lock.Release() }, nil
}

// landedRearmOpenEnrollment reads the enrollment after a re-arm; a fixture
// answers for an installation it never enrolled.
var landedRearmOpenEnrollment = func(installation string) (steward.InstallIdentity, error) {
	pinned, err := steward.OpenEnrolledBinary(installation)
	if err != nil {
		return steward.InstallIdentity{}, err
	}
	defer pinned.Close()
	return pinned.Install, nil
}

// landedRearm is the step prepareTesting takes before it captures the
// candidate and resolves the policy base. It returns the record of a re-arm
// it performed (nil when the enrolled engine already owned the tip) and the
// index tree as it stood before any fast-forward, so a delivery run that
// named that tree can follow the index onto the tip.
func landedRearm(installation, projectRoot, prefix string, namedDeliveryTree bool) (*proofrun.EngineRearm, error) {
	pinned, openErr := steward.OpenEnrolledBinary(installation)
	if openErr != nil {
		// Not enrolled, or rebuilt bytes not yet re-armed: the trusted
		// policy engine step says so in its own words.
		return nil, nil
	}
	if encoded := os.Getenv(engineRearmEnv); encoded != "" {
		// This run is the re-exec of one that already re-armed. The record
		// is believed only when the enrollment says the same thing (that
		// generation, that landed commit); a record the enrollment does not
		// bear is ignored and the run judges the engine like any other.
		var record proofrun.EngineRearm
		if err := json.Unmarshal([]byte(encoded), &record); err == nil &&
			record.Generation == pinned.Install.Generation && record.LandedTip == pinned.Install.LandedCommit {
			_ = pinned.Close()
			return &record, nil
		}
		fmt.Fprintf(os.Stderr, "metasystem test: %s does not match the enrollment (generation %d, landed %s); judging the engine afresh\n", engineRearmEnv, pinned.Install.Generation, pinned.Install.LandedCommit)
	}
	defer pinned.Close()
	seconds := steward.RearmResolveSeconds(installation)
	source := pinned.Install.LandedCommit
	if source == "" {
		source = pinned.BuildStamp()
	}
	facts, err := readLandedRearmFacts(context.Background(), landedRearmClock, seconds, installation, projectRoot, prefix, source, func(tip string) (bool, error) {
		verifyErr := pinned.VerifySourceAtDestinationWithClock(landedRearmClock, installation, tip)
		if errors.Is(verifyErr, steward.ErrNotOwned) {
			return false, nil
		}
		return verifyErr == nil, verifyErr
	})
	if err != nil {
		facts := append(engineCheckoutFacts(projectRoot), enginecause.Value("source", source))
		return nil, judgmentRefusal(err, facts, "judging the enrolled engine against the landed tip failed")
	}
	facts.NamedDeliveryTree = namedDeliveryTree
	if decision := decideLandedRearm(facts, projectRoot); decision.Rearm {
		fmt.Fprintf(os.Stderr, "metasystem test: the enrolled engine (%s) is behind the landed tip %s of %s by landed commits only; fast-forwarding, rebuilding and re-arming\n", facts.Source, facts.Tip, facts.LandingRef)
	}
	// The rebuild and the re-arm are not bounded by the resolver's seconds:
	// a build takes what it takes, and up has its own bounds.
	record, err := landedRearmAct(context.Background(), installation, projectRoot, facts, pinned.Install.Generation)
	if err != nil || record == nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "metasystem test: re-armed %s; restarting this run on the landed engine\n", record.ReArmed)
	if reexecErr := landedRearmReexec(record); reexecErr != nil {
		// The re-exec could not happen; this run continues on its own bytes
		// with the new enrollment as its policy engine, as any run whose
		// invoking binary is not the pin.
		fmt.Fprintf(os.Stderr, "metasystem test: could not restart on the landed engine (%v); continuing with the re-armed enrollment as the policy engine\n", reexecErr)
	}
	return record, nil
}

// runtimeUpCommandFor spells the up invocation the re-arm runs, for the
// fixture that checks it names the rebuilt binary.
func runtimeUpCommandFor(installation, projectRoot string) string {
	return filepath.Join(installation, "bin", "metasystem") + " up --repo " + projectRoot
}
