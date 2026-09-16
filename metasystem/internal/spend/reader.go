package spend

import (
	"os"
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
type readerJobs map[string]bool
type scanResult struct {
	calls            map[string]transcriptRequest
	unmeasured       []UnmeasuredEntry
	foreign, aged    bool
	cacheWriteFailed bool
	err              error
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
