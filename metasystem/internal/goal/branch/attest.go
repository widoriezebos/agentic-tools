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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

const (
	ReadUngatedCode      = "GOAL_READ_UNGATED"
	ReadTestsUnnamedCode = "GOAL_READ_TESTS_UNNAMED"
	ReadStaleCode        = "GOAL_READ_STALE"
	ReadInvalidCode      = "GOAL_READ_INVALID"
	// ReadBriefChangedCode refuses a read of a build whose review already
	// started with another brief, runtime or model.
	ReadBriefChangedCode = "GOAL_READ_BRIEF_CHANGED"
)

type AttestationSubject struct {
	Commit      string `json:"commit"`
	Parent      string `json:"parent"`
	Tree        string `json:"tree"`
	PatchDigest string `json:"patchDigest"`
	UnitDigest  string `json:"unitDigest"`
}

type AttestationSource struct {
	Kind          string `json:"kind"`
	ReadLaunch    string `json:"readLaunch,omitempty"`
	RootJob       string `json:"rootJob,omitempty"`
	Round         int64  `json:"round,omitempty"`
	ClosureSHA256 string `json:"closureSha256,omitempty"`
	ReaderRecord  string `json:"readerRecord,omitempty"`
	RecordSHA256  string `json:"recordSha256,omitempty"`
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

type closureBundle struct {
	SchemaVersion int               `json:"schemaVersion"`
	RootJob       string            `json:"rootJob"`
	Files         map[string]string `json:"files"`
}

// UnitReadBundle preserves the report and launch JSON verbatim as strings.
// The examined base and tree bind the read to one commit's change.
type UnitReadBundle struct {
	SchemaVersion int    `json:"schemaVersion"`
	Goal          string `json:"goal"`
	Commit        string `json:"commit"`
	UnitRun       string `json:"unitRun"`
	Round         int    `json:"round"`
	ReadLaunch    string `json:"readLaunch"`
	ReadRuntime   string `json:"readRuntime"`
	ReadModel     string `json:"readModel"`
	BuildModel    string `json:"buildModel"`
	ExaminedBase  string `json:"examinedBase"`
	ExaminedTree  string `json:"examinedTree"`
	GoalRevision  uint64 `json:"goalRevision"`
	VerdictLine   string `json:"verdictLine"`
	Report        string `json:"report"`
	LaunchRecord  string `json:"launchRecord"`
}

type CommitReadRequest struct {
	Repo, Remote, EndpointTip, GoalID, Unit, OpID string
	Units                                         []string
	CheckClaim                                    func() error
	RootJob, ReaderRecord, Carry                  string
	UnitRead                                      []byte
	GateRunID, GateTree                           string
	TestsChanged                                  []TestChange
	Transport                                     PushTransport
	Inputs                                        *ReadCommitInputs
	GateRepository                                BranchReadRepository
	// CriticStore is the installation whose artifacts/agents holds
	// RootJob's records when that is not Repo (the read owner's
	// CriticStore); "" is Repo.
	CriticStore string
}

type LandedUnit struct {
	Goal, Unit, Digest, CriticRoot, GateRunID string
	ReadLaunch, ReadModel                     string
	Round                                     int64
	GoalRevision                              uint64
	FoldPaths, ChangedPaths                   []string
	HasPlan, Destructive                      bool
}

type LandedUnitError struct {
	Code string
	Err  error
}

func (e *LandedUnitError) Error() string                  { return e.Err.Error() }
func (e *LandedUnitError) Unwrap() error                  { return e.Err }
func (e *LandedUnitError) LandingAttestationCode() string { return e.Code }

func attestationPath(goalID, commit string) string {
	return "metasystem/records/reads/" + goalID + "/" + commit + ".json"
}

func closureBundlePath(goalID, commit string) string {
	return "metasystem/records/reads/" + goalID + "/" + commit + ".closure.json"
}

func computeSubject(repo, commit string) (AttestationSubject, readsubject.ReadSubject, error) {
	return computeSubjectWithReads(gitAttestationReads{}, repo, commit)
}
func computeSubjectWithReads(r attestationReads, repo, commit string) (AttestationSubject, readsubject.ReadSubject, error) {
	read, err := r.ReadSubject(repo, commit)
	if err != nil {
		return AttestationSubject{}, readsubject.ReadSubject{}, err
	}
	raw, err := r.RawEntries(repo, commit)
	if err != nil {
		return AttestationSubject{}, readsubject.ReadSubject{}, err
	}
	return AttestationSubject{Commit: read.Commit, Parent: read.Parent, Tree: read.Tree,
		PatchDigest: read.DiffDigest, UnitDigest: digestRawEntries(raw)}, read, nil
}

func foldRangeWithReads(r attestationReads, repo, endpointTip, unitCommit, goalID string) ([]Fold, error) {
	commits, err := r.Range(repo, endpointTip, unitCommit, goalID)
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
			raw, err := r.RawEntries(repo, commit.ID)
			if err != nil {
				return nil, err
			}
			folds = append(folds, Fold{Kind: commit.Kind, Commit: commit.ID, Digest: digestRawEntries(raw)})
		}
		return folds, nil
	}
	return nil, operationRefusal(ReadInvalidCode, "build %s is not on goal %s's branch\nrun: metasystem work status %s", unitCommit, goalID, goalID)
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

func requiredTestChangesWithReads(r attestationReads, repo, commit string) ([]string, error) {
	raw, err := r.RawEntries(repo, commit)
	if err != nil {
		return nil, err
	}
	entries, err := parseRawEntries(commit, raw)
	if err != nil {
		return nil, err
	}
	return testPathsFromEntries(entries), nil
}

func testPathsFromEntries(entries []Entry) []string {
	var paths []string
	for _, entry := range entries {
		if isExistingTestPath(entry.Path, entry.SrcBlob) {
			paths = append(paths, entry.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func validateTestChangesWithReads(r attestationReads, repo, goalID, commit string, supplied []TestChange) error {
	required, err := requiredTestChangesWithReads(r, repo, commit)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var got []string
	for _, item := range supplied {
		if strings.TrimSpace(item.ReaderWord) == "" || seen[item.Path] {
			return operationRefusal(ReadTestsUnnamedCode, "each changed test needs a reviewer's word, and one has none\nrun: metasystem work review %s", goalID)
		}
		seen[item.Path] = true
		got = append(got, item.Path)
	}
	sort.Strings(got)
	if strings.Join(required, "\x00") != strings.Join(got, "\x00") {
		return operationRefusal(ReadTestsUnnamedCode, "the review names tests %v, but the build changed %v\nrun: metasystem work review %s", got, required, goalID)
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

func fileSHA256At(r attestationReads, repo, snapshot, path string) (string, []byte, error) {
	data, err := attestationFileAt(r, repo, snapshot, path)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

func directSource(r attestationReads, req CommitReadRequest, subject AttestationSubject, read readsubject.ReadSubject) (AttestationSource, []byte, error) {
	if len(req.UnitRead) != 0 {
		if req.RootJob != "" || req.ReaderRecord != "" {
			return AttestationSource{}, nil, fmt.Errorf("read needs exactly one of --root-job, --reader-record or a unit read")
		}
		bundle, err := validateUnitReadBundle(req.UnitRead, req.GoalID, subject)
		if err != nil {
			return AttestationSource{}, nil, err
		}
		sum := sha256.Sum256(req.UnitRead)
		return AttestationSource{Kind: "unit-read", ReadLaunch: bundle.ReadLaunch,
			ClosureSHA256: hex.EncodeToString(sum[:])}, append([]byte(nil), req.UnitRead...), nil
	}
	if (req.RootJob == "") == (req.ReaderRecord == "") {
		return AttestationSource{}, nil, fmt.Errorf("read needs exactly one of --root-job or --reader-record")
	}
	if req.RootJob != "" {
		store := req.Repo
		if req.CriticStore != "" {
			store = req.CriticStore
		}
		closure, files, err := dispatch.CommitCriticClosureFiles(filepath.Join(store, "artifacts", "agents"), req.RootJob, read)
		if err != nil {
			return AttestationSource{}, nil, err
		}
		bundle := closureBundle{SchemaVersion: 1, RootJob: req.RootJob, Files: make(map[string]string, len(files))}
		for path, data := range files {
			bundle.Files[path] = string(data)
		}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return AttestationSource{}, nil, err
		}
		data = append(data, '\n')
		sum := sha256.Sum256(data)
		return AttestationSource{Kind: "critic-root", RootJob: req.RootJob, Round: closure.Round,
			ClosureSHA256: hex.EncodeToString(sum[:])}, data, nil
	}
	if !safeReaderRecord(req.ReaderRecord) {
		return AttestationSource{}, nil, fmt.Errorf("reader record must be a repository-relative path under metasystem/records/misc")
	}
	top, err := r.TopLevel(req.Repo)
	absolute := filepath.Join(top, filepath.FromSlash(req.ReaderRecord))
	if err != nil {
		return AttestationSource{}, nil, err
	}
	digest, data, err := fileSHA256(absolute)
	if err != nil {
		return AttestationSource{}, nil, err
	}
	text := string(data)
	if !strings.Contains(text, subject.Commit) || !strings.Contains(text, subject.UnitDigest) {
		return AttestationSource{}, nil, fmt.Errorf("the reader's record must name commit %s and change %s", subject.Commit, subject.UnitDigest)
	}
	return AttestationSource{Kind: "reader-record", ReaderRecord: req.ReaderRecord, RecordSHA256: digest}, nil, nil
}

// UnitReadVerdictIsLand checks the first VERDICT: line of a report and the
// recorded verdict, ignoring surrounding whitespace on both lines.
func UnitReadVerdictIsLand(verdictLine, report string) bool {
	for _, line := range strings.Split(report, "\n") {
		if strings.HasPrefix(line, "VERDICT:") {
			return strings.TrimSpace(verdictLine) == "VERDICT: land" && strings.TrimSpace(line) == "VERDICT: land"
		}
	}
	return false
}

func validateUnitReadBundle(data []byte, goalID string, subject AttestationSubject) (UnitReadBundle, error) {
	var bundle UnitReadBundle
	var problem string
	if err := json.Unmarshal(data, &bundle); err != nil || bundle.SchemaVersion != 1 {
		problem = "is damaged or has an unknown schema"
	} else if bundle.Goal != goalID || bundle.Commit != subject.Commit {
		problem = "names another goal or commit"
	} else if strings.TrimSpace(bundle.ReadLaunch) == "" {
		problem = "has no read launch"
	} else if strings.ContainsAny(bundle.ReadLaunch, "/ \t\r\n") || strings.ContainsAny(bundle.ReadModel, " \t\r\n") {
		problem = "has a read launch or model that cannot be recorded in landing provenance"
	} else if strings.TrimSpace(bundle.ReadModel) == "" || strings.TrimSpace(bundle.BuildModel) == "" || bundle.ReadModel == bundle.BuildModel {
		problem = "needs different, non-empty read and build models"
	} else if bundle.ExaminedTree != subject.Tree {
		problem = "examined another tree"
	} else if bundle.ExaminedBase != subject.Parent {
		problem = "examined another base than the commit's parent"
	} else {
		if !UnitReadVerdictIsLand(bundle.VerdictLine, bundle.Report) {
			problem = "has no matching first VERDICT: land line in the report"
		} else {
			var record struct {
				State         string `json:"state"`
				Kind          string `json:"kind"`
				VerdictCounts bool   `json:"verdictCounts"`
			}
			if err := json.Unmarshal([]byte(bundle.LaunchRecord), &record); err != nil || record.State != "completed" || record.Kind != "read" || !record.VerdictCounts {
				problem = "has no completed read launch with verdictCounts true"
			}
		}
	}
	if problem != "" {
		return UnitReadBundle{}, operationRefusal(ReadInvalidCode, "the unit read of %s %s\nrun: metasystem work review %s", subject.Commit, problem, goalID)
	}
	return bundle, nil
}

func unitReadBundleAt(r attestationReads, repo, snapshot, goalID, commit string, source AttestationSource) ([]byte, UnitReadBundle, error) {
	digest, data, err := fileSHA256At(r, repo, snapshot, closureBundlePath(goalID, commit))
	if err != nil || digest != source.ClosureSHA256 {
		return nil, UnitReadBundle{}, operationRefusal(ReadInvalidCode, "the unit read's saved files for %s are missing or changed after the review\nrun: metasystem work review %s", commit, goalID)
	}
	var bundle UnitReadBundle
	if err := json.Unmarshal(data, &bundle); err != nil || bundle.ReadLaunch != source.ReadLaunch {
		return nil, UnitReadBundle{}, operationRefusal(ReadInvalidCode, "the unit read's saved files for %s are damaged or name another read launch\nrun: metasystem work review %s", commit, goalID)
	}
	return data, bundle, nil
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

func withoutReadFolds(folds []Fold) []Fold {
	var kept []Fold
	for _, fold := range folds {
		if fold.Kind != Read {
			kept = append(kept, fold)
		}
	}
	return kept
}

func foldsMatchWithReads(r attestationReads, repo, endpointTip string, recorded, current []Fold) bool {
	if sameFoldDigests(recorded, current) {
		return true
	}
	remaining := make([]Fold, 0, len(recorded))
	for _, fold := range recorded {
		landed, err := r.IsAncestor(repo, fold.Commit, endpointTip)
		if err != nil {
			return false
		}
		if !landed {
			remaining = append(remaining, fold)
		}
	}
	return sameFoldDigests(remaining, current)
}

func readAttestationAt(r attestationReads, repo, snapshot, goalID, commit string) (Attestation, error) {
	rel := attestationPath(goalID, commit)
	data, err := attestationFileAt(r, repo, snapshot, rel)
	if err != nil {
		return Attestation{}, err
	}
	var att Attestation
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&att); err != nil {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record %s is damaged (%v)\nrun: metasystem work review %s", rel, err, goalID)
	}
	canonical, err := json.MarshalIndent(att, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record %s was edited by hand\nrun: metasystem work review %s", rel, goalID)
	}
	return att, nil
}

func safeClosureBundlePath(path string) bool {
	windowsAbsolute := len(path) >= 3 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':' && path[2] == '/'
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || windowsAbsolute || strings.Contains(path, `\`) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func withClosureBundle(r attestationReads, repo, snapshot, goalID, commit string, source AttestationSource, use func(string) error) error {
	path := closureBundlePath(goalID, commit)
	data, err := attestationFileAt(r, repo, snapshot, path)
	if err != nil {
		return operationRefusal(ReadInvalidCode, "the reviewer's saved files for %s can't be read\nrun: metasystem work review %s", commit, goalID)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != source.ClosureSHA256 {
		return operationRefusal(ReadInvalidCode, "the reviewer's saved files for %s changed after the review\nrun: metasystem work review %s", commit, goalID)
	}
	var bundle closureBundle
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil || bundle.SchemaVersion != 1 || bundle.RootJob != source.RootJob {
		return operationRefusal(ReadInvalidCode, "the reviewer's saved files for %s are damaged\nrun: metasystem work review %s", commit, goalID)
	}
	canonical, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return operationRefusal(ReadInvalidCode, "the reviewer's saved files for %s were edited by hand\nrun: metasystem work review %s", commit, goalID)
	}
	temporary, done, err := diskstore.ScratchDir("goal-read-closure-*")
	if err != nil {
		return err
	}
	defer done()
	for path, content := range bundle.Files {
		if !safeClosureBundlePath(path) {
			return operationRefusal(ReadInvalidCode, "the reviewer's saved files for %s name a path outside them (%q)\nrun: metasystem work review %s", commit, path, goalID)
		}
		target := filepath.Join(temporary, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return use(temporary)
}

func validateCriticSource(r attestationReads, repo, snapshot, goalID, commit string, source AttestationSource, read readsubject.ReadSubject) (readsubject.Closure, error) {
	var closure readsubject.Closure
	var err error
	if source.ClosureSHA256 == "" {
		return dispatch.ValidateCommitCriticClosure(repo, source.RootJob, read)
	}
	err = withClosureBundle(r, repo, snapshot, goalID, commit, source, func(agentsRoot string) error {
		closure, err = dispatch.ValidateCommitCriticClosureAt(agentsRoot, source.RootJob, read)
		return err
	})
	return closure, err
}

func validateAttestation(r attestationReads, repo, snapshot, endpointTip, goalID, unit, commit string, seen map[string]bool) (Attestation, error) {
	if seen[commit] {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review of %s carries over from itself in a loop\nrun: metasystem work review %s", commit, goalID)
	}
	seen[commit] = true
	att, err := readAttestationAt(r, repo, snapshot, goalID, commit)
	if err != nil {
		return Attestation{}, err
	}
	if att.SchemaVersion != 1 || att.Goal != goalID || att.Unit != unit || att.Verdict != "LAND" || att.Subject.Commit != commit {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record at %s is not a landing verdict for %s/%s\nrun: metasystem work review %s", commit, goalID, unit, goalID)
	}
	digest, err := digestAttestation(att)
	if err != nil || digest != att.SHA256 {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record of %s changed after it was written\nrun: metasystem work review %s", commit, goalID)
	}
	subject, read, err := computeSubjectWithReads(r, repo, commit)
	if err != nil || subject != att.Subject {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record of %s describes other changes than the build holds\nrun: metasystem work review %s", commit, goalID)
	}
	folds, err := foldRangeWithReads(r, repo, endpointTip, commit, goalID)
	if err != nil || !foldsMatchWithReads(r, repo, endpointTip, att.Folds, folds) {
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record of %s no longer matches the goal branch below it\nrun: metasystem work review %s", commit, goalID)
	}
	if att.Gate.Kind != "go-gate-fast" || att.Gate.RunID == "" || att.Gate.Tree != subject.Tree {
		return Attestation{}, operationRefusal(ReadUngatedCode, "the review of %s has no passing quick check of tree %s\nrun: metasystem work review %s", commit, subject.Tree, goalID)
	}
	if err := validateTestChangesWithReads(r, repo, goalID, commit, att.TestsChanged); err != nil {
		return Attestation{}, err
	}
	if att.Carry != nil {
		if err := r.CommitExists(repo, att.Carry.FromCommit); err != nil {
			return Attestation{}, operationRefusal(ReadInvalidCode,
				"the build the review carries over from is not in this repository\nrun: metasystem work status %s", goalID)
		}
		prior, err := validateAttestation(r, repo, snapshot, endpointTip, goalID, unit, att.Carry.FromCommit, seen)
		if err != nil {
			return Attestation{}, err
		}
		priorChange, err := changeDigestWithReads(r, repo, att.Carry.FromCommit)
		if err != nil {
			return Attestation{}, err
		}
		change, err := changeDigestWithReads(r, repo, commit)
		if err != nil {
			return Attestation{}, err
		}
		if att.Carry.FromTree != prior.Subject.Tree || att.Carry.ToCommit != commit || att.Carry.ToTree != subject.Tree ||
			priorChange != change || !sameFoldDigests(withoutReadFolds(prior.Folds), withoutReadFolds(folds)) || prior.Source != att.Source {
			return Attestation{}, operationRefusal(ReadStaleCode, "the review carried over from %s no longer fits: the changes moved since\nrun: metasystem work review %s", att.Carry.FromCommit, goalID)
		}
		return att, nil
	}
	switch att.Source.Kind {
	case "unit-read":
		if att.Source.RootJob != "" || att.Source.Round != 0 || att.Source.ReaderRecord != "" || att.Source.RecordSHA256 != "" {
			return Attestation{}, operationRefusal(ReadInvalidCode, "the unit read of %s names fields from other review sources\nrun: metasystem work review %s", commit, goalID)
		}
		data, _, err := unitReadBundleAt(r, repo, snapshot, goalID, commit, att.Source)
		if err == nil {
			_, err = validateUnitReadBundle(data, goalID, subject)
		}
		if err != nil {
			return Attestation{}, err
		}
	case "critic-root":
		closure, err := validateCriticSource(r, repo, snapshot, goalID, commit, att.Source, read)
		if err != nil || closure.Round != att.Source.Round {
			return Attestation{}, operationRefusal(ReadInvalidCode, "the reviewer's job for %s did not finish cleanly (%s)\nrun: metasystem work review %s", commit, firstLine(err), goalID)
		}
	case "reader-record":
		if att.Source.ClosureSHA256 != "" || !safeReaderRecord(att.Source.ReaderRecord) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "the reader's record is not under records/misc\nrun: metasystem work review %s", goalID)
		}
		digest, data, err := fileSHA256At(r, repo, snapshot, att.Source.ReaderRecord)
		if err != nil || digest != att.Source.RecordSHA256 || !strings.Contains(string(data), commit) || !strings.Contains(string(data), subject.UnitDigest) {
			return Attestation{}, operationRefusal(ReadInvalidCode, "the reader's record for %s is missing or changed\nrun: metasystem work review %s", commit, goalID)
		}
	default:
		return Attestation{}, operationRefusal(ReadInvalidCode, "the review record of %s names an unknown reviewer kind %q\nrun: metasystem work review %s", commit, att.Source.Kind, goalID)
	}
	return att, nil
}

func ValidateAttestation(repo, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(gitAttestationReads{}, repo, "", endpointTip, goalID, unit, commit, map[string]bool{})
}

func ValidateAttestationWithReads(reads AttestationReads, repo, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(reads, repo, "", endpointTip, goalID, unit, commit, map[string]bool{})
}

func ComputeSubjectWithReads(reads AttestationReads, repo, commit string) (AttestationSubject, error) {
	subject, _, err := computeSubjectWithReads(reads, repo, commit)
	return subject, err
}

// ValidateAttestationAt validates the evidence as it exists in one fetched
// branch snapshot while keeping local critic artifacts available at repo.
func ValidateAttestationAt(repo, snapshot, endpointTip, goalID, unit, commit string) (Attestation, error) {
	return validateAttestation(gitAttestationReads{}, repo, snapshot, endpointTip, goalID, unit, commit, map[string]bool{})
}

func rawTransition(repo, before, after string) ([]byte, error) {
	return gitOutput(repo, "diff-tree", "-r", "-z", "--no-renames", "--full-index", before, after)
}

func filteredTransitionDigest(raw []byte, prefix string, excluded map[string]bool) (string, []string, bool, error) {
	parts := bytes.Split(raw, []byte{0})
	var kept bytes.Buffer
	var paths []string
	destructive := false
	for index := 0; index+1 < len(parts) && len(parts[index]) != 0; index += 2 {
		fields := strings.Fields(string(parts[index]))
		if len(fields) != 5 || !strings.HasPrefix(fields[0], ":") {
			return "", nil, false, fmt.Errorf("candidate transition is malformed")
		}
		path := string(parts[index+1])
		if excluded[path] {
			continue
		}
		paths = append(paths, path)
		kept.Write(parts[index])
		kept.WriteByte(0)
		kept.WriteString(prefix + path)
		kept.WriteByte(0)
		if fields[4] == "D" {
			destructive = true
		}
	}
	sum := sha256.Sum256(kept.Bytes())
	return hex.EncodeToString(sum[:]), paths, destructive, nil
}

func treeEntryAt(repo, tree, path string) (string, error) {
	out, err := gitOutput(repo, "ls-tree", "-z", tree, "--", path)
	if err != nil {
		return "", err
	}
	meta, _, _ := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	return meta, nil
}

func rootGoalRevisionAt(agentsRoot, job, goalID string) (uint64, error) {
	record, err := dispatch.ReadRecordObject(filepath.Join(agentsRoot, "jobs", job+".json"))
	if err != nil {
		return 0, err
	}
	lens := dispatch.JobRecordOf(record)
	revision, present := lens.GoalRevision()
	if !present || revision == 0 || lens.GoalID() != goalID {
		return 0, fmt.Errorf("critic root %s is not bound to goal %s at a revision", job, goalID)
	}
	return revision, nil
}

func attestedGoalRevision(r attestationReads, repo, snapshot, goalID, commit string, source AttestationSource) (uint64, error) {
	if source.ClosureSHA256 == "" {
		return rootGoalRevisionAt(filepath.Join(repo, "artifacts", "agents"), source.RootJob, goalID)
	}
	var revision uint64
	err := withClosureBundle(r, repo, snapshot, goalID, commit, source, func(agentsRoot string) error {
		var err error
		revision, err = rootGoalRevisionAt(agentsRoot, source.RootJob, goalID)
		return err
	})
	return revision, err
}

// BindLandedUnit validates the persisted evidence and binds one prospective
// trunk transition to the exact branch contribution it certifies.
func BindLandedUnit(repo, snapshot, endpointTip, goalID, commit, beforeTree, afterTree string) (LandedUnit, error) {
	return bindLandedUnit(gitAttestationReads{}, repo, snapshot, endpointTip, goalID, commit, beforeTree, afterTree)
}
func bindLandedUnit(r attestationReads, repo, snapshot, endpointTip, goalID, commit, beforeTree, afterTree string) (LandedUnit, error) {
	if err := r.CommitExists(repo, commit); err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "unreadable", Err: err}
	}
	info, err := r.Kind(repo, commit, goalID)
	if err != nil || info.Kind != Unit {
		if err == nil {
			err = fmt.Errorf("commit %s is not a Goal-Unit commit", commit)
		}
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	att, err := readAttestationAt(r, repo, snapshot, goalID, commit)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "unreadable", Err: err}
	}
	if att.Goal != goalID {
		return LandedUnit{}, &LandedUnitError{Code: "goal-mismatch", Err: fmt.Errorf("attestation names goal %s", att.Goal)}
	}
	att, err = validateAttestation(r, repo, snapshot, endpointTip, goalID, info.Unit, commit, map[string]bool{})
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	result := LandedUnit{Goal: att.Goal, Unit: att.Unit, Digest: att.Subject.UnitDigest,
		CriticRoot: att.Source.RootJob, Round: att.Source.Round, GateRunID: att.Gate.RunID}
	if att.Source.Kind == "reader-record" {
		return result, nil
	}
	if att.Source.Kind == "unit-read" {
		var bundle UnitReadBundle
		_, bundle, err = unitReadBundleAt(r, repo, snapshot, goalID, commit, att.Source)
		result.ReadLaunch, result.ReadModel, result.GoalRevision = bundle.ReadLaunch, bundle.ReadModel, bundle.GoalRevision
		if err == nil && (bundle.Goal != goalID || bundle.GoalRevision == 0) {
			err = fmt.Errorf("read launch %s is not bound to goal %s at a revision", bundle.ReadLaunch, goalID)
		}
	} else {
		result.GoalRevision, err = attestedGoalRevision(r, repo, snapshot, goalID, commit, att.Source)
	}
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: err}
	}
	prefix, err := r.Prefix(repo)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	excluded := map[string]bool{
		"memory/receipts.log": true,
	}
	for _, readPrefix := range []string{"records/reads/" + goalID + "/"} {
		raw, rawErr := r.Transition(repo, beforeTree, afterTree)
		if rawErr != nil {
			return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: rawErr}
		}
		parts := bytes.Split(raw, []byte{0})
		for index := 1; index < len(parts); index += 2 {
			if strings.HasPrefix(string(parts[index]), readPrefix) {
				excluded[string(parts[index])] = true
			}
		}
	}
	lastFold := map[string]string{}
	for _, fold := range att.Folds {
		result.HasPlan = result.HasPlan || fold.Kind == Plan
		rawEntries, entriesErr := r.RawEntries(repo, fold.Commit)
		if entriesErr != nil {
			return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: entriesErr}
		}
		entries, entriesErr := parseRawEntries(fold.Commit, rawEntries)
		if entriesErr != nil {
			return LandedUnit{}, &LandedUnitError{Code: "invalid", Err: entriesErr}
		}
		for _, entry := range entries {
			path := strings.TrimPrefix(entry.Path, prefix)
			if prefix != "" && path == entry.Path {
				return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: fmt.Errorf("folded path %s is outside the landing workspace", entry.Path)}
			}
			excluded[path] = true
			lastFold[path] = fold.Commit
		}
	}
	for path, foldCommit := range lastFold {
		_, beforeErr := r.TreeEntry(repo, beforeTree, path)
		after, afterErr := r.TreeEntry(repo, afterTree, path)
		want, wantErr := r.TreeEntry(repo, foldCommit, prefix+path)
		if beforeErr != nil || afterErr != nil || wantErr != nil || after != want {
			return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: fmt.Errorf("folded path %s does not match %s", path, foldCommit)}
		}
		result.FoldPaths = append(result.FoldPaths, path)
	}
	raw, err := r.Transition(repo, beforeTree, afterTree)
	if err != nil {
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	digest, paths, destructive, err := filteredTransitionDigest(raw, prefix, excluded)
	if err != nil || digest != att.Subject.UnitDigest {
		if err == nil {
			err = fmt.Errorf("the change to land (%s) isn't the one that was read (%s)", digest, att.Subject.UnitDigest)
		}
		return LandedUnit{}, &LandedUnitError{Code: "change-mismatch", Err: err}
	}
	result.ChangedPaths, result.Destructive = paths, destructive
	sort.Strings(result.FoldPaths)
	return result, nil
}

func unitCommitInRangeWithReads(r attestationReads, repo, endpointTip, tip, goalID string, units []string) (string, error) {
	commits, err := r.Range(repo, endpointTip, tip, goalID)
	if err != nil {
		return "", err
	}
	for i := len(commits) - 1; i >= 0; i-- {
		if commits[i].Kind == Unit && sameUnits(commits[i].Units, units) {
			return commits[i].ID, nil
		}
	}
	return "", operationRefusal(ReadInvalidCode, "goal %s's branch has no build %s\nrun: metasystem work status %s", goalID, unitList(units), goalID)
}

func prospectiveReadPatch(repo string, generated map[string][]byte, readerRecord string) ([]byte, error) {
	scratch, done, err := diskstore.ScratchDir("goal-read-index-*")
	if err != nil {
		return nil, err
	}
	defer done()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(scratch, "index")}
	tree, err := gitOutput(repo, "write-tree")
	if err != nil {
		return nil, err
	}
	if _, err := gitInputEnv(repo, env, nil, "read-tree", strings.TrimSpace(string(tree))); err != nil {
		return nil, err
	}
	for path, data := range generated {
		blob, err := gitInput(repo, data, "hash-object", "-w", "--path="+path, "--stdin")
		if err != nil {
			return nil, err
		}
		entry := "100644," + strings.TrimSpace(string(blob)) + "," + path
		if _, err := gitInputEnv(repo, env, nil, "update-index", "--add", "--cacheinfo", entry); err != nil {
			return nil, err
		}
	}
	if readerRecord != "" {
		if _, err := gitInputEnv(repo, env, nil, "add", "--", ":(top)"+readerRecord); err != nil {
			return nil, err
		}
	}
	return gitInputEnv(repo, env, nil, "diff", "--cached", "--binary", "--full-index")
}

func CommitRead(req CommitReadRequest) (string, Attestation, error) {
	if req.Inputs != nil && (req.Transport == nil || req.GateRepository == nil) {
		return "", Attestation{}, fmt.Errorf("read commit raw transport and gate repository are required")
	}
	reads, effects, err := req.Inputs.readers()
	if err != nil {
		return "", Attestation{}, err
	}
	return commitRead(req, reads, effects)
}
func commitRead(req CommitReadRequest, r attestationReads, e readCommitEffects) (string, Attestation, error) {
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
	state, err := e.Inspect(commitReq)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateRunID == "" || req.GateTree == "" {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "the review can't be recorded without its passing quick check\nrun: metasystem work review %s", req.GoalID)
	}
	unitCommit, err := unitCommitInRangeWithReads(r, req.Repo, req.EndpointTip, state.baseTip, req.GoalID, units)
	if err != nil {
		return "", Attestation{}, err
	}
	subject, read, err := computeSubjectWithReads(r, req.Repo, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	if req.GateTree != subject.Tree {
		return "", Attestation{}, operationRefusal(ReadUngatedCode, "the quick check ran on tree %s, not on the build's tree %s\nrun: metasystem work review %s", req.GateTree, subject.Tree, req.GoalID)
	}
	if err := validateTestChangesWithReads(r, req.Repo, req.GoalID, unitCommit, req.TestsChanged); err != nil {
		return "", Attestation{}, err
	}
	folds, err := foldRangeWithReads(r, req.Repo, req.EndpointTip, unitCommit, req.GoalID)
	if err != nil {
		return "", Attestation{}, err
	}
	att := Attestation{
		SchemaVersion: 1, Goal: req.GoalID, Unit: list, Subject: subject,
		Verdict: "LAND", Gate: GateObservation{Kind: "go-gate-fast", Tree: req.GateTree, RunID: req.GateRunID},
		Folds: folds, TestsChanged: append([]TestChange(nil), req.TestsChanged...),
	}
	var bundleData []byte
	sort.Slice(att.TestsChanged, func(i, j int) bool { return att.TestsChanged[i].Path < att.TestsChanged[j].Path })
	if req.Carry != "" {
		prior, err := validateAttestation(r, req.Repo, "", req.EndpointTip, req.GoalID, list, req.Carry, map[string]bool{})
		if err != nil {
			return "", Attestation{}, err
		}
		priorChange, err := changeDigestWithReads(r, req.Repo, req.Carry)
		if err != nil {
			return "", Attestation{}, err
		}
		change, err := changeDigestWithReads(r, req.Repo, unitCommit)
		if err != nil {
			return "", Attestation{}, err
		}
		if priorChange != change || !sameFoldDigests(withoutReadFolds(prior.Folds), withoutReadFolds(folds)) {
			return "", Attestation{}, operationRefusal(ReadStaleCode, "the review carried over from %s no longer fits: the changes moved since\nrun: metasystem work review %s", req.Carry, req.GoalID)
		}
		att.Source = prior.Source
		if prior.Source.ClosureSHA256 != "" {
			bundleData, err = attestationFileAt(r, req.Repo, "", closureBundlePath(req.GoalID, req.Carry))
			if err != nil {
				return "", Attestation{}, err
			}
		}
		att.Carry = &Carry{FromCommit: req.Carry, FromTree: prior.Subject.Tree, ToCommit: unitCommit, ToTree: subject.Tree}
	} else {
		att.Source, bundleData, err = directSource(r, req, subject, read)
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
	bundleRel := ""
	generated := map[string][]byte{rel: data}
	if len(bundleData) != 0 {
		bundleRel = closureBundlePath(req.GoalID, unitCommit)
		generated[bundleRel] = bundleData
	}
	paths, err := e.StagedPaths(req.Repo)
	if err != nil {
		return "", Attestation{}, err
	}
	paths = append(paths, rel)
	if bundleRel != "" {
		paths = append(paths, bundleRel)
	}
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
	if state.adopt {
		if err := e.AdoptionClean(commitReq, state, []string{rel, bundleRel, readerRecord}); err != nil {
			return "", Attestation{}, err
		}
	}
	patch, err := e.Patch(req.Repo, generated, readerRecord)
	if err != nil {
		return "", Attestation{}, err
	}
	subjectLine, trailer, err := commitMessage(commitReq, unitCommit)
	if err != nil {
		return "", Attestation{}, err
	}
	preparedTip, err := e.Build(commitReq, state, subjectLine, trailer, patch)
	if err != nil {
		return "", Attestation{}, err
	}
	indexBefore, err := e.IndexTree(req.Repo)
	if err != nil {
		return "", Attestation{}, err
	}
	projectRoot, err := r.TopLevel(req.Repo)
	if err != nil {
		return "", Attestation{}, err
	}
	rollbackMaterialized := func(cause error) error {
		var cleanup []string
		if err := e.RestoreIndex(req.Repo, indexBefore); err != nil {
			cleanup = append(cleanup, err.Error())
		}
		for path := range generated {
			if err := os.Remove(filepath.Join(projectRoot, filepath.FromSlash(path))); err != nil && !os.IsNotExist(err) {
				cleanup = append(cleanup, err.Error())
			}
		}
		if len(cleanup) != 0 {
			return fmt.Errorf("%v; read materialization rollback failed: %s", cause, strings.Join(cleanup, "; "))
		}
		return cause
	}
	for path, contents := range generated {
		abs := filepath.Join(projectRoot, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", Attestation{}, rollbackMaterialized(err)
		}
		if err := os.WriteFile(abs, contents, 0o644); err != nil {
			return "", Attestation{}, rollbackMaterialized(err)
		}
	}
	paths = []string{rel}
	if bundleRel != "" {
		paths = append(paths, bundleRel)
	}
	if att.Source.Kind == "reader-record" && att.Carry == nil {
		paths = append(paths, att.Source.ReaderRecord)
	}
	if err := e.Stage(req.Repo, paths); err != nil {
		return "", Attestation{}, rollbackMaterialized(err)
	}
	if err := e.Install(commitReq, state, preparedTip); err != nil {
		return "", Attestation{}, rollbackMaterialized(err)
	}
	return preparedTip, att, nil
}
