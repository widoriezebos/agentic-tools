package mission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// A mission's lifecycle fences bound how much work it may do — wall-clock
// hours, cycles, jobs, concurrency, and per-job minutes — against the signed
// contract. This file serializes fence checks and reservations under a lock,
// authorizes per-pair caps, batches the human ask a tripped fence raises, and
// aggregates typed usage across the mission's finished jobs.

// clock is the time source, overridable in tests.
var clock = time.Now

func nowUTC() time.Time   { return clock().UTC() }
func fenceNowISO() string { return nowUTC().Format("2006-01-02T15:04:05Z") }

var terminalJobStatus = map[string]bool{
	"completed": true, "failed": true, "timeout": true, "cancelled": true,
}

var requiredFenceKeys = []string{
	"fence.wall-clock-hours", "fence.cycles", "fence.jobs", "fence.concurrency", "fence.job-cap-min",
}

var (
	positiveIntRe = regexp.MustCompile(`^[1-9][0-9]*$`)
	sha256HexRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	fenceBoundRe  = regexp.MustCompile(`^fence-bound(?:-([1-9][0-9]*))?\.json$`)
	askReasonRe   = regexp.MustCompile("`([a-z-]+)`")
)

func missionDir(repo, mission string) string {
	return filepath.Join(repo, "artifacts", "agents", "missions", mission)
}

func fencePaths(repo, mission string) (dir, fences, lock string) {
	dir = missionDir(repo, mission)
	return dir, filepath.Join(dir, "fences.json"), filepath.Join(dir, "mission-fence.lock")
}

func readJSONObjectFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func parseTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable timestamp %q", s)
}

// contractValuesFromBytes parses and validates a mission contract's authored
// block: the universal fence keys must be present and well formed, and each
// per-pair cap key must be canonical for its runtime and model.
func contractValuesFromBytes(data []byte) (map[string]string, error) {
	if !isValidUTF8(data) {
		return nil, fmt.Errorf("mission contract is not UTF-8")
	}
	blocks := contract.AuthoredBlocks(string(data))
	if len(blocks) != 1 {
		return nil, fmt.Errorf("mission contract does not have exactly one authored block")
	}
	values, err := contract.ParseAuthoredValues(blocks[0][1], "mission contract")
	if err != nil {
		return nil, fmt.Errorf("mission contract key/value grammar is invalid: %v", err)
	}
	for _, key := range requiredFenceKeys {
		if _, ok := values[key]; !ok {
			return nil, fmt.Errorf("mission contract lacks a universal lifecycle fence")
		}
	}
	wall, err := strconv.ParseFloat(values["fence.wall-clock-hours"], 64)
	if err != nil || !isFinite(wall) || wall <= 0 {
		return nil, fmt.Errorf("mission wall-clock fence is invalid")
	}
	for _, key := range requiredFenceKeys {
		if key == "fence.wall-clock-hours" {
			continue
		}
		if !positiveIntRe.MatchString(values[key]) {
			return nil, fmt.Errorf("mission %s is invalid", key)
		}
	}
	for key, value := range values {
		if !strings.HasPrefix(key, "cap.min.") {
			continue
		}
		// Signed-exposure policy has ONE home (review mission-contract-5):
		// the seal-time check and this runtime fence check can no longer
		// disagree about what a canonical cap key is.
		if err := contract.ValidatePairCap(key, value); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// verifiedContractValues reads the live contract, checks its raw-file sha256
// against the approved digest recorded in the fences, and parses it.
func verifiedContractValues(repo, mission string, fences map[string]any) (map[string]string, error) {
	approved, _ := fences["approvedContractSha256"].(string)
	if !sha256HexRe.MatchString(approved) {
		return nil, fmt.Errorf("mission fence refused: approvedContractSha256 is absent or invalid")
	}
	path := filepath.Join(repo, "plans", fmt.Sprintf("mission-%s.contract.md", mission))
	snapshot, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mission contract is unreadable: %v", err)
	}
	if sha256Hex(string(snapshot)) != approved {
		return nil, fmt.Errorf("mission fence refused: live contract raw-file sha256 does not match approvedContractSha256")
	}
	return contractValuesFromBytes(snapshot)
}

// loadFences reads the mission's fence counters, seeding them from the lease
// start time on first use, and validates their shape.
func loadFences(repo, mission string) (map[string]any, error) {
	dir, path, _ := fencePaths(repo, mission)
	var value map[string]any
	if _, err := os.Stat(path); err == nil {
		value, err = readJSONObjectFile(path)
		if err != nil {
			return nil, fmt.Errorf("mission fence counters are unreadable: %v", err)
		}
	} else {
		started := fenceNowISO()
		leasePath := filepath.Join(dir, "lease.json")
		if _, err := os.Stat(leasePath); err == nil {
			lease, err := readJSONObjectFile(leasePath)
			if err != nil {
				return nil, fmt.Errorf("mission lease start time is invalid")
			}
			s, ok := lease["startedAt"].(string)
			if !ok {
				return nil, fmt.Errorf("mission lease start time is invalid")
			}
			if _, err := parseTimestamp(s); err != nil {
				return nil, fmt.Errorf("mission lease start time is invalid")
			}
			started = s
		}
		value = map[string]any{
			"schemaVersion": 1, "missionId": mission, "startedAt": started,
			"cycles": 0, "reservations": map[string]any{},
		}
	}
	if sv, _ := intValue(value["schemaVersion"]); sv != 1 {
		return nil, fmt.Errorf("mission fence counters have an invalid shape")
	}
	if id, _ := value["missionId"].(string); id != mission {
		return nil, fmt.Errorf("mission fence counters have an invalid shape")
	}
	if !isNonnegInt(value["cycles"]) {
		return nil, fmt.Errorf("mission fence counters have an invalid shape")
	}
	if _, ok := value["reservations"].(map[string]any); !ok {
		return nil, fmt.Errorf("mission fence counters have an invalid shape")
	}
	startedStr, _ := value["startedAt"].(string)
	started, err := parseTimestamp(startedStr)
	if err != nil {
		return nil, fmt.Errorf("mission fence start time is invalid")
	}
	if started.Sub(nowUTC()).Seconds() > 5 {
		return nil, fmt.Errorf("mission fence start time is in the future")
	}
	return value, nil
}

func jobStatus(repo, job string) string {
	value, err := readJSONObjectFile(filepath.Join(repo, "artifacts", "agents", "jobs", job+".json"))
	if err != nil {
		return ""
	}
	s, _ := value["status"].(string)
	return s
}

func reservationsMap(fences map[string]any) map[string]any {
	r, _ := fences["reservations"].(map[string]any)
	return r
}

func activeReservations(repo string, fences map[string]any) []string {
	var out []string
	for job := range reservationsMap(fences) {
		if !terminalJobStatus[jobStatus(repo, job)] {
			out = append(out, job)
		}
	}
	sort.Strings(out)
	return out
}

// violations lists the fences a proposed action would breach. reserve selects
// which fences apply: "cycle" checks only wall-clock and cycles; "job" and
// "authorized-job" add jobs and concurrency; "job" also checks the per-job cap.
func violations(repo string, values map[string]string, fences map[string]any, capMin *int, reserve string) []string {
	var result []string
	started, _ := parseTimestamp(fences["startedAt"].(string))
	elapsedHours := nowUTC().Sub(started).Seconds() / 3600
	wall, _ := strconv.ParseFloat(values["fence.wall-clock-hours"], 64)
	if elapsedHours >= wall {
		result = append(result, "wall-clock-hours")
	}
	cycles, _ := intValue(fences["cycles"])
	if cycles >= atoiOr(values["fence.cycles"]) {
		result = append(result, "cycles")
	}
	if reserve == "job" || reserve == "authorized-job" {
		if int64(len(reservationsMap(fences))) >= int64(atoiOr(values["fence.jobs"])) {
			result = append(result, "jobs")
		}
		if int64(len(activeReservations(repo, fences))) >= int64(atoiOr(values["fence.concurrency"])) {
			result = append(result, "concurrency")
		}
	}
	if reserve == "job" {
		if capMin == nil || int64(*capMin) > int64(atoiOr(values["fence.job-cap-min"])) {
			result = append(result, "job-cap-min")
		}
	}
	return result
}

func atoiOr(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// fenceRefusal words one fence refusal. The batched ask is the DESIGNED
// recovery channel for a tripped fence, so a failed ask write must ride
// the refusal loudly: a mission parked on a fence whose ask never landed
// is otherwise indistinguishable from one waiting on a human, and waits
// forever (review mission-contract-6).
func fenceRefusal(what string, found []string, ask string, askErr error) error {
	if askErr != nil {
		return fmt.Errorf("mission fence refused %s (%s); FAILED to write batched ask: %v", what, strings.Join(found, ", "), askErr)
	}
	return fmt.Errorf("mission fence refused %s (%s); batched ask written: %s", what, strings.Join(found, ", "), ask)
}

// writeBatchedAsk raises (or extends) the single open fence ask, combining its
// reasons, and returns the ask path.
func writeBatchedAsk(repo, mission string, reasons []string) (string, error) {
	asks := filepath.Join(missionDir(repo, mission), "asks")
	if err := os.MkdirAll(asks, 0o755); err != nil {
		return "", err
	}
	entries, _ := filepath.Glob(filepath.Join(asks, "fence-bound*.json"))

	type openAsk struct {
		index int
		path  string
		value map[string]any
	}
	var open []openAsk
	maxIndex := 0
	for _, path := range entries {
		match := fenceBoundRe.FindStringSubmatch(filepath.Base(path))
		if match == nil {
			continue
		}
		index := 1
		if match[1] != "" {
			index, _ = strconv.Atoi(match[1])
		}
		if index > maxIndex {
			maxIndex = index
		}
		value, err := readJSONObjectFile(path)
		if err != nil {
			continue
		}
		if value["answeredAt"] == nil {
			open = append(open, openAsk{index: index, path: path, value: value})
		}
	}

	var path string
	var value map[string]any
	var combined []string
	if len(open) > 0 {
		best := open[0]
		for _, o := range open[1:] {
			if o.index > best.index || (o.index == best.index && o.path > best.path) {
				best = o
			}
		}
		path, value = best.path, best.value
		question, _ := value["question"].(string)
		set := map[string]bool{}
		for _, m := range askReasonRe.FindAllStringSubmatch(question, -1) {
			set[m[1]] = true
		}
		for _, r := range reasons {
			set[r] = true
		}
		combined = sortedKeysOfSet(set)
	} else {
		index := maxIndex + 1
		askID := "fence-bound"
		if index != 1 {
			askID = fmt.Sprintf("fence-bound-%d", index)
		}
		path = filepath.Join(asks, askID+".json")
		value = map[string]any{
			"askId": askID, "streamId": nil, "reasonClass": "fence",
			"question": "", "createdAt": fenceNowISO(), "answeredAt": nil, "answer": nil,
		}
		set := map[string]bool{}
		for _, r := range reasons {
			set[r] = true
		}
		combined = sortedKeysOfSet(set)
	}
	named := make([]string, len(combined))
	for i, reason := range combined {
		named[i] = "`" + reason + "`"
	}
	value["question"] = fmt.Sprintf(
		"Mission %s reached lifecycle fence(s) %s. Choose whether to amend, price, reseal, and sign the contract or leave the mission parked.",
		mission, strings.Join(named, ", "))
	if err := atomicWriteJSON(path, value); err != nil {
		return "", err
	}
	return path, nil
}

func sortedKeysOfSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CheckOrReserve checks the job fences and, when reserve is set and clear,
// records the job's reservation.
func CheckOrReserve(repo, mission, job string, capMin int, reserve bool) error {
	dir, path, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return err
	}
	defer lock.release()
	fences, err := loadFences(repo, mission)
	if err != nil {
		return err
	}
	values, err := verifiedContractValues(repo, mission, fences)
	if err != nil {
		return err
	}
	if found := violations(repo, values, fences, &capMin, "job"); len(found) > 0 {
		ask, askErr := writeBatchedAsk(repo, mission, found)
		return fenceRefusal("job", found, ask, askErr)
	}
	if reserve {
		reservations := reservationsMap(fences)
		if _, exists := reservations[job]; exists {
			return fmt.Errorf("mission fence reservation already exists for job: %s", job)
		}
		reservations[job] = map[string]any{"reservedAt": fenceNowISO(), "capMin": capMin}
		return atomicWriteJSON(path, fences)
	}
	return nil
}

// ReleaseJob deletes a job's fence reservation, for dispatches that died
// during setup without ever starting a process. A husk's reservation
// otherwise counts against fence.jobs forever — rep 1 of cohort
// bm-1-20260813t132947z held 8 reservations for 4 jobs that ever ran, so
// half the signed job budget was consumed by refusals and the prompt's
// headroom lied to the host every turn. Releasing under the fence lock
// also closes the reserve-before-setup window in which a doomed dispatch
// holds a concurrency slot it will never use.
func ReleaseJob(repo, mission, job string) error {
	dir, path, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return err
	}
	defer lock.release()
	fences, err := loadFences(repo, mission)
	if err != nil {
		return err
	}
	reservations := reservationsMap(fences)
	if _, exists := reservations[job]; !exists {
		return nil // never reserved, or already released: nothing to undo
	}
	delete(reservations, job)
	return atomicWriteJSON(path, fences)
}

// ReserveCycle checks the cycle fences and records a cycle.
func ReserveCycle(repo, mission string) error {
	dir, path, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return err
	}
	defer lock.release()
	fences, err := loadFences(repo, mission)
	if err != nil {
		return err
	}
	values, err := verifiedContractValues(repo, mission, fences)
	if err != nil {
		return err
	}
	if found := violations(repo, values, fences, nil, "cycle"); len(found) > 0 {
		ask, askErr := writeBatchedAsk(repo, mission, found)
		return fenceRefusal("cycle", found, ask, askErr)
	}
	cycles, _ := intValue(fences["cycles"])
	fences["cycles"] = cycles + 1
	return atomicWriteJSON(path, fences)
}

// AuthorizeCap authorizes a per-job cap for a runtime/model pair, computing the
// deadline against the mission's remaining wall clock, and records the
// reservation. It returns the authorization result.
func AuthorizeCap(repo, mission, job, runtime, model string, requested *int) (map[string]any, error) {
	dir, path, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return nil, err
	}
	defer lock.release()
	fences, err := loadFences(repo, mission)
	if err != nil {
		return nil, err
	}
	values, err := verifiedContractValues(repo, mission, fences)
	if err != nil {
		return nil, err
	}
	pairKey := fmt.Sprintf("cap.min.%s.%s", runtime, model)
	var authorized int64
	signedRule := "fence-default"
	if v, ok := values[pairKey]; ok {
		authorized = atoiOr(v)
		signedRule = "contract-pair"
	} else {
		authorized = atoiOr(values["fence.job-cap-min"])
	}
	if requested != nil && int64(*requested) > authorized {
		named := "fence.job-cap-min"
		if _, ok := values[pairKey]; ok {
			named = pairKey
		}
		return nil, fmt.Errorf("mission fence refused requested cap %dm above signed %s=%dm", *requested, named, authorized)
	}
	capMin := authorized
	if requested != nil {
		capMin = int64(*requested)
	}
	if found := violations(repo, values, fences, nil, "authorized-job"); len(found) > 0 {
		ask, askErr := writeBatchedAsk(repo, mission, found)
		return nil, fenceRefusal("job", found, ask, askErr)
	}
	launch := nowUTC().Truncate(time.Second)
	started, _ := parseTimestamp(fences["startedAt"].(string))
	wall, _ := strconv.ParseFloat(values["fence.wall-clock-hours"], 64)
	missionEnd := started.Add(time.Duration(wall * 3600 * float64(time.Second)))
	remaining := int64(missionEnd.Sub(launch).Seconds())
	if remaining < 120 {
		return nil, fmt.Errorf("mission has %d seconds of wall clock; refusing to start a job that cannot run", remaining)
	}
	requestedDeadline := launch.Add(time.Duration(capMin) * time.Minute)
	truncated := requestedDeadline.After(missionEnd)
	deadlineTime := requestedDeadline
	if missionEnd.Before(requestedDeadline) {
		deadlineTime = missionEnd
	}
	deadline := deadlineTime.UTC().Format("2006-01-02T15:04:05Z")
	var truncatedBy any
	if truncated {
		truncatedBy = "wall-clock"
	}
	rule := signedRule
	origin := "contract"
	if requested != nil {
		rule = "argument"
		origin = "argument"
	}
	source := map[string]any{"rule": rule, "origin": origin, "truncatedBy": truncatedBy}
	reservations := reservationsMap(fences)
	if _, exists := reservations[job]; exists {
		return nil, fmt.Errorf("mission fence reservation already exists for job: %s", job)
	}
	reservations[job] = map[string]any{
		"reservedAt": fenceNowISO(), "capMin": capMin, "capDeadline": deadline,
		"runtime": runtime, "model": model, "source": source,
	}
	if err := atomicWriteJSON(path, fences); err != nil {
		return nil, err
	}
	return map[string]any{"capMin": capMin, "capDeadline": deadline, "source": source}, nil
}

// Refuse raises a batched fence ask for one allowed reason and returns its path.
func Refuse(repo, mission, reason string) (string, error) {
	allowed := map[string]bool{
		"wall-clock-hours": true, "cycles": true, "jobs": true, "concurrency": true, "job-cap-min": true,
	}
	if !allowed[reason] {
		return "", fmt.Errorf("unknown mission fence refusal reason")
	}
	dir, _, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return "", err
	}
	defer lock.release()
	return writeBatchedAsk(repo, mission, []string{reason})
}

// The provenance a terminal job's aggregate entry carries
// (plans/patience-orphan-usage.md O3): the adapter reported the usage, the
// aggregator derived it from a provably dead round's event stream, the group
// is not yet provably dead, or the usage is unrecoverable by proof.
const (
	usageReported     = "reported"
	usageDerived      = "derived"
	usagePendingProof = "pending-death-proof"
	usageUnavailable  = "unavailable"
)

// usageTokenFields are the typed token counters every per-round usage writer
// emits.
var usageTokenFields = []string{"inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"}

// probeGroupGone probes whether a recorded process group is provably absent.
// Only ESRCH proves it — the shipped group-exists semantics: success or a
// permission denial proves existence, and any other failure proves nothing.
// Overridable in tests.
var probeGroupGone = func(pgid int64) (gone bool, detail string) {
	switch err := unix.Kill(-int(pgid), 0); err {
	case unix.ESRCH:
		return true, ""
	case nil:
		return false, fmt.Sprintf("process group %d is alive", pgid)
	case unix.EPERM:
		return false, fmt.Sprintf("process group %d exists (permission denial proves existence)", pgid)
	default:
		return false, fmt.Sprintf("process group %d probe failed: %v", pgid, err)
	}
}

// custodianProver proves one recorded custodian through the shared kernel
// custodian discipline — the one owner both reapers already judge by.
// Overridable in tests.
var custodianProver = identity.Custodian

// AggregateUsage totals the typed usage — token counts, cost, and provider
// units — across the mission's finished jobs and writes usage.json. A
// terminal job whose record carries no measured usage is recovered by
// DERIVATION from its round's event stream, in memory and never written
// back, gated on proven whole-group death: the recorded pgid probes ESRCH
// AND every recorded custodian is proven dead. A record with no recorded
// pgid can never satisfy that gate and aggregates unavailable — honesty over
// optimism. Every terminal job's provenance lands in the additive top-level
// rounds array, and a content-equal aggregate skips the write entirely, so
// updatedAt changes exactly when content changes.
func AggregateUsage(repo, mission string) error {
	dir, _, lockPath := fencePaths(repo, mission)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock, err := lockFileAt(lockPath)
	if err != nil {
		return err
	}
	defer lock.release()

	type roundEntry struct {
		jobID      string
		round      any
		provenance string
		source     any
		detail     any
	}
	units := map[[2]string]float64{}
	unavailable := []string{}
	var entries []roundEntry
	jobsDir := filepath.Join(repo, "artifacts", "agents", "jobs")
	paths, _ := filepath.Glob(filepath.Join(jobsDir, "*.json"))
	sort.Strings(paths)
	for _, recordPath := range paths {
		record, err := readJSONObjectFile(recordPath)
		if err != nil {
			continue
		}
		if m, _ := record["mission"].(string); m != mission {
			continue
		}
		if s, _ := record["status"].(string); !terminalJobStatus[s] {
			continue
		}
		jobID, _ := record["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(recordPath), ".json")
		}
		provider, _ := record["runtime"].(string)
		if provider == "" {
			provider = "unknown"
		}
		entry := roundEntry{jobID: jobID, provenance: usageReported}
		if round, ok := intValue(record["round"]); ok {
			entry.round = round
		}
		if !addReportedUsage(units, provider, record["usage"]) {
			entry.provenance, entry.source, entry.detail = deriveRoundUsage(repo, jobsDir, jobID, provider, record, units)
			if entry.provenance != usageDerived {
				unavailable = append(unavailable, jobID)
			}
		}
		entries = append(entries, entry)
	}

	keys := make([][2]string, 0, len(units))
	for key := range units {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	unitList := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		unitList = append(unitList, map[string]any{"provider": key[0], "unit": key[1], "value": units[key]})
	}
	sort.Strings(unavailable)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].jobID != entries[j].jobID {
			return entries[i].jobID < entries[j].jobID
		}
		left, _ := intValue(entries[i].round)
		right, _ := intValue(entries[j].round)
		return left < right
	})
	roundList := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		roundList = append(roundList, map[string]any{
			"jobId": entry.jobID, "round": entry.round, "provenance": entry.provenance,
			"source": entry.source, "detail": entry.detail,
		})
	}

	value := map[string]any{
		"schemaVersion": 1, "missionId": mission, "units": unitList,
		"unavailableJobs": unavailable, "rounds": roundList,
	}
	usagePath := filepath.Join(dir, "usage.json")
	if existing, err := readJSONObjectFile(usagePath); err == nil && aggregateContentEqual(existing, value) {
		return nil
	}
	value["updatedAt"] = fenceNowISO()
	return atomicWriteJSON(usagePath, value)
}

// addReportedUsage sums a record's own usage object into the unit totals and
// reports whether anything was measured. A runtime that reports provider
// units or cost instead of tokens is still measured.
func addReportedUsage(units map[[2]string]float64, provider string, rawUsage any) bool {
	usage, ok := rawUsage.(map[string]any)
	if !ok {
		return false
	}
	measured := false
	if a, _ := usage["availability"].(string); a != "unavailable" {
		for _, field := range usageTokenFields {
			if v, ok := nonNegNumber(usage[field]); ok {
				units[[2]string{provider, "tokens." + field}] += v
				measured = true
			}
		}
	}
	if cost, ok := usage["cost"].(map[string]any); ok {
		currency, cok := cost["currency"].(string)
		if amount, aok := nonNegNumber(cost["amount"]); cok && aok {
			units[[2]string{provider, "cost." + currency}] += amount
			measured = true
		}
	}
	if native, ok := usage["providerUnits"].(map[string]any); ok {
		name, nok := native["name"].(string)
		if value, vok := nonNegNumber(native["value"]); nok && vok {
			units[[2]string{provider, "provider." + name}] += value
			measured = true
		}
	}
	return measured
}

// deriveRoundUsage recovers one unmeasured terminal job's usage from its
// round's event stream — the primary recovery, because no graceful cleanup
// survives a cap or a reap. It derives only when the whole group is provably
// gone: the recorded pgid probes ESRCH and every recorded custodian is
// proven dead; a still-provable-alive group defers to a later pass, and a
// record whose stream cannot prove anything aggregates unavailable. The
// derived value is summed in memory and never written back.
func deriveRoundUsage(repo, jobsDir, jobID, provider string, record map[string]any, units map[[2]string]float64) (provenance string, source, detail any) {
	pgid, ok := intValue(record["pgid"])
	if !ok || pgid < 1 {
		return usageUnavailable, nil, "no recorded pgid: whole-group death is unprovable"
	}
	gone, why := probeGroupGone(pgid)
	if !gone {
		return usagePendingProof, nil, why
	}
	if pid, ok := intValue(record["pid"]); ok && pid >= 1 {
		start, _ := intValue(record["pidStartedAt"])
		tag, _ := record["instanceTag"].(string)
		switch custodianProver(pid, start, tag) {
		case identity.Dead:
		case identity.Alive:
			return usagePendingProof, nil, fmt.Sprintf("recorded custodian pid %d is still alive", pid)
		default:
			return usagePendingProof, nil, fmt.Sprintf("recorded custodian pid %d cannot be proven dead", pid)
		}
	}
	rootID, err := usage.RootJobID(jobsDir, jobID)
	if err != nil {
		return usageUnavailable, nil, fmt.Sprintf("chain root is unresolvable: %v", err)
	}
	round, ok := intValue(record["round"])
	if !ok || round < 1 {
		return usageUnavailable, nil, "job record round is unreadable"
	}
	rel := path.Join("artifacts", "agents", rootID, "rounds", strconv.FormatInt(round, 10), "events.jsonl")
	eventsPath := filepath.Join(repo, filepath.FromSlash(rel))
	if _, err := os.Stat(eventsPath); err != nil {
		return usageUnavailable, nil, fmt.Sprintf("event stream is unreadable: %s", rel)
	}
	// Recovery is DECLARED per provider (agnosticism audit classes
	// 6+7): the seam's registered recoverer answers, and a provider
	// without one is honestly unsupported instead of being fed through
	// another runtime's parser.
	outcome := usage.Recover(provider, usage.RecoveryContext{
		Repo: repo, RoundDir: filepath.Dir(eventsPath), EventsPath: eventsPath,
	})
	if outcome.State != usage.Recovered {
		return usageUnavailable, nil, outcome.Detail
	}
	measured := false
	for _, field := range usageTokenFields {
		if v, ok := nonNegNumber(outcome.Fields[field]); ok {
			units[[2]string{provider, "tokens." + field}] += v
			measured = true
		}
	}
	if !measured {
		// A Recovered outcome with every value null normalizes to plain
		// unavailability — the measured count IS the contract (r3-6).
		return usageUnavailable, nil, fmt.Sprintf("event stream carries no usage block: %s", rel)
	}
	return usageDerived, rel, nil
}

// aggregateContentEqual compares the aggregate's content — everything except
// updatedAt — against an existing file, canonicalized through JSON so number
// representations never fake a difference.
func aggregateContentEqual(existing, computed map[string]any) bool {
	subset := func(doc map[string]any) any {
		return []any{doc["schemaVersion"], doc["missionId"], doc["units"], doc["unavailableJobs"], doc["rounds"]}
	}
	before, errBefore := json.Marshal(subset(existing))
	after, errAfter := json.Marshal(subset(computed))
	return errBefore == nil && errAfter == nil && bytes.Equal(before, after)
}

// nonNegNumber returns a non-negative numeric value, excluding booleans.
func nonNegNumber(v any) (float64, bool) {
	if _, isBool := v.(bool); isBool {
		return 0, false
	}
	f, ok := floatValue(v)
	if !ok || f < 0 {
		return 0, false
	}
	return f, true
}

func isValidUTF8(data []byte) bool {
	return strings.ToValidUTF8(string(data), "�") == string(data)
}

// lockFileAt takes an exclusive flock on the given lock file directly (rather
// than a derived path), creating it on first use.
func lockFileAt(lockPath string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}
