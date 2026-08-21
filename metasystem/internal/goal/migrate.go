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
	var manifest *Manifest
	manifestDigest := ""
	if opts.ManifestPath != "" {
		mode = "manifest"
		manifestBytes, err := os.ReadFile(opts.ManifestPath)
		if err != nil {
			return PublishResult{}, fmt.Errorf("the manifest cannot be read: %w", err)
		}
		manifestDigest = sha256HexBytes(manifestBytes)
		manifest, err = ParseManifest(manifestBytes)
		if err != nil {
			return PublishResult{}, err
		}
		// The manifest BINDS the reviewed literal itself (F3): a
		// caller-provided digest that disagrees is a confusion.
		if manifest.ReviewedSHA256 != opts.SourceDigest {
			return PublishResult{}, fmt.Errorf("the manifest binds reviewed digest %s but the caller supplied %s; the manifest is the authority", manifest.ReviewedSHA256, opts.SourceDigest)
		}
	} else {
		manifest = &Manifest{Epoch: r.stamp()}
	}

	// The worktree preconditions, before ANY mutation (F3): the
	// clean-path set must match HEAD — an uncommitted hand edit to
	// the ledger or its baseline dies here, not inside the commit.
	for _, cleanPath := range []string{"plans/goals.md", "plans/goals-accepted.json"} {
		if out, diffErr := gitIn(r.Endpoint.Root, "diff", "HEAD", "--", cleanPath); diffErr == nil && strings.TrimSpace(out) != "" {
			return PublishResult{}, fmt.Errorf("migration precondition refused: %s has uncommitted changes", cleanPath)
		}
	}
	// The fast digest gate on the worktree copy — the authoritative
	// read happens tip-side inside the transaction (F2).
	sourcePath := filepath.Join(r.Endpoint.Root, "plans", "goals.md")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return PublishResult{}, fmt.Errorf("the legacy ledger cannot be read: %w", err)
	}
	if got := sha256HexBytes(sourceBytes); got != opts.SourceDigest {
		return PublishResult{}, fmt.Errorf("source digest mismatch refused: goals.md is %s, the reviewed literal is %s — the migration runs on exactly the reviewed bytes or not at all", got, opts.SourceDigest)
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

			// The AUTHORITATIVE source is the TIP's ledger (F2): a
			// concurrent legacy advance since the review makes the
			// tip's bytes differ from the reviewed literal, and the
			// migration refuses rather than silently discarding it.
			tipSource, catErr := gitIn(r.Endpoint.Root, "cat-file", "-p", tip+":plans/goals.md")
			if catErr != nil {
				return nil, fmt.Errorf("the canonical tip carries no plans/goals.md; a completed migration reruns idempotently, anything else is a confusion: %v", catErr)
			}
			if got := sha256HexBytes([]byte(tipSource)); got != opts.SourceDigest {
				return nil, fmt.Errorf("the canonical ledger advanced past the review: the tip's goals.md is %s, the reviewed literal is %s — re-review before migrating", got, opts.SourceDigest)
			}
			if _, accErr := gitIn(r.Endpoint.Root, "cat-file", "-e", tip+":plans/goals-accepted.json"); accErr != nil {
				return nil, fmt.Errorf("migration precondition refused: the tip carries no goals-accepted.json baseline")
			}
			if lsOut, lsErr := gitIn(r.Endpoint.Root, "ls-tree", "--name-only", tip, "--", goalsPrefix); lsErr == nil && strings.TrimSpace(lsOut) != "" {
				return nil, fmt.Errorf("migration precondition refused: %s already exists on the tip without a root record — not all-legacy: a confusion", goalsPrefix)
			}
			legacy, problems := Parse([]byte(tipSource))
			if len(problems) > 0 {
				lines := make([]string, len(problems))
				for i, p := range problems {
					lines[i] = string(p)
				}
				return nil, fmt.Errorf("the legacy ledger does not parse cleanly; semantic-lossless refuses:\n%s", strings.Join(lines, "\n"))
			}
			tree, synthErr := synthesize(legacy, manifest, r, opts, mode, manifestDigest)
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
func synthesize(legacy *Ledger, manifest *Manifest, r VerbRequest, opts MigrateOptions, mode, manifestDigest string) (map[string][]byte, error) {
	files := map[string][]byte{}
	goals := map[string]*GoalFile{}
	place := func(f *GoalFile) { goals[f.Id] = f }
	legacyPosition := 0

	// Every converted goal's OpenedAt comes from the EPOCH's
	// positional formula (F4): deterministic on any machine, and
	// (OpenedAt, id) ordering reproduces the ledger's textual order.
	migrated := func(id, state, intent, origin, next string) (*GoalFile, error) {
		legacyPosition++
		opened, err := manifest.LegacyOpenedAt(legacyPosition)
		if err != nil {
			return nil, err
		}
		f := &GoalFile{
			Id: id, State: state, Intent: intent, Origin: origin,
			NextStep: next, OpenedAt: opened, Revision: 0,
		}
		touch(f, r, "migrate", []string{id})
		return f, nil
	}
	// Semantic-lossless (F4): Evidence lines and any prose the
	// legacy parser tolerated survive as LegacyNotes — nothing the
	// review read disappears.
	carryNotes := func(f *GoalFile, g Goal) {
		for _, ev := range g.Evidence {
			f.Legacy = append(f.Legacy, "Evidence: "+ev)
		}
	}

	if legacy.Current != nil {
		c := legacy.Current
		f, err := migrated(c.Id, StateClaimed, c.Intent, c.Origin, c.NextStep)
		if err != nil {
			return nil, err
		}
		// The legacy Current IS this machine's claim.
		f.Claimed = &ClaimRecord{Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, At: r.stamp()}
		carryNotes(f, *c)
		place(f)
	}
	for i := range legacy.Queued {
		q := legacy.Queued[i]
		f, err := migrated(q.Id, StateQueued, q.Intent, q.Origin, q.NextStep)
		if err != nil {
			return nil, err
		}
		carryNotes(f, q)
		place(f)
	}
	for i := range legacy.Parked {
		p := legacy.Parked[i]
		f, err := migrated(p.Id, StateParked, p.Intent, p.Origin, p.NextStep)
		if err != nil {
			return nil, err
		}
		f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: manifest.Epoch, Because: p.Parked}
		carryNotes(f, p)
		place(f)
	}
	for i := range legacy.Done {
		d := legacy.Done[i]
		f, err := migrated(d.Id, StateDone, d.Intent, d.Origin, d.NextStep)
		if err != nil {
			return nil, err
		}
		f.Conclude = d.Conclude
		carryNotes(f, d)
		place(f)
	}

	// The manifest's queue writes, applied on the synthesized set.
	for _, entry := range manifest.Entries {
		switch entry.Kind {
		case "add-goal":
			if _, exists := goals[entry.Id]; exists {
				return nil, fmt.Errorf("manifest add-goal %s: the ledger already carries it", entry.Id)
			}
			opened, err := manifest.AddOpenedAt(entry.Position)
			if err != nil {
				return nil, err
			}
			f := &GoalFile{
				Id: entry.Id, State: StateQueued, Intent: entry.Intent,
				Origin: entry.Origin, NextStep: entry.Next,
				OpenedAt: opened, Revision: 0, Arc: entry.Arc,
			}
			touch(f, r, "migrate", []string{entry.Id})
			f.Blocked = append([]string(nil), entry.BlockedBy...)
			place(f)
		case "amend-goal":
			f, exists := goals[entry.Id]
			if !exists {
				return nil, fmt.Errorf("manifest amend-goal %s: no such goal in the synthesized ledger", entry.Id)
			}
			before := string(RenderFile(f))
			if entry.HasNext {
				f.NextStep = entry.Next
			}
			if entry.HasArc {
				f.Arc = entry.Arc
			}
			if entry.HasBlocked {
				if entry.ClearBlocked {
					f.Blocked = nil
				} else {
					f.Blocked = append([]string(nil), entry.BlockedBy...)
				}
			}
			switch entry.State {
			case StateParked:
				if f.State == StateDone {
					return nil, fmt.Errorf("manifest amend-goal %s: a done goal cannot be manifest-parked", entry.Id)
				}
				at := entry.ParkedAt
				if at == "EPOCH" {
					at = manifest.Epoch
				}
				f.State = StateParked
				f.Parked = &ParkRecord{By: entry.ParkedBy, At: at, Because: entry.ParkedBecause}
				f.Claimed = nil
			case StateQueued:
				f.State = StateQueued
				f.Parked = nil
			}
			// No entry is a no-op (R5-09): an amend that changes
			// nothing against the converted output refuses.
			if string(RenderFile(f)) == before {
				return nil, fmt.Errorf("manifest amend-goal %s: the amendment changes nothing against the converted output", entry.Id)
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
		MigrationEpoch: manifest.Epoch, ManifestDigest: manifestDigest,
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

// SourceDigestOf exposes the migration's digest computation to the
// CLI so the rehearsal computes the reviewed literal with the same
// bytes-in, hex-out rule the precondition checks.
func SourceDigestOf(data []byte) string {
	return sha256HexBytes(data)
}
