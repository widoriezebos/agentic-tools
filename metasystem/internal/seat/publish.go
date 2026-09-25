package seat

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// RunnerContext is what only the resident runner knows about itself. It is
// filled by the steward's RunLoop from the identity it was enrolled with and
// from the lineage steward arm handed to steward run; a tick with no runner
// context publishes nothing, so a command process can never publish the
// rollout confirmation of a machine it merely ran a verb on.
type RunnerContext struct {
	RepoIdentity string
	Generation   int
	Engine       string
	ArmedLineage string
	TickSeconds  int
}

// JobRecord is the part of a delegate job record presence reads. A chain is
// a lineage linked by Parent, so the ROOT is resolved by walking that link,
// never taken from the immediate parent: a round-four critic whose parent is
// the round-three implementer belongs to the chain they both descend from.
type JobRecord struct {
	// Parent is the record this one follows; empty means this record is its
	// chain's root.
	Parent    string
	Job       string
	Role      string
	Goal      string
	Round     int
	StartedAt string
	CreatedAt string
	Status    string
	// EndedAt is stamped when a job reaches a status it cannot leave, and is
	// empty on every job still in flight.
	EndedAt string
	// CapMinutes is capRequest.minutes: the minutes this job reserved, which
	// is what the goal's box counts an open job at. Null on a record that
	// names none, because a cap is never assumed.
	CapMinutes *int
	// CapDeadline is the recorded enforcement deadline where dispatch wrote
	// one; without it a cap ends at the start plus the minutes above, which
	// is how the reap reaches the same verdict from the same record.
	CapDeadline string
	// ReviewRoundLimit is the limit dispatch froze on a critic chain's ROOT.
	// It is null on every other record, which is what makes a build's phase
	// show its round without a denominator.
	ReviewRoundLimit *int
}

// JobSet is one read of the machine's delegate job records, naming whatever
// could not be read rather than hiding it.
type JobSet struct {
	Records    []JobRecord
	Unreadable []string
}

// nonTerminalStatuses is dispatch's canonical vocabulary for a job that is
// still in flight (internal/dispatch/record.go:56-60).
var nonTerminalStatuses = map[string]bool{"pending-setup": true, "pending": true, "running": true}

// RunningStatus is the one of those three in which a job is actually being
// worked. The other two are reservations: dispatch stamps a startedAt on a
// record it creates `pending` (build.go:624, 665), so the status and not the
// stamp is what says whether anything is running.
const RunningStatus = "running"

// NewestChain picks the newest non-terminal delegate job: newest by
// createdAt, ties by job id, with the greater id winning so two readers of
// the same records never disagree.
//
// A record that cannot answer for itself is never silently skipped. An
// unreadable record, and a newest record whose fields are incomplete or
// whose ancestry does not resolve to a root, both yield a NULL chain with a
// named detail: a machine that cannot say what it is running says so,
// instead of reporting the second-newest job as though it were the work in
// hand.
func NewestChain(jobs JobSet) (*Chain, string) {
	if len(jobs.Unreadable) > 0 {
		sorted := append([]string(nil), jobs.Unreadable...)
		sort.Strings(sorted)
		return nil, "chain unread: " + sorted[0]
	}
	table := make(map[string]JobRecord, len(jobs.Records))
	for _, record := range jobs.Records {
		if record.Job != "" {
			table[record.Job] = record
		}
	}
	var best *JobRecord
	for index := range jobs.Records {
		candidate := jobs.Records[index]
		if !nonTerminalStatuses[candidate.Status] {
			continue
		}
		if best == nil || newerJob(candidate, *best) {
			winner := candidate
			best = &winner
		}
	}
	if best == nil {
		return nil, ""
	}
	if problem := incompleteJob(*best); problem != "" {
		return nil, "chain unnamed: " + problem
	}
	root := lineageRoot(table, best.Job)
	if root == "" {
		return nil, "chain unrooted: the ancestry of job " + best.Job + " does not resolve to a root"
	}
	chain := Chain{Root: root, Job: best.Job, Role: best.Role, Round: best.Round, Goal: best.Goal}
	if best.StartedAt != "" {
		started := best.StartedAt
		chain.StartedAt = &started
	}
	return &chain, ""
}

// incompleteJob names the first field a chain cannot be composed without.
func incompleteJob(record JobRecord) string {
	switch {
	case record.Job == "":
		return "a job record in flight names no job"
	case record.Role == "":
		return "job " + record.Job + " names no role"
	case record.CreatedAt == "":
		return "job " + record.Job + " names no createdAt"
	case record.Round < 0:
		return "job " + record.Job + " carries a negative round"
	}
	return ""
}

// lineageRoot resolves a job's chain root by walking Parent through one
// loaded table, which is how dispatch resolves it (internal/dispatch/chain.go:28).
// A walk that leaves the table, cycles, or is missing a link roots NOWHERE:
// a member of a corrupt chain is attributed to no chain, never to whatever
// record a broken walk happened to stop on.
func lineageRoot(table map[string]JobRecord, job string) string {
	seen := map[string]bool{}
	for {
		record, present := table[job]
		if !present || seen[job] {
			return ""
		}
		seen[job] = true
		if record.Parent == "" {
			return job
		}
		job = record.Parent
	}
}

func newerJob(candidate, best JobRecord) bool {
	if candidate.CreatedAt != best.CreatedAt {
		return candidate.CreatedAt > best.CreatedAt
	}
	return candidate.Job > best.Job
}

// Compose builds this machine's record for one tick. The detail it returns
// is the chain's own explanation when the job records could not be read; it
// never stops the publish.
//
// The box reader is the one thing this composition cannot read for itself: a
// goal's consumption is dispatch's projection over the accepted tip, and the
// component that owns the tick supplies it. A nil reader composes a record
// whose boxes are null, which is what a build with no projection can say.
func Compose(machine string, runner RunnerContext, jobs JobSet, box BoxReader, tickAt time.Time) (Record, string, error) {
	if err := ValidateMachineName(machine); err != nil {
		return Record{}, "", err
	}
	if runner.Generation < 1 {
		return Record{}, "", fmt.Errorf("presence cannot be composed without an armed generation")
	}
	engine := runner.Engine
	if engine == "" {
		engine = UnknownEngine
	}
	lineage := runner.ArmedLineage
	if lineage == "" {
		lineage = NoLease
	}
	tickSeconds := runner.TickSeconds
	if tickSeconds < 1 {
		tickSeconds = 1
	}
	chain, detail := NewestChain(jobs)
	working, _ := ComposeWorking(jobs, box)
	return Record{
		PresenceSchema: RecordSchema,
		Machine:        machine,
		RepoIdentity:   runner.RepoIdentity,
		Generation:     runner.Generation,
		Engine:         engine,
		ArmedLineage:   lineage,
		TickSeconds:    tickSeconds,
		Chain:          chain,
		Working:        working,
		TickAt:         FormatTime(tickAt),
	}, detail, nil
}

// CommitMessage is the one line every presence commit carries.
func CommitMessage(record Record) string {
	return fmt.Sprintf("seat presence %s %s", record.Machine, record.TickAt)
}

// Write is one publish: which ref, which record, what the commit descends
// from, and whether the ref update may replace what is there. Parent and
// Force are separate because they are separate facts: rung 3's first publish
// creates the branch from nothing and must still not be a force push.
type Write struct {
	Ref     string
	Message string
	File    []byte
	// Parent is the commit the new record descends from; empty means a
	// parentless commit.
	Parent string
	// Force replaces whatever the ref holds. Without it the update must
	// fast-forward, which also creates a ref that does not exist yet.
	Force bool
}

// RemoteWriter publishes one presence commit and moves one ref to it. The
// git implementation is Git; the behaviour tests drive fakes.
type RemoteWriter interface {
	Publish(Write) (commit string, err error)
}

// PublishRequest is one pass up the ladder.
type PublishRequest struct {
	Record Record
	// Start is the rung the publication state remembers.
	Start Rung
	// Pinned is seat.presence-namespace, empty when the operator left the
	// ladder to find its own rung.
	Pinned string
	// BranchTip is the newest commit this clone knows on the branch rung's
	// ref for this machine, the parent a fast-forward push needs.
	BranchTip string
}

// PublishResult names the rung that carried the record.
type PublishResult struct {
	Rung   Rung
	Commit string
	// Fell is true when a rung refused by name and the publisher climbed
	// down, which the component reports as its detail.
	Fell     bool
	Refusals []string
}

// Publish climbs the ladder: each rung is tried in turn, a refusal that
// names the ref moves one rung down, and a transport failure that names no
// ref ends the publish where it stands, because the rung was not the fault.
func Publish(writer RemoteWriter, request PublishRequest) (PublishResult, error) {
	record := request.Record
	if err := ValidateMachineName(record.Machine); err != nil {
		return PublishResult{}, err
	}
	file, err := record.Encode()
	if err != nil {
		return PublishResult{}, err
	}
	message := CommitMessage(record)
	result := PublishResult{}
	rungs := Ladder(request.Start, request.Pinned)
	var lastErr error
	for index, rung := range rungs {
		ref := rung.Ref(record.Machine)
		write := Write{Ref: ref, Message: message, File: file, Force: rung.Force()}
		if !rung.Force() {
			// The fast-forward rung descends from whatever this clone last
			// read on the branch; an empty tip is the branch's first record.
			write.Parent = request.BranchTip
		}
		commit, err := writer.Publish(write)
		if err == nil {
			result.Rung, result.Commit = rung, commit
			result.Fell = index > 0
			return result, nil
		}
		lastErr = err
		var refused *RefRefused
		if !errors.As(err, &refused) {
			return PublishResult{Rung: rung, Refusals: result.Refusals}, err
		}
		result.Refusals = append(result.Refusals, refused.Error())
		if index == len(rungs)-1 {
			return PublishResult{Rung: rung, Refusals: result.Refusals}, err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("presence has no rung to publish on")
	}
	return PublishResult{Refusals: result.Refusals}, lastErr
}

// Conflict reports the one case where another checkout is publishing under
// this machine's nickname: the record already at the remote names a
// different repoIdentity and is newer than the one about to be published.
func Conflict(published *Record, mine Record) error {
	if published == nil || published.RepoIdentity == mine.RepoIdentity {
		return nil
	}
	if !published.At().After(mine.At()) {
		return nil
	}
	return fmt.Errorf("SEAT_PRESENCE_CONFLICT: the presence of %s at %s was published by repository identity %s, not this checkout's %s",
		mine.Machine, published.TickAt, published.RepoIdentity, mine.RepoIdentity)
}
