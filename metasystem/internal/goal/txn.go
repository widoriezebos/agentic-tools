package goal

// The ledger transaction engine (BGS-1/BGS-2): fetch the canonical
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
// are lawful work, not failure (R3-13).
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

// LocalMode reports the declared single-machine mode (R3-12).
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
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// CaptureTip fetches the canonical branch into the per-operation
// ref (R4-03: concurrent operations in one clone can never rewind a
// shared ref under each other) and returns the captured tip oid.
// Single-machine mode reads the dedicated local ledger branch.
func CaptureTip(e Endpoint, opid string) (string, error) {
	if e.LocalMode() {
		out, err := goalGit(e.Root, nil, "rev-parse", "--verify", LocalLedgerBranch)
		if err != nil {
			return "", fmt.Errorf("no local ledger branch %s; migration or adoption creates it: %v", LocalLedgerBranch, err)
		}
		return strings.TrimSpace(out), nil
	}
	if _, err := goalGit(e.Root, nil, "fetch", "--no-tags", e.Remote,
		"+"+e.Branch+":"+fetchRefFor(opid)); err != nil {
		return "", err
	}
	out, err := goalGit(e.Root, nil, "rev-parse", "--verify", fetchRefFor(opid))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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
// and a commit whose parent IS that tip (R3-02), carrying the
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
			"--cacheinfo", "100644,"+oid+","+c.Path); err != nil {
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
	cmd.Stdin = strings.NewReader(string(content))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hash-object: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
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
	for _, marker := range []string{"stale info", "[rejected]", "fetch first", "non-fast-forward"} {
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
	old, err := goalGit(root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		_, createErr := goalGit(root, nil, "update-ref", AcceptedRef, newTip)
		return createErr
	}
	_, err = goalGit(root, nil, "update-ref", AcceptedRef, newTip, strings.TrimSpace(old))
	return err
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
// classification: the rebuilt tip already carries this operation's
// effect (this clone's own opid landed — a resumed recovery, a
// delayed push).
type AlreadyApplied struct{}

func (AlreadyApplied) Error() string { return "already applied" }

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
	// commit's tree (BGS-3). Nil skips — the layers land in order.
	Validate func(commit string) error
	// Deadline bounds the retry loop; zero takes the default.
	Deadline time.Duration
	// BeforePush is the fixture seam: tests inject a competitor
	// between capture and push to force the CAS legs. Nil in
	// production.
	BeforePush func(attempt int) error
}

// PublishResult is the transaction's terminal classification.
type PublishResult struct {
	Outcome Outcome
	Tip     string // the canonical tip this outcome was decided on
	Commit  string // our transaction commit, when one was built
	Detail  string
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
		if err := RecordSteps(e.Root, req.Opid, tip, ""); err != nil {
			return PublishResult{}, err
		}

		changes, err := req.Mutate(tip)
		if err != nil {
			return terminalFromMutate(e, req.Opid, tip, err)
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
			if err := MarkTerminal(e.Root, req.Opid, OutcomeConfirmed, "opid landed on "+commit); err != nil {
				return PublishResult{}, err
			}
			if err := AdvanceAccepted(e.Root, commit); err != nil {
				return PublishResult{Outcome: OutcomeConfirmed, Tip: commit, Commit: commit,
					Detail: "confirmed; accepted ref did not advance: " + err.Error()}, nil
			}
			CleanupRefs(e, req.Opid)
			return PublishResult{Outcome: OutcomeConfirmed, Tip: commit, Commit: commit}, nil

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
func terminalFromMutate(e Endpoint, opid, tip string, err error) (PublishResult, error) {
	switch v := err.(type) {
	case AlreadyApplied:
		if mErr := MarkTerminal(e.Root, opid, OutcomeConfirmed, "already applied at "+tip); mErr != nil {
			return PublishResult{}, mErr
		}
		CleanupRefs(e, opid)
		return PublishResult{Outcome: OutcomeConfirmed, Tip: tip, Detail: "idempotent"}, nil
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
