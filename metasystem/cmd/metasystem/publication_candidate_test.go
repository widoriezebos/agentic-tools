package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

func TestGoalOpenCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"unrelated", "missing"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			f.Candidate("missing")
			e, err := defaultSyncRequestDependencies().endpoint(f.Root)
			if err != nil {
				t.Fatal(err)
			}
			pushes := 0
			e = f.CountPublications(e, &pushes)
			r := f.Request(e)
			result, err := goal.Open(r, id, "Complete the goal", goal.OriginMain, "Build the change")
			if err != nil {
				t.Fatal(err)
			}
			if id == "missing" {
				if result.Outcome != goal.OutcomeConfirmed || pushes != 1 {
					t.Fatalf("newly referenced goal: %+v pushes=%d", result, pushes)
				}
			} else {
				f.AssertRefusal(errors.New(result.Detail))
				if result.Outcome != goal.OutcomeRejected || pushes != 0 {
					t.Fatalf("invalid candidate: %+v pushes=%d", result, pushes)
				}
			}
		})
	}
}

func TestIncidentRecordCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	candidateCases(t, func(t *testing.T, f *testgoal.PublicationFixture, count *int) error {
		owners := laneVerbOwners{now: func() time.Time { return f.Now }, machine: func(string) (string, error) { return "fixture", nil },
			recordMain: func(r goal.VerbRequest, args goal.TrunkRedRecordArgs) (goal.PublishResult, error) {
				r.Endpoint = f.CountPublications(r.Endpoint, count)
				return goal.RecordTrunkRed(r, args)
			},
		}
		return owners.landingIncidentRecorder(f.Root)([]plain.Result{incidentCheckFixture(t, "TestOne")})
	})
}

func TestIncidentClearCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	candidateCases(t, func(t *testing.T, f *testgoal.PublicationFixture, count *int) error {
		e, err := goal.ResolveEndpoint(f.Root)
		if err != nil {
			t.Fatal(err)
		}
		r := f.Request(e)
		r.Ulid = "01J5X000000000000000000004"
		record, err := goal.RecordTrunkRed(r, goal.TrunkRedRecordArgs{Batch: "batch", Attempt: "red", BaseCommit: f.Base, BaseTree: "tree", SeenAt: f.Now.Format(time.RFC3339), Class: goal.TrunkRedClassTrunkRed,
			Groups: []goal.TrunkRedRecordGroup{{Identity: "red:unit", Group: "unit", Status: "failed", Failures: []goal.TrunkRedFailure{}}}})
		if err != nil || record.Outcome != goal.OutcomeConfirmed {
			t.Fatalf("seed incident: %+v %v", record, err)
		}
		owners := laneVerbOwners{now: func() time.Time { return f.Now.Add(time.Minute) }, machine: func(string) (string, error) { return "fixture", nil },
			clearMain: func(r goal.VerbRequest, args []goal.TrunkRedClearArgs) (goal.PublishResult, error) {
				r.Endpoint = f.CountPublications(r.Endpoint, count)
				return goal.ClearTrunkReds(r, args)
			},
		}
		return owners.clearLandingIncidents(f.Root, plain.Result{Result: plain.Green, Scope: "full", Commit: record.Tip, Tree: "tree", FullTree: "tree", At: "now", FullAt: "now", Attempt: "green"})
	})
}

func candidateCases(t *testing.T, run func(*testing.T, *testgoal.PublicationFixture, *int) error) {
	t.Helper()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			f.Candidate(reference)
			count := 0
			err := run(t, f, &count)
			if reference == "missing" {
				f.AssertRefusal(err)
				if count != 0 {
					t.Fatalf("refused candidate published %d times", count)
				}
			} else if err != nil || count != 1 {
				t.Fatalf("valid candidate: publications=%d error=%v", count, err)
			}
		})
	}
}

func TestFlakeObservationCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	candidateCases(t, func(t *testing.T, f *testgoal.PublicationFixture, count *int) error {
		owners := laneVerbOwners{now: func() time.Time { return f.Now }, machine: func(string) (string, error) { return "fixture", nil },
			recordFlake: func(r goal.VerbRequest, args goal.FlakeRecordArgs) (goal.FlakeRecordResult, error) {
				r.Endpoint = f.CountPublications(r.Endpoint, count)
				return goal.RecordFlake(r, args)
			},
		}
		_, err := owners.landingFlakeRecorder(f.Root)(plain.FlakeRecord{Unit: "unit", Tests: []string{"TestOne"}, Commit: f.Base, Tree: "tree", Attempt: "red", Repeat: "whole", RepeatAttempt: "green", Log: "red.log", RepeatLog: "green.log"})
		return err
	})
}

func TestFlakeEmptyOwnerRepairCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			f := testgoal.NewPublicationFixture(t)
			e, err := goal.ResolveEndpoint(f.Root)
			if err != nil {
				t.Fatal(err)
			}
			r := f.Request(e)
			seeded, err := goal.RecordFlake(r, goal.FlakeRecordArgs{ObserveOnly: true, Unit: "unit", Tests: []string{"TestOne"}, Commit: f.Base, Tree: "tree", Attempt: "red", Repeat: "whole", LogPath: "red.log", Rerun: goal.TrunkRedRerun{Attempt: "green", LogPath: "green.log"}})
			if err != nil {
				t.Fatal(err)
			}
			r.Ulid = "01J5X000000000000000000005"
			r.Now = f.Now.Add(time.Minute)
			seeded, err = goal.RecordFlake(r, goal.FlakeRecordArgs{ObserveOnly: true, Unit: "unit", Tests: []string{"TestOne"}, Commit: f.Base, Tree: "tree", Attempt: "red-two", Repeat: "whole", LogPath: "red-two.log", Rerun: goal.TrunkRedRerun{Attempt: "green-two", LogPath: "green-two.log"}})
			if err != nil {
				t.Fatal(err)
			}

			f.Git("checkout", "-q", "--detach", seeded.Publish.Tip)
			f.Base = seeded.Publish.Tip
			candidate := f.Candidate(reference)
			files, err := goal.ReadCommitGoals(f.Root, candidate)
			if err != nil {
				t.Fatal(err)
			}
			tree, problems := goal.ParseTreeFiles(files)
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			fact := goal.FlakeFacts(tree.TrunkRed)[0]
			fixed, err := (laneVerbOwners{}).landingFlakeFixer(f.Root)(fact, goal.FlakeFix{Identity: fact.Identity, ID: "fix-flaky-unit", Intent: "Fix the flaky test", Next: "Repair its cause", Basis: "Two confirmed sightings", Tier: 3, Priority: 1}, f.Now.Add(time.Minute))
			if reference == "missing" {
				f.AssertRefusal(err)
				if tip := f.Git("rev-parse", goal.LocalLedgerBranch); tip != candidate {
					t.Fatalf("refused repair published %s", tip)
				}
			} else if err != nil || fixed.FixGoal == "" {
				t.Fatalf("valid repair: %+v %v", fixed, err)
			}
		})
	}
}

type candidatePushTransport struct {
	tip   string
	calls int
}

func (p *candidatePushTransport) RemoteTip(string, string, string) (string, bool, error) {
	return p.tip, p.tip != "", nil
}
func (p *candidatePushTransport) Fetch(string, string, string, string) error {
	return errors.New("unexpected fetch")
}
func (p *candidatePushTransport) Push(_, _, _, _, tip string) (branch.CASOutcome, error) {
	p.calls++
	p.tip = tip
	return branch.CASLanded, nil
}

func TestWorkLandRecordsPushSelectedCandidateGitAdapter(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"held", "missing"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			b := newRecordsBed(t)
			f := testgoal.NewPublicationFixture(t)
			selected := f.Candidate(reference)
			f.Write("plans/designs/unstaged.md", "# Draft\n\n- Kind: design\n- Id: unstaged\n- Status: draft\n- Goals: unstaged-missing\n")
			transport := &candidatePushTransport{}
			b.owners.connection.recordsCheck = nil
			b.owners.connection.push = func(req branch.PushRequest) (branch.PushResult, error) {
				// Checkout capture and commit completed before the branch moves. The push
				// selects its tip afterward, with repository hooks disabled in the fixture.
				if b.section || len(b.requests) != 1 {
					t.Fatal("push must select after the checkout section")
				}
				req.Repo = f.Root
				req.EndpointTip = f.Base
				req.Transport = transport
				inputs := branch.PushInputs{
					TxnRefs: func(string) ([]string, error) { return nil, nil },
					Tip: func(_ string, ref string) (string, bool, error) {
						if strings.HasPrefix(ref, "refs/heads/goal/") {
							return selected, true, nil
						}
						return "", false, nil
					},
					Range: func(_, _, tip, _ string) ([]branch.Commit, error) {
						if tip != selected {
							t.Fatalf("range tip=%s", tip)
						}
						return []branch.Commit{{ID: tip, Kind: branch.Plan}}, nil
					},
					WriteTxn: func(_, _ string, txn branch.PushTransaction) error {
						if txn.New != selected {
							t.Fatalf("transaction tip=%s", txn.New)
						}
						return nil
					},
					RecordOrigin: func(_, _, tip string) error {
						if tip != selected {
							t.Fatalf("origin tip=%s", tip)
						}
						return nil
					},
					ClearRef: func(string, string) error { return nil },
				}
				result, err := branch.PushWithInputs(req, inputs)
				if err == nil {
					b.published = result.Tip
				}
				return result, err
			}
			code, result := b.land(recordsPath)
			if reference == "missing" {
				f.AssertRefusal(errors.New(result.Summary))
				if code == 0 || transport.calls != 0 {
					t.Fatalf("invalid selected candidate: code=%d publications=%d result=%+v", code, transport.calls, result)
				}
			} else if code != 0 || transport.calls != 1 || transport.tip != selected {
				t.Fatalf("valid selected candidate: code=%d publications=%d tip=%s result=%+v", code, transport.calls, transport.tip, result)
			}
		})
	}
}
