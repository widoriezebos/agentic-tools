# taskrun: a dependency-aware task runner

Build a command-line task runner in Java. Given a file describing tasks, each
with a command, declared inputs and outputs, and dependencies on other tasks, it
works out the order, runs them, skips work whose inputs have not changed, and
reports failures without running anything that depended on the failure.

Everything below is graded from outside the program, by building it and running
it. Nothing is graded by reading your source.

## Project shape

- Java 21, built with Maven. The repository ships the Maven wrapper
  (`mvnw`, `mvnw.cmd`, `.mvn/`), and every build command below uses it.
- `./mvnw -o -q package` produces `target/taskrun.jar`, runnable as
  `java -jar target/taskrun.jar`. Set `finalName` so the jar has exactly that
  name, and make it executable as a single jar with no classpath arguments.
- **Builds run offline.** The environment has no network. A populated Maven
  repository is already present, holding the JDK-era plugins Maven needs plus
  JUnit 5. Every build and test command uses `-o`, and adding a dependency that
  is not already in that repository will simply fail to resolve.
- Main code uses the Java standard library only: no compile-scope or
  runtime-scope dependencies in `pom.xml`. Test scope may use JUnit 5, which is
  already available.

## Configuration

Tasks are described in `tasks.json`:

```json
{
  "tasks": {
    "codegen": {
      "command": "sh -c 'echo generated > gen.out'",
      "inputs": ["schema.txt"],
      "outputs": ["gen.out"]
    },
    "build": {
      "command": "sh -c 'cat gen.out > app'",
      "inputs": ["gen.out"],
      "outputs": ["app"],
      "deps": ["codegen"]
    }
  }
}
```

- The top-level object has exactly one key, `tasks`, whose value is an object
  keyed by task name.
- A task name matches `[A-Za-z0-9][A-Za-z0-9_.-]*`. Anything else is a
  configuration error.
- `command` is required and is a string run through the system shell.
- `inputs`, `outputs` and `deps` are optional arrays of strings, defaulting to
  empty.
- `inputs` and `outputs` are file paths relative to the directory containing the
  configuration file. Commands run with that directory as their working
  directory.
- Any other shape — a missing `tasks` key, a task that is not an object, a
  missing `command`, a non-array `deps`, malformed JSON — is a configuration
  error.

Parsing JSON with the standard library alone is part of the work.

## Requirements

### Command line

1. `java -jar target/taskrun.jar run [task...]` runs the named tasks and
   everything they depend on, directly or indirectly. With no task named, every
   task in the configuration runs. A named task absent from the configuration is
   a usage error.
2. `--file <path>` selects the configuration, defaulting to `tasks.json` in the
   current directory.
3. `--dry-run` prints the plan and runs nothing. See requirement 20 for its
   exact output.
4. `--force` runs every selected task regardless of cache state. No task is
   reported `cached` under `--force`.
5. Exit codes are exactly:
   - `0` when every selected task ended `ran` or `cached`.
   - `1` when any selected task ended `failed`, whatever else happened.
   - `2` for any usage or configuration error: unknown flag, missing flag value,
     unreadable or missing configuration file, malformed configuration, an
     invalid task name, or a named task absent from the configuration.

   A configuration error is detected before any task runs, so a run either exits
   2 having run nothing, or exits 0 or 1 having produced a full report.

### Configuration errors

6. A dependency naming a task that does not exist is a configuration error. The
   message names both the depending task and the missing name.
7. A dependency cycle is a configuration error. The message lists the task names
   forming the cycle.
8. Two tasks declaring the same output path is a configuration error. The
   message names both tasks and the path.

Configuration error messages go to standard error and are otherwise free-form,
graded only for naming what each requirement above says they must name.

### Execution

9. A task runs only after every one of its dependencies has succeeded. A task
   succeeds when its command exits `0`.
10. The reported order of tasks is deterministic: the same configuration and the
    same selected tasks always produce the same reported order, whatever order
    execution actually took. Execution order itself is constrained only to
    respect dependencies.
11. When a task fails, no task depending on it directly or indirectly runs, and
    tasks on unrelated branches still run.
12. Every selected task ends in exactly one reported state: `ran`, `cached`,
    `failed`, or `blocked` when a dependency failed or was itself blocked.
13. The reporting universe is exactly the selected tasks: those named on the
    command line plus everything they depend on transitively, or every task when
    none was named. Tasks outside that set are neither run nor reported.

### Caching

14. A task is skipped as `cached` when all of the following are unchanged since
    its last successful run: its command string, the contents of its declared
    inputs, and the cache identity of every task in `deps`. A task's cache
    identity is the value the cache stores for it after a successful run; how
    you compute it is yours to decide, provided a dependency that re-runs and
    produces different outputs causes its dependents to re-run.
15. Changing a task's command invalidates its cache entry.
16. Changing the contents of any declared input invalidates its cache entry.
17. A task whose declared outputs are not all present is never reported as
    `cached`: it runs again.
18. Cache state lives in a directory named `.taskrun-cache`, beside the
    configuration file. Deleting that directory returns the runner to a cold
    state and has no other effect. Nothing outside it persists between runs.

### Reporting

19. On a run without `--dry-run`, standard output carries one line per selected
    task, in reported order, of exactly `<state> <task-name>`, where state is one
    of `ran`, `cached`, `failed`, `blocked`; then a final line of exactly
    `summary ran=<n> cached=<n> failed=<n> blocked=<n>`.
20. On a run with `--dry-run`, standard output carries one line per selected
    task, in reported order, of exactly `plan <task-name>`, then a final line of
    exactly `summary planned=<n>`. No other requirement's output applies.
21. Everything else your program prints — progress, the output of task commands,
    diagnostics, anything the JVM emits — goes to standard error, so standard
    output is always a parseable record.
22. `--format json` replaces the standard output described in 19 and 20 with a
    single JSON object and nothing else. For a run, exactly the keys `order`
    (array of task names in reported order), `tasks` (object mapping each
    selected task name to its state string), and `summary` (object with integer
    keys `ran`, `cached`, `failed`, `blocked`). For `--dry-run`, exactly the keys
    `order` and `summary`, the latter with the single integer key `planned`.
    `--format text` is the default. For the same invocation, both formats report
    the same order, the same per-task states, and the same counts. Deriving one
    from the other is fine.

### Non-functional

23. `./mvnw -o -q package` succeeds from a fresh clone with no manual steps and
    no network, and `pom.xml` declares no compile-scope or runtime-scope
    dependency.
24. A configuration of 1000 tasks in a single dependency chain, whose work is
    entirely cached, completes within 20 seconds of wall-clock time, measured
    from process start to exit on an ordinary developer machine. The budget is
    generous because it includes JVM startup; it exists to rule out work that
    grows faster than the number of tasks, not to reward micro-optimisation.
25. Ship your own tests and a `requirements-map.json`: an object whose keys are
    the strings `"1"` through `"25"`, one for every requirement above, and whose
    values are arrays of test identifiers in JUnit form,
    `<fully.qualified.ClassName>#<methodName>`, each runnable alone via
    `./mvnw -o test -Dtest='<identifier>'`. Every requirement must have at least
    one. State the test command in `README.md` on a line of exactly
    `Test command: ./mvnw -o test`.

## Where this specification is silent

This specification pins the behaviour that is graded. It leaves exactly one
question open on purpose, and you should be able to find it.

Where you must choose something this specification does not settle, record it in
`DECISIONS.md`, one entry per decision. Each entry is a line of exactly:

```
requirement <n>: <key>: <value>
```

followed by any prose you like on the following lines. For the open question,
the key is `order-rule` and the value is one of `alphabetical`, `config-order`,
or `dependency-depth`, each meaning:

- `alphabetical` — by task name.
- `config-order` — the order tasks appear in the configuration file.
- `dependency-depth` — by longest path from a task with no dependencies, ties
  broken alphabetically.

Implement the rule you declare. A recorded decision honoured in the code is
correct, and all three are equally acceptable. Choosing silently is not.

## What to deliver

A repository that builds and runs from a fresh clone with no manual steps and no
network, containing at least `pom.xml`, the Maven wrapper, your Java sources,
`README.md` with the `Test command:` line, `requirements-map.json`,
`DECISIONS.md`, and your tests.
