package main

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/deploy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func TestDeployBuildsTheCommitsEngineUnderTheHomeAndAnswersAlone(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	registry := t.TempDir()
	f.env["METASYSTEM_SUPERVISION_REGISTRY_HOME"] = registry
	serve := func(operation, request string) (int, deploy.Response) {
		t.Helper()
		f.stdout.Reset()
		d := f.deps()
		// The engine go build links: a program carrying the stamp record
		// its -ldflags name.
		d.goTool = func(_ context.Context, _ string, _, args []string, _, _ io.Writer) error {
			script := "#!/bin/sh\n# " + enginebuild.StampRecord(stampOf(args)) + "\nexit 0\n"
			return testexec.WriteFile(args[slices.Index(args, "-o")+1], []byte(script), 0o755)
		}
		d.stdin = strings.NewReader(request)
		code := run(context.Background(), []string{"deploy", operation}, f.root, d)
		var response deploy.Response
		if f.stdout.Len() > 0 {
			if err := json.Unmarshal(f.stdout.Bytes(), &response); err != nil {
				t.Fatalf("standard output %q is not the response alone: %v", f.stdout.String(), err)
			}
		}
		return code, response
	}
	code, built := serve("build", `{"schema":1,"operation":"build","commit":"`+fixtureCommit+`"}`)
	engine := filepath.Join(registry, ".metasystem", "engines", fixtureCommit, "metasystem")
	if code != 0 || built.Outcome != "built" || built.Version != fixtureCommit || built.Artifact != engine {
		t.Fatalf("build = %d %+v; stderr:\n%s", code, built, f.stderr.String())
	}
	if !strings.Contains(f.stderr.String(), "go-build: test engine @ "+fixtureCommit) {
		t.Fatalf("stderr = %q, want the build's own output in the deploy's log", f.stderr.String())
	}
	if code, active := serve("version", `{"schema":1,"operation":"version"}`); code != 0 || active.Outcome != "none" {
		t.Fatalf("version = %d %+v, want none: a build activates nothing", code, active)
	}
	if code, response := serve("promote", `{}`); code != 64 || response.Outcome != "failed" {
		t.Fatalf("an unknown operation = %d %+v, want exit 64 and a failed answer", code, response)
	}
}
