# taskrun: a dependency-aware task runner

Build a command-line task runner in Python. Given a file describing tasks, each
with a command, declared inputs and outputs, and dependencies on other tasks, it
works out the order, runs them, skips work whose inputs have not changed, and
reports failures without running anything that depended on the failure.

## Configuration

Tasks are described in `tasks.json`:

```json
{
  "tasks": {
    "codegen": {
      "command": "python3 gen.py",
      "inputs": ["schema.txt"],
      "outputs": ["gen.py.out"]
    },
    "build": {
      "command": "cat gen.py.out > app",
      "inputs": ["gen.py.out"],
      "outputs": ["app"],
      "deps": ["codegen"]
    }
  }
}
```

`inputs`, `outputs` and `deps` are optional and default to empty.

## Requirements

### Command line

1. `python3 taskrun.py run [task...]` runs the named tasks and everything they
   depend on. With no task named, every task runs.
2. `--file <path>` selects the configuration, defaulting to `tasks.json`.
3. `--dry-run` prints the execution plan in the order tasks would run, one task
   name per line, and runs nothing.
4. `--force` runs every selected task regardless of cache state.
5. Exit codes are exactly: `0` when every selected task succeeded, `1` when any
   task failed, `2` for a usage or configuration error.

### Configuration errors

6. A dependency naming a task that does not exist is a configuration error. The
   message names both the depending task and the missing name.
7. A dependency cycle is a configuration error. The message lists the task names
   forming the cycle.
8. Two tasks declaring the same output path is a configuration error. The
   message names both tasks and the path.

### Execution

9. A task runs only after every one of its dependencies has succeeded.
10. The reported order of tasks is deterministic: the same configuration always
    produces the same order in the summary and in the JSON result, whatever
    order execution actually took. Execution order itself is constrained only to
    respect dependencies.
11. When a task fails, no task depending on it directly or indirectly runs, and
    tasks on unrelated branches still run.
12. Every task ends in exactly one reported state: `ran`, `cached`, `failed`, or
    `blocked` when a dependency failed.

### Caching

13. A task is skipped as `cached` when its command, the contents of its declared
    inputs, and the recorded results of its dependencies are all unchanged since
    its last successful run.
14. Changing a task's command invalidates its cache entry.
15. Changing the contents of any declared input invalidates its cache entry.
16. A task whose declared outputs are missing is never reported as `cached`: it
    runs again.
17. Cache state is stored under a single directory named in your `README.md`.
    Deleting that directory returns the runner to a cold state and has no other
    effect.

### Reporting

18. A run prints one line per task, in reported order, of exactly
    `<state> <task-name>`, where state is one of `ran`, `cached`, `failed`,
    `blocked`. Its final line is exactly
    `summary ran=<n> cached=<n> failed=<n> blocked=<n>`.
    Any other output goes to standard error, so standard output is a parseable
    record.
19. `--format json` writes a JSON result to standard output with exactly three
    keys:
    - `order`: an array of task names in reported order.
    - `tasks`: an object keyed by task name whose values are one of the four
      state strings.
    - `summary`: an object with integer keys `ran`, `cached`, `failed`,
      `blocked`.

    `--format text` is the default. For the same run, both formats report
    identical order, identical per-task states, and identical counts. Deriving
    one format from the other, or both from a shared result object, is fine.

### Non-functional

20. Python 3.11 or later, standard library only. The tool runs as
    `python3 taskrun.py` with no installation step.
21. A configuration of 1000 tasks whose work is entirely cached completes within
    5 seconds.
22. Ship your own tests and a `requirements-map.json`: an object keyed by
    requirement number (as a string) whose values are arrays of test
    identifiers. A test identifier is a string your stated test command accepts
    in order to run that test alone, in the form `<path>::<test-name>`. State
    that command in `README.md`.

## Where this specification is silent

Record every decision you take on a point this specification does not settle in
`DECISIONS.md`, one entry per decision, naming the requirement number and the
behaviour you chose. A decision recorded there and implemented consistently is
correct. Choosing silently is not.

## What to deliver

A repository that runs from a fresh clone with no manual steps, containing at
least `taskrun.py`, `README.md` stating the test command and the cache
directory, `requirements-map.json`, `DECISIONS.md`, and your tests.
