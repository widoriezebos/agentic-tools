package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	briefPathTokenRun = regexp.MustCompile(`[A-Za-z0-9_.*\-/<>{}$]+`)
	briefPathToken    = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.*-]+)+$`)
)

var briefAuthorityDirectories = map[string]bool{
	"docs": true, "plans": true, "records": true, "internal": true,
	"cmd": true, "scripts": true, "memory": true, "artifacts": true,
}

// BriefAuthorityRefusal names every cited path that the delegate could not
// read from the tree it will receive. Missing paths remain structured so an
// admission caller does not have to interpret the rendered explanation.
type BriefAuthorityRefusal struct {
	MissingPaths []string
}

func (e *BriefAuthorityRefusal) Error() string {
	return fmt.Sprintf("BRIEF_AUTHORITY_REFUSED: missing repository paths: %s", strings.Join(e.MissingPaths, ", "))
}

// BriefMode extracts the working mode a brief declares. A brief must carry
// exactly one filled "Working Mode:" header — none, several, or a template
// placeholder still in place is a silent refusal, and the caller names the
// requirement.
func BriefMode(briefPath string) (string, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return "", silentRefusal(1)
	}
	var values []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Working Mode:") {
			values = append(values, strings.TrimSpace(strings.TrimPrefix(line, "Working Mode:")))
		}
	}
	if len(values) != 1 || values[0] == "" || strings.HasPrefix(values[0], "<") {
		return "", silentRefusal(1)
	}
	return values[0], nil
}

// ValidateBriefAuthority checks explicit repository paths against the exact
// committed tree a delegate will receive. Runtime artifacts are deliberately
// checked in the dispatcher's live checkout instead because they are not tree
// content. A brief with no mechanically extractable paths is admitted.
func ValidateBriefAuthority(briefPath, baseTree, diskRoot string) error {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return fmt.Errorf("brief authority admission cannot read brief: %w", err)
	}
	baseCommit, err := gitOutput(baseTree, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return fmt.Errorf("brief authority admission cannot resolve delegate base tree: %w", err)
	}
	topDirectories, err := treeDirectories(baseTree, baseCommit)
	if err != nil {
		return fmt.Errorf("brief authority admission cannot inspect delegate base tree: %w", err)
	}
	nestedDirectories := map[string]bool{}
	if topDirectories["metasystem"] {
		nestedDirectories, err = treeDirectories(baseTree, baseCommit+":metasystem")
		if err != nil {
			return fmt.Errorf("brief authority admission cannot inspect metasystem base tree: %w", err)
		}
	}

	candidates := extractBriefAuthorityPaths(string(data), topDirectories, nestedDirectories)
	missing := make([]string, 0)
	for _, candidate := range candidates {
		if artifactAuthorityPath(candidate) {
			if _, statErr := os.Stat(filepath.Join(diskRoot, filepath.FromSlash(candidate))); statErr != nil {
				if os.IsNotExist(statErr) {
					missing = append(missing, candidate)
					continue
				}
				return fmt.Errorf("brief authority admission cannot inspect runtime path %s: %w", candidate, statErr)
			}
			continue
		}
		if _, pathErr := gitOutput(baseTree, "cat-file", "-e", baseCommit+":"+candidate); pathErr != nil {
			missing = append(missing, candidate)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return &BriefAuthorityRefusal{MissingPaths: missing}
}

func treeDirectories(baseTree, treeish string) (map[string]bool, error) {
	output, err := gitOutput(baseTree, "ls-tree", "-d", "--name-only", treeish)
	if err != nil {
		return nil, err
	}
	directories := map[string]bool{}
	for _, name := range strings.Split(output, "\n") {
		if name != "" {
			directories[name] = true
		}
	}
	return directories, nil
}

func extractBriefAuthorityPaths(brief string, topDirectories, nestedDirectories map[string]bool) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(brief, "\n") {
		lowerLine := strings.ToLower(line)
		boundaryExample := strings.Contains(lowerLine, "diffboundary") && strings.Contains(lowerLine, "example")
		for _, token := range briefPathTokenRun.FindAllString(line, -1) {
			if strings.ContainsAny(token, "*<>${") || !briefPathToken.MatchString(token) {
				continue
			}
			parts := strings.Split(token, "/")
			eligible := briefAuthorityDirectories[parts[0]] && (topDirectories[parts[0]] || parts[0] == "artifacts")
			if len(parts) >= 3 && parts[0] == "metasystem" {
				eligible = topDirectories["metasystem"] && briefAuthorityDirectories[parts[1]] &&
					(nestedDirectories[parts[1]] || parts[1] == "artifacts")
			}
			if eligible && !(boundaryExample && strings.HasPrefix(token, "metasystem/")) {
				seen[token] = true
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func artifactAuthorityPath(path string) bool {
	return strings.HasPrefix(path, "artifacts/") || strings.HasPrefix(path, "metasystem/artifacts/")
}

// WriteCapResolution records a non-mission cap decision: the authorized
// minutes, the absolute deadline they imply from now, and the provenance of
// the rule that chose them. Mission caps come from the mission fence instead;
// this is the non-mission authority's receipt.
func WriteCapResolution(output string, capMin int64, rule, origin string) error {
	deadline := time.Now().UTC().Truncate(time.Second).Add(time.Duration(capMin) * time.Minute)
	return writeCompactJSON(output, map[string]any{
		"capMin":      capMin,
		"capDeadline": deadline.Format("2006-01-02T15:04:05Z"),
		"source": map[string]any{
			"rule":        rule,
			"origin":      origin,
			"truncatedBy": nil,
		},
	})
}
