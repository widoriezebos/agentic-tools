package steward

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// The steward's duties are driven through the resident runner's loop with the
// real tick, as production runs them: each cycle ticks (and observes health),
// then runs the duties, so a tick reads the previous cycle's outcomes. The
// world is an enrolled Git checkout with live process evidence, so the
// unchanged roles stay alive and every change below belongs to a duty.

var (
	errDutyA = errors.New("an answer needs a valid recorded operation id")
	errDutyB = errors.New("the reply names no machine")
	errDutyC = errors.New("the channel timed out")
	errDutyD = errors.New("the channel refused the token")
)

// dutyTestRole is a duty's health role, as the person reads it.
func dutyTestRole(name string) HealthRole { return HealthRole("steward-duty:" + name) }

// dutyCycle is what the injected duties return on one cycle.
type dutyCycle struct{ replies, channel, trim, pending error }

type dutyBed struct {
	t        *testing.T
	root     string
	health   *healthBed
	cfg      TickConfig
	noTrim   bool
	now      time.Time
	cycle    int
	verdicts map[int]HealthVerdict
	records  map[int]healthRecord
	journal  map[int]int
	alerts   map[int][]AlertEpisode
}

func newDutyBed(t *testing.T, goals ...*goal.GoalFile) *dutyBed {
	t.Helper()
	root := canonicalPath(claimQuotaTickBed(t, nil, goals...))
	testprovider.Register(t, root)
	health := newHealthBedAt(t, root, EnrollmentFixture, "")
	self := int64(os.Getpid())
	health.probe[self] = struct {
		exact identity.Exact
		state identity.Liveness
		err   error
	}{exact: identity.Exact{Pid: self, StartedAt: time.Unix(1_700_000_000, 0), StartTicks: 7, BootID: "health-bed-boot"}, state: identity.Alive}
	// The bed's own host state home, with a machine disk report no other test
	// writes.
	home := t.TempDir()
	report, err := json.Marshal(diskstore.Report{Schema: diskstore.ReportSchema, Kind: "machine", At: health.base,
		Health: diskstore.Health{Status: diskstore.HealthOK, Reason: "free space is above the floor"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(diskstore.MachineReportPath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diskstore.MachineReportPath(home), report, 0o644); err != nil {
		t.Fatal(err)
	}
	return &dutyBed{t: t, root: root, health: health, now: health.base,
		cfg: TickConfig{ProviderHome: testprovider.Home(root), AskHome: t.TempDir(), WorkStateRoot: home}, verdicts: map[int]HealthVerdict{}, records: map[int]healthRecord{}, journal: map[int]int{}, alerts: map[int][]AlertEpisode{}}
}

// run drives the runner loop for cycles more cycles. after runs inside each
// cycle's sleep, after that cycle's duties.
func (b *dutyBed) run(cycles int, plan func(int) dutyCycle, after func(int)) {
	t := b.t
	t.Helper()
	last := b.cycle + cycles
	slept := b.cycle
	deps := runnerLoopDependencies{
		Self: b.health.probe,
		Tick: func(top string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
			b.cycle++
			// The watcher keeps its own evidence fresh between ticks.
			b.health.census(b.now)
			b.health.componentSuccess("repo-watcher", 1, b.health.watcher, b.now)
			cfg.Now, cfg.Prober = b.now, b.health.probe
			result, err := RunTick(top, cfg, census)
			if err != nil {
				t.Errorf("cycle %d: tick: %v", b.cycle, err)
			}
			episodes, episodesErr := AlertEpisodes(top)
			if episodesErr != nil {
				t.Errorf("cycle %d: %v", b.cycle, episodesErr)
			}
			record, recordErr := loadHealthRecord(HealthRecordPath(top))
			if recordErr != nil {
				t.Errorf("cycle %d: the health record does not load: %v", b.cycle, recordErr)
			}
			// A helm tick returns no health; its observation is the record's.
			observed := result.Health
			if observed.Schema == 0 {
				observed = record.Verdict
			}
			b.verdicts[b.cycle], b.records[b.cycle], b.journal[b.cycle], b.alerts[b.cycle] = observed, record, b.journalLines(), episodes
			return result, err
		},
		SettleReplies: func(context.Context, string) (int, error) { return 0, plan(b.cycle).replies },
		Channel: func(context.Context, string) (int, error) {
			if err := plan(b.cycle).channel; err != nil {
				return 2, err
			}
			return 0, nil
		},
		DeliverPending: func(string) (int, error) { return 0, plan(b.cycle).pending },
		Resumable:      func(string) (string, bool, error) { return "", false, nil },
		TrimCaches:     func(context.Context, string, TickConfig) error { return plan(b.cycle).trim },
		SweepDisk:      func(string, time.Time, bool) error { return nil },
		StartSeat: func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
			t.Errorf("cycle %d started a seat", b.cycle)
			return SeatRecord{}, errors.New("no seat in this bed")
		},
		Now: func() time.Time { return b.now },
		Sleep: func(time.Duration) {
			b.now = b.now.Add(time.Minute)
			if slept == b.cycle {
				return
			}
			slept = b.cycle
			if after != nil {
				after(b.cycle)
			}
			if b.cycle >= last {
				if err := stopRunnerLoop(b.root); err != nil {
					t.Fatal(err)
				}
			}
		},
	}
	if b.noTrim {
		deps.TrimCaches = nil
	}
	if err := runLoopWithDependencies(b.root, fakeCensus{}, nil, time.Minute, b.cfg, deps); err != nil {
		t.Fatal(err)
	}
	if b.cycle != last {
		t.Fatalf("the loop ran %d cycles, want %d", b.cycle, last)
	}
}

func (b *dutyBed) journalLines() int {
	file, err := os.Open(filepath.Join(runnerDir(b.root), NotificationJournalName))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		b.t.Fatal(err)
	}
	defer file.Close()
	lines := 0
	for scanner := bufio.NewScanner(file); scanner.Scan(); {
		lines++
	}
	return lines
}

// duties reads the duties record on disk.
func (b *dutyBed) duties() map[string]struct {
	Error string `json:"error"`
} {
	b.t.Helper()
	record := map[string]struct {
		Error string `json:"error"`
	}{}
	data, err := os.ReadFile(filepath.Join(b.root, "artifacts", "agents", "steward", "duties", "duties.json"))
	if err != nil {
		b.t.Fatalf("the duties record: %v", err)
	}
	if err := json.Unmarshal(data, &record); err != nil {
		b.t.Fatalf("the duties record: %v", err)
	}
	return record
}

// role is one role of the cycle's observation; it must appear exactly once.
func (b *dutyBed) role(cycle int, role HealthRole) RoleVerdict {
	b.t.Helper()
	var found []RoleVerdict
	for _, candidate := range b.verdicts[cycle].Roles {
		if candidate.Role == role {
			found = append(found, candidate)
		}
	}
	if len(found) != 1 {
		b.t.Fatalf("cycle %d holds %d %s roles: %+v", cycle, len(found), role, b.verdicts[cycle].Roles)
	}
	return found[0]
}

// episodes are the episodes owner held right after the cycle's tick.
func (b *dutyBed) episodes(cycle int, owner string) []AlertEpisode {
	var owned []AlertEpisode
	for _, episode := range b.alerts[cycle] {
		if episode.Owner == owner {
			owned = append(owned, episode)
		}
	}
	return owned
}

// quiet runs the two-cycle baseline with every duty succeeding: every role
// alive, nothing to alert, no episode and no delivery.
func (b *dutyBed) quiet() {
	b.t.Helper()
	b.run(2, func(int) dutyCycle { return dutyCycle{} }, nil)
	for cycle := 1; cycle <= 2; cycle++ {
		verdict := b.verdicts[cycle]
		for _, role := range verdict.Roles {
			if role.Status != HealthAlive {
				b.t.Fatalf("baseline cycle %d: %s is %s: %s", cycle, role.Role, role.Status, role.Reason)
			}
		}
		// The disk role read the bed's own machine report.
		if got := b.role(cycle, RoleDisk); got.Reason != "free space is above the floor and every disk setting is readable" {
			b.t.Fatalf("baseline cycle %d: the disk role did not read the bed's report: %s", cycle, got.Reason)
		}
		if verdict.ShouldAlert || verdict.Aggregate != "healthy" || b.journal[cycle] != 0 || len(b.alerts[cycle]) != 0 {
			b.t.Fatalf("baseline cycle %d is not quiet: alert %t, %s, %d deliveries, episodes %+v", cycle, verdict.ShouldAlert, verdict.Aggregate, b.journal[cycle], b.alerts[cycle])
		}
	}
}

// submitted is how many delivery attempts the cycle's episodes hold.
func (b *dutyBed) submitted(cycle int) int {
	attempts := 0
	for _, episode := range b.alerts[cycle] {
		attempts += len(episode.Attempts)
	}
	return attempts
}

// failing reports a dead duty role that is not standing, with its count.
func (b *dutyBed) failing(cycle int, role HealthRole, reason string, count int) {
	b.t.Helper()
	got := b.role(cycle, role)
	if got.Status != HealthDead || got.Reason != reason || got.ConsecutiveFailures != count || got.Standing || got.FailureEscalation != NoLawfulRemedy {
		b.t.Fatalf("cycle %d: %s = %+v, want dead %q at failure %d, not standing, with no lawful remedy", cycle, role, got, reason, count)
	}
}

func TestAFailingDutyIsARowAndStandsAtFive(t *testing.T) {
	t.Parallel()
	replies := dutyTestRole("question-replies")
	standing := PatternOwner("health-standing-red")

	t.Run("one failure is a row and notifies nothing; another error restarts the count", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		bed.quiet()
		bed.run(3, func(cycle int) dutyCycle {
			switch cycle {
			case 3:
				return dutyCycle{replies: errDutyA}
			case 4:
				return dutyCycle{replies: errDutyB}
			}
			return dutyCycle{}
		}, func(cycle int) {
			if cycle == 3 {
				if got := bed.duties()["question-replies"].Error; got != errDutyA.Error() {
					t.Fatalf("the duties record holds %q for question-replies, want %q", got, errDutyA)
				}
			}
		})
		bed.failing(4, replies, "question-replies failed: "+errDutyA.Error(), 1)
		verdict := bed.verdicts[4]
		if verdict.ShouldAlert != bed.verdicts[2].ShouldAlert || verdict.Aggregate != "unhealthy" {
			t.Fatalf("one duty failure: alert %t, aggregate %s; want the baseline's alert and unhealthy", verdict.ShouldAlert, verdict.Aggregate)
		}
		if verdict.FindingDigest != bed.verdicts[2].FindingDigest {
			t.Fatal("a duty role that is not standing entered the finding digest")
		}
		// An unhealthy reading keeps its finding unsent; nothing is submitted.
		if bed.submitted(4) != bed.submitted(2) || bed.journal[4] != bed.journal[2] {
			t.Fatalf("one duty failure delivered %d notices", bed.journal[4]-bed.journal[2])
		}
		if line := verdict.Line(); !strings.Contains(line, string(replies)+"=dead (question-replies failed: "+errDutyA.Error()) {
			t.Fatalf("the health line does not show the row: %s", line)
		}
		bed.failing(5, replies, "question-replies failed: "+errDutyB.Error(), 1)
		// The stderr line each duty says is the one it always said.
		for name, label := range map[string]string{"disk-sweep": "disk sweep", "tick-timing": "tick timing",
			"question-replies": "question replies", "unit-boundary": "unit boundary", "seat-recovery-requests": "seat recovery requests",
			"notifications-pending": "notifications pending", "channel-pending": "channel pending", "cache-trim": "cache trim",
			"breach-stop-notice": "breach-stop notice not recorded"} {
			if got := dutyLine(name, errDutyA); got != label+": "+errDutyA.Error() {
				t.Fatalf("the %s duty says %q", name, got)
			}
		}
	})

	t.Run("five in a row stand once; a sixth adds nothing; a success clears", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		bed.quiet()
		bed.run(8, func(cycle int) dutyCycle {
			if cycle >= 3 && cycle <= 8 {
				return dutyCycle{replies: errDutyA}
			}
			return dutyCycle{}
		}, nil)
		for cycle := 4; cycle <= 7; cycle++ {
			bed.failing(cycle, replies, "question-replies failed: "+errDutyA.Error(), cycle-3)
			if bed.verdicts[cycle].ShouldAlert != bed.verdicts[2].ShouldAlert || bed.journal[cycle] != bed.journal[2] {
				t.Fatalf("cycle %d notified before the duty stood", cycle)
			}
		}
		for cycle := 8; cycle <= 9; cycle++ {
			if got := bed.role(cycle, replies); !got.Standing || got.ConsecutiveFailures != healthFailureLimit {
				t.Fatalf("cycle %d: %+v, want standing at five", cycle, got)
			}
			defects := bed.episodes(cycle, standing)
			if len(defects) != 1 || defects[0].ScopeID != string(replies) || defects[0].Cleared || bed.journal[cycle] != bed.journal[2]+1 {
				t.Fatalf("cycle %d: standing defects %+v and %d deliveries, want one open for %s delivered once", cycle, defects, bed.journal[cycle]-bed.journal[2], replies)
			}
		}
		if got := bed.role(10, replies); got.Status != HealthAlive {
			t.Fatalf("a success did not clear the role: %+v", got)
		}
		if defects := bed.episodes(10, standing); len(defects) != 1 || !defects[0].Cleared {
			t.Fatalf("a success did not close the standing defect: %+v", defects)
		}
	})

	t.Run("a flapping duty neither delays nor masks a steady one", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		bed.quiet()
		bed.run(6, func(cycle int) dutyCycle {
			if cycle < 3 || cycle > 7 {
				return dutyCycle{}
			}
			if cycle%2 == 1 {
				return dutyCycle{replies: errDutyA, channel: errDutyC}
			}
			return dutyCycle{replies: errDutyA, channel: errDutyD}
		}, nil)
		for cycle := 4; cycle <= 8; cycle++ {
			want := errDutyC
			if cycle%2 == 1 {
				want = errDutyD
			}
			bed.failing(cycle, dutyTestRole("channel-pending"), "channel-pending failed: 2 undelivered: "+want.Error(), 1)
			if got := bed.role(cycle, replies); got.Standing != (cycle == 8) || got.ConsecutiveFailures != cycle-3 {
				t.Fatalf("cycle %d: replies %+v, want failure %d and standing only at five", cycle, got, cycle-3)
			}
		}
	})

	t.Run("another role's open alert keeps its one notice", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t, claimQuotaHeld("held-h"), claimQuotaReady("ready-r", 1, "m1e"))
		bed.cfg.ClaimLaneState = claimQuotaLane(t, bed.root, nil)
		bed.run(4, func(cycle int) dutyCycle {
			if cycle == 3 {
				return dutyCycle{replies: errDutyA}
			}
			return dutyCycle{}
		}, nil)
		if got := bed.role(1, RoleClaimQuota); got.Status != HealthDead || !bed.verdicts[1].ShouldAlert {
			t.Fatalf("the blocked goal is not an alerting row: %+v", got)
		}
		open := bed.episodes(3, "")
		if len(open) != 1 || open[0].Cleared || bed.journal[3] == 0 {
			t.Fatalf("the blocked goal's alert: %+v, %d deliveries", open, bed.journal[3])
		}
		bed.failing(4, replies, "question-replies failed: "+errDutyA.Error(), 1)
		if bed.journal[4] != bed.journal[3] {
			t.Fatalf("one duty failure beside an open alert delivered %d more notices", bed.journal[4]-bed.journal[3])
		}
		if after := bed.episodes(4, ""); len(after) != 1 || after[0].EpisodeID != open[0].EpisodeID || after[0].Cleared {
			t.Fatalf("the open alert did not keep its episode: before %+v, after %+v", open, after)
		}
	})

	t.Run("a failed breach-stop notice keeps counting past passes with nothing to ask", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		stops := 0
		bed.cfg.BreachStop = func(id string, revision uint64) (string, error) {
			stops++
			stamp := bed.now.UTC().Format(time.RFC3339)
			return "stopped", goal.WriteStopBatch(bed.root, goal.StopBatch{
				StopID: "stop-held-h-r3-f1", GoalID: id, GoalRevision: revision, FenceEpoch: 1, CapabilityGeneration: 3,
				Machine: "m1e", ClaimEpoch: 1, Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchComplete,
				OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
			})
		}
		bed.quiet()
		// A claimed goal past its box on a configured channel, whose question
		// store refuses the stop's question on the one pass that asks.
		fenced := claimQuotaHeld("held-h")
		fenced.StopCapability.FenceEpoch = 1
		fenced.StopFence = &goal.StopFence{StopID: "stop-held-h-r3-f1", Revision: 3, Epoch: 1, CapabilityGeneration: 3,
			ClosedAt: "2026-08-23T01:02:00Z", Reason: goal.StopReasonElapsedLimit}
		bed.health.writeFile("plans/goals/held-h.md", goal.RenderFile(fenced))
		conf, err := os.ReadFile(filepath.Join(bed.root, "metasystem.conf"))
		if err != nil {
			t.Fatal(err)
		}
		bed.health.writeFile("metasystem.conf", append(conf, []byte("channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+t.TempDir()+"\n")...))
		seatGit(t, bed.root, "add", "plans/goals")
		seatGit(t, bed.root, "commit", "-qm", "a breached claim")
		seatGit(t, bed.root, "update-ref", goal.AcceptedRef, "HEAD")
		questions := filepath.Join(bed.root, "artifacts", "agents", "channel", "questions")
		if err := os.MkdirAll(questions, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(questions, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(questions, 0o755) })
		bed.run(5, func(int) dutyCycle { return dutyCycle{} }, func(cycle int) {
			if cycle == 3 {
				if err := os.Chmod(questions, 0o755); err != nil {
					t.Fatal(err)
				}
			}
		})
		notice := dutyTestRole("breach-stop-notice")
		reason := bed.role(3, notice).Reason
		if !strings.HasPrefix(reason, "breach-stop-notice failed: ") {
			t.Fatalf("the failed notice: %+v", bed.role(3, notice))
		}
		for cycle := 3; cycle <= 6; cycle++ {
			bed.failing(cycle, notice, reason, cycle-2)
		}
		if got := bed.role(7, notice); !got.Standing || got.Reason != reason {
			t.Fatalf("the failed notice did not stand at five: %+v", got)
		}
		if got := bed.duties()["breach-stop-notice"].Error; stops != 1 || "breach-stop-notice failed: "+got != reason {
			t.Fatalf("%d stops; the record holds %q", stops, got)
		}
	})

	t.Run("a failing cache trim is a row; with no trim there is no entry", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		bed.quiet()
		trimErr := errors.New("the cache lock is held")
		bed.run(2, func(cycle int) dutyCycle {
			if cycle == 3 {
				return dutyCycle{trim: trimErr}
			}
			return dutyCycle{}
		}, nil)
		bed.failing(4, dutyTestRole("cache-trim"), "cache-trim failed: "+trimErr.Error(), 1)

		none := newDutyBed(t)
		none.noTrim = true
		none.run(2, func(int) dutyCycle { return dutyCycle{} }, nil)
		if _, ok := none.duties()["cache-trim"]; ok {
			t.Fatal("a runner with no cache trim recorded one")
		}
		if got := none.role(2, dutyTestRole("cache-trim")); got.Status != HealthAlive {
			t.Fatalf("an absent cache trim: %+v", got)
		}
	})

	t.Run("a write that fails keeps the failure", func(t *testing.T) {
		t.Parallel()
		bed := newDutyBed(t)
		folder := filepath.Join(bed.root, "artifacts", "agents", "steward", "duties")
		t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
		bed.run(5, func(cycle int) dutyCycle {
			if cycle >= 3 {
				return dutyCycle{replies: errDutyA}
			}
			return dutyCycle{}
		}, func(cycle int) {
			switch cycle {
			case 2:
				if got, ok := bed.duties()["question-replies"]; !ok || got.Error != "" {
					t.Fatalf("the success record: %+v %t", got, ok)
				}
				if err := os.Chmod(folder, 0o555); err != nil {
					t.Fatal(err)
				}
			case 3:
				if got := bed.duties()["question-replies"]; got.Error != "" {
					t.Fatalf("the failed write replaced the record: %+v", got)
				}
			case 4:
				if err := os.Chmod(folder, 0o755); err != nil {
					t.Fatal(err)
				}
			}
		})
		bed.failing(4, replies, "question-replies failed: "+errDutyA.Error(), 1)
		if got := bed.duties()["question-replies"].Error; got != errDutyA.Error() {
			t.Fatalf("the next write carries %q, want %q", got, errDutyA)
		}
		unpersistedDuties.Lock()
		held := unpersistedDuties.byTop[bed.root]
		unpersistedDuties.Unlock()
		if len(held) != 0 {
			t.Fatalf("a successful write kept the outcomes in memory: %+v", held)
		}
	})
}

// TestTheHelmCountsItsOwnDuties takes the helm after full cycles, in the
// order a seat lives them: the helm observes the duties that run at the helm,
// files their standing defects, and leaves every other role as it was.
func TestTheHelmCountsItsOwnDuties(t *testing.T) {
	t.Parallel()
	replies, channel, pending := dutyTestRole("question-replies"), dutyTestRole("channel-pending"), dutyTestRole("notifications-pending")
	standing := PatternOwner("health-standing-red")
	errPending := errors.New("the outbox is locked")
	bed := newDutyBed(t)
	bed.quiet()

	// Full cycles 3 to 8: the channel fails on five cycles and stands on the
	// sixth tick; the notifications fail once.
	bed.run(6, func(cycle int) dutyCycle {
		switch {
		case cycle == 7:
			return dutyCycle{channel: errDutyC, pending: errPending}
		case cycle >= 3 && cycle <= 7:
			return dutyCycle{channel: errDutyC}
		}
		return dutyCycle{}
	}, nil)
	scoped := func(cycle int, role HealthRole) []AlertEpisode {
		var found []AlertEpisode
		for _, episode := range bed.episodes(cycle, standing) {
			if episode.ScopeID == string(role) {
				found = append(found, episode)
			}
		}
		return found
	}
	if got := bed.role(8, channel); !got.Standing {
		t.Fatalf("the channel did not stand on the sixth tick: %+v", got)
	}
	channelDefect := scoped(8, channel)
	if len(channelDefect) != 1 || channelDefect[0].Cleared {
		t.Fatalf("the channel's standing defect: %+v", channelDefect)
	}
	channelEvidence, err := json.Marshal(channelDefect[0].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	bed.failing(8, pending, "notifications-pending failed: "+errPending.Error(), 1)

	// Helm cycles 9 to 14: the replies fail on the first five.
	takeHelmFixture(t, bed.root)
	bed.run(6, func(cycle int) dutyCycle {
		if cycle >= 9 && cycle <= 13 {
			return dutyCycle{replies: errDutyA}
		}
		return dutyCycle{}
	}, nil)
	for cycle := 9; cycle <= 14; cycle++ {
		if !bed.records[cycle].Verdict.Partial {
			t.Fatalf("helm cycle %d: the record is not partial", cycle)
		}
		open := scoped(cycle, channel)
		if len(open) != 1 || open[0].EpisodeID != channelDefect[0].EpisodeID || open[0].Cleared {
			t.Fatalf("helm cycle %d: the channel's defect is %+v, want it open as before", cycle, open)
		}
		if evidence, err := json.Marshal(open[0].Evidence); err != nil || !bytes.Equal(evidence, channelEvidence) {
			t.Fatalf("helm cycle %d: the channel's evidence changed:\n%s\nwant\n%s", cycle, evidence, channelEvidence)
		}
		if got := bed.role(cycle, pending); got.ConsecutiveFailures != 1 {
			t.Fatalf("helm cycle %d: notifications-pending counts %d, want 1", cycle, got.ConsecutiveFailures)
		}
	}
	bed.failing(13, replies, "question-replies failed: "+errDutyA.Error(), 4)
	if got := bed.role(14, replies); !got.Standing || got.ConsecutiveFailures != healthFailureLimit {
		t.Fatalf("helm cycle 14: %+v, want the replies standing at five", got)
	}
	if defects := scoped(14, replies); len(defects) != 1 || defects[0].Cleared || bed.journal[14] != bed.journal[13]+1 {
		t.Fatalf("helm cycle 14: replies defects %+v and %d deliveries, want one open, delivered once", defects, bed.journal[14]-bed.journal[13])
	}

	// Released: the next full tick reads every role, and the replies' success
	// clears their defect.
	if _, err := helm.Remove(bed.root); err != nil {
		t.Fatal(err)
	}
	bed.run(1, func(int) dutyCycle { return dutyCycle{} }, nil)
	if verdict := bed.verdicts[15]; verdict.Partial || len(verdict.Roles) != len(bed.verdicts[2].Roles) {
		t.Fatalf("the tick after release: partial %t with %d roles, want a full %d", verdict.Partial, len(verdict.Roles), len(bed.verdicts[2].Roles))
	}
	if got := bed.role(15, replies); got.Status != HealthAlive {
		t.Fatalf("after the release: %+v, want the replies alive", got)
	}
	if defects := scoped(15, replies); len(defects) != 1 || !defects[0].Cleared {
		t.Fatalf("after the release the replies defect is %+v, want it closed", defects)
	}
}
