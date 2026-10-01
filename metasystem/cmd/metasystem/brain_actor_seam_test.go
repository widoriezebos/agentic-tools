package main

// Ported from scripts/agents/brain-fixtures.sh scenario
// brain-actor-seam-coverage (verb redesign U7b part 3). The subject is the Go
// source text itself: every non-test site in cmd/metasystem and internal that
// builds a goal actor, sets a human actor, or classifies a verb caller is on
// an explicit allow-list, so a new one is inspected before it lands. An
// installed dependency tree (node_modules, at any depth) is never a brain
// actor seam and is pruned (g1-s8 revision 5, the exclusion slice).

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var brainActorSitePattern = regexp.MustCompile(`goal\.Actor\{|Actor\.Human[[:space:]]*=[^=]|classifyVerbCaller(With)?\(|lease\.ClassifyVerbAt\(e\.Root`)

var brainActorSiteExempt = regexp.MustCompile(`^cmd/metasystem/goalsync_mutations.go:[[:space:]]*Endpoint: e, Actor:`)

// scanBrainActorSites returns "path:line" for every matching line under
// cmd/metasystem and internal of root, byte-sorted.
func scanBrainActorSites(t *testing.T, root string) []string {
	t.Helper()
	var sites []string
	for _, top := range []string{"cmd/metasystem", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(top)), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
				if !brainActorSitePattern.MatchString(line) {
					continue
				}
				site := filepath.ToSlash(relative) + ":" + line
				if brainActorSiteExempt.MatchString(site) {
					continue
				}
				sites = append(sites, site)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(sites)
	return sites
}

func TestBrainBedActorSeamScanPrunesDependencyTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, relative := range []string{"cmd/metasystem/seen.go", "internal/x/node_modules/p/hidden.go"} {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("classification, err := classifyVerbCaller(root, 0)\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sites := scanBrainActorSites(t, root)
	if len(sites) != 1 || !strings.HasPrefix(sites[0], "cmd/metasystem/seen.go:") {
		t.Fatalf("brain actor seam scan no longer prunes an installed dependency tree: %q", sites)
	}
}

func TestBrainBedActorSeamCoverage(t *testing.T) {
	t.Parallel()
	got := strings.Join(scanBrainActorSites(t, filepath.Join("..", "..")), "\n")
	if got != strings.TrimSuffix(expectedBrainActorSites, "\n") {
		t.Fatalf("brain actor seam allow-list changed; inspect every added Actor or caller-classification site:\n%s", got)
	}
}

const expectedBrainActorSites = `cmd/metasystem/goal.go:			return goal.Actor{}, 0, errors.New("the session holding this checkout hasn't announced itself; start it with metasystem session start")
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the Stop main %q does not match the announced checkout holder %q", mainID, holder.MainId)
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the announced checkout holder could not be resolved: %w", err)
cmd/metasystem/goal.go:			return goal.Actor{}, 0, fmt.Errorf("the seat machine could not be resolved: %w", err)
cmd/metasystem/goal.go:		return goal.Actor{Machine: machine, Lineage: holder.OwnerLineage}, holder.ClaimEpoch, nil
cmd/metasystem/goal.go:	view, err := classifyVerbCallerWith(root, callerPid, repositoryTop)
cmd/metasystem/goalsync_mutations.go:		classification, classErr := classifyVerbCaller(f.root, int64(os.Getppid()))
cmd/metasystem/goalsync_mutations.go:		classification, classifyErr = classifyVerbCallerWith(root, callerPid, facts.repositoryTop)
cmd/metasystem/goalsync_mutations.go:		req.Actor.Human = f.by
cmd/metasystem/goalsync_verbs.go:		return goal.Actor{}, err
cmd/metasystem/goalsync_verbs.go:	return goal.Actor{Machine: machine, Lineage: lineage, Human: human}, nil
cmd/metasystem/intent_delivery.go:	caller, err := classifyVerbCaller(root, int64(os.Getpid()))
cmd/metasystem/process_verbs.go:	return classifyVerbCallerWith(root, callerPid, stateroot.RepositoryTop)
cmd/metasystem/process_verbs.go:func classifyVerbCaller(root string, callerPid int64) (lease.ClassifyResult, error) {
cmd/metasystem/process_verbs.go:func classifyVerbCallerWith(root string, callerPid int64, repositoryTop func(string) (string, error)) (lease.ClassifyResult, error) {
cmd/metasystem/proof_run.go:				Actor: goal.Actor{Machine: binding.Machine, Lineage: binding.Lineage}, Ulid: ulid, Now: now,
cmd/metasystem/run.go:		view, err := classifyVerbCaller(root, int64(os.Getpid()))
cmd/metasystem/wait_register.go:	view, err := classifyVerbCaller(root, callerPID)
internal/channel/poll.go:				approved, approveErr := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: a.UserID}, Ulid: a.ApprovalULID, Now: a.At}, []string{q.Goal}, q.Budget, &proof)
internal/channel/poll.go:		published, err := goal.Answer(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage}, Ulid: a.ULID, Now: a.At}, q.Goal, q.ID, a.Text, wants, goal.AnswerProof{Provider: c.ProviderName, User: a.UserID, Ref: a.Ref.ThreadID + "/" + a.Ref.ID, Step: a.Step})
internal/channel/poll.go:		published, err := goal.Approve(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: c.Machine, Lineage: c.Lineage, Human: "wido"}, Ulid: ulid, Now: c.Now}, []string{status.GoalID}, nil, &proof)
internal/channel/question.go:		published, e := goal.Asked(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: r.Machine, Lineage: r.Lineage}, Ulid: ulid, Now: r.Now}, r.Goal, id, r.Kind, r.Facts[0])
internal/dispatch/claim.go:		Endpoint: endpoint, Actor: goal.Actor{Machine: binding.Machine, Lineage: binding.Lineage},
internal/dispatch/finding_register.go:	req := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: lineage}, Ulid: deterministicULID(rootJob), Now: time.Now().UTC(), ClaimEpoch: epoch}
internal/dispatch/stop.go:				Actor:    goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage, Human: human},
internal/dispatch/stop.go:	actor := goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage}
internal/goal/recover.go:		r.Actor.Human = by
internal/missionrunner/launch.go:	if view, err := lease.ClassifyVerbAt(e.Root, e.classifierInstallation(), int64(pid)); err == nil {
internal/steward/revive.go:		Actor: goal.Actor{
internal/steward/validation_window.go:	request := goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}, Ulid: ulid, Now: now}
internal/ui/act/act.go:		Actor:           goal.Actor{Machine: machine, Lineage: a.lineage, Human: a.human},
`
