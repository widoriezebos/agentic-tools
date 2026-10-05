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

// briefRemedy is a brief refusal's second line: the brief is the caller's
// file, so the fix is an edit, then the same command again.
const briefRemedy = "fix the brief, then repeat the metasystem command that sent it"

// Error is the refusal's two plain lines; its code is RefusalCode.
func (e *BriefAuthorityRefusal) Error() string {
	return "the brief cites paths the delegate's tree does not hold: " + strings.Join(e.MissingPaths, ", ") + "\n" + briefRemedy
}

// RefusalCode and RefusalDetail are the code and the code-first line
// --verbose and the refusal records keep.
func (e *BriefAuthorityRefusal) RefusalCode() string { return "BRIEF_AUTHORITY_REFUSED" }
func (e *BriefAuthorityRefusal) RefusalDetail() string {
	return e.RefusalCode() + ": missing repository paths: " + strings.Join(e.MissingPaths, ", ")
}

type BriefBounds struct {
	Boundary []string
	Ceiling  *int64
}
type BriefBoundsRefusal struct {
	Header, Detail string
	// Remedy is line 2: how the brief's author resolves it.
	Remedy string
}

func (e *BriefBoundsRefusal) Error() string {
	return "the brief's " + e.Header + " header " + e.Detail + "\n" + e.Remedy
}

func (e *BriefBoundsRefusal) RefusalCode() string { return "BRIEF_BOUNDS_INVALID" }
func (e *BriefBoundsRefusal) RefusalDetail() string {
	return e.RefusalCode() + ": " + e.Header + ": " + e.Detail
}

type BriefAdmission struct {
	Bytes  []byte
	Bounds BriefBounds
	Mode   string
}
type briefHeaders struct{ mode, boundary, ceiling []string }
type briefInstallPrefix func() (string, error)

const briefCeilingDetail = "must be a whole number from 0 to 9223372036854775807"

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
	// Settings keys name modes in lower case, so the header is read that way.
	return strings.ToLower(headers.mode[0]), nil
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

// ReadReviewBriefAdmission admits a critic's brief against the trees the
// critic reads: the reviewed commit when reviews names one
// ("commit:<sha>", the unit under review, which may hold files main does
// not), and the dispatcher's HEAD, whose checkout is the critic's
// workspace. A cited path either tree holds is admitted; a reviewed commit
// the repository does not hold refuses. With a reviewed commit, citations
// in the frozen build section are exempt. An optional Git reader keeps
// admission on the caller's repository port.
func ReadReviewBriefAdmission(briefPath, installRoot, baseTree, diskRoot, reviews string, git ...func(string, ...string) (string, error)) (BriefAdmission, error) {
	reviewed, named := strings.CutPrefix(reviews, "commit:")
	if !named {
		reviewed = ""
	}
	facts := gitBriefTreeFacts{}
	if len(git) > 0 {
		facts.run = git[0]
	}
	return readBriefAdmissionWithFacts(briefPath, installRoot, baseTree, diskRoot, reviewed, false, facts)
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

type gitBriefTreeFacts struct {
	run func(string, ...string) (string, error)
}

func (f gitBriefTreeFacts) output(root string, args ...string) (string, error) {
	if f.run != nil {
		return f.run(root, args...)
	}
	return gitOutput(root, args...)
}

func (f gitBriefTreeFacts) InstallPrefix(root string) (string, error) {
	if f.run != nil {
		prefix, err := f.output(root, "rev-parse", "--show-prefix")
		return strings.TrimSuffix(prefix, "/"), err
	}
	return projectInstallPrefix(root)
}
func (f gitBriefTreeFacts) BaseCommit(root string) (string, error) {
	return f.output(root, "rev-parse", "--verify", "HEAD^{commit}")
}
func (f gitBriefTreeFacts) Directories(root, treeish string) (map[string]bool, error) {
	output, err := f.output(root, "ls-tree", "-d", "--name-only", treeish)
	return briefTreeDirectories(output), err
}
func (f gitBriefTreeFacts) HasPath(root, commit, name string) (bool, error) {
	_, err := f.output(root, "cat-file", "-e", commit+":"+name)
	return err == nil, nil
}

func readBriefAdmissionAtRootWithFacts(briefPath, installRoot, baseTree, diskRoot string, requireMode bool, facts briefTreeFacts) (BriefAdmission, error) {
	return readBriefAdmissionWithFacts(briefPath, installRoot, baseTree, diskRoot, "", requireMode, facts)
}

func readBriefAdmissionWithFacts(briefPath, installRoot, baseTree, diskRoot, reviewed string, requireMode bool, facts briefTreeFacts) (BriefAdmission, error) {
	data, err := os.ReadFile(briefPath)
	if err != nil {
		if baseTree == "" {
			return BriefAdmission{}, silentRefusal(1)
		}
		return BriefAdmission{}, fmt.Errorf("brief authority admission cannot read brief: %w", err)
	}
	resolveInstallPrefix := func() (string, error) {
		installPrefix, err := facts.InstallPrefix(installRoot)
		if err != nil {
			return "", fmt.Errorf("brief admission cannot resolve installation prefix: %w", err)
		}
		return installPrefix, nil
	}
	authority := func(admitted []byte, bounds BriefBounds) error {
		return validateBriefAuthority(admitted, bounds, baseTree, diskRoot, facts, resolveInstallPrefix, reviewed)
	}
	if baseTree == "" {
		authority = nil
	}
	return admitBriefBytes(data, resolveInstallPrefix, requireMode, authority)
}
func ParseBriefBounds(data []byte, installPrefix string) (BriefBounds, error) {
	return parseBriefBounds(scanBriefHeaders(data), func() (string, error) { return installPrefix, nil })
}
func parseBriefBounds(headers briefHeaders, resolveInstallPrefix briefInstallPrefix) (BriefBounds, error) {
	if len(headers.boundary) > 1 {
		return BriefBounds{}, boundsRefusal("Boundary", "is written more than once")
	}
	if len(headers.ceiling) > 1 {
		return BriefBounds{}, boundsRefusal("Ceiling", "is written more than once")
	}
	if err := validateBriefBoundsPair(len(headers.boundary) == 1, len(headers.ceiling) == 1); err != nil {
		return BriefBounds{}, err
	}
	if len(headers.boundary) == 0 {
		return BriefBounds{}, nil
	}
	var boundary []string
	if err := json.Unmarshal([]byte(headers.boundary[0]), &boundary); err != nil || boundary == nil {
		return BriefBounds{}, boundsRefusal("Boundary", "must be a JSON array of paths")
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
		return boundsRefusal("Ceiling", "is needed with a Boundary header")
	}
	return boundsRefusal("Boundary", "is needed with a Ceiling header")
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
		return boundsRefusal("Boundary", "cannot be used: the installation folder's name holds a pattern character")
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
	return boundsRefusal("Boundary", "names a path or pattern that is not valid: "+string(encoded))
}
func boundsRefusal(header, detail string) error {
	return &BriefBoundsRefusal{Header: header, Detail: detail, Remedy: briefRemedy}
}

// ValidateBriefAuthority checks explicit repository paths against the exact
// committed tree a delegate will receive. Runtime artifacts are deliberately
// checked in the dispatcher's live checkout instead because they are not tree
// content. A committed path the tree lacks is admitted only as a frozen input
// (brief_frozen.go): a copy of the draft's exact bytes inside the dispatcher's
// checkout. A brief with no mechanically extractable paths is admitted.
func ValidateBriefAuthority(briefPath, baseTree, diskRoot string) error {
	_, err := ReadBriefAdmissionAtRoot(briefPath, baseTree, baseTree, diskRoot, false)
	return err
}

func validateBriefAuthority(data []byte, bounds BriefBounds, baseTree, diskRoot string, facts briefTreeFacts, installPrefix briefInstallPrefix, reviewed string) error {
	baseCommit, err := facts.BaseCommit(baseTree)
	if err != nil {
		return fmt.Errorf("brief authority admission cannot resolve delegate base tree: %w", err)
	}
	// The trees the delegate reads: a reviewed commit first, then the
	// dispatcher's HEAD.
	commits := []string{baseCommit}
	if reviewed != "" {
		resolver, ok := facts.(briefCommitFacts)
		if !ok {
			return fmt.Errorf("brief authority admission cannot resolve the reviewed commit %s", reviewed)
		}
		commit, err := resolver.ResolveCommit(baseTree, reviewed)
		if err != nil {
			return fmt.Errorf("brief authority admission cannot resolve the reviewed commit %s: %w", reviewed, err)
		}
		commits = []string{commit, baseCommit}
	}
	topDirectories, nestedDirectories := map[string]bool{}, map[string]bool{}
	for _, commit := range commits {
		top, err := facts.Directories(baseTree, commit)
		if err != nil {
			return fmt.Errorf("brief authority admission cannot inspect delegate base tree: %w", err)
		}
		for name := range top {
			topDirectories[name] = true
		}
		if top["metasystem"] {
			nested, err := facts.Directories(baseTree, commit+":metasystem")
			if err != nil {
				return fmt.Errorf("brief authority admission cannot inspect metasystem base tree: %w", err)
			}
			for name := range nested {
				nestedDirectories[name] = true
			}
		}
	}
	// A path cited from the installation (plans/designs/X.md) names
	// metasystem/plans/designs/X.md in the tree.
	prefix := ""
	if installPrefix != nil {
		prefix, _ = installPrefix()
	}

	authorityText := string(data)
	if reviewed != "" {
		lines := strings.Split(authorityText, "\n")
		for i, line := range lines {
			if line == "# Supplied accepted implementation brief (frozen at dispatch)" {
				authorityText = strings.Join(lines[:i], "\n")
				break
			}
		}
	}
	candidates := extractBriefAuthorityPaths(authorityText, bounds, topDirectories, nestedDirectories)
	frozen := briefFrozenInputs(data)
	missing := make([]string, 0)
	for _, candidate := range candidates {
		if artifactAuthorityPath(candidate) {
			present, statErr := runtimePathPresent(diskRoot, candidate, installPrefix, nil)
			if statErr != nil {
				return statErr
			}
			if !present {
				missing = append(missing, candidate)
			}
			continue
		}
		present, pathErr := treeHolds(facts, baseTree, commits, candidate, prefix)
		if pathErr != nil {
			return pathErr
		}
		if present || frozenInputHolds(frozen[candidate], diskRoot) {
			continue
		}
		// A path Git ignores can never be in a tree: it is runtime state,
		// read where artifacts are, on the critic's disk.
		ignored := func(location string) bool {
			if facts, ok := facts.(briefIgnoreFacts); ok {
				return facts.Ignored(baseTree, location)
			}
			return false
		}
		runtime, statErr := runtimePathPresent(diskRoot, candidate, installPrefix, ignored)
		if statErr != nil {
			return statErr
		}
		if !runtime {
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
	return briefTreeDirectories(output), nil
}

func briefTreeDirectories(output string) map[string]bool {
	directories := map[string]bool{}
	for _, name := range strings.Split(output, "\n") {
		if name != "" {
			directories[name] = true
		}
	}
	return directories
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

// treeHolds reports whether one of the commits holds candidate, at the
// cited path or under the installation folder prefix.
func treeHolds(facts briefTreeFacts, baseTree string, commits []string, candidate, prefix string) (bool, error) {
	names := []string{candidate}
	if prefix != "" && !strings.HasPrefix(candidate, prefix+"/") {
		names = append(names, prefix+"/"+candidate)
	}
	for _, commit := range commits {
		for _, name := range names {
			present, err := facts.HasPath(baseTree, commit, name)
			if err != nil {
				return false, fmt.Errorf("brief authority admission cannot inspect committed path %s: %w", name, err)
			}
			if present {
				return true, nil
			}
		}
	}
	return false, nil
}

// briefCommitFacts resolves a commit the delegate reviews. Facts that cannot
// resolve one refuse a review brief that names it.
type briefCommitFacts interface {
	ResolveCommit(root, revision string) (string, error)
}

func (f gitBriefTreeFacts) ResolveCommit(root, revision string) (string, error) {
	return f.output(root, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
}

// briefIgnoreFacts says whether Git ignores a path in a checkout. Facts
// that cannot say treat no path as ignored, so it must be in the tree.
type briefIgnoreFacts interface {
	Ignored(root, name string) bool
}

func (f gitBriefTreeFacts) Ignored(root, name string) bool {
	_, err := f.output(root, "check-ignore", "-q", "--", name)
	return err == nil
}

// runtimePathPresent reports whether the disk the critic gets holds a cited
// runtime path: at the path from the repository top, or under the
// installation folder, as a brief written from the installation names it
// (artifacts/agents/context for metasystem/artifacts/agents/context). When
// ignored is set, a location counts only when Git ignores it there.
func runtimePathPresent(diskRoot, candidate string, installPrefix briefInstallPrefix, ignored func(string) bool) (bool, error) {
	locations := []string{candidate}
	if installPrefix != nil {
		if prefix, err := installPrefix(); err == nil && prefix != "" && !strings.HasPrefix(candidate, prefix+"/") {
			locations = append(locations, prefix+"/"+candidate)
		}
	}
	for _, location := range locations {
		if _, err := os.Stat(filepath.Join(diskRoot, filepath.FromSlash(location))); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("brief authority admission cannot inspect runtime path %s: %w", location, err)
		}
		if ignored == nil || ignored(location) {
			return true, nil
		}
	}
	return false, nil
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
