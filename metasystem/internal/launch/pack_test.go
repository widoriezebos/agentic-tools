package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func packTemplates(t *testing.T, m *Manager, design, review string) {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "design-brief.md"), []byte(design), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "review-brief.md"), []byte(review), 0o600); err != nil {
		t.Fatal(err)
	}
	field := reflect.ValueOf(m).Elem().FieldByName("TemplateDirectory")
	if field.IsValid() {
		field.SetString(directory)
	}
}

func TestPackCheckRefusesAnUnfilledBrief(t *testing.T) {
	rows := []struct {
		name     string
		template string
		brief    string
		want     string
		line     int
	}{
		{name: "template itself", template: "# <page name>\nBudget: <N>\n", brief: "# <page name>\nBudget: <N>\n", want: "<page name>", line: 1},
		{name: "one token left", template: "# <page name>\nBudget: <N>\n", brief: "# Filled\nBudget: <N>\n", want: "<N>", line: 2},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			m, _, _, _ := manager(t)
			m.Supervisor = childStarter(m)
			packTemplates(t, m, row.template, "review\n")
			brief := writeLaunchFile(t, "brief.md", row.brief)
			_, err := m.Start(StartSpec{ID: "unfilled", Kind: "design", Brief: brief, WorkingDirectory: t.TempDir()})
			want := "LAUNCH_BRIEF_PACK_UNFILLED kind=design\nplaceholder=" + row.want + " line=" + map[int]string{1: "1", 2: "2"}[row.line]
			if row.name == "template itself" {
				want += "\nplaceholder=<N> line=2"
			}
			if err == nil || err.Error() != want {
				t.Fatalf("error=%v", err)
			}
			if _, statErr := os.Stat(filepath.Join(m.Store.Root, "unfilled")); !os.IsNotExist(statErr) {
				t.Fatalf("state directory exists: %v", statErr)
			}
			refusals, readErr := m.Store.Refusals()
			if readErr != nil || len(refusals) != 1 || refusals[0].Code != "LAUNCH_BRIEF_PACK_UNFILLED" {
				t.Fatalf("refusals=%+v err=%v", refusals, readErr)
			}
		})
	}
}

func TestPackCheckRefusesADriftedExcerpt(t *testing.T) {
	rows := []struct {
		name, kind, file, content, brief, want string
	}{
		{name: "changed byte", kind: "design", file: "source.txt", content: "one\ntwo\n", brief: "1. `source.txt:1-2`\n\n   ```text\n   one\n   too\n   ```\n", want: "line 2 differs source_bytes=3 excerpt_bytes=3"},
		{name: "short file", kind: "design", file: "short.txt", content: "one\n", brief: "`short.txt:1-2`\n", want: "past end (1 lines)"},
		{name: "missing file", kind: "design", brief: "`missing.txt:1-1`\n", want: "missing file"},
		{name: "review past end", kind: "read", file: "review.txt", content: "one\ntwo", brief: "Check `review.txt:2-3` here.\n", want: "past end (2 lines)"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			m, _, _, _ := manager(t)
			packTemplates(t, m, "filled design\n", "filled review\n")
			directory := t.TempDir()
			if row.file != "" {
				if err := os.WriteFile(filepath.Join(directory, row.file), []byte(row.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			brief := writeLaunchFile(t, "brief.md", row.brief)
			spec := StartSpec{Kind: row.kind, Brief: brief, WorkingDirectory: directory}
			if row.kind == "read" {
				spec.DiffFile = writeLaunchFile(t, "review.diff", "")
			}
			err := m.Admit(spec)
			if err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_PACK_DRIFTED") || !strings.Contains(err.Error(), "range=`") || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestPackCheckAdmitsAFilledBrief(t *testing.T) {
	m, _, _, _ := manager(t)
	packTemplates(t, m, "# <name>\n`<file>:<start>-<end>`\n<excerpt>\n<recurring finding classes, or none recorded>\n", "# <name>\n`<file>:<start>-<end>`\n")
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "alpha.txt"), []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "metasystem"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "metasystem", "nested.txt"), []byte("first\nsecond"), 0o600); err != nil {
		t.Fatal(err)
	}
	design := writeLaunchFile(t, "design.md", "# Ready\n1. `alpha.txt:1-2`\n\n   ```text\n   alpha\n   beta\n   ```\n2. `nested.txt:2-2`\n\n   ```\n   second\n   ```\n\n## Recurring findings\n\nnone recorded\n")
	if err := m.Admit(StartSpec{Kind: "design", Brief: design, WorkingDirectory: directory}); err != nil {
		t.Fatalf("filled design: %v", err)
	}
	review := writeLaunchFile(t, "review.md", "# Ready\nCheck `alpha.txt:1-1`.\n")
	if err := m.Admit(StartSpec{Kind: "read", Brief: review, WorkingDirectory: directory, DiffFile: writeLaunchFile(t, "read.diff", "")}); err != nil {
		t.Fatalf("filled review: %v", err)
	}
	if !reflect.ValueOf(m).MethodByName("CheckPack").IsValid() {
		t.Fatal("filled briefs admitted without a reusable pack check")
	}
	m.Settings = DefaultSettings()
	m.Settings.BriefCap = 1
	if err := m.Admit(StartSpec{Kind: "design", Brief: design, WorkingDirectory: directory}); err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_OVERSIZE") {
		t.Fatalf("oversize filled design error=%v", err)
	}
}

func TestPackCheckChecksEveryShippedTemplatePlaceholder(t *testing.T) {
	t.Parallel()
	pattern := regexp.MustCompile(`<[^<>]+>`)
	directory := filepath.Join("..", "..", "scripts", "agents", "templates")
	for _, row := range []struct {
		kind string
		name string
	}{
		{kind: "design", name: "design-brief.md"},
		{kind: "read", name: "review-brief.md"},
	} {
		t.Run(row.kind, func(t *testing.T) {
			template, err := os.ReadFile(filepath.Join(directory, row.name))
			if err != nil {
				t.Fatal(err)
			}
			matches := pattern.FindAllIndex(template, -1)
			if len(matches) == 0 {
				t.Fatal("shipped template has no placeholders")
			}

			m, _, _, _ := manager(t)
			m.TemplateDirectory = directory
			brief := writeLaunchFile(t, row.kind+"-unfilled.md", string(template))
			_, err = m.CheckPack(StartSpec{Kind: row.kind, Brief: brief, WorkingDirectory: t.TempDir()})
			want := []string{"LAUNCH_BRIEF_PACK_UNFILLED kind=" + row.kind}
			for _, match := range matches {
				token := strings.Join(strings.Fields(string(template[match[0]:match[1]])), " ")
				line := 1 + strings.Count(string(template[:match[0]]), "\n")
				want = append(want, "placeholder="+token+" line="+fmt.Sprint(line))
			}
			if err == nil || err.Error() != strings.Join(want, "\n") {
				t.Fatalf("unfilled shipped template error:\n%v\nwant:\n%s", err, strings.Join(want, "\n"))
			}

			filled := pattern.ReplaceAll(template, []byte("filled"))
			brief = writeLaunchFile(t, row.kind+"-filled.md", string(filled))
			spec := StartSpec{Kind: row.kind, Brief: brief, WorkingDirectory: t.TempDir()}
			if row.kind == "read" {
				spec.DiffFile = writeLaunchFile(t, "filled.diff", "")
			}
			if err := m.Admit(spec); err != nil {
				t.Fatalf("filled shipped template: %v", err)
			}

			for _, match := range matches {
				token := string(template[match[0]:match[1]])
				if !strings.Contains(token, "\n") {
					continue
				}
				collapsed := strings.Join(strings.Fields(token), " ")
				rewrapped := strings.Replace(collapsed, " ", "\n       ", 1)
				brief = writeLaunchFile(t, row.kind+"-rewrapped.md", rewrapped+"\n")
				_, err := m.CheckPack(StartSpec{Kind: row.kind, Brief: brief, WorkingDirectory: t.TempDir()})
				want := "LAUNCH_BRIEF_PACK_UNFILLED kind=" + row.kind + "\nplaceholder=" + collapsed + " line=1"
				if err == nil || err.Error() != want {
					t.Fatalf("rewrapped placeholder error=%v want=%q", err, want)
				}
				return
			}
			t.Fatal("shipped template has no multi-line placeholder")
		})
	}
}

func TestDesignAndReadLaunchesRunThePackCheck(t *testing.T) {
	m, _, _, _ := manager(t)
	packTemplates(t, m, "Design <N>\n", "Read <N>\n")
	for _, kind := range []string{"build", "critique"} {
		content := "Keep <N>\n"
		if kind == "build" {
			content = "| Unit | Lines |\n|---|---|\n| fixture | 1 |\n" + content
		}
		brief := writeLaunchFile(t, kind+".md", content)
		spec := StartSpec{Kind: kind, Brief: brief, WorkingDirectory: t.TempDir()}
		if kind == "critique" {
			spec.Tag = "pack-check"
			spec.Inputs = []string{writeLaunchFile(t, "page.md", "page\n"), writeLaunchFile(t, "design.md", "design\n")}
		}
		if err := m.Admit(spec); err != nil {
			t.Fatalf("%s should not check packs: %v", kind, err)
		}
	}
	for _, kind := range []string{"design", "read"} {
		kindName := map[string]string{"design": "Design", "read": "Read"}[kind]
		brief := writeLaunchFile(t, kind+".md", kindName+" <N>\n")
		spec := StartSpec{Kind: kind, Brief: brief, WorkingDirectory: t.TempDir()}
		if kind == "read" {
			spec.DiffFile = writeLaunchFile(t, "invalid.diff", "")
		}
		if err := m.Admit(spec); err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_PACK_UNFILLED") {
			t.Fatalf("%s error=%v", kind, err)
		}
	}
}

func TestTemplatesTellTheDelegateToBatchReads(t *testing.T) {
	const sentence = "Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn."
	directory := filepath.Join("..", "..", "scripts", "agents", "templates")
	for _, name := range []string{"design-brief.md", "review-brief.md"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || !strings.Contains(string(data), sentence) {
			t.Errorf("%s does not carry the batching instruction: %v", name, err)
		}
	}
}

func TestDesignTemplateCarriesRecurringFindings(t *testing.T) {
	directory := filepath.Join("..", "..", "scripts", "agents", "templates")
	data, err := os.ReadFile(filepath.Join(directory, "design-brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	const placeholder = "<recurring finding classes, or none recorded>"
	if !strings.Contains(string(data), "## Recurring findings") || !strings.Contains(string(data), placeholder) {
		t.Fatalf("design template does not carry the recurring-findings section")
	}
	m, _, _, _ := manager(t)
	field := reflect.ValueOf(m).Elem().FieldByName("TemplateDirectory")
	if field.IsValid() {
		field.SetString(directory)
	}
	brief := writeLaunchFile(t, "brief.md", placeholder+"\n")
	if err := m.Admit(StartSpec{Kind: "design", Brief: brief, WorkingDirectory: t.TempDir()}); err == nil || !strings.HasPrefix(err.Error(), "LAUNCH_BRIEF_PACK_UNFILLED") {
		t.Fatalf("error=%v", err)
	}
}

func TestPackRefusalCodesAreRegistered(t *testing.T) {
	want := map[string]bool{
		"LAUNCH_BRIEF_PACK_UNFILLED": false,
		"LAUNCH_BRIEF_PACK_DRIFTED":  false,
	}
	for _, row := range refusal.Rows {
		file, _, _ := strings.Cut(row.Site, ":")
		if _, ok := want[row.Code]; ok && row.Owner == "internal/launch" && file == "pack.go" && row.Shape == refusal.Question {
			want[row.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("%s has no internal/launch pack.go Question row", code)
		}
	}
}
