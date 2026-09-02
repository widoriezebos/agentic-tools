// Package landing owns the two-bars landing classification. Observe mode
// makes the same decision enforcement will consume, but turns every policy
// failure into a durable would-refuse verdict instead of an exit failure.
package landing

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const (
	BarChain     = "a"
	BarDirectFix = "b"
	BarRefusal   = "c"
)

var (
	landingID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	rulingID  = regexp.MustCompile(`^R-[0-9]+[a-z]?(?:-[a-z0-9]+)?$`)
	treeOID   = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

// ObserveParams names the final project tree and its declaration. Chain may
// combine only with register-carriage; every other declaration is singular.
// RepoRoot may itself be nested in a Git worktree.
type ObserveParams struct {
	RepoRoot      string
	CandidateTree string
	Chain         string
	DirectFix     string
	RevertOf      string
}

// Observation is safe to put directly in a commit trailer. The values never
// echo malformed caller text, so bad input cannot mint another trailer line.
type Observation struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Mode           string `json:"mode"`
	Bar            string `json:"bar"`
	Verdict        string `json:"verdict"`
	Code           string `json:"code"`
	Provenance     string `json:"provenance"`
	VerdictTrailer string `json:"verdictTrailer"`
}

// Observe evaluates one prospective landing, then applies the promotion
// policy recorded in the landing base. The caller remains responsible for
// enforcing Mode only for agent commits; human commits stay sovereign.
func Observe(params ObserveParams) Observation {
	return applyPromotion(params, observe(params))
}

func observe(params ObserveParams) Observation {
	if !treeOID.MatchString(params.CandidateTree) {
		return wouldRefuse("malformed-candidate-tree", "none change=unknown")
	}
	change, err := changeDigest(params.RepoRoot, params.CandidateTree)
	if err != nil {
		return wouldRefuse("candidate-tree-unreadable", "none change=unknown")
	}
	if params.RevertOf != "" && params.DirectFix != "exact-revert" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.DirectFix == "exact-revert" && params.RevertOf == "" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.Chain == "" && params.DirectFix == "" {
		return wouldRefuse("missing-declaration", "none change="+change)
	}
	if params.Chain != "" && params.DirectFix != "" && params.DirectFix != "register-carriage" {
		return wouldRefuse("conflicting-declarations", "invalid change="+change)
	}
	if params.Chain != "" {
		return observeChain(params, change)
	}
	return observeDirectFix(params, change)
}

func changeDigest(root, candidateTree string) (string, error) {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return "", err
	}
	patch, err := workspace.Diff(baseTree, candidateTree)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(patch)
	return fmt.Sprintf("%x", sum), nil
}

func observeChain(params ObserveParams, change string) Observation {
	provenance := "invalid change=" + change
	if !landingID.MatchString(params.Chain) {
		return wouldRefuse("malformed-chain-id", provenance)
	}
	provenance = fmt.Sprintf("chain=%s change=%s", params.Chain, change)
	recordPath := filepath.Join(params.RepoRoot, "artifacts", "agents", "jobs", params.Chain+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		return wouldRefuse("chain-record-unreadable", provenance)
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil || record["jobId"] != params.Chain || record["parentJob"] != nil {
		return wouldRefuse("chain-record-malformed", provenance)
	}
	if record["role"] != "implementer" {
		return wouldRefuse("chain-not-implementation", provenance)
	}
	hazard, _ := record["destructiveReach"].(string)
	if hazard != "DESIGN-BEARING" && hazard != "DESTRUCTIVE-REACH" {
		return wouldRefuse("chain-not-design-bearing", provenance)
	}
	if record["chainClosed"] != true {
		return wouldRefuse("chain-open", provenance)
	}
	output, err := chainCertifiedOutput(params.RepoRoot, params.Chain, record)
	if err != nil {
		return wouldRefuse("chain-output-unreadable", provenance)
	}
	certifiedDigest, extraPaths, err := bindCertifiedChange(params.RepoRoot, params.CandidateTree, output)
	if err != nil {
		return wouldRefuse("chain-output-mismatch", provenance)
	}
	provenance = fmt.Sprintf("chain=%s change=%s certified-change=%s", params.Chain, change, certifiedDigest)
	if len(extraPaths) > 0 && params.DirectFix != "register-carriage" {
		return wouldRefuse("chain-has-uncarried-paths", provenance)
	}
	if params.DirectFix == "register-carriage" {
		provenance = fmt.Sprintf("chain=%s direct-fix class=register-carriage change=%s certified-change=%s", params.Chain, change, certifiedDigest)
		if err := registerCarriage(params.RepoRoot, params.CandidateTree, extraPaths); err != nil {
			return wouldRefuse(carriageRefusalCode(err), provenance)
		}
		observation := pass(BarChain, "closed-chain", provenance)
		observation.VerdictTrailer = "pass bar=a carriage=register-carriage"
		return observation
	}
	return pass(BarChain, "closed-chain", provenance)
}

type certifiedOutput struct {
	round          int
	implementerJob string
	reviewedTree   string
	patch          []byte
}

// chainCertifiedOutput follows conformance's real storage seam. A root-id
// invocation writes the current review under rounds/1 even after follow-up
// rounds, while a follow-up-id invocation writes under rounds/N. Every
// parseable review is considered; the closed critic's reviewedTree is the
// strongest selector, the terminal implementer job is next, and the numeric
// round is only the deterministic fallback. A single review needs no guess.
func chainCertifiedOutput(root, chain string, rootRecord map[string]any) (certifiedOutput, error) {
	roundsRoot := filepath.Join(root, "artifacts", "agents", chain, "rounds")
	entries, err := os.ReadDir(roundsRoot)
	if err != nil {
		return certifiedOutput{}, err
	}
	var outputs []certifiedOutput
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		round, err := strconv.Atoi(entry.Name())
		if err != nil || round < 1 {
			continue
		}
		roundDir := filepath.Join(roundsRoot, entry.Name())
		data, err := os.ReadFile(filepath.Join(roundDir, "review.json"))
		if err != nil {
			continue
		}
		var review struct {
			DiffArtifact   string `json:"diffArtifact"`
			ImplementerJob string `json:"implementerJob"`
			ReviewedTree   string `json:"reviewedTree"`
		}
		if json.Unmarshal(data, &review) != nil || review.DiffArtifact != "diff.patch" ||
			!landingID.MatchString(review.ImplementerJob) || !treeOID.MatchString(review.ReviewedTree) {
			continue
		}
		diffPath := filepath.Join(roundDir, review.DiffArtifact)
		diffInfo, err := os.Stat(diffPath)
		if err != nil || !diffInfo.Mode().IsRegular() {
			continue
		}
		patch, err := os.ReadFile(diffPath)
		if err != nil {
			continue
		}
		outputs = append(outputs, certifiedOutput{
			round: round, implementerJob: review.ImplementerJob,
			reviewedTree: review.ReviewedTree, patch: patch,
		})
	}
	if len(outputs) == 0 {
		return certifiedOutput{}, fmt.Errorf("chain has no parseable conformance output")
	}
	if len(outputs) == 1 {
		return outputs[0], nil
	}
	if reviewedTree := closedCriticReviewedTree(root, rootRecord); reviewedTree != "" {
		for _, output := range outputs {
			if output.reviewedTree == reviewedTree {
				return output, nil
			}
		}
	}
	if terminal := terminalImplementerJob(root, chain); terminal != "" {
		for _, output := range outputs {
			if output.implementerJob == terminal {
				return output, nil
			}
		}
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].round > outputs[j].round })
	return outputs[0], nil
}

func closedCriticReviewedTree(root string, rootRecord map[string]any) string {
	critic, _ := rootRecord["independentCritiqueJobRef"].(string)
	if !landingID.MatchString(critic) {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", critic+".json"))
	if err != nil {
		return ""
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil || record["jobId"] != critic || record["status"] != "completed" {
		return ""
	}
	round, ok := jsonInteger(record["round"])
	if !ok || round < 1 {
		return ""
	}
	data, err = os.ReadFile(filepath.Join(root, "artifacts", "agents", critic, "rounds", strconv.Itoa(round), "return.json"))
	if err != nil {
		return ""
	}
	var result map[string]any
	if json.Unmarshal(data, &result) != nil {
		return ""
	}
	reviewed, _ := result["reviewedTree"].(string)
	if !treeOID.MatchString(reviewed) {
		return ""
	}
	return reviewed
}

func terminalImplementerJob(root, chain string) string {
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	records := map[string]map[string]any{}
	for _, recordPath := range paths {
		data, err := os.ReadFile(recordPath)
		if err != nil {
			continue
		}
		var record map[string]any
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		id, _ := record["jobId"].(string)
		if landingID.MatchString(id) {
			records[id] = record
		}
	}
	bestID, bestRound := "", 0
	for id, record := range records {
		if record["role"] != "implementer" || lineageRoot(records, id) != chain {
			continue
		}
		round, ok := jsonInteger(record["round"])
		if ok && round > bestRound {
			bestID, bestRound = id, round
		}
	}
	return bestID
}

func lineageRoot(records map[string]map[string]any, id string) string {
	seen := map[string]bool{}
	for {
		if seen[id] {
			return ""
		}
		seen[id] = true
		record, ok := records[id]
		if !ok {
			return ""
		}
		parent, present := record["parentJob"]
		if !present || parent == nil {
			return id
		}
		id, ok = parent.(string)
		if !ok {
			return ""
		}
	}
}

func jsonInteger(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		integer := int(number)
		return integer, number == float64(integer)
	case json.Number:
		integer, err := strconv.Atoi(number.String())
		return integer, err == nil
	default:
		return 0, false
	}
}

// bindCertifiedChange applies the certified patch to the landing's current
// base, then compares canonical change digests over exactly the certified
// paths. Unrelated base movement and bundled carriage paths are outside that
// digest; a changed certified blob, mode, addition, or deletion changes it.
func bindCertifiedChange(root, candidateTree string, output certifiedOutput) (string, []string, error) {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return "", nil, err
	}
	expectedTree, err := workspace.Apply(baseTree, output.patch)
	if err != nil {
		return "", nil, err
	}
	certifiedPaths, err := workspace.ChangedPaths(baseTree, expectedTree)
	if err != nil || len(certifiedPaths) == 0 {
		return "", nil, fmt.Errorf("certified diff has no changed paths")
	}
	// The patch itself is not enough: its postimage must still be the one
	// named by conformance's reviewedTree.
	expectedEntries, err := workspace.Entries(expectedTree, certifiedPaths)
	if err != nil {
		return "", nil, err
	}
	reviewedEntries, err := workspace.Entries(output.reviewedTree, certifiedPaths)
	if err != nil || !reflect.DeepEqual(expectedEntries, reviewedEntries) {
		return "", nil, fmt.Errorf("certified diff and reviewed tree disagree")
	}
	want, err := pathChangeDigest(workspace, baseTree, expectedTree, certifiedPaths)
	if err != nil {
		return "", nil, err
	}
	got, err := pathChangeDigest(workspace, baseTree, candidateTree, certifiedPaths)
	if err != nil || got != want {
		return "", nil, fmt.Errorf("landing changed certified output")
	}
	landingPaths, err := workspace.ChangedPaths(baseTree, candidateTree)
	if err != nil {
		return "", nil, err
	}
	certified := map[string]bool{}
	for _, changedPath := range certifiedPaths {
		certified[changedPath] = true
	}
	var extras []string
	for _, changedPath := range landingPaths {
		if !certified[changedPath] {
			extras = append(extras, changedPath)
		}
	}
	return want, extras, nil
}

func pathChangeDigest(workspace gittree.Workspace, fromTree, toTree string, paths []string) (string, error) {
	ordered := append([]string(nil), paths...)
	sort.Strings(ordered)
	before, err := workspace.Entries(fromTree, ordered)
	if err != nil {
		return "", err
	}
	after, err := workspace.Entries(toTree, ordered)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, changedPath := range ordered {
		fmt.Fprintf(hash, "%d:%s\n", len(changedPath), changedPath)
		writeDigestEntry(hash, before[changedPath])
		writeDigestEntry(hash, after[changedPath])
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type digestWriter interface {
	Write([]byte) (int, error)
}

func writeDigestEntry(writer digestWriter, entry gittree.Entry) {
	if entry.Mode == "" {
		fmt.Fprintln(writer, "absent")
		return
	}
	fmt.Fprintf(writer, "%s %s\n", entry.Mode, entry.OID)
}

type carriageError struct {
	code string
	err  error
}

func (e *carriageError) Error() string { return e.err.Error() }

func carriageRefusalCode(err error) string {
	var carriage *carriageError
	if errors.As(err, &carriage) {
		return carriage.code
	}
	return "register-carriage-policy-unreadable"
}

type landingClassManifest struct {
	SchemaVersion       int `json:"schemaVersion"`
	EnginePolicyVersion int `json:"enginePolicyVersion"`
	Classes             []struct {
		ID             string   `json:"id"`
		PathRule       string   `json:"pathRule"`
		RequiredFields []string `json:"requiredFields"`
		AuthorizedBy   string   `json:"authorizedBy"`
	} `json:"classes"`
}

type carriagePattern struct {
	exact  string
	dir    string
	prefix string
}

func registerCarriage(root, candidateTree string, changedPaths []string) error {
	if len(changedPaths) == 0 {
		return nil
	}
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return err
	}
	patterns, err := loadCarriagePolicy(workspace, baseTree)
	if err != nil {
		return err
	}
	var disallowed []string
	for _, changedPath := range changedPaths {
		if !carriagePathAllowed(changedPath, patterns) {
			disallowed = append(disallowed, changedPath)
		}
	}
	for _, changedPath := range disallowed {
		if neverDirectFix(changedPath) {
			return &carriageError{code: "direct-fix-floor-refused", err: fmt.Errorf("path %s is on the never-direct-fix floor", changedPath)}
		}
	}
	if len(disallowed) > 0 {
		return &carriageError{code: "register-carriage-path-refused", err: fmt.Errorf("path %s is not register carriage", disallowed[0])}
	}
	for _, changedPath := range changedPaths {
		switch changedPath {
		case "memory/rulings.md":
			if err := addRulingRowsOnly(workspace, baseTree, candidateTree); err != nil {
				return &carriageError{code: "register-carriage-not-append-only", err: err}
			}
		case "memory/receipts.log", "records/narrator-digest.log":
			if err := appendOnly(workspace, baseTree, candidateTree, changedPath); err != nil {
				return &carriageError{code: "register-carriage-not-append-only", err: err}
			}
		}
	}
	return nil
}

func loadCarriagePolicy(workspace gittree.Workspace, baseTree string) ([]carriagePattern, error) {
	if err := loadLandingClasses(workspace, baseTree); err != nil {
		return nil, err
	}
	allowlist, present, err := workspace.FileAt(baseTree, "scripts/agents/register-carriage-paths.txt")
	if err != nil || !present {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("register carriage allowlist is unreadable")}
	}
	return parseCarriageAllowlist(allowlist)
}

func loadLandingClasses(workspace gittree.Workspace, baseTree string) error {
	manifestBytes, present, err := workspace.FileAt(baseTree, "scripts/agents/landing-classes.json")
	if err != nil || !present {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is unreadable")}
	}
	var manifest landingClassManifest
	if json.Unmarshal(manifestBytes, &manifest) != nil || manifest.SchemaVersion != 1 ||
		manifest.EnginePolicyVersion != 1 || len(manifest.Classes) != 2 {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is malformed")}
	}
	rulings, present, err := workspace.FileAt(baseTree, "memory/rulings.md")
	if err != nil || !present {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("rulings register is unreadable")}
	}
	rulingRows := parseRulingRows(rulings)
	found := map[string]bool{}
	for _, class := range manifest.Classes {
		if found[class.ID] || !rulingID.MatchString(class.AuthorizedBy) || !rulingRows[class.AuthorizedBy] {
			return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest is malformed")}
		}
		switch class.ID {
		case "register-carriage":
			if class.PathRule != "register-carriage-allowlist" || len(class.RequiredFields) != 0 {
				return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest has the wrong carriage rule")}
			}
		case "exact-revert":
			if class.PathRule != "tree-shaped-exact-inverse" || !reflect.DeepEqual(class.RequiredFields, []string{"revert-of"}) {
				return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest has the wrong exact-revert rule")}
			}
		default:
			return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest contains an unknown class")}
		}
		found[class.ID] = true
	}
	if !found["register-carriage"] || !found["exact-revert"] {
		return &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("landing class manifest omits a compiled class")}
	}
	return nil
}

func parseCarriageAllowlist(data []byte) ([]carriagePattern, error) {
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	patterns := make([]carriagePattern, 0, len(lines))
	wildcards := 0
	for _, line := range lines {
		if line == "" || line != filepath.ToSlash(filepath.Clean(line)) || strings.HasPrefix(line, "/") || strings.HasPrefix(line, "../") {
			return nil, &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("register carriage allowlist contains an invalid path")}
		}
		if strings.ContainsAny(line, "*?[") {
			base := path.Base(line)
			if strings.Count(line, "*") != 1 || strings.ContainsAny(line, "?[") ||
				!strings.HasSuffix(base, "-*.md") || strings.Contains(path.Dir(line), "*") {
				return nil, &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("register carriage allowlist contains an invalid glob")}
			}
			wildcards++
			patterns = append(patterns, carriagePattern{
				dir: path.Dir(line), prefix: strings.TrimSuffix(base, "*.md"),
			})
			continue
		}
		patterns = append(patterns, carriagePattern{exact: line})
	}
	if wildcards > 1 {
		return nil, &carriageError{code: "register-carriage-policy-unreadable", err: fmt.Errorf("register carriage allowlist contains more than one glob")}
	}
	return patterns, nil
}

func carriagePathAllowed(changedPath string, patterns []carriagePattern) bool {
	clean := filepath.ToSlash(filepath.Clean(changedPath))
	for _, pattern := range patterns {
		if pattern.exact == clean {
			return true
		}
		if pattern.dir != "" && path.Dir(clean) == pattern.dir {
			name := path.Base(clean)
			if strings.HasPrefix(name, pattern.prefix) && strings.HasSuffix(name, ".md") {
				return true
			}
		}
	}
	return false
}

func appendOnly(workspace gittree.Workspace, baseTree, candidateTree, changedPath string) error {
	baseEntries, err := workspace.Entries(baseTree, []string{changedPath})
	if err != nil {
		return err
	}
	candidateEntries, err := workspace.Entries(candidateTree, []string{changedPath})
	if err != nil {
		return err
	}
	if baseEntry, existed := baseEntries[changedPath]; existed {
		if candidateEntry, present := candidateEntries[changedPath]; !present || candidateEntry.Mode != baseEntry.Mode {
			return fmt.Errorf("append-only register %s was deleted or changed mode", changedPath)
		}
	}
	before, existed, err := workspace.FileAt(baseTree, changedPath)
	if err != nil {
		return err
	}
	after, present, err := workspace.FileAt(candidateTree, changedPath)
	if err != nil {
		return err
	}
	if !present || len(after) == 0 || after[len(after)-1] != '\n' {
		return fmt.Errorf("append-only register %s was deleted or has an incomplete appended line", changedPath)
	}
	if !existed {
		return nil
	}
	if len(before) > 0 && before[len(before)-1] != '\n' {
		return fmt.Errorf("append-only register %s has an unterminated existing line", changedPath)
	}
	if len(after) <= len(before) || !bytes.Equal(after[:len(before)], before) {
		return fmt.Errorf("append-only register %s deletes or rewrites an existing line", changedPath)
	}
	return nil
}

func addRulingRowsOnly(workspace gittree.Workspace, baseTree, candidateTree string) error {
	const rulingsPath = "memory/rulings.md"
	before, existed, err := workspace.FileAt(baseTree, rulingsPath)
	if err != nil {
		return err
	}
	if !existed {
		return fmt.Errorf("rulings register does not exist in the landing base")
	}
	if err := appendOnly(workspace, baseTree, candidateTree, rulingsPath); err != nil {
		return err
	}
	after, _, err := workspace.FileAt(candidateTree, rulingsPath)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(after[len(before):]), "\n"), "\n") {
		if _, ok := rulingRowID(line); !ok {
			return fmt.Errorf("rulings carriage appended a malformed ruling row")
		}
	}
	return nil
}

func parseRulingRows(data []byte) map[string]bool {
	rows := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if id, ok := rulingRowID(line); ok {
			rows[id] = true
		}
	}
	return rows
}

func rulingRowID(line string) (string, bool) {
	if !strings.HasPrefix(line, "| ") {
		return "", false
	}
	remainder := strings.TrimPrefix(line, "| ")
	id, remainder, ok := strings.Cut(remainder, " |")
	if !ok || !rulingID.MatchString(id) || (remainder != "" && !strings.HasPrefix(remainder, " ")) {
		return "", false
	}
	return id, true
}

func observeDirectFix(params ObserveParams, change string) Observation {
	switch params.DirectFix {
	case "register-carriage":
		provenance := "direct-fix class=register-carriage change=" + change
		workspace := gittree.Workspace{Dir: params.RepoRoot}
		baseTree, err := workspace.HeadTree()
		if err != nil {
			return wouldRefuse("register-carriage-policy-unreadable", provenance)
		}
		paths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
		if err != nil {
			return wouldRefuse("register-carriage-policy-unreadable", provenance)
		}
		if err := registerCarriage(params.RepoRoot, params.CandidateTree, paths); err != nil {
			return wouldRefuse(carriageRefusalCode(err), provenance)
		}
		return pass(BarDirectFix, "register-carriage", provenance)
	case "exact-revert":
		provenance := "direct-fix class=exact-revert change=" + change
		if !treeOID.MatchString(params.RevertOf) {
			return wouldRefuse("malformed-revert-commit", provenance)
		}
		workspace := gittree.Workspace{Dir: params.RepoRoot}
		baseTree, err := workspace.HeadTree()
		if err != nil || loadLandingClasses(workspace, baseTree) != nil {
			return wouldRefuse("direct-fix-policy-unreadable", provenance)
		}
		provenance = fmt.Sprintf("direct-fix class=exact-revert revert-of=%s change=%s", params.RevertOf, change)
		if err := exactRevert(params.RepoRoot, params.CandidateTree, params.RevertOf); err != nil {
			return wouldRefuse(exactRevertRefusalCode(err), provenance)
		}
		return pass(BarDirectFix, "exact-revert", provenance)
	default:
		return wouldRefuse("unknown-direct-fix-class", "invalid change="+change)
	}
}

type exactRevertError struct {
	code string
	err  error
}

func (e *exactRevertError) Error() string { return e.err.Error() }

func exactRevertRefusalCode(err error) string {
	var revert *exactRevertError
	if errors.As(err, &revert) {
		return revert.code
	}
	return "not-exact-revert"
}

func exactRevert(root, candidateTree, revertOf string) error {
	workspace := gittree.Workspace{Dir: root}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return err
	}
	candidatePaths, candidateErr := workspace.ChangedPaths(baseTree, candidateTree)
	parent, err := workspace.SingleParent(revertOf)
	if err != nil {
		return exactRevertFloorError(candidatePaths, nil, err)
	}
	preimageTree, err := workspace.TreeOf(parent)
	if err != nil {
		return exactRevertFloorError(candidatePaths, nil, err)
	}
	postimageTree, err := workspace.TreeOf(revertOf)
	if err != nil {
		return exactRevertFloorError(candidatePaths, nil, err)
	}
	targetPaths, err := workspace.ChangedPaths(preimageTree, postimageTree)
	if err != nil || len(targetPaths) == 0 {
		return exactRevertFloorError(candidatePaths, targetPaths, fmt.Errorf("reverted commit has no decidable changed paths"))
	}
	if err := exactRevertFloorError(candidatePaths, targetPaths, nil); err != nil {
		return err
	}
	if candidateErr != nil {
		return candidateErr
	}
	sort.Strings(targetPaths)
	sort.Strings(candidatePaths)
	if !reflect.DeepEqual(candidatePaths, targetPaths) {
		return fmt.Errorf("candidate changes paths outside the exact inverse")
	}
	baseEntries, err := workspace.Entries(baseTree, targetPaths)
	if err != nil {
		return err
	}
	postimageEntries, err := workspace.Entries(postimageTree, targetPaths)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(baseEntries, postimageEntries) {
		return fmt.Errorf("current base no longer carries the reverted commit postimage")
	}
	candidateEntries, err := workspace.Entries(candidateTree, targetPaths)
	if err != nil {
		return err
	}
	preimageEntries, err := workspace.Entries(preimageTree, targetPaths)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(candidateEntries, preimageEntries) {
		return fmt.Errorf("candidate is not the exact tree-shaped inverse")
	}
	return nil
}

func exactRevertFloorError(candidatePaths, targetPaths []string, fallback error) error {
	for _, changedPath := range append(append([]string(nil), targetPaths...), candidatePaths...) {
		if neverDirectFix(changedPath) {
			return &exactRevertError{
				code: "direct-fix-floor-refused",
				err:  fmt.Errorf("path %s is on the never-direct-fix floor", changedPath),
			}
		}
	}
	return fallback
}

// The initial floor is deliberately broad. Register carriage is the exact
// allowlisted exception; anything else that can change instructions,
// authority, orchestration, or landing policy takes an implementation chain.
func neverDirectFix(path string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	for _, exact := range []string{
		"AGENTS.md", "CLAUDE.md", "wow.md", "metasystem.conf",
		"go.mod", "go.sum", "go.work", "go.work.sum", ".gitattributes", ".gitignore",
		"bin/metasystem",
	} {
		if clean == exact || strings.HasSuffix(clean, "/"+exact) {
			return true
		}
	}
	for _, prefix := range []string{
		".agents/", ".claude/", ".codex/", ".github/", "bin/", "cmd/", "docs/", "internal/",
		"memory/", "optional-skills/", "plans/", "records/", "roles/", "schemas/",
		"scripts/", "skills/", "templates/",
	} {
		if strings.HasPrefix(clean, prefix) || strings.Contains(clean, "/"+prefix) {
			return true
		}
	}
	return false
}

func pass(bar, code, provenance string) Observation {
	return Observation{
		SchemaVersion: 1, Mode: "observe", Bar: bar, Verdict: "pass", Code: code,
		Provenance: provenance, VerdictTrailer: "pass bar=" + bar,
	}
}

func wouldRefuse(code, provenance string) Observation {
	return Observation{
		SchemaVersion: 1, Mode: "observe", Bar: BarRefusal, Verdict: "would-refuse", Code: code,
		Provenance: provenance, VerdictTrailer: "would-refuse code=" + code,
	}
}
