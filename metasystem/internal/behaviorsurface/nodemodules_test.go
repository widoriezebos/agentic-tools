package behaviorsurface

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The installed frontend dependency tree lies under internal/**, which is both
// an engine path and a payload root, so membership is the only fence: without
// the nonRepositoryPaths refusal in every projection it would join the ENGINE
// and PAYLOAD digests (g1-s8 revision 5, the exclusion slice).
func TestDependencyTreeIsOutsideEveryProjection(t *testing.T) {
	t.Parallel()
	policy, err := Load()
	testutil.Require(t, "policy loads", err, nil)

	for _, name := range []string{
		"internal/ui/web/_app/node_modules",
		"internal/ui/web/_app/node_modules/poison/seam.go",
		"internal/ui/web/_app/node_modules/poison/node_modules/deeper/seam.go",
		"internal/ui/web/_app/node_modules/.bin/poison",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			class, err := policy.Classify(name, "")
			testutil.Expect(t, "classify error", err, nil)
			testutil.Expect(t, "class", class, NonRepository)
			for _, projection := range []Projection{Engine, Landing, Payload} {
				include, err := policy.Includes(projection, name, "")
				testutil.Expect(t, string(projection)+" includes error", err, nil)
				testutil.Expect(t, string(projection)+" includes", include, false)
			}
		})
	}
}

// The neighbouring application source stays inside the projections the
// dependency tree leaves: the exclusion is the one directory, not _app.
func TestApplicationSourceStaysInsideTheProjections(t *testing.T) {
	t.Parallel()
	policy, err := Load()
	testutil.Require(t, "policy loads", err, nil)

	const name = "internal/ui/web/_app/src/main.tsx"
	for projection, want := range map[Projection]bool{Engine: true, Landing: true, Payload: true} {
		include, err := policy.Includes(projection, name, "")
		testutil.Expect(t, string(projection)+" includes error", err, nil)
		testutil.Expect(t, string(projection)+" includes", include, want)
	}
}
