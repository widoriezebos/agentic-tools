package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The steward's per-tick duties, in the runner's order. Each records its
// latest outcome, and health reads one role per duty from that record.
var stewardDuties = []string{
	"disk-sweep", "tick-timing", "question-replies", "unit-boundary", "seat-recovery-requests",
	"notifications-pending", "channel-pending", "cache-trim", "breach-stop-notice",
}

// dutyLabels are the words each duty's failure has always been said with on
// stderr.
var dutyLabels = map[string]string{
	"disk-sweep": "disk sweep", "tick-timing": "tick timing", "question-replies": "question replies",
	"unit-boundary": "unit boundary", "seat-recovery-requests": "seat recovery requests",
	"notifications-pending": "notifications pending", "channel-pending": "channel pending",
	"cache-trim": "cache trim", "breach-stop-notice": "breach-stop notice not recorded",
}

// helmDuties are the duties that still run at the helm, so the helm's
// partial observation reads them.
var helmDuties = []string{"disk-sweep", "question-replies", "tick-timing"}

const dutyRolePrefix = "steward-duty:"

func dutyRole(name string) HealthRole { return HealthRole(dutyRolePrefix + name) }

func isDutyRole(role HealthRole) bool { return strings.HasPrefix(string(role), dutyRolePrefix) }

// dutiesPath is in a folder of its own, beside the runner's other state.
func dutiesPath(top string) string { return filepath.Join(runnerDir(top), "duties", "duties.json") }

// dutyEntry is one duty's latest outcome; an empty Error is a success.
type dutyEntry struct {
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

// unpersistedDuties holds, per checkout, the outcomes whose write failed: a
// failed replacement leaves the old record readable, so health reads these
// over it until a write succeeds.
var unpersistedDuties = struct {
	sync.Mutex
	byTop map[string]map[string]dutyEntry
}{byTop: map[string]map[string]dutyEntry{}}

func dutyLine(name string, err error) string { return fmt.Sprintf("%s: %v", dutyLabels[name], err) }

// recordDuty records one duty's outcome and says a failure on stderr, as the
// duty always did. It never stops the loop.
func recordDuty(top, name string, err error, now time.Time) {
	entry := dutyEntry{At: now.UTC()}
	if err != nil {
		entry.Error = err.Error()
		fmt.Fprintln(os.Stderr, dutyLine(name, err))
	}
	unpersistedDuties.Lock()
	defer unpersistedDuties.Unlock()
	record, readErr := loadDuties(top)
	if readErr != nil {
		record = map[string]dutyEntry{}
	}
	held := unpersistedDuties.byTop[top]
	if held == nil {
		held = map[string]dutyEntry{}
	}
	held[name] = entry
	for duty, kept := range held {
		record[duty] = kept
	}
	if writeErr := writeJSONAtomic(dutiesPath(top), record); writeErr != nil {
		unpersistedDuties.byTop[top] = held
		fmt.Fprintf(os.Stderr, "steward duties not recorded: %v\n", writeErr)
		return
	}
	delete(unpersistedDuties.byTop, top)
}

// loadDuties reads the record; a missing one is empty.
func loadDuties(top string) (map[string]dutyEntry, error) {
	record := map[string]dutyEntry{}
	data, err := os.ReadFile(dutiesPath(top))
	if os.IsNotExist(err) {
		return record, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &record)
	}
	return record, err
}

// checkDuties is one verdict per duty: the record with the outcomes held in
// memory laid over it. A missing record means no duty has run; an unreadable
// one, with nothing in memory, leaves every duty unknown.
func checkDuties(top string) []RoleVerdict {
	record, readErr := loadDuties(top)
	unpersistedDuties.Lock()
	held := unpersistedDuties.byTop[top]
	for name, entry := range held {
		record[name] = entry
	}
	unpersistedDuties.Unlock()
	verdicts := make([]RoleVerdict, 0, len(stewardDuties))
	for _, name := range stewardDuties {
		role := dutyRole(name)
		entry, ran := record[name]
		switch {
		case readErr != nil && len(held) == 0:
			verdicts = append(verdicts, roleUnknown(role, "the steward duties record is unreadable: "+readErr.Error(), "", RemedyFact{Cause: CauseUnreadable}))
		case ran && entry.Error != "":
			verdicts = append(verdicts, roleDead(role, name+" failed: "+entry.Error, "", RemedyFact{Cause: CauseUnavailable}))
		case ran:
			verdicts = append(verdicts, roleAlive(role, name+" ran without an error"))
		default:
			verdicts = append(verdicts, roleAlive(role, name+" has not run"))
		}
	}
	return verdicts
}
