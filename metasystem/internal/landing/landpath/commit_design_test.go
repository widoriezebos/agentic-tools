package landpath

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func TestLandingDesignCheckHandRoute(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"warn", "refuse"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			b.epoch = epochOf(4)
			design := landing.ObserveDesign(landing.DesignFacts{Facts: designgate.Facts{Goal: "g1", Tier: 2, Mode: mode}}, false)
			b.observed.Design = &design
			want, commits := 0, 1
			if mode == "refuse" {
				b.observed.Mode, b.observed.RefusesAgent, b.observed.Code = "refuse", true, "LANDING_DESIGN_NOT_STANDING"
				b.observed.VerdictTrailer = "would-refuse code=LANDING_DESIGN_NOT_STANDING"
				want, commits = 3, 0
			}
			b.expect(b.commit(CommitRequest{Goal: "g1", GoalSet: true, Chain: "j1", OwnerLineage: "L"}), want)
			pair := design.Pair[0] + "\n" + design.Pair[1] + "\n"
			if mode == "refuse" {
				pair = design.Pair[0] + "\nrun: " + design.Pair[1] + "\n"
			}
			if b.git.commits != commits || strings.Count(b.stderr.String(), pair) != 1 {
				t.Fatalf("commits=%d stderr=%q pair=%q", b.git.commits, b.stderr.String(), design.Pair)
			}
		})
	}
}

func TestLandingDesignCheckPersonException(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	design := landing.ObserveDesign(landing.DesignFacts{Facts: designgate.Facts{Goal: "g1", Tier: 2, Mode: "refuse"}}, true)
	b.observed.Design = &design
	b.observed.Code, b.observed.Provenance, b.observed.VerdictTrailer = "human-carried", "carried opid=op1 past=group:unit ledger=L1 x", "pass carried"
	b.observed.Carried = &landing.CarriedBinding{Opid: "op1", Past: "group:unit", Ledger: "L1"}
	b.expect(b.commit(CommitRequest{Goal: "g1", GoalSet: true, Carried: "op1", LedgerTip: "L1", CarriedBy: "human:wido", CarriedPast: "group:unit", HeldEpoch: "human"}), 0)
	if b.git.commits != 1 || strings.Count(b.stderr.String(), design.Pair[0]+"\n"+design.Pair[1]+"\n") != 1 || !strings.HasSuffix(design.Pair[0], "; it goes on at your word") {
		t.Fatalf("person's exception: commits=%d stderr=%q", b.git.commits, b.stderr.String())
	}
}

func TestLandingDesignCheckPersonHandObserveToCommit(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	b.owners.Live = func() Judge {
		return Judge{Observe: func(request ObserveRequest) (landing.Observation, int) {
			observed := landing.Observe(landing.ObserveParams{RepoRoot: request.Root, CandidateTree: request.Tree,
				Goal: request.Goal, Actor: request.Actor, DesignFacts: func() landing.DesignFacts {
					return landing.DesignFacts{Facts: designgate.Facts{Goal: request.Goal, Tier: 2, Mode: "refuse"}}
				}})
			if observed.Design == nil || !observed.Design.Person || observed.Design.RefusesAgent {
				t.Fatalf("person's observation: %+v", observed)
			}
			return observed, 0
		}}
	}
	b.expect(b.commit(CommitRequest{Goal: "g1", GoalSet: true}), 0)
	if b.git.commits != 1 || strings.Count(b.stderr.String(), "; it goes on at your word\n") != 1 || strings.Contains(b.stderr.String(), "nothing was landed") {
		t.Fatalf("person's landing: commits=%d stderr=%q", b.git.commits, b.stderr.String())
	}
}

func TestLandingDesignCheckDigestAfterCompletion(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"commit", "commit push", "land", "land commit only", "failed push", "failed commit push", "digest failure", "no goal"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newBed(t)
			writes := 0
			b.owners.Live = func() Judge {
				return Judge{Observe: func(request ObserveRequest) (landing.Observation, int) {
					return landing.Observe(landing.ObserveParams{RepoRoot: request.Root, CandidateTree: request.Tree,
						Goal: request.Goal, Actor: request.Actor, DesignFacts: func() landing.DesignFacts {
							return landing.DesignFacts{Facts: designgate.Facts{Goal: request.Goal, Tier: 2, Mode: "refuse"}}
						}}), 0
				}}
			}
			b.owners.Verify = func(VerifyRequest, io.Writer, io.Writer) int {
				if writes != 0 {
					t.Fatal("digest changed the checkout before proof")
				}
				return 0
			}
			b.owners.Advance = func(string, string, io.Writer, io.Writer) int {
				if writes != 0 {
					t.Fatal("digest changed the checkout before rebase")
				}
				b.git.head = "rebased"
				return 0
			}
			pushes := 0
			b.git.on("push", func(GitCall) GitResult {
				if writes != 0 {
					t.Fatal("digest changed the checkout before push")
				}
				pushes++
				if strings.HasPrefix(route, "failed") {
					return failed(1, "remote: permission denied")
				}
				return ok("")
			})
			b.owners.RecordDesign = func(root, goal string, design *landing.DesignObservation, stderr io.Writer) {
				writes++
				if root != b.root || goal != "g1" || b.git.commits != 1 || !design.Person || !strings.HasSuffix(design.Pair[0], "; it goes on at your word") {
					t.Fatalf("digest after landing: root=%s goal=%s commits=%d design=%+v", root, goal, b.git.commits, design)
				}
				wantHead := "c1"
				if route == "land" || route == "digest failure" {
					wantHead = "rebased"
					if pushes != 1 {
						t.Fatalf("digest before the landing's push: pushes=%d", pushes)
					}
				}
				if b.git.head != wantHead {
					t.Fatalf("digest commit=%s, want %s", b.git.head, wantHead)
				}
				if route == "digest failure" {
					fmt.Fprintln(stderr, "warning: digest unavailable; the landing goes on\nnothing to do: the verdict still stands")
					return
				}
				if err := os.WriteFile(filepath.Join(root, "digest"), []byte(design.Pair[0]), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			goal := "g1"
			wantStatus, wantWrites := 0, 1
			if route == "no goal" {
				goal, wantWrites = "", 0
			}
			if strings.HasPrefix(route, "failed") {
				wantStatus, wantWrites = 1, 0
			}
			var status int
			if strings.HasPrefix(route, "commit") || route == "failed commit push" {
				status = b.commit(CommitRequest{Goal: goal, GoalSet: true, Push: route != "commit"})
			} else {
				status = b.land(LandRequest{StagedOnly: true, Goal: goal, GoalSet: goal != "", CommitOnly: route == "land commit only"})
			}
			b.expect(status, wantStatus)
			if writes != wantWrites {
				t.Fatalf("digest writes=%d, want %d", writes, wantWrites)
			}
		})
	}
}
