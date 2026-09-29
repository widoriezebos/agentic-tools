package diskstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RegisteredStores is the class of every record in one registry. A store is
// released only by its owner kind's proof (R3); a git worktree store and a
// plain workspace also need the checkout-use proof; the removal runs in the
// store's critical section (3.1).
type RegisteredStores struct {
	Registry Registry
	Proofs   map[OwnerKind]OwnerProof
}

// Name names the class by its registry.
func (s RegisteredStores) Name() string { return "stores in " + s.Registry.Dir }

// NeedsUseProof reports the stores that need the checkout-use proof: every
// git worktree store and a plain workspace (3.1).
func NeedsUseProof(record Record) bool {
	return !record.Identity.Marker || record.Layout == "plain"
}

// Plan observes every unreleased record: a missing proof, a held record
// lock, an unreadable record and an incomplete census are pending; a live
// user keeps the store; nothing is created.
func (s RegisteredStores) Plan(ctx context.Context, pass *Pass) ([]Item, error) {
	records, unreadable := s.Registry.Inventory()
	var items []Item
	for _, bad := range unreadable {
		items = append(items, Item{Class: s.Name(), Key: bad.Path, Path: bad.Path,
			Verdict: Verdict{Decision: Pending, Reason: "record unreadable: " + bad.Reason, Command: "metasystem disk show"}})
	}
	for _, record := range records {
		if record.State == StateReleased {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		item := Item{Class: s.Name(), Key: record.ID, Path: record.Path, Record: record.ID}
		if info, err := os.Lstat(record.Path); err == nil {
			item.Device, item.Inode, _ = fileID(info)
			item.Generation = pathGeneration(record.Path)
		}
		item.Verdict = s.observe(ctx, pass, record)
		if item.Verdict.Decision == Release {
			item.Bytes, _, _ = Measure(ctx, record.Path)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s RegisteredStores) observe(ctx context.Context, pass *Pass, record Record) Verdict {
	proof := s.Proofs[record.Owner.Kind]
	if proof == nil {
		return Verdict{Decision: Pending, Reason: fmt.Sprintf("this engine has no proof for owner kind %s yet; the store is kept", record.Owner.Kind),
			Command: "metasystem disk show"}
	}
	free, err := s.Registry.ProbeRecordLock(record.ID)
	if err != nil {
		return Verdict{Decision: Pending, Reason: "record lock unreadable: " + err.Error(), Command: "metasystem disk show"}
	}
	if !free {
		return Verdict{Decision: Pending, Reason: "an engine verb is inside the store (its record lock is held)", Command: "metasystem disk clean, once that verb has ended"}
	}
	verdict := proof.Observe(ctx, record)
	if verdict.Decision != Release || record.State == StateReleasing {
		return verdict
	}
	if NeedsUseProof(record) {
		return useVerdict(pass.Census(ctx), record)
	}
	return verdict
}

// useVerdict is the checkout-use proof's judgement from a census.
func useVerdict(census *UseCensus, record Record) Verdict {
	switch {
	case census == nil || !census.Taken:
		reason := "use census not taken"
		if census != nil && census.NotTaken != "" {
			reason = census.NotTaken
		}
		return Verdict{Decision: Pending, Reason: reason, Command: "metasystem disk clean"}
	case !census.Complete():
		return Verdict{Decision: Pending, Reason: "use census incomplete: " + joinLines(census.GapLines()),
			Command: "metasystem disk clean --release " + record.ID}
	}
	paths := []string{record.Path}
	if record.Class == WorkspaceClass {
		paths = append(paths, WorkspaceTmp(record))
	}
	for _, path := range paths {
		if holders := census.Holders(path); len(holders) != 0 {
			holder := holders[0]
			return Verdict{Decision: Keep, Reason: fmt.Sprintf("in use by pid %d (uid %d, %s)", holder.Pid, holder.UID, holder.Command),
				Command: fmt.Sprintf("metasystem disk clean --release %s once pid %d has ended", record.ID, holder.Pid)}
		}
	}
	return Verdict{Decision: Release, Reason: "owner ended and no live process uses it"}
}

func joinLines(lines []string) string {
	text := ""
	for index, line := range lines {
		if index > 0 {
			text += "; "
		}
		text += line
	}
	return text
}

// Apply is the critical section of 3.1 for one store: the record lock taken
// without waiting, the record reloaded and its identity revalidated, the
// content and use proofs re-run (unless the record is already releasing),
// releasing written before the first unlink, the owner's removal, released
// written, the lock released. A cut-short removal stays releasing and the
// next pass finishes it.
func (s RegisteredStores) Apply(ctx context.Context, pass *Pass, item Item) Verdict {
	critical, err := s.Registry.TryCritical(item.Record)
	var held *HeldError
	switch {
	case errors.As(err, &held):
		return Verdict{Decision: Pending, Reason: "an engine verb entered the store since the plan (its record lock is held)", Command: "metasystem disk clean, once that verb has ended"}
	case err != nil:
		return Verdict{Decision: Pending, Reason: "record unreadable: " + err.Error(), Command: "metasystem disk show"}
	}
	defer critical.Release()
	record := critical.Record()
	proof := s.Proofs[record.Owner.Kind]
	switch {
	case proof == nil:
		return Verdict{Decision: Pending, Reason: "no proof for owner kind " + string(record.Owner.Kind), Command: "metasystem disk show"}
	case record.State == StateReleased:
		return Verdict{Decision: Release, Reason: "already released"}
	case record.State == StateReleasing:
		// Its content proofs were established before its first unlink; only
		// its identity is checked again.
		if err := revalidateReleasing(record); err != nil {
			return Verdict{Decision: Pending, Reason: "a releasing store changed: " + err.Error() + "; a person decides", Command: "metasystem disk show"}
		}
	default:
		if err := Revalidate(record); err != nil {
			return Verdict{Decision: Pending, Reason: "the store is not the recorded one: " + err.Error(), Command: "metasystem disk show"}
		}
		if verdict := proof.Observe(ctx, record); verdict.Decision != Release {
			return verdict
		}
		if NeedsUseProof(record) {
			census := pass.Census(ctx)
			if census != nil && census.Taken && pass.CensusReader() != nil {
				if err := census.ReadNew(ctx, *pass.CensusReader()); err != nil {
					return Verdict{Decision: Pending, Reason: "processes started since the census could not be read: " + err.Error(), Command: "metasystem disk clean"}
				}
			}
			if verdict := useVerdict(census, record); verdict.Decision != Release {
				return verdict
			}
		}
		record.State = StateReleasing
		if err := critical.Write(record); err != nil {
			return Verdict{Decision: Pending, Reason: "could not record releasing: " + err.Error(), Command: "metasystem disk show"}
		}
	}
	if err := proof.Apply(ctx, critical); err != nil {
		return Verdict{Decision: Pending, Reason: "removal cut short (" + err.Error() + "); the store is releasing and the next pass finishes it", Command: "metasystem disk clean"}
	}
	record = critical.Record()
	record.State = StateReleased
	record.ReleasedBy = "sweeper"
	if err := critical.Write(record); err != nil {
		return Verdict{Decision: Pending, Reason: "removed, but released could not be recorded: " + err.Error(), Command: "metasystem disk clean"}
	}
	return Verdict{Decision: Release, Reason: "owner ended; released"}
}

// RemoveStore removes a marker store's tree inside its critical section,
// the marker last, so an interrupted removal still carries the marker that
// names it; a git worktree store is its owner's to remove (goal, session,
// delegate and workspace releases run their own git).
func RemoveStore(ctx context.Context, record Record) error {
	return removeStore(ctx, record, nil)
}

func removeStore(ctx context.Context, record Record, after func(string)) error {
	if !record.Identity.Marker {
		return fmt.Errorf("store %s is a git worktree; its owner removes it", record.Path)
	}
	entries, err := os.ReadDir(record.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == MarkerName {
			continue
		}
		if err := removeTree(ctx, filepath.Join(record.Path, entry.Name()), after); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(record.Path, MarkerName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Remove(record.Path)
}

// revalidateReleasing checks a store whose removal was cut short: absent is
// done; a marker store still carries its own marker, or is an empty
// directory (the marker goes last); a worktree still has its recorded .git.
func revalidateReleasing(record Record) error {
	info, err := os.Lstat(record.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", record.Path)
	}
	if !record.Identity.Marker {
		if _, err := os.Lstat(filepath.Join(record.Path, ".git")); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return Revalidate(record)
	}
	if _, err := os.Lstat(filepath.Join(record.Path, MarkerName)); err == nil {
		return Revalidate(record)
	}
	entries, err := os.ReadDir(record.Path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("%s lost its marker but is not empty", record.Path)
	}
	return nil
}
