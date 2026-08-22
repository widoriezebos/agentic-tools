package goal

// The migration: ONE commit, ONE opid turns the
// legacy single-file ledger into the synthesized live+done tree
// with the root record, deleting goals.md and goals-accepted.json —
// the exact clean-path set. Semantic-lossless over the shipped
// parser domain: a legacy ledger that does not parse cleanly
// refuses by name before anything mutates, and the REVIEWED source
// digest literal is checked first of all — the migration
// runs on exactly the bytes the review saw or not at all.
// Deterministic under injected identity and timestamp; the rerun is
// idempotent keyed on the root record and mode.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
// migrationComplete reports whether the tip already carries EXACTLY
// this migration's root record — identity, mode, sync mode, and the
// manifest digest all matching (identity+mode
// alone let a rerun requesting the opposite sync mode or a different
// manifest read as idempotent although that cutover never landed).
// Any mismatch on a completed migration is the named confusion.
func migrationComplete(root, tip, identity, mode, syncMode, manifestDigest string) (bool, error) {
	// Absence and failure are different facts here exactly as they
	// are for the ledger probe: only a tip PROVABLY carrying no root
	// record reads as "not migrated"; a probe that cannot answer, or
	// an existing record that does not parse, refuses — falling
	// through would journal a migration against a world nobody read.
	probe, err := gitIn(root, "ls-tree", "--name-only", tip, "--", goalsPrefix+"backlog.md")
	if err != nil {
		return false, fmt.Errorf("the tip's root record cannot be probed: %w", err)
	}
	if strings.TrimSpace(probe) == "" {
		return false, nil
	}
	existing, err := gitIn(root, "cat-file", "-p", tip+":./"+goalsPrefix+"backlog.md")
	if err != nil {
		return false, fmt.Errorf("the tip's root record cannot be read: %w", err)
	}
	record, problems := ParseRoot([]byte(existing))
	if len(problems) != 0 {
		return false, fmt.Errorf("the tip carries a root record that does not parse; the ledger needs repair, not a migration: %v", problems)
	}
	if record.Identity == identity && record.MigrationMode == mode &&
		record.SyncMode == syncMode && record.ManifestDigest == manifestDigest {
		return true, nil
	}
	return false, fmt.Errorf("a migration already completed with identity %s mode %s sync %s manifest %s; rerunning with mode %s sync %s manifest %s is a confusion, not a repair",
		record.Identity, record.MigrationMode, record.SyncMode, short(record.ManifestDigest),
		mode, syncMode, short(manifestDigest))
}

func Migrate(r VerbRequest, opts MigrateOptions) (PublishResult, error) {
	if opts.Identity == "" {
		return PublishResult{}, fmt.Errorf("migration needs its adoption identity — minted once, injected for determinism")
	}
	if opts.SyncMode != SyncRemote && opts.SyncMode != SyncLocal {
		return PublishResult{}, fmt.Errorf("migration commits a sync mode: remote or local")
	}
	mode := "bare"
	if opts.ManifestPath != "" {
		mode = "manifest"
	}

	// The worktree preconditions, before ANY mutation: the
	// clean-path set must match HEAD — an uncommitted hand edit to
	// the ledger, its baseline, the destination directory, or an
	// in-repo manifest dies here, not inside the commit. Porcelain
	// status covers modified AND untracked alike.
	cleanPaths := []string{"plans/goals.md", "plans/goals-accepted.json", "plans/goals"}
	manifestRel := ""
	if opts.ManifestPath != "" {
		if abs, absErr := filepath.Abs(opts.ManifestPath); absErr == nil {
			if rootAbs, rootErr := filepath.Abs(r.Endpoint.Root); rootErr == nil {
				// IsLocal, not a ".." prefix test: a lawful in-root
				// name like "..review/manifest.md" must not be
				// misread as external and skip the cleanliness and
				// tip-side proofs.
				if rel, relErr := filepath.Rel(rootAbs, abs); relErr == nil && filepath.IsLocal(rel) {
					manifestRel = filepath.ToSlash(rel)
					cleanPaths = append(cleanPaths, rel)
				}
			}
		}
	}
	for _, cleanPath := range cleanPaths {
		out, stErr := gitIn(r.Endpoint.Root, "status", "--porcelain", "--untracked-files=all", "--", cleanPath)
		if stErr != nil {
			// A probe that cannot answer proves nothing: refusing
			// beats reading failure as clean.
			return PublishResult{}, fmt.Errorf("migration precondition refused: the cleanliness of %s cannot be proven: %v", cleanPath, stErr)
		}
		if strings.TrimSpace(out) != "" {
			return PublishResult{}, fmt.Errorf("migration precondition refused: %s has uncommitted or untracked changes", cleanPath)
		}
	}

	// The manifest is read AFTER its cleanliness is proven, and the
	// bytes read here are the bytes digested and synthesized — an
	// in-repo manifest is additionally proven byte-identical at the
	// captured tip inside the transaction, so the published root
	// record can never bind a digest the committed artifact lacks.
	var manifest *Manifest
	manifestDigest := ""
	if opts.ManifestPath != "" {
		manifestBytes, err := os.ReadFile(opts.ManifestPath)
		if err != nil {
			return PublishResult{}, fmt.Errorf("the manifest cannot be read: %w", err)
		}
		manifestDigest = sha256HexBytes(manifestBytes)
		manifest, err = ParseManifest(manifestBytes)
		if err != nil {
			return PublishResult{}, err
		}
		// The manifest BINDS the reviewed literal itself: a
		// caller-provided digest that disagrees is a confusion.
		if manifest.ReviewedSHA256 != opts.SourceDigest {
			return PublishResult{}, fmt.Errorf("the manifest binds reviewed digest %s but the caller supplied %s; the manifest is the authority", manifest.ReviewedSHA256, opts.SourceDigest)
		}
	} else {
		manifest = &Manifest{Epoch: r.stamp()}
	}
	// The fast digest gate on the worktree copy — the authoritative
	// read happens tip-side inside the transaction.
	sourcePath := filepath.Join(r.Endpoint.Root, "plans", "goals.md")
	sourceBytes, err := os.ReadFile(sourcePath)
	if os.IsNotExist(err) {
		// A completed cutover deleted goals.md; the RERUN must still
		// classify idempotently. The fast worktree gate
		// has nothing to read, and the tip-side authoritative checks
		// inside the transaction own the verdict.
		sourceBytes = nil
	} else if err != nil {
		return PublishResult{}, fmt.Errorf("the legacy ledger cannot be read: %w", err)
	}
	if got := sha256HexBytes(sourceBytes); sourceBytes != nil && got != opts.SourceDigest {
		return PublishResult{}, fmt.Errorf("source digest mismatch refused: goals.md is %s, the reviewed literal is %s — the migration runs on exactly the reviewed bytes or not at all", got, opts.SourceDigest)
	}

	// The rerun is answered BEFORE any journal write: a
	// completed migration is a fact about the canonical tip, and a
	// freshly minted opid journaled "confirmed" against it would be
	// a confirmation with no History line and no trailer — a lie the
	// replay path then reads as branch surgery. The same check
	// re-runs inside the transaction for the racing case.
	// A check that cannot answer refuses HERE, before any journal
	// write: falling through to Publish on a capture outage journals
	// a fresh entry that the same outage immediately abandons — a
	// husk recovery then has to classify. Nothing is lost by
	// refusing: the transaction's own capture would fail identically.
	preNonce, nonceErr := readNonce()
	if nonceErr != nil {
		return PublishResult{}, fmt.Errorf("the rerun check cannot run: %v", nonceErr)
	}
	preTip, capErr := CaptureTip(r.Endpoint, preNonce)
	if capErr != nil {
		CleanupRefs(r.Endpoint, preNonce)
		return PublishResult{}, fmt.Errorf("the rerun check cannot capture the canonical tip: %v", capErr)
	}
	done, doneErr := migrationComplete(r.Endpoint.Root, preTip, opts.Identity, mode, opts.SyncMode, manifestDigest)
	CleanupRefs(r.Endpoint, preNonce)
	if doneErr != nil {
		return PublishResult{}, doneErr
	}
	if done {
		return PublishResult{Outcome: OutcomeConfirmed, Tip: preTip, Detail: "idempotent"}, nil
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
			if done, doneErr := migrationComplete(r.Endpoint.Root, tip, opts.Identity, mode, opts.SyncMode, manifestDigest); doneErr != nil {
				return nil, doneErr
			} else if done {
				// A racing migrator completed between the pre-journal
				// check and this capture: the desired state holds
				// WITHOUT this opid — abandoned, never a
				// manufactured confirm. The SAME four-way comparison
				// as the pre-journal check.
				return nil, NothingToDo{Reason: "the migration already completed under this identity, mode, sync mode, and manifest"}
			}

			// The AUTHORITATIVE source is the TIP's ledger: a
			// concurrent legacy advance since the review makes the
			// tip's bytes differ from the reviewed literal, and the
			// migration refuses rather than silently discarding it.
			tipSource, catErr := gitIn(r.Endpoint.Root, "cat-file", "-p", tip+":./plans/goals.md")
			if catErr != nil {
				return nil, fmt.Errorf("the canonical tip carries no plans/goals.md; a completed migration reruns idempotently, anything else is a confusion: %v", catErr)
			}
			if got := sha256HexBytes([]byte(tipSource)); got != opts.SourceDigest {
				return nil, fmt.Errorf("the canonical ledger advanced past the review: the tip's goals.md is %s, the reviewed literal is %s — re-review before migrating", got, opts.SourceDigest)
			}
			if accJSON, accErr := gitIn(r.Endpoint.Root, "cat-file", "-p", tip+":./plans/goals-accepted.json"); accErr != nil {
				return nil, fmt.Errorf("migration precondition refused: the tip carries no goals-accepted.json baseline")
			} else {
				var accepted struct {
					SchemaVersion int    `json:"schemaVersion"`
					Ledger        string `json:"ledger"`
					Sha256        string `json:"sha256"`
				}
				if jsonErr := json.Unmarshal([]byte(accJSON), &accepted); jsonErr != nil || accepted.Sha256 == "" {
					return nil, fmt.Errorf("migration precondition refused: the accepted baseline does not parse (schemaVersion/ledger/sha256)")
				}
				// The WHOLE baseline certifies:
				// its schema version, its digest, AND its full ledger
				// bytes must all agree with the tip's goals.md — a
				// crafted digest over foreign bytes proves nothing.
				if accepted.SchemaVersion != 1 {
					return nil, fmt.Errorf("migration precondition refused: the accepted baseline's schemaVersion %d is not 1", accepted.SchemaVersion)
				}
				if accepted.Sha256 != sha256HexBytes([]byte(tipSource)) || accepted.Ledger != tipSource {
					return nil, fmt.Errorf("migration precondition refused: goals.md diverges from its accepted baseline — run the legacy reconcile first")
				}
			}
			if lsOut, lsErr := gitIn(r.Endpoint.Root, "ls-tree", "--name-only", tip, "--", goalsPrefix); lsErr == nil && strings.TrimSpace(lsOut) != "" {
				return nil, fmt.Errorf("migration precondition refused: %s already exists on the tip without a root record — not all-legacy: a confusion", goalsPrefix)
			}
			if manifestRel != "" {
				// An in-repo manifest must be byte-identical at the
				// captured tip: the worktree probe and the file read
				// are separate moments, and a swap between them
				// would publish amendments nobody reviewed under a
				// digest the committed artifact does not carry.
				tipManifest, mErr := gitIn(r.Endpoint.Root, "cat-file", "-p", tip+":./"+manifestRel)
				if mErr != nil {
					return nil, fmt.Errorf("migration precondition refused: the manifest is not committed at the canonical tip: %v", mErr)
				}
				if sha256HexBytes([]byte(tipManifest)) != manifestDigest {
					return nil, fmt.Errorf("migration precondition refused: the manifest read for this migration differs from the one committed at the canonical tip")
				}
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
	// positional formula: deterministic on any machine, and
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
	// Semantic-lossless: Evidence lines and any prose the
	// legacy parser tolerated survive as LegacyNotes — nothing the
	// review read disappears.
	carryNotes := func(f *GoalFile, g Goal) {
		for _, ev := range g.Evidence {
			f.Legacy = append(f.Legacy, strings.TrimRight("Evidence: "+ev, " \t"))
		}
		// EVERY tolerated prose line survives: migration deletes
		// goals.md afterwards, so a line dropped here is gone
		// irreversibly. Trailing whitespace is trimmed at carry —
		// the destination parser right-trims every line, and a
		// carried line must read back as exactly what was stored.
		f.Legacy = append(f.Legacy, trimRightAll(g.Prose)...)
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
			// No entry is a no-op: an amend that changes
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
		// Root-level tolerated prose survives on the root record
		// exactly as per-goal prose survives on its goal.
		Legacy: trimRightAll(legacy.RootProse),
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

// trimRightAll right-trims every carried prose line: the destination
// parser right-trims on read, so only right-trimmed lines survive a
// render/parse round trip unchanged.
func trimRightAll(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return out
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
