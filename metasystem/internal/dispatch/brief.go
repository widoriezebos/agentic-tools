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
	briefPathLine     = regexp.MustCompile(`:[0-9]+(-[0-9]+)?$`)
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

// BriefTextMode applies BriefModeOnly's rules to brief text a caller is
// composing. declared counts the "Working Mode:" headers, so a composer can
// tell a headerless brief from a malformed one.
func BriefTextMode(data []byte) (mode string, declared int, err error) {
	headers := scanBriefHeaders(data)
	mode, err = briefModeFromHeaders(headers)
	return mode, len(headers.mode), err
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
	return readBriefAdmissionAtRootWithFacts(briefPath, installRoot, baseTree, diskRoot, requireMode, gitBriefTreeFacts{})
}

// briefTreeFacts supplies the repository facts used by one admission. The
// same instance must answer every query so prefix and path authority refer
// to the same repository view.
type briefTreeFacts interface {
	InstallPrefix(root string) (string, error)
	BaseCommit(root string) (string, error)
	Directories(root, treeish string) (map[string]bool, error)
	HasPath(root, commit, name string) (bool, error)
}

type gitBriefTreeFacts struct{}

func (gitBriefTreeFacts) InstallPrefix(root string) (string, error) {
	return projectInstallPrefix(root)
}
func (gitBriefTreeFacts) BaseCommit(root string) (string, error) {
	return gitOutput(root, "rev-parse", "--verify", "HEAD^{commit}")
}
func (gitBriefTreeFacts) Directories(root, treeish string) (map[string]bool, error) {
	return treeDirectories(root, treeish)
}
func (gitBriefTreeFacts) HasPath(root, commit, name string) (bool, error) {
	_, err := gitOutput(root, "cat-file", "-e", commit+":"+name)
	return err == nil, nil
}

func readBriefAdmissionAtRootWithFacts(briefPath, installRoot, baseTree, diskRoot string, requireMode bool, facts briefTreeFacts) (BriefAdmission, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		if baseTree == "" {
			return BriefAdmission{}, silentRefusal(1)
		}
		return BriefAdmission{}, fmt.Errorf("brief authority admission cannot read brief: %w", err)
	}
	authority := func(admitted []byte, bounds BriefBounds) error {
		return validateBriefAuthority(admitted, bounds, baseTree, diskRoot, facts)
	}
	if baseTree == "" {
		authority = nil
	}
	resolveInstallPrefix := func() (string, error) {
		installPrefix, err := facts.InstallPrefix(installRoot)
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

func validateBriefAuthority(data []byte, bounds BriefBounds, baseTree, diskRoot string, facts briefTreeFacts) error {
	baseCommit, err := facts.BaseCommit(baseTree)
	if err != nil {
		return fmt.Errorf("brief authority admission cannot resolve delegate base tree: %w", err)
	}
	topDirectories, err := facts.Directories(baseTree, baseCommit)
	if err != nil {
		return fmt.Errorf("brief authority admission cannot inspect delegate base tree: %w", err)
	}
	nestedDirectories := map[string]bool{}
	if topDirectories["metasystem"] {
		nestedDirectories, err = facts.Directories(baseTree, baseCommit+":metasystem")
		if err != nil {
			return fmt.Errorf("brief authority admission cannot inspect metasystem base tree: %w", err)
		}
	}

	candidates := extractBriefAuthorityPaths(string(data), bounds, topDirectories, nestedDirectories)
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
		present, pathErr := facts.HasPath(baseTree, baseCommit, candidate)
		if pathErr != nil {
			return fmt.Errorf("brief authority admission cannot inspect committed path %s: %w", candidate, pathErr)
		}
		if !present {
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

type briefAuthorityCitation struct {
	start, end int
	path       string
}

func extractBriefAuthorityPaths(brief string, bounds BriefBounds, topDirectories, nestedDirectories map[string]bool) []string {
	type pathUse struct {
		input  bool
		output bool
	}
	uses := map[string]pathUse{}
	concreteMembers := map[string]bool{}
	for _, member := range bounds.Boundary {
		if briefBoundaryMemberIsConcrete(member) {
			concreteMembers[member] = true
		}
	}
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
		citations := briefBoundaryCitations(line, concreteMembers)
		record := func(candidate string, start int) {
			use := uses[candidate]
			if workspaceOutputLine || briefCreatePrefix.MatchString(line[:start]) {
				use.output = true
			} else {
				use.input = true
			}
			uses[candidate] = use
		}
		for _, citation := range citations {
			if (briefAuthorityPathEligible(citation.path, topDirectories, nestedDirectories) || concreteMembers[citation.path]) &&
				!(boundaryExample && strings.HasPrefix(citation.path, "metasystem/")) {
				record(citation.path, citation.start)
			}
		}
		for _, location := range briefPathTokenRun.FindAllStringIndex(line, -1) {
			if briefCitationConsumes(citations, location) {
				continue
			}
			token := strings.TrimRight(line[location[0]:location[1]], ".")
			if strings.ContainsAny(token, "*<>${") || !briefPathToken.MatchString(token) {
				continue
			}
			if !briefAuthorityPathEligible(token, topDirectories, nestedDirectories) || (boundaryExample && strings.HasPrefix(token, "metasystem/")) {
				continue
			}
			record(token, location[0])
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

func briefBoundaryMemberIsConcrete(member string) bool {
	return !strings.HasSuffix(member, "/") &&
		!briefBoundaryIsPattern(member) &&
		!strings.ContainsAny(member, "<>${") &&
		!briefPathLine.MatchString(member)
}

func briefBoundaryCitations(line string, members map[string]bool) []briefAuthorityCitation {
	var citations []briefAuthorityCitation
	for start := 0; start < len(line); {
		switch line[start] {
		case '`':
			offset := strings.IndexByte(line[start+1:], '`')
			if offset < 0 {
				return citations
			}
			end := start + offset + 1
			value := line[start+1 : end]
			var decoded string
			if json.Unmarshal([]byte(value), &decoded) == nil {
				value = decoded
			}
			if members[value] {
				citations = append(citations, briefAuthorityCitation{start, end + 1, value})
			}
			start = end + 1
		case '"':
			value, end, terminated, valid := briefJSONStringAt(line, start)
			if !terminated {
				return citations
			}
			if valid && members[value] {
				citations = append(citations, briefAuthorityCitation{start, end + 1, value})
			}
			start = end + 1
		default:
			start++
		}
	}
	return citations
}

func briefJSONStringAt(line string, start int) (string, int, bool, bool) {
	escaped := false
	for end := start + 1; end < len(line); end++ {
		if escaped {
			escaped = false
			continue
		}
		if line[end] == '\\' {
			escaped = true
			continue
		}
		if line[end] == '"' {
			var value string
			err := json.Unmarshal([]byte(line[start:end+1]), &value)
			return value, end, true, err == nil
		}
	}
	return "", len(line), false, false
}

func briefCitationConsumes(citations []briefAuthorityCitation, location []int) bool {
	for _, citation := range citations {
		if location[0] < citation.end && location[1] > citation.start {
			return true
		}
	}
	return false
}

func briefAuthorityPathEligible(candidate string, topDirectories, nestedDirectories map[string]bool) bool {
	parts := strings.Split(candidate, "/")
	eligible := briefAuthorityDirectories[parts[0]] && (topDirectories[parts[0]] || parts[0] == "artifacts")
	if len(parts) >= 3 && parts[0] == "metasystem" {
		eligible = topDirectories["metasystem"] && briefAuthorityDirectories[parts[1]] &&
			(nestedDirectories[parts[1]] || parts[1] == "artifacts")
	}
	return eligible
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
