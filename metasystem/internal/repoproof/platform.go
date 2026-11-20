package repoproof

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const summaryPrefix = "landing result "
const coveragePrefix = "landing passed "

// ProofSummary binds the native reporter's final verdict to its checkout and selection.
type ProofSummary struct {
	Commit    string              `json:"commit"`
	Platform  string              `json:"platform"`
	Route     string              `json:"route"`
	Scope     string              `json:"scope"`
	Narrowing []string            `json:"narrowing,omitempty"`
	Verdict   string              `json:"verdict"`
	Failed    map[string][]string `json:"failed"`
}

type PlatformProof struct {
	ProofSummary
	Path, Digest string
	Passed       map[string][]string
}

var fingerprintPlatform = regexp.MustCompile(`^[^;]+; GOOS="([a-z0-9]+)"; GOARCH="([a-z0-9]+)"(?:;|$)`)
var commitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func platformFromEnvironment(line string) string {
	parts := fingerprintPlatform.FindStringSubmatch(line)
	if parts == nil {
		return ""
	}
	return parts[1] + "/" + parts[2]
}

func hostSummary(root, fingerprint string, getenv func(string) string, command Command) (*ProofSummary, error) {
	commit := getenv("LANDING_COMMIT")
	// Runs without a requested commit keep the ordinary, unbound verdict.
	if commit == "" {
		return nil, nil
	}
	var head bytes.Buffer
	if err := command([]string{"git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}"}, &head, io.Discard); err != nil || !commitID.MatchString(commit) || strings.TrimSpace(head.String()) != commit {
		return nil, errors.New("the landing commit does not match the proof checkout's HEAD")
	}
	summary := &ProofSummary{Commit: commit, Platform: platformFromEnvironment(fingerprint), Route: "host", Scope: "complete"}
	if summary.Platform == "" {
		return nil, errors.New("the landing environment has no platform")
	}
	if getenv("LANDING_PROOF_ROUTE") == "vm" {
		summary.Route = "vm"
	}
	for _, key := range []string{"LANDING_ONLY", "LANDING_ONLY_TESTS", "LANDING_PROOF_GROUPS", "LANDING_PROOF_PACKAGES"} {
		if getenv(key) != "" {
			summary.Scope = "narrowed"
			summary.Narrowing = append(summary.Narrowing, key)
		}
	}
	if scope := getenv("LANDING_PROOF_SCOPE"); scope != "" && scope != "full" {
		summary.Scope = "scoped"
		summary.Narrowing = append(summary.Narrowing, "LANDING_PROOF_SCOPE")
	}
	return summary, nil
}

// ReadPlatformProof hashes the same stream it inspects. A legacy verdict or a
// partial transcript cannot attest to the commit, route and executed coverage.
func ReadPlatformProof(path string) (PlatformProof, error) {
	var p PlatformProof
	path, err := filepath.Abs(path)
	if err != nil {
		return p, err
	}
	file, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer file.Close()
	hash := sha256.New()
	reader := bufio.NewReader(io.TeeReader(file, hash))
	platform := ""
	summarySeen, coverageSeen := false, false
	checked := -1
	failed := map[string]string{}
	incomplete := errors.New("the output is not a complete run with a final proof summary")
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF && line == "" {
			break
		}
		if err != nil {
			return p, incomplete
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if summarySeen || checked >= 0 && !strings.HasPrefix(line, summaryPrefix) {
			return p, incomplete
		}
		switch {
		case strings.HasPrefix(line, "landing environment "):
			if platform != "" {
				return p, incomplete
			}
			platform = platformFromEnvironment(strings.TrimPrefix(line, "landing environment "))
			if platform == "" {
				return p, errors.New("no landing environment line with a platform")
			}
		case strings.HasPrefix(line, coveragePrefix):
			if coverageSeen || json.Unmarshal([]byte(strings.TrimPrefix(line, coveragePrefix)), &p.Passed) != nil {
				return p, incomplete
			}
			coverageSeen = true
		case strings.HasPrefix(line, "LANDING-FAILED\t"):
			fields := strings.Split(line, "\t")
			if len(fields) != 3 || fields[1] == "" {
				return p, incomplete
			}
			if _, dup := failed[fields[1]]; dup {
				return p, incomplete
			}
			failed[fields[1]] = fields[2]
		case strings.HasPrefix(line, "LANDING-NOT-RUN"):
			return p, incomplete
		case strings.HasPrefix(line, "LANDING-CHECKED\t"):
			checked, err = strconv.Atoi(strings.TrimPrefix(line, "LANDING-CHECKED\t"))
			if err != nil || checked != len(failed) {
				return p, incomplete
			}
		case strings.HasPrefix(line, summaryPrefix):
			if checked < 0 || json.Unmarshal([]byte(strings.TrimPrefix(line, summaryPrefix)), &p.ProofSummary) != nil {
				return p, incomplete
			}
			summarySeen = true
		}
	}
	if platform == "" {
		return p, errors.New("no landing environment line")
	}
	if !summarySeen || !commitID.MatchString(p.Commit) || p.Platform != platform || len(p.Failed) != len(failed) {
		return p, incomplete
	}
	for unit, names := range p.Failed {
		words, ok := failed[unit]
		if !ok || strings.Join(names, " ") != words {
			return p, incomplete
		}
	}
	if p.Route != "host" {
		return p, errors.New("a platform incident needs a native host proof")
	}
	if p.Scope != "complete" || len(p.Narrowing) > 0 {
		return p, fmt.Errorf("the run is %s; filing and closing need a complete run (selection inputs: %s)", p.Scope, strings.Join(p.Narrowing, ", "))
	}
	if checked == 0 && p.Verdict != "green" || checked > 0 && p.Verdict != "red" {
		return p, incomplete
	}
	p.Path, p.Digest = path, fmt.Sprintf("%x", hash.Sum(nil))
	return p, nil
}
