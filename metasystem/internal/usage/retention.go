package usage

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

const (
	callRetentionSchema     = 1
	callRetentionWindowDays = 14
	callRetentionWindow     = callRetentionWindowDays * 24 * time.Hour
	// CallRetentionWindow is the minimum age the usage store may retire.
	CallRetentionWindow = callRetentionWindow
)

type callRetention struct {
	SchemaVersion int       `json:"schemaVersion"`
	RetainedSince time.Time `json:"retainedSince"`
}

type callRetirementJournal struct {
	SchemaVersion    int       `json:"schemaVersion"`
	Runtime          string    `json:"runtime"`
	Session          string    `json:"session"`
	SamplesBytes     int64     `json:"samplesBytes"`
	Cutoff           time.Time `json:"cutoff"`
	CursorDigest     string    `json:"cursorDigest"`
	CursorFileDev    uint64    `json:"cursorFileDev"`
	CursorFileInode  uint64    `json:"cursorFileInode"`
	SamplesPresent   bool      `json:"samplesPresent"`
	SamplesDigest    string    `json:"samplesDigest,omitempty"`
	SamplesFileDev   uint64    `json:"samplesFileDev,omitempty"`
	SamplesFileInode uint64    `json:"samplesFileInode,omitempty"`
}

type callRetirementCandidate struct {
	Session     CallSession
	CursorPath  string
	SamplesPath string
	Newest      time.Time
	Orphan      bool
}

type callRetirementCompletedError struct{ err error }

func (e *callRetirementCompletedError) Error() string { return e.err.Error() }
func (e *callRetirementCompletedError) Unwrap() error { return e.err }

var (
	writeCallRetentionText = atomicfile.WriteText
	removeCallStorePath    = os.Remove
	syncCallStoreDirectory = syncCallDirectory
	callRetentionNow       = time.Now
)

// PruneCallSessions retires complete cursor/sample pairs whose filesystem and
// committed evidence are both strictly older than before. Registry history
// and stable lock files are never removed. It waits for the maintenance
// lock and judges the cutoff against the wall clock.
func PruneCallSessions(installationRoot string, before time.Time) (removed int, err error) {
	return retireCallSessions(context.Background(), installationRoot, before, callRetentionNow(), 0, false)
}

// PruneCallSessionsAt is PruneCallSessions judged against the caller's
// clock: the context-prune verb passes the now it prunes handoffs with.
func PruneCallSessionsAt(installationRoot string, before, now time.Time) (removed int, err error) {
	return retireCallSessions(context.Background(), installationRoot, before, now, 0, false)
}

// RetireCallSessions is the disk sweeper's retirement (Part B 3.5): the
// cutoff is judged against now, the maintenance lock is taken exclusively
// without waiting (held is a *CallStoreBusyError at once), the interrupted
// journals present are recovered, at most limit pairs are retired, the
// context is checked between pairs, and the lock is released on return.
func RetireCallSessions(ctx context.Context, installationRoot string, before, now time.Time, limit int) (removed int, err error) {
	if limit < 1 {
		return 0, fmt.Errorf("call retirement limit must be positive, got %d", limit)
	}
	return retireCallSessions(ctx, installationRoot, before, now, limit, true)
}

func retireCallSessions(ctx context.Context, installationRoot string, before, now time.Time, limit int, nonBlocking bool) (removed int, err error) {
	if !filepath.IsAbs(installationRoot) {
		return 0, fmt.Errorf("state root must be absolute: %s", installationRoot)
	}
	if err := validateCallCutoff(before, now); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := validateCallStorageParents(installationRoot); err != nil {
		return 0, err
	}
	maintenance, err := lockCallMaintenance(installationRoot, true, nonBlocking)
	if err != nil {
		return 0, err
	}
	defer unlockCallFile(maintenance)

	recovered, err := recoverAllCallRetirements(installationRoot)
	removed += recovered
	if err != nil {
		return removed, err
	}
	retention, err := readCallRetention(installationRoot)
	if err != nil {
		return removed, err
	}
	registrations, _, err := callRegistrationsUnderMaintenance(installationRoot)
	if err != nil {
		return removed, err
	}
	candidates, err := callRetirementCandidates(installationRoot, registrations, before, false)
	if err != nil {
		return removed, err
	}
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	if len(candidates) == 0 {
		return removed, nil
	}

	hasPair := false
	for _, candidate := range candidates {
		if !candidate.Orphan {
			hasPair = true
			break
		}
	}
	if hasPair {
		candidateBoundary := callRetentionBoundary(before)
		if retention.RetainedSince.After(candidateBoundary) {
			candidateBoundary = retention.RetainedSince
		}
		retention = callRetention{SchemaVersion: callRetentionSchema, RetainedSince: candidateBoundary}
		if err := publishCallRetention(installationRoot, retention); err != nil {
			return removed, err
		}
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		var retireErr error
		if candidate.Orphan {
			retireErr = retireEmptySamplesOrphan(candidate, before)
		} else {
			retireErr = retireCallSession(installationRoot, candidate.Session, before)
		}
		if retireErr != nil {
			var completed *callRetirementCompletedError
			if errors.As(retireErr, &completed) {
				removed++
			}
			return removed, retireErr
		}
		removed++
	}
	return removed, nil
}

func validateCallCutoff(before, now time.Time) error {
	if now.IsZero() {
		return fmt.Errorf("call retirement needs one nonzero clock observation")
	}
	if !before.Before(now.UTC().Add(-callRetentionWindow)) {
		return fmt.Errorf("call retirement cutoff must be older than retention window: window=%dd cutoff=%s", callRetentionWindowDays, before.UTC().Format(time.RFC3339Nano))
	}
	return nil
}

// CallInspection is InspectCallSessions' observation.
type CallInspection struct {
	// Candidates are the pairs a retirement would retire now, oldest first.
	Candidates []CallSession
	// Orphans are empty samples files with no cursor a retirement removes.
	Orphans []string
	// Interrupted are the cursor paths whose retirement journal a retirement
	// would recover first.
	Interrupted []string
	// MaintenanceHeld reports the maintenance lock held at the probe.
	MaintenanceHeld bool
}

// InspectCallSessions is the read-only half of the retirement (Part B 3.5,
// DL3B-05): it lists the retirable pairs and the interrupted journals,
// recovers nothing, publishes no boundary, truncates nothing and creates no
// file or directory; the maintenance lock is probed without being created.
func InspectCallSessions(installationRoot string, before, now time.Time) (CallInspection, error) {
	var inspection CallInspection
	if !filepath.IsAbs(installationRoot) {
		return inspection, fmt.Errorf("state root must be absolute: %s", installationRoot)
	}
	if err := validateCallCutoff(before, now); err != nil {
		return inspection, err
	}
	if err := validateCallStorageParents(installationRoot); err != nil {
		return inspection, err
	}
	free, err := ProbeMaintenance(installationRoot)
	if err != nil {
		return inspection, err
	}
	inspection.MaintenanceHeld = !free
	inspection.Interrupted, err = callRetirementCursorPaths(installationRoot)
	if err != nil {
		return inspection, err
	}
	registrations, _, err := readCallRegistrations(filepath.Join(installationRoot, "artifacts", "agents", "context", "sessions.jsonl"))
	if err != nil {
		return inspection, err
	}
	candidates, err := callRetirementCandidates(installationRoot, registrations, before, true)
	if err != nil {
		return inspection, err
	}
	for _, candidate := range candidates {
		if candidate.Orphan {
			inspection.Orphans = append(inspection.Orphans, candidate.SamplesPath)
		} else {
			inspection.Candidates = append(inspection.Candidates, candidate.Session)
		}
	}
	return inspection, nil
}

// recoverCallRetirement completes one journal-authorized deletion. The caller
// holds the maintenance lock and the stable lock beside cursorPath.
func recoverCallRetirement(installationRoot, cursorPath string) error {
	journalPath := callRetirementPath(cursorPath)
	journal, present, err := readCallRetirementJournal(installationRoot, cursorPath)
	if err != nil || !present {
		return err
	}
	retention, err := readCallRetention(installationRoot)
	if err != nil {
		return err
	}
	if retention.RetainedSince.Before(callRetentionBoundary(journal.Cutoff)) {
		return fmt.Errorf("call retirement record %s is not covered by retention boundary %s", journalPath, retention.RetainedSince.Format(time.RFC3339Nano))
	}
	if err := validateCallRetirementTargets(installationRoot, cursorPath, journal); err != nil {
		return err
	}
	// A journal can be visible after a post-rename sync failure. Rewriting both
	// authorizations makes their durability explicit before recovery unlinks.
	if err := publishCallRetention(installationRoot, retention); err != nil {
		return err
	}
	if err := publishCallRetirementJournal(installationRoot, journalPath, journal); err != nil {
		return err
	}
	return completeCallRetirement(installationRoot, cursorPath, journal)
}

// retireCallSession publishes a recoverable authorization before deleting a
// pair. Its caller holds the exclusive maintenance lock.
func retireCallSession(installationRoot string, session CallSession, before time.Time) error {
	cursorPath := CursorPath(installationRoot, session.Runtime, session.Session)
	lock, err := lockCallFile(cursorPath + ".lock")
	if err != nil {
		return fmt.Errorf("cannot lock call session retirement %s: %w", cursorPath, err)
	}
	defer unlockCallFile(lock)

	journal, err := prepareCallRetirement(installationRoot, session, before)
	if err != nil {
		return err
	}
	journalPath := callRetirementPath(cursorPath)
	if err := requireUnusedCallRetirementPath(journalPath); err != nil {
		return err
	}
	if err := publishCallRetirementJournal(installationRoot, journalPath, journal); err != nil {
		return err
	}
	return completeCallRetirement(installationRoot, cursorPath, journal)
}

func recoverAllCallRetirements(installationRoot string) (int, error) {
	paths, err := callRetirementCursorPaths(installationRoot)
	if err != nil {
		return 0, err
	}
	// Validate the whole journal set before any unlink. One damaged record
	// must not let lexically earlier records authorize further deletion.
	for _, cursorPath := range paths {
		lock, err := lockCallFile(cursorPath + ".lock")
		if err != nil {
			return 0, fmt.Errorf("cannot lock call retirement %s: %w", cursorPath, err)
		}
		validateErr := validateRecoverableCallRetirement(installationRoot, cursorPath)
		unlockCallFile(lock)
		if validateErr != nil {
			return 0, validateErr
		}
	}
	recovered := 0
	for _, cursorPath := range paths {
		lock, err := lockCallFile(cursorPath + ".lock")
		if err != nil {
			return recovered, fmt.Errorf("cannot lock call retirement %s: %w", cursorPath, err)
		}
		recoverErr := recoverCallRetirement(installationRoot, cursorPath)
		unlockCallFile(lock)
		if recoverErr != nil {
			var completed *callRetirementCompletedError
			if errors.As(recoverErr, &completed) {
				recovered++
			}
			return recovered, recoverErr
		}
		recovered++
	}
	return recovered, nil
}

func validateRecoverableCallRetirement(installationRoot, cursorPath string) error {
	journal, present, err := readCallRetirementJournal(installationRoot, cursorPath)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("call retirement record disappeared before recovery: %s", callRetirementPath(cursorPath))
	}
	retention, err := readCallRetention(installationRoot)
	if err != nil {
		return err
	}
	if retention.RetainedSince.Before(callRetentionBoundary(journal.Cutoff)) {
		return fmt.Errorf("call retirement record %s is not covered by retention boundary %s", callRetirementPath(cursorPath), retention.RetainedSince.Format(time.RFC3339Nano))
	}
	return validateCallRetirementTargets(installationRoot, cursorPath, journal)
}

func callRetirementCandidates(installationRoot string, registrations []CallRegistration, before time.Time, observe bool) ([]callRetirementCandidate, error) {
	if err := validateCallStorageParents(installationRoot); err != nil {
		return nil, err
	}
	cursorDir := filepath.Join(installationRoot, "artifacts", "agents", "context", "cursors")
	samplesDir := filepath.Join(installationRoot, "artifacts", "agents", "context", "samples")
	stems := map[string]bool{}
	if err := collectCallStoreStems(cursorDir, ".json", stems); err != nil {
		return nil, err
	}
	if err := collectCallStoreStems(samplesDir, ".jsonl", stems); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(stems))
	for stem := range stems {
		names = append(names, stem)
	}
	sort.Strings(names)

	var candidates []callRetirementCandidate
	for _, stem := range names {
		cursorPath := filepath.Join(cursorDir, stem+".json")
		if observe {
			candidate, eligible, inspectErr := inspectCallRetirementCandidate(installationRoot, cursorPath, registrations, before, true)
			if inspectErr != nil {
				return nil, inspectErr
			}
			if eligible {
				candidates = append(candidates, candidate)
			}
			continue
		}
		lock, err := lockCallFile(cursorPath + ".lock")
		if err != nil {
			return nil, fmt.Errorf("cannot lock call retirement candidate %s: %w", cursorPath, err)
		}
		candidate, eligible, inspectErr := inspectCallRetirementCandidate(installationRoot, cursorPath, registrations, before, false)
		unlockCallFile(lock)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if eligible {
			candidates = append(candidates, candidate)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].Newest.Equal(candidates[j].Newest) {
			return candidates[i].Newest.Before(candidates[j].Newest)
		}
		return candidates[i].CursorPath < candidates[j].CursorPath
	})
	return candidates, nil
}

// inspectCallRetirementCandidate judges one pair. observe never truncates:
// a samples log past its committed boundary needs recovery first, so an
// observation reports the pair not eligible this pass.
func inspectCallRetirementCandidate(installationRoot, cursorPath string, registrations []CallRegistration, before time.Time, observe bool) (callRetirementCandidate, bool, error) {
	samplesPath := filepath.Join(installationRoot, "artifacts", "agents", "context", "samples", strings.TrimSuffix(filepath.Base(cursorPath), ".json")+".jsonl")
	cursorInfo, cursorExists, err := callStoreMember(cursorPath)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	samplesInfo, samplesExists, err := callStoreMember(samplesPath)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	if !cursorExists {
		if !samplesExists {
			return callRetirementCandidate{}, false, nil
		}
		if samplesInfo.Size() != 0 {
			return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("nonempty call samples have no trustworthy cursor identity"))
		}
		if !samplesInfo.ModTime().Before(before) {
			return callRetirementCandidate{}, false, nil
		}
		return callRetirementCandidate{CursorPath: cursorPath, SamplesPath: samplesPath, Newest: samplesInfo.ModTime(), Orphan: true}, true, nil
	}
	if cursorInfo.Mode().Perm()&0o444 == 0 {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("call cursor is unreadable: %s", cursorPath))
	}
	cursor, err := readDiscoveredCallCursor(cursorPath)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	if CursorPath(installationRoot, cursor.Runtime, cursor.Session) != cursorPath || SamplesPath(installationRoot, cursor.Runtime, cursor.Session) != samplesPath {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("cursor identity %s/%s does not map to both store basenames", cursor.Runtime, cursor.Session))
	}
	if observe {
		if samplesExists && samplesInfo.Size() < cursor.SamplesBytes {
			return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("call samples length %d is below committed boundary %d", samplesInfo.Size(), cursor.SamplesBytes))
		}
		if samplesExists && samplesInfo.Size() > cursor.SamplesBytes {
			return callRetirementCandidate{}, false, nil
		}
	} else if err := reconcileCallRows(samplesPath, cursor, true); err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	cursorInfo, cursorExists, err = callStoreMember(cursorPath)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	if !cursorExists {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, fmt.Errorf("call cursor disappeared during inspection"))
	}
	samplesInfo, samplesExists, err = callStoreMember(samplesPath)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	if !cursorInfo.ModTime().Before(before) || samplesExists && !samplesInfo.ModTime().Before(before) {
		return callRetirementCandidate{}, false, nil
	}
	newest := cursorInfo.ModTime()
	if samplesExists && samplesInfo.ModTime().After(newest) {
		newest = samplesInfo.ModTime()
	}
	if !callRegistrationsAllowRetirement(registrations, cursor.Runtime, cursor.Session, before) {
		return callRetirementCandidate{}, false, nil
	}
	rowsSafe, err := callRowsAllowRetirement(samplesPath, samplesExists, cursor, before)
	if err != nil {
		return callRetirementCandidate{}, false, pairError(cursorPath, samplesPath, err)
	}
	if !rowsSafe {
		return callRetirementCandidate{}, false, nil
	}
	return callRetirementCandidate{
		Session: CallSession{Runtime: cursor.Runtime, Session: cursor.Session}, CursorPath: cursorPath,
		SamplesPath: samplesPath, Newest: newest,
	}, true, nil
}

func callRegistrationsAllowRetirement(registrations []CallRegistration, runtime, session string, before time.Time) bool {
	found := false
	for _, row := range registrations {
		if row.Runtime != runtime || row.Session != session {
			continue
		}
		found = true
		if row.FirstSeen.IsZero() || !row.FirstSeen.Before(before) {
			return false
		}
	}
	return found
}

func callRowsAllowRetirement(path string, present bool, cursor CursorState, before time.Time) (bool, error) {
	if !present {
		return cursor.SamplesBytes == 0, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if info.Mode().Perm()&0o444 == 0 || info.Size() != cursor.SamplesBytes {
		return false, nil
	}
	observeCallOpen(path)
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, cursor.SamplesBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), maxCallLineBytes)
	samplesSeen := map[string]bool{}
	markersSeen := map[string]bool{}
	for scanner.Scan() {
		var header struct {
			Kind string `json:"kind"`
		}
		if !decodeOneJSON(scanner.Bytes(), &header) {
			return false, nil
		}
		switch header.Kind {
		case "sample":
			var row sampleRow
			if !decodeOneJSON(scanner.Bytes(), &row) || row.Runtime != cursor.Runtime || row.Session != cursor.Session || row.InvocationID == "" || row.At.IsZero() || !expectedCallSampleSource(row.Runtime, row.Source) || row.PromptTokens < 0 || row.InputTokens < 0 || row.CacheCreation < 0 || row.CacheRead < 0 || row.Ordinal < 0 {
				return false, nil
			}
			if !row.At.Before(before) || samplesSeen[row.InvocationID] {
				return false, nil
			}
			samplesSeen[row.InvocationID] = true
		case "marker":
			var row markerRow
			if !decodeOneJSON(scanner.Bytes(), &row) || row.Runtime != cursor.Runtime || row.Session != cursor.Session || row.At.IsZero() || row.Ordinal < 0 {
				return false, nil
			}
			identity := fmt.Sprintf("%s\x00%d\x00%s", row.At.Format(time.RFC3339Nano), row.Ordinal, row.Detail)
			if !row.At.Before(before) || markersSeen[identity] {
				return false, nil
			}
			markersSeen[identity] = true
		default:
			return false, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return true, nil
}

func expectedCallSampleSource(runtime, source string) bool {
	want := map[string]string{"claude": "claude-transcript", "codex": "codex-rollout"}[runtime]
	return want != "" && (source == want || strings.HasPrefix(source, want+"; "))
}

func decodeOneJSON(data []byte, value any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(value) != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func decodeOneStrictJSON(data []byte, value any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func prepareCallRetirement(installationRoot string, session CallSession, before time.Time) (callRetirementJournal, error) {
	cursorPath := CursorPath(installationRoot, session.Runtime, session.Session)
	cursorBytes, err := os.ReadFile(cursorPath)
	if err != nil {
		return callRetirementJournal{}, fmt.Errorf("cannot read call cursor for retirement %s: %w", cursorPath, err)
	}
	cursor, err := readDiscoveredCallCursor(cursorPath)
	if err != nil {
		return callRetirementJournal{}, fmt.Errorf("cannot validate call cursor for retirement %s: %w", cursorPath, err)
	}
	if cursor.Runtime != session.Runtime || cursor.Session != session.Session {
		return callRetirementJournal{}, fmt.Errorf("call cursor retirement identity changed at %s", cursorPath)
	}
	registrations := []CallRegistration{{Runtime: session.Runtime, Session: session.Session, FirstSeen: before.Add(-time.Nanosecond)}}
	candidate, eligible, err := inspectCallRetirementCandidate(installationRoot, cursorPath, registrations, before, false)
	if err != nil {
		return callRetirementJournal{}, err
	}
	if !eligible || candidate.Orphan {
		return callRetirementJournal{}, fmt.Errorf("call session is no longer eligible for retirement: %s/%s", session.Runtime, session.Session)
	}
	samplesPath := SamplesPath(installationRoot, session.Runtime, session.Session)
	_, samplesPresent, err := callStoreMember(samplesPath)
	if err != nil {
		return callRetirementJournal{}, err
	}
	journal := callRetirementJournal{
		SchemaVersion: callRetentionSchema, Runtime: session.Runtime, Session: session.Session,
		SamplesBytes: cursor.SamplesBytes, Cutoff: before, CursorDigest: digestBytes(cursorBytes),
		SamplesPresent: samplesPresent,
	}
	cursorInfo, cursorPresent, err := callStoreMember(cursorPath)
	if err != nil {
		return callRetirementJournal{}, err
	}
	if !cursorPresent {
		return callRetirementJournal{}, fmt.Errorf("call cursor disappeared before retirement authorization: %s", cursorPath)
	}
	journal.CursorFileDev, journal.CursorFileInode, err = callStoreIdentity(cursorInfo)
	if err != nil {
		return callRetirementJournal{}, err
	}
	if samplesPresent {
		journal.SamplesDigest, err = digestCallStore(samplesPath)
		if err != nil {
			return callRetirementJournal{}, err
		}
		samplesInfo, stillPresent, statErr := callStoreMember(samplesPath)
		if statErr != nil {
			return callRetirementJournal{}, statErr
		}
		if !stillPresent {
			return callRetirementJournal{}, fmt.Errorf("call samples disappeared before retirement authorization: %s", samplesPath)
		}
		journal.SamplesFileDev, journal.SamplesFileInode, err = callStoreIdentity(samplesInfo)
		if err != nil {
			return callRetirementJournal{}, err
		}
	}
	return journal, nil
}

func validateCallRetirementTargets(installationRoot, cursorPath string, journal callRetirementJournal) error {
	if err := validateCallStorageParents(installationRoot); err != nil {
		return err
	}
	if CursorPath(installationRoot, journal.Runtime, journal.Session) != cursorPath {
		return fmt.Errorf("call retirement record %s identity %s/%s does not map to its own stem", callRetirementPath(cursorPath), journal.Runtime, journal.Session)
	}
	samplesPath := SamplesPath(installationRoot, journal.Runtime, journal.Session)
	if info, present, err := callStoreMember(cursorPath); err != nil {
		return pairError(cursorPath, samplesPath, err)
	} else if present {
		dev, inode, identityErr := callStoreIdentity(info)
		if identityErr != nil || dev != journal.CursorFileDev || inode != journal.CursorFileInode {
			return pairError(cursorPath, samplesPath, fmt.Errorf("surviving cursor file identity does not match retirement authorization"))
		}
		data, readErr := os.ReadFile(cursorPath)
		if readErr != nil {
			return pairError(cursorPath, samplesPath, readErr)
		}
		cursor, cursorErr := readDiscoveredCallCursor(cursorPath)
		if cursorErr != nil || cursor.Runtime != journal.Runtime || cursor.Session != journal.Session || cursor.SamplesBytes != journal.SamplesBytes || digestBytes(data) != journal.CursorDigest {
			return pairError(cursorPath, samplesPath, fmt.Errorf("surviving cursor does not match retirement authorization"))
		}
	}
	if info, present, err := callStoreMember(samplesPath); err != nil {
		return pairError(cursorPath, samplesPath, err)
	} else if present {
		if !journal.SamplesPresent {
			return pairError(cursorPath, samplesPath, fmt.Errorf("samples appeared after retirement authorization"))
		}
		dev, inode, identityErr := callStoreIdentity(info)
		if identityErr != nil || dev != journal.SamplesFileDev || inode != journal.SamplesFileInode {
			return pairError(cursorPath, samplesPath, fmt.Errorf("surviving samples file identity does not match retirement authorization"))
		}
		digest, digestErr := digestCallStore(samplesPath)
		if digestErr != nil {
			return pairError(cursorPath, samplesPath, fmt.Errorf("cannot checksum the surviving samples: %w", digestErr))
		}
		if digest != journal.SamplesDigest {
			return pairError(cursorPath, samplesPath, fmt.Errorf("surviving samples do not match retirement authorization"))
		}
	}
	return nil
}

func completeCallRetirement(installationRoot, cursorPath string, journal callRetirementJournal) error {
	if err := validateCallRetirementTargets(installationRoot, cursorPath, journal); err != nil {
		return err
	}
	samplesPath := SamplesPath(installationRoot, journal.Runtime, journal.Session)
	journalPath := callRetirementPath(cursorPath)
	for _, path := range []string{samplesPath, cursorPath} {
		if err := removeCallStorePath(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot remove retired call store member %s: %w", path, err)
		}
	}
	directories := []string{filepath.Dir(cursorPath)}
	if journal.SamplesPresent {
		directories = append([]string{filepath.Dir(samplesPath)}, directories...)
	}
	for _, directory := range directories {
		if err := syncCallStoreDirectory(directory); err != nil {
			return fmt.Errorf("cannot sync retired call store directory %s: %w", directory, err)
		}
	}
	if err := removeCallStorePath(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot remove completed call retirement record %s: %w", journalPath, err)
	}
	if err := syncCallStoreDirectory(filepath.Dir(journalPath)); err != nil {
		return &callRetirementCompletedError{err: fmt.Errorf("cannot sync completed call retirement record folder %s: %w", filepath.Dir(journalPath), err)}
	}
	return nil
}

func retireEmptySamplesOrphan(candidate callRetirementCandidate, before time.Time) error {
	lock, err := lockCallFile(candidate.CursorPath + ".lock")
	if err != nil {
		return err
	}
	defer unlockCallFile(lock)
	info, present, err := callStoreMember(candidate.SamplesPath)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if info.Size() != 0 || !info.ModTime().Before(before) {
		return fmt.Errorf("empty samples orphan is no longer eligible for retirement: %s", candidate.SamplesPath)
	}
	if err := removeCallStorePath(candidate.SamplesPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := syncCallStoreDirectory(filepath.Dir(candidate.SamplesPath)); err != nil {
		return &callRetirementCompletedError{err: fmt.Errorf("cannot sync retired empty samples directory %s: %w", filepath.Dir(candidate.SamplesPath), err)}
	}
	return nil
}

func callRetentionPath(installationRoot string) string {
	return filepath.Join(installationRoot, "artifacts", "agents", "context", "retention.json")
}

func callRetirementPath(cursorPath string) string { return cursorPath + ".retiring.json" }

func callRetentionBoundary(before time.Time) time.Time {
	utc := before.UTC()
	midnight := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	if utc.Equal(midnight) {
		return midnight
	}
	return midnight.AddDate(0, 0, 1)
}

func readCallRetention(installationRoot string) (callRetention, error) {
	path := callRetentionPath(installationRoot)
	info, statErr := os.Lstat(path)
	if statErr == nil && !info.Mode().IsRegular() {
		return callRetention{}, fmt.Errorf("call retention boundary is not a regular file: %s", path)
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return callRetention{}, fmt.Errorf("cannot inspect call retention boundary %s: %w", path, statErr)
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return callRetention{}, nil
	}
	if err != nil {
		return callRetention{}, fmt.Errorf("cannot read call retention boundary %s: %w", path, err)
	}
	var retention callRetention
	if !decodeOneStrictJSON(data, &retention) || retention.SchemaVersion != callRetentionSchema || !validCallRetentionTime(retention.RetainedSince) {
		return callRetention{}, fmt.Errorf("call retention boundary is malformed: %s", path)
	}
	return retention, nil
}

func publishCallRetention(installationRoot string, retention callRetention) error {
	if retention.SchemaVersion != callRetentionSchema || !validCallRetentionTime(retention.RetainedSince) {
		return fmt.Errorf("invalid call retention boundary")
	}
	return publishCallRetentionValue(callRetentionPath(installationRoot), retention, installationRoot, "call retention boundary")
}

func publishCallRetirementJournal(installationRoot, path string, journal callRetirementJournal) error {
	return publishCallRetentionValue(path, journal, installationRoot, "record of retiring calls")
}

func publishCallRetentionValue(path string, value any, anchor, label string) error {
	rendered, err := wiredoc.RenderValue(value)
	if err != nil {
		return fmt.Errorf("cannot render %s %s: %w", label, path, err)
	}
	durable, err := writeCallRetentionText(path, string(rendered), anchor)
	if err != nil {
		return fmt.Errorf("cannot publish %s %s: %w", label, path, err)
	}
	if !durable {
		return fmt.Errorf("%s %s was published with durability unknown", label, path)
	}
	return nil
}

func readCallRetirementJournal(installationRoot, cursorPath string) (callRetirementJournal, bool, error) {
	path := callRetirementPath(cursorPath)
	if _, present, err := callStoreMember(path); err != nil {
		return callRetirementJournal{}, true, err
	} else if !present {
		return callRetirementJournal{}, false, nil
	}
	if callStorePathHasValidCursor(path) {
		return callRetirementJournal{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return callRetirementJournal{}, true, fmt.Errorf("cannot read call retirement record %s: %w", path, err)
	}
	var journal callRetirementJournal
	if !decodeOneStrictJSON(data, &journal) || journal.SchemaVersion != callRetentionSchema ||
		!runtimeNamePattern.MatchString(journal.Runtime) || journal.Session == "" || journal.SamplesBytes < 0 || journal.Cutoff.IsZero() ||
		!validDigest(journal.CursorDigest) || journal.CursorFileDev == 0 || journal.CursorFileInode == 0 ||
		journal.SamplesPresent && (!validDigest(journal.SamplesDigest) || journal.SamplesFileDev == 0 || journal.SamplesFileInode == 0) ||
		!journal.SamplesPresent && (journal.SamplesDigest != "" || journal.SamplesFileDev != 0 || journal.SamplesFileInode != 0) {
		return callRetirementJournal{}, true, fmt.Errorf("call retirement record is malformed: %s", path)
	}
	if CursorPath(installationRoot, journal.Runtime, journal.Session) != cursorPath {
		return callRetirementJournal{}, true, fmt.Errorf("call retirement record %s identity does not map to its own stem", path)
	}
	return journal, true, nil
}

func callRetirementCursorPaths(installationRoot string) ([]string, error) {
	cursorDir := filepath.Join(installationRoot, "artifacts", "agents", "context", "cursors")
	if err := validateCallStorageDirectory(cursorDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(cursorDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot list call retirement records %s: %w", cursorDir, err)
	}
	var paths []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json.retiring.json") {
			path := filepath.Join(cursorDir, entry.Name())
			if !callStorePathHasValidCursor(path) {
				paths = append(paths, strings.TrimSuffix(path, ".retiring.json"))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func validateCallStorageParents(installationRoot string) error {
	for _, directory := range []string{
		filepath.Join(installationRoot, "artifacts"),
		filepath.Join(installationRoot, "artifacts", "agents"),
		filepath.Join(installationRoot, "artifacts", "agents", "context"),
		filepath.Join(installationRoot, "artifacts", "agents", "context", "cursors"),
		filepath.Join(installationRoot, "artifacts", "agents", "context", "samples"),
	} {
		if err := validateCallStorageDirectory(directory); err != nil {
			return err
		}
	}
	return nil
}

func callStorePathHasValidCursor(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var cursor CursorState
	if json.Unmarshal(data, &cursor) != nil || !validCallCursor(cursor, cursor.Runtime, cursor.Session) {
		return false
	}
	return filepath.Base(CursorPath("", cursor.Runtime, cursor.Session)) == filepath.Base(path)
}

func requireUnusedCallRetirementPath(path string) error {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot inspect call retirement record path %s: %w", path, err)
	}
	return fmt.Errorf("call retirement record path collides with an existing cursor or record: %s", path)
}

func validateCallStorageDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot inspect call storage directory %s: %w", path, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("call storage parent is not a regular directory: %s", path)
	}
	return nil
}

func validCallRetentionTime(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	_, offset := value.Zone()
	return offset == 0 && value.Equal(callRetentionBoundary(value))
}

func callStoreIdentity(info os.FileInfo) (uint64, uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("call store file has no filesystem identity")
	}
	return uint64(stat.Dev), uint64(stat.Ino), nil
}

func digestCallStore(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func digestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func syncCallDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}
