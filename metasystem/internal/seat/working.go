package seat

// What a seat is doing, composed once per tick beside the chain and published
// on the same record.
//
// The chain says which job is newest. This says what that job is a phase OF:
// the goal, the round and the limit it counts against, the cap the job
// reserved and when that cap ends, the goal's box, and the members of the
// chain around it. Every one of them is a name, a number, a status or a time
// the records already carry.
//
// Three rules hold this file, and each of them is something the kit already
// decided that this composer must not decide again.
//
// Nothing here recomputes consumed minutes. Settlement is dispatch's, it
// rounds up, floors at a minute, clamps at the cap, and for a terminal job
// depends on process identity and ownership proof; it is unexported. So a
// chain member carries its cap and its two instants and no charge.
//
// Nothing here computes elapsed time against a budget either. The projection
// this reads counts attempts and reserved job minutes and no elapsed clock —
// the one that does elapsed is the authority lens, which is not this. Where
// that projection cannot answer, the box carries its own problem and no
// numbers, because a zero would read as a goal that has spent nothing.
//
// A field the records cannot supply is null. Nothing here guesses one.
//
// The design is plans/designs/user-interface/g1-s45-what-a-seat-is-doing.md.

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// WorkingChainMembers bounds the chain one record carries. A record is read
// by every seat on every host, so its size is held by a count of entries
// rather than left to however long a chain has grown.
const WorkingChainMembers = 8

// Phase is what the newest job is a phase of: the role, the round it is on,
// and the limit that round counts against.
//
// RoundLimit is null for a build, which shows its round without a
// denominator: a build has no round limit of its own, and the goal's attempt
// limit covers other work too. For a critic it is the value dispatch froze on
// the chain root's own reviewRoundLimit, which is the chain's effective limit
// (internal/dispatch/build.go:683, read back at finding_register.go:266).
type Phase struct {
	Role       string `json:"role"`
	Round      int    `json:"round"`
	RoundLimit *int   `json:"roundLimit"`
}

// WorkingJob is the job in hand: what it is, where it stands, when it began,
// the minutes it reserved and the instant that reservation ends.
//
// CapEndsAt is the recorded enforcement deadline where the record carries
// one, and otherwise the start plus the cap — the two-branch rule the reap
// reaches its own verdict by (internal/dispatch/reapfacts.go:137). It is null
// while the job has neither, because a cap that has not begun to run ends at
// no instant anyone can name.
type WorkingJob struct {
	ID         string  `json:"id"`
	Role       string  `json:"role"`
	Status     string  `json:"status"`
	StartedAt  *string `json:"startedAt"`
	CapMinutes *int    `json:"capMinutes"`
	CapEndsAt  *string `json:"capEndsAt"`
}

// Box is the goal's consumption against its limits, as dispatch's own
// projection counts it: build attempts, proof attempts and governed runs
// alike, and open jobs counted at their full cap.
//
// Every number is nullable and Problem is why. A projection that could not be
// made carries its own reason and no figures: an all-zero box would say a
// goal has spent nothing, which is the one thing an unknown projection does
// not know.
type Box struct {
	Attempts             *int64 `json:"attempts"`
	AttemptLimit         *int64 `json:"attemptLimit"`
	ReservedMinutes      *int64 `json:"reservedMinutes"`
	ReservedMinutesLimit *int64 `json:"reservedMinutesLimit"`
	Problem              string `json:"problem"`
}

// ChainMember is one job of the chain in hand: what it was, how it stands,
// when it ran and the cap it reserved. It carries no consumed minutes.
type ChainMember struct {
	Job        string  `json:"job"`
	Role       string  `json:"role"`
	Round      int     `json:"round"`
	Status     string  `json:"status"`
	StartedAt  *string `json:"startedAt"`
	EndedAt    *string `json:"endedAt"`
	CapMinutes *int    `json:"capMinutes"`
}

// Working is one thing a machine is doing, whole.
type Working struct {
	Goal  string        `json:"goal"`
	Phase Phase         `json:"phase"`
	Job   WorkingJob    `json:"job"`
	Box   *Box          `json:"box"`
	Chain []ChainMember `json:"chain"`
}

// BoxReader projects one goal's consumption. It answers null for a goal that
// carries no box at all, and a Box with a Problem and no numbers where the
// projection could not be made. The tick and the interface each supply their
// own, and a test supplies a fake, so no composition here opens a repository.
type BoxReader func(goal string) *Box

// ComposeWorking is what this machine publishes it is doing: the newest
// non-terminal job, the chain it belongs to, and the box of the goal it
// serves.
//
// The second return is the jobs reader's own explanation, exactly as
// NewestChain gives it: a machine that cannot say what it is running
// publishes null with a named detail, rather than the second-newest job as
// though it were the work in hand.
func ComposeWorking(jobs JobSet, box BoxReader) (*Working, string) {
	chain, detail := NewestChain(jobs)
	if chain == nil {
		return nil, detail
	}
	table := jobTable(jobs)
	working := workingOf(table[chain.Job], chain.Root, table, box)
	return &working, detail
}

// WorkingInFlight is every job in flight on this host, newest first.
//
// Only the machine itself can read these records, so only its own interface
// shows more than the one chain a presence record carries. Each goal's box is
// projected once however many jobs name it: the projection is a scan of the
// job directory and of the goal's attempts and runs, and asking twice would
// buy nothing.
func WorkingInFlight(jobs JobSet, box BoxReader) ([]Working, string) {
	if detail := unreadableDetail(jobs); detail != "" {
		return nil, detail
	}
	table := jobTable(jobs)
	inFlight := make([]JobRecord, 0, len(jobs.Records))
	for _, record := range jobs.Records {
		if nonTerminalStatuses[record.Status] && incompleteJob(record) == "" {
			inFlight = append(inFlight, record)
		}
	}
	sortNewestFirst(inFlight)
	boxes := map[string]*Box{}
	cached := func(goal string) *Box {
		if box == nil || goal == "" {
			return nil
		}
		if held, seen := boxes[goal]; seen {
			return held
		}
		held := box(goal)
		boxes[goal] = held
		return held
	}
	working := make([]Working, 0, len(inFlight))
	for _, record := range inFlight {
		root := lineageRoot(table, record.Job)
		if root == "" {
			continue
		}
		working = append(working, workingOf(record, root, table, cached))
	}
	return working, ""
}

// unreadableDetail is NewestChain's first rule said once: a record that could
// not be read is named, and nothing is reported as running until it is.
func unreadableDetail(jobs JobSet) string {
	if len(jobs.Unreadable) == 0 {
		return ""
	}
	sorted := append([]string(nil), jobs.Unreadable...)
	sort.Strings(sorted)
	return "chain unread: " + sorted[0]
}

func jobTable(jobs JobSet) map[string]JobRecord {
	table := make(map[string]JobRecord, len(jobs.Records))
	for _, record := range jobs.Records {
		if record.Job != "" {
			table[record.Job] = record
		}
	}
	return table
}

func workingOf(record JobRecord, root string, table map[string]JobRecord, box BoxReader) Working {
	working := Working{
		Goal: record.Goal,
		Phase: Phase{
			Role:       record.Role,
			Round:      record.Round,
			RoundLimit: roundLimitOf(table, root),
		},
		Job: WorkingJob{
			ID: record.Job, Role: record.Role, Status: record.Status,
			StartedAt:  presenceStamp(record.StartedAt),
			CapMinutes: minutesOf(record.CapMinutes),
			CapEndsAt:  capEndsAt(record),
		},
		Chain: chainMembers(table, root),
	}
	if box != nil && record.Goal != "" {
		working.Box = box(record.Goal)
	}
	return working
}

// roundLimitOf is the chain root's frozen reviewRoundLimit, and null where
// the root carries none.
//
// That one rule is both halves of the design's. Dispatch writes the field on
// a critic's root and on nothing else, so a build chain answers null without
// this file keeping a second copy of dispatch's role vocabulary — and a copy
// is exactly what would go stale when the vocabulary grows.
func roundLimitOf(table map[string]JobRecord, root string) *int {
	record, present := table[root]
	if !present || record.ReviewRoundLimit == nil {
		return nil
	}
	limit := *record.ReviewRoundLimit
	return &limit
}

// capEndsAt is when the job's reservation runs out: the recorded deadline, or
// the start plus the cap, and null where the record can say neither.
func capEndsAt(record JobRecord) *string {
	if at, err := parsePresenceTime(record.CapDeadline); err == nil {
		return presenceStamp(FormatTime(at))
	}
	if record.CapMinutes == nil || *record.CapMinutes < 1 {
		return nil
	}
	started, err := parsePresenceTime(record.StartedAt)
	if err != nil {
		return nil
	}
	return presenceStamp(FormatTime(started.Add(time.Duration(*record.CapMinutes) * time.Minute)))
}

// chainMembers is every job sharing this root, newest first, bounded.
func chainMembers(table map[string]JobRecord, root string) []ChainMember {
	held := make([]JobRecord, 0, len(table))
	for job, record := range table {
		if lineageRoot(table, job) == root {
			held = append(held, record)
		}
	}
	sortNewestFirst(held)
	if len(held) > WorkingChainMembers {
		held = held[:WorkingChainMembers]
	}
	members := make([]ChainMember, 0, len(held))
	for _, record := range held {
		members = append(members, ChainMember{
			Job: record.Job, Role: record.Role, Round: record.Round, Status: record.Status,
			StartedAt: presenceStamp(record.StartedAt), EndedAt: presenceStamp(record.EndedAt),
			CapMinutes: minutesOf(record.CapMinutes),
		})
	}
	return members
}

// sortNewestFirst is NewestChain's own order: newest by createdAt, ties by
// job id with the greater winning, so two readers of the same records never
// disagree about what comes first.
func sortNewestFirst(records []JobRecord) {
	sort.SliceStable(records, func(i, j int) bool { return newerJob(records[i], records[j]) })
}

// presenceStamp holds a record's time to the one form presence carries, and
// answers null for a time the record does not have or this engine cannot
// read. A guessed instant would be worse than none.
func presenceStamp(value string) *string {
	at, err := parsePresenceTime(value)
	if err != nil {
		return nil
	}
	held := FormatTime(at)
	return &held
}

func minutesOf(value *int) *int {
	if value == nil {
		return nil
	}
	held := *value
	return &held
}

/* ---------------------------------------------------------------- words -- */

// minutesRollUpAt is where a count of minutes stops reading as minutes.
const minutesRollUpAt = 120

// MinutesWords is how every duration on this surface reads.
//
// Minutes up to two hours, and hours and minutes from there: the records carry
// minutes — a cap is a count of them and so is a box's reservation — and up to
// two hours the count is the plainest thing to read, while "481 min" is a
// number a human has to divide before it means anything.
//
// Never a day, however many hours it comes to. A budget's day is eight hours
// (internal/goalbudget/budget.go:43), so a duration printed in days would read
// as two different lengths depending on who read it.
func MinutesWords(minutes int64) string {
	if minutes < minutesRollUpAt {
		return strconv.FormatInt(minutes, 10) + " min"
	}
	words := strconv.FormatInt(minutes/60, 10) + " h"
	if rest := minutes % 60; rest != 0 {
		words += " " + strconv.FormatInt(rest, 10) + " min"
	}
	return words
}

// PhaseWords is the one sentence a machine's Running column says.
func PhaseWords(working *Working, now time.Time) string {
	if working == nil {
		return "idle"
	}
	words := working.Phase.Role
	if working.Phase.Round > 0 {
		words += " round " + strconv.Itoa(working.Phase.Round)
		if working.Phase.RoundLimit != nil {
			words += " of " + strconv.Itoa(*working.Phase.RoundLimit)
		}
	}
	if working.Goal != "" {
		words += " on " + working.Goal
	}
	return words + " · " + JobWords(working.Job, now)
}

// JobWords is where the job in hand stands and the cap it reserved.
//
// The STATUS decides, not the timestamp. A build record is created `pending`
// with a startedAt stamped at the same instant (internal/dispatch/build.go:624,
// 665), so a minute count taken from that stamp says a job has been running
// for forty minutes while the block beside it says pending. A job that has not
// reached `running` says what it is; a terminal one says how it ended; only a
// running job is counted in minutes.
//
// The recorded stamp is untouched by this: it is still what the block shows
// as the start and what the cap's own end is computed from.
func JobWords(job WorkingJob, now time.Time) string {
	if job.Status == "" {
		return "not started"
	}
	cap := ""
	if job.CapMinutes != nil {
		cap = ", cap " + MinutesWords(int64(*job.CapMinutes))
	}
	switch {
	case !nonTerminalStatuses[job.Status]:
		// A job that has ended is what its status says. The cap it reserved
		// is a bound on work that is over, and the chain carries it.
		return job.Status
	case job.Status != RunningStatus:
		return job.Status + cap
	}
	started, err := parsePresenceTime(stampOf(job.StartedAt))
	if err != nil {
		return RunningStatus + cap
	}
	return "running " + MinutesWords(wholeMinutes(now.UTC().Sub(started))) + cap
}

// CapWords is the one forward-looking sentence this surface says, and it
// names the CAP rather than completion: the kit measures and bounds and never
// forecasts. It is empty where no deadline can be named.
func CapWords(job WorkingJob, now time.Time) string {
	endsAt, err := parsePresenceTime(stampOf(job.CapEndsAt))
	if err != nil {
		return ""
	}
	remaining := endsAt.Sub(now.UTC())
	if remaining > 0 {
		return "its cap ends in " + MinutesWords(roundedUpMinutes(remaining))
	}
	return "its cap ended " + MinutesWords(roundedUpMinutes(-remaining)) + " ago"
}

// BoxWords is the goal's box in one line: what it has spent, against what,
// and what is left of its attempts.
func BoxWords(box *Box) string {
	if box == nil {
		return "no box on this goal"
	}
	if box.Problem != "" {
		return box.Problem
	}
	parts := []string{}
	left := ""
	if box.Attempts != nil && box.AttemptLimit != nil {
		parts = append(parts, "attempt "+strconv.FormatInt(*box.Attempts, 10)+
			" of "+strconv.FormatInt(*box.AttemptLimit, 10))
		remaining := *box.AttemptLimit - *box.Attempts
		if remaining < 0 {
			remaining = 0
		}
		left = strconv.FormatInt(remaining, 10) + " attempts left"
	}
	if box.ReservedMinutes != nil && box.ReservedMinutesLimit != nil {
		// Both sides read the same way: one number rolled up into hours beside
		// one that was not would be two units in one phrase.
		parts = append(parts, MinutesWords(*box.ReservedMinutes)+" of "+
			MinutesWords(*box.ReservedMinutesLimit)+" reserved")
	}
	if left != "" {
		parts = append(parts, left)
	}
	if len(parts) == 0 {
		return "this goal's box carries no numbers this engine could read"
	}
	return strings.Join(parts, " · ")
}

// ReservedMeaning is what a reserved minute counts, said wherever the number
// is: an open job is counted at its FULL cap and not at what it has used, so
// a box that looks nearly spent may be holding reservations that come back.
const ReservedMeaning = "Reserved minutes count open jobs at their full cap."

// ChainWords is one member of the chain in words: what it was, how it stands,
// and when it ran.
func ChainWords(member ChainMember) string {
	words := member.Role
	if member.Round > 0 {
		words += " round " + strconv.Itoa(member.Round)
	}
	words += " · " + member.Status
	if member.StartedAt != nil {
		words += " · started " + *member.StartedAt
	}
	if member.EndedAt != nil {
		words += " · ended " + *member.EndedAt
	}
	if member.CapMinutes != nil {
		words += " · cap " + MinutesWords(int64(*member.CapMinutes))
	}
	return words
}

func stampOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// wholeMinutes floors, and never answers a negative: a job whose start is
// dated ahead of the reader's clock has run for no time at all.
func wholeMinutes(elapsed time.Duration) int64 {
	if elapsed < 0 {
		return 0
	}
	return int64(elapsed / time.Minute)
}

// roundedUpMinutes is the cap's own rounding: a remainder of one second is a
// whole minute, because the bound is a bound and rounding it down would name
// an instant that has already passed.
func roundedUpMinutes(remaining time.Duration) int64 {
	minutes := int64(remaining / time.Minute)
	if remaining%time.Minute != 0 {
		minutes++
	}
	return minutes
}
