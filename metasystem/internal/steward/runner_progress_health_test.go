package steward

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"golang.org/x/sys/unix"
)

func progressFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expectProgress(t *testing.T, root, operation, state, reason string, done int) RunnerProgress {
	t.Helper()
	r := readProgress(t, root)
	if r.Operation != operation || r.State != state || r.Reason != reason || r.Done != done {
		t.Fatalf("progress=%+v, want %s %s %s done=%d", r, operation, state, reason, done)
	}
	return r
}

type progressCensus struct{ check func() }

func (c progressCensus) Workers(string) (Workers, error) {
	c.check()
	return Workers{CensusComplete: true}, nil
}

func TestHealthPublishesEachRoleAndIterationUnderItsLocks(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	progressFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=none\n")
	progressFile(t, filepath.Join(root, "memory/receipts.log"), "")
	now := claimQuotaClock
	p := itemsProgress(t, root, &now)
	_, probe := progressProbe(now)
	lanes, unitCalls, doneAtAsk := 0, 0, 0
	var laneDone []int
	cfg := TickConfig{Now: now, Prober: probe, Progress: p, WorkStateRoot: t.TempDir(), AskHome: t.TempDir()}
	cfg.ClaimLaneState = func(string, string, string) (string, error) {
		held, err := TryAcquireArbitration(root)
		if held != nil {
			held.Release()
		}
		if !errors.Is(err, ErrArbitrationHeld) {
			t.Fatalf("arbitration during health: %v", err)
		}
		f, err := os.OpenFile(healthLockPath(root), os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		}
		f.Close()
		if !errors.Is(err, unix.EWOULDBLOCK) {
			t.Fatalf("health lock during lane read: %v", err)
		}
		r := readProgress(t, root)
		if r.Operation != "tick:health" || r.State != "working" {
			t.Fatalf("lane read: %+v", r)
		}
		laneDone = append(laneDone, r.Done)
		lanes++
		now = now.Add(time.Second)
		return "", nil
	}
	cfg.Units = func(string, string) ([]UnitStage, error) {
		unitCalls++
		r := readProgress(t, root)
		if r.Operation != "tick:health" || r.State != "working" {
			t.Fatalf("ask: %+v", r)
		}
		doneAtAsk = r.Done
		return nil, nil
	}
	baseline, err := RunTick(root, cfg, fakeCensus{})
	if err != nil {
		t.Fatal(err)
	}
	// One spend item, the preceding twelve applicable roles, three ledger
	// stages and the claimed goal's budget precede the first quota query.
	if !slices.Equal(laneDone, []int{17, 18, 19}) || lanes != 3 || unitCalls != 1 {
		t.Fatalf("lane done=%v reads=%d asks=%d", laneDone, lanes, unitCalls)
	}
	// The empty machine report is still a completed read, as is the checkout
	// report. The bed has no sessions, jobs, proof attempts, runs or runtimes.
	// The quota also examines the unranked ready goal, without a lane read.
	want := len(baseline.Health.Roles) + 3 + 1 + 4 + 2
	if doneAtAsk != want {
		t.Fatalf("ask done=%d, want %d (roles=%d)", doneAtAsk, want, len(baseline.Health.Roles))
	}
	for i := 0; i < 2; i++ {
		progressFile(t, filepath.Join(root, "artifacts/agents/jobs", fmt.Sprintf("terminal-%d.json", i)), `{"status":"completed"}`)
		progressFile(t, filepath.Join(root, "artifacts/agents/proof-runs/attempts", fmt.Sprintf("terminal-%d.json", i)), `{"terminal":{"result":"success"}}`)
		store := &run.Store{Root: root, Now: func() time.Time { return now }}
		if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: fmt.Sprintf("ordinary-%d", i), Kind: "suite", Display: "ordinary validation", Log: "artifacts/ordinary.log"}); err != nil {
			t.Fatal(err)
		}
	}
	lanes, unitCalls, laneDone = 0, 0, nil
	withRecords, err := RunTick(root, cfg, fakeCensus{})
	if err != nil {
		t.Fatal(err)
	}
	if lanes != 3 || unitCalls != 1 || doneAtAsk != want+10 {
		t.Fatalf("extra records: lanes=%d asks=%d done=%d want=%d", lanes, unitCalls, doneAtAsk, want+10)
	}
	for _, role := range withRecords.Health.Roles {
		if role.Role == RoleNonterminalJobs || role.Role == RoleProofAttempts || role.Role == RoleGovernedObligations {
			index := slices.IndexFunc(baseline.Health.Roles, func(before RoleVerdict) bool { return before.Role == role.Role })
			if index < 0 {
				t.Fatalf("missing baseline role %s", role.Role)
			}
			before := baseline.Health.Roles[index]
			if before.Status != role.Status || before.Reason != role.Reason {
				t.Fatalf("terminal records changed verdict: before=%+v after=%+v", before, role)
			}
		}
	}
}

func TestEachLoopingRoleCountsAfterItsWork(t *testing.T) {
	t.Parallel()
	t.Run("jobs, attempts and session announcements", func(t *testing.T) {
		for _, kind := range []string{"jobs", "attempts", "mains"} {
			t.Run(kind, func(t *testing.T) {
				root := t.TempDir()
				now := claimQuotaClock
				count, probes := 0, 0
				item := func() { count++; now = now.Add(time.Second) }
				dir := map[string]string{"jobs": "jobs", "attempts": "proof-runs/attempts", "mains": "mains"}[kind]
				for i := 0; i < 2; i++ {
					body := fmt.Sprintf(`{"pid":%d,"pidStartedAt":100,"status":"running","sessionId":"s-%d"}`, 501+i, i)
					if kind == "attempts" {
						body = fmt.Sprintf(`{"launcher":{"pid":%d,"pidStartedAt":100}}`, 501+i)
					}
					progressFile(t, filepath.Join(root, "artifacts/agents", dir, fmt.Sprintf("%d.json", i)), body)
				}
				probe := censusSeamProbe(func(pid int64) (identity.Exact, identity.Liveness, error) {
					want := probes
					if kind == "attempts" {
						want += 2
					} // both file loads already returned
					if count != want {
						t.Fatalf("%s probe %d saw %d completed items, want %d", kind, probes, count, want)
					}
					probes++
					now = now.Add(time.Second)
					state := identity.Dead
					if kind == "mains" && probes == 2 {
						state = identity.Alive
					}
					return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, state, nil
				})
				switch kind {
				case "jobs":
					checkNonterminalJobs(root, probe, item)
				case "attempts":
					checkProofAttempts(root, probe, item)
				case "mains":
					checkSessionMain(root, probe, item)
				}
				want := 2
				if kind == "attempts" {
					want = 4
				}
				if count != want || probes != 2 {
					t.Fatalf("%s count=%d probes=%d", kind, count, probes)
				}
			})
		}
	})
	t.Run("capabilities", func(t *testing.T) {
		root := t.TempDir()
		now := claimQuotaClock
		count, calls := 0, 0
		progressFile(t, filepath.Join(root, "metasystem.conf"), "metasystem.runtimes=claude,codex\n")
		_, _ = capabilitySnapshotStatus(root, root, now, func(string) (string, error) {
			if count != calls {
				t.Fatalf("capability %d counted early: %d", calls, count)
			}
			calls++
			now = now.Add(time.Second)
			return "", errors.New("not installed")
		}, func() { count++ })
		if count != 2 || calls != 2 {
			t.Fatalf("capabilities count=%d calls=%d", count, calls)
		}
	})
	t.Run("budgets across machines", func(t *testing.T) {
		a, b := claimQuotaHeld("a"), claimQuotaHeld("b")
		b.Claimed.Machine = "other"
		b.StopCapability.Machine = "other"
		root := claimQuotaTickBed(t, nil, a, b)
		ep, err := goal.ResolveEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := goal.Project(ep, false, claimQuotaClock)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		checkClaimedGoalBudgetsFromProjection(root, claimQuotaClock, projection, true, nil, nil, func() { count++ })
		if count != 2 {
			t.Fatalf("budgets=%d, want both machines", count)
		}
	})
	t.Run("delivery, governed and disk records", func(t *testing.T) {
		root, home := t.TempDir(), t.TempDir()
		count := 0
		item := func() { count++ }
		for i := 0; i < 2; i++ {
			progressFile(t, filepath.Join(root, "artifacts/agents/jobs", fmt.Sprintf("%d.json", i)), `{"status":"completed"}`)
			store := &run.Store{Root: root, Now: func() time.Time { return claimQuotaClock }}
			if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: fmt.Sprintf("r-%d", i), Kind: "suite", Display: "ordinary validation", Log: "artifacts/ordinary.log"}); err != nil {
				t.Fatal(err)
			}
		}
		readDeliveryJobs(root, map[string]*claimedDeliveryGoal{}, item)
		if count != 2 {
			t.Fatalf("delivery=%d", count)
		}
		checkGovernedObligations(root, item)
		if count != 4 {
			t.Fatalf("governed=%d", count)
		}
		for _, path := range []string{diskstore.CheckoutReportPath(root), diskstore.MachineReportPath(home)} {
			progressFile(t, path, fmt.Sprintf(`{"schema":%q,"kind":"checkout","name":%q,"health":{"status":"ok"}}`, diskstore.ReportSchema, root))
		}
		checkDiskAt(root, home, item)
		if count != 6 {
			t.Fatalf("disk=%d", count)
		}
		progressFile(t, diskstore.CheckoutReportPath(root), "{torn")
		checkDiskAt(root, home, item)
		if count != 7 {
			t.Fatalf("failed disk read=%d", count)
		}
	})
}

func TestTheTickNamesItsPhasesAndWaits(t *testing.T) {
	t.Parallel()
	root := claimQuotaTickBed(t, nil, claimQuotaHeld("held-h"))
	now := claimQuotaClock
	self, probe := progressProbe(now)
	var writes []RunnerProgress
	p := newProgress(root, self, 1, func() time.Time { return now }, recordProgress(t, &writes))
	p.Begin("tick")
	projections, reports, censuses := 0, 0, 0
	cfg := TickConfig{Now: now, Prober: probe, Progress: p}
	cfg.GoalProjection = func(root string, at time.Time) (goal.Projection, error) {
		r := expectProgress(t, root, "tick:decide", "waiting", "external", 0)
		if r.Deadline != nil {
			t.Fatalf("fresh read acquired a deadline: %+v", r)
		}
		projections++
		now = now.Add(time.Second)
		ep, err := goal.ResolveEndpoint(root)
		if err != nil {
			return goal.Projection{}, err
		}
		return goal.Project(ep, false, at)
	}
	cfg.Patterns = func(string, time.Time) error {
		expectProgress(t, root, "tick:reports", "working", "", 0)
		reports++
		return nil
	}
	census := progressCensus{func() { expectProgress(t, root, "tick:decide", "working", "", 1); censuses++ }}
	if _, err := RunTick(root, cfg, census); err != nil {
		t.Fatal(err)
	}
	if projections != 1 || reports != 1 || censuses != 1 {
		t.Fatalf("projections=%d reports=%d censuses=%d", projections, reports, censuses)
	}
	var phases []string
	for _, r := range writes {
		if len(phases) == 0 || phases[len(phases)-1] != r.Operation {
			phases = append(phases, r.Operation)
		}
	}
	want := []string{"tick", "tick:provider-expiry", "tick:breach-stops", "tick:spent-box-question", "tick:governed-obligations", "tick:direct-validation", "tick:ruling-reviews", "tick:counselor-brief", "tick:reap", "tick:ledger-attention", "tick:seat-presence", "tick:evidence", "tick:decide", "tick:narrator-digest", "tick:ledger-notices", "tick:save-evidence", "tick:narrate", "tick:health", "tick:complete", "tick:reports"}
	if !slices.Equal(phases, want) {
		t.Fatalf("phases=%v", phases)
	}
	t.Run("direct validation link", func(t *testing.T) {
		root := t.TempDir()
		now := claimQuotaClock
		self, probe := progressProbe(now)
		var writes []RunnerProgress
		state := newValidationWindow()
		state.Observations = []validationWindowObservation{{RunID: "failed", Missing: []string{cadenceCatchGroups[0]}}}
		if err := saveValidationWindow(root, state); err != nil {
			t.Fatal(err)
		}
		p := newProgress(root, self, 1, func() time.Time { return now }, recordProgress(t, &writes))
		p.Begin("tick")
		if _, err := RunTick(root, TickConfig{Now: now, Prober: probe, Progress: p}, fakeCensus{}); err != nil {
			t.Fatal(err)
		}
		var link []RunnerProgress
		for _, r := range writes {
			if r.Operation == "tick:direct-validation" {
				link = append(link, r)
			}
		}
		if len(link) != 4 || link[1].State != "waiting" || link[1].Reason != "external" || link[1].Deadline != nil || link[2].State != "working" || link[3].Done != 1 {
			t.Fatalf("validation link=%+v", link)
		}
	})
	t.Run("helm", func(t *testing.T) {
		takeHelmFixture(t, root)
		if _, err := RunTick(root, TickConfig{Now: now, Prober: probe, Progress: p}, fakeCensus{}); err != nil {
			t.Fatal(err)
		}
		if r := readProgress(t, root); r.Operation != "tick:helm" {
			t.Fatalf("helm=%+v", r)
		}
	})
}

// The arbitration hook is package state, so this witness runs serially.
func TestTheTickPublishesItsArbitrationWait(t *testing.T) {
	root := t.TempDir()
	now := claimQuotaClock
	p := itemsProgress(t, root, &now)
	p.Begin("tick")
	_, probe := progressProbe(now)
	held, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	previous := beforeArbitrationWait
	defer func() { beforeArbitrationWait = previous }()
	seen := false
	beforeArbitrationWait = func() {
		r := readProgress(t, root)
		if r.Operation != "tick" || r.State != "waiting" || r.Reason != "arbitration" || r.Done != 0 {
			t.Errorf("queued tick: %+v", r)
		}
		if r.Deadline != nil {
			t.Error("arbitration has a deadline")
		}
		seen = true
		held.Release()
	}
	result := make(chan error, 1)
	go func() {
		_, err := RunTick(root, TickConfig{Now: now, Prober: probe, Progress: p}, fakeCensus{})
		result <- err
	}()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if !seen {
		t.Fatal("arbitration did not queue")
	}
}

func TestALedgerStageIsPublished(t *testing.T) {
	t.Parallel()
	bed := newAttentionPolicyBed(t)
	bed.run(bed.now, attentionBaselineCalls())
	bed.move("next", attentionWorld(approvedAttentionGoal("next-goal")), "base")
	bed.stageTime["next"] = bed.now
	f := newTickContinuationFixture(t, bed, false)
	now := bed.now
	self, _ := progressProbe(now)
	var writes []RunnerProgress
	f.cfg.Progress = newProgress(bed.root, self, 1, func() time.Time { return now }, recordProgress(t, &writes))
	repo := f.dependencies.ledgerRepository
	capture, advance := repo.CaptureTipBounded, repo.AdvanceAccepted
	repo.CaptureTipBounded = func(ep goal.Endpoint, budget time.Duration) (goal.BoundedCapture, error) {
		r := expectProgress(t, bed.root, "tick:ledger-attention", "waiting", "external", 0)
		if r.Deadline == nil || !r.Deadline.Equal(now.Add(ledgerAttentionFetchBudget)) {
			t.Fatalf("fetch deadline=%+v", r)
		}
		now = now.Add(time.Second)
		return capture(ep, budget)
	}
	repo.AdvanceAccepted = func(root, tip string) error {
		expectProgress(t, root, "tick:ledger-attention", "working", "", 2)
		now = now.Add(time.Second)
		return advance(root, tip)
	}
	if _, completed, err := f.run(t); err != nil || !completed {
		t.Fatalf("completed=%t err=%v", completed, err)
	}
	var counts []int
	for _, r := range writes {
		if r.Operation == "tick:ledger-attention" && r.State == "working" {
			counts = append(counts, r.Done)
		}
	}
	if !slices.Equal(counts, []int{0, 0, 1, 2, 3, 4}) {
		t.Fatalf("ledger stages=%v", counts)
	}
}

func TestAPersonsHealthPublishesNothing(t *testing.T) {
	t.Parallel()
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			root := claimQuotaTickBed(t, nil)
			now := claimQuotaClock
			_, probe := progressProbe(now)
			var before []byte
			if exists {
				p := itemsProgress(t, root, &now)
				p.Begin("resident")
				before, _ = os.ReadFile(runnerProgressPath(root))
			}
			_ = PreviewInstalledHealth(root, root, now, probe)
			if _, err := RunTick(root, TickConfig{Now: now, Prober: probe}, fakeCensus{}); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(runnerProgressPath(root))
			if exists {
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("manual command changed progress: %v", err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("manual command created progress: %v", err)
			}
		})
	}
}

func TestTheSituationPublishesItsLedgerStages(t *testing.T) {
	t.Parallel()
	root := situationQuotaBed(t)
	now := claimQuotaClock
	p := itemsProgress(t, root, &now)
	p.Begin("situation")
	projections, lanes := 0, 0
	src := withEmptyHost(t, SituationSources{Now: func() time.Time { return now }, Progress: p, ClaimLaneState: func(string, string, string) (string, error) {
		// Machine and lane sources, three ledger stages, ledger source and
		// asks source have completed; the quota source is still in progress.
		expectProgress(t, root, "situation", "working", "", 7+lanes)
		lanes++
		return "", nil
	}})
	src.Project = func(ep goal.Endpoint, fetch bool, at time.Time) (goal.Projection, error) {
		r := expectProgress(t, root, "situation", "waiting", "external", 4)
		if r.Deadline != nil {
			t.Fatalf("projection deadline=%+v", r)
		}
		projections++
		return goal.ProjectWithDeadline(ep, fetch, at, neverProjectionDeadline)
	}
	s := readSituation(t, root, src)
	if projections != 1 || lanes != 3 || len(s.Quota) != 1 {
		t.Fatalf("projections=%d lanes=%d quota=%v", projections, lanes, s.Quota)
	}
	// Eleven sources, three ledger stages, four ready goals and one facts item.
	if r := readProgress(t, root); r.Done != 19 {
		t.Fatalf("situation=%+v", r)
	}
}
