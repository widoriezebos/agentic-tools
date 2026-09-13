package readsubject

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type ReadSubjectKind string

const (
	SubjectLive   ReadSubjectKind = "live"
	SubjectDesign ReadSubjectKind = "design"
	SubjectCommit ReadSubjectKind = "commit"
)

type ReadSubject struct {
	Kind                  ReadSubjectKind `json:"kind"`
	ImplementerRoot       string          `json:"implementerRoot,omitempty"`
	ReviewedMember        string          `json:"reviewedMember,omitempty"`
	ReviewedProjectTree   string          `json:"reviewedProjectTree,omitempty"`
	DiffDigest            string          `json:"diffDigest,omitempty"`
	DesignPath            string          `json:"designPath,omitempty"`
	ContentDigest         string          `json:"contentDigest,omitempty"`
	DeclaredOutputsDigest string          `json:"declaredOutputsDigest,omitempty"`
	ReviewedCommit        string          `json:"reviewedCommit,omitempty"`
	Commit                string          `json:"commit,omitempty"`
	Parent                string          `json:"parent,omitempty"`
	Tree                  string          `json:"tree,omitempty"`
}

func (s ReadSubject) Equal(other ReadSubject) bool {
	if s.Kind != other.Kind {
		return false
	}
	switch s.Kind {
	case SubjectLive:
		return s.ImplementerRoot == other.ImplementerRoot &&
			s.ReviewedProjectTree == other.ReviewedProjectTree &&
			s.DiffDigest == other.DiffDigest
	case SubjectDesign:
		return s.DesignPath == other.DesignPath &&
			s.ContentDigest == other.ContentDigest &&
			s.DeclaredOutputsDigest == other.DeclaredOutputsDigest
	case SubjectCommit:
		return s.Commit == other.Commit &&
			s.Tree == other.Tree &&
			s.DiffDigest == other.DiffDigest
	default:
		return false
	}
}

func (s ReadSubject) Digest() string {
	var identity []any
	switch s.Kind {
	case SubjectLive:
		identity = []any{s.Kind, s.ImplementerRoot, s.ReviewedProjectTree, s.DiffDigest}
	case SubjectDesign:
		identity = []any{s.Kind, s.DesignPath, s.ContentDigest, s.DeclaredOutputsDigest}
	case SubjectCommit:
		identity = []any{s.Kind, s.Commit, s.Tree, s.DiffDigest}
	default:
		identity = []any{s.Kind}
	}
	encoded, _ := json.Marshal(identity)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func DecodeReadSubject(value any) (ReadSubject, bool, error) {
	if value == nil {
		return ReadSubject{}, false, nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return ReadSubject{}, true, fmt.Errorf("read subject must be an object")
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return ReadSubject{}, true, fmt.Errorf("cannot encode read subject: %w", err)
	}
	var subject ReadSubject
	if err := json.Unmarshal(encoded, &subject); err != nil {
		return ReadSubject{}, true, fmt.Errorf("cannot decode read subject: %w", err)
	}
	switch subject.Kind {
	case SubjectLive, SubjectDesign, SubjectCommit:
		return subject, true, nil
	default:
		return ReadSubject{}, true, fmt.Errorf("read subject has unknown kind %q", subject.Kind)
	}
}

var validJobID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func ReadRoundSubject(agents, rootJob string, round int64) (ReadSubject, bool, error) {
	if !validJobID.MatchString(rootJob) {
		return ReadSubject{}, false, fmt.Errorf("read subject root %q is not a valid job identifier", rootJob)
	}
	if round < 1 {
		return ReadSubject{}, false, fmt.Errorf("read subject round must be a positive integer")
	}
	path := filepath.Join(agents, rootJob, "rounds", strconv.FormatInt(round, 10), "subject.json")
	object, err := readObject(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadSubject{}, false, nil
		}
		return ReadSubject{}, false, err
	}
	return DecodeReadSubject(object)
}

func readObject(path string) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("document must be a JSON object")
	}
	return object, nil
}
