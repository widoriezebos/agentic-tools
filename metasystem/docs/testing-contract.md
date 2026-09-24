# Application testing contract

An adopted application declares its own tests in `testing.json`, selected by
`testing.contract=testing.json` in `metasystem.conf`. The engine plans, runs,
retains, and verifies those tests. The application can use any language: a
`command` group runs its `argv`. With `format: exit-status`, an acceptance
`unit` or `integration` group succeeds only when that native command exits
zero. The application author must supply a command that runs the intended
tests. Exit-status evidence cannot inspect the command's semantics or guarantee
that its test framework discovers a nonempty suite; it proves only that the
configured command completed successfully. A command that emits JUnit XML can
instead declare its reports and expected test identities. Go package and test
discovery applies only to `go` groups; other languages declare their inputs
themselves.

This complete schema 2 example runs an application's ordinary full test
command. Save it as `testing.json`:

<!-- full-suite-command-contract -->
```json
{
  "schemaVersion": 2,
  "projectRisk": {"severity": 1, "exposure": 1, "reversibility": "revert", "detection": "immediate", "recovery": "bounded"},
  "fallback": "other",
  "surfaces": [
    {"id": "application", "paths": ["src/**", "package.json", "package-lock.json", "testing.json"], "dependsOn": [], "standard": ["application-tests"], "deep": [], "critical": []},
    {"id": "other", "paths": [], "dependsOn": [], "standard": ["application-tests"], "deep": [], "critical": []}
  ],
  "groups": [
    {
      "id": "application-tests", "kind": "unit", "adapter": "command", "cwd": ".",
      "phase": "acceptance", "environmentMode": "inherit",
      "resources": {"class": "heavy"}, "freshness": "reusable",
      "inputs": ["*", "*/**"], "outputs": [],
      "tools": [{"id": "npm", "executable": "npm", "versionArgs": ["--version"]}],
      "obligations": [], "platforms": ["any"], "targetMs": 120000,
      "argv": ["npm", "test"], "format": "exit-status"
    }
  ],
  "always": {"canary": [], "standard": ["application-tests"]},
  "unknown": ["application-tests"], "cadence": []
}
```

The two input patterns bind every tracked project path: `*` covers root files
and `*/**` covers paths below root directories. Bare `**` and an empty input
list are unsupported. This is stable full-suite setup, not a per-file test map.
The execution identity still requires exact source, tool, external-input, and
environment matches. With `environmentMode: inherit`, the inherited environment
is part of that identity. Declare known external files explicitly; the engine
does not infer arbitrary dynamic or remote dependencies from source. No JUnit
output is declared here, so evidence contains one native command result and no
fabricated expected, observed, missing, or unexpected testcase census. A
nonzero native exit fails the group.

This complete schema 2 example uses a shell check in place of an application
test command. In an enrolled application's root, put `ready` on one line in
both `app/harness.txt` and `app/input.txt`, create `scripts/check.sh`, and ignore
generated `reports/` in `.gitignore`:

```sh
#!/bin/sh
set -eu
id=$1 input=$2 report=$3
: "${METASYSTEM_TEST_WORKERS:?the test runner must declare its worker allowance}"
[ "$METASYSTEM_TEST_WORKERS" -eq 1 ] || {
  printf 'this single-case adapter received %s workers, want 1\n' "$METASYSTEM_TEST_WORKERS" >&2
  exit 2
}
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
claims use the bounded local execution owner. For `command` and `section`
adapters, `resources.workers` declares the adapter's worker share: omission is
one worker, explicit zero is the complete attempt allowance, and a positive
integer is that exact share. A share larger than the attempt allowance is
refused before the native command starts. Go groups do not declare this field;
the Go adapter allocates one worker to each isolated native partition.
Performance groups always consume the complete attempt allowance so the
scheduler reservation, prepared identity, result, and exported environment
describe the same allocation. Their declaration is still protected and bound
into identity, even though it cannot reduce the performance reservation.

`shards` may partition either `tests: "all"` or an explicit list of named Go
tests. The existing Go adapter remains the sole discovery, partition, report,
and `TestMain` owner. Whole-package coverage still requires `tests: "all"`.

The engine exports the effective share as the reserved
`METASYSTEM_TEST_WORKERS` environment variable after constructing either an
inherited or explicit group environment. A contract cannot set that name. An
application adapter must consume it by setting its framework's process or
worker limit; merely receiving or forwarding the variable does not prove that
internal work is bounded. A nested standard runner treats a nonempty inherited
value as a cooperative ceiling: its resolved allowance is
`min(testing.workers-or-default, METASYSTEM_TEST_WORKERS)`. Thus a parent
allowance of one still caps a child whose committed setting is four. A
malformed or non-positive inherited value is refused. The variable
communicates a runner decision; it is neither a host lease nor additional
authentication or security authority.

`testing.workers` in `metasystem.conf` is the positive total allowance for one
attempt. For a top-level invocation with no inherited ceiling, an explicit
value overrides the computed default and has no arbitrary machine-size cap.
When it is absent, the engine captures `runtime.GOMAXPROCS(0)` once and divides
it by the resolved `proof.admission.top-level-max`, treating unlimited
admission (`0`) as a divisor of one and flooring the result at one. Nested
invocations apply the inherited ceiling after either resolution. This does not
change or claim an operating-system-wide CPU limit. `testing.concurrency`
continues to mean the maximum number of active test groups. Structured results
report the worker policy version, final attempt allowance, and admission
maximum separately; an admission maximum of zero is reported as unlimited.

Before trusted policy-plan execution, package expansion, candidate-engine
construction, native preparation, proof reservation, or admission, `test run`
asks the trusted destination worker for `metasystem test worker-capabilities`.
The response binds the capability schema, protocol
version, result schema, execution-identity version, and worker-policy version.
An unknown verb, malformed response, or mismatch is unsupported and refuses
the run with `TEST_WORKER_POLICY_UNSUPPORTED`; it never authorizes switching
execution to the untrusted candidate engine. Roll out the backend capability
and compatibility machinery under the existing frontend first, install that
trusted backend, and only then activate worker settings and declarations. This
is the supported two-step dependency, not a promise of arbitrary old/new
interoperability.

`freshness: reusable` permits a
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
