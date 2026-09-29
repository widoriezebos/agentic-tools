package steward

// The sweeper's retry of landing release sets (design engine-owns-disk-
// lifetimes Part B, 3.6): a hand landing records the goal's workspaces its
// selection contains in its own landed.json and releases them once the
// merged branch is swept; a set left unfinished (a held store, a missing
// census, a crash after the push) is finished by the next work land of the
// goal and by this class in every checkout pass, independently of the
// goal's conclusion. It finishes only the recorded ids.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// ExecWorkspaceGit runs git in dir under ctx with the repository-steering
// environment scrubbed, so the repository is always dir's own.
func ExecWorkspaceGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// LandingReleaseSets is the class of landing records with an unfinished
// release set in one checkout.
type LandingReleaseSets struct {
	// Installation holds artifacts/agents/landing-intent; StateRoot the
	// store registry; GitRoot the repository.
	Installation, StateRoot, GitRoot string
	Git                              diskstore.WorkspaceGit
}

func (LandingReleaseSets) Name() string { return "landing release sets" }

func (c LandingReleaseSets) records() []string {
	paths, _ := filepath.Glob(filepath.Join(c.Installation, "artifacts", "agents", "landing-intent", "*", "*", "landed.json"))
	sort.Strings(paths)
	return paths
}

// landingRecord is the part of a hand landing's landed.json this class
// reads; the rest of the record is carried through unchanged.
type landingRecord struct {
	Landing    string
	Swept      bool
	ReleaseSet *diskstore.ReleaseSet
}

func readLandingRecord(path string) (landingRecord, map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return landingRecord{}, nil, err
	}
	var record landingRecord
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(data, &record); err != nil {
		return landingRecord{}, nil, err
	}
	if err := json.Unmarshal(data, &whole); err != nil {
		return landingRecord{}, nil, err
	}
	return record, whole, nil
}

func unfinished(record landingRecord) bool {
	return record.Landing != "" && record.Swept && record.ReleaseSet != nil && !record.ReleaseSet.Finished()
}

// Plan lists every swept landing whose set is unfinished; it writes nothing.
func (c LandingReleaseSets) Plan(_ context.Context, _ *diskstore.Pass) ([]diskstore.Item, error) {
	var items []diskstore.Item
	for _, path := range c.records() {
		record, _, err := readLandingRecord(path)
		if err != nil || !unfinished(record) {
			continue
		}
		pending := 0
		for _, entry := range record.ReleaseSet.Stores {
			if entry.State == diskstore.ReleasePending {
				pending++
			}
		}
		items = append(items, diskstore.Item{Class: c.Name(), Key: path, Path: path,
			Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: fmt.Sprintf("landing %s has %d workspace(s) left to release", record.Landing, pending)}})
	}
	return items, nil
}

// Apply runs the pending entries of one landing's set and rewrites its
// record when an entry changed.
func (c LandingReleaseSets) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	release, err := diskstore.LockLandingRecord(item.Path, false)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: err.Error(), Command: "metasystem disk clean"}
	}
	defer release()
	record, whole, err := readLandingRecord(item.Path)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the landing record is unreadable: " + err.Error(), Command: "metasystem disk show"}
	}
	if !unfinished(record) {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "the release set is finished"}
	}
	registry := diskstore.CheckoutRegistry(c.StateRoot)
	request := diskstore.WorkspaceReleaseRequest{Registry: registry, GitRoot: c.GitRoot, Git: c.Git,
		By: "the sweeper, for the landing of " + record.Landing, Now: pass.Now,
		TakeCensus: func() *diskstore.UseCensus {
			census := pass.Census(ctx)
			if census != nil && census.Taken && pass.CensusReader() != nil {
				if err := census.ReadNew(ctx, *pass.CensusReader()); err != nil {
					return &diskstore.UseCensus{NotTaken: "processes started since the census could not be read: " + err.Error()}
				}
			}
			return census
		}}
	if diskstore.RunReleaseSet(ctx, request, record.ReleaseSet) {
		encoded, err := json.Marshal(record.ReleaseSet)
		if err == nil {
			whole["ReleaseSet"] = encoded
			var data []byte
			if data, err = json.Marshal(whole); err == nil {
				_, err = atomicfile.WriteFile(item.Path, data, 0o644, filepath.Dir(item.Path))
			}
		}
		if err != nil {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the landing record could not be rewritten: " + err.Error(), Command: "metasystem disk clean"}
		}
	}
	if record.ReleaseSet.Finished() {
		return diskstore.Verdict{Decision: diskstore.Release, Reason: "the landing's release set is finished"}
	}
	for _, entry := range record.ReleaseSet.Stores {
		if entry.State == diskstore.ReleasePending {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "workspace " + entry.Path + ": " + entry.Reason, Command: entry.Command}
		}
	}
	return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the release set is unfinished", Command: "metasystem disk clean"}
}

// checkoutProofs are the owner-kind proofs of one checkout's pass: the
// engine's, plus the workspace proof over this checkout's repository and
// goal ledger. Fixtures that name their own proofs get exactly those.
func checkoutProofs(top string, pass DiskPass) map[diskstore.OwnerKind]diskstore.OwnerProof {
	if pass.Proofs != nil {
		return pass.Proofs
	}
	proofs := diskOwnerProofs()
	layout, err := stateroot.ResolveLayout(top)
	if err != nil {
		return proofs
	}
	proofs[diskstore.OwnerGoal] = diskstore.WorkspaceProof{GitRoot: layout.GitRoot, Git: ExecWorkspaceGit, Ended: goalEnded(checkoutLedger(top, pass.Now)),
		Now: pass.Now}
	return proofs
}

// goalEnded answers from the checkout's accepted goal ledger, read once
// without a fetch: a goal is ended when it is done or abandoned; an
// unreadable ledger is unknown for every goal. The basis names the ledger
// tip the answer was read at, for the release record.
func goalEnded(ledger *ledgerView) func(diskstore.Owner) (bool, bool, string) {
	return func(owner diskstore.Owner) (bool, bool, string) {
		if owner.Kind != diskstore.OwnerGoal {
			return false, false, ""
		}
		projection, err := ledger.get()
		if err != nil {
			return false, false, ""
		}
		_, done := projection.Tree.Done[owner.Ref]
		_, abandoned := projection.Tree.Abandoned[owner.Ref]
		return done || abandoned, true, "the accepted goal ledger at " + projection.Tip
	}
}
