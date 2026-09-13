package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type Capability string

const (
	PerCall       Capability = "per-call"
	PerInvocation Capability = "per-invocation"
	NoStream      Capability = "none"
)

type CallSample struct {
	Runtime       string    `json:"runtime"`
	Session       string    `json:"session"`
	InvocationID  string    `json:"invocationId"`
	PromptTokens  int64     `json:"promptTokens"`
	InputTokens   int64     `json:"inputTokens"`
	CacheCreation int64     `json:"cacheCreation"`
	CacheRead     int64     `json:"cacheRead"`
	At            time.Time `json:"at"`
	Ordinal       int64     `json:"ordinal"`
	Source        string    `json:"source"`
}

type Marker struct {
	Runtime string    `json:"runtime"`
	Session string    `json:"session"`
	Kind    string    `json:"kind"`
	At      time.Time `json:"at"`
	Ordinal int64     `json:"ordinal"`
	Detail  string    `json:"detail,omitempty"`
}

type CursorState struct {
	SchemaVersion   int             `json:"schemaVersion"`
	Runtime         string          `json:"runtime"`
	Session         string          `json:"session"`
	Path            string          `json:"path"`
	Dev             uint64          `json:"dev"`
	Inode           uint64          `json:"inode"`
	Size            int64           `json:"size"`
	Offset          int64           `json:"offset"`
	Tail            []byte          `json:"tail,omitempty"`
	LastReadAt      time.Time       `json:"lastReadAt"`
	Latest          *CallSample     `json:"latest,omitempty"`
	SampleCount     int64           `json:"sampleCount"`
	CompactionCount int64           `json:"compactionCount"`
	LastMarker      *Marker         `json:"lastMarker,omitempty"`
	Seen            map[string]bool `json:"seen"`
	Line            int64           `json:"line"`
	SidechainCount  int64           `json:"sidechainCount"`
	SamplesBytes    int64           `json:"samplesBytes"`
}

type ReadOptions struct {
	Capability   Capability
	Transcript   string
	Home         string
	Toplevel     string
	Installation string
	Now          time.Time
}

type Reading struct {
	Capability     Capability
	Latest         *CallSample
	Reason         string
	NewSamples     int
	NewMarkers     int
	PreviousReadAt time.Time
	Cursor         CursorState
}

var (
	runtimeNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

	callBytesRead   func(int)
	callFileOpens   func(string)
	callJSONDecodes func()
)

func LatestCall(stateRoot, runtime, session string, opts ReadOptions) (Reading, error) {
	if err := validateCallLocation(stateRoot, runtime); err != nil {
		return Reading{}, err
	}
	reading := Reading{Capability: opts.Capability}
	switch opts.Capability {
	case PerInvocation:
		reading.Reason = "unknown (per-invocation usage only)"
		return reading, nil
	case NoStream:
		reading.Reason = "unknown (no per-call stream)"
		return reading, nil
	case PerCall:
	default:
		return Reading{}, fmt.Errorf("invalid call-sample capability %q", opts.Capability)
	}

	var path, reason string
	switch runtime {
	case "claude":
		path, reason = claudeTranscript(opts, session)
	case "codex":
		path, reason = codexRollout(opts, session)
	default:
		reading.Reason = fmt.Sprintf("unknown (no reader for runtime %s)", runtime)
		return reading, nil
	}
	if reason != "" {
		reading.Reason = reason
		return reading, nil
	}

	if runtime == "claude" {
		return readUnderCursor(stateRoot, runtime, session, path, func(line []byte, ordinal int64) (*CallSample, *Marker, bool) {
			return parseClaudeLine(line, ordinal, runtime, session)
		}, opts)
	}

	tokenCounts := 0
	usageRecords := 0
	reading, err := readUnderCursor(stateRoot, runtime, session, path, func(line []byte, _ int64) (*CallSample, *Marker, bool) {
		raw := decodeCallLine(line)
		kind, tokenCount := codexRecordKind(raw)
		if kind == "token_usage_record" {
			usageRecords++
		}
		if tokenCount {
			tokenCounts++
		}
		sample, marker := parseCodexRecord(raw, runtime, session)
		return sample, marker, false
	}, opts)
	if err != nil {
		return reading, err
	}
	if reading.Cursor.SampleCount == 0 && usageRecords == 0 && tokenCounts > 0 {
		reading.Latest = nil
		reading.Reason = "unknown (rollout carries no token_usage_record (codex CLI before 0.153))"
	}
	return reading, nil
}

func SessionSlug(session string) string {
	if sessionNamePattern.MatchString(session) {
		return session
	}
	digest := sha256.Sum256([]byte(session))
	return hex.EncodeToString(digest[:])
}

func CursorPath(stateRoot, runtime, session string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "context", "cursors", runtime+"-"+SessionSlug(session)+".json")
}

func SamplesPath(stateRoot, runtime, session string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "context", "samples", runtime+"-"+SessionSlug(session)+".jsonl")
}

func validateCallLocation(stateRoot, runtime string) error {
	if !filepath.IsAbs(stateRoot) {
		return fmt.Errorf("state root must be absolute: %s", stateRoot)
	}
	if !runtimeNamePattern.MatchString(runtime) {
		return fmt.Errorf("invalid runtime name %q", runtime)
	}
	return nil
}

func callHome(opts ReadOptions) (string, string) {
	if opts.Home != "" {
		return opts.Home, ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Sprintf("unknown (home directory unreadable: %v)", err)
	}
	return home, ""
}

func observeCallOpen(path string) {
	if callFileOpens != nil {
		callFileOpens(path)
	}
}
