package dispatch

import (
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
)

var (
	briefPathTokenRun = regexp.MustCompile(`[A-Za-z0-9_.*\-/<>{}$]+`)
	briefPathToken    = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.*-]+)+$`)
	briefHeading      = regexp.MustCompile(`^\s*(#{1,6})\s+(.+?)\s*$`)
	briefOutputLine   = regexp.MustCompile(`(?i)^\s*(?:[-*]\s*)?may[- ](?:write|touch)\s*:`)
	briefCreatePrefix = regexp.MustCompile("(?i)(?:new[ \\t]+file|create)[ \\t]*:?[ \\t]*[`\\\"']?[ \\t]*$")
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
	admission, err := ReadBriefAdmission(briefPath, "", true, nil)
	return admission.Mode, err
}

type BriefBounds struct {
	Boundary []string
	Ceiling  *int64
}
type BriefBoundsRefusal struct{ Header, Detail string }

func (e *BriefBoundsRefusal) Error() string {
	return fmt.Sprintf("BRIEF_BOUNDS_INVALID: %s: %s", e.Header, e.Detail)
}

type BriefAdmission struct {
	Bytes  []byte
	Bounds BriefBounds
	Mode   string
}
type briefHeaders struct{ mode, boundary, ceiling []string }
type briefInstallPrefix func() (string, error)

const briefCeilingDetail = "expected a nonnegative decimal integer no greater than 9223372036854775807"

func scanBriefHeaders(data []byte) briefHeaders {
	var headers briefHeaders
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "Working Mode:"):
			headers.mode = append(headers.mode, strings.TrimSpace(strings.TrimPrefix(line, "Working Mode:")))
		case strings.HasPrefix(line, "Boundary:"):
			headers.boundary = append(headers.boundary, strings.TrimSpace(strings.TrimPrefix(line, "Boundary:")))
		case strings.HasPrefix(line, "Ceiling:"):
			headers.ceiling = append(headers.ceiling, strings.TrimSpace(strings.TrimPrefix(line, "Ceiling:")))
		}
	}
	return headers
}
func briefModeFromHeaders(headers briefHeaders) (string, error) {
	if len(headers.mode) != 1 || headers.mode[0] == "" || strings.HasPrefix(headers.mode[0], "<") {
		return "", silentRefusal(1)
	}
	return headers.mode[0], nil
}
func BriefModeOnly(briefPath string) (string, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return "", silentRefusal(1)
	}
	return briefModeFromHeaders(scanBriefHeaders(data))
}
func ReadBriefAdmission(briefPath, installPrefix string, requireMode bool, authority func([]byte, BriefBounds) error) (BriefAdmission, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		if authority == nil {
			return BriefAdmission{}, silentRefusal(1)
		}
		return BriefAdmission{}, fmt.Errorf("brief authority admission cannot read brief: %w", err)
	}
	return admitBriefBytes(data, func() (string, error) { return installPrefix, nil }, requireMode, authority)
}
func admitBriefBytes(data []byte, resolveInstallPrefix briefInstallPrefix, requireMode bool, authority func([]byte, BriefBounds) error) (BriefAdmission, error) {
	headers := scanBriefHeaders(data)
	mode, modeErr := briefModeFromHeaders(headers)
	if requireMode && authority == nil && modeErr != nil {
		return BriefAdmission{}, modeErr
	}
	bounds, err := parseBriefBounds(headers, resolveInstallPrefix)
	if err != nil {
		return BriefAdmission{}, err
	}
	if authority != nil {
		if err := authority(data, bounds); err != nil {
			return BriefAdmission{}, err
		}
	}
	if requireMode && modeErr != nil {
		return BriefAdmission{}, modeErr
	}
	return BriefAdmission{Bytes: data, Bounds: bounds, Mode: mode}, nil
}

func ReadBriefAdmissionAtRoot(briefPath, installRoot, baseTree, diskRoot string, requireMode bool) (BriefAdmission, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		if baseTree == "" {
			return BriefAdmission{}, silentRefusal(1)
		}
		return BriefAdmission{}, fmt.Errorf("brief authority admission cannot read brief: %w", err)
	}
	authority := func(admitted []byte, bounds BriefBounds) error {
		return validateBriefAuthority(admitted, bounds, baseTree, diskRoot)
	}
	if baseTree == "" {
		authority = nil
	}
	resolveInstallPrefix := func() (string, error) {
		installPrefix, err := projectInstallPrefix(installRoot)
		if err != nil {
			return "", fmt.Errorf("brief admission cannot resolve installation prefix: %w", err)
		}
		return installPrefix, nil
	}
	return admitBriefBytes(data, resolveInstallPrefix, requireMode, authority)
}
func ParseBriefBounds(data []byte, installPrefix string) (BriefBounds, error) {
	return parseBriefBounds(scanBriefHeaders(data), func() (string, error) { return installPrefix, nil })
}
func parseBriefBounds(headers briefHeaders, resolveInstallPrefix briefInstallPrefix) (BriefBounds, error) {
	if len(headers.boundary) > 1 {
		return BriefBounds{}, boundsRefusal("Boundary", "header occurs more than once")
	}
	if len(headers.ceiling) > 1 {
		return BriefBounds{}, boundsRefusal("Ceiling", "header occurs more than once")
	}
	if err := validateBriefBoundsPair(len(headers.boundary) == 1, len(headers.ceiling) == 1); err != nil {
		return BriefBounds{}, err
	}
	if len(headers.boundary) == 0 {
		return BriefBounds{}, nil
	}
	var boundary []string
	if err := json.Unmarshal([]byte(headers.boundary[0]), &boundary); err != nil || boundary == nil {
		return BriefBounds{}, boundsRefusal("Boundary", "expected a JSON array of paths")
	}
	for _, member := range boundary {
		if err := validateBriefMember(member); err != nil {
			return BriefBounds{}, err
		}
		if !strings.HasSuffix(member, "/") && briefBoundaryIsPattern(member) {
			if _, err := path.Match(member, ""); err != nil {
				return BriefBounds{}, invalidBriefMember(member)
			}
		}
	}
	ceilingText := headers.ceiling[0]
	if ceilingText == "" || strings.IndexFunc(ceilingText, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return BriefBounds{}, boundsRefusal("Ceiling", briefCeilingDetail)
	}
	ceiling, err := strconv.ParseInt(ceilingText, 10, 64)
	if err != nil {
		return BriefBounds{}, boundsRefusal("Ceiling", briefCeilingDetail)
	}
	installPrefix, err := resolveInstallPrefix()
	if err != nil {
		return BriefBounds{}, err
	}
	if err := validateBriefInstallPrefix(installPrefix); err != nil {
		return BriefBounds{}, err
	}
	for _, member := range boundary {
		if _, _, err := ProjectBriefBoundary(member, installPrefix); err != nil {
			return BriefBounds{}, err
		}
	}
	return BriefBounds{Boundary: boundary, Ceiling: &ceiling}, nil
}
func ValidateBriefBounds(bounds BriefBounds) error {
	if err := validateBriefBoundsPair(bounds.Boundary != nil, bounds.Ceiling != nil); err != nil {
		return err
	}
	for _, member := range bounds.Boundary {
		if err := validateBriefMember(member); err != nil {
			return err
		}
	}
	if bounds.Ceiling != nil && *bounds.Ceiling < 0 {
		return boundsRefusal("Ceiling", briefCeilingDetail)
	}
	return nil
}
func validateBriefBoundsPair(boundary, ceiling bool) error {
	if boundary == ceiling {
		return nil
	}
	if boundary {
		return boundsRefusal("Ceiling", "required with Boundary")
	}
	return boundsRefusal("Boundary", "required with Ceiling")
}
func ProjectBriefBoundary(member, installPrefix string) (string, bool, error) {
	if err := validateBriefInstallPrefix(installPrefix); err != nil {
		return "", false, err
	}
	directory := strings.HasSuffix(member, "/")
	if installPrefix == "" {
		return member, directory, nil
	}
	prefix := installPrefix + "/"
	if !strings.HasPrefix(member, prefix) {
		return "", false, invalidBriefMember(member)
	}
	return strings.TrimPrefix(member, prefix), directory, nil
}
func validateBriefInstallPrefix(installPrefix string) error {
	if strings.ContainsAny(installPrefix, "*?[]\\") {
		return boundsRefusal("Boundary", "installation prefix contains unsupported pattern bytes")
	}
	return nil
}
func validateBriefMember(member string) error {
	directory := strings.HasSuffix(member, "/")
	if member == "" || path.IsAbs(member) || strings.ContainsRune(member, 0) {
		return invalidBriefMember(member)
	}
	parts := strings.Split(member, "/")
	for i, part := range parts {
		if part == "." || part == ".." || (part == "" && !(directory && i == len(parts)-1)) {
			return invalidBriefMember(member)
		}
	}
	return nil
}
func briefBoundaryIsPattern(member string) bool { return strings.ContainsAny(member, "*?[]\\") }
func invalidBriefMember(member string) error {
	encoded, _ := json.Marshal(member)
	return boundsRefusal("Boundary", "invalid path or pattern "+string(encoded))
}
func boundsRefusal(header, detail string) error {
	return &BriefBoundsRefusal{Header: header, Detail: detail}
}

// ValidateBriefAuthority checks explicit repository paths against the exact
// committed tree a delegate will receive. Runtime artifacts are deliberately
// checked in the dispatcher's live checkout instead because they are not tree
// content. A brief with no mechanically extractable paths is admitted.
func ValidateBriefAuthority(briefPath, baseTree, diskRoot string) error {
	_, err := ReadBriefAdmissionAtRoot(briefPath, baseTree, baseTree, diskRoot, false)
	return err
}

func validateBriefAuthority(data []byte, _ BriefBounds, baseTree, diskRoot string) error {
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
	type pathUse struct {
		input  bool
		output bool
	}
	uses := map[string]pathUse{}
	workspaceHeadingLevel := 0
	for _, line := range strings.Split(brief, "\n") {
		if heading := briefHeading.FindStringSubmatch(line); heading != nil {
			level := len(heading[1])
			name := strings.TrimSpace(strings.TrimRight(heading[2], "#"))
			if strings.EqualFold(name, "Workspace") {
				workspaceHeadingLevel = level
			} else if workspaceHeadingLevel > 0 && level <= workspaceHeadingLevel {
				workspaceHeadingLevel = 0
			}
		}
		lowerLine := strings.ToLower(line)
		boundaryExample := strings.Contains(lowerLine, "diffboundary") && strings.Contains(lowerLine, "example")
		workspaceOutputLine := workspaceHeadingLevel > 0 && briefOutputLine.MatchString(line)
		if strings.HasPrefix(line, "Boundary:") || inertBriefBoundsLine(line) {
			continue
		}
		for _, location := range briefPathTokenRun.FindAllStringIndex(line, -1) {
			token := strings.TrimRight(line[location[0]:location[1]], ".")
			if strings.ContainsAny(token, "*<>${") || !briefPathToken.MatchString(token) {
				continue
			}
			parts := strings.Split(token, "/")
			eligible := briefAuthorityDirectories[parts[0]] && (topDirectories[parts[0]] || parts[0] == "artifacts")
			if len(parts) >= 3 && parts[0] == "metasystem" {
				eligible = topDirectories["metasystem"] && briefAuthorityDirectories[parts[1]] &&
					(nestedDirectories[parts[1]] || parts[1] == "artifacts")
			}
			if !eligible || (boundaryExample && strings.HasPrefix(token, "metasystem/")) {
				continue
			}
			use := uses[token]
			if workspaceOutputLine || briefCreatePrefix.MatchString(line[:location[0]]) {
				use.output = true
			} else {
				use.input = true
			}
			uses[token] = use
		}
	}
	paths := make([]string, 0, len(uses))
	for path, use := range uses {
		if use.input || !use.output {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func inertBriefBoundsLine(line string) bool {
	return len(line) != len(strings.TrimLeft(line, " \t")) && (strings.HasPrefix(strings.TrimLeft(line, " \t"), "Boundary:") || strings.HasPrefix(strings.TrimLeft(line, " \t"), "Ceiling:"))
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
