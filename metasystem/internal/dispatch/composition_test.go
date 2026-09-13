package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func compositionRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Every caller composes against the real checkout, so an operator's
	// exported cap would decide these results. Pin the configured cap for the
	// whole test and restore whatever the environment had.
	if original, present := os.LookupEnv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); present {
		t.Cleanup(func() { _ = os.Setenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB", original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB") })
	}
	if err := os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); err != nil {
		t.Fatal(err)
	}
	return root
}

func compositionStageDir(t *testing.T) string {
	t.Helper()
	root := compositionRepoRoot(t)
	testRoot := filepath.Join(root, "artifacts", "agents", "test-"+t.Name())
	t.Cleanup(func() { os.RemoveAll(testRoot) })
	return filepath.Join(testRoot, "rounds", "1", "staged")
}

func compositionTemporaryStageDir(t *testing.T) string {
	t.Helper()
	root := compositionRepoRoot(t)
	parent := filepath.Join(root, "artifacts", "agents", "record-locks")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(parent, "composition-test.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func packetFitRoot(t *testing.T) string {
	t.Helper()
	sourceRoot := compositionRepoRoot(t)
	root := t.TempDir()
	copyFile := func(path string) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		rolePacketTablePath,
		"scripts/agents/roles/implementer.md",
		"docs/orchestration.md",
		"scripts/agents/schemas/implementer.schema.json",
		"scripts/agents/roles/verifier.md",
		"skills/verify/SKILL.md",
		"scripts/agents/schemas/verifier.schema.json",
	} {
		copyFile(path)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertPacketFitProvenance(t *testing.T, p ComposeRolePacketParams, record CompositionRecord, rawBySlot map[string][]byte) []byte {
	t.Helper()
	packet, err := os.ReadFile(p.Output)
	if err != nil {
		t.Fatal(err)
	}
	storedBytes, err := os.ReadFile(p.CompositionOutput)
	if err != nil {
		t.Fatal(err)
	}
	var stored CompositionRecord
	if err := json.Unmarshal(storedBytes, &stored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, record) {
		t.Fatalf("stored composition differs from returned record\nstored: %+v\nreturned: %+v", stored, record)
	}

	type expectedSource struct {
		slot   string
		source string
		raw    []byte
	}
	_, _, recipe, err := readRolePacketRecipe(p.Root, p.Role)
	if err != nil {
		t.Fatal(err)
	}
	expected := []expectedSource{{slot: "task-direction", source: "caller:brief", raw: rawBySlot["task-direction"]}}
	for _, recipeSource := range recipe.Sources {
		raw, err := readRecipeSource(p.Root, recipeSource)
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, expectedSource{slot: recipeSource.Slot, source: recipeSource.Path, raw: raw})
	}
	if recipe.ArtifactMember != "" {
		expected = append(expected, expectedSource{slot: "artifact-member", source: rolePacketTablePath + "#" + p.Role + ".artifactMember", raw: []byte(recipe.ArtifactMember + "\n")})
	}
	toolNotice := fmt.Sprintf("Permission tool policy: %s\n", p.ToolPolicy)
	if p.Runtime == "fake" {
		toolNotice += "Tool-name observation: exact\nTool names: (none; the fake runtime opens no model tool channel)\n"
	} else {
		toolNotice += "Tool-name observation: unobserved\nTool names: UNOBSERVED; this broad-read launcher cannot prove the provider tool catalog, so this job is advisory.\n"
	}
	expected = append(expected, expectedSource{slot: "tool-names", source: "generated:tool-names", raw: []byte(toolNotice)})
	configuration, err := ResolveHazardConfiguration(p.Root, p.DestructiveReach, p.GoalTier)
	if err != nil {
		t.Fatal(err)
	}
	identity := fmt.Sprintf("Job-Id: %s\nRole: %s\nRuntime: %s\nModel: %s\nRound: %d\nMission: %s\nDestructive reach class: %s\nBuilder effort tier: %s\nBuilder reasoning effort: %s\nIndependent critique required: %t\nIndependent critique effort tier: %s\nIndependent critique reasoning effort: %s\nLive proof required: %t\n",
		p.JobID, p.Role, p.Runtime, p.Model, p.Round, emptyAsNone(p.Mission), p.DestructiveReach,
		configuration.BuilderEffortTier, configuration.BuilderReasoningEffort,
		configuration.IndependentCritiqueRequired, configuration.IndependentCritiqueEffortTier,
		configuration.IndependentCritiqueReasoningEffort, configuration.LiveProofRequired)
	runtimeNotice := identity + "Context classification: advisory. This broad-read runtime does not prove context isolation or independent examination.\n" + returnBySentence(p.CapMinutes, p.ReturnMarginMinutes, p.CapTruncated)
	expected = append(expected, expectedSource{slot: "generated-runtime-notice", source: "generated:runtime-notice", raw: []byte(runtimeNotice)})
	for _, continuation := range p.Continuations {
		raw, ok := rawBySlot[continuation.Slot]
		if !ok {
			t.Fatalf("missing expected raw bytes for %s", continuation.Slot)
		}
		expected = append(expected, expectedSource{slot: continuation.Slot, source: "engine:" + continuation.Slot, raw: raw})
	}
	if len(record.Sources) != len(expected) {
		t.Fatalf("source count = %d, want %d", len(record.Sources), len(expected))
	}
	previousEnd := 0
	for index, want := range expected {
		got := record.Sources[index]
		if got.Slot != want.slot || got.Source != want.source || got.SourceDigest != digestBytes(want.raw) || got.SourceBytes != len(want.raw) {
			t.Fatalf("source %d = %+v, want slot=%s source=%s bytes=%d digest=%s", index, got, want.slot, want.source, len(want.raw), digestBytes(want.raw))
		}
		if got.StartByte != previousEnd || got.EndByte <= got.StartByte || got.EndByte > len(packet) {
			t.Fatalf("source %s has invalid range %d:%d after %d", got.Slot, got.StartByte, got.EndByte, previousEnd)
		}
		if got.DeliveredDigest != digestBytes(packet[got.StartByte:got.EndByte]) {
			t.Fatalf("source %s delivered digest does not match its packet range", got.Slot)
		}
		previousEnd = got.EndByte
	}
	if previousEnd != len(packet) || record.PacketDigest != digestBytes(packet) {
		t.Fatalf("packet provenance ends at %d of %d bytes with digest %s, want %s", previousEnd, len(packet), record.PacketDigest, digestBytes(packet))
	}

	references := make(map[string]CompositionReference, len(record.References))
	for _, reference := range record.References {
		if _, exists := references[reference.Slot]; exists {
			t.Fatalf("duplicate reference for %s", reference.Slot)
		}
		references[reference.Slot] = reference
	}
	wantReferenceOrder := make([]string, 0, len(record.References))
	canonicalRoot := resolvePath(p.Root)
	canonicalReference := resolvePath(p.ReferenceDir)
	canonicalStage := resolvePath(p.StageDir)
	for index, want := range expected {
		purpose := map[string]string{
			"task-direction": "brief", "prior-brief": "brief", "prior-return": "return",
			"critique-register": "critique", "prior-worktree": "evidence",
		}[want.slot]
		if purpose == "" {
			continue
		}
		reference, referenced := references[want.slot]
		stagePath := filepath.Join(canonicalStage, want.slot+".md")
		if !referenced {
			if _, statErr := os.Stat(stagePath); !os.IsNotExist(statErr) {
				t.Fatalf("inline body %s has staged file %s", want.slot, stagePath)
			}
			continue
		}
		wantReferenceOrder = append(wantReferenceOrder, want.slot)
		if record.References[len(wantReferenceOrder)-1].Slot != want.slot {
			t.Fatalf("reference %d has slot %s, want packet-order slot %s", len(wantReferenceOrder)-1, record.References[len(wantReferenceOrder)-1].Slot, want.slot)
		}
		openPath := filepath.Join(canonicalReference, want.slot+".md")
		rel, err := filepath.Rel(canonicalRoot, openPath)
		if err != nil {
			t.Fatal(err)
		}
		if reference.Purpose != purpose || reference.Path != filepath.ToSlash(rel) || reference.OpenPath != openPath ||
			reference.Digest != digestBytes(want.raw) || reference.Bytes != len(want.raw) || reference.Lifetime != "staged" {
			t.Fatalf("reference for %s = %+v", want.slot, reference)
		}
		staged, err := os.ReadFile(stagePath)
		if err != nil || !bytes.Equal(staged, want.raw) {
			t.Fatalf("staged body %s does not match its source: %v", want.slot, err)
		}
		stanza := fmt.Sprintf("Referenced body: open %s (%d bytes, sha256 %s). Read it in bounded views; it is not inlined.\n", openPath, len(want.raw), digestBytes(want.raw))
		delivered := record.Sources[index]
		if !bytes.Contains(packet[delivered.StartByte:delivered.EndByte], []byte(stanza)) {
			t.Fatalf("delivered range for %s lacks its exact final-path stanza", want.slot)
		}
	}
	if len(wantReferenceOrder) != len(record.References) {
		t.Fatalf("reference count = %d, want %d", len(record.References), len(wantReferenceOrder))
	}
	for index, slot := range wantReferenceOrder {
		if record.References[index].Slot != slot {
			t.Fatalf("reference %d slot = %s, want %s", index, record.References[index].Slot, slot)
		}
	}
	if _, err := readCompositionForJob(p.CompositionOutput, p.JobID, p.Role, p.Runtime, p.Model, p.Mission, p.DestructiveReach, p.GoalTier, p.Round, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("final composition provenance was refused: %v", err)
	}
	return packet
}

func TestComposeRolePacketStagesBriefToFitPacketCap(t *testing.T) {
	if original, present := os.LookupEnv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); present {
		t.Cleanup(func() { _ = os.Setenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB", original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB") })
	}
	if err := os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); err != nil {
		t.Fatal(err)
	}
	root := packetFitRoot(t)
	cases := []struct {
		name string
		body []byte
	}{
		{name: "sixteen-kibibytes", body: bytes.Repeat([]byte("b"), 16*1024)},
		{name: "exact-bound-multibyte", body: bytes.Repeat([]byte("é"), MaxDirectiveBytes/2)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			brief := filepath.Join(root, testCase.name+"-brief.md")
			if err := os.WriteFile(brief, testCase.body, 0o644); err != nil {
				t.Fatal(err)
			}
			params := ComposeRolePacketParams{
				Root: root, Role: "implementer", Brief: brief, JobID: testCase.name,
				Runtime: "fake", Model: "fake-model", ToolPolicy: "read-write", Round: 1,
				DestructiveReach: HazardMechanical,
				Output:           filepath.Join(root, testCase.name+"-prompt.md"), CompositionOutput: filepath.Join(root, testCase.name+"-composition.json"),
				StageDir: filepath.Join(root, "record-locks", testCase.name), ReferenceDir: filepath.Join(root, "artifacts", testCase.name, "rounds", "1", "staged"),
			}
			record, err := ComposeRolePacket(params)
			if err != nil {
				t.Fatal(err)
			}
			packet := assertPacketFitProvenance(t, params, record, map[string][]byte{"task-direction": testCase.body})
			if len(record.References) != 1 || record.References[0].Slot != "task-direction" {
				t.Fatal("the brief was not staged to fit the complete packet")
			}
			if len(packet) > 65536 {
				t.Fatalf("packet has %d bytes, cap is 65536", len(packet))
			}
		})
	}
}

func TestComposeRolePacketFitsCombinedContinuations(t *testing.T) {
	if original, present := os.LookupEnv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); present {
		t.Cleanup(func() { _ = os.Setenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB", original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB") })
	}
	if err := os.Unsetenv("METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"); err != nil {
		t.Fatal(err)
	}
	type bodyFixture struct {
		slot string
		raw  []byte
	}
	cases := []struct {
		name       string
		capKiB     int
		fixedSizes [3]int
		task       []byte
		bodies     []bodyFixture
		wantRefs   []string
	}{
		{
			name: "repeated-selection", capKiB: 24, fixedSizes: [3]int{1024, 4096, 1024}, task: bytes.Repeat([]byte("t"), 8*1024),
			bodies:   []bodyFixture{{slot: "prior-brief", raw: bytes.Repeat([]byte("b"), 12*1024)}, {slot: "prior-return", raw: bytes.Repeat([]byte("r"), 16*1024)}},
			wantRefs: []string{"prior-brief", "prior-return"},
		},
		{
			name: "saving-is-not-raw-length", capKiB: 8, fixedSizes: [3]int{256, 512, 256}, task: bytes.Repeat([]byte("t"), 4097),
			bodies: []bodyFixture{{slot: "prior-brief", raw: bytes.Repeat([]byte("b"), 4096)}}, wantRefs: []string{"prior-brief"},
		},
		{
			name: "equal-saving-prefers-packet-order", capKiB: 24, fixedSizes: [3]int{1024, 4096, 1024}, task: bytes.Repeat([]byte("t"), 12*1024),
			bodies: []bodyFixture{{slot: "prior-worktree", raw: bytes.Repeat([]byte("w"), 12*1024)}}, wantRefs: []string{"task-direction"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := packetFitRoot(t)
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(fmt.Sprintf("dispatch.max-inline-input-kb=%d\n", testCase.capKiB)), 0o644); err != nil {
				t.Fatal(err)
			}
			fixedPaths := []string{
				"scripts/agents/roles/implementer.md",
				"docs/orchestration.md",
				"scripts/agents/schemas/implementer.schema.json",
			}
			for index, path := range fixedPaths {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), bytes.Repeat([]byte{byte('a' + index)}, testCase.fixedSizes[index]), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			briefPath := filepath.Join(root, testCase.name+"-brief.md")
			if err := os.WriteFile(briefPath, testCase.task, 0o644); err != nil {
				t.Fatal(err)
			}
			rawBySlot := map[string][]byte{"task-direction": testCase.task}
			continuations := make([]CompositionContinuation, 0, len(testCase.bodies))
			for _, body := range testCase.bodies {
				path := filepath.Join(root, testCase.name+"-"+body.slot+".md")
				if err := os.WriteFile(path, body.raw, 0o644); err != nil {
					t.Fatal(err)
				}
				continuations = append(continuations, CompositionContinuation{Slot: body.slot, Path: path})
				rawBySlot[body.slot] = body.raw
			}
			referenceDir := filepath.Join(root, "artifacts", testCase.name, "rounds", "2", "staged")
			params := ComposeRolePacketParams{
				Root: root, Role: "implementer", Brief: briefPath, JobID: testCase.name,
				Runtime: "fake", Model: "fake-model", ToolPolicy: "read-write", Round: 2,
				DestructiveReach: HazardMechanical, Continuations: continuations,
				Output: filepath.Join(root, testCase.name+"-prompt.md"), CompositionOutput: filepath.Join(root, testCase.name+"-composition.json"),
				StageDir: filepath.Join(root, "record-locks", testCase.name+"-first"), ReferenceDir: referenceDir,
			}

			type renderedFixtureSection struct {
				slot string
				raw  []byte
			}
			fixtureSections := []renderedFixtureSection{{slot: "task-direction", raw: testCase.task}}
			for index, slot := range []string{"role-instructions", "required-skill", "response-contract"} {
				fixtureSections = append(fixtureSections, renderedFixtureSection{slot: slot, raw: bytes.Repeat([]byte{byte('a' + index)}, testCase.fixedSizes[index])})
			}
			toolNotice := []byte("Permission tool policy: read-write\nTool-name observation: exact\nTool names: (none; the fake runtime opens no model tool channel)\n")
			fixtureSections = append(fixtureSections, renderedFixtureSection{slot: "tool-names", raw: toolNotice})
			configuration, err := ResolveHazardConfiguration(root, HazardMechanical, 0)
			if err != nil {
				t.Fatal(err)
			}
			identity := fmt.Sprintf("Job-Id: %s\nRole: implementer\nRuntime: fake\nModel: fake-model\nRound: 2\nMission: none\nDestructive reach class: MECHANICAL\nBuilder effort tier: %s\nBuilder reasoning effort: %s\nIndependent critique required: %t\nIndependent critique effort tier: %s\nIndependent critique reasoning effort: %s\nLive proof required: %t\nContext classification: advisory. This broad-read runtime does not prove context isolation or independent examination.\n",
				testCase.name, configuration.BuilderEffortTier, configuration.BuilderReasoningEffort,
				configuration.IndependentCritiqueRequired, configuration.IndependentCritiqueEffortTier,
				configuration.IndependentCritiqueReasoningEffort, configuration.LiveProofRequired)
			fixtureSections = append(fixtureSections, renderedFixtureSection{slot: "generated-runtime-notice", raw: []byte(identity)})
			for _, body := range testCase.bodies {
				fixtureSections = append(fixtureSections, renderedFixtureSection(body))
			}
			render := func(referenceSlots map[string]bool) []byte {
				t.Helper()
				var packet bytes.Buffer
				for _, section := range fixtureSections {
					body := section.raw
					if referenceSlots[section.slot] {
						openPath := filepath.Join(referenceDir, section.slot+".md")
						body = []byte(fmt.Sprintf("Referenced body: open %s (%d bytes, sha256 %s). Read it in bounded views; it is not inlined.\n", openPath, len(section.raw), digestBytes(section.raw)))
					}
					packet.WriteString("# " + packetHeading(section.slot) + "\n\n")
					packet.Write(body)
					if packet.Bytes()[packet.Len()-1] != '\n' {
						packet.WriteByte('\n')
					}
					packet.WriteByte('\n')
				}
				return packet.Bytes()
			}
			sectionSize := func(slot string, raw []byte, referenced bool) int {
				t.Helper()
				body := raw
				if referenced {
					openPath := filepath.Join(referenceDir, slot+".md")
					body = []byte(fmt.Sprintf("Referenced body: open %s (%d bytes, sha256 %s). Read it in bounded views; it is not inlined.\n", openPath, len(raw), digestBytes(raw)))
				}
				size := len("# "+packetHeading(slot)+"\n\n") + len(body) + 1
				if len(body) == 0 || body[len(body)-1] != '\n' {
					size++
				}
				return size
			}
			capBytes := int64(testCase.capKiB * 1024)
			if int64(len(render(nil))) <= capBytes {
				t.Fatal("fixture inline packet does not exceed its cap")
			}
			switch testCase.name {
			case "repeated-selection":
				if int64(len(render(map[string]bool{"prior-return": true}))) <= capBytes || int64(len(render(map[string]bool{"prior-brief": true, "prior-return": true}))) > capBytes {
					t.Fatal("fixture does not require both continuation substitutions")
				}
			case "saving-is-not-raw-length":
				taskSaving := sectionSize("task-direction", testCase.task, false) - sectionSize("task-direction", testCase.task, true)
				priorSaving := sectionSize("prior-brief", testCase.bodies[0].raw, false) - sectionSize("prior-brief", testCase.bodies[0].raw, true)
				if priorSaving <= taskSaving || int64(len(render(map[string]bool{"prior-brief": true}))) > capBytes {
					t.Fatalf("fixture savings task=%d prior-brief=%d do not select the shorter body", taskSaving, priorSaving)
				}
			case "equal-saving-prefers-packet-order":
				taskSaving := sectionSize("task-direction", testCase.task, false) - sectionSize("task-direction", testCase.task, true)
				worktreeSaving := sectionSize("prior-worktree", testCase.bodies[0].raw, false) - sectionSize("prior-worktree", testCase.bodies[0].raw, true)
				if taskSaving <= 0 || taskSaving != worktreeSaving || int64(len(render(map[string]bool{"task-direction": true}))) > capBytes {
					t.Fatalf("fixture savings task=%d prior-worktree=%d do not establish an equal positive tie", taskSaving, worktreeSaving)
				}
			}

			record, err := ComposeRolePacket(params)
			if err != nil {
				t.Fatal(err)
			}
			packet := assertPacketFitProvenance(t, params, record, rawBySlot)
			if int64(len(packet)) > capBytes {
				t.Fatalf("packet has %d bytes, cap is %d", len(packet), capBytes)
			}
			if len(record.References) != len(testCase.wantRefs) {
				t.Fatalf("references = %+v, want %v", record.References, testCase.wantRefs)
			}
			for index, slot := range testCase.wantRefs {
				if record.References[index].Slot != slot {
					t.Fatalf("reference %d = %s, want %s", index, record.References[index].Slot, slot)
				}
			}

			repeatedParams := params
			repeatedParams.Output = filepath.Join(root, testCase.name+"-repeated-prompt.md")
			repeatedParams.CompositionOutput = filepath.Join(root, testCase.name+"-repeated-composition.json")
			repeatedParams.StageDir = filepath.Join(root, "record-locks", testCase.name+"-second")
			repeated, err := ComposeRolePacket(repeatedParams)
			if err != nil {
				t.Fatal(err)
			}
			repeatedPacket := assertPacketFitProvenance(t, repeatedParams, repeated, rawBySlot)
			if !bytes.Equal(packet, repeatedPacket) || !reflect.DeepEqual(record, repeated) {
				t.Fatal("identical composition changed with a different temporary stage directory")
			}
		})
	}
}

func TestComposeRolePacketHonorsConfiguredPacketCap(t *testing.T) {
	const limitEnv = "METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"
	if original, present := os.LookupEnv(limitEnv); present {
		t.Cleanup(func() { _ = os.Setenv(limitEnv, original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(limitEnv) })
	}
	if err := os.Unsetenv(limitEnv); err != nil {
		t.Fatal(err)
	}

	t.Run("reader-resolution-and-validation", func(t *testing.T) {
		dir := t.TempDir()
		conf := filepath.Join(dir, "metasystem.conf")
		if err := os.WriteFile(conf, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := InlineInputLimitBytes(conf); err != nil || got != 65536 {
			t.Fatalf("missing key = %d, %v; want 65536, nil", got, err)
		}
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=8\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := InlineInputLimitBytes(conf); err != nil || got != 8192 {
			t.Fatalf("configured key = %d, %v; want 8192, nil", got, err)
		}
		if err := os.WriteFile(conf+".local", []byte("dispatch.max-inline-input-kb=16\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := InlineInputLimitBytes(conf); err != nil || got != 16384 {
			t.Fatalf("local override = %d, %v; want 16384, nil", got, err)
		}
		t.Setenv(limitEnv, "32")
		if got, err := InlineInputLimitBytes(conf); err != nil || got != 32768 {
			t.Fatalf("environment override = %d, %v; want 32768, nil", got, err)
		}
		t.Setenv(limitEnv, "9007199254740991")
		if got, err := InlineInputLimitBytes(conf); err != nil || got != int64(9223372036854774784) {
			t.Fatalf("maximum value = %d, %v; want 9223372036854774784, nil", got, err)
		}
	})

	invalidValues := []string{"0", "-1", "+64", "064", "1.5", "abc", "", "9007199254740992", "9223372036854775808", " 64", "64 "}
	for _, value := range invalidValues {
		name := "value-" + strings.NewReplacer(" ", "space", "+", "plus", "-", "minus", ".", "dot").Replace(value)
		if value == "" {
			name = "empty-value"
		}
		t.Run(name, func(t *testing.T) {
			conf := filepath.Join(t.TempDir(), "metasystem.conf")
			if err := os.WriteFile(conf, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(limitEnv, value)
			if _, err := InlineInputLimitBytes(conf); err == nil {
				t.Fatalf("InlineInputLimitBytes accepted %q", value)
			}
		})
	}

	t.Run("duplicate-key", func(t *testing.T) {
		if err := os.Unsetenv(limitEnv); err != nil {
			t.Fatal(err)
		}
		conf := filepath.Join(t.TempDir(), "metasystem.conf")
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=8\ndispatch.max-inline-input-kb=16\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := InlineInputLimitBytes(conf); err == nil {
			t.Fatal("duplicate key was accepted")
		}
	})
	t.Run("unreadable-config", func(t *testing.T) {
		if err := os.Unsetenv(limitEnv); err != nil {
			t.Fatal(err)
		}
		conf := filepath.Join(t.TempDir(), "configuration-directory")
		if err := os.Mkdir(conf, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := InlineInputLimitBytes(conf); err == nil {
			t.Fatal("configuration read error was defaulted")
		}
	})

	composeInvalid := func(t *testing.T, name string, prepare func(string)) {
		t.Helper()
		root := packetFitRoot(t)
		prepare(root)
		brief := filepath.Join(root, name+"-brief.md")
		if err := os.WriteFile(brief, []byte("valid brief\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		params := ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: name, Runtime: "fake", Model: "fake-model",
			ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: filepath.Join(root, name+"-prompt.md"), CompositionOutput: filepath.Join(root, name+"-composition.json"),
			StageDir: filepath.Join(root, "record-locks", name), ReferenceDir: filepath.Join(root, "artifacts", name, "rounds", "1", "staged"),
		}
		if _, err := ComposeRolePacket(params); err == nil {
			t.Fatal("composition accepted invalid inline-input configuration")
		}
		for _, path := range []string{params.Output, params.CompositionOutput, params.StageDir, filepath.Join(params.StageDir, "task-direction.md")} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("invalid configuration published %s", path)
			}
		}
	}
	for _, value := range invalidValues {
		value := value
		name := "compose-invalid-" + strings.NewReplacer(" ", "space", "+", "plus", "-", "minus", ".", "dot").Replace(value)
		if value == "" {
			name = "compose-invalid-empty"
		}
		t.Run(name, func(t *testing.T) {
			composeInvalid(t, name, func(string) { t.Setenv(limitEnv, value) })
		})
	}
	t.Run("compose-duplicate-key", func(t *testing.T) {
		if err := os.Unsetenv(limitEnv); err != nil {
			t.Fatal(err)
		}
		composeInvalid(t, "compose-duplicate-key", func(root string) {
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=8\ndispatch.max-inline-input-kb=16\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("compose-unreadable-config", func(t *testing.T) {
		if err := os.Unsetenv(limitEnv); err != nil {
			t.Fatal(err)
		}
		composeInvalid(t, "compose-unreadable-config", func(root string) {
			conf := filepath.Join(root, "metasystem.conf")
			if err := os.Remove(conf); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(conf, 0o755); err != nil {
				t.Fatal(err)
			}
		})
	})

	t.Run("equality-one-byte-over-and-raised-cap", func(t *testing.T) {
		if err := os.Unsetenv(limitEnv); err != nil {
			t.Fatal(err)
		}
		root := packetFitRoot(t)
		for _, path := range []string{"scripts/agents/roles/verifier.md", "skills/verify/SKILL.md", "scripts/agents/schemas/verifier.schema.json"} {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		conf := filepath.Join(root, "metasystem.conf")
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=64\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		brief := filepath.Join(root, "configured-cap-brief.md")
		if err := os.WriteFile(brief, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		base := ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: "configured-cap", Runtime: "fake", Model: "fake-model",
			ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: filepath.Join(root, "baseline-prompt.md"), CompositionOutput: filepath.Join(root, "baseline-composition.json"),
			StageDir: filepath.Join(root, "record-locks", "baseline"), ReferenceDir: filepath.Join(root, "artifacts", "configured-cap", "rounds", "1", "staged"),
		}
		baselineRecord, err := ComposeRolePacket(base)
		if err != nil {
			t.Fatal(err)
		}
		baseline := assertPacketFitProvenance(t, base, baselineRecord, map[string][]byte{"task-direction": []byte("x")})
		fixedBytes := len(baseline) - 1
		equalityLength := 8192 - fixedBytes
		if equalityLength <= 0 || equalityLength > MaxDirectiveBytes {
			t.Fatalf("fixture needs invalid equality brief length %d (fixed bytes %d)", equalityLength, fixedBytes)
		}
		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=8\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		equalityBody := bytes.Repeat([]byte("e"), equalityLength)
		if err := os.WriteFile(brief, equalityBody, 0o644); err != nil {
			t.Fatal(err)
		}
		equality := base
		equality.Output = filepath.Join(root, "equality-prompt.md")
		equality.CompositionOutput = filepath.Join(root, "equality-composition.json")
		equality.StageDir = filepath.Join(root, "record-locks", "equality")
		equalityRecord, err := ComposeRolePacket(equality)
		if err != nil {
			t.Fatal(err)
		}
		equalityPacket := assertPacketFitProvenance(t, equality, equalityRecord, map[string][]byte{"task-direction": equalityBody})
		if len(equalityPacket) != 8192 || len(equalityRecord.References) != 0 {
			t.Fatalf("equality packet has %d bytes and %d references", len(equalityPacket), len(equalityRecord.References))
		}

		overBody := append(append([]byte(nil), equalityBody...), 'x')
		if err := os.WriteFile(brief, overBody, 0o644); err != nil {
			t.Fatal(err)
		}
		over := base
		over.Output = filepath.Join(root, "over-prompt.md")
		over.CompositionOutput = filepath.Join(root, "over-composition.json")
		over.StageDir = filepath.Join(root, "record-locks", "over")
		overRecord, err := ComposeRolePacket(over)
		if err != nil {
			t.Fatal(err)
		}
		overPacket := assertPacketFitProvenance(t, over, overRecord, map[string][]byte{"task-direction": overBody})
		if len(overRecord.References) != 1 || overRecord.References[0].Slot != "task-direction" || len(overPacket) > 8192 {
			t.Fatalf("one-byte-over packet has %d bytes and references %+v", len(overPacket), overRecord.References)
		}

		if err := os.WriteFile(conf, []byte("dispatch.max-inline-input-kb=16\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		raised := base
		raised.Output = filepath.Join(root, "raised-prompt.md")
		raised.CompositionOutput = filepath.Join(root, "raised-composition.json")
		raised.StageDir = filepath.Join(root, "record-locks", "raised")
		raisedRecord, err := ComposeRolePacket(raised)
		if err != nil {
			t.Fatal(err)
		}
		raisedPacket := assertPacketFitProvenance(t, raised, raisedRecord, map[string][]byte{"task-direction": overBody})
		if len(raisedRecord.References) != 0 || len(raisedPacket) != 8193 {
			t.Fatalf("raised-cap packet has %d bytes and references %+v", len(raisedPacket), raisedRecord.References)
		}
	})
}

func TestComposeRolePacketRefusesWhenFixedPartsCannotFit(t *testing.T) {
	const limitEnv = "METASYSTEM_DISPATCH_MAX_INLINE_INPUT_KB"
	if original, present := os.LookupEnv(limitEnv); present {
		t.Cleanup(func() { _ = os.Setenv(limitEnv, original) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(limitEnv) })
	}
	if err := os.Unsetenv(limitEnv); err != nil {
		t.Fatal(err)
	}

	t.Run("impossible-fit-publishes-nothing", func(t *testing.T) {
		root := packetFitRoot(t)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		fixedPaths := []string{"scripts/agents/roles/verifier.md", "skills/verify/SKILL.md", "scripts/agents/schemas/verifier.schema.json"}
		fixedBodies := make([][]byte, len(fixedPaths))
		for index, path := range fixedPaths {
			fixedBodies[index] = bytes.Repeat([]byte{byte('a' + index)}, 2*1024)
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), fixedBodies[index], 0o644); err != nil {
				t.Fatal(err)
			}
		}
		briefBody := bytes.Repeat([]byte("b"), 40*1024)
		brief := filepath.Join(root, "impossible-brief.md")
		if err := os.WriteFile(brief, briefBody, 0o644); err != nil {
			t.Fatal(err)
		}
		continuationBody := []byte("r")
		continuation := filepath.Join(root, "impossible-prior-return.md")
		if err := os.WriteFile(continuation, continuationBody, 0o644); err != nil {
			t.Fatal(err)
		}
		referenceDir := filepath.Join(root, "artifacts", "impossible", "rounds", "1", "staged")
		params := ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: "impossible", Runtime: "fake", Model: "fake-model",
			ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: filepath.Join(root, "impossible-prompt.md"), CompositionOutput: filepath.Join(root, "impossible-composition.json"),
			StageDir: filepath.Join(root, "record-locks", "impossible"), ReferenceDir: referenceDir,
			Continuations: []CompositionContinuation{{Slot: "prior-return", Path: continuation}},
		}
		canonicalReferenceDir := resolvePath(referenceDir)
		configuration, err := ResolveHazardConfiguration(root, HazardMechanical, 0)
		if err != nil {
			t.Fatal(err)
		}
		toolNotice := []byte("Permission tool policy: read-only\nTool-name observation: exact\nTool names: (none; the fake runtime opens no model tool channel)\n")
		identity := fmt.Sprintf("Job-Id: impossible\nRole: verifier\nRuntime: fake\nModel: fake-model\nRound: 1\nMission: none\nDestructive reach class: MECHANICAL\nBuilder effort tier: %s\nBuilder reasoning effort: %s\nIndependent critique required: %t\nIndependent critique effort tier: %s\nIndependent critique reasoning effort: %s\nLive proof required: %t\nContext classification: advisory. This broad-read runtime does not prove context isolation or independent examination.\n",
			configuration.BuilderEffortTier, configuration.BuilderReasoningEffort,
			configuration.IndependentCritiqueRequired, configuration.IndependentCritiqueEffortTier,
			configuration.IndependentCritiqueReasoningEffort, configuration.LiveProofRequired)
		type fixtureSection struct {
			slot string
			raw  []byte
			ref  bool
		}
		sections := []fixtureSection{{slot: "task-direction", raw: briefBody, ref: true}}
		for index, slot := range []string{"role-instructions", "required-skill", "response-contract"} {
			sections = append(sections, fixtureSection{slot: slot, raw: fixedBodies[index]})
		}
		sections = append(sections,
			fixtureSection{slot: "tool-names", raw: toolNotice},
			fixtureSection{slot: "generated-runtime-notice", raw: []byte(identity)},
			fixtureSection{slot: "prior-return", raw: continuationBody},
		)
		renderSection := func(section fixtureSection, referenced bool) []byte {
			t.Helper()
			body := section.raw
			if referenced {
				openPath := filepath.Join(canonicalReferenceDir, section.slot+".md")
				body = []byte(fmt.Sprintf("Referenced body: open %s (%d bytes, sha256 %s). Read it in bounded views; it is not inlined.\n", openPath, len(section.raw), digestBytes(section.raw)))
			}
			var rendered bytes.Buffer
			rendered.WriteString("# " + packetHeading(section.slot) + "\n\n")
			rendered.Write(body)
			if rendered.Bytes()[rendered.Len()-1] != '\n' {
				rendered.WriteByte('\n')
			}
			rendered.WriteByte('\n')
			return rendered.Bytes()
		}
		var smallest bytes.Buffer
		for _, section := range sections {
			smallest.Write(renderSection(section, section.ref))
		}
		inlineContinuation := renderSection(sections[len(sections)-1], false)
		referencedContinuation := renderSection(sections[len(sections)-1], true)
		if len(inlineContinuation)-len(referencedContinuation) > 0 {
			t.Fatal("tiny continuation unexpectedly has a positive reference saving")
		}
		overhead := smallest.Len() - len(continuationBody)
		wantDetail := fmt.Sprintf("role packet exceeds dispatch.max-inline-input-kb: %d bytes > 1024 bytes; fixed/reference overhead %d bytes", smallest.Len(), overhead)

		assertRefusal := func(t *testing.T, p ComposeRolePacketParams) {
			t.Helper()
			_, err := ComposeRolePacket(p)
			var refusal *CompositionRefusal
			if !errors.As(err, &refusal) || refusal.Code != "REFUSED-INLINE-INPUT-LIMIT" || refusal.Source != "dispatch.max-inline-input-kb" {
				t.Fatalf("impossible fit returned %T %v", err, err)
			}
			if refusal.Detail != wantDetail {
				t.Fatalf("refusal detail = %q, want %q", refusal.Detail, wantDetail)
			}
			if strings.Contains(refusal.Detail, "pass a file reference") {
				t.Fatalf("composition refusal carries obsolete advice: %q", refusal.Detail)
			}
		}
		assertRefusal(t, params)
		for _, path := range []string{params.Output, params.CompositionOutput, params.StageDir, filepath.Join(params.StageDir, "task-direction.md"), filepath.Join(params.StageDir, "prior-return.md")} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("impossible fit published %s", path)
			}
		}

		preexisting := params
		preexisting.Output = filepath.Join(root, "preexisting-prompt.md")
		preexisting.CompositionOutput = filepath.Join(root, "preexisting-composition.json")
		preexisting.StageDir = filepath.Join(root, "record-locks", "preexisting")
		if err := os.MkdirAll(preexisting.StageDir, 0o755); err != nil {
			t.Fatal(err)
		}
		before := map[string][]byte{
			preexisting.Output:                                       []byte("original packet\n"),
			preexisting.CompositionOutput:                            []byte("original composition\n"),
			filepath.Join(preexisting.StageDir, "task-direction.md"): []byte("original task stage\n"),
			filepath.Join(preexisting.StageDir, "prior-return.md"):   []byte("original return stage\n"),
		}
		for path, content := range before {
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		assertRefusal(t, preexisting)
		for path, content := range before {
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, content) {
				t.Fatalf("refusal changed pre-existing %s: %v", path, err)
			}
		}
	})

	t.Run("reference-paths-are-validated-when-small-body-must-stage", func(t *testing.T) {
		root := packetFitRoot(t)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=8\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for index, path := range []string{"scripts/agents/roles/verifier.md", "skills/verify/SKILL.md", "scripts/agents/schemas/verifier.schema.json"} {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), bytes.Repeat([]byte{byte('a' + index)}, 2*1024), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		brief := filepath.Join(root, "small-needs-stage.md")
		if err := os.WriteFile(brief, bytes.Repeat([]byte("b"), 4*1024), 0o644); err != nil {
			t.Fatal(err)
		}
		validStage := filepath.Join(root, "record-locks", "valid-stage")
		validReference := filepath.Join(root, "artifacts", "small", "rounds", "1", "staged")
		outside := filepath.Join(t.TempDir(), "outside")
		escapeTarget := t.TempDir()
		escapeStage := filepath.Join(root, "escape-stage")
		if err := os.Symlink(escapeTarget, escapeStage); err != nil {
			t.Skipf("cannot create escaping directory symlink: %v", err)
		}
		cases := []struct{ name, stage, reference string }{
			{name: "missing-stage", stage: "", reference: validReference},
			{name: "missing-reference", stage: validStage, reference: ""},
			{name: "outside", stage: outside, reference: validReference},
			{name: "root-itself", stage: validStage, reference: root},
			{name: "escaping-symlink", stage: escapeStage, reference: validReference},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				params := ComposeRolePacketParams{
					Root: root, Role: "verifier", Brief: brief, JobID: "bad-path-" + testCase.name, Runtime: "fake", Model: "fake-model",
					ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
					Output: filepath.Join(root, testCase.name+"-prompt.md"), CompositionOutput: filepath.Join(root, testCase.name+"-composition.json"),
					StageDir: testCase.stage, ReferenceDir: testCase.reference,
				}
				_, err := ComposeRolePacket(params)
				var refusal *CompositionRefusal
				if !errors.As(err, &refusal) || refusal.Code != "REFUSED-REFERENCE-PATH" {
					t.Fatalf("bad reference path returned %T %v", err, err)
				}
				for _, path := range []string{params.Output, params.CompositionOutput} {
					if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
						t.Fatalf("bad reference path published %s", path)
					}
				}
				if params.StageDir != "" {
					if _, statErr := os.Stat(filepath.Join(resolvePath(params.StageDir), "task-direction.md")); !os.IsNotExist(statErr) {
						t.Fatalf("bad reference path staged task direction through %s", params.StageDir)
					}
				}
			})
		}
	})

	t.Run("late-validation-precedes-staging", func(t *testing.T) {
		root := packetFitRoot(t)
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("dispatch.max-inline-input-kb=64\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		brief := filepath.Join(root, "late-brief.md")
		if err := os.WriteFile(brief, bytes.Repeat([]byte("b"), 40*1024), 0o644); err != nil {
			t.Fatal(err)
		}
		invalid := filepath.Join(root, "invalid-continuation.md")
		if err := os.WriteFile(invalid, []byte{0xff}, 0o644); err != nil {
			t.Fatal(err)
		}
		params := ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: "late-validation", Runtime: "fake", Model: "fake-model",
			ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: filepath.Join(root, "late-prompt.md"), CompositionOutput: filepath.Join(root, "late-composition.json"),
			StageDir: filepath.Join(root, "record-locks", "late"), ReferenceDir: filepath.Join(root, "artifacts", "late", "rounds", "1", "staged"),
			Continuations: []CompositionContinuation{{Slot: "prior-return", Path: invalid}},
		}
		_, err := ComposeRolePacket(params)
		var refusal *CompositionRefusal
		if !errors.As(err, &refusal) || refusal.Code != "REFUSED-CONTEXT-SOURCE" {
			t.Fatalf("late validation returned %T %v", err, err)
		}
		for _, path := range []string{params.Output, params.CompositionOutput, params.StageDir, filepath.Join(params.StageDir, "task-direction.md")} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("late validation published %s", path)
			}
		}
	})
}

func TestComposeRolePacketFollowsClosedRecipeAndRecordsEveryRange(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
		ExtraSources: []string{"skills/verify/SKILL.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(packet), "# Task Direction\n\nDo the focused task.\n") {
		t.Fatalf("packet does not begin with the exact task direction:\n%s", packet)
	}
	wantSlots := []string{"task-direction", "role-instructions", "required-skill", "response-contract", "tool-names", "generated-runtime-notice"}
	if len(record.Sources) != len(wantSlots) {
		t.Fatalf("source count = %d, want %d", len(record.Sources), len(wantSlots))
	}
	for index, source := range record.Sources {
		if source.Slot != wantSlots[index] {
			t.Fatalf("source %d slot = %s, want %s", index, source.Slot, wantSlots[index])
		}
		if source.StartByte < 0 || source.EndByte > len(packet) || source.StartByte >= source.EndByte {
			t.Fatalf("source %s has invalid range %d:%d", source.Slot, source.StartByte, source.EndByte)
		}
		if digestBytes(packet[source.StartByte:source.EndByte]) != source.DeliveredDigest {
			t.Fatalf("source %s delivered digest does not match its packet range", source.Slot)
		}
	}
	if digestBytes(packet) != record.PacketDigest || record.ContextProof.Classification != "advisory" ||
		record.ContextProof.ProofState != "no-leak-not-proven" || record.ContextProof.ReasonCode != "BROAD-READ-RUNTIME" {
		t.Fatalf("packet digest or bootstrap-honest classification is wrong: %+v", record)
	}
	if record.MachineSlot.Outcome != "DEFERRED" || record.MachineSlot.OwnerGoal != "machine-concurrency-governor" {
		t.Fatalf("machine governor seam was not retained: %+v", record.MachineSlot)
	}
	if record.ToolSurface.Policy != "read-only" || record.ToolSurface.NameState != "exact" || len(record.ToolSurface.Names) != 0 {
		t.Fatalf("fake runtime tool surface is not exact: %+v", record.ToolSurface)
	}
	if record.DestructiveReach != HazardMechanical || record.ConfigurationObligations.BuilderEffortTier != "ordinary" {
		t.Fatalf("hazard configuration was not recorded: %+v", record.ConfigurationObligations)
	}
	stored, err := os.ReadFile(composition)
	if err != nil || !json.Valid(stored) {
		t.Fatalf("composition record is not stored JSON: %v", err)
	}
}

func TestComposeRolePacketReferencesABriefOverMaxDirectiveBytes(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	briefPath := filepath.Join(temp, "brief.md")
	brief := bytes.Repeat([]byte("b"), 40*1024)
	if err := os.WriteFile(briefPath, brief, 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	stageDir := compositionTemporaryStageDir(t)
	referenceDir := compositionStageDir(t)
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: briefPath, JobID: "large-brief", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
		Output: prompt, CompositionOutput: composition, StageDir: stageDir, ReferenceDir: referenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(record.References) != 1 {
		t.Fatalf("reference count = %d, want 1", len(record.References))
	}
	reference := record.References[0]
	if reference.Slot != "task-direction" || reference.Purpose != "brief" || reference.Lifetime != "staged" ||
		reference.Bytes != len(brief) || reference.Digest != digestBytes(brief) {
		t.Fatalf("unexpected task-direction reference: %+v", reference)
	}
	if !filepath.IsAbs(reference.OpenPath) || filepath.Join(root, filepath.FromSlash(reference.Path)) != reference.OpenPath {
		t.Fatalf("reference paths do not bind the control-root copy: %+v", reference)
	}
	if reference.OpenPath != filepath.Join(referenceDir, "task-direction.md") {
		t.Fatalf("reference names %s, want final round path under %s", reference.OpenPath, referenceDir)
	}
	staged, err := os.ReadFile(filepath.Join(stageDir, "task-direction.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, brief) {
		t.Fatal("staged task direction differs from the brief")
	}
	packet, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	stanza := fmt.Sprintf("Referenced body: open %s (%d bytes, sha256 %s). Read it in bounded views; it is not inlined.\n", reference.OpenPath, len(brief), digestBytes(brief))
	if !bytes.Contains(packet, []byte(stanza)) || bytes.Contains(packet, brief) {
		t.Fatal("packet did not replace the large brief with its reference stanza")
	}
	if record.Sources[0].SourceDigest != digestBytes(brief) || record.Sources[0].SourceBytes != len(brief) {
		t.Fatalf("task-direction source lost the referenced body's identity: %+v", record.Sources[0])
	}
	previousEnd := 0
	for _, source := range record.Sources {
		if source.StartByte != previousEnd || source.EndByte > len(packet) || digestBytes(packet[source.StartByte:source.EndByte]) != source.DeliveredDigest {
			t.Fatalf("source %s does not tile its packet range", source.Slot)
		}
		previousEnd = source.EndByte
	}
	if previousEnd != len(packet) {
		t.Fatalf("source ranges end at %d, packet has %d bytes", previousEnd, len(packet))
	}
	if _, err := readCompositionForJob(composition, "large-brief", "verifier", "fake", "fake-model", "", HazardMechanical, 0, 1, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("composition with a reference was refused: %v", err)
	}

	exact := bytes.Repeat([]byte("e"), MaxDirectiveBytes)
	if err := os.WriteFile(briefPath, exact, 0o644); err != nil {
		t.Fatal(err)
	}
	composeExact := func(name, stageDir string) (CompositionRecord, []byte) {
		t.Helper()
		promptPath := filepath.Join(temp, name+".md")
		recordPath := filepath.Join(temp, name+".json")
		record, err := ComposeRolePacket(ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: briefPath, JobID: "exact-brief", Runtime: "fake",
			Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: promptPath, CompositionOutput: recordPath, StageDir: stageDir, ReferenceDir: compositionStageDir(t),
		})
		if err != nil {
			t.Fatal(err)
		}
		packet, err := os.ReadFile(promptPath)
		if err != nil {
			t.Fatal(err)
		}
		return record, packet
	}
	withoutStage, packetWithoutStage := composeExact("exact-without-stage", "")
	withStage, packetWithStage := composeExact("exact-with-stage", compositionTemporaryStageDir(t))
	if len(packetWithoutStage) > 65536 || len(packetWithStage) > 65536 {
		t.Fatalf("exact-bound verifier packets have %d and %d bytes, cap is 65536", len(packetWithoutStage), len(packetWithStage))
	}
	if !bytes.Equal(packetWithoutStage, packetWithStage) || len(withoutStage.References) != 0 || len(withStage.References) != 0 {
		t.Fatal("a body at MaxDirectiveBytes did not stay on the unchanged inline path")
	}
}

func TestComposeRolePacketRefusesAReferenceOutsideTheRoot(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, bytes.Repeat([]byte("b"), 40*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	compose := func(name, stageDir, referenceDir string) *CompositionRefusal {
		t.Helper()
		prompt := filepath.Join(temp, name+".md")
		composition := filepath.Join(temp, name+".json")
		_, err := ComposeRolePacket(ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: name, Runtime: "fake",
			Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: prompt, CompositionOutput: composition, StageDir: stageDir, ReferenceDir: referenceDir,
		})
		var refusal *CompositionRefusal
		if !errors.As(err, &refusal) || refusal.Code != "REFUSED-REFERENCE-PATH" {
			t.Fatalf("composition result = %T %v", err, err)
		}
		for _, path := range []string{prompt, composition} {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("refused composition wrote %s", path)
			}
		}
		return refusal
	}
	outside := filepath.Join(t.TempDir(), "staged")
	finalDir := compositionStageDir(t)
	if refusal := compose("outside", outside, finalDir); refusal.Source != outside {
		t.Fatalf("outside refusal source = %q, want %q", refusal.Source, outside)
	}
	symlinkStage := compositionStageDir(t)
	if err := os.MkdirAll(filepath.Dir(symlinkStage), 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkTarget := t.TempDir()
	if err := os.Symlink(symlinkTarget, symlinkStage); err != nil {
		t.Skipf("cannot create stage-directory symlink: %v", err)
	}
	if refusal := compose("symlink", symlinkStage, finalDir); refusal.Source != symlinkStage {
		t.Fatalf("symlink refusal source = %q, want %q", refusal.Source, symlinkStage)
	}
	if refusal := compose("missing-stage", "", finalDir); refusal.Source != "task-direction" {
		t.Fatalf("missing-stage refusal source = %q, want task-direction", refusal.Source)
	}
	validStage := compositionTemporaryStageDir(t)
	outsideFinal := filepath.Join(t.TempDir(), "final")
	if refusal := compose("outside-final", validStage, outsideFinal); refusal.Source != outsideFinal {
		t.Fatalf("outside final refusal source = %q, want %q", refusal.Source, outsideFinal)
	}
}

func TestComposeRolePacketRefusesCallerSourceOutsideRecipeBeforeWriting(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	_, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
		ExtraSources: []string{"plans/role-context-composition-design.md"},
	})
	var refusal *CompositionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "REFUSED-CONTEXT-SOURCE" {
		t.Fatalf("forbidden source result = %T %v", err, err)
	}
	for _, path := range []string{prompt, composition} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("refused composition wrote %s", path)
		}
	}
}

func TestJobRecordRejectsExpandedOrDishonestComposition(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical, Output: prompt, CompositionOutput: composition,
	})
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(prompt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readCompositionForJob(composition, "verify-a", "verifier", "fake", "fake-model", "", HazardMechanical, 0, 1, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("generated composition did not validate: %v", err)
	}

	var expanded map[string]any
	stored, err := os.ReadFile(composition)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored, &expanded); err != nil {
		t.Fatal(err)
	}
	expanded["undeclaredContext"] = "must not enter the job record"
	tampered, err := json.Marshal(expanded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(composition, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCompositionForJob(composition, "verify-a", "verifier", "fake", "fake-model", "", HazardMechanical, 0, 1, int64(len(packet)), record.PacketDigest); err == nil || !strings.Contains(err.Error(), "expanded") {
		t.Fatalf("expanded composition result = %v", err)
	}
	delete(expanded, "undeclaredContext")
	firstSource := expanded["sources"].([]any)[0].(map[string]any)
	relativeReference := filepath.ToSlash(filepath.Join("artifacts", "agents", "test-dishonest", "staged", "task-direction.md"))
	expanded["references"] = []any{map[string]any{
		"slot": "task-direction", "purpose": "brief", "path": relativeReference,
		"openPath": filepath.Join(root, filepath.FromSlash(relativeReference)),
		"digest":   strings.Repeat("0", 64), "bytes": firstSource["sourceBytes"], "lifetime": "staged",
	}}
	writeRecord(composition, expanded)
	if _, err := readCompositionForJob(composition, "verify-a", "verifier", "fake", "fake-model", "", HazardMechanical, 0, 1, int64(len(packet)), record.PacketDigest); err == nil || !strings.Contains(err.Error(), "unbound") {
		t.Fatalf("dishonest reference result = %v", err)
	}
}

func TestTierThreeMechanicalCompositionCarriesEffectiveFollowUpObligations(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Continue the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	params := ComposeRolePacketParams{
		Root: root, Role: "implementer", Brief: brief, JobID: "mechanical-tier-3-r2", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-write", Round: 2, DestructiveReach: HazardMechanical, GoalTier: 3,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
	}
	record, err := ComposeRolePacket(params)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(params.Output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readCompositionForJob(params.CompositionOutput, params.JobID, params.Role, params.Runtime, params.Model, "", HazardMechanical, 3, 2, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("tier-3 effective obligations were refused: %v", err)
	}

	stored := readJSONFile(t, params.CompositionOutput)
	stored["configurationObligations"] = requiredConfigurationByHazard[HazardMechanical]
	writeRecord(params.CompositionOutput, stored)
	if _, err := readCompositionForJob(params.CompositionOutput, params.JobID, params.Role, params.Runtime, params.Model, "", HazardMechanical, 3, 2, int64(len(packet)), record.PacketDigest); err == nil || !strings.Contains(err.Error(), "does not carry the hazard configuration obligations") {
		t.Fatalf("bare mechanical class floor on a tier-3 follow-up = %v", err)
	}
}

func TestRolePacketTableCoversEveryDispatchableRole(t *testing.T) {
	root := compositionRepoRoot(t)
	tableBytes, err := os.ReadFile(filepath.Join(root, rolePacketTablePath))
	if err != nil {
		t.Fatal(err)
	}
	var table rolePacketTable
	if err := json.Unmarshal(tableBytes, &table); err != nil {
		t.Fatal(err)
	}
	entries, err := filepath.Glob(filepath.Join(root, "scripts", "agents", "roles", "*.requirements.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(entries))
	for _, entry := range entries {
		want = append(want, strings.TrimSuffix(filepath.Base(entry), ".requirements.json"))
	}
	sort.Strings(want)
	got := make([]string, 0, len(table.Roles))
	for role := range table.Roles {
		got = append(got, role)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("role packet table = %v, dispatchable roles = %v", got, want)
	}
	if len(table.DestructiveReach) != 3 {
		t.Fatalf("destructiveReach class count = %d, want 3", len(table.DestructiveReach))
	}
}

func TestHazardConfigurationRefusesAWeakenedMinimum(t *testing.T) {
	root := compositionRepoRoot(t)
	destructive, err := ResolveHazardConfiguration(root, HazardDestructiveReach, 0)
	if err != nil {
		t.Fatal(err)
	}
	if destructive.BuilderEffortTier != "maximal" || destructive.BuilderReasoningEffort != "xhigh" ||
		!destructive.IndependentCritiqueRequired || destructive.IndependentCritiqueEffortTier != "maximal" ||
		destructive.IndependentCritiqueReasoningEffort != "xhigh" || !destructive.LiveProofRequired {
		t.Fatalf("destructive-reach minimum is incomplete: %+v", destructive)
	}

	tableBytes, err := os.ReadFile(filepath.Join(root, rolePacketTablePath))
	if err != nil {
		t.Fatal(err)
	}
	var table map[string]any
	if err := json.Unmarshal(tableBytes, &table); err != nil {
		t.Fatal(err)
	}
	classes := table["destructiveReach"].(map[string]any)
	weakened := classes[string(HazardDestructiveReach)].(map[string]any)
	weakened["builderReasoningEffort"] = "medium"
	encoded, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	tamperedRoot := t.TempDir()
	path := filepath.Join(tamperedRoot, rolePacketTablePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveHazardConfiguration(tamperedRoot, HazardDestructiveReach, 0); err == nil || !strings.Contains(err.Error(), "does not match its required configuration") {
		t.Fatalf("weakened destructive-reach configuration result = %v", err)
	}

	t.Run("tampered independent critique tier rule", func(t *testing.T) {
		var tampered map[string]any
		if err := json.Unmarshal(tableBytes, &tampered); err != nil {
			t.Fatal(err)
		}
		tampered["independentCritiqueByTier"].(map[string]any)["3"] = false
		encoded, err := json.Marshal(tampered)
		if err != nil {
			t.Fatal(err)
		}
		tamperedRoot := t.TempDir()
		path := filepath.Join(tamperedRoot, rolePacketTablePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = ResolveHazardConfiguration(tamperedRoot, HazardMechanical, 3)
		if err == nil || !strings.Contains(err.Error(), "role packet table independentCritiqueByTier does not match its required rule") {
			t.Fatalf("tampered independentCritiqueByTier result = %v", err)
		}
	})

	t.Run("missing independent critique tier rule", func(t *testing.T) {
		var missing map[string]any
		if err := json.Unmarshal(tableBytes, &missing); err != nil {
			t.Fatal(err)
		}
		delete(missing, "independentCritiqueByTier")
		encoded, err := json.Marshal(missing)
		if err != nil {
			t.Fatal(err)
		}
		missingRoot := t.TempDir()
		path := filepath.Join(missingRoot, rolePacketTablePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveHazardConfiguration(missingRoot, HazardMechanical, 3); err == nil ||
			!strings.Contains(err.Error(), "independentCritiqueByTier, and at least one role") {
			t.Fatalf("role packet table without independentCritiqueByTier was not refused by the reader: %v", err)
		}
	})

	if _, err := EffectiveObligations(HazardMechanical, 4); err == nil || err.Error() != "goal tier must be 1, 2, or 3" {
		t.Fatalf("invalid goal tier result = %v", err)
	}
}

func TestHazardConfigurationRefusesRuntimeWithoutExecutableMaximum(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(temp, "prompt.md")
	composition := filepath.Join(temp, "composition.json")
	_, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "verify-a", Runtime: "claude",
		Model: "claude-non-maximal", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardDestructiveReach,
		Output: prompt, CompositionOutput: composition,
	})
	var refusal *CompositionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "REFUSED-HAZARD-CONFIGURATION" || !strings.Contains(refusal.Detail, "no executable maximal-effort mapping") {
		t.Fatalf("unsupported runtime hazard result = %T %v", err, err)
	}
	for _, path := range []string{prompt, composition} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("refused hazard composition wrote %s", path)
		}
	}
}

func TestHazardConfigurationAcceptsConfiguredMaximalModel(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Working Mode: review\nReview the focused change.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "code-critic", Brief: brief, JobID: "claude-maximal", Runtime: "claude",
		Model: "claude-fable-5-1", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardDesignBearing,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Runtime != "claude" || record.Model != "claude-fable-5-1" ||
		record.ConfigurationObligations.BuilderEffortTier != "maximal" ||
		record.ConfigurationObligations.BuilderReasoningEffort != "xhigh" {
		t.Fatalf("configured maximal-model composition = %+v", record)
	}
}

func TestHazardConfigurationUsesResolvedLocalMaximalModelMapping(t *testing.T) {
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	if err := os.WriteFile(conf, []byte("runtime.claude.maximal-models=claude-base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf+".local", []byte("runtime.claude.maximal-models=claude-fable-5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuntimeHazardConfiguration(root, "claude", "claude-fable-5", HazardDesignBearing); err != nil {
		t.Fatalf("resolved local maximal model refused: %v", err)
	}
	if err := ValidateRuntimeHazardConfiguration(root, "claude", "claude-base", HazardDesignBearing); err == nil || !strings.Contains(err.Error(), "no executable maximal-effort mapping") {
		t.Fatalf("shadowed committed maximal model result = %v", err)
	}
}

func TestFMA_R2_ClaudeGateFixtureOmitted(t *testing.T) {
	sourceRoot := compositionRepoRoot(t)
	root := t.TempDir()
	for _, relative := range []string{
		"scripts/agents/role-packets.json",
		"scripts/agents/roles/verifier.md",
		"skills/verify/SKILL.md",
		"scripts/agents/schemas/verifier.schema.json",
	} {
		content, err := os.ReadFile(filepath.Join(sourceRoot, relative))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	conf := filepath.Join(root, "metasystem.conf")
	withAlias := "metasystem.runtimes=claude\n" +
		"runtime.claude.maximal-models=claude-fable-5-1\n" +
		"runtime.claude.model-alias.claude-fable-5=claude-fable-5-1\n" +
		"role.verifier.runtime=claude\nrole.verifier.model.claude=claude-fable-5\n"
	if err := os.WriteFile(conf, []byte(withAlias), 0o644); err != nil {
		t.Fatal(err)
	}
	resolution, err := ResolveRoster(RosterParams{ConfPath: conf, Role: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Model != "claude-fable-5-1" || resolution.AliasedFrom != "claude-fable-5" {
		t.Fatalf("alias roster resolution = %+v", resolution)
	}
	if err := ValidateRuntimeHazardConfiguration(root, resolution.Runtime, resolution.Model, HazardDesignBearing); err != nil {
		t.Fatalf("resolved target failed the Claude maximal gate: %v", err)
	}

	withoutAlias := strings.Replace(withAlias, "runtime.claude.model-alias.claude-fable-5=claude-fable-5-1\n", "", 1)
	if err := os.WriteFile(conf, []byte(withoutAlias), 0o644); err != nil {
		t.Fatal(err)
	}
	resolution, err = ResolveRoster(RosterParams{ConfPath: conf, Role: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(root, "brief.md")
	if err := os.WriteFile(brief, []byte("Verify the focused behavior.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ComposeRolePacket(ComposeRolePacketParams{
		Root: root, Role: "verifier", Brief: brief, JobID: "alias-gate", Runtime: resolution.Runtime,
		Model: resolution.Model, ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardDesignBearing,
		Output: filepath.Join(root, "prompt.md"), CompositionOutput: filepath.Join(root, "composition.json"),
	})
	var refusal *CompositionRefusal
	if !errors.As(err, &refusal) || refusal.Code != "REFUSED-HAZARD-CONFIGURATION" {
		t.Fatalf("unalias roster hazard result = %T %v", err, err)
	}
}

func closeReadyHazardChain(t *testing.T, class HazardClass) (repo, evidence, job string) {
	t.Helper()
	return closeReadyHazardChainAtTier(t, class, nil)
}

func closeReadyHazardChainAtTier(t *testing.T, class HazardClass, tier any) (repo, evidence, job string) {
	t.Helper()
	repo, evidence, job = mirrorFixture(t)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	goalTier := uint8(0)
	if tier != nil {
		var ok bool
		goalTier, ok = tier.(uint8)
		if !ok {
			t.Fatalf("test goal tier has type %T, want uint8 or nil", tier)
		}
		record["goalId"] = "goal-a"
		record["goalRevision"] = 1
	}
	configuration, err := EffectiveObligations(class, goalTier)
	if err != nil {
		t.Fatal(err)
	}
	record["destructiveReach"] = class
	record["goalTier"] = tier
	record["configurationObligations"] = configuration
	record["dispatchMode"] = DispatchModeFresh
	record["resumedSessionId"] = nil
	record["sessionId"] = "builder-session"
	record["endedAt"] = "2026-08-30T10:00:00Z"
	writeRecord(path, record)
	refreshHazardMirror(t, repo, evidence, job)
	return repo, evidence, job
}

func refreshHazardMirror(t *testing.T, repo, evidence, job string) {
	t.Helper()
	result := filepath.Join(t.TempDir(), "mirror.json")
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
	mirrored := readJSONFile(t, result)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["mirror"] = map[string]any{"path": asString(mirrored["path"]), "manifest": mirrored["manifest"]}
	writeRecord(path, record)
	if err := Mirror(repo, repo, evidence, job, job, result); err != nil {
		t.Fatal(err)
	}
}

func writeHazardEvidenceJob(t *testing.T, repo, job string, class HazardClass, record map[string]any) {
	t.Helper()
	stampHazardEvidenceAdmission(t, repo, job, class, record)
	writeHandwrittenHazardEvidenceJob(t, repo, job, record)
}

func writeHandwrittenHazardEvidenceJob(t *testing.T, repo, job string, record map[string]any) {
	t.Helper()
	record["jobId"] = job
	record["status"] = "completed"
	if _, present := record["endedAt"]; !present {
		record["endedAt"] = "2026-08-30T10:01:00Z"
	}
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), job+".json", record)
}

func stampHazardEvidenceAdmission(t *testing.T, repo, job string, class HazardClass, record map[string]any) {
	t.Helper()
	digest := strings.Repeat("a", 64)
	runtimeName := asString(record["runtime"])
	if runtimeName == "" {
		runtimeName = "fake"
	}
	model := asString(record["requestedModel"])
	if model == "" {
		model = "fake-model"
	}
	request := CanonicalLaunchRequest{
		SessionKey: "session-" + job, DispatchMode: DispatchModeFresh, ResumedSessionID: "",
		Runtime: runtimeName, CanonicalModelKey: model, Role: asString(record["role"]),
		LaunchMode: LaunchModeSharedCheckout, PermissionEnvelopeDigest: digest,
		ProductRoots: []string{repo}, CapMinutes: 30, InputHash: strings.Repeat("b", 64),
		DestructiveReach: class,
	}
	fingerprint, err := LaunchFingerprintV2(request)
	if err != nil {
		t.Fatal(err)
	}
	tag := "metasystem-job-" + job + "-evidence"
	record["operationId"] = job
	record["round"] = 1
	if _, present := record["parentJob"]; !present {
		record["parentJob"] = nil
	}
	record["proofLevel"] = "proven"
	record["sessionKey"] = request.SessionKey
	record["dispatchMode"] = request.DispatchMode
	record["resumedSessionId"] = request.ResumedSessionID
	record["runtime"] = request.Runtime
	record["canonicalModelKey"] = request.CanonicalModelKey
	record["requestedModel"] = model
	record["launchMode"] = request.LaunchMode
	record["permissionEnvelopeDigest"] = request.PermissionEnvelopeDigest
	record["productRoots"] = request.ProductRoots
	record["capMin"] = request.CapMinutes
	record["capRequest"] = map[string]any{"minutes": request.CapMinutes}
	record["inputHash"] = request.InputHash
	record["goalId"] = nil
	record["goalRevision"] = nil
	record["destructiveReach"] = request.DestructiveReach
	record["fingerprintVersion"] = fingerprint.Version
	record["fingerprint"] = fingerprint.Digest
	record["instanceTag"] = tag
	record["launchCapability"] = map[string]any{
		"digest": strings.Repeat("c", 64), "jobId": job, "operationId": job,
		"instanceTag": tag, "adapterVerb": "dispatch", "status": "consumed",
		"mintedAt": "2026-08-30T09:58:00Z", "consumedAt": "2026-08-30T09:59:00Z",
		"supervisor": exactIdentityFields(nativeTestExact(7001, 3).Ref()),
	}
	if _, present := record["configurationObligations"]; !present {
		record["configurationObligations"] = requiredConfigurationByHazard[class]
	}
	if _, present := record["reasoningEffort"]; !present {
		record["reasoningEffort"] = requiredConfigurationByHazard[class].BuilderReasoningEffort
	}
}

func wantHazardClosureRefusal(t *testing.T, err error, reason string) {
	t.Helper()
	var refusal *OpError
	if !errors.As(err, &refusal) || refusal.Code != 9 || refusal.Reason != reason {
		t.Fatalf("closure refusal = %T %v, want typed %s", err, err, reason)
	}
}

func closeReadyCriticHazardChain(t *testing.T, class HazardClass, role any) (repo, evidence, job string) {
	t.Helper()
	repo, evidence, job = mirrorFixture(t)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["role"] = role
	record["destructiveReach"] = class
	record["configurationObligations"] = requiredConfigurationByHazard[class]
	record["dispatchMode"] = DispatchModeFresh
	record["resumedSessionId"] = nil
	record["sessionId"] = "critic-chain-session"
	record["endedAt"] = "2026-08-30T10:00:00Z"
	record[findingRegisterField] = []any{}
	record[findingRegisterRoundField] = 1
	writeRecord(path, record)
	refreshHazardMirror(t, repo, evidence, job)
	return repo, evidence, job
}

func TestCriticOnlyHazardClosureDoesNotRecurse(t *testing.T) {
	for _, row := range []struct {
		role  string
		class HazardClass
	}{
		{role: "code-critic", class: HazardDesignBearing},
		{role: "design-critic", class: HazardDesignBearing},
		{role: "warden", class: HazardDestructiveReach},
	} {
		t.Run(row.role, func(t *testing.T) {
			repo, _, job := closeReadyCriticHazardChain(t, row.class, row.role)
			if err := CloseCheck(repo, job); err != nil {
				t.Fatalf("review-only %s chain required recursive builder evidence: %v", row.role, err)
			}
		})
	}
}

func TestCriticOnlyHazardClosurePreservesOtherCloseChecks(t *testing.T) {
	t.Run("malformed hazard class", func(t *testing.T) {
		repo, evidence, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["destructiveReach"] = "DESIGN-BEARING "
		writeRecord(path, record)
		refreshHazardMirror(t, repo, evidence, job)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardRecordClosureRefusal)
	})

	for _, row := range []struct {
		name   string
		role   any
		absent bool
	}{
		{name: "absent", absent: true},
		{name: "non-string", role: 7},
		{name: "empty", role: ""},
		{name: "trailing-space", role: "code-critic "},
		{name: "unknown", role: "critic"},
	} {
		t.Run("role-"+row.name, func(t *testing.T) {
			repo, evidence, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
			path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
			record := readJSONFile(t, path)
			if row.absent {
				delete(record, "role")
			} else {
				record["role"] = row.role
			}
			writeRecord(path, record)
			refreshHazardMirror(t, repo, evidence, job)
			wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardCritiqueClosureRefusal)
		})
	}

	t.Run("open finding register", func(t *testing.T) {
		repo, evidence, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record[findingRegisterField] = encodeFindingRegister([]registerFinding{{
			FindingID: "HOST-T-OPEN", Critic: job, RigorClass: "bounded",
			FactsDigest: strings.Repeat("a", 64), Facts: registerFacts(),
			Artifact: "metasystem/internal/dispatch/hazard.go", Title: "open fixture finding",
			Status: "open", Evidence: "fixture evidence", EvidenceDigest: strings.Repeat("b", 64), Multiplicity: 1,
		}})
		writeRecord(path, record)
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "cannot close with unresolved finding HOST-T-OPEN") {
			t.Fatalf("open critic register closure = %v", err)
		}
	})

	t.Run("unfolded register", func(t *testing.T) {
		repo, evidence, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record[findingRegisterRoundField] = 0
		writeRecord(path, record)
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "register is folded through round 0 while terminal round 1 exists") {
			t.Fatalf("unfolded critic register closure = %v", err)
		}
	})

	t.Run("missing mirror", func(t *testing.T) {
		repo, _, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		delete(record, "mirror")
		writeRecord(path, record)
		if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "unmirrored") {
			t.Fatalf("missing critic mirror closure = %v", err)
		}
	})

	t.Run("stale mirror", func(t *testing.T) {
		repo, _, job := closeReadyCriticHazardChain(t, HazardDesignBearing, "code-critic")
		record := readJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs", job+".json"))
		mirror := record["mirror"].(map[string]any)
		if err := os.WriteFile(filepath.Join(asString(mirror["path"]), "manifest.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := CloseCheck(repo, job); err == nil || !strings.Contains(err.Error(), "manifest does not cover job record") {
			t.Fatalf("stale critic mirror closure = %v", err)
		}
	})
}

func TestMixedHazardChainRetainsBuilderDuties(t *testing.T) {
	repo, evidence, job := closeReadyHazardChain(t, HazardDesignBearing)
	child := job + "-r2"
	writeJSONFile(t, filepath.Join(repo, "artifacts", "agents", "jobs"), child+".json", map[string]any{
		"jobId": child, "round": 2, "parentJob": job, "status": "completed",
		"role": "code-critic", "sessionId": "member-critic-session",
		"endedAt": "2026-08-30T10:02:00Z", "destructiveReach": HazardDestructiveReach,
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		"capabilitySnapshot":       "artifacts/agents/capabilities/snap.json",
	})
	result := filepath.Join(t.TempDir(), "child-mirror.json")
	if err := Mirror(repo, repo, evidence, job, child, result); err != nil {
		t.Fatal(err)
	}
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardCritiqueClosureRefusal)

	writeHazardEvidenceJob(t, repo, "independent-mixed-critic", HazardDestructiveReach, map[string]any{
		"role": "code-critic", "reviews": job, "parentJob": nil,
		"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
		"sessionId": "independent-mixed-session", "reasoningEffort": "xhigh",
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
	})
	rootPath := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	root := readJSONFile(t, rootPath)
	root["independentCritiqueJobRef"] = "independent-mixed-critic"
	writeRecord(rootPath, root)
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardLiveProofClosureRefusal)
}

func TestHazardDutiesGateChainCompletion(t *testing.T) {
	t.Run("destructive reach requires critique and live proof", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")

		writeHazardEvidenceJob(t, repo, "critic-job", HazardDestructiveReach, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		})
		if err := StampClaimedReviewReference(repo, "critic-job"); err != nil {
			t.Fatalf("could not attach post-work critique evidence: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF")

		writeHazardEvidenceJob(t, repo, "live-proof-job", HazardDestructiveReach, map[string]any{
			"role": "verifier", "reviews": job,
		})
		if err := StampClaimedReviewReference(repo, "live-proof-job"); err != nil {
			t.Fatalf("could not attach post-work live proof evidence: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("complete destructive-reach evidence did not discharge the chain: %v", err)
		}
	})

	t.Run("mechanical at tier 3 requires a critique", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChainAtTier(t, HazardMechanical, uint8(3))
		wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardCritiqueClosureRefusal)

		writeHazardEvidenceJob(t, repo, "mechanical-tier-3-critic", HazardDesignBearing, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "mechanical-tier-3-critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		})
		if err := StampClaimedReviewReference(repo, "mechanical-tier-3-critic"); err != nil {
			t.Fatalf("could not attach tier-required critique: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("mechanical tier-3 chain with independent critique did not close: %v", err)
		}
	})

	t.Run("mechanical at tier 2 requires a critique", func(t *testing.T) {
		repo, _, job := closeReadyHazardChainAtTier(t, HazardMechanical, uint8(2))
		wantHazardClosureRefusal(t, CloseCheck(repo, job), hazardCritiqueClosureRefusal)
	})

	t.Run("mechanical at tier 1 closes without one", func(t *testing.T) {
		repo, _, job := closeReadyHazardChainAtTier(t, HazardMechanical, uint8(1))
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("mechanical tier-1 chain required hazard evidence: %v", err)
		}
	})

	t.Run("mechanical without a goal keeps the class floor", func(t *testing.T) {
		repo, _, job := closeReadyHazardChainAtTier(t, HazardMechanical, nil)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("goal-less mechanical chain required hazard evidence: %v", err)
		}
	})

	t.Run("mechanical record without a goalTier key keeps the class floor", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChainAtTier(t, HazardMechanical, nil)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		delete(record, "goalTier")
		writeRecord(path, record)
		refreshHazardMirror(t, repo, evidence, job)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("a record without the goalTier key required hazard evidence: %v", err)
		}
	})

	t.Run("mechanical at tier 3 refuses a critic dispatched at its own class by name", func(t *testing.T) {
		repo, evidence, job := closeReadyHazardChainAtTier(t, HazardMechanical, uint8(3))
		critic, err := EffectiveObligations(HazardMechanical, 3)
		if err != nil {
			t.Fatal(err)
		}
		writeHazardEvidenceJob(t, repo, "same-class-critic", HazardMechanical, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "critic-session", "reasoningEffort": "medium",
			"configurationObligations": critic,
		})
		if err := StampClaimedReviewReference(repo, "same-class-critic"); err != nil {
			t.Fatalf("could not attach the same-class critic: %v", err)
		}
		refreshHazardMirror(t, repo, evidence, job)
		err = CloseCheck(repo, job)
		wantHazardClosureRefusal(t, err, "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
		if !strings.Contains(err.Error(), "its builder rows are ordinary/medium and its reasoning effort medium, the critique requires maximal/xhigh; dispatch the critic at a class whose builder rows are the critique's (DESIGN-BEARING)") {
			t.Fatalf("the refusal does not name the field and the class: %v", err)
		}
	})

	t.Run("configured claude maximal model closes critique duty", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
		if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"), []byte("runtime.claude.maximal-models=claude-fable-5\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeHazardEvidenceJob(t, repo, "claude-critic", HazardDesignBearing, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"runtime": "claude", "requestedModel": "claude-fable-5",
			"sessionId": "claude-critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
		})
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "claude-critic"
		writeRecord(path, record)
		if err := CloseCheck(repo, job); err != nil {
			t.Fatalf("configured Claude maximal critic did not discharge the chain: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, "metasystem.conf"), []byte("runtime.claude.maximal-models=claude-other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	t.Run("dangling critique reference", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDestructiveReach)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "missing-critic"
		record["liveProofEvidenceRef"] = "missing-proof"
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	for _, invalid := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "critic resumed instead of fresh", mutate: func(record map[string]any) { record["parentJob"] = "older-critic" }},
		{name: "critic reused builder session", mutate: func(record map[string]any) { record["sessionId"] = "builder-session" }},
		{name: "critic below maximum effort", mutate: func(record map[string]any) {
			record["reasoningEffort"] = "medium"
			record["configurationObligations"] = requiredConfigurationByHazard[HazardMechanical]
		}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
			critic := map[string]any{
				"role": "code-critic", "reviews": job, "parentJob": nil,
				"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
				"sessionId": "critic-session", "reasoningEffort": "xhigh",
				"configurationObligations": requiredConfigurationByHazard[HazardDesignBearing],
			}
			invalid.mutate(critic)
			writeHazardEvidenceJob(t, repo, "critic-job", HazardDesignBearing, critic)
			path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
			record := readJSONFile(t, path)
			record["independentCritiqueJobRef"] = "critic-job"
			writeRecord(path, record)
			wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
		})
	}

	t.Run("critic must be distinct", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDesignBearing)
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = job
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-INDEPENDENT-CRITIQUE")
	})

	t.Run("dangling live-proof reference", func(t *testing.T) {
		repo, _, job := closeReadyHazardChain(t, HazardDestructiveReach)
		writeHazardEvidenceJob(t, repo, "critic-job", HazardDestructiveReach, map[string]any{
			"role": "code-critic", "reviews": job, "parentJob": nil,
			"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
			"sessionId": "critic-session", "reasoningEffort": "xhigh",
			"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		})
		path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
		record := readJSONFile(t, path)
		record["independentCritiqueJobRef"] = "critic-job"
		record["liveProofEvidenceRef"] = "missing-proof"
		writeRecord(path, record)
		wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF")
	})
}

func TestHazardEvidenceRequiresAdmissionProvenance(t *testing.T) {
	repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
	critic := map[string]any{
		"role": "code-critic", "reviews": job, "parentJob": nil,
		"dispatchMode": DispatchModeFresh, "resumedSessionId": nil,
		"sessionId": "critic-session", "reasoningEffort": "xhigh",
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
	}
	proof := map[string]any{"role": "verifier", "reviews": job}
	writeHandwrittenHazardEvidenceJob(t, repo, "critic-c", critic)
	writeHandwrittenHazardEvidenceJob(t, repo, "proof-c", proof)
	path := filepath.Join(repo, "artifacts", "agents", "jobs", job+".json")
	record := readJSONFile(t, path)
	record["independentCritiqueJobRef"] = "critic-c"
	record["liveProofEvidenceRef"] = "proof-c"
	writeRecord(path, record)
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "critic-c", HazardDestructiveReach, critic)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "proof-c", HazardDestructiveReach, proof)
	proofPath := filepath.Join(repo, "artifacts", "agents", "jobs", "proof-c.json")
	proof = readJSONFile(t, proofPath)
	proof["fingerprint"] = strings.Repeat("d", 64)
	writeRecord(proofPath, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")

	writeHazardEvidenceJob(t, repo, "proof-c", HazardDestructiveReach, proof)
	proof = readJSONFile(t, proofPath)
	capability := proof["launchCapability"].(map[string]any)
	delete(capability, "consumedAt")
	writeRecord(proofPath, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-EVIDENCE-PROVENANCE")
}

func TestHazardEvidenceMustCoverFinalWorkState(t *testing.T) {
	repo, evidence, job := closeReadyHazardChain(t, HazardDestructiveReach)
	agents := filepath.Join(repo, "artifacts", "agents")
	terminalJob := job + "-r2"
	writeJSONFile(t, filepath.Join(agents, "jobs"), terminalJob+".json", map[string]any{
		"jobId": terminalJob, "round": 2, "parentJob": job, "status": "completed",
		"role": "implementer", "endedAt": "2026-08-30T10:02:00Z",
		"destructiveReach":         HazardDestructiveReach,
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		"capabilitySnapshot":       "artifacts/agents/capabilities/snap.json",
	})
	if err := os.MkdirAll(filepath.Join(agents, job, "rounds", "2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, job, "rounds", "2", "diff.patch"), []byte("round two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(t.TempDir(), "mirror.json")
	if err := Mirror(repo, repo, evidence, job, terminalJob, result); err != nil {
		t.Fatal(err)
	}

	critic := map[string]any{
		"role": "code-critic", "reviews": job, "parentJob": nil,
		"sessionId": "critic-session", "reasoningEffort": "xhigh",
		"configurationObligations": requiredConfigurationByHazard[HazardDestructiveReach],
		"endedAt":                  "2026-08-30T10:01:00Z",
	}
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	proof := map[string]any{
		"role": "verifier", "reviews": terminalJob,
		"endedAt": "2026-08-30T10:03:00Z",
	}
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	rootPath := filepath.Join(agents, "jobs", job+".json")
	root := readJSONFile(t, rootPath)
	root["independentCritiqueJobRef"] = "critic-final"
	root["liveProofEvidenceRef"] = "proof-final"
	writeRecord(rootPath, root)
	refreshHazardMirror(t, repo, evidence, job)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-CRITIQUE-STALE")

	critic["reviews"] = terminalJob
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-CRITIQUE-STALE")

	critic["endedAt"] = "2026-08-30T10:02:00Z"
	writeHazardEvidenceJob(t, repo, "critic-final", HazardDestructiveReach, critic)
	if err := CloseCheck(repo, job); err != nil {
		t.Fatalf("critique and live proof of the terminal work round did not discharge the chain: %v", err)
	}

	proof["endedAt"] = "2026-08-30T10:01:00Z"
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF-STALE")

	proof["reviews"] = job
	proof["endedAt"] = "2026-08-30T10:03:00Z"
	writeHazardEvidenceJob(t, repo, "proof-final", HazardDestructiveReach, proof)
	wantHazardClosureRefusal(t, CloseCheck(repo, job), "REFUSED-R22-M1-RULING-O-LIVE-PROOF-STALE")
}

func TestDefaultOperationIdentityAndFingerprintBindGoalRevision(t *testing.T) {
	digest := strings.Repeat("a", 64)
	one, err := DefaultOperationID("goal-a", 7, DispatchModeFresh, "verifier", digest, "")
	if err != nil {
		t.Fatal(err)
	}
	two, err := DefaultOperationID("goal-a", 8, DispatchModeFresh, "verifier", digest, "")
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("goal revision change did not mint a distinct operation id")
	}
	followOne, err := DefaultOperationID("goal-a", 7, DispatchModeFollowUp, "verifier", digest, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	followTwo, err := DefaultOperationID("goal-a", 8, DispatchModeFollowUp, "verifier", digest, "chain-a")
	if err != nil {
		t.Fatal(err)
	}
	if followOne == followTwo || followOne == one {
		t.Fatal("follow-up operation identity did not bind mode and goal revision")
	}
	followOtherChain, err := DefaultOperationID("goal-a", 7, DispatchModeFollowUp, "verifier", digest, "chain-b")
	if err != nil {
		t.Fatal(err)
	}
	if followOtherChain == followOne {
		t.Fatal("follow-up operation identity did not bind the direct parent")
	}
	request := launchFingerprintRequestForTest()
	request.GoalID, request.GoalRevision = "goal-a", 7
	root := t.TempDir()
	fingerprintOne, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatal(err)
	}
	request.GoalRevision = 8
	fingerprintTwo, err := CanonicalizeLaunchFingerprint(root, request, 120)
	if err != nil {
		t.Fatal(err)
	}
	if fingerprintOne.Version != 2 || fingerprintOne.Digest == fingerprintTwo.Digest {
		t.Fatalf("v2 fingerprints did not bind revision: %+v %+v", fingerprintOne, fingerprintTwo)
	}
}

func TestComposeRolePacketAcceptsThePriorWorktreeSlotAndRefusesOthers(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	worktreeFact := filepath.Join(temp, "prior-worktree.md")
	if err := os.WriteFile(brief, []byte("Continue the build.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(worktreeFact, []byte("Round 1 of this chain was cut off at its 120-minute cap.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	params := ComposeRolePacketParams{
		Root: root, Role: "implementer", Brief: brief, JobID: "build-a-r2", Runtime: "fake",
		Model: "fake-model", ToolPolicy: "read-write", Round: 2, DestructiveReach: HazardMechanical,
		Output: filepath.Join(temp, "prompt.md"), CompositionOutput: filepath.Join(temp, "composition.json"),
		Continuations: []CompositionContinuation{{Slot: "prior-worktree", Path: worktreeFact}},
	}
	record, err := ComposeRolePacket(params)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := os.ReadFile(params.Output)
	if err != nil {
		t.Fatal(err)
	}
	last := record.Sources[len(record.Sources)-1]
	if last.Slot != "prior-worktree" || last.Source != "engine:prior-worktree" || !strings.Contains(string(packet), "# Prior Worktree\n\nRound 1 of this chain was cut off") {
		t.Fatalf("the prior-worktree slot did not compose: %+v\n%s", last, packet)
	}
	if _, err := readCompositionForJob(params.CompositionOutput, "build-a-r2", "implementer", "fake", "fake-model", "", HazardMechanical, 0, 2, int64(len(packet)), record.PacketDigest); err != nil {
		t.Fatalf("the composition validation refused the engine slot: %v", err)
	}
	params.Continuations = []CompositionContinuation{{Slot: "prior-worktree-notes", Path: worktreeFact}}
	if _, err := ComposeRolePacket(params); err == nil || !strings.Contains(err.Error(), "undeclared continuation slot") {
		t.Fatalf("an undeclared slot was accepted: %v", err)
	}
}

func TestComposeRolePacketWritesTheReturnBySentenceForAKnownCap(t *testing.T) {
	root := compositionRepoRoot(t)
	temp := t.TempDir()
	brief := filepath.Join(temp, "brief.md")
	if err := os.WriteFile(brief, []byte("Do the focused task.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compose := func(name string, capMinutes, margin int64, truncated bool) string {
		t.Helper()
		params := ComposeRolePacketParams{
			Root: root, Role: "verifier", Brief: brief, JobID: "verify-" + name, Runtime: "fake",
			Model: "fake-model", ToolPolicy: "read-only", Round: 1, DestructiveReach: HazardMechanical,
			Output: filepath.Join(temp, name+".md"), CompositionOutput: filepath.Join(temp, name+".json"),
			CapMinutes: capMinutes, ReturnMarginMinutes: margin, CapTruncated: truncated,
		}
		if _, err := ComposeRolePacket(params); err != nil {
			t.Fatal(err)
		}
		packet, err := os.ReadFile(params.Output)
		if err != nil {
			t.Fatal(err)
		}
		return string(packet)
	}
	if packet := compose("known", 120, 10, false); !strings.Contains(packet, "Your round is capped at 120 minutes from its reservation; write your return by minute 110, naming what is left.") {
		t.Fatalf("a known cap did not write the return-by sentence:\n%s", packet)
	}
	if packet := compose("zero", 120, 0, false); !strings.Contains(packet, "write your return by minute 120, naming what is left.") {
		t.Fatalf("a zero margin did not ask for the return by the cap itself:\n%s", packet)
	}
	for name, packet := range map[string]string{
		"no cap":      compose("nocap", 0, 10, false),
		"margin over": compose("over", 1, 10, false),
		"truncated":   compose("cut", 120, 10, true),
	} {
		if strings.Contains(packet, "write your return by") {
			t.Fatalf("%s wrote a return-by sentence:\n%s", name, packet)
		}
	}
	if sentence := returnBySentence(120, 10, false); !strings.HasSuffix(sentence, "\n") || strings.Contains(sentence, "2026") {
		t.Fatalf("the sentence is not one clock-free line: %q", sentence)
	}
}

func TestReturnMarginMinutesReadsTheConfiguredMargin(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if margin, err := ReturnMarginMinutes(conf); err != nil || margin != 10 {
		t.Fatalf("default margin = %d, %v", margin, err)
	}
	if err := os.WriteFile(conf, []byte("dispatch.return-margin-min=15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if margin, err := ReturnMarginMinutes(conf); err != nil || margin != 15 {
		t.Fatalf("configured margin = %d, %v", margin, err)
	}
	if err := os.WriteFile(conf, []byte("dispatch.return-margin-min=soon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReturnMarginMinutes(conf); err == nil {
		t.Fatal("a malformed margin was accepted")
	}
}
