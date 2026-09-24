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

// JobRecord is the part of a delegate job record presence reads.
type JobRecord struct {
	Root      string
	Job       string
	Role      string
	Goal      string
	Round     int
	StartedAt string
	CreatedAt string
	Status    string
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

// NewestChain picks the newest non-terminal delegate job: newest by
// createdAt, ties by job id, with the greater id winning so two readers of
// the same records never disagree. An unreadable or corrupt record makes the
// chain null and names itself in the returned detail rather than failing the
// publish.
func NewestChain(jobs JobSet) (*Chain, string) {
	if len(jobs.Unreadable) > 0 {
		sorted := append([]string(nil), jobs.Unreadable...)
		sort.Strings(sorted)
		return nil, "chain unread: " + sorted[0]
	}
	var best *JobRecord
	for index := range jobs.Records {
		candidate := jobs.Records[index]
		if !nonTerminalStatuses[candidate.Status] || candidate.Job == "" {
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
	chain := Chain{Root: best.Root, Job: best.Job, Role: best.Role, Round: best.Round, Goal: best.Goal}
	if best.StartedAt != "" {
		started := best.StartedAt
		chain.StartedAt = &started
	}
	return &chain, ""
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
func Compose(machine string, runner RunnerContext, jobs JobSet, tickAt time.Time) (Record, string, error) {
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
	return Record{
		PresenceSchema: RecordSchema,
		Machine:        machine,
		RepoIdentity:   runner.RepoIdentity,
		Generation:     runner.Generation,
		Engine:         engine,
		ArmedLineage:   lineage,
		TickSeconds:    tickSeconds,
		Chain:          chain,
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
