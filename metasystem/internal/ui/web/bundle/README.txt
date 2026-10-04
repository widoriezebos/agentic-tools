This directory holds the built interface bundle.

bundle.json is the manifest and the publish point: the bundle script removes it
before the first destructive step and writes it last, so its presence means a
complete build and its absence means no bundle at all. dist/ is the Vite output,
emptied on every build and served by the engine.

Neither is written by hand. Rebuild both with, from internal/ui/web/_app:

    npm ci --ignore-scripts
    npx -y --ignore-scripts=false -p node@24.21.0 -- npm run bundle

metasystem/testing.json declares bundle.json and dist/ as generated with these
commands, so metasystem landing resolve regenerates them after a merge, and
this README is not generated.

This file is committed so that the //go:embed pattern in embed.go always
matches: an engine built from a checkout without dist/ still compiles and states
that it carries no interface bundle.
