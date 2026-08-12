package missionrunner

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// Patience floors (plans/patience-satellite-4.md, ACCEPTED 2026-08-12): the
// vocal-only chain-drought observation. A chain round is WITNESSED exactly
// when a concluded turn's durable log certifies its job, by jobId, with
// verdict accepted and a non-empty trimmed evidence string; patience counts
// the chain's current drought — counted jobs newer than the newest witness —
// against a floor sealed in the mission contract. Breaches book bounded
// annotations beside the cycle line and act on nothing: floors never kill,
// never park, never feed the breaker, and never write fuse-visible lines.

// patienceIDRe is the job-id grammar every counted identifier must match; a
// record whose recorded jobId is absent, invalid, or different from its
// filename stem is outside patience entirely (its remedies cannot act on it).
var patienceIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// patienceLawfulNonterminal is the shipped nonterminal status vocabulary: a
// job in one of these is in flight and never counts — nagging over work
// still legitimately being paid for would be noise.
var patienceLawfulNonterminal = map[string]bool{
	"pending-setup": true, "pending": true, "running": true,
}

// patienceNeverStarted is the never-started error vocabulary (r17/P4-084,
// r18/P4-085), named exactly from the shipped writers: HandshakeEval's
// pre-running refusals (internal/dispatch/handshake.go) and the adapter
// setup failures (scripts/agents/dispatch.sh, runtime-common.sh). A
// cancelled or failed record with one of these errors and no spend-proving
// usage never started: nobody worked, so no one's patience is debited.
// resume_collision is deliberately absent: a pre-start collision carries no
// effectiveModel and is excluded by the model requirement, while a
// post-running collision is real lost work. Spend-proving usage trumps every
// entry (r14/P4-078).
var patienceNeverStarted = map[string]bool{
	"abandoned-setup":              true,
	"handshake_timeout":            true,
	"launch_failed":                true,
	"handshake_missing_session_id": true,
}

// patienceNeverStartedPrefix matches the parameterized member of the
// never-started vocabulary.
const patienceNeverStartedPrefix = "permissions_mismatch:"

// patienceModelSentinels are shipped effectiveModel values that canonicalize
// nonempty but are NOT model evidence (r6/P4-041): treating them as evidence
// would silently resolve a configured floor to infinite patience.
var patienceModelSentinels = map[string]bool{"unobserved": true}

const patienceMultiModelPrefix = "multi-model:"

// patienceDetailBound is the landed-returns bound copied exactly
// (r2/P4-027, r8/P4-051): at most 20 Patience lines per booking — 20 detail,
// or 19 detail plus 1 overflow whose count includes breaches AND orphans.
const patienceDetailBound = 20

// patienceFloors is the sealed floor table: role|runtime|model → rounds.
type patienceFloors map[string]int

// parsePatienceFloors reads the patience.rounds.* entries from the approved
// contract's authored values. Floors exist only there (r1/P4-006, r1/P4-007):
// no conf, local, environment, or per-dispatch surface.
func parsePatienceFloors(values map[string]string) patienceFloors {
	floors := patienceFloors{}
	for key, value := range values {
		rest, ok := strings.CutPrefix(key, "patience.rounds.")
		if !ok {
			continue
		}
		parts := strings.SplitN(rest, ".", 3)
		if len(parts) != 3 {
			continue
		}
		n := 0
		for _, r := range value {
			if r < '0' || r > '9' {
				n = -1
				break
			}
			n = n*10 + int(r-'0')
		}
		if n > 0 {
			floors[parts[0]+"|"+parts[1]+"|"+parts[2]] = n
		}
	}
	return floors
}

// patienceChain is one chain under evaluation: its root id, its participating
// records, and whether its lineage is broken (an orphan damage report).
type patienceChain struct {
	root   string
	jobs   []jobRecord
	orphan bool
	closed bool
	count  int
	//lint:ignore U1000 staged by the patience stream (plans/patience-satellite-4.md); wired by its next satellite, not dead code
	floor int // 0 = infinite / not applicable
	//lint:ignore U1000 staged by the patience stream (plans/patience-satellite-4.md); wired by its next satellite, not dead code
	breach bool
	streak []jobRecord
	//lint:ignore U1000 staged by the patience stream (plans/patience-satellite-4.md); wired by its next satellite, not dead code
	witness bool
}

// patienceEvaluate derives the bounded Patience annotations for one cycle
// booking. Pure over its inputs: the sealed floors, the mission's job
// records, the durable turn log, and the in-flight conclusion's certified
// entries (which reset streaks before anything is written, r2/P4-016). The
// pre-append read is the linearization point; the one-booking lag windows
// are the design's stated crash contract.
func patienceEvaluate(floors patienceFloors, records []jobRecord, turnLog []any, inflightCertified any) []string {
	if len(floors) == 0 {
		return nil // unconfigured: byte-identical to today, structurally (r9/P4-059)
	}

	// Participation boundary (r10, r12/P4-075): clean records only.
	participating := []jobRecord{}
	excluded := 0
	byID := map[string]jobRecord{}
	for _, record := range records {
		id, _ := record.doc["jobId"].(string)
		stem := strings.TrimSuffix(pathBase(record.path), ".json")
		status, _ := record.doc["status"].(string)
		if id == "" || !patienceIDRe.MatchString(id) || id != stem ||
			(!terminalJobStatuses[status] && !patienceLawfulNonterminal[status]) {
			excluded++
			continue
		}
		participating = append(participating, record)
		byID[id] = record
	}

	// Chain sets via the branch-tolerant parent walk (r3/P4-029); a record
	// whose walk breaks forms a single-round orphan chain (r4/P4-036).
	chains := map[string]*patienceChain{}
	for _, record := range participating {
		id, _ := record.doc["jobId"].(string)
		current := record.doc
		seen := map[string]bool{}
		rootID := id
		orphan := false
		for current["parentJob"] != nil {
			parent, ok := current["parentJob"].(string)
			if !ok || seen[parent] {
				orphan = true
				break
			}
			parentRecord, present := byID[parent]
			if !present {
				orphan = true
				break
			}
			seen[parent] = true
			rootID = parent
			current = parentRecord.doc
		}
		key := rootID
		if orphan {
			key = id
		}
		chain := chains[key]
		if chain == nil {
			chain = &patienceChain{root: key, orphan: orphan}
			chains[key] = chain
		}
		chain.orphan = chain.orphan || orphan
		chain.jobs = append(chain.jobs, record)
	}

	// Closed chains leave the evaluation set at derivation time (r20/P4-093).
	for _, chain := range chains {
		if rootRecord, ok := byID[chain.root]; ok {
			if closed, _ := rootRecord.doc["chainClosed"].(bool); closed {
				chain.closed = true
			}
		}
	}

	// Witness set (r2/P4-024, r9/P4-056, r11/P4-071): accepted certifications
	// with non-empty trimmed string evidence, joined by jobId to a
	// participating, STARTED job. Both the durable log and the in-flight
	// conclusion contribute.
	witnessed := map[string]bool{}
	recordWitness := func(entry any) {
		cert, ok := entry.(map[string]any)
		if !ok {
			return
		}
		verdict, _ := cert["verdict"].(string)
		evidence, _ := cert["evidence"].(string)
		jobID, _ := cert["jobId"].(string)
		if verdict != "accepted" || strings.TrimSpace(evidence) == "" {
			return
		}
		record, ok := byID[jobID]
		if !ok || !patienceStarted(record) {
			return
		}
		status, _ := record.doc["status"].(string)
		if !terminalJobStatuses[status] {
			return
		}
		witnessed[jobID] = true
	}
	for _, raw := range turnLog {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if certified, ok := entry["certified"].([]any); ok {
			for _, cert := range certified {
				recordWitness(cert)
			}
		}
	}
	if certified, ok := inflightCertified.([]any); ok {
		for _, cert := range certified {
			recordWitness(cert)
		}
	}

	// Count each chain's current drought (r9/P4-057): counted jobs strictly
	// newer, in the total order, than the chain's newest witnessed job.
	type ranked struct {
		annotation string
		distance   int
		count      int
		root       string
		orphan     bool
	}
	var details []ranked
	for _, key := range sortedKeys(chains) {
		chain := chains[key]
		if chain.closed {
			continue
		}
		sortJobsNewestFirst(chain.jobs)
		newestWitness := -1
		for i, record := range chain.jobs {
			id, _ := record.doc["jobId"].(string)
			if witnessed[id] {
				newestWitness = i
				break
			}
		}
		streak := chain.jobs
		if newestWitness >= 0 {
			streak = chain.jobs[:newestWitness]
		}
		for _, record := range streak {
			id, _ := record.doc["jobId"].(string)
			status, _ := record.doc["status"].(string)
			if witnessed[id] || !terminalJobStatuses[status] || !patienceStarted(record) {
				continue
			}
			chain.count++
			chain.streak = append(chain.streak, record)
		}
		if chain.count == 0 {
			continue
		}
		if chain.orphan {
			details = append(details, ranked{
				annotation: mission.PatienceOrphanAnnotation(chain.root, chain.count),
				distance:   0, count: chain.count, root: chain.root, orphan: true,
			})
			continue
		}
		//lint:ignore U1000 staged by the patience stream (plans/patience-satellite-4.md); wired by its next satellite, not dead code
		floor := selectPatienceFloor(floors, chain.streak)
		if floor <= 0 || chain.count <= floor {
			continue // infinite patience, or tolerated (strictly-exceeds, r1/P4-011)
		}
		details = append(details, ranked{
			annotation: mission.PatienceChainAnnotation(chain.root, chain.count, floor),
			distance:   chain.count - floor, count: chain.count, root: chain.root,
		})
	}

	// Ranking (r5/P4-040, r7/P4-049): breach distance descending, count
	// descending, root ascending; orphans after all breaches.
	sort.SliceStable(details, func(i, j int) bool {
		a, b := details[i], details[j]
		if a.orphan != b.orphan {
			return !a.orphan
		}
		if a.distance != b.distance {
			return a.distance > b.distance
		}
		if a.count != b.count {
			return a.count > b.count
		}
		return a.root < b.root
	})

	annotations := []string{}
	if len(details) > patienceDetailBound {
		for _, detail := range details[:patienceDetailBound-1] {
			annotations = append(annotations, detail.annotation)
		}
		annotations = append(annotations, mission.PatienceOverflowAnnotation(len(details)-(patienceDetailBound-1)))
	} else {
		for _, detail := range details {
			annotations = append(annotations, detail.annotation)
		}
	}
	// The excluded aggregate sits outside the detail bound: at most one line
	// per booking (r11/P4-069, r12/P4-075).
	if excluded > 0 {
		annotations = append(annotations, mission.PatienceExcludedAnnotation(excluded))
	}
	return annotations
}

// patienceStarted reports whether a terminal record proves the delegate
// actually worked (r9/P4-058 through r18/P4-085). completed and timeout are
// structural: the lifecycle CAS map reaches them only through running.
// cancelled and failed prove it with spend-proving usage — money provably
// spent is work, whatever the error is called — or, lacking usage, a
// non-empty effectiveModel with an error outside the never-started
// vocabulary (HandshakeEval patches the model before deciding failure, so
// the field alone cannot prove work).
func patienceStarted(record jobRecord) bool {
	status, _ := record.doc["status"].(string)
	switch status {
	case "completed", "timeout":
		return true
	case "cancelled", "failed":
		if patienceUsageProvesSpend(record.doc["usage"]) {
			return true
		}
		model, _ := record.doc["effectiveModel"].(string)
		if model == "" {
			return false
		}
		errText, _ := record.doc["error"].(string)
		if patienceNeverStarted[errText] || strings.HasPrefix(errText, patienceNeverStartedPrefix) {
			return false
		}
		return true
	}
	return false
}

// patienceUsageProvesSpend reports whether a usage object proves money was
// spent: at least one concrete spend field strictly positive — token counts,
// the nested cost amount, or a nested provider-unit value. Presence proves
// nothing (adapters write a native usage object even when no block exists,
// r15/P4-081); zero proves nothing (r20/P4-092, r21/P4-097); the
// availability marker plays no part (Devin's tokenless accounts record
// unavailable beside real provider-unit spend, r16/P4-082).
func patienceUsageProvesSpend(raw any) bool {
	usage, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"} {
		if positiveNumber(usage[key]) {
			return true
		}
	}
	if cost, ok := usage["cost"].(map[string]any); ok && positiveNumber(cost["amount"]) {
		return true
	}
	if positiveNumber(usage["cost"]) {
		return true
	}
	if units, ok := usage["providerUnits"].([]any); ok {
		for _, raw := range units {
			if unit, ok := raw.(map[string]any); ok && positiveNumber(unit["value"]) {
				return true
			}
		}
	}
	return false
}

// positiveNumber reports whether a JSON value is a number strictly greater
// than zero.
func positiveNumber(v any) bool {
	f, ok := toFloat(v)
	return ok && f > 0
}

// selectPatienceFloor resolves a chain's floor over its STREAK set only
// (r10/P4-065): the whole (role, runtime, model) triple comes from one
// qualifying evidence record (r8/P4-054, r9/P4-060), rows tried in order.
// Returns 0 for infinite patience.
func selectPatienceFloor(floors patienceFloors, streak []jobRecord) int {
	// Rows 1-2: the newest streak job qualifying as an effective-model
	// evidence record.
	for _, record := range streak {
		role, runtime, model, ok := patienceEvidenceTriple(record, "effectiveModel")
		if !ok {
			continue
		}
		if floor, present := floors[role+"|"+runtime+"|"+model]; present {
			return floor
		}
		return 0 // configured-nothing for a real pair (r3/P4-030)
	}
	// Rows 3-4: the newest streak job whose requestedModel is model evidence
	// (r11/P4-070, r12/P4-074 — the pre-witness root is never consulted).
	for _, record := range streak {
		role, runtime, model, ok := patienceEvidenceTriple(record, "requestedModel")
		if !ok {
			continue
		}
		if floor, present := floors[role+"|"+runtime+"|"+model]; present {
			return floor
		}
		return 0
	}
	// Row 5: the streak carries no model identity; configured-nothing is the
	// honest verdict — the spend stays visible through Landed Returns and
	// the usage ledger.
	return 0
}

// patienceEvidenceTriple extracts a qualifying (role, runtime, canonical
// model) triple from one record: model evidence for the named field plus a
// valid role and runtime on the same record — one record, one triple, no
// cross-record chimeras.
func patienceEvidenceTriple(record jobRecord, modelField string) (role, runtime, model string, ok bool) {
	value, _ := record.doc[modelField].(string)
	if value == "" || patienceModelSentinels[value] || strings.HasPrefix(value, patienceMultiModelPrefix) {
		return "", "", "", false
	}
	canonical := config.CanonicalModel(value)
	if canonical == "" {
		return "", "", "", false
	}
	role, _ = record.doc["role"].(string)
	runtime, _ = record.doc["runtime"].(string)
	if !patienceIDRe.MatchString(role) || !patienceIDRe.MatchString(runtime) {
		return "", "", "", false
	}
	return role, runtime, canonical, true
}

// sortJobsNewestFirst orders records by (endedAt, startedAt, jobId)
// descending (r4/P4-034, r5/P4-038): RFC3339 parsing, missing and
// unparseable timestamps in one oldest bucket, ties falling through, jobId
// the final lexicographic tiebreak — total over damaged records.
func sortJobsNewestFirst(records []jobRecord) {
	stamp := func(doc map[string]any, field string) (int64, bool) {
		raw, _ := doc[field].(string)
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return 0, false
		}
		return parsed.UnixNano(), true
	}
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i].doc, records[j].doc
		for _, field := range []string{"endedAt", "startedAt"} {
			av, aok := stamp(a, field)
			bv, bok := stamp(b, field)
			if aok != bok {
				return aok // a parseable timestamp is newer than the oldest bucket
			}
			if aok && av != bv {
				return av > bv
			}
		}
		ai, _ := a["jobId"].(string)
		bi, _ := b["jobId"].(string)
		return ai > bi
	})
}

// pathBase is filepath.Base without importing path/filepath twice in this
// package's mental model — records always come from one flat directory.
func pathBase(path string) string {
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
