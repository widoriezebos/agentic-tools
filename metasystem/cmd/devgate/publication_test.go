package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestEnginePublicationKeepsTheEnrolledEngine(t *testing.T) {
	t.Parallel()
	const untouched = "bin/metasystem untouched: this checkout takes its engine from the deploy\n"
	for _, tc := range []struct {
		name, delivery, escape string
		enrolled, keep         bool
	}{
		{"deployed", "central", "", true, true},
		{"escape requires one", "central", "true", true, true},
		{"local", "source", "", true, false},
		{"escape", "central", "1", true, false},
		{"unenrolled", "central", "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newGateWorld(t)
			w.write("metasystem.conf", "metasystem.engine-delivery="+tc.delivery+"\n")
			if tc.enrolled {
				if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(w.root)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(steward.RepoIdentityPath(w.root), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			w.env = append(w.env, checkoutBuildVariable+"="+tc.escape)
			d := w.deps()
			// The build fixture's cache resolver stubs its own Git queries.
			d.cacheDomain = newBuildFixture(t).deps().cacheDomain
			ctx := context.Background()
			engine := filepath.Join(w.root, "bin", "metasystem")
			const deployed = "the deployed engine\n"
			reset := func() {
				w.writeMode("bin/metasystem", deployed, 0o755)
				w.stdout.Reset()
				w.stderr.Reset()
			}
			read := func(path string) string {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			check := func(path string, keep bool) {
				t.Helper()
				got := read(path)
				if keep && got != deployed {
					t.Fatalf("deployed engine changed: %q", got)
				}
				if !keep && !strings.HasPrefix(got, "engine built in "+w.root) {
					t.Fatalf("built engine was not published: %q", got)
				}
			}
			checkPublication := func() {
				t.Helper()
				check(engine, tc.keep)
				wantCount := 0
				if tc.keep {
					wantCount = 1
				}
				if count := strings.Count(w.output(), untouched); count != wantCount {
					t.Fatalf("untouched line count = %d; output: %s", count, w.output())
				}
				entries, err := os.ReadDir(filepath.Dir(engine))
				if err != nil || len(entries) != 1 {
					t.Fatalf("publication staging survived: %v, %v", entries, err)
				}
			}

			reset()
			if code := runBuild(ctx, nil, w.root, d); code != 0 {
				t.Fatalf("build exit %d: %s", code, w.output())
			}
			checkPublication()
			if tc.keep && w.output() != untouched {
				t.Fatalf("build output = %q, want %q", w.output(), untouched)
			}

			reset()
			if code := runStatic(ctx, nil, w.root, d); code != 0 {
				t.Fatalf("static exit %d: %s", code, w.output())
			}
			checkPublication()

			reset()
			candidate := filepath.Join(w.tmp, "witness-engine")
			if code := runBuild(ctx, []string{"--out", candidate}, w.root, d); code != 0 {
				t.Fatalf("private build exit %d: %s", code, w.output())
			}
			check(candidate, false)
			check(engine, true)
			w.stdout.Reset()
			if err := installBinary(candidate, w.root, d); err != nil {
				t.Fatal(err)
			}
			checkPublication()
			check(candidate, false)

			reset()
			proof := filepath.Join(w.tmp, "static-proof")
			if code := runStatic(ctx, []string{"--proof-out", proof}, w.root, d); code != 0 {
				t.Fatalf("private static exit %d: %s", code, w.output())
			}
			check(proof, false)
			check(engine, true)
			if strings.Contains(w.output(), untouched) {
				t.Fatalf("private publication was treated as a checkout install: %s", w.output())
			}
			if scratch, err := filepath.Glob(filepath.Join(w.tmp, "metasystem-gate-collect.*")); err != nil || len(scratch) != 0 {
				t.Fatalf("static scratch survived: %v, %v", scratch, err)
			}
		})
	}
	t.Run("dispatch refusal", func(t *testing.T) {
		t.Parallel()
		f := newCheckoutDeliveryFixture(t)
		if code := f.run(); code != 1 {
			t.Fatalf("dispatch exit %d: %s", code, f.stderr.String())
		}
		if f.stderr.String() != checkoutEngineRefusal(f.root, f.deps()) {
			t.Fatalf("dispatch did not use the shared refusal: %s", f.stderr.String())
		}
		for _, want := range []string{"takes its engine from the deploy", checkoutBuildVariable + "=1"} {
			if !strings.Contains(f.stderr.String(), want) {
				t.Fatalf("dispatch refusal lacks %q: %s", want, f.stderr.String())
			}
		}
		if len(f.goCalls) != 0 {
			t.Fatalf("dispatch refusal built an engine: %v", f.goCalls)
		}
	})
}
