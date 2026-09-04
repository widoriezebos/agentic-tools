package goal

// The ledger transaction engine: fetch the canonical
// branch to a per-operation ref, rebuild the mutation on exactly
// that tip in an isolated index, commit with the explicit fetched
// parent and the Goal-Transaction trailer, publish by
// compare-and-swap — force-with-lease with the explicit expected
// oid in remote mode, update-ref with the old-value assertion in
// single-machine mode — classify by refetch against the opid
// postcondition, and retry rebuilds under benign advancement until
// the publish deadline. The user's HEAD, index, and worktree never
// participate and never move.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Endpoint is the resolved synchronization endpoint.
type Endpoint struct {
	Root   string // repository worktree root
	Remote string // goal.sync-remote; "local" is single-machine mode
	Branch string // goal.sync-branch, fully qualified
}

// LocalLedgerBranch is the dedicated single-machine ledger branch —
// never the user's checked-out branch, so HEAD provably never moves
// in either mode.
const LocalLedgerBranch = "refs/heads/metasystem/goals"

// AcceptedRef is the clone's accepted-tree pointer, advanced by CAS
// on confirmation (the read-side validator owns its other advances).
const AcceptedRef = "refs/metasystem/goals/accepted"

// DefaultPublishDeadline bounds the benign-advancement retry loop —
// a deadline, not a fixed count: four benign advancements in a row
// are lawful work, not failure.
const DefaultPublishDeadline = 60 * time.Second

// ResolveEndpoint reads goal.sync-remote (default origin) and
// goal.sync-branch (default refs/heads/main) from the repository's
// git configuration.
func ResolveEndpoint(root string) (Endpoint, error) {
	e := Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
	if out, err := gitIn(root, "config", "--get", "goal.sync-remote"); err == nil && strings.TrimSpace(out) != "" {
		e.Remote = strings.TrimSpace(out)
	}
	if out, err := gitIn(root, "config", "--get", "goal.sync-branch"); err == nil && strings.TrimSpace(out) != "" {
		e.Branch = strings.TrimSpace(out)
	}
	if !strings.HasPrefix(e.Branch, "refs/") {
		return Endpoint{}, fmt.Errorf("goal.sync-branch must be fully qualified (refs/...), got %q", e.Branch)
	}
	return e, nil
}

// LocalMode reports the declared single-machine mode.
func (e Endpoint) LocalMode() bool { return e.Remote == "local" }

func fetchRefFor(opid string) string { return "refs/metasystem/goals/fetch/" + opid }
func txnRefFor(opid string) string   { return "refs/metasystem/goals/txn/" + opid }

// goalGit runs one git invocation in the repository with the
// engine's determinism pins.
func goalGit(root string, extraEnv []string, args ...string) (string, error) {
	full := append([]string{
		"-C", root,
		"-c", "core.logAllRefUpdates=false",
	}, args...)
	cmd := exec.Command("git", full...)
	// The scrubbed base, never os.Environ: -C does not defeat an
	// inherited GIT_DIR, and injected config can replace the remote
	// URL — the transaction must be steerable by NOTHING but its
	// arguments.
	cmd.Env = append(environWithoutGitSteering(), extraEnv...)
	// stdout is the PARSED channel and stderr the diagnostic one: a git
	// wrapper printing a warning must never pollute a tip a caller
	// parses (goal-git-stderr-pollution).
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// The ERROR path returns both channels: push-rejection
		// classification (stale lease vs fatal) reads git's stderr
		// voice, and losing it turned every CAS race into "transport
		// unknown". Only the SUCCESS return is the parsed channel.
		combined := stdout.String() + stderr.String()
		return combined, fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// CaptureTip fetches the canonical branch into the per-operation
// ref (concurrent operations in one clone can never rewind a
// shared ref under each other) and returns the captured tip oid.
// Single-machine mode reads the dedicated local ledger branch.
func CaptureTip(e Endpoint, opid string) (string, error) {
	if e.LocalMode() {
		out, err := goalGit(e.Root, nil, "rev-parse", "--verify", LocalLedgerBranch)
		if err != nil {
			// The branch is BORN by the first publication (local
			// migration bootstraps it): until then the checkout HEAD
			// is the world the transaction reads — it carries the
			// legacy sources and no ledger, so ordinary verbs still
			// refuse on their own terms while the migration proceeds.
			headOut, headErr := goalGit(e.Root, nil, "rev-parse", "--verify", "HEAD")
			if headErr != nil {
				return "", fmt.Errorf("no local ledger branch %s and no HEAD to bootstrap from: %v", LocalLedgerBranch, headErr)
			}
			return strings.TrimSpace(headOut), nil
		}
		return strings.TrimSpace(out), nil
	}
	// --refmap= keeps this fetch to EXACTLY the per-op refspec: git
	// otherwise opportunistically updates the remote-tracking ref
	// too, and two concurrent operations then collide on that ONE
	// shared ref's lock ("cannot lock ref 'refs/remotes/...'") — the
	// exact contention the per-op refs exist to prevent — true
	// concurrency hit it on its first certified run.
	if _, err := goalGit(e.Root, nil, "fetch", "--no-tags", "--refmap=", e.Remote,
		"+"+e.Branch+":"+fetchRefFor(opid)); err != nil {
		return "", err
	}
	out, err := goalGit(e.Root, nil, "rev-parse", "--verify", fetchRefFor(opid))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// acceptedTipForGates resolves the accepted ref for gating: a ref
// that provably does not exist gates nothing (pre-bootstrap), but a
// ref that EXISTS and cannot resolve to a commit is a broken world
// that refuses — reading breakage as absence would skip identity and
// descent exactly when they matter most.
func acceptedTipForGates(root string) (string, bool, error) {
	out, err := gitIn(root, "rev-parse", "--verify", "--quiet", AcceptedRef+"^{commit}")
	if err == nil {
		return strings.TrimSpace(out), true, nil
	}
	if _, refErr := gitIn(root, "show-ref", "--verify", "--quiet", AcceptedRef); refErr != nil {
		// Exit 1 is show-ref's word for absent — but git ALSO answers
		// exit 1 for a broken loose ref file, warning and ignoring
		// it. The file's presence is the tell: a ref file git cannot
		// read must refuse, never read as pre-bootstrap.
		var ge *gitError
		if errors.As(refErr, &ge) && ge.ExitCode() == 1 {
			if commonOut, pathErr := gitIn(root, "rev-parse", "--path-format=absolute", "--git-common-dir"); pathErr == nil {
				// The loose-ref path is BUILT from the common dir,
				// never asked of --git-path: git resolves a ref
				// symlink to its TARGET there, and the probe must
				// see the LINK. Lstat for the same reason — a
				// dangling symlink is an existing entry git ignores.
				// Any probe error other than not-exist refuses.
				refPath := filepath.Join(strings.TrimRight(commonOut, "\n"), filepath.FromSlash(AcceptedRef))
				if _, lstatErr := os.Lstat(refPath); lstatErr == nil {
					return "", false, fmt.Errorf("the accepted ref's file exists but git reports no valid ref; repair %s before continuing", AcceptedRef)
				} else if !os.IsNotExist(lstatErr) {
					return "", false, fmt.Errorf("the accepted ref's file cannot be proven absent: %v", lstatErr)
				}
			}
			return "", false, nil
		}
		return "", false, fmt.Errorf("the accepted ref cannot be probed: %v", refErr)
	}
	return "", false, fmt.Errorf("the accepted ref exists but does not resolve to a commit; repair %s before continuing", AcceptedRef)
}

// tipCarriesLedger reports whether the tip has a ledger root at
// all — the pre-migration discriminator the mutation-side
// validation and the sync-mode gate share. Absence and FAILURE are
// different facts: ls-tree answers absence with empty output and
// success, while an execution failure returns the error — a probe
// that cannot answer must never read as "pre-migration, skip
// validation".
func tipCarriesLedger(e Endpoint, tip string) (bool, error) {
	out, err := gitIn(e.Root, "ls-tree", "--name-only", tip, "--", goalsPrefix+"backlog.md")
	if err != nil {
		return false, fmt.Errorf("the tip's ledger presence cannot be proven: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// SyncModeGate refuses the durable/declared mode mismatch at EVERY
// operational boundary (the projection alone checked it): the
// fetched tip's root record speaks for the ledger, the endpoint for
// this clone's config. A tip with no root record is pre-migration
// and gates nothing; a torn record is the validator's to refuse.
func SyncModeGate(e Endpoint, tip string) error {
	hasLedger, probeErr := tipCarriesLedger(e, tip)
	if probeErr != nil {
		return probeErr
	}
	if !hasLedger {
		return nil
	}
	content, err := gitIn(e.Root, "cat-file", "-p", tip+":./"+goalsPrefix+"backlog.md")
	if err != nil {
		// The ledger provably exists; a root that cannot be read
		// gates CLOSED — reporting "already current" over an
		// unreadable world is the exact skip this gate forbids.
		return fmt.Errorf("the tip's root record cannot be read for the sync-mode gate: %v", err)
	}
	record, problems := ParseRoot([]byte(content))
	if record == nil || len(problems) != 0 {
		return fmt.Errorf("the tip's root record does not parse; the sync mode cannot be gated: %v", problems)
	}
	if record.SyncMode == SyncLocal && !e.LocalMode() {
		return fmt.Errorf("sync-mode mismatch refused: the ledger is committed local, the config says remote %q — promotion is the backlog-local-promotion goal, not a config flip", e.Remote)
	}
	if record.SyncMode == SyncRemote && e.LocalMode() {
		return fmt.Errorf("sync-mode mismatch refused: the ledger is committed remote, the config says local — a split brain is not a mode")
	}
	return nil
}

// Change is one path's mutation relative to the fetched tree.
// Content is the whole new file; Delete removes the path.
type Change struct {
	Path    string
	Content []byte
	Delete  bool
}

// BuildCommit rebuilds the mutation on exactly the captured tip: an
// isolated index seeded from the tip's tree, the changes applied,
// and a commit whose parent IS that tip, carrying the
// Goal-Transaction trailer. The commit is stored under the
// operation's temporary ref and returned.
func BuildCommit(e Endpoint, opid, tip string, changes []Change, message string) (string, error) {
	if len(changes) == 0 {
		return "", fmt.Errorf("a transaction mutates at least one path")
	}
	indexDir, err := os.MkdirTemp("", "goal-txn-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(indexDir)
	env := []string{
		"GIT_INDEX_FILE=" + filepath.Join(indexDir, "index"),
		"GIT_AUTHOR_NAME=metasystem", "GIT_AUTHOR_EMAIL=goals@metasystem.invalid",
		"GIT_COMMITTER_NAME=metasystem", "GIT_COMMITTER_EMAIL=goals@metasystem.invalid",
	}
	if _, err := goalGit(e.Root, env, "read-tree", tip+"^{tree}"); err != nil {
		return "", err
	}
	// A root below the git toplevel (the real repository's shape)
	// addresses tree entries under its prefix: --cacheinfo takes RAW
	// toplevel-rooted index paths — unlike the pathspec form, which
	// git prefixes with the working directory itself.
	prefixOut, err := goalGit(e.Root, nil, "rev-parse", "--show-prefix")
	if err != nil {
		return "", err
	}
	treePrefix := strings.TrimSpace(prefixOut)
	for _, c := range changes {
		if c.Delete {
			if _, err := goalGit(e.Root, env, "update-index", "--force-remove", "--", c.Path); err != nil {
				return "", err
			}
			continue
		}
		oid, err := hashObject(e.Root, c.Content)
		if err != nil {
			return "", err
		}
		if _, err := goalGit(e.Root, env, "update-index", "--add",
			"--cacheinfo", "100644,"+oid+","+treePrefix+c.Path); err != nil {
			return "", err
		}
	}
	treeOut, err := goalGit(e.Root, env, "write-tree")
	if err != nil {
		return "", err
	}
	tree := strings.TrimSpace(treeOut)
	full := message + "\n\nGoal-Transaction: " + opid + "\n"
	commitOut, err := goalGit(e.Root, env, "commit-tree", tree, "-p", tip, "-m", full)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(commitOut)
	if _, err := goalGit(e.Root, nil, "update-ref", txnRefFor(opid), commit); err != nil {
		return "", err
	}
	return commit, nil
}

func hashObject(root string, content []byte) (string, error) {
	cmd := exec.Command("git", "-C", root, "hash-object", "-w", "--stdin")
	cmd.Env = environWithoutGitSteering()
	cmd.Stdin = strings.NewReader(string(content))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("hash-object: %v (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CASOutcome is one push attempt's classification.
type CASOutcome string

const (
	CASLanded CASOutcome = "landed"
	// CASRefused: the lease failed — someone advanced the branch.
	// Refetch and classify; retry is lawful.
	CASRefused CASOutcome = "refused"
	// CASUnknown: transport failed with the outcome undetermined —
	// the entry stays pushed and this process stops (the one
	// recovery rule classifies it later).
	CASUnknown CASOutcome = "unknown"
)

// classifyPushFailure separates a lease refusal (definite, retry
// lawful) from transport-unknown (definite nothing).
func classifyPushFailure(output string) CASOutcome {
	lower := strings.ToLower(output)
	// The lost-lease shapes differ by transport: a smart-protocol
	// remote says "stale info", while receive-pack on a file-path
	// remote says "cannot lock ref ... but expected" with "[remote
	// rejected] ... (failed to update ref)". Both ARE the lost
	// compare — the race certification hit the second shape as
	// "unknown" on its first concurrent same-ref push.
	for _, marker := range []string{
		"stale info", "[rejected]", "fetch first", "non-fast-forward",
		"[remote rejected]", "but expected",
	} {
		if strings.Contains(lower, marker) {
			return CASRefused
		}
	}
	return CASUnknown
}

// PublishCAS attempts the compare-and-swap: force-with-lease with
// the EXPLICIT expected oid in remote mode (no plain push exists in
// the protocol), update-ref with the old-value assertion in local
// mode.
func PublishCAS(e Endpoint, tip, commit string) (CASOutcome, error) {
	if e.LocalMode() {
		if _, err := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", LocalLedgerBranch); err != nil {
			// The first publication CREATES the branch, and creation
			// IS the compare: the empty old-value makes update-ref
			// refuse a concurrent creator atomically.
			if out, cErr := goalGit(e.Root, nil, "update-ref", LocalLedgerBranch, commit, ""); cErr != nil {
				return CASRefused, fmt.Errorf("local CAS refused: %s", strings.TrimSpace(out))
			}
			return CASLanded, nil
		}
		if out, err := goalGit(e.Root, nil, "update-ref", LocalLedgerBranch, commit, tip); err != nil {
			// No transport exists locally: a failed old-value
			// assertion is always a lost compare.
			return CASRefused, fmt.Errorf("local CAS refused: %s", strings.TrimSpace(out))
		}
		return CASLanded, nil
	}
	out, err := goalGit(e.Root, nil, "push", e.Remote,
		"--force-with-lease="+e.Branch+":"+tip,
		commit+":"+e.Branch)
	if err != nil {
		outcome := classifyPushFailure(out)
		return outcome, err
	}
	return CASLanded, nil
}

// TrailerPresent walks the canonical history from the given tip for
// the opid's provenance trailer — the postcondition's TOTALITY leg:
// even when the touched file later moved or died, the commit trailer
// resolves the predicate.
func TrailerPresent(e Endpoint, tip, opid string) (bool, error) {
	out, err := goalGit(e.Root, nil, "log", "--format=%(trailers:key=Goal-Transaction,valueonly)", tip)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == opid {
			return true, nil
		}
	}
	return false, nil
}

// AdvanceAccepted CAS-advances the accepted ref onto a tip this
// process just validated and confirmed. An absent ref bootstraps.
func AdvanceAccepted(root, newTip string) error {
	return advanceAcceptedForward(root, newTip)
}

// advanceAcceptedForward moves the accepted ref FORWARD only:
// the CAS asserts the value read here, and a lost CAS re-reads —
// if the ref already descends to (or past) the new tip, someone
// else carried it and this pass is done; a ref that does NOT
// descend retries the CAS. The ref can never move backward through
// this path.
func advanceAcceptedForward(root, newTip string) error {
	for attempt := 0; attempt < 5; attempt++ {
		old, err := goalGit(root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
		if err != nil {
			// Creation IS the compare: the empty old-value
			// refuses a concurrent creator, so a later bootstrap can
			// never replace a descendant with its ancestor.
			if _, createErr := goalGit(root, nil, "update-ref", AcceptedRef, newTip, ""); createErr == nil {
				return nil
			}
			continue
		}
		oldTip := strings.TrimSpace(old)
		// Already at or PAST the new tip: forward means done.
		if _, ancErr := goalGit(root, nil, "merge-base", "--is-ancestor", newTip, oldTip); ancErr == nil {
			return nil
		}
		// The new tip must descend from the current value, or this
		// pass has a stale world and someone else owns the move.
		if _, ancErr := goalGit(root, nil, "merge-base", "--is-ancestor", oldTip, newTip); ancErr != nil {
			return fmt.Errorf("the accepted ref at %s and the tip %s have diverged; the read-side validator owns this move", short(oldTip), short(newTip))
		}
		if _, err := goalGit(root, nil, "update-ref", AcceptedRef, newTip, oldTip); err == nil {
			return nil
		}
	}
	return fmt.Errorf("the accepted ref CAS lost five times; a later pass advances it")
}

// CleanupRefs deletes the operation's temporary refs at its
// terminal phase.
func CleanupRefs(e Endpoint, opid string) {
	_, _ = goalGit(e.Root, nil, "update-ref", "-d", fetchRefFor(opid))
	_, _ = goalGit(e.Root, nil, "update-ref", "-d", txnRefFor(opid))
}

// LostToCompetitor is the mutation callback's way of classifying a
// rebuilt tip: the same-target operation already landed under
// someone else's opid — a loss, not a rejection.
type LostToCompetitor struct{ Winner string }

func (l LostToCompetitor) Error() string {
	return "lost to competitor " + l.Winner
}

// AlreadyApplied is the mutation callback's idempotent-success
// classification, lawful ONLY when the callback FOUND THIS
// OPERATION'S OWN OPID on the rebuilt tip (a resumed recovery, a
// delayed push). A semantic no-op without that proof is
// NothingToDo — the journal's truth predicate is the opid, and a
// confirmed entry whose opid is nowhere would be a lie.
type AlreadyApplied struct{}

func (AlreadyApplied) Error() string { return "already applied" }

// NothingToDo classifies a fresh operation whose desired state
// already holds without this opid's involvement: abandoned by its
// own reading of the world, never confirmed.
type NothingToDo struct{ Reason string }

func (n NothingToDo) Error() string { return "nothing to do: " + n.Reason }

// PublishRequest drives one full transaction.
type PublishRequest struct {
	Opid    string
	Machine string
	Lineage string
	Intent  Intent
	Message string
	// Mutate produces the changes for exactly the given tip — called
	// again on every rebuild, so the verb re-reads and re-decides on
	// the current world. Returning LostToCompetitor or
	// AlreadyApplied classifies the rebuilt tip; any other error is
	// a definite rejection by name.
	Mutate func(tip string) ([]Change, error)
	// Validate runs the full read-set revalidation on the built
	// commit's tree. Nil skips — the layers land in order.
	Validate func(commit string) error
	// Deadline bounds the retry loop; zero takes the default.
	Deadline time.Duration
	// BeforePush is the fixture seam: tests inject a competitor
	// between capture and push to force the CAS legs. Nil in
	// production.
	BeforePush func(attempt int) error
	// AfterConfirmed runs after the operation's trailer is visible at tip but
	// before the journal may terminalize confirmed. It is reserved for an
	// idempotent local effect whose omission would make confirmation a lie.
	AfterConfirmed func(tip string) error
}

// PublishResult is the transaction's terminal classification.
type PublishResult struct {
	Outcome    Outcome
	Tip        string // the canonical tip this outcome was decided on
	Commit     string // our transaction commit, when one was built
	Detail     string
	RiskRaised bool // the edit transaction raised the approved goal's risk derivation
}

// Publish runs the whole transaction under the journal's rules:
// entry created before anything, pushed durably before the push
// leaves, terminal with the evidence. A pushed entry already
// blocking this clone refuses up front (process-independent
// exclusion).
func Publish(e Endpoint, req PublishRequest) (PublishResult, error) {
	if req.Opid == "" || req.Mutate == nil {
		return PublishResult{}, fmt.Errorf("a publish needs an opid and a mutation")
	}
	if blocking, isBlocked, err := PushedBlocking(e.Root); err != nil {
		return PublishResult{}, err
	} else if isBlocked && blocking.Opid != req.Opid {
		return PublishResult{}, fmt.Errorf("journal entry %s is pushed with its outcome unknown; this clone mutates nothing until it is classified", blocking.Opid)
	}
	// A REPLAY of an opid this clone already journaled: a terminal
	// confirmed entry re-verifies its postcondition on a fresh
	// capture — the opid is the truth, not the belief — and returns
	// idempotent success; a non-terminal entry belongs to the
	// recovery rule, never to a second publish.
	if existing, readErr := ReadEntry(e.Root, req.Opid); readErr == nil {
		if existing.Phase == PhaseTerminal &&
			(existing.Outcome == OutcomeConfirmed || existing.Outcome == OutcomeConfirmedLate) {
			nonce, nErr := readNonce()
			if nErr != nil {
				return PublishResult{}, nErr
			}
			tip, capErr := CaptureTip(e, nonce)
			CleanupRefs(e, nonce)
			if capErr != nil {
				return PublishResult{}, capErr
			}
			present, trErr := TrailerPresent(e, tip, req.Opid)
			if trErr != nil {
				return PublishResult{}, trErr
			}
			if present {
				return PublishResult{Outcome: OutcomeConfirmed, Tip: tip, Detail: "idempotent"}, nil
			}
			return PublishResult{}, fmt.Errorf("journal entry %s says confirmed but its opid is not in canonical history; branch surgery needs the repair path", req.Opid)
		}
		return PublishResult{}, fmt.Errorf("journal entry %s exists and is %s; the recovery rule owns it, not a second publish", req.Opid, existing.Phase)
	}
	if _, err := CreateEntry(e.Root, req.Opid, req.Machine, req.Lineage, req.Intent); err != nil {
		return PublishResult{}, err
	}
	return runTransaction(e, req)
}

// runTransaction drives one journaled transaction whose entry
// ALREADY exists (created or pushed): Publish creates then runs;
// recovery takes over a dead owner's entry then runs the same loop
// from the stored intent (recovery completes, it does not
// kill).
func runTransaction(e Endpoint, req PublishRequest) (PublishResult, error) {
	deadline := req.Deadline
	if deadline <= 0 {
		deadline = DefaultPublishDeadline
	}
	stopAt := time.Now().Add(deadline)

	attempt := 0
	for {
		attempt++
		tip, err := CaptureTip(e, req.Opid)
		if err != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "capture failed: "+err.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{}, err
		}
		// The acceptance gates stand between EVERY mutation and the
		// fetched tip: a foreign or rewound canonical branch is
		// refused here exactly as the read side refuses it — a
		// mutation must never build on a world this clone would not
		// accept.
		acceptedTip, hasAccepted, accErr := acceptedTipForGates(e.Root)
		if accErr != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "accepted ref unreadable: "+accErr.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{}, accErr
		}
		if hasAccepted {
			if gateErr := AcceptanceGates(e.Root, acceptedTip, tip); gateErr != nil {
				_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "acceptance gates refused: "+gateErr.Error())
				CleanupRefs(e, req.Opid)
				return PublishResult{}, gateErr
			}
		}
		if gateErr := SyncModeGate(e, tip); gateErr != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "sync-mode gate refused: "+gateErr.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{}, gateErr
		}
		// The captured tip must pass the SAME whole-tree validation
		// the read side runs: identity and descent alone let a
		// malformed descendant be "healed" incidentally by whatever
		// verb ran next, publishing a repair nobody reviewed. A tree
		// that carries no ledger yet (pre-migration) validates
		// nothing — migration owns that world.
		hasLedger, ledgerErr := tipCarriesLedger(e, tip)
		if ledgerErr != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "ledger probe failed: "+ledgerErr.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{}, ledgerErr
		}
		if hasLedger {
			if valErr := ValidateCommit(e.Root, tip); valErr != nil {
				_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "captured tip refused: "+valErr.Error())
				CleanupRefs(e, req.Opid)
				return PublishResult{}, fmt.Errorf("the captured tip does not validate; repair the canonical branch deliberately: %w", valErr)
			}
		}
		if err := RecordSteps(e.Root, req.Opid, tip, ""); err != nil {
			return PublishResult{}, err
		}

		changes, err := req.Mutate(tip)
		if err != nil {
			return terminalFromMutate(e, req, tip, err)
		}
		commit, err := BuildCommit(e, req.Opid, tip, changes, req.Message)
		if err != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeAbandoned, "build failed: "+err.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{}, err
		}
		if err := RecordSteps(e.Root, req.Opid, "", commit); err != nil {
			return PublishResult{}, err
		}
		if err := validateLegacyArchiveReadOnly(e.Root, tip, commit); err != nil {
			_ = MarkTerminal(e.Root, req.Opid, OutcomeRejected, "legacy archive write refused: "+err.Error())
			CleanupRefs(e, req.Opid)
			return PublishResult{Outcome: OutcomeRejected, Tip: tip, Commit: commit, Detail: err.Error()}, nil
		}
		if req.Validate != nil {
			if err := req.Validate(commit); err != nil {
				_ = MarkTerminal(e.Root, req.Opid, OutcomeRejected, "validation refused: "+err.Error())
				CleanupRefs(e, req.Opid)
				return PublishResult{Outcome: OutcomeRejected, Tip: tip, Commit: commit, Detail: err.Error()}, nil
			}
		}

		if req.BeforePush != nil {
			if err := req.BeforePush(attempt); err != nil {
				return PublishResult{}, err
			}
		}

		// Durable BEFORE the push leaves this process.
		if err := MarkPushed(e.Root, req.Opid, tip, attempt, stopAt); err != nil {
			return PublishResult{}, err
		}
		outcome, pushErr := PublishCAS(e, tip, commit)
		switch outcome {
		case CASLanded:
			// The postcondition, not the push's exit code, is the
			// truth: refetch and find OUR trailer before
			// anything terminalizes.
			verifyNonce, nErr := readNonce()
			if nErr != nil {
				return PublishResult{}, nErr
			}
			newTip, capErr := CaptureTip(e, verifyNonce)
			CleanupRefs(e, verifyNonce)
			if capErr != nil {
				return PublishResult{Outcome: "", Tip: tip, Commit: commit,
						Detail: "pushed; the confirming refetch failed and the journal entry stays pushed"},
					fmt.Errorf("confirming refetch failed: %v", capErr)
			}
			present, trErr := TrailerPresent(e, newTip, req.Opid)
			if trErr != nil || !present {
				return PublishResult{Outcome: "", Tip: newTip, Commit: commit,
						Detail: "pushed; the opid is not visible on the refetched tip and the journal entry stays pushed"},
					fmt.Errorf("postcondition unresolved after a landed push (present=%v err=%v)", present, trErr)
			}
			if req.AfterConfirmed != nil {
				if hookErr := req.AfterConfirmed(newTip); hookErr != nil {
					return PublishResult{Outcome: "", Tip: newTip, Commit: commit,
						Detail: "pushed; the confirmed follow-on effect failed and the journal entry stays pushed"}, hookErr
				}
			}
			if err := MarkTerminal(e.Root, req.Opid, OutcomeConfirmed, "opid verified on "+short(newTip)); err != nil {
				return PublishResult{}, err
			}
			// The refetched tip may already be a DESCENDANT someone
			// else pushed after ours: accepted advances only onto a
			// tree this clone validated — never incidentally
			// onto a stranger's unvalidated descendant.
			advanceTarget := newTip
			if newTip != commit {
				if valErr := ValidateCommit(e.Root, newTip); valErr != nil {
					advanceTarget = commit
				}
			}
			if err := advanceAcceptedForward(e.Root, advanceTarget); err != nil {
				return PublishResult{Outcome: OutcomeConfirmed, Tip: newTip, Commit: commit,
					Detail: "confirmed; accepted ref did not advance: " + err.Error()}, nil
			}
			CleanupRefs(e, req.Opid)
			return PublishResult{Outcome: OutcomeConfirmed, Tip: newTip, Commit: commit}, nil

		case CASRefused:
			// Someone advanced the branch. The rebuilt world decides:
			// the loop continues inside the deadline; Mutate on the
			// new tip classifies loss and idempotent success.
			if time.Now().After(stopAt) {
				if err := MarkTerminal(e.Root, req.Opid, OutcomeExpired,
					fmt.Sprintf("deadline after %d attempts; last refusal: %v", attempt, pushErr)); err != nil {
					return PublishResult{}, err
				}
				CleanupRefs(e, req.Opid)
				return PublishResult{Outcome: OutcomeExpired, Tip: tip, Commit: commit,
					Detail: fmt.Sprintf("%d attempts", attempt)}, nil
			}
			continue

		default: // CASUnknown
			// The entry STAYS pushed; a later process classifies it
			// by the one recovery rule.
			return PublishResult{Outcome: "", Tip: tip, Commit: commit,
					Detail: "transport unknown; the journal entry stays pushed"},
				fmt.Errorf("push outcome unknown: %v", pushErr)
		}
	}
}

// terminalFromMutate maps the mutation callback's classification of
// a (re)built tip onto the journal.
func terminalFromMutate(e Endpoint, req PublishRequest, tip string, err error) (PublishResult, error) {
	opid := req.Opid
	switch v := err.(type) {
	case AlreadyApplied:
		if req.AfterConfirmed != nil {
			if hookErr := req.AfterConfirmed(tip); hookErr != nil {
				return PublishResult{Outcome: "", Tip: tip, Detail: "already applied; the confirmed follow-on effect failed and the journal entry stays open"}, hookErr
			}
		}
		if mErr := MarkTerminal(e.Root, opid, OutcomeConfirmed, "already applied at "+tip); mErr != nil {
			return PublishResult{}, mErr
		}
		CleanupRefs(e, opid)
		return PublishResult{Outcome: OutcomeConfirmed, Tip: tip, Detail: "idempotent"}, nil
	case NothingToDo:
		if mErr := MarkTerminal(e.Root, opid, OutcomeAbandoned, v.Reason); mErr != nil {
			return PublishResult{}, mErr
		}
		CleanupRefs(e, opid)
		return PublishResult{Outcome: OutcomeAbandoned, Tip: tip, Detail: v.Reason}, nil
	case LostToCompetitor:
		if mErr := MarkTerminal(e.Root, opid, OutcomeLost, "winner: "+v.Winner); mErr != nil {
			return PublishResult{}, mErr
		}
		CleanupRefs(e, opid)
		return PublishResult{Outcome: OutcomeLost, Tip: tip, Detail: "winner: " + v.Winner}, nil
	default:
		if mErr := MarkTerminal(e.Root, opid, OutcomeRejected, err.Error()); mErr != nil {
			return PublishResult{}, mErr
		}
		CleanupRefs(e, opid)
		return PublishResult{Outcome: OutcomeRejected, Tip: tip, Detail: err.Error()}, nil
	}
}
