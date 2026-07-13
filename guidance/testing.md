# Testing Guidance

## Regression Safety Net

For new or modified tests:

### Coverage expectations
- Every new branch (success, failure, retry, timeout) should have a test; bug fixes add a test that fails before the fix.
- Exercise edge cases: null/empty input, duplicates, large payloads, permission failures, and error paths that trigger retries.
- Concurrency/time-based code should rely on fake clocks or controlled executors; avoid `Thread.sleep` or wall-clock dependence.
- Integration flows use stable fixtures (Testcontainers with clear lifecycle) rather than shared mutable singletons.
- In this repository, Java-side Maven unit tests should use `mvn -Pjava-tests test`, or
  `mvn -Pjava-tests -Dtest=SomeTest test` for focused runs. Treat `java-tests` as the Java unit-test profile, not as
  an ad hoc UI-skipping flag.

### Quality signals
- Assertions verify behaviour and side effects, not just invocation counts or `isNotNull`.
- Stubs/mocks live only at IO boundaries; prefer real domain objects for behaviour-heavy code.
- Tests are deterministic: fixed seeds, reproducible IDs, isolated temp directories, and no hidden coupling through static state.

### Red flags to call out
- Network/filesystem access in unit tests when a lightweight double exists.
- Newly added disabled/ignored tests without a linked issue or justification.
- Tests that log secrets or rely on global static state shared across classes.
