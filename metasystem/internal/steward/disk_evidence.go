package steward

// The evidence path's production seams in the steward's passes (design
// engine-owns-disk-lifetimes 3.5, 3.12): the checkout pass distils and moves
// suite-failure bundles with the host's blob store, the checkout's
// repository as a de-duplication hint, and the checkout's four facts read
// under the pass's context.

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/evidence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// userHome is the user's home directory, where the host's blob store and
// the default evidence roots live; a fixture passes its own.
func (p DiskPass) userHome() (string, error) {
	if p.UserHome != "" {
		return p.UserHome, nil
	}
	return os.UserHomeDir()
}

// suiteFailureClass is the checkout pass's evidence ageing for top.
func suiteFailureClass(top string, settings diskstore.Settings, pass DiskPass) (diskstore.Class, error) {
	home, err := pass.userHome()
	if err != nil {
		return nil, err
	}
	gitRoot := gitRootAbove(top)
	segmentDir := ""
	if root := settings.EvidenceRoot.Path; root != "" {
		segmentDir = filepath.Join(root, "suite-failures", diskstore.Segment(gitRoot))
	}
	class := diskstore.SuiteFailures{Dir: filepath.Join(top, "artifacts", "agents", "suite-failures"), SegmentDir: segmentDir,
		Installation: top, GitRoot: gitRoot, Blobs: diskstore.BlobStore{Dir: diskstore.BlobStoreDir(home)},
		CompressAbove: settings.Bytes(config.DiskCompressAboveKey), DistillAfter: settings.Duration(config.DiskSuiteDistillKey),
		MoveAfter: settings.Duration(config.DiskSuiteMoveKey), Target: settings.Bytes(config.DiskSuiteTargetKey), Entropy: rand.Reader,
		Known: repositoryKnows(gitRoot), Facts: checkoutFactsReader(top, gitRoot),
		AttemptGoal: func(attempt string) (string, bool) {
			record, err := proofrun.ReadAttempt(top, attempt)
			if err != nil {
				return "", false
			}
			return record.AccountedGoal(), true
		}}
	if pass.SuiteFailureSeams != nil {
		pass.SuiteFailureSeams(&class)
	}
	return class, nil
}

// gitContext runs git in root under ctx with the scrubbed environment.
func gitContext(ctx context.Context, root, stdin string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	out, err := command.Output()
	return string(out), err
}

// repositoryKnows answers which files are byte-identical to an object of
// the repository at gitRoot: one hash-object over the paths, one
// batch-check over the ids. It is a de-duplication hint; the recipe never
// depends on the repository.
func repositoryKnows(gitRoot string) func(ctx context.Context, paths []string) (map[string]bool, error) {
	return func(ctx context.Context, paths []string) (map[string]bool, error) {
		sorted := append([]string(nil), paths...)
		sort.Strings(sorted)
		hashed, err := gitContext(ctx, gitRoot, strings.Join(sorted, "\n")+"\n", "hash-object", "--no-filters", "--stdin-paths")
		if err != nil {
			return nil, err
		}
		ids := strings.Fields(hashed)
		if len(ids) != len(sorted) {
			return nil, fmt.Errorf("hash-object answered %d ids for %d paths", len(ids), len(sorted))
		}
		checked, err := gitContext(ctx, gitRoot, strings.Join(ids, "\n")+"\n", "cat-file", "--batch-check")
		if err != nil {
			return nil, err
		}
		known := map[string]bool{}
		scanner := bufio.NewScanner(strings.NewReader(checked))
		for index := 0; scanner.Scan() && index < len(sorted); index++ {
			if !strings.HasSuffix(scanner.Text(), " missing") {
				known[sorted[index]] = true
			}
		}
		return known, scanner.Err()
	}
}

// checkoutFactsReader reads the checkout's root commit and ledger identity
// under the pass's context; the ledger read is abandoned, not waited for,
// when the context ends first.
func checkoutFactsReader(installation, gitRoot string) func(ctx context.Context) (diskstore.CheckoutFacts, error) {
	return func(ctx context.Context) (diskstore.CheckoutFacts, error) {
		facts := diskstore.CheckoutFacts{GitRoot: gitRoot, Installation: installation}
		roots, err := gitContext(ctx, gitRoot, "", "rev-list", "--max-parents=0", "HEAD")
		if err != nil {
			return facts, err
		}
		if fields := strings.Fields(roots); len(fields) > 0 {
			sort.Strings(fields)
			facts.RootCommit = fields[0]
		}
		identity := make(chan string, 1)
		go func() {
			endpoint, err := goal.ResolveEndpoint(installation)
			if err != nil {
				identity <- ""
				return
			}
			identity <- goal.ExistingLedgerIdentityAtEndpoint(endpoint)
		}()
		select {
		case facts.LedgerIdentity = <-identity:
		case <-ctx.Done():
			return facts, errors.Join(errors.New("the ledger identity was not read within the pass budget"), ctx.Err())
		}
		return facts, nil
	}
}

// gitRootAbove is the nearest directory from dir upward holding a .git
// entry (a directory, or a linked worktree's file), else dir: the git
// top-level the chain mirror hashes into its segment, found without running
// git.
func gitRootAbove(dir string) string {
	for current := dir; ; {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return dir
		}
		current = parent
	}
}

// installationOf is the installation of an armed checkout's state root:
// itself in the template layout, else the one its layout names.
func installationOf(checkout string) string {
	if regularFileExists(filepath.Join(checkout, "metasystem.conf")) {
		return checkout
	}
	if layout, err := stateroot.ResolveLayout(checkout); err == nil {
		return layout.InstallationRoot
	}
	return checkout
}

// evidenceBoundClass is the machine pass's evidence bound over every root
// of the host (design engine-owns-disk-lifetimes 3.12).
func evidenceBoundClass(ctx context.Context, home, top string, checkouts []string, participants []diskstore.Participant,
	host diskstore.HostSettings, pass DiskPass) (*evidence.BoundClass, error) {
	userHome, err := pass.userHome()
	if err != nil {
		return nil, err
	}
	readFacts := pass.Facts
	if readFacts == nil {
		readFacts = func(ctx context.Context, installation string) (diskstore.CheckoutFacts, error) {
			return checkoutFactsReader(installation, gitRootAbove(installation))(ctx)
		}
	}
	hostHasEvidence := anyEvidenceUnder(diskstore.EvidenceParent(userHome))
	for index := range checkouts {
		if index < len(participants) && participants[index].Err == nil && evidenceRootHoldsSegments(participants[index].Settings.EvidenceRoot.Path) {
			hostHasEvidence = true
		}
	}
	var hostCheckouts []evidence.HostCheckout
	var ageFloor time.Duration
	var extras = map[string]string{}
	for index, checkout := range checkouts {
		installation := installationOf(checkout)
		entry := evidence.HostCheckout{Installation: installation}
		if index < len(participants) {
			entry.Settings, entry.SettingsErr = participants[index].Settings, participants[index].Err
		}
		// Every armed checkout's facts are read once any root of the host
		// holds evidence: a local-mode peer's ledger identity is part of
		// every union (Round B2, F-10). With no evidence anywhere nothing
		// needs them and no git runs.
		if hostHasEvidence {
			entry.Facts, entry.FactsErr = readFacts(ctx, installation)
		}
		if entry.Facts.GitRoot == "" {
			entry.Facts.GitRoot = gitRootAbove(installation)
		}
		entry.Facts.Installation = installation
		if entry.SettingsErr == nil {
			if floor := entry.Settings.Duration(config.DiskEvidenceAgeFloorKey); floor > ageFloor {
				ageFloor = floor
			}
			extras[installation] = entry.Settings.Values[config.DiskEvidenceCitationKey]
		}
		hostCheckouts = append(hostCheckouts, entry)
	}
	class := &evidence.BoundClass{UserHome: userHome, HomeStateRoot: home, Checkouts: hostCheckouts,
		MachineCap: host.Bytes(config.DiskEvidenceMachineCapKey), BlobGrace: host.Duration(config.DiskEvidenceBlobGraceKey), AgeFloor: ageFloor,
		Observe: evidence.GoalLedgerObserver(pass.Clock), Tip: evidence.AcceptedTipReader(),
		Citations: &evidence.Citations{Dir: filepath.Join(home, "stores", "citations"), Now: pass.Now,
			Roots: func() ([]string, error) {
				var roots []string
				for installation, extra := range extras {
					found, err := evidence.CitationRoots(installation, extra)
					if err != nil {
						return nil, err
					}
					roots = append(roots, found...)
				}
				sort.Strings(roots)
				return roots, nil
			}},
		Bound: evidence.Bound{BoundLock: diskstore.BoundLockPath(home), Now: pass.Now, Entropy: rand.Reader, By: "steward " + top,
			Locks: evidence.OwnerLocks(int64(os.Getpid()), os.Args[0], pass.Clock, func(time.Duration) {}),
			Blobs: diskstore.BlobStore{Dir: diskstore.BlobStoreDir(userHome)}}}
	if pass.EvidenceSeams != nil {
		pass.EvidenceSeams(class)
	}
	return class, nil
}

// EvidenceEnv is what a person's evidence verb sees for the checkout top:
// the host's armed checkouts with their settings and facts, the goal
// ledger observer, the citation index, the lifecycle locks with the
// reaper's bound, and the host's blob store. pass carries the fixture
// seams (Home, UserHome, Checkouts, Facts); by names the actor.
func EvidenceEnv(ctx context.Context, top string, pass DiskPass, by string) (evidence.Env, error) {
	if pass.Clock == nil {
		pass.Clock = time.Now
	}
	if pass.Now.IsZero() {
		pass.Now = pass.Clock().UTC()
	}
	home := pass.Home
	if home == "" {
		var err error
		if home, err = HomeStateRoot(); err != nil {
			return evidence.Env{}, err
		}
	}
	userHome, err := pass.userHome()
	if err != nil {
		return evidence.Env{}, err
	}
	checkouts := pass.Checkouts
	if checkouts == nil {
		checkouts = armedCheckouts()
	}
	if !containsPath(checkouts, top) {
		checkouts = append(checkouts, top)
	}
	var participants []diskstore.Participant
	var live []string
	for _, checkout := range checkouts {
		if checkout != top && checkoutRemoved(checkout) {
			continue
		}
		settings, err := diskSettingsFor(checkout)
		participants = append(participants, diskstore.Participant{Checkout: checkout, Settings: settings, Err: err})
		live = append(live, checkout)
	}
	class, err := evidenceBoundClass(ctx, home, top, live, participants, diskstore.HostSettings{}, pass)
	if err != nil {
		return evidence.Env{}, err
	}
	env := evidence.Env{UserHome: userHome, HomeStateRoot: home, Checkouts: class.Checkouts, Now: pass.Now, Entropy: rand.Reader,
		Observe: class.Observe, Tip: class.Tip, Citations: class.Citations, Blobs: class.Bound.Blobs, By: by,
		Locks: evidence.OwnerLocks(int64(os.Getpid()), os.Args[0], pass.Clock, time.Sleep)}
	installation := installationOf(top)
	for _, checkout := range class.Checkouts {
		if checkout.Installation == installation {
			env.This = checkout
		}
	}
	return env, nil
}

// evidenceRootHoldsSegments reports an evidence root with any segment
// directory in it.
func evidenceRootHoldsSegments(root string) bool {
	if root == "" {
		return false
	}
	for _, name := range []string{"agents", "suite-failures", "events"} {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// anyEvidenceUnder reports a directory under the evidence parent that holds
// a segment directory.
func anyEvidenceUnder(parent string) bool {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != diskstore.BlobStoreName && evidenceRootHoldsSegments(filepath.Join(parent, entry.Name())) {
			return true
		}
	}
	return false
}
