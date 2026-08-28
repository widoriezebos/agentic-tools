package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const sessionOccupancySchemaVersion = 1
const claimOccupancyPreparationSchemaVersion = 1

// SessionOccupancyReader owns preparation, revalidation, and publication for
// one session. Preparation may inspect the whole registry. Resolve holds only
// the per-session lock and re-reads the bounded set named by the current index.
type SessionOccupancyReader interface {
	Prepare(root, sessionKey string) (SessionOccupancyPreparation, error)
	Resolve(root, sessionKey, excludingOpID string, prepared SessionOccupancyPreparation, decide func(SessionOccupancy, *SessionIndexTransaction) error) error
}

type SessionOccupancy struct {
	Busy         *SessionOccupant
	Unprovable   *SessionOccupant
	FreeEvidence []SessionOccupant
	Healing      *SessionOccupancyHealing
}

type SessionOccupant struct {
	OpID       string `json:"opid"`
	Status     string `json:"status"`
	ProofLevel string `json:"proofLevel,omitempty"`
	Reason     string `json:"reason"`
}

// SessionOccupancyHealing records how fallback evidence was treated. Applied
// means this claimant published a rebuilt index. A generation-race resolution
// means it discarded the scan and decided from the newer index instead.
type SessionOccupancyHealing struct {
	Trigger             string `json:"trigger"`
	Resolution          string `json:"resolution"`
	Applied             bool   `json:"applied"`
	ExpectedGeneration  int64  `json:"expectedGeneration"`
	ObservedGeneration  int64  `json:"observedGeneration"`
	PublishedGeneration int64  `json:"publishedGeneration,omitempty"`
	RecordsRead         int    `json:"recordsRead"`
}

type sessionOccupancyIndex struct {
	SessionKey string
	Generation int64
	Occupants  []SessionOccupant
}

type sessionIndexWitness struct {
	Exists     bool
	Valid      bool
	Generation int64
	Digest     string
}

type sessionRegistryScan struct {
	Occupants    []SessionOccupant
	FreeEvidence []SessionOccupant
	Unprovable   *SessionOccupant
	RecordsRead  int
}

// SessionOccupancyPreparation carries either a valid index witness or the one
// fallback registry scan needed to repair it. The generation check consumes
// this value while the per-session lock is held.
type SessionOccupancyPreparation struct {
	witness  sessionIndexWitness
	scan     *sessionRegistryScan
	trigger  string
	recovery string
}

type claimOccupancyPreparationDocument struct {
	SchemaVersion int                  `json:"schemaVersion"`
	SessionKey    string               `json:"sessionKey"`
	Witness       sessionIndexWitness  `json:"witness"`
	Scan          *sessionRegistryScan `json:"scan,omitempty"`
	Trigger       string               `json:"trigger,omitempty"`
	Recovery      string               `json:"recovery,omitempty"`
}

// IndexedSessionOccupancyReader is the production occupancy owner. The hooks
// are empty in production and let concurrency fixtures stop at the two sides
// of the off-lock/on-lock boundary.
type IndexedSessionOccupancyReader struct {
	BeforeRegistryScan    func()
	BeforeGenerationCheck func()
}

// WriteClaimOccupancyPreparation performs the only potentially registry-wide
// launch scan and writes its generation witness to a transient hand-off. The
// caller does this before taking cap authority; claim-launch later consumes the
// witness and revalidates it under the session lock.
func WriteClaimOccupancyPreparation(root, sessionKey, output string) error {
	if root == "" || sessionKey == "" || output == "" {
		return fmt.Errorf("claim occupancy preparation requires root, session, and output")
	}
	prepared, err := (IndexedSessionOccupancyReader{}).Prepare(root, sessionKey)
	if err != nil {
		return err
	}
	document := claimOccupancyPreparationDocument{
		SchemaVersion: claimOccupancyPreparationSchemaVersion,
		SessionKey:    sessionKey,
		Witness:       prepared.witness,
		Scan:          prepared.scan,
		Trigger:       prepared.trigger,
		Recovery:      prepared.recovery,
	}
	return writeCompactJSON(output, document)
}

// ReadClaimOccupancyPreparation accepts only a hand-off for this exact
// session. Its generation witness remains advisory until Resolve compares it
// with the live index under the per-session lock.
func ReadClaimOccupancyPreparation(path, sessionKey string) (SessionOccupancyPreparation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionOccupancyPreparation{}, fmt.Errorf("cannot read claim occupancy preparation: %w", err)
	}
	var document claimOccupancyPreparationDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return SessionOccupancyPreparation{}, fmt.Errorf("cannot decode claim occupancy preparation: %w", err)
	}
	if document.SchemaVersion != claimOccupancyPreparationSchemaVersion || document.SessionKey == "" || document.SessionKey != sessionKey {
		return SessionOccupancyPreparation{}, fmt.Errorf("claim occupancy preparation does not match session %s", sessionKey)
	}
	if document.Scan == nil {
		if document.Trigger != "" || document.Recovery != "" {
			return SessionOccupancyPreparation{}, fmt.Errorf("claim occupancy preparation has recovery labels without a scan")
		}
	} else {
		validTrigger := document.Trigger == "index-missing" || document.Trigger == "index-unreadable" || document.Trigger == "index-stale"
		validRecovery := document.Recovery == "registry-fallback" || document.Recovery == "session-record-recovery"
		if !validTrigger || !validRecovery {
			return SessionOccupancyPreparation{}, fmt.Errorf("claim occupancy preparation has invalid recovery labels")
		}
	}
	return SessionOccupancyPreparation{
		witness:  document.Witness,
		scan:     document.Scan,
		trigger:  document.Trigger,
		recovery: document.Recovery,
	}, nil
}

// SessionIndexTransaction publishes index changes while Resolve holds the
// session's flock. Record creation writes a busy entry through this value
// before publishing the record.
type SessionIndexTransaction struct {
	indexPath string
	index     sessionOccupancyIndex
	disabled  bool
}

func sessionOccupancyPaths(root, sessionKey string) (storageKey, indexPath, lockPath string) {
	digest := sha256.Sum256([]byte(sessionKey))
	storageKey = hex.EncodeToString(digest[:])
	agents := filepath.Join(root, "artifacts", "agents")
	indexPath = filepath.Join(agents, "sessions", storageKey+".json")
	lockPath = filepath.Join(agents, "session-locks", storageKey+".lock")
	return
}

func withSessionOccupancyLock(root, sessionKey string, fn func(indexPath string) error) error {
	_, indexPath, lockPath := sessionOccupancyPaths(root, sessionKey)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	handle, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open session lock for %s: %w", sessionKey, err)
	}
	defer handle.Close()
	deadline := time.Now().Add(recordLockWait())
	for {
		err = unix.Flock(int(handle.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("session lock for %s is busy after %s; a wedged holder keeps it", sessionKey, recordLockWait())
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer unix.Flock(int(handle.Fd()), unix.LOCK_UN)
	return fn(indexPath)
}

func (reader IndexedSessionOccupancyReader) Prepare(root, sessionKey string) (SessionOccupancyPreparation, error) {
	index, witness, err := readSessionOccupancyIndexWitness(root, sessionKey)
	if err == nil {
		recovered, stale, _ := recoverSessionOccupancyIndex(root, index)
		if !stale {
			return SessionOccupancyPreparation{witness: witness}, nil
		}
		return SessionOccupancyPreparation{
			witness:  witness,
			scan:     &recovered,
			trigger:  "index-stale",
			recovery: "session-record-recovery",
		}, nil
	}
	trigger := "index-missing"
	if witness.Exists && !witness.Valid {
		trigger = "index-unreadable"
	}
	if reader.BeforeRegistryScan != nil {
		reader.BeforeRegistryScan()
	}
	scan := scanSessionRegistry(root, sessionKey)
	return SessionOccupancyPreparation{
		witness:  witness,
		scan:     &scan,
		trigger:  trigger,
		recovery: "registry-fallback",
	}, nil
}

func (reader IndexedSessionOccupancyReader) Resolve(root, sessionKey, excludingOpID string, prepared SessionOccupancyPreparation, decide func(SessionOccupancy, *SessionIndexTransaction) error) error {
	if reader.BeforeGenerationCheck != nil {
		reader.BeforeGenerationCheck()
	}
	return withSessionOccupancyLock(root, sessionKey, func(indexPath string) error {
		current, currentWitness, currentErr := readSessionOccupancyIndexWitness(root, sessionKey)
		sameGeneration := sameSessionIndexWitness(prepared.witness, currentWitness)
		if prepared.scan != nil && sameGeneration {
			scan := *prepared.scan
			if prepared.recovery == "session-record-recovery" && currentErr == nil {
				rechecked, stale, _ := recoverSessionOccupancyIndex(root, current)
				scan.RecordsRead += rechecked.RecordsRead
				if !stale {
					healing := &SessionOccupancyHealing{
						Trigger:            prepared.trigger,
						Resolution:         "session-record-recovery-revalidated-current",
						ExpectedGeneration: prepared.witness.Generation,
						ObservedGeneration: currentWitness.Generation,
						RecordsRead:        scan.RecordsRead,
					}
					occupancy := occupancyFromSessionIndex(current, excludingOpID)
					occupancy.Healing = healing
					return decide(occupancy, &SessionIndexTransaction{indexPath: indexPath, index: current})
				}
				scan = rechecked
				scan.RecordsRead += prepared.scan.RecordsRead
			}
			healing := SessionOccupancyHealing{
				Trigger:            prepared.trigger,
				ExpectedGeneration: prepared.witness.Generation,
				ObservedGeneration: currentWitness.Generation,
				RecordsRead:        scan.RecordsRead,
			}
			if scan.Unprovable != nil {
				healing.Resolution = prepared.recovery + "-unprovable"
				occupancy := SessionOccupancy{Unprovable: scan.Unprovable, Healing: &healing}
				return decide(occupancy, &SessionIndexTransaction{disabled: true})
			}
			generation := int64(1)
			if currentWitness.Valid {
				generation = currentWitness.Generation + 1
			}
			rebuilt := sessionOccupancyIndex{
				SessionKey: sessionKey,
				Generation: generation,
				Occupants:  append([]SessionOccupant(nil), scan.Occupants...),
			}
			if err := writeSessionOccupancyIndex(indexPath, rebuilt); err != nil {
				return err
			}
			healing.Resolution = prepared.recovery + "-applied"
			healing.Applied = true
			healing.PublishedGeneration = generation
			occupancy := occupancyFromSessionIndex(rebuilt, excludingOpID)
			occupancy.FreeEvidence = append([]SessionOccupant(nil), scan.FreeEvidence...)
			occupancy.Healing = &healing
			return decide(occupancy, &SessionIndexTransaction{indexPath: indexPath, index: rebuilt})
		}

		var healing *SessionOccupancyHealing
		if prepared.scan != nil {
			healing = &SessionOccupancyHealing{
				Trigger:            prepared.trigger,
				Resolution:         "generation-changed-index-reread",
				ExpectedGeneration: prepared.witness.Generation,
				ObservedGeneration: currentWitness.Generation,
				RecordsRead:        prepared.scan.RecordsRead,
			}
		}
		if currentErr != nil {
			occupancy := SessionOccupancy{
				Unprovable: &SessionOccupant{Reason: "index-unreadable-after-generation-check"},
				Healing:    healing,
			}
			return decide(occupancy, &SessionIndexTransaction{disabled: true})
		}
		recovered, stale, _ := recoverSessionOccupancyIndex(root, current)
		if stale && recovered.Unprovable != nil {
			occupancy := SessionOccupancy{
				Unprovable: recovered.Unprovable,
				Healing:    healing,
			}
			return decide(occupancy, &SessionIndexTransaction{disabled: true})
		}
		if stale {
			current.Generation++
			current.Occupants = recovered.Occupants
			if err := writeSessionOccupancyIndex(indexPath, current); err != nil {
				return err
			}
			healing = &SessionOccupancyHealing{
				Trigger:             "index-became-stale",
				Resolution:          "session-record-recovery-applied",
				Applied:             true,
				ExpectedGeneration:  prepared.witness.Generation,
				ObservedGeneration:  currentWitness.Generation,
				PublishedGeneration: current.Generation,
				RecordsRead:         recovered.RecordsRead,
			}
			occupancy := occupancyFromSessionIndex(current, excludingOpID)
			occupancy.FreeEvidence = recovered.FreeEvidence
			occupancy.Healing = healing
			return decide(occupancy, &SessionIndexTransaction{indexPath: indexPath, index: current})
		}
		occupancy := occupancyFromSessionIndex(current, excludingOpID)
		occupancy.Healing = healing
		return decide(occupancy, &SessionIndexTransaction{indexPath: indexPath, index: current})
	})
}

func sameSessionIndexWitness(left, right sessionIndexWitness) bool {
	if left.Exists != right.Exists || left.Valid != right.Valid {
		return false
	}
	if !left.Exists {
		return true
	}
	if left.Valid {
		return left.Generation == right.Generation
	}
	return left.Digest == right.Digest
}

func readSessionOccupancyIndex(root, sessionKey string) (sessionOccupancyIndex, error) {
	_, indexPath, _ := sessionOccupancyPaths(root, sessionKey)
	index, _, err := readSessionOccupancyIndexPath(indexPath, sessionKey)
	return index, err
}

func readSessionOccupancyIndexWitness(root, sessionKey string) (sessionOccupancyIndex, sessionIndexWitness, error) {
	_, indexPath, _ := sessionOccupancyPaths(root, sessionKey)
	return readSessionOccupancyIndexPath(indexPath, sessionKey)
}

func readSessionOccupancyIndexPath(indexPath, sessionKey string) (sessionOccupancyIndex, sessionIndexWitness, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return sessionOccupancyIndex{}, sessionIndexWitness{Exists: !os.IsNotExist(err)}, err
	}
	digest := sha256.Sum256(data)
	witness := sessionIndexWitness{Exists: true, Digest: hex.EncodeToString(digest[:])}
	record, err := readObject(indexPath)
	if err != nil {
		return sessionOccupancyIndex{}, witness, err
	}
	schema, schemaOK := numInt(record["schemaVersion"])
	generation, generationOK := numInt(record["generation"])
	items, itemsOK := record["occupants"].([]any)
	if !schemaOK || schema != sessionOccupancySchemaVersion || asString(record["sessionKey"]) != sessionKey || !generationOK || generation < 1 || !itemsOK {
		return sessionOccupancyIndex{}, witness, fmt.Errorf("invalid session occupancy index")
	}
	occupants := make([]SessionOccupant, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return sessionOccupancyIndex{}, witness, fmt.Errorf("invalid session occupancy entry")
		}
		occupant := SessionOccupant{
			OpID:       asString(entry["opid"]),
			Status:     asString(entry["status"]),
			ProofLevel: asString(entry["proofLevel"]),
			Reason:     asString(entry["reason"]),
		}
		if !validJobID.MatchString(occupant.OpID) || occupant.Reason == "" || seen[occupant.OpID] {
			return sessionOccupancyIndex{}, witness, fmt.Errorf("invalid session occupancy entry")
		}
		seen[occupant.OpID] = true
		occupants = append(occupants, occupant)
	}
	sortSessionOccupants(occupants)
	witness.Valid = true
	witness.Generation = generation
	return sessionOccupancyIndex{SessionKey: sessionKey, Generation: generation, Occupants: occupants}, witness, nil
}

func writeSessionOccupancyIndex(indexPath string, index sessionOccupancyIndex) error {
	if index.SessionKey == "" || index.Generation < 1 {
		return fmt.Errorf("session occupancy index requires a key and positive generation")
	}
	occupants := append([]SessionOccupant(nil), index.Occupants...)
	sortSessionOccupants(occupants)
	items := make([]any, 0, len(occupants))
	for _, occupant := range occupants {
		items = append(items, sessionOccupantObject(occupant))
	}
	record := map[string]any{
		"schemaVersion": sessionOccupancySchemaVersion,
		"sessionKey":    index.SessionKey,
		"generation":    index.Generation,
		"occupants":     items,
	}
	return writeRecord(indexPath, record)
}

func sessionOccupantObject(occupant SessionOccupant) map[string]any {
	return map[string]any{
		"opid":       occupant.OpID,
		"status":     occupant.Status,
		"proofLevel": occupant.ProofLevel,
		"reason":     occupant.Reason,
	}
}

func sortSessionOccupants(occupants []SessionOccupant) {
	sort.Slice(occupants, func(left, right int) bool { return occupants[left].OpID < occupants[right].OpID })
}

func recoverSessionOccupancyIndex(root string, index sessionOccupancyIndex) (sessionRegistryScan, bool, string) {
	recovered := sessionRegistryScan{}
	stale := false
	reason := ""
	for _, indexed := range index.Occupants {
		recovered.RecordsRead++
		recordPath := filepath.Join(root, "artifacts", "agents", "jobs", indexed.OpID+".json")
		record, err := readObject(recordPath)
		if os.IsNotExist(err) {
			stale = true
			reason = "indexed-record-missing"
			continue
		}
		if err != nil {
			recovered.Unprovable = &SessionOccupant{OpID: indexed.OpID, Reason: "indexed-record-unreadable"}
			return recovered, true, "indexed-record-unreadable"
		}
		if asString(record["jobId"]) != indexed.OpID || asString(record["sessionKey"]) != index.SessionKey {
			recovered.Unprovable = &SessionOccupant{OpID: indexed.OpID, Reason: "indexed-record-identity-mismatch"}
			return recovered, true, "indexed-record-identity-mismatch"
		}
		observed := classifySessionRecord(indexed.OpID, record)
		if observed != indexed {
			stale = true
			reason = "indexed-record-state-mismatch"
		}
		if sessionIndexTracks(observed) {
			recovered.Occupants = append(recovered.Occupants, observed)
		} else if sessionRecordIsFreeEvidence(observed) {
			recovered.FreeEvidence = append(recovered.FreeEvidence, observed)
		}
	}
	sortSessionOccupants(recovered.Occupants)
	sortSessionOccupants(recovered.FreeEvidence)
	return recovered, stale, reason
}

func scanSessionRegistry(root, sessionKey string) sessionRegistryScan {
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobs)
	if os.IsNotExist(err) {
		return sessionRegistryScan{}
	}
	if err != nil {
		return sessionRegistryScan{Unprovable: &SessionOccupant{Reason: "registry-unreadable"}}
	}
	result := sessionRegistryScan{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		result.RecordsRead++
		opid := strings.TrimSuffix(entry.Name(), ".json")
		record, readErr := readObject(filepath.Join(jobs, entry.Name()))
		if readErr != nil {
			result.Unprovable = &SessionOccupant{OpID: opid, Reason: "record-unreadable-during-fallback"}
			return result
		}
		if asString(record["sessionKey"]) != sessionKey {
			continue
		}
		if recorded := asString(record["jobId"]); recorded != "" {
			opid = recorded
		}
		occupant := classifySessionRecord(opid, record)
		if sessionIndexTracks(occupant) {
			result.Occupants = append(result.Occupants, occupant)
			continue
		}
		if sessionRecordIsFreeEvidence(occupant) {
			result.FreeEvidence = append(result.FreeEvidence, occupant)
		}
	}
	sortSessionOccupants(result.Occupants)
	sortSessionOccupants(result.FreeEvidence)
	return result
}

func occupancyFromSessionIndex(index sessionOccupancyIndex, excludingOpID string) SessionOccupancy {
	result := SessionOccupancy{}
	for _, occupant := range index.Occupants {
		if occupant.OpID == excludingOpID {
			continue
		}
		if sessionRecordIsBusy(occupant) {
			if result.Busy == nil {
				copy := occupant
				result.Busy = &copy
			}
			continue
		}
		if result.Unprovable == nil {
			copy := occupant
			result.Unprovable = &copy
		}
	}
	return result
}

func sessionIndexTracks(occupant SessionOccupant) bool {
	return sessionRecordIsBusy(occupant) || !sessionRecordIsFree(occupant)
}

func sessionRecordIsBusy(occupant SessionOccupant) bool {
	switch occupant.Reason {
	case "reservation", "custodial-liveness-indeterminate", "live-seam-observation":
		return true
	default:
		return false
	}
}

func sessionRecordIsFreeEvidence(occupant SessionOccupant) bool {
	switch occupant.Reason {
	case "opaque-does-not-prove-occupancy", "seam-self-report-proven-ended":
		return true
	default:
		return false
	}
}

func sessionRecordIsFree(occupant SessionOccupant) bool {
	if sessionRecordIsFreeEvidence(occupant) {
		return true
	}
	switch occupant.Reason {
	case "terminal", "reconciled-proven-absent", "seam-archived":
		return true
	default:
		return false
	}
}

func (transaction *SessionIndexTransaction) publishBusy(occupant SessionOccupant) (int64, error) {
	if transaction == nil || transaction.disabled {
		return 0, nil
	}
	transaction.index.Occupants = []SessionOccupant{occupant}
	transaction.index.Generation++
	if err := writeSessionOccupancyIndex(transaction.indexPath, transaction.index); err != nil {
		return 0, err
	}
	return transaction.index.Generation, nil
}

func (transaction *SessionIndexTransaction) syncRecord(opid string, record map[string]any) error {
	if transaction == nil || transaction.disabled {
		return nil
	}
	if asString(record["sessionKey"]) != transaction.index.SessionKey {
		return fmt.Errorf("job record %s does not match its session occupancy transaction", opid)
	}
	observed := classifySessionRecord(opid, record)
	next := make([]SessionOccupant, 0, len(transaction.index.Occupants)+1)
	changed := false
	found := false
	for _, occupant := range transaction.index.Occupants {
		if occupant.OpID != opid {
			next = append(next, occupant)
			continue
		}
		found = true
		if sessionIndexTracks(observed) {
			next = append(next, observed)
			changed = occupant != observed
		} else {
			changed = true
		}
	}
	if !found && sessionIndexTracks(observed) {
		next = append(next, observed)
		changed = true
	}
	if !changed {
		return nil
	}
	transaction.index.Occupants = next
	transaction.index.Generation++
	return writeSessionOccupancyIndex(transaction.indexPath, transaction.index)
}

func healingObject(healing *SessionOccupancyHealing) any {
	if healing == nil {
		return nil
	}
	result := map[string]any{
		"trigger":            healing.Trigger,
		"resolution":         healing.Resolution,
		"applied":            healing.Applied,
		"expectedGeneration": healing.ExpectedGeneration,
		"observedGeneration": healing.ObservedGeneration,
		"recordsRead":        healing.RecordsRead,
	}
	if healing.PublishedGeneration > 0 {
		result["publishedGeneration"] = healing.PublishedGeneration
	}
	return result
}

func classifySessionRecord(opid string, record map[string]any) SessionOccupant {
	status := asString(record["status"])
	proofLevel := asString(record["proofLevel"])
	occupant := SessionOccupant{OpID: opid, Status: status, ProofLevel: proofLevel}
	if TerminalStatus(status) {
		if proofLevel == "seam" {
			occupant.Reason = "invalid-seam-terminal-status"
		} else {
			occupant.Reason = "terminal"
		}
		return occupant
	}
	switch status {
	case "reconciled-proven-absent":
		if proofLevel != "proven" {
			occupant.Reason = "invalid-reconciliation-proof-level"
		} else {
			occupant.Reason = "reconciled-proven-absent"
		}
	case "seam-archived":
		if proofLevel != "seam" {
			occupant.Reason = "invalid-seam-proof-level"
		} else {
			occupant.Reason = "seam-archived"
		}
	case "seam-opaque":
		if proofLevel != "seam" {
			occupant.Reason = "invalid-seam-proof-level"
		} else {
			occupant.Reason = "opaque-does-not-prove-occupancy"
		}
	case "observing", "seam-stalled":
		if proofLevel != "seam" {
			occupant.Reason = "invalid-seam-proof-level"
		} else if seamSelfReportProvesEnded(record["selfReport"]) {
			occupant.Reason = "seam-self-report-proven-ended"
		} else {
			occupant.Reason = "live-seam-observation"
		}
	case "pending-setup", "pending":
		occupant.Reason = "reservation"
	case "running":
		if proofLevel == "proven" {
			occupant.Reason = "custodial-liveness-indeterminate"
		} else if proofLevel == "" {
			occupant.Reason = "legacy-proof-level"
		} else {
			occupant.Reason = "invalid-custodial-proof-level"
		}
	default:
		occupant.Reason = fmt.Sprintf("unknown-nonterminal-status:%s", status)
	}
	return occupant
}

func seamSelfReportProvesEnded(value any) bool {
	report, ok := value.(map[string]any)
	if !ok {
		return false
	}
	return TerminalStatus(asString(report["status"]))
}
