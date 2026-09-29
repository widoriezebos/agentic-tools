package dispatch

// A delegate chain's workspace as a registered store (engine-owns-disk-
// lifetimes Part B, 3.1's delegate row, 3.2 "Delegate", 3.7, U5f): the
// linked worktree artifacts/agents/worktrees/<chain> on agent/<chain> with
// its quarantine object store. It is released only when the chain is
// closed, CloseCheck passes, every round's custody is proven dead (in the
// observation form, which expires no marker), and the workspace rules as
// landed hold (a status that prints nothing, no submodules, no
// skip-worktree or assume-unchanged entry, readable reflogs, every tip
// archived and read back, a clear use census); inside the store's critical
// section its quarantine is absorbed into the common store and verified by
// content before its alternates line goes, and only then are the worktree
// and the branch removed. A person's discard is this invocation's alone.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// DelegateWorkspaceProof is the delegate owner kind's proof.
type DelegateWorkspaceProof struct {
	// Repo is the control root the chain's job records live under; GitRoot
	// the checkout's repository.
	Repo, GitRoot string
	Git           diskstore.WorkspaceGit
	// Custody are the kernel observations of the custody proof; the proof
	// runs in its observation form whatever ExpireMarker says.
	Custody CustodyDeathDependencies
	// CustodyDead and Closed replace ProveCustodyDeath and CloseCheck
	// (fixtures); nil is production.
	CustodyDead func(record map[string]any) CustodyDeathResult
	Closed      func(repo, chain string) error
	Now         time.Time
	// Stage names the absorb's partial files.
	Stage string
}

func (DelegateWorkspaceProof) Kind() diskstore.OwnerKind { return diskstore.OwnerDelegate }

// Observe reads only: the chain's records, its close check, its custody and
// the workspace's content.
func (p DelegateWorkspaceProof) Observe(ctx context.Context, record diskstore.Record) diskstore.Verdict {
	return p.observe(ctx, record, false)
}

func (p DelegateWorkspaceProof) observe(ctx context.Context, record diskstore.Record, discard bool) diskstore.Verdict {
	chain := record.Owner.Ref
	pending := func(reason, command string) diskstore.Verdict {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: reason, Command: command}
	}
	if record.Class != diskstore.DelegateClass {
		return pending("a "+record.Class+" store of chain "+chain+" is released by its own owner, not this proof", "metasystem disk show")
	}
	// chainMembers skips a record it cannot read, and that record may name
	// a round of this chain: any unreadable job record holds the whole
	// delegate class this pass (Round D2 F-3).
	if unreadable := unreadableJobRecord(filepath.Join(p.Repo, "artifacts", "agents", "jobs")); unreadable != "" {
		return pending("a job record cannot be read ("+unreadable+"); no delegate workspace is released this pass", "metasystem system check")
	}
	members, err := chainMembers(filepath.Join(p.Repo, "artifacts", "agents", "jobs"), chain)
	switch {
	case err != nil:
		return pending("the chain's job records cannot be read ("+err.Error()+"); it is kept", "metasystem system check")
	case len(members) == 0:
		return pending("the chain has no job records, so nothing proves its work captured; a person decides", "metasystem disk show")
	}
	var root map[string]any
	for _, member := range members {
		if asString(member.record["jobId"]) == chain {
			root = member.record
		}
	}
	if root == nil {
		return pending("the chain's root record cannot be read; it is kept", "metasystem system check")
	}
	if closed, _ := root["chainClosed"].(bool); !closed {
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "the chain is open; its workspace is its working state",
			Command: "metasystem work review j2:" + chain + ", then metasystem work finish j2:" + chain}
	}
	closed := p.Closed
	if closed == nil {
		closed = CloseCheck
	}
	if err := closed(p.Repo, chain); err != nil {
		return pending("the chain's evidence is not durable ("+err.Error()+")", "metasystem work finish j2:"+chain)
	}
	dead := p.CustodyDead
	if dead == nil {
		custody := p.Custody
		var wouldExpire []string
		custody.ExpireMarker = ObserveMarkerExpiry(&wouldExpire)
		dead = func(record map[string]any) CustodyDeathResult { return ProveCustodyDeath(p.Repo, record, custody) }
	}
	for _, member := range members {
		result := dead(member.record)
		switch result.Outcome {
		case CustodyDeathProven:
		case CustodyDeathAlive:
			return diskstore.Verdict{Decision: diskstore.Keep, Reason: "a round of the chain still runs (" + result.Reason + ")", Command: "metasystem work stop j2:" + chain}
		default:
			return pending("a round's custody is not proven dead ("+result.Reason+")", "metasystem disk clean")
		}
	}
	if verdict := diskstore.JudgeLinkedContent(ctx, p.Git, record, discard); verdict.Decision != diskstore.Release {
		return verdict
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "chain " + chain + " is closed, captured and custody-dead"}
}

// Apply is never reached: RegisteredStores hands a delegate workspace to
// Release. It refuses, so nothing is removed by a caller that skipped it.
func (DelegateWorkspaceProof) Apply(context.Context, *diskstore.Critical) error {
	return errors.New("a delegate workspace is released through its own release sequence")
}

// Release is the sweeper's release inside the critical section: the
// workspace release sequence with the quarantine absorbed before any
// removal.
func (p DelegateWorkspaceProof) Release(ctx context.Context, critical *diskstore.Critical, census *diskstore.UseCensus) diskstore.Verdict {
	return p.release(ctx, critical, census, nil, "sweeper")
}

func (p DelegateWorkspaceProof) release(ctx context.Context, critical *diskstore.Critical, census *diskstore.UseCensus, discard *diskstore.Discard, by string) diskstore.Verdict {
	outcome, err := diskstore.ReleaseInSection(ctx, critical, diskstore.WorkspaceReleaseRequest{GitRoot: p.GitRoot, Git: p.Git, Census: census,
		Discard: discard, By: by, Now: p.Now, BeforeRemoval: p.absorb})
	switch {
	case err != nil:
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk show"}
	case outcome.Done:
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "chain closed; its workspace released"}
	case outcome.Kept:
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: outcome.Reason, Command: outcome.Command}
	}
	return diskstore.Verdict{Decision: diskstore.Pending, Reason: outcome.Reason, Command: outcome.Command}
}

// absorb publishes the quarantine into the common store and drops its
// alternates line; it refuses while any other store still borrows it.
func (p DelegateWorkspaceProof) absorb(ctx context.Context, record diskstore.Record) error {
	gitdir := record.Identity.Gitdir
	if gitdir == "" {
		return fmt.Errorf("the workspace's git directory is not recorded")
	}
	common := filepath.Dir(filepath.Dir(gitdir))
	quarantine := filepath.Join(gitdir, diskstore.QuarantineName)
	if _, err := diskstore.AbsorbQuarantine(ctx, diskstore.AbsorbRequest{Git: p.Git, GitRoot: p.GitRoot, CommonObjects: filepath.Join(common, "objects"),
		Quarantine: quarantine, Stage: p.Stage}); err != nil {
		return fmt.Errorf("the quarantine could not be absorbed: %w", err)
	}
	borrowers, err := diskstore.QuarantineBorrowers(common, quarantine)
	if err != nil {
		return err
	}
	if len(borrowers) > 0 {
		return fmt.Errorf("the quarantine is still borrowed through %s", strings.Join(borrowers, ", "))
	}
	return nil
}

// ReleaseDiscarded is a person's discard of a chain's uncommitted work for
// this invocation alone (Round B3-3 rule 2): the delegate proof with the
// content keep waived (committed history and every tip are still archived;
// an unreadable reflog still keeps it), the use census the person's pass
// took, and the note kept as history in the record.
func (p DelegateWorkspaceProof) ReleaseDiscarded(ctx context.Context, registry diskstore.Registry, id string, census *diskstore.UseCensus, discard diskstore.Discard) (diskstore.Verdict, error) {
	critical, err := registry.TryCritical(id)
	var held *diskstore.HeldError
	switch {
	case errors.Is(err, diskstore.ErrNotFound):
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "no such workspace; nothing to release"}, nil
	case errors.As(err, &held):
		return diskstore.Verdict{Decision: diskstore.Keep, Reason: "an engine verb is inside the workspace (its record lock is held)",
			Command: "metasystem disk clean --discard j2:ID --reason TEXT once that verb has ended"}, nil
	case err != nil:
		return diskstore.Verdict{}, err
	}
	defer critical.Release()
	record := critical.Record()
	if record.State == diskstore.StateReleased {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "already released"}, nil
	}
	if verdict := p.observe(ctx, record, true); verdict.Decision != diskstore.Release {
		return verdict, nil
	}
	return p.release(ctx, critical, census, &discard, "person "+discard.By), nil
}

// unreadableJobRecord names the first job record under jobsDir that cannot
// be read as a JSON object, or the listing error; empty when all read.
func unreadableJobRecord(jobsDir string) string {
	paths, err := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	if err != nil {
		return err.Error()
	}
	if _, err := os.ReadDir(jobsDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return jobsDir + ": " + err.Error()
	}
	for _, path := range paths {
		if _, err := readObject(path); err != nil {
			return filepath.Base(path) + ": " + err.Error()
		}
	}
	return ""
}
