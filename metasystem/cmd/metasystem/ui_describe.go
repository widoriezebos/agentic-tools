package main

import (
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/web"
)

// Where the interface's description is assembled, for the two processes that
// need it.
//
// The interface server answers GET /api/interface with it, and the tool server
// it hands the Project Partner answers interface() with it. They are separate
// processes with separately resolved roots, and the join is written once here
// so that the page and the Partner cannot be told two different things about
// the same build.
//
// Every owner is reached for exactly once: the built half from the bundle this
// executable carries, the acts from the server that offers them, the settings
// from the configuration reader, the record homes from the resolver, the
// runtimes from the package that admits them.

// confPathFor is where this installation keeps its configuration.
func confPathFor(roots lifecycle.Roots) string {
	return filepath.Join(roots.Installation, "metasystem.conf")
}

// describeInterface composes the manifest for one checkout, reading the
// seat's configuration afresh on every call so that a setting changed while a
// server runs is described without a restart.
func describeInterface(roots lifecycle.Roots) func() (manifest.Manifest, error) {
	confPath := confPathFor(roots)
	return func() (manifest.Manifest, error) {
		configured, _, _, err := config.UIPartner(confPath)
		if err != nil {
			configured = ""
		}
		return manifest.Compose(manifest.Sources{
			Dist:     web.Dist(),
			ConfPath: confPath,
			Roots: resolver.Roots{
				Checkout: roots.Checkout, Installation: roots.Installation, StateRoot: roots.StateRoot,
			},
			Acts:     httpd.Acts(),
			Runtimes: admittedRuntimes(configured),
			Partner: manifest.Partner{
				Configured: configured != "",
				Reads:      partner.Reads,
				Refused:    partner.Refused,
			},
		}), nil
	}
}

// admittedRuntimes is every runtime this build will start as the Partner, with
// the model and command it starts them with. The names, models and commands
// are the partner package's; nothing is restated here.
func admittedRuntimes(configured string) []manifest.Runtime {
	admitted := partner.Admitted()
	runtimes := make([]manifest.Runtime, 0, len(admitted))
	for _, name := range admitted {
		command := ""
		if argv := partner.DefaultCommands[name]; len(argv) > 0 {
			command = argv[0]
		}
		runtimes = append(runtimes, manifest.Runtime{
			Name:       name,
			Model:      partner.DefaultModels[name],
			Command:    command,
			Configured: name == configured,
		})
	}
	return runtimes
}

// commandCatalogue is the public command table followed by the engine's own
// verb table, in the shape the kit tool reads it. It is derived from the table main routes with, so a verb this
// binary answers and a verb the Partner can describe are the same verb.
func commandCatalogue() []uitools.CommandFamily {
	routed := families()
	catalogue := make([]uitools.CommandFamily, 0, len(routed)+1)
	// The public commands come first; every engine family follows unchanged,
	// reachable directly or as metasystem internal FAMILY VERB.
	public := uitools.CommandFamily{Name: "metasystem", Summary: "public commands for what a person or agent wants done; families below are the internal catalogue"}
	for _, command := range intentCommands() {
		public.Verbs = append(public.Verbs, uitools.Command{Name: command.name, Summary: command.summary})
	}
	public.Verbs = append(public.Verbs,
		uitools.Command{Name: "help", Summary: "help human, help agent, help COMMAND, help internal"},
		uitools.Command{Name: "internal", Summary: "run an engine family verb: metasystem internal FAMILY VERB ..."})
	catalogue = append(catalogue, public)
	for _, family := range routed {
		verbs := make([]uitools.Command, 0, len(family.verbs))
		for _, one := range family.verbs {
			verbs = append(verbs, uitools.Command{Name: one.name, Summary: one.summary})
		}
		catalogue = append(catalogue, uitools.CommandFamily{
			Name: family.name, Summary: family.summary, Verbs: verbs,
		})
	}
	return catalogue
}
