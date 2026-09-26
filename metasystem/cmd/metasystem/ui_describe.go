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

// commandCatalogue is the public command table in the shape the kit tool
// reads it, derived from the table main routes with, so a command this binary
// answers and a command the Partner can describe are the same command. The
// private process protocols are not part of it.
func commandCatalogue() []uitools.CommandFamily {
	public := uitools.CommandFamily{Summary: "the public commands: say what you want done; metasystem help lists them by area"}
	for _, command := range publicIntentCommands() {
		public.Verbs = append(public.Verbs, uitools.Command{Name: command.name, Summary: command.summary, Scope: command.helpScope(),
			Usage: command.usage, AdministrationUsage: command.administrationUsage})
	}
	public.Verbs = append(public.Verbs,
		uitools.Command{Name: "help", Summary: "help goals, help work, help questions, help operations, help administration, help human, help agent, help all, help COMMAND; add --json for structured help"})
	return []uitools.CommandFamily{public}
}
