package diskstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// A person's acts on the disk (3.8): each one works from what a preview or
// the registry shows, revalidates every item at execution, declines an item
// a live process holds (naming the holder and what to run once it ended),
// never waives a content proof, and completes the rest. A repeat whose
// effect holds writes nothing (R-129).

// PersonOutcome is one item of a person's act.
type PersonOutcome struct {
	Path    string `json:"path"`
	Done    bool   `json:"done"`
	Reason  string `json:"reason"`
	Command string `json:"command,omitempty"`
}

// ExecuteStrays removes exactly the stray items of a plan: each still lies
// directly under one of roots with an engine-prefixed name, still has the
// device and inode the preview recorded, is still idle a day, and no
// readable live process has its cwd, executable or an open file inside. A
// process the census could not read is the person's judgement: the preview
// named it before the person chose to run --strays. Foreign entries are
// never in a strays plan and are skipped here too.
func ExecuteStrays(ctx context.Context, plan Plan, now time.Time, census *UseCensus, roots []string) []PersonOutcome {
	var outcomes []PersonOutcome
	allowed := map[string]bool{}
	for _, root := range roots {
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			allowed[resolved] = true
		}
	}
	for _, item := range plan.Items {
		if !item.Stray {
			continue
		}
		outcomes = append(outcomes, executeStray(ctx, item, now, census, allowed))
	}
	return outcomes
}

func executeStray(ctx context.Context, item Item, now time.Time, census *UseCensus, allowed map[string]bool) PersonOutcome {
	outcome := PersonOutcome{Path: item.Path}
	name := filepath.Base(item.Path)
	if !allowed[filepath.Dir(item.Path)] || !hasAnyPrefix(name, strayPrefixes) || hasAnyPrefix(name, notStrays) || name == "metasystem" {
		outcome.Reason = "not an engine stray in a temporary root; it is never removed by this command"
		return outcome
	}
	info, err := os.Lstat(item.Path)
	if errors.Is(err, os.ErrNotExist) {
		outcome.Done, outcome.Reason = true, "already gone"
		return outcome
	}
	if err != nil {
		outcome.Reason, outcome.Command = "cannot be read: "+err.Error(), "metasystem disk clean --preview"
		return outcome
	}
	if device, inode, _ := fileID(info); device != item.Device || inode != item.Inode || pathGeneration(item.Path) != item.Generation {
		outcome.Reason, outcome.Command = "replaced since the preview", "metasystem disk clean --preview, then --strays with the new plan"
		return outcome
	}
	_, newest, complete := Measure(ctx, item.Path)
	if !complete {
		outcome.Reason, outcome.Command = "could not be walked to the end", "metasystem disk clean --strays again"
		return outcome
	}
	if newest.IsZero() {
		newest = info.ModTime()
	}
	if idle := now.Sub(newest); idle < StrayIdle {
		outcome.Reason = fmt.Sprintf("written %s ago; a stray is removed once it has been idle a day", idle.Round(time.Minute))
		outcome.Command = "metasystem disk clean --preview tomorrow, then --strays"
		return outcome
	}
	if census != nil {
		if holders := census.Holders(item.Path); len(holders) != 0 {
			holder := holders[0]
			outcome.Reason = fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command)
			outcome.Command = fmt.Sprintf("metasystem disk clean --strays once pid %d has ended", holder.Pid)
			return outcome
		}
	}
	if err := RemoveTree(ctx, item.Path); err != nil {
		outcome.Reason, outcome.Command = "removal stopped: "+err.Error(), "metasystem disk clean --strays again"
		return outcome
	}
	outcome.Done, outcome.Reason = true, "removed"
	return outcome
}

// FindRecord looks an id up in each registry in turn.
func FindRecord(registries []Registry, id string) (Registry, Record, error) {
	for _, registry := range registries {
		record, err := registry.Load(id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		return registry, record, err
	}
	return Registry{}, Record{}, ErrNotFound
}

// ReleaseByPerson is `disk clean --release ID` (DL3B-12): the person
// supplies the use judgement a census could not make. Everything else
// stands: the owner kind's proof must say the owner ended and the content
// is safe (clean, landed, captured, contained: never waived), the identity
// is revalidated inside the critical section, the record lock is taken
// without waiting, and every readable live process must be outside the
// store. A released store is success and writes nothing.
func ReleaseByPerson(ctx context.Context, registry Registry, id string, proof OwnerProof, census *UseCensus, by string) (Verdict, error) {
	record, err := registry.Load(id)
	if err != nil {
		return Verdict{}, err
	}
	if record.State == StateReleased {
		return Verdict{Decision: Release, Reason: "already released"}, nil
	}
	if proof == nil {
		return Verdict{Decision: Keep, Reason: fmt.Sprintf("this engine has no proof for owner kind %s yet, so nothing proves its owner ended; the store is kept", record.Owner.Kind),
			Command: "metasystem disk show"}, nil
	}
	critical, err := registry.TryCritical(id)
	var held *HeldError
	if errors.As(err, &held) {
		return Verdict{Decision: Keep, Reason: "an engine verb is inside the store (its record lock is held)", Command: "metasystem disk clean --release " + id + " once that verb has ended"}, nil
	}
	if err != nil {
		return Verdict{}, err
	}
	defer critical.Release()
	record = critical.Record()
	if record.State != StateReleasing {
		if err := Revalidate(record); err != nil {
			return Verdict{Decision: Keep, Reason: "the store at the path is not the recorded one: " + err.Error(), Command: "metasystem disk show"}, nil
		}
		if verdict := proof.Observe(ctx, record); verdict.Decision != Release {
			return verdict, nil
		}
		if census == nil || !census.Taken {
			return Verdict{Decision: Keep, Reason: "no use census could be taken, so no live process is known to be outside", Command: "metasystem disk clean --release " + id}, nil
		}
		if holders := census.Holders(record.Path); len(holders) != 0 {
			holder := holders[0]
			return Verdict{Decision: Keep, Reason: fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command),
				Command: fmt.Sprintf("metasystem disk clean --release %s once pid %d has ended", id, holder.Pid)}, nil
		}
		record.State = StateReleasing
		if err := critical.Write(record); err != nil {
			return Verdict{}, err
		}
	}
	if err := proof.Apply(ctx, critical); err != nil {
		return Verdict{Decision: Pending, Reason: "removal stopped (" + err.Error() + "); the store is releasing", Command: "metasystem disk clean --release " + id}, nil
	}
	record = critical.Record()
	record.State, record.ReleasedBy = StateReleased, "person "+by
	if err := critical.Write(record); err != nil {
		return Verdict{}, err
	}
	return Verdict{Decision: Release, Reason: "released"}, nil
}

// RecordDiscard records a person's authorized discard of a delegate chain's
// uncaptured work on its workspace record, so the delegate proof can pass
// (3.2). The same person's discard again is success and writes nothing.
func RecordDiscard(registry Registry, owner Owner, by, reason string, now time.Time) (Record, bool, error) {
	records, _ := registry.Inventory()
	for _, record := range records {
		if record.Owner != owner || record.State == StateReleased {
			continue
		}
		if record.AuthorizedDiscard != nil && record.AuthorizedDiscard.By == by {
			return record, false, nil
		}
		updated, err := registry.setDiscard(record.ID, Discard{By: by, At: now.UTC(), Reason: reason})
		return updated, err == nil, err
	}
	return Record{}, false, ErrNotFound
}

// setDiscard writes the discard under the record lock, waiting for it: a
// person's act is bounded by one sweeper critical section.
func (r Registry) setDiscard(id string, discard Discard) (Record, error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDWR, 0)
	if err != nil {
		return Record{}, err
	}
	if err := flockRetry(file, unix.LOCK_EX); err != nil {
		_ = file.Close()
		return Record{}, err
	}
	defer unlockAndClose(file)
	recordLockAcquired(file)
	record, err := r.Load(id)
	if err != nil {
		return Record{}, err
	}
	record.AuthorizedDiscard = &discard
	return record, r.write(record)
}
