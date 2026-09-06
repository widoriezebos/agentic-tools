package spend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

const spendCacheSchemaVersion = 1

type cachedJobMeasurement struct {
	Path         string                 `json:"path"`
	Size         int64                  `json:"size"`
	ModTimeNanos int64                  `json:"modTime"`
	Measurement  mission.JobMeasurement `json:"measurement"`
}

type terminalJobMeasurementCache struct {
	SchemaVersion int                             `json:"schemaVersion"`
	Entries       map[string]cachedJobMeasurement `json:"entries"`
}

type cachedTranscriptRequest struct {
	ID        string         `json:"id,omitempty"`
	Session   string         `json:"session,omitempty"`
	CWD       string         `json:"cwd,omitempty"`
	Model     string         `json:"model,omitempty"`
	Timestamp string         `json:"timestamp,omitempty"`
	Line      int            `json:"line"`
	Usage     map[string]any `json:"usage,omitempty"`
	Detail    string         `json:"detail,omitempty"`
}

type transcriptCursorCache struct {
	SchemaVersion  int                                `json:"schemaVersion"`
	Path           string                             `json:"path"`
	Size           int64                              `json:"size"`
	ModTimeNanos   int64                              `json:"modTime"`
	Offset         int64                              `json:"offset"`
	Line           int                                `json:"line"`
	FirstCWD       string                             `json:"firstCwd,omitempty"`
	DelegateDigest string                             `json:"delegateDigest"`
	Requests       map[string]cachedTranscriptRequest `json:"requests"`
	Invalid        []cachedTranscriptRequest          `json:"invalid"`
	Tail           []byte                             `json:"tail,omitempty"`
}

func spendCacheDir(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "spend", "cache")
}

func terminalJobCachePath(repoRoot string) string {
	return filepath.Join(spendCacheDir(repoRoot), "terminal-jobs.json")
}

func transcriptCursorPath(repoRoot, transcriptPath string) string {
	digest := sha256.Sum256([]byte(transcriptPath))
	return filepath.Join(spendCacheDir(repoRoot), hex.EncodeToString(digest[:])+".json")
}

func emptyTerminalJobCache() terminalJobMeasurementCache {
	return terminalJobMeasurementCache{SchemaVersion: spendCacheSchemaVersion, Entries: map[string]cachedJobMeasurement{}}
}

// loadTerminalJobCache returns rewrite=true when an existing cache could not
// be trusted. Its callers rebuild every entry from the job records before
// replacing corrupt state.
func loadTerminalJobCache(path string) (cache terminalJobMeasurementCache, rewrite bool) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return emptyTerminalJobCache(), false
	}
	if err != nil || json.Unmarshal(data, &cache) != nil || !validTerminalJobCache(cache) {
		return emptyTerminalJobCache(), true
	}
	return cache, false
}

func validTerminalJobCache(cache terminalJobMeasurementCache) bool {
	if cache.SchemaVersion != spendCacheSchemaVersion || cache.Entries == nil {
		return false
	}
	for path, entry := range cache.Entries {
		status, _ := entry.Measurement.Record["status"].(string)
		if path == "" || entry.Path != path || entry.Size < 0 || entry.Measurement.Record == nil ||
			!terminalStatus(status) || !settledJobMeasurement(entry.Measurement.Provenance) {
			return false
		}
	}
	return true
}

func loadTranscriptCursor(path, transcriptPath string) (transcriptCursorCache, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return transcriptCursorCache{}, false
	}
	var cache transcriptCursorCache
	if json.Unmarshal(data, &cache) != nil || !validTranscriptCursor(cache, transcriptPath) {
		return transcriptCursorCache{}, false
	}
	return cache, true
}

func validTranscriptCursor(cache transcriptCursorCache, transcriptPath string) bool {
	if cache.SchemaVersion != spendCacheSchemaVersion || cache.Path != transcriptPath || cache.Size < 0 ||
		cache.Offset < 0 || cache.Offset != cache.Size || cache.Line < 0 || cache.DelegateDigest == "" ||
		cache.Requests == nil || cache.Invalid == nil {
		return false
	}
	for key, request := range cache.Requests {
		if key == "" || request.Line < 1 || request.Detail != "" {
			return false
		}
	}
	for _, request := range cache.Invalid {
		if request.Line < 1 || request.Detail == "" {
			return false
		}
	}
	return true
}

var spendCacheWriter = atomicfile.WriteVolatile

func writeSpendCache(path string, value any) error {
	rendered, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return spendCacheWriter(path, string(append(rendered, '\n')))
}
