# Application testing contract

An adopted application declares its own tests in `testing.json`, selected by
`testing.contract=testing.json` in `metasystem.conf`. The engine plans, runs,
retains, and verifies those tests. The application can use any language: a
`command` group runs its `argv`, then checks its exit status and declared JUnit
XML test identities. The application command or its framework adapter must
produce the report. Go package and test discovery applies only to `go` groups;
other languages declare their source and dependency paths themselves.

This complete schema 2 example uses a shell check in place of an application
test command. In an enrolled application's root, put `ready` on one line in
both `app/harness.txt` and `app/input.txt`, create `scripts/check.sh`, and ignore
generated `reports/` in `.gitignore`:

```sh
#!/bin/sh
set -eu
id=$1 input=$2 report=$3
mkdir -p "$report"
IFS= read -r value < "$input"
if [ "$value" = ready ]; then
  printf '<testsuite><testcase classname="app" name="%s"/></testsuite>\n' "$id" > "$report/result.xml"
else
  printf '<testsuite><testcase classname="app" name="%s"><failure message="input is not ready"/></testcase></testsuite>\n' "$id" > "$report/result.xml"
  exit 1
fi
```

Save this as `testing.json` (the generated `reports/` directories are outputs):

```json
{
  "schemaVersion": 2,
  "projectRisk": {"severity": 1, "exposure": 1, "reversibility": "revert", "detection": "immediate", "recovery": "bounded"},
  "surfaces": [
    {"id": "app", "paths": ["app/**", "scripts/check.sh", "testing.json"], "dependsOn": [], "standard": ["app-check"], "deep": [], "critical": ["app-read"]}
  ],
  "groups": [
    {
      "id": "harness", "kind": "component", "adapter": "command", "cwd": ".",
      "phase": "admission", "environmentMode": "explicit", "env": {"PATH": "/usr/bin:/bin", "LC_ALL": "C"},
      "resources": {"class": "cheap"}, "freshness": "reusable",
      "inputs": ["app/harness.txt", "scripts/check.sh"], "outputs": ["reports/harness"],
      "tools": [{"id": "shell", "executable": "sh", "versionArgs": ["-c", "printf shell"]}],
      "obligations": [], "platforms": ["any"], "targetMs": 1000,
      "argv": ["sh", "scripts/check.sh", "harness", "app/harness.txt", "reports/harness"],
      "reports": ["reports/harness"], "format": "junit-xml",
      "expectedTests": [{"report": "reports/harness/result.xml", "classname": "app", "name": "harness"}]
    },
    {
      "id": "app-check", "requires": ["harness"], "kind": "component", "adapter": "command", "cwd": ".",
      "phase": "acceptance", "environmentMode": "explicit", "env": {"PATH": "/usr/bin:/bin", "LC_ALL": "C"},
      "resources": {"class": "heavy"}, "freshness": "reusable",
      "inputs": ["app/input.txt", "scripts/check.sh"], "outputs": ["reports/app-check"],
      "tools": [{"id": "shell", "executable": "sh", "versionArgs": ["-c", "printf shell"]}],
      "obligations": ["app-read"], "platforms": ["any"], "targetMs": 1000,
      "argv": ["sh", "scripts/check.sh", "app-check", "app/input.txt", "reports/app-check"],
      "reports": ["reports/app-check"], "format": "junit-xml",
      "expectedTests": [{"report": "reports/app-check/result.xml", "classname": "app", "name": "app-check"}]
    }
  ],
  "always": {"canary": ["harness"], "standard": []},
  "unknown": ["app-check"], "cadence": []
}
```

Run `bin/metasystem test check --root .` to validate the contract and native
tool readiness. `test plan --root . --goal <id> --mode auto` shows the selected
groups; `test run` executes them, and `test verify --root . --goal <id> --tree
<tree>` checks sufficient retained evidence without launching tests or builds.
Use the exact candidate tree and the same goal when running and verifying.
Diagnostic plans and results cannot authorize delivery.

`requires` closes selection over prerequisites and runs them before their
dependents. A failed prerequisite blocks its dependents, including a dependent
with cached success; independent groups still run. `phase: admission` permits
an early join check, while the full selected acceptance plan remains required
for delivery. `inputs` names repository paths that affect a group; `tools`
binds executable identity and version-command output; `environmentMode:
explicit` constructs exactly `env`, while `inherit` includes the caller's
environment. Declare consumed files outside the tree with `externalInputs`
entries (`id` and an absolute path or `${NAME}/path` locator). Declare reports
and other produced artifacts in `outputs`. An explicit environment must include
`NAME` when an external locator uses `${NAME}`. A prerequisite edge does not imply
that its produced artifact is an input: declare that consumed path separately.

`resources.class` is `cheap` or `heavy`; `resources.exclusive` lists named
shared resources a group actually holds, such as a fixture database. These
claims use the bounded local execution owner. `freshness: reusable` permits a
matching retained result; `episode` requires new evidence for a new decision
while allowing exact resume within its retained episode. Optional positive
`freshnessMaxAgeMs` limits an episode's lifetime. A caller resuming an episode
passes the same `--fresh-episode` token and `--fresh-expires-at` timestamp to
`test run` and `test verify`.

Upgrade and enroll the engine while the application still uses a valid schema
1 contract; it retains its conservative behavior. Then add reviewed schema 2
fields, run `test check` and `test plan`, and complete the selected delivery
proof against the trusted destination policy. Keep existing acceptance groups
and prerequisite edges during the transition; candidate policy cannot remove
protected checks. Do not use a diagnostic admission result as the delivery
receipt.
