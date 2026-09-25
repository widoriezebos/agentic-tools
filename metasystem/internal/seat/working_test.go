package seat

// What a seat publishes it is doing, from fake job records and a fake
// consumption projection. Nothing here opens a repository and nothing reads
// the wall: the box is a function this file supplies, and every age is
// measured against fixtureClock.

import (
	"strings"
	"testing"
	"time"
)

// workingBox is the projection this package never makes for itself: one goal
// with numbers, one that carries no box at all, and one nobody could count.
func workingBox(goal string) *Box {
	switch goal {
	case "goal-a":
		attempts, attemptLimit := int64(3), int64(10)
		reserved, reservedLimit := int64(610), int64(720)
		return &Box{
			Attempts: &attempts, AttemptLimit: &attemptLimit,
			ReservedMinutes: &reserved, ReservedMinutesLimit: &reservedLimit,
		}
	case "goal-unknown":
		return &Box{Problem: "the claim episode timestamp is malformed (plans/goals/goal-unknown.md)"}
	default:
		return nil
	}
}

func runningJobs() JobSet {
	return JobSet{Records: []JobRecord{
		{Job: "job-root", Role: "implementer", Goal: "goal-a", Round: 1, Status: "completed",
			CreatedAt: "2026-09-24T08:00:00Z", StartedAt: "2026-09-24T08:00:00Z",
			EndedAt: "2026-09-24T08:40:00Z", CapMinutes: intOf(90)},
		{Job: "job-new", Parent: "job-root", Role: "implementer", Goal: "goal-a", Round: 2, Status: "running",
			CreatedAt: "2026-09-24T08:59:00Z", StartedAt: "2026-09-24T08:59:00Z", CapMinutes: intOf(120)},
	}}
}

func intOf(value int) *int { return &value }

func TestWorkingIsComposedFromTheNewestJobAndItsChain(t *testing.T) {
	t.Parallel()
	working, detail := ComposeWorking(runningJobs(), workingBox)
	if detail != "" || working == nil {
		t.Fatalf("working = %+v detail %q", working, detail)
	}
	if working.Goal != "goal-a" || working.Phase.Role != "implementer" || working.Phase.Round != 2 {
		t.Fatalf("phase = %+v", working.Phase)
	}
	// A build has no round limit of its own, so its round shows without a
	// denominator rather than against a number borrowed from somewhere else.
	if working.Phase.RoundLimit != nil {
		t.Fatalf("a build's roundLimit = %v, want null", *working.Phase.RoundLimit)
	}
	if working.Job.ID != "job-new" || working.Job.Status != "running" {
		t.Fatalf("job = %+v", working.Job)
	}
	if working.Job.CapMinutes == nil || *working.Job.CapMinutes != 120 {
		t.Fatalf("job capMinutes = %v", working.Job.CapMinutes)
	}
	// No recorded deadline, so the cap ends at the start plus the cap.
	if working.Job.CapEndsAt == nil || *working.Job.CapEndsAt != "2026-09-24T10:59:00Z" {
		t.Fatalf("job capEndsAt = %v", working.Job.CapEndsAt)
	}
	if len(working.Chain) != 2 || working.Chain[0].Job != "job-new" || working.Chain[1].Job != "job-root" {
		t.Fatalf("chain = %+v, want newest first", working.Chain)
	}
	if working.Chain[1].EndedAt == nil || *working.Chain[1].EndedAt != "2026-09-24T08:40:00Z" {
		t.Fatalf("the ended member's endedAt = %v", working.Chain[1].EndedAt)
	}
}

func TestARecordedCapDeadlineWinsOverTheStartPlusTheCap(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-one", Role: "implementer", Goal: "goal-a", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:00Z",
			CapMinutes: intOf(120), CapDeadline: "2026-09-24T10:30:00Z"},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil || working.Job.CapEndsAt == nil || *working.Job.CapEndsAt != "2026-09-24T10:30:00Z" {
		t.Fatalf("capEndsAt = %+v; dispatch enforces the recorded deadline where one exists", working)
	}
}

func TestACriticsRoundCountsAgainstTheLimitItsChainRootFroze(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-root", Role: "code-critic", Goal: "goal-a", Round: 1, Status: "completed",
			CreatedAt: "2026-09-24T08:00:00Z", ReviewRoundLimit: intOf(2)},
		{Job: "job-two", Parent: "job-root", Role: "code-critic", Goal: "goal-a", Round: 2, Status: "running",
			CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:00Z"},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil || working.Phase.RoundLimit == nil || *working.Phase.RoundLimit != 2 {
		t.Fatalf("roundLimit = %+v, want the root's frozen 2", working)
	}
}

func TestAReservationThatHasNotStartedNamesNoStartAndNoCapEnd(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-held", Role: "code-critic", Goal: "goal-a", Round: 1, Status: "pending-setup",
			CreatedAt: "2026-09-24T09:30:00Z", CapMinutes: intOf(60)},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil || working.Job.StartedAt != nil || working.Job.CapEndsAt != nil {
		t.Fatalf("an unstarted job = %+v; a cap that has not begun ends at no instant", working)
	}
	if PhaseWords(working, fixtureClock) != "code-critic round 1 on goal-a · pending-setup, cap 60 min" {
		t.Fatalf("phase words = %q", PhaseWords(working, fixtureClock))
	}
}

func TestAPendingJobIsNeverCountedInMinutesHoweverItIsStamped(t *testing.T) {
	t.Parallel()
	// Dispatch creates a build record `pending` with a startedAt stamped at
	// the same instant (internal/dispatch/build.go:624, 665). Counting from
	// that stamp said a job had been running for forty minutes while the
	// block beside it said pending.
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-new", Role: "implementer", Goal: "goal-a", Round: 1, Status: "pending",
			CreatedAt: "2026-09-24T08:59:00Z", StartedAt: "2026-09-24T08:59:00Z", CapMinutes: intOf(120)},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil {
		t.Fatal("no working")
	}
	if words := JobWords(working.Job, fixtureClock); words != "pending, cap 120 min" {
		t.Fatalf("a pending job = %q", words)
	}
	// The stamp and the cap's own end are untouched: they are what the block
	// shows as the start and the bound.
	if working.Job.StartedAt == nil || *working.Job.StartedAt != "2026-09-24T08:59:00Z" {
		t.Fatalf("startedAt = %v", working.Job.StartedAt)
	}
	if working.Job.CapEndsAt == nil || *working.Job.CapEndsAt != "2026-09-24T10:59:00Z" {
		t.Fatalf("capEndsAt = %v", working.Job.CapEndsAt)
	}
}

func TestAJobThatHasEndedSaysHowItEnded(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"completed", "failed", "cancelled", "timeout"} {
		job := WorkingJob{
			ID: "job-done", Role: "implementer", Status: status,
			StartedAt: presenceStamp("2026-09-24T08:00:00Z"), CapMinutes: intOf(90),
		}
		if words := JobWords(job, fixtureClock); words != status {
			t.Fatalf("a %s job = %q", status, words)
		}
	}
}

func TestARunningJobWithNoStartIsStillRunningRatherThanCounted(t *testing.T) {
	t.Parallel()
	job := WorkingJob{ID: "job-odd", Role: "implementer", Status: "running", CapMinutes: intOf(45)}
	if words := JobWords(job, fixtureClock); words != "running, cap 45 min" {
		t.Fatalf("words = %q", words)
	}
}

func TestOnlyTheEightNewestChainMembersTravel(t *testing.T) {
	t.Parallel()
	records := []JobRecord{{Job: "job-00", Role: "implementer", Goal: "goal-a", Round: 1,
		Status: "completed", CreatedAt: "2026-09-24T00:00:00Z"}}
	for index := 1; index < 12; index++ {
		records = append(records, JobRecord{
			Job: "job-" + twoDigits(index), Parent: "job-" + twoDigits(index-1),
			Role: "implementer", Goal: "goal-a", Round: index, Status: "completed",
			CreatedAt: "2026-09-24T" + twoDigits(index) + ":00:00Z",
		})
	}
	records[len(records)-1].Status = "running"
	records[len(records)-1].StartedAt = "2026-09-24T11:00:00Z"
	working, _ := ComposeWorking(JobSet{Records: records}, workingBox)
	if working == nil || len(working.Chain) != WorkingChainMembers {
		t.Fatalf("chain length = %d, want %d", len(working.Chain), WorkingChainMembers)
	}
	if working.Chain[0].Job != "job-11" || working.Chain[7].Job != "job-04" {
		t.Fatalf("chain = %+v, want the eight newest", working.Chain)
	}
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

func TestAGoalWithNoBoxCarriesNullAndSaysSo(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-one", Role: "implementer", Goal: "goal-boxless", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:00Z"},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil || working.Box != nil {
		t.Fatalf("box = %+v, want null for a goal that carries none", working)
	}
	if BoxWords(nil) != "no box on this goal" {
		t.Fatalf("boxless words = %q", BoxWords(nil))
	}
}

func TestAProjectionThatCouldNotBeMadeCarriesItsProblemAndNoZeros(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-one", Role: "implementer", Goal: "goal-unknown", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:00Z"},
	}}
	working, _ := ComposeWorking(jobs, workingBox)
	if working == nil || working.Box == nil {
		t.Fatalf("working = %+v", working)
	}
	box := *working.Box
	if box.Attempts != nil || box.AttemptLimit != nil || box.ReservedMinutes != nil || box.ReservedMinutesLimit != nil {
		t.Fatalf("an unknown box carries numbers: %+v; zeros would say the goal has spent nothing", box)
	}
	if !strings.Contains(BoxWords(&box), "the claim episode timestamp is malformed") {
		t.Fatalf("box words = %q, want the projection's own reason", BoxWords(&box))
	}
}

func TestAMalformedJobLeavesWorkingNullWithANamedDetail(t *testing.T) {
	t.Parallel()
	jobs := JobSet{
		Records:    runningJobs().Records,
		Unreadable: []string{"artifacts/agents/jobs/torn.json: the job record is not readable JSON"},
	}
	working, detail := ComposeWorking(jobs, workingBox)
	if working != nil {
		t.Fatalf("working = %+v, want null while a record could not be read", working)
	}
	if !strings.Contains(detail, "torn.json") {
		t.Fatalf("detail = %q, want the unreadable record named", detail)
	}
	inFlight, problem := WorkingInFlight(jobs, workingBox)
	if inFlight != nil || !strings.Contains(problem, "torn.json") {
		t.Fatalf("in flight = %+v problem %q", inFlight, problem)
	}
}

func TestEveryJobInFlightTravelsForTheSeatThatCanReadTheRecords(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-a", Role: "implementer", Goal: "goal-a", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:20:00Z", StartedAt: "2026-09-24T09:20:00Z", CapMinutes: intOf(45)},
		{Job: "job-b", Role: "code-critic", Goal: "goal-boxless", Round: 1, Status: "pending",
			CreatedAt: "2026-09-24T09:05:00Z"},
		{Job: "job-done", Role: "implementer", Goal: "goal-a", Round: 1, Status: "completed",
			CreatedAt: "2026-09-24T07:00:00Z"},
	}}
	inFlight, problem := WorkingInFlight(jobs, workingBox)
	if problem != "" || len(inFlight) != 2 {
		t.Fatalf("in flight = %+v problem %q; a terminal job is not in flight", inFlight, problem)
	}
	if inFlight[0].Job.ID != "job-a" || inFlight[1].Job.ID != "job-b" {
		t.Fatalf("in flight order = %q, %q; newest first", inFlight[0].Job.ID, inFlight[1].Job.ID)
	}
}

func TestTheProjectionIsAskedOncePerGoalHoweverManyJobsNameIt(t *testing.T) {
	t.Parallel()
	asked := map[string]int{}
	counted := func(goal string) *Box {
		asked[goal]++
		return workingBox(goal)
	}
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-a", Role: "implementer", Goal: "goal-a", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:20:00Z", StartedAt: "2026-09-24T09:20:00Z"},
		{Job: "job-b", Role: "code-critic", Goal: "goal-a", Round: 1, Status: "running",
			CreatedAt: "2026-09-24T09:10:00Z", StartedAt: "2026-09-24T09:10:00Z"},
	}}
	if _, problem := WorkingInFlight(jobs, counted); problem != "" {
		t.Fatalf("problem = %q", problem)
	}
	if asked["goal-a"] != 1 {
		t.Fatalf("the projection was asked %d times for one goal", asked["goal-a"])
	}
}

/* ---------------------------------------------------------------- words -- */

func TestThePhaseSentenceSaysTheRoundTheGoalAndTheCap(t *testing.T) {
	t.Parallel()
	working, _ := ComposeWorking(runningJobs(), workingBox)
	// The clock is injected: 09:40 against a start of 08:59 is 41 minutes.
	words := PhaseWords(working, fixtureClock)
	if words != "implementer round 2 on goal-a · running 41 min, cap 120 min" {
		t.Fatalf("phase words = %q", words)
	}
	if PhaseWords(nil, fixtureClock) != "idle" {
		t.Fatalf("idle words = %q", PhaseWords(nil, fixtureClock))
	}
}

func TestTheOnlyForwardLookingWordsNameTheCapAndRoundUp(t *testing.T) {
	t.Parallel()
	working, _ := ComposeWorking(runningJobs(), workingBox)
	// 09:40 to a cap that ends at 10:59 is 79 minutes exactly; a second more
	// elapsed is still 79, because a bound rounds up.
	if words := CapWords(working.Job, fixtureClock); words != "its cap ends in 79 min" {
		t.Fatalf("cap words = %q", words)
	}
	past := fixtureClock.Add(200 * time.Minute)
	if words := CapWords(working.Job, past); !strings.HasPrefix(words, "its cap ended ") {
		t.Fatalf("a passed cap = %q", words)
	}
}

func TestTheBoxSaysWhatIsSpentAndWhatIsLeft(t *testing.T) {
	t.Parallel()
	working, _ := ComposeWorking(runningJobs(), workingBox)
	words := BoxWords(working.Box)
	if words != "attempt 3 of 10 · 610 of 720 min reserved · 7 attempts left" {
		t.Fatalf("box words = %q", words)
	}
}

func TestNoDurationOnThisSurfaceIsEverPrintedInDays(t *testing.T) {
	t.Parallel()
	// A budget's day is eight hours in this kit, so a duration printed in
	// days would read as two different lengths depending on who read it.
	for _, minutes := range []int64{1, 59, 60, 481, 2880} {
		words := MinutesWords(minutes)
		if !strings.HasSuffix(words, " min") || strings.Contains(words, "d") {
			t.Fatalf("minutes words = %q", words)
		}
	}
}

/* ---------------------------------------------------------- the reader -- */

func TestTheReaderAcceptsARecordWithWorkingAndOneWithout(t *testing.T) {
	t.Parallel()
	record, _, err := Compose("m1e", fixtureRunner(), runningJobs(), workingBox, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := ParseRecord(encoded)
	if err != nil {
		t.Fatalf("a record with working refused: %v", err)
	}
	if read.Working == nil || read.Working.Job.ID != "job-new" || read.Working.Box == nil {
		t.Fatalf("parsed working = %+v", read.Working)
	}
	if read.Working.Box.Attempts == nil || *read.Working.Box.Attempts != 3 {
		t.Fatalf("parsed box = %+v", read.Working.Box)
	}
	// Every record written before this key has none, and a reader that
	// refused those would read a live machine as dead.
	without := strings.Replace(string(encoded), `"working"`, `"workingOnce"`, 1)
	older, err := ParseRecord([]byte(without))
	if err != nil {
		t.Fatalf("a record without working refused: %v", err)
	}
	if older.Working != nil {
		t.Fatalf("working = %+v, want null where the key is absent", older.Working)
	}
}

func TestAnExplicitNullWorkingIsAMachineRunningNothing(t *testing.T) {
	t.Parallel()
	record, _, err := Compose("m1e", fixtureRunner(), JobSet{}, workingBox, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"working": null`) {
		t.Fatalf("an idle record = %s", encoded)
	}
	read, err := ParseRecord(encoded)
	if err != nil || read.Working != nil {
		t.Fatalf("parsed = %+v err %v", read.Working, err)
	}
}

func TestAWorkingThatNamesNoJobIsAMalformedRecord(t *testing.T) {
	t.Parallel()
	torn := `{"presenceSchema":1,"machine":"m1e","repoIdentity":"r","generation":4,"engine":"e",
		"armedLineage":"l","tickSeconds":600,"chain":null,"tickAt":"2026-09-24T09:40:00Z",
		"working":{"goal":"goal-a","phase":{"role":"implementer","round":2,"roundLimit":null},
		"job":{"id":"","role":"implementer","status":"running","startedAt":null,
		"capMinutes":null,"capEndsAt":null},"box":null,"chain":[]}}`
	if _, err := ParseRecord([]byte(torn)); err == nil ||
		!strings.Contains(err.Error(), "SEAT_PRESENCE_MALFORMED") {
		t.Fatalf("a working with no job id = %v", err)
	}
}

/* ------------------------------------------------------- the seat verb -- */

func TestSeatFleetTextSaysThePhaseSentence(t *testing.T) {
	t.Parallel()
	record, _, err := Compose("m1e", fixtureRunner(), runningJobs(), workingBox, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	report := Report{Machines: []MachineStanding{{Machine: "m1e", Standing: Reachable, Record: &record}}}
	report.SetNow(fixtureClock)
	if !strings.Contains(report.Text(), "implementer round 2 on goal-a · running 41 min, cap 120 min") {
		t.Fatalf("seat fleet text = %q", report.Text())
	}
}

func TestSeatFleetTextFallsBackToTheChainForAMachineWithoutWorking(t *testing.T) {
	t.Parallel()
	// A fleet is not one build: a machine still publishing the chain alone is
	// answered from the chain rather than reported idle.
	started := "2026-09-24T09:30:05Z"
	record := Record{
		PresenceSchema: RecordSchema, Machine: "m2a", RepoIdentity: "r", Generation: 4,
		Engine: "e", ArmedLineage: NoLease, TickSeconds: 600, TickAt: FormatTime(fixtureClock),
		Chain: &Chain{Root: "r-1", Job: "j-1", Role: "critic", Round: 1, Goal: "goal-b", StartedAt: &started},
	}
	report := Report{Machines: []MachineStanding{{Machine: "m2a", Standing: Reachable, Record: &record}}}
	report.SetNow(fixtureClock)
	if !strings.Contains(report.Text(), "running critic round 1 on goal-b") {
		t.Fatalf("seat fleet text = %q", report.Text())
	}
}
