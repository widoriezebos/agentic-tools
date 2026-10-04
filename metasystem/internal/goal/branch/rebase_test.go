package branch

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type rebaseFixture struct {
	req                                            RebaseRequest
	d                                              rebaseDependencies
	tip, remote, origin                            string
	onMain, dirty, empty                           bool
	prefix                                         string
	mainPaths                                      []string
	replayErr, carryErr, pushErr                   error
	conflict                                       string
	units, old                                     []Commit
	reads                                          map[string]string
	kept                                           []string
	changed                                        bool
	events                                         []string
	claims, loseAt, carries, gates, pushes, closed int
}

func newRebaseFixture(t *testing.T) *rebaseFixture {
	t.Helper()
	f := &rebaseFixture{tip: policyFirst, remote: policyFirst, reads: map[string]string{},
		prefix: "metasystem/", mainPaths: []string{"metasystem/code.go"}}
	f.req = RebaseRequest{Repo: t.TempDir(), GoalID: "goal-a", Remote: "origin", EndpointTip: pushPolicyBase,
		CheckClaim: func() error {
			f.claims++
			if f.claims == f.loseAt {
				return errors.New("claim lost\nrun: metasystem goal claim goal-a")
			}
			return nil
		}, Transport: f}
	f.d = rebaseDependencies{
		repository: commitRepository{
			facts: commitFacts{
				Tip: func(_, ref string) (string, bool, error) {
					v := f.origin
					if ref == policyGoalRef {
						v = f.tip
					}
					return v, v != "", nil
				},
				Staged: func(string) ([]string, error) {
					if f.dirty {
						return []string{"code.go"}, nil
					}
					return nil, nil
				},
				Unstaged: func(string) ([]string, error) { return nil, nil },
				Ancestor: func(_, _, _ string) (bool, error) { return f.onMain, nil },
				Head:     func(string) (string, error) { return policySecond, nil },
				Suffix: func(_, _, _ string) ([]string, error) {
					ids := []string{}
					for _, u := range f.units {
						ids = append(ids, u.ID)
					}
					return ids, nil
				},
				Kind: func(_, id, _ string) (KindInfo, error) {
					if subject, ok := f.reads[id]; ok {
						return KindInfo{Kind: Read, CommitID: subject}, nil
					}
					for _, u := range append(append([]Commit{}, f.units...), f.old...) {
						if u.ID == id {
							return KindInfo{Kind: u.Kind, Units: u.Units, Unit: u.Unit}, nil
						}
					}
					return KindInfo{Kind: Plan}, nil
				},
				Entries: func(_, id string) ([]Entry, error) {
					if f.empty {
						return nil, nil
					}
					return []Entry{{Path: "code.go"}}, nil
				},
				Range: func(_, _, tip, _ string) ([]Commit, error) {
					if tip == policyFirst {
						return f.old, nil
					}
					return f.units, nil
				},
				Index:   func(string) (string, error) { return policyFirst, nil },
				Changes: func(_, _, _ string) ([]string, error) { return nil, nil },
				HeadRef: func(string) string { return policyGoalRef },
			},
			effects: commitEffects{
				Open:     func(_, base string, _ bool) (string, func(), error) { return "scratch", func() { f.closed++ }, nil },
				Checkout: func(_, before, after string) error { f.events = append(f.events, "checkout"); return nil },
				Publish: func(_, _, before, after, origin string) error {
					f.events = append(f.events, "move")
					if before != f.tip {
						t.Fatal("move did not compare the old tip")
					}
					f.tip, f.origin, f.onMain = after, origin, true
					return nil
				},
			},
		},
		git: func(_ string, args ...string) ([]byte, error) {
			switch strings.Join(args, " ") {
			case "merge-base " + pushPolicyBase + " " + f.tip:
				return []byte(policyMoved + "\n"), nil
			case "diff --name-only -z " + policyMoved + " " + pushPolicyBase:
				return []byte(strings.Join(f.mainPaths, "\x00") + "\x00"), nil
			case "rev-parse --show-prefix":
				return []byte(f.prefix + "\n"), nil
			case "rebase --reapply-cherry-picks --empty=keep --no-autosquash " + pushPolicyBase:
				return nil, f.replayErr
			case "diff --name-only --diff-filter=U":
				return []byte(f.conflict), nil
			case "-c format.pretty=%H rebase --show-current-patch":
				return []byte(policyMoved), nil
			case "rebase --abort":
				f.events = append(f.events, "abort")
				return nil, nil
			}
			if args[0] == "update-ref" {
				f.events = append(f.events, "keep")
				f.kept = []string{policyFirst}
				return nil, nil
			}
			t.Fatalf("unexpected git %v", args)
			return nil, nil
		},
		keptTips: func(_, _ string) ([]string, error) { return f.kept, nil },
		change: func(_, id string) (string, error) {
			if f.changed && id != policyMoved {
				return "changed", nil
			}
			return "same", nil
		},
		tests: func(_, _, _ string) ([]TestChange, error) {
			return []TestChange{{Path: "code_test.go", ReaderWord: "the test still checks the same behavior"}}, nil
		},
		gate: func(req ReadGateRequest) (GateObservation, error) {
			f.gates++
			if req.UnitCommit != policySecond || req.NewID != nil {
				t.Fatalf("gate request %+v", req)
			}
			return GateObservation{RunID: "gate-run", Tree: policySecond}, nil
		},
		commitRead: func(req CommitReadRequest) (string, Attestation, error) {
			f.carries++
			if len(req.TestsChanged) != 1 || req.TestsChanged[0].ReaderWord == "" || req.Carry != policyMoved || req.GateRunID != "gate-run" || req.GateTree != policySecond || req.Repo != f.req.Repo || req.EndpointTip != f.req.EndpointTip || req.OpID == "" || req.CheckClaim == nil {
				t.Fatalf("carry request %+v", req)
			}
			if f.remote != f.origin {
				t.Fatalf("carry would be stale: remote %s record %s", f.remote, f.origin)
			}
			if f.carryErr != nil {
				return "", Attestation{}, f.carryErr
			}
			f.tip = policySecond + "read"
			f.units = append(f.units, Commit{ID: "new-read", Kind: Read})
			f.reads["new-read"] = policySecond
			return f.tip, Attestation{}, nil
		},
		push: func(req PushRequest) (PushResult, error) {
			f.pushes++
			if f.origin != f.remote {
				t.Fatal("push would be stale")
			}
			if f.pushErr != nil {
				return PushResult{}, f.pushErr
			}
			f.remote, f.origin = f.tip, f.tip
			return PushResult{Tip: f.tip}, nil
		},
		newID: func(prefix string) (string, error) { return prefix + "-fixture", nil },
	}
	return f
}

func (f *rebaseFixture) RemoteTip(_, _, _ string) (string, bool, error) {
	return f.remote, f.remote != "", nil
}
func (*rebaseFixture) Fetch(_, _, _, _ string) error { panic("unexpected fetch") }
func (*rebaseFixture) Push(_, _, _, _, _ string) (CASOutcome, error) {
	panic("unexpected transport push")
}

func (f *rebaseFixture) reviewedUnit() {
	f.units = []Commit{{ID: policySecond, Kind: Unit, Unit: "u1", Units: []string{"u1"}}}
	f.old = []Commit{{ID: policyMoved, Kind: Unit, Unit: "u1", Units: []string{"u1"}}, {ID: "old-read", Kind: Read}}
	f.reads["old-read"] = policyMoved
}

func TestRebasePolicy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, prefix, state string
		paths               []string
	}{
		{"live ledger", "metasystem/", "held", []string{"metasystem/plans/goals/goal-a.md"}},
		{"recorded ledger", "metasystem/", "held", []string{"metasystem/records/goals/goal-a.md"}},
		{"mixed change", "metasystem/", "rebased", []string{"metasystem/plans/goals/goal-a.md", "metasystem/code.go"}},
		{"outside installation", "metasystem/", "rebased", []string{"plans/goals/goal-a.md"}},
		{"root installation", "", "held", []string{"plans/goals/goal-a.md", "records/goals/goal-b.md"}},
		{"no net change", "metasystem/", "held", nil},
		{"path with newline", "metasystem/", "held", []string{"metasystem/records/goals/a\nb.md"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newRebaseFixture(t)
			f.prefix, f.mainPaths = test.prefix, test.paths
			got, err := rebaseWith(f.req, f.d)
			if err != nil || got.State != test.state {
				t.Fatalf("main changes %q: %+v %v", test.paths, got, err)
			}
			if test.state == "held" && (got.NewTip != policyFirst || len(f.events) != 0 || f.carries+f.gates+f.pushes+f.closed != 0) {
				t.Fatalf("ledger-only main wrote something: %+v %+v", got, f)
			}
		})
	}
	t.Run("ledger-only main still publishes an unpublished branch", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.mainPaths = []string{"metasystem/plans/goals/goal-a.md"}
		f.remote = ""
		got, err := rebaseWith(f.req, f.d)
		if err != nil || got.State != "pushed" || got.NewTip != policyFirst || f.pushes != 1 || len(f.events) != 0 || f.closed != 0 {
			t.Fatalf("publication: %+v %v %+v", got, err, f)
		}
	})
	t.Run("ledger-only main still carries missing reviews", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.mainPaths = []string{"metasystem/plans/goals/goal-a.md"}
		f.reviewedUnit()
		f.tip, f.origin, f.kept = policySecond, f.remote, []string{policyFirst}
		got, err := rebaseWith(f.req, f.d)
		if err != nil || !reflect.DeepEqual(got.Carried, []string{"u1"}) || f.carries != 1 || f.gates != 1 || f.pushes != 1 || len(f.events) != 0 || f.closed != 0 {
			t.Fatalf("carry: %+v %v %+v", got, err, f)
		}
	})
	for _, command := range []string{"merge-base", "diff", "rev-parse"} {
		t.Run("main change read fails at "+command, func(t *testing.T) {
			f := newRebaseFixture(t)
			git := f.d.git
			failure := errors.New("main changes unavailable")
			f.d.git = func(repo string, args ...string) ([]byte, error) {
				if args[0] == command {
					return nil, failure
				}
				return git(repo, args...)
			}
			_, err := rebaseWith(f.req, f.d)
			if !errors.Is(err, failure) || len(f.events) != 0 || f.carries+f.gates+f.pushes+f.closed != 0 {
				t.Fatalf("read failure: %v %+v", err, f)
			}
		})
	}
	t.Run("hold_writes_nothing", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.onMain = true
		got, err := rebaseWith(f.req, f.d)
		if err != nil || got.State != "held" || len(f.events) != 0 || f.carries+f.gates+f.pushes+f.closed != 0 {
			t.Fatalf("hold %+v %v %+v", got, err, f)
		}
	})
	for _, name := range []string{"dirty", "stale", "claim"} {
		t.Run(name, func(t *testing.T) {
			f := newRebaseFixture(t)
			switch name {
			case "dirty":
				f.dirty = true
			case "stale":
				f.remote = policyMoved
			case "claim":
				f.loseAt = 1
			}
			_, err := rebaseWith(f.req, f.d)
			if err == nil || len(f.events) != 0 || f.closed != 0 {
				t.Fatalf("precondition %v %+v", err, f.events)
			}
		})
	}
	t.Run("conflict_aborts", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.replayErr = errors.New("conflict")
		f.conflict = "code.go\nother.go"
		f.reviewedUnit()
		_, err := rebaseWith(f.req, f.d)
		var refusal *OpError
		if !errors.As(err, &refusal) || refusal.Code != RebaseConflictCode || !strings.Contains(err.Error(), "build u1") || !strings.Contains(err.Error(), "other.go") || !reflect.DeepEqual(f.events, []string{"abort"}) || f.closed != 1 || f.tip != policyFirst {
			t.Fatalf("conflict %v %+v", err, f)
		}
	})
	t.Run("empty_unit", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.reviewedUnit()
		f.empty = true
		_, err := rebaseWith(f.req, f.d)
		if err == nil || !strings.Contains(err.Error(), "already on main") || len(f.events) != 0 || f.tip != policyFirst || f.closed != 1 {
			t.Fatalf("empty %v %+v", err, f)
		}
	})
	for _, origin := range []string{"", pushPolicyBase} {
		t.Run("origin_record_"+origin, func(t *testing.T) {
			f := newRebaseFixture(t)
			f.origin = origin
			f.reviewedUnit()
			got, err := rebaseWith(f.req, f.d)
			if err != nil || got.State != "rebased" || !reflect.DeepEqual(got.Carried, []string{"u1"}) || f.carries != 1 || f.gates != 1 || f.pushes != 1 || !reflect.DeepEqual(f.events, []string{"keep", "checkout", "move"}) {
				t.Fatalf("rebase %+v %v %+v", got, err, f)
			}
			before := append([]string{}, f.events...)
			got, err = rebaseWith(f.req, f.d)
			if err != nil || got.State != "held" || f.carries != 1 || f.pushes != 1 || !reflect.DeepEqual(before, f.events) {
				t.Fatalf("repeat %+v %v %+v", got, err, f)
			}
		})
	}
	for _, name := range []string{"changed", "unreviewed", "no_predecessor", "stale_read"} {
		t.Run(name, func(t *testing.T) {
			f := newRebaseFixture(t)
			f.reviewedUnit()
			switch name {
			case "changed":
				f.changed = true
			case "unreviewed":
				delete(f.reads, "old-read")
				f.old = f.old[:1]
			case "no_predecessor":
				f.old = nil
			case "stale_read":
				f.carryErr = operationRefusal(ReadStaleCode, "review changed")
			}
			got, err := rebaseWith(f.req, f.d)
			checks := 0
			if name == "stale_read" {
				checks = 1
			}
			if err != nil || !reflect.DeepEqual(got.NeedsReview, []string{"u1"}) || f.gates != checks || f.carries != checks || f.pushes != 1 {
				t.Fatalf("needs review %+v %v %+v", got, err, f)
			}
		})
	}
	t.Run("resume_after_move", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.reviewedUnit()
		f.onMain = true
		f.tip = policySecond
		f.origin = f.remote
		f.kept = []string{policyFirst}
		f.units = append(f.units, Commit{ID: "other", Kind: Unit, Unit: "u2", Units: []string{"u2"}}, Commit{ID: "other-read", Kind: Read})
		f.reads["other-read"] = "other"
		got, err := rebaseWith(f.req, f.d)
		if err != nil || f.carries != 1 || f.pushes != 1 || len(f.events) != 0 || len(got.Carried) != 1 {
			t.Fatalf("resume %+v %v %+v", got, err, f)
		}
	})
	t.Run("lost_claim_before_move", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.loseAt = 3
		_, err := rebaseWith(f.req, f.d)
		if err == nil || f.tip != policyFirst || !reflect.DeepEqual(f.events, []string{"keep", "checkout", "checkout"}) || f.pushes != 0 {
			t.Fatalf("lost claim %v %+v", err, f)
		}
	})
	t.Run("range_refusal", func(t *testing.T) {
		f := newRebaseFixture(t)
		failure := rangeRefusal("goal-a", policySecond, "the branch's commits do not fit")
		f.d.repository.facts.Range = func(_, _, _, _ string) ([]Commit, error) { return nil, failure }
		_, err := rebaseWith(f.req, f.d)
		if !errors.Is(err, failure) || len(f.events) != 0 || f.closed != 1 || f.tip != policyFirst {
			t.Fatalf("range refusal %v %+v", err, f)
		}
	})
	t.Run("lost_claim_before_keep", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.loseAt = 2
		_, err := rebaseWith(f.req, f.d)
		if err == nil || len(f.events) != 0 || f.tip != policyFirst {
			t.Fatalf("keep without claim %v %+v", err, f)
		}
	})
	t.Run("lost_claim_before_push", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.loseAt = 4
		_, err := rebaseWith(f.req, f.d)
		if err == nil || f.pushes != 0 || f.remote != policyFirst || f.tip != policySecond {
			t.Fatalf("push without claim %v %+v", err, f)
		}
	})
	t.Run("carry_error_stops", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.reviewedUnit()
		f.carryErr = errors.New("review record unavailable")
		_, err := rebaseWith(f.req, f.d)
		if !errors.Is(err, f.carryErr) || f.pushes != 0 || f.tip != policySecond {
			t.Fatalf("carry error %v %+v", err, f)
		}
	})

	t.Run("resume_after_carry", func(t *testing.T) {
		f := newRebaseFixture(t)
		f.reviewedUnit()
		f.pushErr = errors.New("push interrupted")
		if _, err := rebaseWith(f.req, f.d); err == nil {
			t.Fatal("push interruption passed")
		}
		f.pushErr = nil
		got, err := rebaseWith(f.req, f.d)
		if err != nil || f.carries != 1 || f.gates != 1 || f.pushes != 2 || got.NewTip != f.remote {
			t.Fatalf("resume %+v %v %+v", got, err, f)
		}
	})
}
