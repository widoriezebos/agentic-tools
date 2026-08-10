package lease

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// resolveRoot canonicalises the checkout root (absolute, symlinks resolved) so
// every lease path derives from one spelling of it.
func resolveRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

func registryLockPath(root string) string {
	return filepath.Join(root, "artifacts/agents/mains", ".registry.lock")
}

// Announce records this process as a main and claims (or reconciles) the
// checkout lease for it. It returns the path of the announcement file.
func Announce(root, session string, pid, start int64, tag, runtime, ownerLineage string) (string, error) {
	root = resolveRoot(root)
	if ownerLineage != "" && !validLineage(ownerLineage) {
		return "", fmt.Errorf("owner lineage must match [A-Za-z0-9._-]{1,128}")
	}
	actualStart, startOK := StartedAt(pid)
	command, commandOK := ProcessCommand(pid)
	if !startOK || !commandOK || actualStart != start {
		return "", fmt.Errorf("announcement identity is not a live, readable process")
	}
	dir := filepath.Join(root, "artifacts/agents/mains")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	lock, err := acquireBounded(registryLockPath(root), "lease")
	if err != nil {
		return "", err
	}
	defer lock.release()

	claimer := newClaimer(root)
	records, err := readAnnouncements(root, false)
	if err != nil {
		return "", err
	}
	for _, rec := range records {
		if rec.Ann.Pid != pid || rec.Ann.PidStartedAt != start {
			continue
		}
		existing := rec.Ann
		stored := existing.OwnerLineage
		if ownerLineage != "" && stored != "" && stored != ownerLineage {
			// A process does not change its logical owner mid-life.
			return "", fmt.Errorf("announcement already carries owner lineage %s; refusing to replace it with %s", stored, ownerLineage)
		}
		if ownerLineage != "" && stored == "" {
			// Absent-to-present fill. The lease is written first so no sibling
			// of the same lineage is briefly judged foreign and sweeps.
			existing.OwnerLineage = ownerLineage
			if err := claimer.claim(&existing); err != nil {
				return "", err
			}
			if err := atomicJSON(rec.Path, &existing); err != nil {
				return "", err
			}
			return rec.Path, nil
		}
		if err := claimer.claim(&existing); err != nil {
			return "", err
		}
		return rec.Path, nil
	}

	// Not previously announced: mint a fresh identity.
	pgid, err := unix.Getpgid(int(pid))
	if err != nil {
		return "", fmt.Errorf("announcement identity is not a live, readable process")
	}
	suffix, err := tokenHex(3)
	if err != nil {
		return "", err
	}
	mainID := fmt.Sprintf("main-%d-%d-%s", start, pid, suffix)
	ann := Announcement{
		SessionId:    session,
		MainId:       mainID,
		Pid:          pid,
		PidStartedAt: start,
		Pgid:         int64(pgid),
		Runtime:      runtime,
		InstanceTag:  tag,
		CommandHash:  CommandHash(command),
		AnnouncedAt:  nowStamp(),
		OwnerLineage: ownerLineage,
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d.json", safeSession(session), pid))
	if err := atomicJSON(path, &ann); err != nil {
		return "", err
	}
	if err := initializeCursor(root, mainID); err != nil {
		return "", err
	}
	if err := claimer.claim(&ann); err != nil {
		return "", err
	}
	return path, nil
}

// Retire removes this process's announcement.
func Retire(root, session string, pid, start int64) error {
	root = resolveRoot(root)
	lock, err := acquireBounded(registryLockPath(root), "lease")
	if err != nil {
		return err
	}
	defer lock.release()
	records, err := readAnnouncements(root, false)
	if err != nil {
		return err
	}
	for _, rec := range records {
		if rec.Ann.Pid == pid && rec.Ann.PidStartedAt == start && rec.Ann.SessionId == session {
			if err := os.Remove(rec.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// Classify resolves and reports who a caller is, plus whether it holds the
// checkout and the current epoch/revision.
func ClassifyVerb(root string, callerPid int64) (map[string]any, error) {
	root = resolveRoot(root)
	identity, err := Classify(root, callerPid)
	if err != nil {
		return nil, err
	}
	lease, err := loadLease(root, false)
	if err != nil {
		return nil, err
	}
	out := classificationMap(identity)
	out["holder"] = identity.Class == ClassMain && (lease == nil || identity.MainId == lease.HolderMainId)
	if lease != nil {
		out["claimEpoch"] = lease.ClaimEpoch
		out["revision"] = lease.Revision
	}
	return out, nil
}

func classificationMap(c Classification) map[string]any {
	out := map[string]any{"class": c.Class}
	switch c.Class {
	case ClassMain:
		out["mainId"] = c.MainId
		out["announcement"] = c.Announcement
	case ClassDelegate, ClassSupervision:
		out["pid"] = c.Pid
	case ClassAdapterSupervisor:
		out["pid"] = c.Pid
		out["jobId"] = c.JobId
	}
	return out
}

// RequireHolder gates a write: it succeeds only for the authenticated holder
// (claiming an unheld checkout for an authenticated main), and reports HUMAN
// and internal-helper callers without re-gating them.
func RequireHolder(root string, callerPid int64, expectedEpoch *int64) (map[string]any, error) {
	root = resolveRoot(root)
	identity, err := Classify(root, callerPid)
	if err != nil {
		return nil, err
	}
	switch identity.Class {
	case ClassHuman:
		return map[string]any{"class": ClassHuman, "holder": true, "claimEpoch": nil, "mainId": nil}, nil
	case ClassDelegate, ClassAdapterSupervisor, ClassSupervision:
		return map[string]any{"class": identity.Class, "holder": false, "claimEpoch": nil, "mainId": nil}, nil
	}
	lease, err := loadLease(root, false)
	if err != nil {
		return nil, err
	}
	if lease == nil {
		if identity.Class != ClassMain {
			return nil, fmt.Errorf("checkout lease is absent and caller pid %d is %s, not an authenticated main", callerPid, identity.Class)
		}
		if err := newClaimer(root).claim(identity.Announcement); err != nil {
			return nil, err
		}
		if lease, err = loadLease(root, true); err != nil {
			return nil, err
		}
	}
	if identity.Class != ClassMain || identity.MainId != lease.HolderMainId {
		return nil, ownedElsewhere(lease, identity)
	}
	if !newClaimer(root).stampComplete(lease) {
		return nil, fmt.Errorf("checkout lease claim sweep is incomplete for the current claim epoch")
	}
	if expectedEpoch != nil && lease.ClaimEpoch != *expectedEpoch {
		return nil, fmt.Errorf("checkout lease claim epoch changed before the final mutation")
	}
	return map[string]any{
		"class": "HOLDER", "holder": true,
		"claimEpoch": lease.ClaimEpoch, "revision": lease.Revision, "mainId": lease.HolderMainId,
	}, nil
}

func ownedElsewhere(lease *Lease, identity Classification) error {
	return fmt.Errorf("OWNED-ELSEWHERE: this checkout is held by %s (caller is %s %s); "+
		"use scripts/agents/second-session.sh for an isolated writer",
		lease.HolderMainId, identity.Class, identity.MainId)
}

// Renew bumps the holder's lease revision and renewal time.
func Renew(root string, callerPid int64) (map[string]any, error) {
	root = resolveRoot(root)
	lock, err := acquireBounded(leasePaths(root).Lock, "renew")
	if err != nil {
		return nil, err
	}
	defer lock.release()
	identity, err := Classify(root, callerPid)
	if err != nil {
		return nil, err
	}
	lease, err := loadLease(root, true)
	if err != nil {
		return nil, err
	}
	if identity.Class != ClassMain || identity.MainId != lease.HolderMainId {
		return nil, fmt.Errorf("checkout lease renewal refused: caller is not the authenticated holder")
	}
	expected := lease.Revision
	lease.RenewedAt = nowStamp()
	lease.Revision++
	current, err := loadLease(root, true)
	if err != nil {
		return nil, err
	}
	if err := verifyRevision(current, &expected); err != nil {
		return nil, err
	}
	if err := saveLease(root, lease); err != nil {
		return nil, err
	}
	return map[string]any{"claimEpoch": lease.ClaimEpoch, "revision": lease.Revision}, nil
}

// RunHeld runs argv while holding the lease lock, gated on the caller being
// the holder (or a HUMAN, who runs ungated). It returns the child's exit code.
func RunHeld(root string, callerPid int64, expectedEpoch *int64, argv []string) (int, error) {
	root = resolveRoot(root)
	identity, err := Classify(root, callerPid)
	if err != nil {
		return 1, err
	}
	if identity.Class == ClassHuman {
		return runProcess(argv)
	}
	lock, err := acquireBounded(leasePaths(root).Lock, "run-held")
	if err != nil {
		return 1, err
	}
	defer lock.release()
	if err := gateHolder(root, identity, expectedEpoch); err != nil {
		return 1, err
	}
	return runProcess(argv)
}

// gateHolder is the run-held authority check: HUMAN and internal helpers pass,
// a MAIN must be the holder at the expected epoch, anyone else is refused.
func gateHolder(root string, identity Classification, expectedEpoch *int64) error {
	switch identity.Class {
	case ClassHuman, ClassDelegate, ClassAdapterSupervisor, ClassSupervision:
		return nil
	}
	lease, err := loadLease(root, true)
	if err != nil {
		return err
	}
	if identity.Class != ClassMain || identity.MainId != lease.HolderMainId {
		return fmt.Errorf("OWNED-ELSEWHERE: this checkout is held by %s; use scripts/agents/second-session.sh for an isolated writer", lease.HolderMainId)
	}
	if expectedEpoch != nil && lease.ClaimEpoch != *expectedEpoch {
		return fmt.Errorf("checkout lease claim epoch changed before the final mutation")
	}
	return nil
}

// ProtocolGrowth reports how many new protocol errors appeared since a main
// last advanced its cursor.
func ProtocolGrowth(root, mainID string) (map[string]any, error) {
	root = resolveRoot(root)
	cursorPath := filepath.Join(root, "artifacts/agents/mains", mainID+".protocol-cursor.json")
	cursor, err := readObject(cursorPath)
	if err != nil || cursor == nil {
		return nil, fmt.Errorf("protocol-error cursor schema is invalid")
	}
	base, _ := cursor["counts"].(map[string]any)
	if id, _ := cursor["mainId"].(string); id != mainID || base == nil {
		return nil, fmt.Errorf("protocol-error cursor schema is invalid")
	}
	counts := protocolCounts(root)
	growth := map[string]int{}
	total := 0
	for chain, count := range counts {
		prev, _ := jsonInt(base[chain])
		if int64(count) > prev {
			growth[chain] = count - int(prev)
			total += growth[chain]
		}
	}
	message := ""
	if total > 0 {
		parts := make([]string, 0, len(growth))
		for _, chain := range sortedKeys(growth) {
			parts = append(parts, fmt.Sprintf("%s=+%d", chain, growth[chain]))
		}
		message = fmt.Sprintf("PROTOCOL-ERRORS: %d new validation error(s) since this main's last report (%s).", total, strings.Join(parts, ", "))
	}
	return map[string]any{"message": message, "counts": counts}, nil
}

// ProtocolAdvance merges a main's reported protocol-error counts into its
// cursor (monotonic max), gated to that main or a HUMAN.
func ProtocolAdvance(root, mainID string, callerPid int64, countsJSON string) error {
	root = resolveRoot(root)
	identity, err := Classify(root, callerPid)
	if err != nil {
		return err
	}
	if identity.Class != ClassHuman && !(identity.Class == ClassMain && identity.MainId == mainID) {
		return fmt.Errorf("a main may advance only its own protocol-error cursor")
	}
	var counts map[string]any
	if json.Unmarshal([]byte(countsJSON), &counts) != nil {
		return fmt.Errorf("protocol-error cursor counts are not JSON")
	}
	for _, v := range counts {
		if _, ok := jsonInt(v); !ok {
			return fmt.Errorf("protocol-error cursor counts are invalid")
		}
	}
	cursorPath := filepath.Join(root, "artifacts/agents/mains", mainID+".protocol-cursor.json")
	lock, err := acquireBounded(cursorPath+".lock", "lease")
	if err != nil {
		return err
	}
	defer lock.release()
	current, err := readObject(cursorPath)
	if err != nil || current == nil {
		return fmt.Errorf("%s is unreadable", cursorPath)
	}
	if id, _ := current["mainId"].(string); id != mainID {
		return fmt.Errorf("protocol-error cursor belongs to another main")
	}
	merged := map[string]any{}
	if base, ok := current["counts"].(map[string]any); ok {
		for k, v := range base {
			merged[k] = v
		}
	}
	for chain, v := range counts {
		count, _ := jsonInt(v)
		prev, _ := jsonInt(merged[chain])
		if count > prev {
			merged[chain] = count
		} else {
			merged[chain] = prev
		}
	}
	return atomicJSON(cursorPath, map[string]any{"mainId": mainID, "counts": merged, "updatedAt": nowStamp()})
}

// initializeCursor writes a fresh protocol-error cursor for a new main,
// baselined at the checkout's current protocol-error counts.
func initializeCursor(root, mainID string) error {
	return atomicJSON(
		filepath.Join(root, "artifacts/agents/mains", mainID+".protocol-cursor.json"),
		map[string]any{"mainId": mainID, "counts": protocolCounts(root), "updatedAt": nowStamp()},
	)
}

var sessionUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeSession(session string) string {
	safe := strings.ToLower(strings.Trim(sessionUnsafe.ReplaceAllString(session, "-"), "-."))
	if safe == "" {
		return "session"
	}
	return safe
}

func tokenHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
