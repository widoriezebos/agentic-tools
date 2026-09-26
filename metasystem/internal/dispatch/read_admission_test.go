package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func admissionLiveSubject(implementer, member, treeByte, diffByte string) ReadSubject {
	return ReadSubject{
		Kind:                SubjectLive,
		ImplementerRoot:     implementer,
		ReviewedMember:      member,
		ReviewedProjectTree: strings.Repeat(treeByte, 40),
		DiffDigest:          strings.Repeat(diffByte, 64),
	}
}

func admissionDesignSubject(path, contentByte, outputsByte, commitByte string) ReadSubject {
	return ReadSubject{
		Kind: SubjectDesign, DesignPath: path,
		ContentDigest: strings.Repeat(contentByte, 64), DeclaredOutputsDigest: strings.Repeat(outputsByte, 64),
		ReviewedCommit: strings.Repeat(commitByte, 40),
	}
}

func writeSubjectReturn(t *testing.T, repo, root, job string, round int64, subject ReadSubject) {
	t.Helper()
	roundDir := filepath.Join(repo, "artifacts", "agents", root, "rounds", strconv.FormatInt(round, 10))
	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteReadSubject(filepath.Join(roundDir, "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"schemaVersion": 3, "jobId": job, "round": round, "findings": []any{}, "rigor": []any{}}
	switch subject.Kind {
	case SubjectLive:
		result["reviewedTree"] = subject.ReviewedProjectTree
	case SubjectCommit:
		result["reviewedTree"] = subject.Tree
	case SubjectDesign:
		result["reviewedCommit"] = subject.ReviewedCommit
	}
	writeJSONFile(t, roundDir, "return.json", result)
}

func cleanReadValue(round int64, subject ReadSubject) map[string]any {
	return map[string]any{"round": round, "subject": encodeReadSubject(subject)}
}

func seedImplementerScope(t *testing.T, repo string, subject ReadSubject) {
	t.Helper()
	if subject.Kind != SubjectLive {
		return
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), subject.ImplementerRoot+".json", map[string]any{
		"jobId": subject.ImplementerRoot, "role": "implementer", "round": 1, "parentJob": nil, "status": "completed",
	})
	if subject.ReviewedMember != "" && subject.ReviewedMember != subject.ImplementerRoot {
		writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), subject.ReviewedMember+".json", map[string]any{
			"jobId": subject.ReviewedMember, "role": "implementer", "round": 2,
			"parentJob": subject.ImplementerRoot, "status": "completed",
		})
	}
}

func seedProvenCleanRead(t *testing.T, repo, root, role string, round int64, subject ReadSubject, history bool) {
	t.Helper()
	seedImplementerScope(t, repo, subject)
	member := root
	rootRecord := map[string]any{
		"jobId": root, "role": role, "round": 1, "parentJob": nil, "status": "completed",
		findingRegisterField: []any{}, findingRegisterRoundField: round,
		findingRegisterSubjectDigestField: subject.Digest(),
	}
	if subject.Kind == SubjectLive {
		rootRecord["reviews"] = subject.ReviewedMember
	} else if subject.Kind == SubjectCommit {
		rootRecord["reviews"] = "commit:" + subject.Commit
	} else {
		rootRecord["design"] = subject.DesignPath
		rootRecord["declaredOutputsDigest"] = subject.DeclaredOutputsDigest
	}
	if history {
		rootRecord[cleanReadRoundsField] = []any{cleanReadValue(round, subject)}
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), root+".json", rootRecord)
	if round > 1 {
		member = root + "-r" + strconv.FormatInt(round, 10)
		writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), member+".json", map[string]any{
			"jobId": member, "role": role, "round": round, "parentJob": root, "status": "completed",
		})
	}
	writeSubjectReturn(t, repo, root, member, round, subject)
}

func bindFoldRound(t *testing.T, repo, root, job string, round int, subject ReadSubject, findings, rigor []any) {
	t.Helper()
	writeCriticRound(t, repo, root, job, round, findings, rigor)
	if round == 1 {
		setCriticSubjectFiles(t, repo, root, subject.ReviewedMember, subject.ReviewedProjectTree)
	} else {
		resultPath := filepath.Join(repo, "artifacts", "agents", root, "rounds", strconv.Itoa(round), "return.json")
		result := readJSONFile(t, resultPath)
		result["reviewedTree"] = subject.ReviewedProjectTree
		if err := writeRecord(resultPath, result); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteReadSubject(filepath.Join(repo, "artifacts", "agents", root, "rounds", strconv.Itoa(round), "subject.json"), subject); err != nil {
		t.Fatal(err)
	}
}

func readCleanHistory(t *testing.T, repo, root string) []any {
	t.Helper()
	record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", root+".json"))
	history, _ := record[cleanReadRoundsField].([]any)
	return history
}

func TestCleanReadHistorySurvivesChangedFold(t *testing.T) {
	for _, prehistory := range []bool{false, true} {
		t.Run(map[bool]string{false: "native-history", true: "pre-history-backfill"}[prehistory], func(t *testing.T) {
			repo := t.TempDir()
			subjectA := admissionLiveSubject("implementer", "implementer", "a", "1")
			subjectB := admissionLiveSubject("implementer", "implementer", "b", "2")
			bindFoldRound(t, repo, "critic", "critic", 1, subjectA, []any{}, []any{})
			if outcome, err := advanceWithPrefix(t, repo, "critic", "critic"); err != nil || outcome != "advanced" {
				t.Fatalf("clean fold = %q, %v", outcome, err)
			}
			if prehistory {
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, path)
				delete(root, cleanReadRoundsField)
				if err := writeRecord(path, root); err != nil {
					t.Fatal(err)
				}
			}
			bindFoldRound(t, repo, "critic", "critic-r2", 2, subjectB,
				[]any{registerFindingValue("F-1", true, "changed subject defect")},
				[]any{registerRigor("F-1", "severe")})
			if outcome, err := advanceWithPrefix(t, repo, "critic", "critic-r2"); err != nil || outcome != "advanced" {
				t.Fatalf("changed fold = %q, %v", outcome, err)
			}
			history := readCleanHistory(t, repo, "critic")
			if len(history) != 1 {
				t.Fatalf("clean history = %v; want round 1 only", history)
			}
			result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subjectA)
			assertReadRefusal(t, result, err, redundantReadRefusal, "critic", 1)
			if len(readRegister(t, repo, "critic")) != 1 {
				t.Fatal("admission changed the later finding register")
			}
			root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
			if _, present := root[closureField]; present {
				t.Fatal("admission synthesized a closure")
			}
			before := string(canonicalJSON(history))
			if outcome, err := advanceWithFacts(t, repo, "critic", "critic-r2"); err != nil || outcome != "unchanged" {
				t.Fatalf("fold retry = %q, %v", outcome, err)
			}
			if after := string(canonicalJSON(readCleanHistory(t, repo, "critic"))); after != before {
				t.Fatalf("fold retry changed clean history: before=%s after=%s", before, after)
			}
		})
	}
}

func TestCleanReadHistoryRejectsNonCleanRounds(t *testing.T) {
	t.Run("failed-cancelled-and-unbound", func(t *testing.T) {
		for _, tc := range []struct {
			name, status string
			bound        bool
		}{
			{name: "failed", status: "failed", bound: true},
			{name: "cancelled", status: "cancelled", bound: true},
			{name: "unbound", status: "completed", bound: false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				repo := t.TempDir()
				subject := admissionLiveSubject("implementer", "implementer", "a", "1")
				bindFoldRound(t, repo, "critic", "critic", 1, subject, []any{}, []any{})
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				record := readJSONFile(t, path)
				record["status"] = tc.status
				if err := writeRecord(path, record); err != nil {
					t.Fatal(err)
				}
				if !tc.bound {
					resultPath := filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "return.json")
					result := readJSONFile(t, resultPath)
					result["reviewedTree"] = strings.Repeat("b", 40)
					if err := writeRecord(resultPath, result); err != nil {
						t.Fatal(err)
					}
				}
				calls := []critiqueFactCall{}
				if tc.status == "completed" {
					calls = append(calls, declaredPrefix(repo, ""))
				}
				if _, err := advanceWithFacts(t, repo, "critic", "critic", calls...); err != nil {
					t.Fatal(err)
				}
				if history := readCleanHistory(t, repo, "critic"); len(history) != 0 {
					t.Fatalf("%s fold recorded clean history: %v", tc.name, history)
				}
			})
		}
	})

	t.Run("non-clean-dispositions", func(t *testing.T) {
		for _, tc := range []struct{ status, resolution string }{
			{"accepted-risk", "accepted-risk"}, {"deferred", "deferred"}, {"resolved", "out-of-scope"},
		} {
			t.Run(tc.status+"-"+tc.resolution, func(t *testing.T) {
				repo := t.TempDir()
				subject := admissionLiveSubject("implementer", "implementer", "a", "1")
				seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, false)
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, path)
				root[findingRegisterField] = encodeFindingRegister([]registerFinding{{
					FindingID: "F-1", Critic: "critic", RigorClass: "bounded", FactsDigest: digestJSON(nil),
					Status: tc.status, Resolution: tc.resolution, DecisionOpID: "decision-op",
					EvidenceDigest: digestJSON(nil), Multiplicity: 1,
				}})
				if err := writeRecord(path, root); err != nil {
					t.Fatal(err)
				}
				reads, err := cleanReadsForRoot(loadCritiqueState(repo), "critic", root)
				if err != nil || len(reads) != 0 {
					t.Fatalf("%s/%s clean reads = %v, %v", tc.status, tc.resolution, reads, err)
				}
			})
		}
	})

	t.Run("withdrawn-is-clean", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, false)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, path)
		root[findingRegisterField] = encodeFindingRegister([]registerFinding{{
			FindingID: "F-1", Critic: "critic", RigorClass: "bounded", FactsDigest: digestJSON(nil),
			Status: "resolved", Resolution: "withdrawn", EvidenceDigest: digestJSON(nil), Multiplicity: 1,
		}})
		if err := writeRecord(path, root); err != nil {
			t.Fatal(err)
		}
		reads, err := cleanReadsForRoot(loadCritiqueState(repo), "critic", root)
		if err != nil || len(reads) != 1 || reads[0].Round != 1 || !reads[0].Subject.Equal(subject) {
			t.Fatalf("withdrawn clean reads = %v, %v", reads, err)
		}
	})

	t.Run("generic-cas-cannot-inject", func(t *testing.T) {
		repo := sandbox(t)
		createPending(t, repo, "job-a")
		setupPending(t, repo, "job-a")
		patch := writeJSON(t, filepath.Join(t.TempDir(), "patch.json"), map[string]any{
			cleanReadRoundsField: []any{cleanReadValue(1, admissionLiveSubject("impl", "impl", "a", "1"))},
		})
		if _, err := RecordCAS(repo, "job-a", "pending", "pending", patch); err == nil {
			t.Fatal("generic record CAS injected clean read history")
		}
	})

	t.Run("conflicting-round", func(t *testing.T) {
		repo := t.TempDir()
		one := admissionLiveSubject("implementer", "implementer", "a", "1")
		two := admissionLiveSubject("implementer", "implementer", "b", "2")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, one, true)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, path)
		root[cleanReadRoundsField] = []any{cleanReadValue(1, one), cleanReadValue(1, two)}
		if err := writeRecord(path, root); err != nil {
			t.Fatal(err)
		}
		if _, err := cleanReadsForRoot(loadCritiqueState(repo), "critic", root); err == nil || !strings.Contains(err.Error(), "round 1") {
			t.Fatalf("conflicting history = %v", err)
		}
	})
}

func TestReadAdmissionTreatsUnavailableCleanEvidenceAsNoProof(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string, ReadSubject)
	}{
		{
			name: "collected-chain-payload",
			mutate: func(t *testing.T, repo string, _ ReadSubject) {
				t.Helper()
				rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, rootPath)
				root["chainClosed"] = true
				if err := writeRecord(rootPath, root); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(filepath.Join(repo, "artifacts", "agents", "critic")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing-return",
			mutate: func(t *testing.T, repo string, _ ReadSubject) {
				t.Helper()
				if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "critic", "rounds", "1", "return.json")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "stale-pre-history-digest",
			mutate: func(t *testing.T, repo string, original ReadSubject) {
				t.Helper()
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, path)
				delete(root, cleanReadRoundsField)
				root[findingRegisterRoundField] = int64(2)
				root[findingRegisterSubjectDigestField] = original.Digest()
				if err := writeRecord(path, root); err != nil {
					t.Fatal(err)
				}
				writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic-r2.json", map[string]any{
					"jobId": "critic-r2", "role": "code-critic", "round": 2, "parentJob": "critic", "status": "cancelled",
				})
				changed := admissionLiveSubject("implementer", "implementer", "b", "2")
				writeSubjectReturn(t, repo, "critic", "critic-r2", 2, changed)
			},
		},
		{
			name: "folded-member-not-completed",
			mutate: func(t *testing.T, repo string, _ ReadSubject) {
				t.Helper()
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
				root := readJSONFile(t, path)
				root["status"] = "cancelled"
				if err := writeRecord(path, root); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			prior := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedProvenCleanRead(t, repo, "critic", "code-critic", 1, prior, true)
			tc.mutate(t, repo, prior)

			root := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json"))
			reads, err := cleanReadsForRoot(loadCritiqueState(repo), "critic", root)
			if err != nil || len(reads) != 0 {
				t.Fatalf("unavailable evidence yielded clean reads %v, %v", reads, err)
			}
			fresh := admissionLiveSubject("implementer", "implementer", "c", "3")
			got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, fresh)
			if err != nil || got.Decision != readAdmittedDecision {
				t.Fatalf("fresh admission = %+v, %v", got, err)
			}
		})
	}
}

func TestReadAdmissionTreatsMissingFoldMemberAsNoProof(t *testing.T) {
	t.Run("collected-member", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 2, subject, true)
		rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, rootPath)
		root["chainClosed"] = true
		if err := writeRecord(rootPath, root); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "jobs", "critic-r2.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(filepath.Join(repo, "artifacts", "agents", "critic")); err != nil {
			t.Fatal(err)
		}

		got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("admission with collected member evidence = %+v, %v", got, err)
		}
	})

	t.Run("contradictory-member", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 2, subject, true)
		memberPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic-r2.json")
		member := readJSONFile(t, memberPath)
		member["role"] = "warden"
		if err := writeRecord(memberPath, member); err != nil {
			t.Fatal(err)
		}

		got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		if err == nil || !strings.Contains(err.Error(), "has role") || got.Decision == readAdmittedDecision {
			t.Fatalf("admission with contradictory member evidence = %+v, %v", got, err)
		}
	})
}

func TestCritiqueRegisterAdvanceContinuesWithoutHistoricalProof(t *testing.T) {
	repo := t.TempDir()
	stale := admissionLiveSubject("implementer", "implementer", "a", "1")
	bindFoldRound(t, repo, "critic", "critic", 1, stale, []any{}, []any{})
	if outcome, err := advanceWithPrefix(t, repo, "critic", "critic"); err != nil || outcome != "advanced" {
		t.Fatalf("round one fold = %q, %v", outcome, err)
	}
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	root := readJSONFile(t, rootPath)
	delete(root, cleanReadRoundsField)
	root[findingRegisterRoundField] = int64(2)
	root[findingRegisterSubjectDigestField] = stale.Digest()
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic-r2.json", map[string]any{
		"jobId": "critic-r2", "role": "code-critic", "round": 2, "parentJob": "critic", "status": "cancelled",
	})
	writeSubjectReturn(t, repo, "critic", "critic-r2", 2, admissionLiveSubject("implementer", "implementer", "b", "2"))

	current := admissionLiveSubject("implementer", "implementer", "c", "3")
	bindFoldRound(t, repo, "critic", "critic-r3", 3, current, []any{}, []any{})
	if outcome, err := advanceWithPrefix(t, repo, "critic", "critic-r3"); err != nil || outcome != "advanced" {
		t.Fatalf("round three fold = %q, %v", outcome, err)
	}
	history := readCleanHistory(t, repo, "critic")
	if len(history) != 1 {
		t.Fatalf("round three clean history = %v", history)
	}
	entry := history[0].(map[string]any)
	if round, ok := numInt(entry["round"]); !ok || round != 3 {
		t.Fatalf("clean history did not retain only the provable round: %v", history)
	}
}

func TestCritiqueRegisterAdvanceContinuesWithMissingFoldMember(t *testing.T) {
	repo := t.TempDir()
	first := admissionLiveSubject("implementer", "implementer", "a", "1")
	bindFoldRound(t, repo, "critic", "critic", 1, first, []any{}, []any{})
	if outcome, err := advanceWithPrefix(t, repo, "critic", "critic"); err != nil || outcome != "advanced" {
		t.Fatalf("round one fold = %q, %v", outcome, err)
	}
	second := admissionLiveSubject("implementer", "implementer", "b", "2")
	bindFoldRound(t, repo, "critic", "critic-r2", 2, second, []any{}, []any{})
	if outcome, err := advanceWithPrefix(t, repo, "critic", "critic-r2"); err != nil || outcome != "advanced" {
		t.Fatalf("round two fold = %q, %v", outcome, err)
	}
	if err := os.Remove(filepath.Join(repo, "artifacts", "agents", "jobs", "critic-r2.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(repo, "artifacts", "agents", "critic")); err != nil {
		t.Fatal(err)
	}

	current := admissionLiveSubject("implementer", "implementer", "c", "3")
	bindFoldRound(t, repo, "critic", "critic-r3", 3, current, []any{}, []any{})
	if outcome, err := advanceWithPrefix(t, repo, "critic", "critic-r3"); err != nil || outcome != "advanced" {
		t.Fatalf("round three fold = %q, %v", outcome, err)
	}
	history := readCleanHistory(t, repo, "critic")
	if len(history) != 1 {
		t.Fatalf("round three clean history = %v", history)
	}
	entry := history[0].(map[string]any)
	if round, ok := numInt(entry["round"]); !ok || round != 3 {
		t.Fatalf("clean history did not retain only the provable round: %v", history)
	}
}

func TestReadAdmissionIgnoresMalformedUnrelatedSibling(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		round      any
	}{
		{name: "malformed-round", role: "code-critic", round: "two"},
		{name: "different-member-role", role: "warden", round: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			prior := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedUnfoldedRead(t, repo, "sibling", "code-critic", "completed", prior)
			writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "sibling-r2.json", map[string]any{
				"jobId": "sibling-r2", "role": tc.role, "round": tc.round, "parentJob": "sibling", "status": "running",
			})

			fresh := admissionLiveSubject("implementer", "implementer", "b", "2")
			got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, fresh)
			if err != nil || got.Decision != readAdmittedDecision {
				t.Fatalf("fresh admission = %+v, %v", got, err)
			}
		})
	}
}

func TestReadAdmissionSkipsUnrelatedSiblingCleanStateErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		want   string
		mutate func(map[string]any)
	}{
		{
			name: "round-not-one",
			want: "is not round one",
			mutate: func(root map[string]any) {
				root["round"] = int64(2)
			},
		},
		{
			name: "round-missing",
			want: "is not round one",
			mutate: func(root map[string]any) {
				delete(root, "round")
			},
		},
		{
			name: "register-round-string",
			want: "malformed register round state",
			mutate: func(root map[string]any) {
				root[findingRegisterRoundField] = "one"
			},
		},
		{
			name: "register-malformed",
			want: "malformed finding register",
			mutate: func(root map[string]any) {
				root[findingRegisterField] = "malformed"
			},
		},
		{
			name: "history-without-register",
			want: "history without a finding register",
			mutate: func(root map[string]any) {
				delete(root, findingRegisterField)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			prior := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedProvenCleanRead(t, repo, "sibling", "code-critic", 1, prior, true)
			rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "sibling.json")
			root := readJSONFile(t, rootPath)
			tc.mutate(root)
			if err := writeRecord(rootPath, root); err != nil {
				t.Fatal(err)
			}

			fresh := admissionLiveSubject("implementer", "implementer", "z", "9")
			got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, fresh)
			if err != nil || got.Decision != readAdmittedDecision {
				t.Fatalf("fresh admission = %+v, %v", got, err)
			}
			got, err = CritiqueReadAdmission(repo, "code-critic", "sibling", 2, fresh)
			if err == nil || !strings.Contains(err.Error(), tc.want) || got.Decision == readAdmittedDecision {
				t.Fatalf("requesting-root validation = %+v, %v", got, err)
			}
		})
	}
}

func TestReadAdmissionErrorsAreNeverAdmitted(t *testing.T) {
	valid := admissionLiveSubject("implementer", "implementer", "a", "1")
	if got, err := CritiqueReadAdmission(t.TempDir(), "code-critic", "INVALID", 1, valid); err == nil || got.Decision == readAdmittedDecision {
		t.Fatalf("invalid input result = %+v, %v", got, err)
	}

	repo := t.TempDir()
	seedProvenCleanRead(t, repo, "critic", "code-critic", 1, valid, true)
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
	root := readJSONFile(t, rootPath)
	root[cleanReadRoundsField] = []any{map[string]any{"round": 1, "subject": "forged"}}
	if err := writeRecord(rootPath, root); err != nil {
		t.Fatal(err)
	}
	if got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, valid); err == nil || got.Decision == readAdmittedDecision {
		t.Fatalf("evidence error result = %+v, %v", got, err)
	}
}

func TestReadAdmissionRefusesCleanSubject(t *testing.T) {
	repo := t.TempDir()
	live := admissionLiveSubject("implementer", "implementer", "a", "1")
	seedProvenCleanRead(t, repo, "live-critic", "code-critic", 1, live, true)

	result, err := CritiqueReadAdmission(repo, "code-critic", "fresh-live", 1, live)
	assertReadRefusal(t, result, err, redundantReadRefusal, "live-critic", 1)
	result, err = CritiqueReadAdmission(repo, "code-critic", "live-critic", 2, live)
	assertReadRefusal(t, result, err, redundantReadRefusal, "live-critic", 1)

	design := admissionDesignSubject("metasystem/plans/design.md", "b", "3", "4")
	seedProvenCleanRead(t, repo, "design-critic", "design-critic", 1, design, true)
	result, err = CritiqueReadAdmission(repo, "design-critic", "fresh-design", 1, design)
	assertReadRefusal(t, result, err, redundantReadRefusal, "design-critic", 1)
	result, err = CritiqueReadAdmission(repo, "design-critic", "design-critic", 2, design)
	assertReadRefusal(t, result, err, redundantReadRefusal, "design-critic", 1)

	for name, changed := range map[string]ReadSubject{
		"tree":    admissionLiveSubject("implementer", "implementer", "c", "1"),
		"diff":    admissionLiveSubject("implementer", "implementer", "a", "5"),
		"content": admissionDesignSubject("metasystem/plans/design.md", "c", "3", "4"),
		"outputs": admissionDesignSubject("metasystem/plans/design.md", "b", "5", "4"),
	} {
		role := "code-critic"
		if changed.Kind == SubjectDesign {
			role = "design-critic"
		}
		got, err := CritiqueReadAdmission(repo, role, "changed-"+name, 1, changed)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("changed %s admission = %+v, %v", name, got, err)
		}
	}
}

func TestReadAdmissionDirectsClosedLiveRootToImplementationClose(t *testing.T) {
	subject := admissionLiveSubject("implementer", "implementer", "a", "1")

	t.Run("validated closed live root", func(t *testing.T) {
		repo := t.TempDir()
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, true)
		closeReadyCriticChain(t, repo, "critic", "critic")
		if err := CritiqueChainClose(repo, "critic", false); err != nil {
			t.Fatal(err)
		}
		result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		assertReadRefusal(t, result, err, redundantReadRefusal, "critic", 1)
		for _, text := range []string{
			"next: dispatch.sh close --job implementer --reconcile-evidence critic",
			"completion still checks terminal coverage and required evidence",
		} {
			if !strings.Contains(err.Error(), text) {
				t.Fatalf("closed-root recovery %q does not contain %q", err, text)
			}
		}
	})

	t.Run("open clean root", func(t *testing.T) {
		repo := t.TempDir()
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, true)
		closeReadyCriticChain(t, repo, "critic", "critic")
		result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		assertReadRefusal(t, result, err, redundantReadRefusal, "critic", 1)
		if !strings.Contains(err.Error(), "next: dispatch.sh close --job critic") || strings.Contains(err.Error(), "--reconcile-evidence") {
			t.Fatalf("open-root recovery changed: %v", err)
		}
	})

	t.Run("historical read superseded by later round", func(t *testing.T) {
		repo := t.TempDir()
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, true)
		writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "critic-r2.json", map[string]any{
			"jobId": "critic-r2", "role": "code-critic", "round": 2,
			"parentJob": "critic", "status": "completed", "reviews": "implementer",
		})
		result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
		assertReadRefusal(t, result, err, redundantReadRefusal, "critic", 1)
		if !strings.Contains(err.Error(), "later round 2") || strings.Contains(err.Error(), "--reconcile-evidence") {
			t.Fatalf("historical recovery changed: %v", err)
		}
	})
}

func TestReadAdmissionChecksScopeAndRole(t *testing.T) {
	t.Run("scope-and-role", func(t *testing.T) {
		repo := t.TempDir()
		live := admissionLiveSubject("implementer-a", "implementer-a", "a", "1")
		seedProvenCleanRead(t, repo, "code-read", "code-critic", 1, live, true)
		seedProvenCleanRead(t, repo, "warden-read", "warden", 1, live, true)
		design := admissionDesignSubject("metasystem/plans/a.md", "b", "3", "4")
		seedProvenCleanRead(t, repo, "design-read", "design-critic", 1, design, true)

		otherImplementer := admissionLiveSubject("implementer-b", "implementer-b", "a", "1")
		got, err := CritiqueReadAdmission(repo, "code-critic", "other-live", 1, otherImplementer)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("different implementer admission = %+v, %v", got, err)
		}
		got, err = CritiqueReadAdmission(repo, "design-critic", "other-design", 1, admissionDesignSubject("metasystem/plans/b.md", "b", "3", "4"))
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("different design path admission = %+v, %v", got, err)
		}
		wardenOnly := t.TempDir()
		seedProvenCleanRead(t, wardenOnly, "warden-read", "warden", 1, live, true)
		got, err = CritiqueReadAdmission(wardenOnly, "code-critic", "code-candidate", 1, live)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("different role admission = %+v, %v", got, err)
		}

		live.ReviewedMember = "implementer-a-r2"
		got, err = CritiqueReadAdmission(repo, "code-critic", "provenance-live", 1, live)
		assertReadRefusal(t, got, err, redundantReadRefusal, "code-read", 1)
		design.ReviewedCommit = strings.Repeat("9", 40)
		got, err = CritiqueReadAdmission(repo, "design-critic", "provenance-design", 1, design)
		assertReadRefusal(t, got, err, redundantReadRefusal, "design-read", 1)
	})

	t.Run("malformed-relevant-history", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 1, subject, true)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, path)
		root[cleanReadRoundsField] = []any{map[string]any{"round": 1, "subject": "forged"}}
		if err := writeRecord(path, root); err != nil {
			t.Fatal(err)
		}
		if _, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject); err == nil || !strings.Contains(err.Error(), "clean read") {
			t.Fatalf("malformed relevant history = %v", err)
		}
	})

	t.Run("absent-history-is-not-invented", func(t *testing.T) {
		repo := t.TempDir()
		old := admissionLiveSubject("implementer", "implementer", "a", "1")
		current := admissionLiveSubject("implementer", "implementer", "b", "2")
		seedProvenCleanRead(t, repo, "critic", "code-critic", 2, current, false)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", "critic.json")
		root := readJSONFile(t, path)
		root[findingRegisterField] = encodeFindingRegister([]registerFinding{{
			FindingID: "F-1", Critic: "critic-r2", RigorClass: "severe", FactsDigest: digestJSON(nil),
			Status: "open", EvidenceDigest: digestJSON(nil), Multiplicity: 1,
		}})
		if err := writeRecord(path, root); err != nil {
			t.Fatal(err)
		}
		writeSubjectReturn(t, repo, "critic", "critic", 1, old)
		got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, old)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("absent history admission = %+v, %v", got, err)
		}
	})

	for _, tc := range []struct {
		role    string
		subject ReadSubject
	}{
		{"implementer", admissionLiveSubject("impl", "impl", "a", "1")},
		{"design-critic", admissionLiveSubject("impl", "impl", "a", "1")},
		{"code-critic", admissionDesignSubject("metasystem/plans/a.md", "a", "1", "2")},
	} {
		if _, err := CritiqueReadAdmission(t.TempDir(), tc.role, "candidate", 1, tc.subject); err == nil {
			t.Fatalf("role %s accepted subject kind %s", tc.role, tc.subject.Kind)
		}
	}
}

func seedUnfoldedRead(t *testing.T, repo, root, role, status string, subject ReadSubject) {
	t.Helper()
	seedImplementerScope(t, repo, subject)
	record := map[string]any{
		"jobId": root, "role": role, "round": 1, "parentJob": nil, "status": status,
		findingRegisterField: []any{}, findingRegisterRoundField: 0,
	}
	if subject.Kind == SubjectLive {
		record["reviews"] = subject.ReviewedMember
	} else {
		record["design"] = subject.DesignPath
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), root+".json", record)
	writeSubjectReturn(t, repo, root, root, 1, subject)
}

func TestReadAdmissionRefusesConcurrentLive(t *testing.T) {
	for _, status := range []string{"running", "pending-setup", "completed", "failed"} {
		t.Run(status+"-unfolded", func(t *testing.T) {
			repo := t.TempDir()
			subject := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedUnfoldedRead(t, repo, "outstanding", "code-critic", status, subject)
			result, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
			assertReadRefusal(t, result, err, concurrentReadRefusal, "outstanding", 1)
		})
	}

	for _, tc := range []struct {
		name   string
		mutate func(string, ReadSubject)
	}{
		{"cancelled", func(repo string, _ ReadSubject) {
			path := filepath.Join(repo, "artifacts", "agents", "jobs", "outstanding.json")
			record := readJSONFile(t, path)
			record["status"] = "cancelled"
			_ = writeRecord(path, record)
		}},
		{"folded", func(repo string, _ ReadSubject) {
			path := filepath.Join(repo, "artifacts", "agents", "jobs", "outstanding.json")
			record := readJSONFile(t, path)
			record["status"] = "completed"
			record[findingRegisterRoundField] = 1
			record[findingRegisterField] = encodeFindingRegister([]registerFinding{{
				FindingID: "F-1", Critic: "outstanding", RigorClass: "severe", FactsDigest: digestJSON(nil),
				Status: "open", EvidenceDigest: digestJSON(nil), Multiplicity: 1,
			}})
			_ = writeRecord(path, record)
		}},
		{"different-role", func(repo string, _ ReadSubject) {
			path := filepath.Join(repo, "artifacts", "agents", "jobs", "outstanding.json")
			record := readJSONFile(t, path)
			record["role"] = "warden"
			_ = writeRecord(path, record)
		}},
		{"different-subject", func(repo string, _ ReadSubject) {
			_ = WriteReadSubject(filepath.Join(repo, "artifacts", "agents", "outstanding", "rounds", "1", "subject.json"), admissionLiveSubject("implementer", "implementer", "b", "1"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			subject := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedUnfoldedRead(t, repo, "outstanding", "code-critic", "running", subject)
			tc.mutate(repo, subject)
			got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject)
			if err != nil || got.Decision != readAdmittedDecision {
				t.Fatalf("admission = %+v, %v", got, err)
			}
		})
	}

	t.Run("own-root", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionLiveSubject("implementer", "implementer", "a", "1")
		seedUnfoldedRead(t, repo, "outstanding", "code-critic", "running", subject)
		got, err := CritiqueReadAdmission(repo, "code-critic", "outstanding", 2, subject)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("own-root admission = %+v, %v", got, err)
		}
	})

	t.Run("invalid-and-duplicate-highest-rounds", func(t *testing.T) {
		for _, duplicate := range []bool{false, true} {
			repo := t.TempDir()
			subject := admissionLiveSubject("implementer", "implementer", "a", "1")
			seedUnfoldedRead(t, repo, "outstanding", "code-critic", "running", subject)
			if duplicate {
				writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), "outstanding-other.json", map[string]any{
					"jobId": "outstanding-other", "role": "code-critic", "round": 1, "parentJob": "outstanding", "status": "running",
				})
			} else {
				path := filepath.Join(repo, "artifacts", "agents", "jobs", "outstanding.json")
				record := readJSONFile(t, path)
				record["round"] = "one"
				if err := writeRecord(path, record); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, subject); err == nil {
				t.Fatalf("duplicate=%v malformed latest state was accepted", duplicate)
			}
		}
	})

	t.Run("design-has-no-concurrency-rule", func(t *testing.T) {
		repo := t.TempDir()
		subject := admissionDesignSubject("metasystem/plans/a.md", "a", "1", "2")
		seedUnfoldedRead(t, repo, "outstanding", "design-critic", "running", subject)
		got, err := CritiqueReadAdmission(repo, "design-critic", "candidate", 1, subject)
		if err != nil || got.Decision != readAdmittedDecision {
			t.Fatalf("design admission = %+v, %v", got, err)
		}
	})
}

func TestReadAdmissionExemptsCommitSubjects(t *testing.T) {
	commit := ReadSubject{Kind: SubjectCommit, Commit: strings.Repeat("a", 40), Parent: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40), DiffDigest: strings.Repeat("d", 64)}
	for _, clean := range []bool{false, true} {
		t.Run(map[bool]string{false: "unfolded", true: "proven-clean"}[clean], func(t *testing.T) {
			repo := t.TempDir()
			if clean {
				seedProvenCleanRead(t, repo, "prior", "code-critic", 1, commit, true)
			} else {
				seedUnfoldedRead(t, repo, "prior", "code-critic", "running", commit)
			}
			got, err := CritiqueReadAdmission(repo, "code-critic", "candidate", 1, commit)
			if err != nil || got.Decision != readAdmittedDecision {
				t.Fatalf("commit admission = %+v, %v", got, err)
			}
		})
	}
	malformed := commit
	malformed.Tree = ""
	if _, err := CritiqueReadAdmission(t.TempDir(), "code-critic", "candidate", 1, malformed); err == nil {
		t.Fatal("malformed commit subject was exempt")
	}
	if _, err := CritiqueReadAdmission(t.TempDir(), "design-critic", "candidate", 1, commit); err == nil {
		t.Fatal("role-mismatched commit subject was exempt")
	}
}

func assertReadRefusal(t *testing.T, result ReadAdmissionResult, err error, reason, root string, round int64) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) || op.Code != 11 || op.Reason != reason {
		t.Fatalf("refusal = %+v, %T %v", result, err, err)
	}
	if result.Decision != reason || result.CriticRoot != root || result.Round != round || result.SubjectDigest == "" {
		t.Fatalf("refusal result = %+v", result)
	}
	if !strings.Contains(err.Error(), root) || !strings.Contains(err.Error(), result.SubjectDigest) {
		t.Fatalf("diagnostic lacks root or digest: %v", err)
	}
}

// TestReadAdmissionRefusesFreshDesignRootWhileChainOpen: a fresh
// design-critic root for a goal is refused while that goal's chain of the
// same document is not closed, running, failed, capped or exhausted, and
// whatever bytes the document now has. The chain's own next round, another
// goal, another document, a closed chain and a goal-less dispatch are
// admitted as before.
func TestReadAdmissionRefusesFreshDesignRootWhileChainOpen(t *testing.T) {
	t.Parallel()
	path := "plans/designs/reader.md"
	seed := func(t *testing.T, status string, change func(map[string]any)) string {
		repo := t.TempDir()
		seedUnfoldedRead(t, repo, "chain", "design-critic", status, admissionDesignSubject(path, "a", "1", "c"))
		file := filepath.Join(repo, "artifacts", "agents", "jobs", "chain.json")
		record := readJSONFile(t, file)
		record["goalId"] = "g"
		if change != nil {
			change(record)
		}
		if err := writeRecord(file, record); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	changed := admissionDesignSubject(path, "b", "1", "c")
	for _, status := range []string{"running", "failed", "completed"} {
		repo := seed(t, status, nil)
		result, err := CritiqueReadAdmissionForGoal(repo, "design-critic", "fresh", "g", 1, changed)
		assertReadRefusal(t, result, err, designChainOpenRefusal, "chain", 1)
	}
	exhausted := seed(t, "completed", func(record map[string]any) { record["chainExhausted"] = true })
	result, err := CritiqueReadAdmissionForGoal(exhausted, "design-critic", "fresh", "g", 1, changed)
	assertReadRefusal(t, result, err, designChainOpenRefusal, "chain", 1)

	for name, admit := range map[string]func(string) (ReadAdmissionResult, error){
		"own next round": func(repo string) (ReadAdmissionResult, error) {
			return CritiqueReadAdmissionForGoal(repo, "design-critic", "chain", "g", 2, changed)
		},
		"another goal": func(repo string) (ReadAdmissionResult, error) {
			return CritiqueReadAdmissionForGoal(repo, "design-critic", "fresh", "other", 1, changed)
		},
		"another document": func(repo string) (ReadAdmissionResult, error) {
			return CritiqueReadAdmissionForGoal(repo, "design-critic", "fresh", "g", 1, admissionDesignSubject("plans/designs/writer.md", "b", "1", "c"))
		},
		"no goal": func(repo string) (ReadAdmissionResult, error) {
			return CritiqueReadAdmission(repo, "design-critic", "fresh", 1, changed)
		},
	} {
		if result, err := admit(seed(t, "running", nil)); err != nil || result.Decision != readAdmittedDecision {
			t.Fatalf("%s: %+v %v", name, result, err)
		}
	}
	closed := seed(t, "completed", func(record map[string]any) { record["chainClosed"] = true })
	if result, err := CritiqueReadAdmissionForGoal(closed, "design-critic", "fresh", "g", 1, changed); err != nil || result.Decision != readAdmittedDecision {
		t.Fatalf("closed chain: %+v %v", result, err)
	}
}
