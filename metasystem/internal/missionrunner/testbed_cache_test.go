package missionrunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type missionBedTemplate struct {
	dir     string
	rootRel string
	origin  bool
}

var (
	missionBedTemplateRoot string
	missionBedTemplateMu   sync.Mutex
	missionBedTemplateSeq  int
	fullCycleTemplates     = map[string]*missionBedTemplate{}
	wallRepoTemplate       *missionBedTemplate
	nestedScopeTemplate    *missionBedTemplate
	parkedSoloTemplates    = map[string]*missionBedTemplate{}
)

func prepareMissionBedTemplates() error {
	root, err := os.MkdirTemp("", "missionrunner-bed-templates.")
	if err != nil {
		return err
	}
	missionBedTemplateRoot = root
	return nil
}

func cleanMissionBedTemplates() {
	if missionBedTemplateRoot != "" {
		_ = os.RemoveAll(missionBedTemplateRoot)
	}
}

func copyMissionBedTree(source, destination string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("cp", "-cR", source, destination)
	} else {
		cmd = exec.Command("cp", "-a", "--reflink=auto", source, destination)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy mission bed: %w\n%s", err, out)
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func captureMissionBedTemplate(t *testing.T, root string) *missionBedTemplate {
	t.Helper()
	top, err := gitOutput(root, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	resolvedTop, err := filepath.EvalSymlinks(top)
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(resolvedTop, resolvedRoot)
	if err != nil {
		t.Fatal(err)
	}

	missionBedTemplateMu.Lock()
	missionBedTemplateSeq++
	templateDir := filepath.Join(missionBedTemplateRoot, fmt.Sprintf("bed-%03d", missionBedTemplateSeq))
	missionBedTemplateMu.Unlock()
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(templateDir, "checkout")
	if err := copyMissionBedTree(top, checkout); err != nil {
		t.Fatal(err)
	}

	template := &missionBedTemplate{dir: templateDir, rootRel: rel}
	origin, originErr := gitOutput(root, "remote", "get-url", "origin")
	if originErr == nil {
		if !filepath.IsAbs(origin) {
			origin = filepath.Join(root, origin)
		}
		if info, statErr := os.Stat(origin); statErr == nil && info.IsDir() {
			template.origin = true
			templateOrigin := filepath.Join(templateDir, "origin.git")
			if err := copyMissionBedTree(origin, templateOrigin); err != nil {
				t.Fatal(err)
			}
			if _, err := gitOutput(filepath.Join(checkout, rel), "remote", "set-url", "origin", templateOrigin); err != nil {
				t.Fatal(err)
			}
		}
	}
	return template
}

func (template *missionBedTemplate) clone(t *testing.T) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "bed")
	if err := copyMissionBedTree(template.dir, destination); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(destination, "checkout", template.rootRel)
	if template.origin {
		origin := filepath.Join(destination, "origin.git")
		if _, err := gitOutput(root, "remote", "set-url", "origin", origin); err != nil {
			t.Fatal(err)
		}
	}
	resetMissionBedTransportState(t, root)
	return root
}

func resetMissionBedTransportState(t *testing.T, root string) {
	t.Helper()
	gitDir, err := gitOutput(root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	// FETCH_HEAD describes how the template itself was transported. It is
	// not part of a mission bed and would not exist in a fresh checkout.
	if err := os.Remove(filepath.Join(gitDir, "FETCH_HEAD")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func copyFullCycleRoot(t *testing.T, behavior string) *Engine {
	t.Helper()
	missionBedTemplateMu.Lock()
	template := fullCycleTemplates[behavior]
	missionBedTemplateMu.Unlock()
	if template == nil {
		engine := buildFullCycleRoot(t, behavior)
		template = captureMissionBedTemplate(t, engine.Root)
		missionBedTemplateMu.Lock()
		fullCycleTemplates[behavior] = template
		missionBedTemplateMu.Unlock()
		return engine
	}

	engine := &Engine{Root: template.clone(t), Mission: "alpha"}
	writeFreshSupervision(t, engine)
	engine.anchorFn = func(statePath, ledgerPath, identityName string) error {
		return mission.Anchor(statePath, engine.Root, ledgerPath)
	}
	return engine
}
