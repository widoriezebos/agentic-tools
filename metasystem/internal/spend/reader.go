package spend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

type readerCapability string

const (
	readerCapabilityPerCall readerCapability = "per-call"
	readerCapabilityNone    readerCapability = "none"
)

type reader struct {
	name       string
	capability readerCapability
	inScope    bool
	discover   func(string) discoveryResult
	scan       func(transcriptFile, readerCursor, readerJobs) scanResult
}
type transcriptFile struct {
	path, session, toplevel string
	delegate                bool
	parentSession           string
	registered              *reader
}
type discoveryResult struct {
	files    []transcriptFile
	gaps     []UnmeasuredEntry
	counters SeatSummary
	toplevel string
	fatal    bool
}
type readerCursor struct {
	repoRoot, toplevel string
	now                time.Time
	info               os.FileInfo
}
type readerJob struct {
	path, id, sessionID, resumedSessionID, role, runtime, startedAt string
	record                                                          map[string]any
}
type readerJobs struct {
	records            []readerJob
	bySession          map[string][]readerJob
	referencedSessions map[string]bool
}
type scanResult struct {
	calls            map[string]transcriptRequest
	unmeasured       []UnmeasuredEntry
	foreign, aged    bool
	cacheWriteFailed bool
	err              error
}

func newReaderJobs() readerJobs {
	return readerJobs{bySession: map[string][]readerJob{}, referencedSessions: map[string]bool{}}
}

func (jobs *readerJobs) add(path string, record map[string]any) {
	job := readerJob{
		path: path, id: textOr(record["jobId"], ""), sessionID: textOr(record["sessionId"], ""),
		resumedSessionID: textOr(record["resumedSessionId"], ""), role: textOr(record["role"], ""),
		runtime: textOr(record["runtime"], ""), startedAt: textOr(record["startedAt"], ""), record: record,
	}
	jobs.records = append(jobs.records, job)
	if job.sessionID != "" {
		jobs.bySession[job.sessionID] = append(jobs.bySession[job.sessionID], job)
	}
	for _, session := range []string{job.sessionID, job.resumedSessionID} {
		if session != "" {
			jobs.referencedSessions[session] = true
		}
	}
}

func jobDigest(jobs readerJobs) string {
	records := append([]readerJob(nil), jobs.records...)
	sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })
	hash := sha256.New()
	for _, job := range records {
		fields, _ := json.Marshal([]string{job.path, job.id, job.sessionID, job.resumedSessionID, job.role, job.runtime, job.startedAt})
		hash.Write(fields)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

var readerRegistry = []reader{claudeReader()}

func readerScopeLabel(registry []reader) string {
	names := make([]string, 0, len(registry))
	for _, registered := range registry {
		if registered.inScope {
			names = append(names, registered.name)
		}
	}
	return strings.Join(names, "+")
}
