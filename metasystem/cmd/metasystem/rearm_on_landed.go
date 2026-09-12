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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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

// landedRearmDecision is the decision: nothing to do, re-arm, or a refusal
// in plain words.
type landedRearmDecision struct {
	Rearm   bool
	Refusal string
}

func landedRearmCommand(checkout string) string {
	return "git fetch origin && git reset --hard origin/main && scripts/agents/go-build.sh && bin/metasystem up --repo " + checkout
}

// decideLandedRearm applies DONE's two clauses: a run whose enrolled engine
// is behind the tip by landed commits only re-arms itself; a run whose
// checkout has commits the remote lacks, or is dirty in an engine input,
// refuses with the typed reason naming the two commits and the one command.
func decideLandedRearm(facts landedRearmFacts, checkout string) landedRearmDecision {
	if facts.SourceOwnsTip {
		return landedRearmDecision{}
	}
	prefix := fmt.Sprintf("TEST_POLICY_ENGINE_REQUIRED: the enrolled engine was built from %s and the landed tip of %s is %s", facts.Source, facts.LandingRef, facts.Tip)
	if facts.FetchErr != nil {
		return landedRearmDecision{Refusal: fmt.Sprintf("%s as last fetched; the landing ref could not be fetched (%v), so the run cannot bring the checkout to a landed tip; run: %s", prefix, facts.FetchErr, landedRearmCommand(checkout))}
	}
	if !facts.HeadIsAncestor {
		return landedRearmDecision{Refusal: fmt.Sprintf("%s; the checkout's HEAD %s is not an ancestor of that tip, so the run cannot bring the checkout to it; run: %s", prefix, facts.Head, landedRearmCommand(checkout))}
	}
	if len(facts.DirtyEnginePaths) > 0 {
		return landedRearmDecision{Refusal: fmt.Sprintf("%s; the checkout is dirty in engine inputs (%s), so the run does not rebuild under them; land or stash them, then run: %s", prefix, strings.Join(facts.DirtyEnginePaths, ", "), landedRearmCommand(checkout))}
	}
	if facts.NamedDeliveryTree {
		return landedRearmDecision{Refusal: fmt.Sprintf("%s; this delivery run names the exact index it proves, and bringing the checkout to the tip would move that index under it; carry the staged work onto the tip first (git fetch origin && git merge --ff-only origin/main, or a three-way apply), rebuild and re-arm: %s", prefix, landedRearmCommand(checkout))}
	}
	if len(facts.LiveAttempts) > 0 {
		return landedRearmDecision{Refusal: fmt.Sprintf("%s; a proof attempt of this installation is live (%s) and the engine is never rebuilt under a live attempt; rerun when it has ended, or then run: %s", prefix, strings.Join(facts.LiveAttempts, ", "), landedRearmCommand(checkout))}
	}
	return landedRearmDecision{Rearm: true}
}

// landingRefParts reads the landing ref the way the trusted policy base
// does and splits it into the remote and branch the fetch needs.
func landingRefParts(projectRoot string) (ref, remote, branch string, err error) {
	command := exec.Command("git", "-C", projectRoot, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref")
	command.Env = gittree.ScrubbedEnviron()
	data, err := command.Output()
	ref = strings.TrimSpace(string(data))
	tail := strings.TrimPrefix(ref, "refs/remotes/")
	remote, branch, qualified := strings.Cut(tail, "/")
	if err != nil || tail == ref || !qualified || remote == "" || branch == "" {
		return "", "", "", fmt.Errorf("trusted testing policy base requires local metasystem.steward.landing-ref shaped refs/remotes/<remote>/<branch>")
	}
	return ref, remote, branch, nil
}

// landedRearmGitPins keep git on one behavior whatever the checkout's
// configuration says: no replacement objects (a replaced commit could make
// a divergent HEAD look ancestral), no hooks on the fast-forward, no
// background maintenance under a rebuild.
var landedRearmGitPins = []string{"-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}

func landedRearmGit(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append(append([]string{"-C", dir}, landedRearmGitPins...), args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// fetchLandingRef brings the landing ref to the remote's tip, bounded like
// the re-arm resolver. A fixture installation may carry the landing ref
// without a remote to fetch it from: then, as when the remote is
// unreachable, the local ref is judged as before and only a re-arm is
// refused (a re-arm from a tip that could not be fetched is a stale base).
func fetchLandingRef(ctx context.Context, projectRoot, remote, branch string) error {
	_, err := landedRearmGit(ctx, projectRoot, "fetch", "--quiet", remote, branch)
	return err
}

// dirtyEnginePaths lists the paths changed against HEAD (index or working
// tree) or untracked that the behavior-surface policy classifies as ENGINE,
// top-relative under the installation prefix: the same classification
// go-build.sh applies before it stamps a build dirty.
func dirtyEnginePaths(ctx context.Context, installation, prefix string) ([]string, error) {
	policy, err := behaviorsurface.Load()
	if err != nil {
		return nil, err
	}
	// --no-relative: an inherited diff.relative would strip the installation
	// prefix and let every engine path slip past the classification.
	changed, err := landedRearmGit(ctx, installation, "diff", "--name-only", "--no-renames", "--no-relative", "-z", "HEAD", "--")
	if err != nil {
		return nil, err
	}
	untracked, err := landedRearmGit(ctx, installation, "ls-files", "--others", "--exclude-standard", "--full-name", "-z")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var dirty []string
	for _, listing := range []string{changed, untracked} {
		for _, path := range strings.Split(listing, "\x00") {
			path = strings.TrimSpace(path)
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			engine, classifyErr := policy.Includes(behaviorsurface.Engine, path, prefix)
			if classifyErr != nil {
				return nil, classifyErr
			}
			if engine {
				dirty = append(dirty, path)
			}
		}
	}
	sort.Strings(dirty)
	return dirty, nil
}

// readLandedRearmFacts fetches the landing ref and reads the three facts the
// decision needs: source names the enrolled engine's commit, ownsTip says
// whether that engine still owns the ENGINE projection at a commit.
func readLandedRearmFacts(ctx context.Context, installation, projectRoot, prefix, source string, ownsTip func(tip string) bool) (landedRearmFacts, error) {
	ref, remote, branch, err := landingRefParts(projectRoot)
	if err != nil {
		return landedRearmFacts{}, err
	}
	facts := landedRearmFacts{LandingRef: ref, Remote: remote, Branch: branch, Source: source}
	facts.FetchErr = fetchLandingRef(ctx, projectRoot, remote, branch)
	workspace := gittree.Workspace{Dir: projectRoot}
	facts.Tip, err = workspace.ResolveCommit(ref)
	if err != nil {
		return facts, fmt.Errorf("resolve trusted testing policy base %s: %w", ref, err)
	}
	head, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return facts, fmt.Errorf("testing requires a committed project HEAD")
	}
	facts.Head = head
	facts.SourceOwnsTip = ownsTip(facts.Tip)
	if facts.SourceOwnsTip {
		return facts, nil
	}
	_, ancestorErr := landedRearmGit(ctx, projectRoot, "merge-base", "--is-ancestor", head, facts.Tip)
	facts.HeadIsAncestor = ancestorErr == nil
	facts.DirtyEnginePaths, err = dirtyEnginePaths(ctx, installation, prefix)
	if err != nil {
		return facts, err
	}
	attempts, err := proofrun.ReadAttempts(installation)
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
	landedRearmFastForward = func(ctx context.Context, projectRoot, tip string) error {
		_, err := landedRearmGit(ctx, projectRoot, "merge", "--ff-only", tip)
		return err
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
	if err := landedRearmFastForward(ctx, projectRoot, facts.Tip); err != nil {
		return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: fast-forward the checkout to the landed tip %s: %w; run: %s", facts.Tip, err, landedRearmCommand(projectRoot))
	}
	if err := landedRearmRebuild(ctx, installation); err != nil {
		return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: rebuild the engine at the landed tip %s: %w; run: %s", facts.Tip, err, landedRearmCommand(projectRoot))
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
			return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: re-arm the enrollment on the landed tip %s: %w", facts.Tip, upErr)
		}
		return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: re-arm the enrollment on the landed tip %s: the enrollment did not advance past generation %d (%v; up said: %s)", facts.Tip, previousGeneration, openErr, result.Line)
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
		return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: take the proof mutation lock before rebuilding the engine: %w", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	defer cancel()
	source := pinned.Install.LandedCommit
	if source == "" {
		source = pinned.BuildStamp()
	}
	facts, err := readLandedRearmFacts(ctx, installation, projectRoot, prefix, source, func(tip string) bool {
		return pinned.VerifySourceAtDestination(installation, tip) == nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("TEST_POLICY_ENGINE_REQUIRED: judging the enrolled engine against the landed tip exceeded %d seconds", seconds)
		}
		return nil, err
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
