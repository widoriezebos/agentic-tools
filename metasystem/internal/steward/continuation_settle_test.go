package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func unstampedContinuationFixture(t *testing.T, root string) string {
	t.Helper()
	nonce := "ended-continuation"
	writeStewardRecord(t, filepath.Join(consumedDir(root), nonce+".json"), map[string]any{
		"nonce": nonce, "jobId": "steward-" + nonce, "reason": "seatHandoff", "launchStamped": false,
	})
	return nonce
}

func endedContinuationFixture(t *testing.T, root string, ended bool) string {
	t.Helper()
	nonce := unstampedContinuationFixture(t, root)
	job := map[string]any{"status": "failed", "phase": "handshake", "error": "runtime_error"}
	if ended {
		job["endedAt"] = "2026-10-01T10:00:00Z"
	}
	writeStewardRecord(t, filepath.Join(root, "artifacts", "agents", "jobs", "steward-"+nonce+".json"), job)
	return nonce
}

func TestAnEndedContinuationSettlesOnTheTick(t *testing.T) {
	t.Parallel()
	for _, state := range []struct {
		name             string
		ended, jobRecord bool
	}{
		{"ended=true", true, true},
		{"ended=false", false, true},
		{"consumed, unstamped, no job record", false, false},
	} {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/pending=%t", state.name, pending), func(t *testing.T) {
				t.Parallel()
				ended := state.ended
				bed := newAttentionPolicyBed(t)
				file := seatReadyGoal("fix-it", "Continue it.")
				bed.world("base", map[string]*goal.GoalFile{file.Id: file})
				if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=claude\nlaunch.seat.runtime=claude\n"), 0600); err != nil {
					t.Fatal(err)
				}
				seat := &seatBed{t: t, root: bed.root, now: bed.now, goals: map[string]*goal.GoalFile{}, launcher: &fakeSeatLauncher{}, tips: map[string]string{}}
				seat.put(file)
				home := testprovider.Register(t, bed.root)
				f := newTickContinuationFixture(t, bed, false)
				f.cfg.ProviderHome = home
				f.census = fakeCensus{workers: deadWorkers}
				f.dependencies.openWork.Seat = seat.seatDependencies()
				var nonce string
				if state.jobRecord {
					nonce = endedContinuationFixture(t, bed.root, ended)
				} else {
					nonce = unstampedContinuationFixture(t, bed.root)
				}
				decision := &decisionTickRepository{t: t, root: bed.root, head: strings.Repeat("a", 40), files: bed.worlds["base"]}
				before := Evidence{Marks: decision.currentMarks(), CurrentContinuation: nonce, DryRevivals: 1}
				if pending {
					before.AbnormalCount = 1
					before.Abnormal[0] = AbnormalRestart{At: bed.now.Add(-time.Minute), Class: "failed", Nonce: nonce, Pending: true, Marks: before.Marks}
				}
				if err := writeSeatRecord(bed.root, SeatRecord{Schema: 1, LaunchID: "earlier-seat", Goal: "fix-it", StartedAt: bed.now.Add(-time.Hour).Format(time.RFC3339Nano), ReapedAt: bed.now.Add(-time.Minute).Format(time.RFC3339), LaunchState: "completed", Outcome: SeatProgress}); err != nil {
					t.Fatal(err)
				}
				if err := SaveEvidence(bed.root, EvidencePath(bed.root), before); err != nil {
					t.Fatal(err)
				}
				result, completed, err := f.run(t)
				f.checkWork()
				if err != nil || !completed || result.Evidence.SampledAt == "" {
					t.Fatalf("tick/provider observation: %+v completed=%t %v", result, completed, err)
				}
				saved, err := LoadEvidence(EvidencePath(bed.root))
				if err != nil || saved != result.Evidence || saved.DryRevivals != before.DryRevivals {
					t.Fatalf("tick lost evidence or its charge: %+v %v", saved, err)
				}
				if ended {
					if saved.CurrentContinuation != "" || saved.SettledContinuation != nonce || pending && saved.Abnormal[0].Pending || strings.Contains(result.Decision.Reason, "unknown launch outcome") {
						t.Fatalf("ended job still holds: %+v %+v", saved, result.Decision)
					}
				} else if saved.CurrentContinuation != nonce || saved.SettledContinuation != "" || pending && !saved.Abnormal[0].Pending {
					t.Fatalf("live job was settled: %+v", saved)
				}
				class, reason, err := abnormalRestartState(bed.root, saved, bed.now, nil)
				if err != nil {
					t.Fatal(err)
				}
				if !ended {
					if !strings.Contains(reason, "the previous automatic restart has an unknown launch outcome") || !strings.Contains(result.Decision.Reason, "the previous automatic restart has an unknown launch outcome") {
						t.Fatalf("live job lost its hold: %s", reason)
					}
					return
				}
				if class != "failed" || strings.Contains(reason, "unknown launch outcome") {
					t.Fatalf("ended job outcome was replaced by the earlier seat's progress: %s %s", class, reason)
				}
				if pending {
					return
				}
				record, err := startSeatWithDependencies(bed.root, SeatSelection{Goal: "fix-it"}, *seat.seatDependencies())
				saved, readErr := LoadEvidence(EvidencePath(bed.root))
				if err != nil || record.LaunchID == "" || len(seat.launcher.starts) != 1 || readErr != nil || saved.AbnormalCount != 1 || saved.Abnormal[0].Class != "failed" || saved.SettledContinuation != "" {
					t.Fatalf("next seat did not consume the outcome once: %+v %+v %v %v", record, saved, err, readErr)
				}
			})
		}
	}
}

func TestReviveSettlesAnEndedPendingContinuation(t *testing.T) {
	t.Parallel()
	for _, ended := range []bool{true, false} {
		t.Run(fmt.Sprint(ended), func(t *testing.T) {
			t.Parallel()
			reads := int32(0)
			if ended {
				reads = 1
			}
			revival := newRevivalFixture(t, reads, reads)
			root := revival.root
			nonce := endedContinuationFixture(t, root, ended)
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			before := Evidence{CurrentContinuation: nonce, DryRevivals: 1, AbnormalCount: 1}
			before.Abnormal[0] = AbnormalRestart{At: now.Add(-time.Minute), Class: "failed", Nonce: nonce, Pending: true}
			if err := SaveEvidence(root, EvidencePath(root), before); err != nil {
				t.Fatal(err)
			}
			intent := testIntent("next-continuation")
			if err := PrepareIntent(root, filepath.Join(root, "receipt.log"), intent); err != nil {
				t.Fatal(err)
			}
			out, err := revival.complete(TickConfig{Now: now}, deadCensus(), intent.Nonce, func(Intent) error { t.Fatal("held revival launched"); return nil })
			saved, readErr := LoadEvidence(EvidencePath(root))
			if err != nil || readErr != nil || out.Launched || saved.DryRevivals != before.DryRevivals || saved.Abnormal[0].Pending == ended {
				t.Fatalf("pending resolution: %+v %+v %v %v", out, saved, err, readErr)
			}
			if strings.Contains(out.Reason, "unknown launch outcome") == ended {
				t.Fatalf("wrong pending hold: %+v", out)
			}
		})
	}
}

func TestContinuationCloseRefusesUnreadableEvidence(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"intent", "job", "malformed"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			nonce := endedContinuationFixture(t, root, false)
			path := filepath.Join(consumedDir(root), nonce+".json")
			if missing != "intent" {
				path = filepath.Join(root, "artifacts", "agents", "jobs", "steward-"+nonce+".json")
			}
			if missing == "malformed" {
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			before := Evidence{CurrentContinuation: nonce, AbnormalCount: 1, DryRevivals: 2}
			if err := SaveEvidence(root, EvidencePath(root), before); err != nil {
				t.Fatal(err)
			}
			err := closeUnboundLaunch(root, nil, "")
			saved, readErr := LoadEvidence(EvidencePath(root))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if missing == "malformed" {
				if _, ok := err.(SeatRevivalClosed); err == nil || ok || saved != before {
					t.Fatalf("unreadable close changed evidence: %+v %v", saved, err)
				}
			} else if saved.CurrentContinuation != "" || saved.AbnormalCount != 0 {
				t.Fatalf("missing record still holds: %+v %v", saved, err)
			}
		})
	}
}
