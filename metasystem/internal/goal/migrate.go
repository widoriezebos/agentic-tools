package goal

// The migration (BGS-4): ONE commit, ONE opid (R7-04) turns the
// legacy single-file ledger into the synthesized live+done tree
// with the root record, deleting goals.md and goals-accepted.json —
// the exact clean-path set. Semantic-lossless over the shipped
// parser domain: a legacy ledger that does not parse cleanly
// refuses by name before anything mutates, and the REVIEWED source
// digest literal is checked first of all (R5-08) — the migration
// runs on exactly the bytes the review saw or not at all.
// Deterministic under injected identity and timestamp; the rerun is
// idempotent keyed on the root record and mode.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateOptions pins everything the synthesis needs. Identity and
// the VerbRequest's Now are INJECTED so the same inputs produce the
// same bytes on any machine (the determinism legs).
type MigrateOptions struct {
	// SourceDigest is the reviewed goals.md sha256 literal, checked
	// against the file's bytes BEFORE any mutation.
	SourceDigest string
	// ManifestPath is empty for a bare migration; otherwise the
	// amendment document applied after synthesis.
	ManifestPath string
	// Identity is the adoption ULID, minted once, never rewritten.
	Identity string
	// SyncMode is committed into the root record: remote or local.
	SyncMode string
}

// Migrate synthesizes and publishes the new ledger.
func Migrate(r VerbRequest, opts MigrateOptions) (PublishResult, error) {
	if opts.Identity == "" {
		return PublishResult{}, fmt.Errorf("migration needs its adoption identity — minted once, injected for determinism")
	}
	if opts.SyncMode != SyncRemote && opts.SyncMode != SyncLocal {
		return PublishResult{}, fmt.Errorf("migration commits a sync mode: remote or local")
	}
	mode := "bare"
	var manifestEntries []ManifestEntry
	manifestDigest := ""
	if opts.ManifestPath != "" {
		mode = "manifest"
		manifestBytes, err := os.ReadFile(opts.ManifestPath)
		if err != nil {
			return PublishResult{}, fmt.Errorf("the manifest cannot be read: %w", err)
		}
		manifestDigest = sha256HexBytes(manifestBytes)
		manifestEntries, err = ParseManifest(manifestBytes)
		if err != nil {
			return PublishResult{}, err
		}
	}

	// The reviewed-source precondition, before ANY mutation: the
	// ledger bytes are exactly what the review saw.
	sourcePath := filepath.Join(r.Endpoint.Root, "plans", "goals.md")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return PublishResult{}, fmt.Errorf("the legacy ledger cannot be read: %w", err)
	}
	if got := sha256HexBytes(sourceBytes); got != opts.SourceDigest {
		return PublishResult{}, fmt.Errorf("source digest mismatch refused: goals.md is %s, the reviewed literal is %s — the migration runs on exactly the reviewed bytes or not at all", got, opts.SourceDigest)
	}
	legacy, problems := Parse(sourceBytes)
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = string(p)
		}
		return PublishResult{}, fmt.Errorf("the legacy ledger does not parse cleanly; semantic-lossless refuses:\n%s", strings.Join(lines, "\n"))
	}

	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "migrate", Args: map[string]string{
			"sourceDigest": opts.SourceDigest, "manifestDigest": manifestDigest,
			"mode": mode, "syncMode": opts.SyncMode, "identity": opts.Identity,
		}},
		Message: "goal migrate (" + mode + ")",
		Mutate: func(tip string) ([]Change, error) {
			// The idempotent rerun, keyed on the root record + mode:
			// a tip already carrying this ledger's root record is the
			// completed migration — never re-synthesized. A DIFFERENT
			// identity or mode on the tip is a confusion refused by
			// name (--manifest after bare included).
			if existing, catErr := gitIn(r.Endpoint.Root, "cat-file", "-p", tip+":"+goalsPrefix+"backlog.md"); catErr == nil {
				root, rootProblems := ParseRoot([]byte(existing))
				if len(rootProblems) == 0 {
					if root.Identity == opts.Identity && root.MigrationMode == mode {
						return nil, AlreadyApplied{}
					}
					return nil, fmt.Errorf("a migration already completed with identity %s mode %s; rerunning with mode %s is a confusion, not a repair", root.Identity, root.MigrationMode, mode)
				}
			}

			tree, synthErr := synthesize(legacy, manifestEntries, r, opts, mode, manifestDigest)
			if synthErr != nil {
				return nil, synthErr
			}
			var changes []Change
			for p, content := range tree {
				changes = append(changes, Change{Path: p, Content: content})
			}
			// The exact clean-path set: the legacy ledger and its
			// acceptance baseline die in the same commit.
			changes = append(changes,
				Change{Path: "plans/goals.md", Delete: true},
				Change{Path: "plans/goals-accepted.json", Delete: true},
			)
			return changes, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// synthesize maps the legacy ledger plus the manifest onto the new
// tree's bytes, deterministically.
func synthesize(legacy *Ledger, entries []ManifestEntry, r VerbRequest, opts MigrateOptions, mode, manifestDigest string) (map[string][]byte, error) {
	files := map[string][]byte{}
	goals := map[string]*GoalFile{}
	place := func(f *GoalFile) { goals[f.Id] = f }

	migrated := func(id, state, intent, origin, next string) *GoalFile {
		f := &GoalFile{
			Id: id, State: state, Intent: intent, Origin: origin,
			NextStep: next, OpenedAt: r.stamp(), Revision: 0,
		}
		touch(f, r, "migrate", []string{id})
		return f
	}

	if legacy.Current != nil {
		c := legacy.Current
		f := migrated(c.Id, StateClaimed, c.Intent, c.Origin, c.NextStep)
		// The legacy Current IS this machine's claim.
		f.Claimed = &ClaimRecord{Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, At: r.stamp()}
		place(f)
	}
	for i := range legacy.Queued {
		q := legacy.Queued[i]
		place(migrated(q.Id, StateQueued, q.Intent, q.Origin, q.NextStep))
	}
	for i := range legacy.Parked {
		p := legacy.Parked[i]
		f := migrated(p.Id, StateParked, p.Intent, p.Origin, p.NextStep)
		f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: p.Parked}
		place(f)
	}
	doneSet := map[string]bool{}
	for i := range legacy.Done {
		d := legacy.Done[i]
		f := migrated(d.Id, StateDone, d.Intent, d.Origin, d.NextStep)
		f.Conclude = d.Conclude
		place(f)
		doneSet[d.Id] = true
	}

	// The manifest's queue writes, applied on the synthesized set.
	for _, entry := range entries {
		switch entry.Kind {
		case "add-goal":
			if _, exists := goals[entry.Id]; exists {
				return nil, fmt.Errorf("manifest add-goal %s: the ledger already carries it", entry.Id)
			}
			f := migrated(entry.Id, StateQueued, entry.Intent, entry.Origin, entry.Next)
			if entry.ParkedBecause != "" {
				f.State = StateParked
				by := entry.ParkedBy
				if by == "" {
					by = r.Actor.historyActor()
				}
				at := entry.ParkedAt
				if at == "" {
					at = r.stamp()
				}
				f.Parked = &ParkRecord{By: by, At: at, Because: entry.ParkedBecause}
			}
			f.Blocked = append([]string(nil), entry.BlockedBy...)
			place(f)
		case "amend-goal":
			f, exists := goals[entry.Id]
			if !exists {
				return nil, fmt.Errorf("manifest amend-goal %s: no such goal in the synthesized ledger", entry.Id)
			}
			if entry.HasNext {
				f.NextStep = entry.Next
			}
			if entry.HasBlocked {
				if entry.ClearBlocked {
					f.Blocked = nil
				} else {
					f.Blocked = append([]string(nil), entry.BlockedBy...)
				}
			}
			if entry.ParkedBecause != "" && f.State != StateDone {
				f.State = StateParked
				by := entry.ParkedBy
				if by == "" {
					by = r.Actor.historyActor()
				}
				at := entry.ParkedAt
				if at == "" {
					at = r.stamp()
				}
				f.Parked = &ParkRecord{By: by, At: at, Because: entry.ParkedBecause}
				f.Claimed = nil
			}
		}
	}

	for id, f := range goals {
		if f.State == StateDone {
			files[donePath(id)] = RenderFile(f)
		} else {
			files[livePath(id)] = RenderFile(f)
		}
	}

	root := &RootRecord{
		Identity: opts.Identity, FormatVersion: "1", SyncMode: opts.SyncMode,
		MigrationEpoch: r.stamp(), ManifestDigest: manifestDigest,
		MigrationMode: mode, Revision: 1,
	}
	if legacy.Free != nil {
		root.Free = &FreeRecord{Declared: legacy.Free.Declared, Origin: legacy.Free.Origin, Digest: legacy.Free.Digest}
	}
	root.History = append(root.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: "migrate",
		Actor: r.Actor.historyActor(), Keep: -1,
	})
	files[goalsPrefix+"backlog.md"] = RenderRoot(root)
	return files, nil
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
