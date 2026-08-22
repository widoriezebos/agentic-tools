package missionrunner

// Certification adjudication, the host-implementer wall's certification
// gate: a return's certified claims copied into the turn log
// UNVERIFIED would let a host declare any job certified and reduce the
// wall's whole authorization machinery to decoration. Every accepted
// certification proves, against records the host cannot write, that
// the named integration authorization exists, binds this exact job and
// mission incarnation, covers the job record as it stands today, is the
// live head of its chain's supersession order, and has never been
// consumed. A rejected certification consumes nothing and needs no
// authorization — but the job it reports on must be real.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

var authorizationDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// authorizationIndex is one scan of the mission's authorization records:
// every content-addressed record by digest, the set of digests any record
// supersedes, and the derived heads per root job. A record whose embedded
// digest disagrees with its filename is not an authorization at all.
type authorizationIndex struct {
	records     map[string]map[string]any
	superseded  map[string]bool
	headsByRoot map[string][]string
}

func readAuthorizationIndex(dir string) (*authorizationIndex, error) {
	index := &authorizationIndex{
		records:     map[string]map[string]any{},
		superseded:  map[string]bool{},
		headsByRoot: map[string][]string{},
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return index, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		digest := strings.TrimSuffix(name, ".json")
		if !authorizationDigestRe.MatchString(digest) {
			continue
		}
		doc, err := readJSONDoc(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if recorded, _ := doc["authorizationDigest"].(string); recorded != digest {
			continue
		}
		// The record's bytes must still hash to the digest they claim:
		// a rewritten record beside an intact
		// filename is not an authorization at all.
		if recomputed, err := validate.AuthorizationRecordDigest(doc); err != nil || recomputed != digest {
			continue
		}
		index.records[digest] = doc
	}
	for _, doc := range index.records {
		supersedes, _ := doc["supersedes"].([]any)
		for _, item := range supersedes {
			if d, ok := item.(string); ok {
				index.superseded[d] = true
			}
		}
	}
	for digest, doc := range index.records {
		if index.superseded[digest] {
			continue
		}
		rootJob, _ := doc["rootJob"].(string)
		index.headsByRoot[rootJob] = append(index.headsByRoot[rootJob], digest)
	}
	return index, nil
}

// adjudicateCertified judges every certification claim in the return.
// Verified entries come back as adjudicated facts — jobId, verdict,
// evidence, and the consumed authorization digest (null on a rejection) —
// and ONLY these enter the turn log; rejections come back with reasons for
// the auto-ask machinery. An error is a runner-level defect (corrupt
// consumption index, unreadable authorization directory), never a judgment
// on any single claim.
func adjudicateCertified(root, missionID string, state map[string]any, entries []map[string]any) (certified, rejected []map[string]any, err error) {
	certified = []map[string]any{}
	rejected = []map[string]any{}
	if len(entries) == 0 {
		return certified, rejected, nil
	}
	consumed, err := mission.ConsumedAuthorizations(state)
	if err != nil {
		return nil, nil, err
	}
	index, err := readAuthorizationIndex(filepath.Join(missionDirPath(root, missionID), "authorizations"))
	if err != nil {
		return nil, nil, err
	}
	// The live incarnation reads fail-closed: without it no accepted
	// certification can prove which sealed contract its chain ran under.
	incarnation := ""
	if fences, ferr := readJSONDoc(filepath.Join(missionDirPath(root, missionID), "fences.json")); ferr == nil {
		incarnation, _ = fences["approvedContractSha256"].(string)
	}

	reject := func(entry map[string]any, reason string) {
		rejected = append(rejected, map[string]any{"kind": "certified", "value": entry, "reason": reason})
	}
	for _, entry := range entries {
		jobID, _ := entry["jobId"].(string)
		record, recordErr := readJSONDoc(filepath.Join(jobsDirPath(root), jobID+".json"))
		if jobID == "" || recordErr != nil {
			reject(entry, "job record does not exist or is unreadable")
			continue
		}
		if !numericEqual(record["mission"], missionID) {
			reject(entry, "job record is not stamped for this mission")
			continue
		}
		verdict, _ := entry["verdict"].(string)
		if verdict == "rejected" {
			// A rejected certification is a lawful report and consumes
			// nothing: any digest it names is inert and normalized away so
			// no unverified authorization claim survives into the log.
			certified = append(certified, map[string]any{
				"jobId": jobID, "verdict": "rejected",
				"evidence": entry["evidence"], "authorizationDigest": nil,
			})
			continue
		}
		digest, _ := entry["authorizationDigest"].(string)
		if !authorizationDigestRe.MatchString(digest) {
			reject(entry, "an accepted certification must name its integration authorization digest")
			continue
		}
		authorization, exists := index.records[digest]
		if !exists {
			reject(entry, "integration authorization record does not exist")
			continue
		}
		if authJob, _ := authorization["jobId"].(string); authJob != jobID {
			reject(entry, "integration authorization was issued for a different job")
			continue
		}
		if authMission, _ := authorization["mission"].(string); authMission != missionID {
			reject(entry, "integration authorization was issued for a different mission")
			continue
		}
		if authIncarnation, _ := authorization["missionIncarnation"].(string); incarnation == "" || authIncarnation != incarnation {
			reject(entry, "integration authorization was issued under a different mission incarnation")
			continue
		}
		identityDigest, digestErr := validate.JobIdentityDigest(record)
		if digestErr != nil || authorization["jobRecordDigest"] != identityDigest {
			reject(entry, "job record identity no longer matches the authorization")
			continue
		}
		if index.superseded[digest] {
			reject(entry, "integration authorization is superseded by a later round")
			continue
		}
		rootJob, _ := authorization["rootJob"].(string)
		if heads := index.headsByRoot[rootJob]; len(heads) != 1 {
			reject(entry, fmt.Sprintf("authorization chain %s carries %d live heads; a forked chain certifies nothing", rootJob, len(heads)))
			continue
		}
		if consumer, taken := consumed[digest]; taken {
			reject(entry, "integration authorization was already consumed by turn "+consumer)
			continue
		}
		certified = append(certified, map[string]any{
			"jobId": jobID, "verdict": "accepted",
			"evidence": entry["evidence"], "authorizationDigest": digest,
		})
	}
	return certified, rejected, nil
}
