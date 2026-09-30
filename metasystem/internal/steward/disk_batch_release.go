package steward

// The sweeper's retry of landing batch members' release sets (design
// engine-owns-disk-lifetimes Part B 3.6; Round D3 N6): a batch member's set
// is recorded at join and run by the batch's P6 step; one left unfinished (a
// held store, a missing census) is retried by the batch's own recovery and,
// here, by the checkout pass of the member's seat, which touches only its
// own members' recorded ids. The batch records are the landing package's,
// which imports this one, so the engine registers the retry.

import (
	"context"

	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// BatchReleaseRetry reads and retries the batch members' release sets of
// one seat in one landing lane.
type BatchReleaseRetry struct {
	// Unfinished lists the ids of the lane's batches holding a landed
	// member of seat whose release set is unfinished; it reads only.
	Unfinished func(lane, seat string) ([]string, error)
	// Retry runs those members' pending entries in batch id and records
	// each outcome.
	Retry func(ctx context.Context, lane, seat, id string, at time.Time) error
}

var registeredBatchReleaseRetry BatchReleaseRetry

// RegisterBatchReleaseRetry installs the engine's batch release retry.
func RegisterBatchReleaseRetry(retry BatchReleaseRetry) { registeredBatchReleaseRetry = retry }

// BatchReleaseSets is the class of a seat's batch members with an
// unfinished release set, over the host's landing lanes.
type BatchReleaseSets struct {
	Seat    string
	Lanes   []string
	LaneErr error
	Retry   BatchReleaseRetry
}

func (BatchReleaseSets) Name() string { return "batch release sets" }

// Plan lists every batch holding one of this seat's landed members with an
// unfinished set; an unresolvable lane or unreadable batches hold the
// class.
func (c BatchReleaseSets) Plan(_ context.Context, _ *diskstore.Pass) ([]diskstore.Item, error) {
	if c.Retry.Unfinished == nil || c.Retry.Retry == nil {
		return nil, nil
	}
	if c.LaneErr != nil {
		return []diskstore.Item{{Class: c.Name(), Key: "~lane", Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the landing lane cannot be resolved: " + c.LaneErr.Error(), Command: "metasystem system check"}}}, nil
	}
	var items []diskstore.Item
	for _, lane := range c.Lanes {
		ids, err := c.Retry.Unfinished(lane, c.Seat)
		if err != nil {
			return []diskstore.Item{{Class: c.Name(), Key: "~" + lane, Path: lane, Verdict: diskstore.Verdict{Decision: diskstore.Pending,
				Reason: "the landing batches cannot be read: " + err.Error(), Command: "metasystem disk show"}}}, nil
		}
		for _, id := range ids {
			items = append(items, diskstore.Item{Class: c.Name(), Key: lane + "\x00" + id, Path: lane,
				Verdict: diskstore.Verdict{Decision: diskstore.Release, Reason: fmt.Sprintf("landing batch %s holds a landed member of this checkout with workspaces left to release", id)}})
		}
	}
	return items, nil
}

// Apply retries one batch's members of this seat and says what is left.
func (c BatchReleaseSets) Apply(ctx context.Context, pass *diskstore.Pass, item diskstore.Item) diskstore.Verdict {
	lane, id, ok := cutKey(item.Key)
	if !ok {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "not a batch item", Command: "metasystem disk show"}
	}
	if err := c.Retry.Retry(ctx, lane, c.Seat, id, pass.Now); err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "landing batch " + id + ": " + err.Error(), Command: "metasystem disk clean"}
	}
	ids, err := c.Retry.Unfinished(lane, c.Seat)
	if err != nil {
		return diskstore.Verdict{Decision: diskstore.Pending, Reason: "the landing batches cannot be read: " + err.Error(), Command: "metasystem disk show"}
	}
	for _, left := range ids {
		if left == id {
			return diskstore.Verdict{Decision: diskstore.Pending, Reason: "landing batch " + id + " still has workspaces of this checkout to release", Command: "metasystem disk show"}
		}
	}
	return diskstore.Verdict{Decision: diskstore.Release, Reason: "landing batch " + id + "'s release sets of this checkout are finished"}
}

func cutKey(key string) (string, string, bool) {
	for index := range key {
		if key[index] == 0 {
			return key[:index], key[index+1:], true
		}
	}
	return "", "", false
}
