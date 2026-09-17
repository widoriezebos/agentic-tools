package branch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

const (
	ReadUngatedCode      = "GOAL_READ_UNGATED"
	ReadTestsUnnamedCode = "GOAL_READ_TESTS_UNNAMED"
	ReadStaleCode        = "GOAL_READ_STALE"
	ReadInvalidCode      = "GOAL_READ_INVALID"
)

type AttestationSubject struct {
	Commit      string `json:"commit"`
	Parent      string `json:"parent"`
	Tree        string `json:"tree"`
	PatchDigest string `json:"patchDigest"`
	UnitDigest  string `json:"unitDigest"`
}

type AttestationSource struct {
	Kind         string `json:"kind"`
	RootJob      string `json:"rootJob,omitempty"`
	Round        int64  `json:"round,omitempty"`
	ReaderRecord string `json:"readerRecord,omitempty"`
	RecordSHA256 string `json:"recordSha256,omitempty"`
}

type GateObservation struct {
	Kind  string `json:"kind"`
	Tree  string `json:"tree"`
	RunID string `json:"runId"`
}

type Fold struct {
	Kind   Kind   `json:"kind"`
	Commit string `json:"commit"`
	Digest string `json:"digest"`
}

type Carry struct {
	FromCommit string `json:"fromCommit"`
	FromTree   string `json:"fromTree"`
	ToCommit   string `json:"toCommit"`
	ToTree     string `json:"toTree"`
}

type TestChange struct {
	Path       string `json:"path"`
	ReaderWord string `json:"readerWord"`
}

type Attestation struct {
	SchemaVersion int                `json:"schemaVersion"`
	Goal          string             `json:"goal"`
	Unit          string             `json:"unit"`
	Subject       AttestationSubject `json:"subject"`
	Source        AttestationSource  `json:"source"`
	Verdict       string             `json:"verdict"`
	Gate          GateObservation    `json:"gate"`
	Folds         []Fold             `json:"folds"`
	Carry         *Carry             `json:"carry,omitempty"`
	TestsChanged  []TestChange       `json:"testsChanged"`
	SHA256        string             `json:"sha256"`
}

type CommitReadRequest struct {
	Repo, Remote, EndpointTip, GoalID, Unit, OpID string
	Units                                         []string
	CheckClaim                                    func() error
	RootJob, ReaderRecord, Carry                  string
	GateRunID, GateTree                           string
	TestsChanged                                  []TestChange
	Transport                                     PushTransport
}

func attestationPath(goalID, commit string) string {
	return "metasystem/records/reads/" + goalID + "/" + commit + ".json"
}

func computeSubject(repo, commit string) (AttestationSubject, readsubject.ReadSubject, error) {
	read, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{
		RepoRoot: repo, Role: "code-critic", Reviews: "commit:" + commit,
	})
	if err != nil || !present {
		return AttestationSubject{}, readsubject.ReadSubject{}, fmt.Errorf("commit subject %s is unreadable: %v", commit, err)
	}
	digest, err := UnitDigest(repo, commit)
	if err != nil {
		return AttestationSubject{}, readsubject.ReadSubject{}, err
	}
	return AttestationSubject{
		Commit: read.Commit, Parent: read.Parent, Tree: read.Tree,
		PatchDigest: read.DiffDigest, UnitDigest: digest,
	}, read, nil
}

func foldRange(repo, endpointTip, unitCommit, goalID string) ([]Fold, error) {
	commits, err := ValidateRange(repo, endpointTip, unitCommit, goalID)
	if err != nil {
		return nil, err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		if commits[i].ID != unitCommit {
			continue
		}
		var folds []Fold
		for _, commit := range commits[:i] {
			if commit.Kind == Unit {
				continue
			}
			digest, err := UnitDigest(repo, commit.ID)
			if err != nil {
				return nil, err
			}
			folds = append(folds, Fold{Kind: commit.Kind, Commit: commit.ID, Digest: digest})
		}
		return folds, nil
	}
	return nil, operationRefusal(ReadInvalidCode, "unit commit %s is outside the goal range", unitCommit)
}

func digestAttestation(att Attestation) (string, error) {
	att.SHA256 = ""
	data, err := json.Marshal(att)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func isExistingTestPath(path, sourceBlob string) bool {
	if strings.Trim(sourceBlob, "0") == "" {
		return false
	}
	clean := filepath.ToSlash(path)
	base := filepath.Base(clean)
	return strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".bats") || strings.HasSuffix(base, "-fixtures.sh") ||
		strings.Contains(clean, "/testdata/") || strings.HasPrefix(clean, "tests/") || strings.Contains(clean, "/tests/")
}

func requiredTestChanges(repo, commit string) ([]string, error) {
	entries, err := RawEntries(repo, commit)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if isExistingTestPath(entry.Path, entry.SrcBlob) {
			paths = append(paths, entry.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func validateTestChanges(repo, commit string, supplied []TestChange) error {
	required, err := requiredTestChanges(repo, commit)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var got []string
	for _, item := range supplied {
		if strings.TrimSpace(item.ReaderWord) == "" || seen[item.Path] {
			return operationRefusal(ReadTestsUnnamedCode, "every changed test needs one nonempty reader word")
		}
		seen[item.Path] = true
		got = append(got, item.Path)
	}
	sort.Strings(got)
	if strings.Join(required, "\x00") != strings.Join(got, "\x00") {
		return operationRefusal(ReadTestsUnnamedCode, "changed tests are %v, but the attestation names %v", required, got)
	}
	return nil
}

func safeReaderRecord(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == path && strings.HasPrefix(clean, "metasystem/records/misc/") && !strings.Contains(clean, "../")
}

func fileSHA256(path string) (string, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

func fileAt(repo, snapshot, path string) ([]byte, error) {
	if snapshot == "" {
		return os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
	}
	data, err := gitOutput(repo, "show", snapshot+":"+path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func fileSHA256At(repo, snapshot, path string) (string, []byte, error) {
	data, err := fileAt(repo, snapshot, path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

func directSource(req CommitReadRequest, subject AttestationSubject, read readsubject.ReadSubject) (AttestationSource, error) {
	if (req.RootJob == "") == (req.ReaderRecord == "") {
		return AttestationSource{}, fmt.Errorf("read needs exactly one of --root-job or --reader-record")
	}
	if req.RootJob != "" {
		closure, err := dispatch.ValidateCommitCriticClosure(req.Repo, req.RootJob, read)
		if err != nil {
			return AttestationSource{}, err
		}
		return AttestationSource{Kind: "critic-root", RootJob: req.RootJob, Round: closure.Round}, nil
	}
	if !safeReaderRecord(req.ReaderRecord) {
		return AttestationSource{}, fmt.Errorf("reader record must be a repository-relative path under metasystem/records/misc")
	}
	digest, data, err := fileSHA256(filepath.Join(req.Repo, filepath.FromSlash(req.ReaderRecord)))
	if err != nil {
		return AttestationSource{}, err
	}
	text := string(data)
	if !strings.Contains(text, subject.Commit) || !strings.Contains(text, subject.UnitDigest) {
		return AttestationSource{}, fmt.Errorf("reader record must name commit %s and unit digest %s", subject.Commit, subject.UnitDigest)
	}
	return AttestationSource{Kind: "reader-record", ReaderRecord: req.ReaderRecord, RecordSHA256: digest}, nil
}

func sameFoldDigests(a, b []Fold) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Digest != b[i].Digest {
			return false
		}
	}
	return true
}

func readAttestationAt(repo, snapshot, goalID, commit string) (Attestation, error) {
	rel := attestationPath(goalID, commit)
	data, err := fileAt(repo, snapshot, rel)
	if err != nil {
		return Attestation{}, err
	}
	var att Attestation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&att); err != nil {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s is malformed: %v", rel, err)
	}
	canonical, err := json.MarshalIndent(att, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s is not in its canonical form", rel)
	}
	return att, nil
}

func validateAttestation(repo, snapshot, endpointTip, goalID, unit, commit string, seen map[string]bool) (Attestation, error) {
	if seen[commit] {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation carry cycle at %s", commit)
	}
	seen[commit] = true
	att, err := readAttestationAt(repo, snapshot, goalID, commit)
	if err != nil {
		return Attestation{}, err
	}
	if att.SchemaVersion != 1 || att.Goal != goalID || att.Unit != unit || att.Verdict != "LAND" || att.Subject.Commit != commit {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation identity or verdict does not match %s/%s at %s", goalID, unit, commit)
	}
	digest, err := digestAttestation(att)
	if err != nil || digest != att.SHA256 {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s fails its self-digest", commit)
	}
	subject, read, err := computeSubject(repo, commit)
	if err != nil || subject != att.Subject {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s does not match its commit subject", commit)
	}
	folds, err := foldRange(repo, endpointTip, commit, goalID)
	if err != nil || !sameFoldDigests(folds, att.Folds) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s does not match its fold range", commit)
	}
	if att.Gate.Kind != "go-gate-fast" || att.Gate.RunID == "" || att.Gate.Tree != subject.Tree {
		return Attestation{}, operationRefusal(ReadUngatedCode, "attestation %s has no fast-gate observation on tree %s", commit, subject.Tree)
	}
	if err := validateTestChanges(repo, commit, att.TestsChanged); err != nil {
		return Attestation{}, err
	}
	if att.Carry != nil {
		prior, err := validateAttestation(repo, snapshot, endpointTip, goalID, unit, att.Carry.FromCommit, seen)
		if err != nil {
			return Attestation{}, err
		}
		if att.Carry.FromTree != prior.Subject.Tree || att.Carry.ToCommit != commit || att.Carry.ToTree != subject.Tree ||
			prior.Subject.UnitDigest != subject.UnitDigest || !sameFoldDigests(prior.Folds, folds) || prior.Source != att.Source {
			return Attestation{}, operationRefusal(ReadStaleCode, "carry from %s does not preserve the unit and fold bytes", att.Carry.FromCommit)
		}
		return att, nil
	}
	switch att.Source.Kind {
	case "critic-root":
		closure, err := dispatch.ValidateCommitCriticClosure(repo, att.Source.RootJob, read)
		if err != nil || closure.Round != att.Source.Round {
			return Attestation{}, operationRefusal(ReadInvalidCode, "critic source for %s is not a clean bound closure", commit)
		}
	case "reader-record":
		if !safeReaderRecord(att.Source.ReaderRecord) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "reader record path is outside records/misc")
		}
		digest, data, err := fileSHA256At(repo, snapshot, att.Source.ReaderRecord)
		if err != nil || digest != att.Source.RecordSHA256 || !strings.Contains(string(data), commit) || !strings.Contains(string(data), subject.UnitDigest) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "reader record for %s is missing or changed", commit)
		}
	default:
		return Attestation{}, operationRefusal(ReadInvalidCode, "attestation %s has unknown source %q", commit, att.Source.Kind)
	}
	return att, nil
}

func ValidateAttestation(repo, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(repo, "", endpointTip, goalID, unit, commit, map[string]bool{})
}

// ValidateAttestationAt validates the evidence as it exists in one fetched
// branch snapshot while keeping local critic artifacts available at repo.
func ValidateAttestationAt(repo, snapshot, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(repo, snapshot, endpointTip, goalID, unit, commit, map[string]bool{})
}

func unitCommitInRange(repo, endpointTip, tip, goalID string, units []string) (string, error) {
	commits, err := ValidateRange(repo, endpointTip, tip, goalID)
	if err != nil {
		return "", err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		if commits[i].Kind == Unit && sameUnits(commits[i].Units, units) {
			return commits[i].ID, nil
		}
	}
	return "", operationRefusal(ReadInvalidCode, "goal branch has no build %s", unitList(units))
}

func prospectiveReadPatch(repo, attestationPath string, data []byte, readerRecord string) ([]byte, error) {
	scratch, err := os.MkdirTemp("", "goal-read-index-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	tree, err := gitOutput(repo, "write-tree")
	if err != nil {
		return nil, err
	}
	if _, err := gitInputEnv(repo, env, nil, "read-tree", strings.TrimSpace(string(tree))); err != nil {
		return nil, err
	}
	blob, err := gitInput(repo, data, "hash-object", "-w", "--path="+attestationPath, "--stdin")
	if err != nil {
		return nil, err
	}
	entry := "100644," + strings.TrimSpace(string(blob)) + "," + attestationPath
	if _, err := gitInputEnv(repo, env, nil, "update-index", "--add", "--cacheinfo", entry); err != nil {
		return nil, err
	}
	if readerRecord != "" {
		if _, err := gitInputEnv(repo, env, nil, "add", "--", readerRecord); err != nil {
			return nil, err
		}
	}
	return gitInputEnv(repo, env, nil, "diff", "--cached", "--binary", "--full-index")
}

func CommitRead(req CommitReadRequest) (string, Attestation, error) {
	unitRequest := CommitRequest{Unit: req.Unit, Units: req.Units}
	units, err := requestUnits(unitRequest)
	if err != nil || len(units) == 0 {
		return "", Attestation{}, fmt.Errorf("read commits need one or more distinct unit names")
	}
	list := unitList(units)
	commitReq := CommitRequest{
		Repo: req.Repo, Remote: req.Remote, EndpointTip: req.EndpointTip, GoalID: req.GoalID,
		Units: units, OpID: req.OpID, Kind: Read, CheckClaim: req.CheckClaim, Transport: req.Transport,
	}
	if err := CheckCommitAccess(req.GoalID, req.CheckClaim); err != nil {
		return "", Attestation{}, err
	}
	state, err := inspectCommitBranch(commitReq)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateRunID == "" || req.GateTree == "" {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "a fast-gate run id and tree are required")
	}
	unitCommit, err := unitCommitInRange(req.Repo, req.EndpointTip, state.baseTip, req.GoalID, units)
	if err != nil {
		return "", Attestation{}, err
	}
	subject, read, err := computeSubject(req.Repo, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateTree != subject.Tree {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "fast gate observed tree %s, not unit tree %s", req.GateTree, subject.Tree)
	}
	if err := validateTestChanges(req.Repo, unitCommit, req.TestsChanged); err != nil {
		return "", Attestation{}, err
	}
	folds, err := foldRange(req.Repo, req.EndpointTip, unitCommit, req.GoalID)
	if err != nil {
		return "", Attestation{}, err
	}
	att := Attestation{
		SchemaVersion: 1, Goal: req.GoalID, Unit: list, Subject: subject,
		Verdict: "LAND", Gate: GateObservation{Kind: "go-gate-fast", Tree: req.GateTree, RunID: req.GateRunID},
		Folds: folds, TestsChanged: append([]TestChange(nil), req.TestsChanged...),
	}
	sort.Slice(att.TestsChanged, func(i, j int) bool { return att.TestsChanged[i].Path < att.TestsChanged[j].Path })
	if req.Carry != "" {
		prior, err := ValidateAttestation(req.Repo, req.EndpointTip, req.GoalID, list, req.Carry)
		if err != nil {
			return "", Attestation{}, err
		}
		if prior.Subject.UnitDigest != subject.UnitDigest || !sameFoldDigests(prior.Folds, folds) {
			return "", Attestation{}, operationRefusal(ReadStaleCode, "carry from %s does not preserve the unit and fold bytes", req.Carry)
		}
		att.Source = prior.Source
		att.Carry = &Carry{FromCommit: req.Carry, FromTree: prior.Subject.Tree, ToCommit: unitCommit, ToTree: subject.Tree}
	} else {
		att.Source, err = directSource(req, subject, read)
		if err != nil {
			return "", Attestation{}, err
		}
	}
	att.SHA256, err = digestAttestation(att)
	if err != nil {
		return "", Attestation{}, err
	}
	data, err := json.MarshalIndent(att, "", "  ")
	if err != nil {
		return "", Attestation{}, err
	}
	data = append(data, '\n')
	rel := attestationPath(req.GoalID, unitCommit)
	paths, err := stagedPaths(req.Repo)
	if err != nil {
		return "", Attestation{}, err
	}
	paths = append(paths, rel)
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		paths = append(paths, att.Source.ReaderRecord)
	}
	if err := validateCommitPaths(Read, paths, req.GoalID); err != nil {
		return "", Attestation{}, err
	}
	readerRecord := ""
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		readerRecord = att.Source.ReaderRecord
	}
	if err := adoptionCheckoutClean(commitReq, state, []string{rel, readerRecord}); err != nil {
		return "", Attestation{}, err
	}
	patch, err := prospectiveReadPatch(req.Repo, rel, data, readerRecord)
	if err != nil {
		return "", Attestation{}, err
	}
	subjectLine, trailer, err := commitMessage(commitReq, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	preparedTip, err := buildCommitOnto(commitReq, state, subjectLine, trailer, patch)
	if err != nil {
		return "", Attestation{}, err
	}
	abs := filepath.Join(req.Repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", Attestation{}, err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", Attestation{}, err
	}
	paths = []string{rel}
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		paths = append(paths, att.Source.ReaderRecord)
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := gitOutput(req.Repo, args...); err != nil {
		return "", Attestation{}, err
	}
	if err := installCommitOnto(commitReq, state, preparedTip); err != nil {
		return "", Attestation{}, err
	}
	return preparedTip, att, nil
}
