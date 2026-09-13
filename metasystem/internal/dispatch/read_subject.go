package dispatch

import "github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"

type ReadSubjectKind = readsubject.ReadSubjectKind

const (
	SubjectLive   = readsubject.SubjectLive
	SubjectDesign = readsubject.SubjectDesign
	SubjectCommit = readsubject.SubjectCommit
)

type ReadSubject = readsubject.ReadSubject
